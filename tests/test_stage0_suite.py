import urllib.request
import urllib.error
import http.cookiejar
import ssl
import json
import subprocess
import time
import sys
import os

print("=" * 60)
print(" AttackBox Stage 0 Hardening & Service Verification Suite")
print("=" * 60)

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE
BASE_URL = 'https://127.0.0.1'

def make_session():
    cj = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(
        urllib.request.HTTPCookieProcessor(cj),
        urllib.request.HTTPSHandler(context=ctx)
    )
    def req(method, path, data=None):
        url = BASE_URL + path
        body = None
        headers = {}
        if data is not None:
            body = json.dumps(data).encode('utf-8')
            headers['Content-Type'] = 'application/json'
        r = urllib.request.Request(url, data=body, headers=headers, method=method)
        with opener.open(r) as resp:
            return json.loads(resp.read().decode('utf-8'))
    req.opener = opener
    req.cj = cj
    return req

client = make_session()

# Step 1: Cleanup old target_lab and create fresh target namespace
print("\n[STEP 1] Setting up multi-service target lab on br0...")
subprocess.run("sudo ip netns del target_lab 2>/dev/null || true", shell=True)
subprocess.run("sudo ip link del veth-tgt-h 2>/dev/null || true", shell=True)
subprocess.run("sudo pkill -f 'http.server 80' || true", shell=True)
subprocess.run("sudo pkill -f 'impacket-smbserver' || true", shell=True)

# Create namespace and veth attached to br0
subprocess.check_call("sudo ip netns add target_lab", shell=True)
subprocess.check_call("sudo ip link add veth-tgt-h type veth peer name veth-tgt-ns", shell=True)
subprocess.check_call("sudo ip link set veth-tgt-h master br0 up", shell=True)
subprocess.check_call("sudo ip link set veth-tgt-ns netns target_lab", shell=True)
subprocess.check_call("sudo ip netns exec target_lab ip link set lo up", shell=True)
subprocess.check_call("sudo ip netns exec target_lab ip addr add 10.200.0.99/24 dev veth-tgt-ns", shell=True)
subprocess.check_call("sudo ip netns exec target_lab ip link set veth-tgt-ns up", shell=True)
subprocess.check_call("sudo ip netns exec target_lab ip route add default via 10.200.0.1", shell=True)

# Prepare target files
subprocess.check_call("sudo mkdir -p /tmp/target_web /tmp/target_smb", shell=True)
subprocess.check_call("echo 'FLAG_HTTP_ATTACKBOX_VERIFIED' | sudo tee /tmp/target_web/flag.txt >/dev/null", shell=True)
subprocess.check_call("echo 'FLAG_SMB_IMPACKET_VERIFIED' | sudo tee /tmp/target_smb/secret.txt >/dev/null", shell=True)

# Start Web Server on Target (Port 80)
subprocess.Popen("sudo ip netns exec target_lab python3 -m http.server 80 --directory /tmp/target_web", shell=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

# Start SMB Server on Target (Port 445) using Impacket SMBServer
subprocess.Popen("sudo ip netns exec target_lab impacket-smbserver -smb2support SHARED /tmp/target_smb", shell=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

time.sleep(3)

# Test target ports from host
res80 = subprocess.getoutput("nc -z -w 3 10.200.0.99 80; echo $?")
res445 = subprocess.getoutput("nc -z -w 3 10.200.0.99 445; echo $?")
print(f"Target 10.200.0.99 Port 80 status: {'OPEN' if res80.endswith('0') else 'FAIL'}")
print(f"Target 10.200.0.99 Port 445 status: {'OPEN' if res445.endswith('0') else 'FAIL'}")
assert res80.endswith('0'), "Target Port 80 failed to open"
assert res445.endswith('0'), "Target Port 445 failed to open"
print("✓ Target multi-service lab running at 10.200.0.99 (HTTP:80, SMB:445)")

# Step 2: Login and Spawn Sandbox
print("\n[STEP 2] Logging into AttackBox and spawning sandbox...")
login_res = client('POST', '/api/login', {'username': 'laji', 'password': 'qwer1234!'})
assert login_res.get('success') is True, "Login failed"

# Clean any existing sandbox
client('POST', '/api/env/destroy')
time.sleep(1)

start_res = client('POST', '/api/env/cli/toggle')
print("Spawn result:", start_res)
assert start_res.get('success') is True, "Failed to start sandbox"

box_id = start_res.get('boxId', 'box1')
time.sleep(2)

pid_file = f"/run/attackbox/{box_id}/init.pid"
init_pid = subprocess.getoutput(f"cat {pid_file} 2>/dev/null").strip()
print(f"Sandbox {box_id} init PID: {init_pid}")
assert init_pid and init_pid.isdigit(), "Invalid sandbox PID"

def sandbox_exec(cmd):
    full_cmd = f"sudo nsenter --target {init_pid} --mount --uts --ipc --net --pid --root --wd /bin/bash -c {json.dumps(cmd)}"
    p = subprocess.run(full_cmd, shell=True, capture_output=True, text=True)
    return p.returncode, p.stdout.strip(), p.stderr.strip()

# Step 3: Test Tools inside Sandbox
print("\n[STEP 3] Testing reconnaissance and penetration tools from inside sandbox...")

# 3.1 Nmap Scanning Target
print("\n--- 3.1 Nmap Target Scanning ---")
rc, out, err = sandbox_exec("nmap -sT -p 80,445 -Pn 10.200.0.99")
print(out)
assert "80/tcp  open  http" in out, "Nmap failed to detect open port 80"
assert "445/tcp open  microsoft-ds" in out, "Nmap failed to detect open port 445"
print("✓ Nmap successfully scanned target ports through br0!")

# 3.2 HTTP Web Tool
print("\n--- 3.2 Curl HTTP Access ---")
rc, out, err = sandbox_exec("curl -s http://10.200.0.99/flag.txt")
print("Response:", out)
assert "FLAG_HTTP_ATTACKBOX_VERIFIED" in out, "HTTP request failed"
print("✓ Curl web request to target succeeded!")

# 3.3 Impacket SMB Client
print("\n--- 3.3 Impacket SMB Client & Retrieval ---")
rootfs = f"/var/lib/attackbox/boxes/{box_id}/rootfs"
impacket_py_content = """from impacket.smbconnection import SMBConnection
import io
conn = SMBConnection('10.200.0.99', '10.200.0.99', sess_port=445)
conn.login('', '')
files = conn.listPath('SHARED', '*')
print('FILES:', [f.get_longname() for f in files])
buf = io.BytesIO()
conn.getFile('SHARED', 'secret.txt', buf.write)
print('CONTENT:', buf.getvalue().decode('utf-8').strip())
"""
subprocess.run(f"sudo tee {rootfs}/tmp/run_smb.py >/dev/null << 'EOF'\n{impacket_py_content}\nEOF", shell=True)
rc, out, err = sandbox_exec("python3 /tmp/run_smb.py")
print("Impacket output:\n", out)
if err:
    print("Impacket stderr:\n", err)
assert "FLAG_SMB_IMPACKET_VERIFIED" in out, "Impacket failed to retrieve SMB flag"
print("✓ Impacket successfully authenticated to target SMB and retrieved secret!")

# Step 4: Outbound Security Policy Verification
print("\n[STEP 4] Verifying outbound security policies...")

# 4.1 Allowed Web Outbound (Port 80/443 for apt/pip/wordlists)
print("\n--- 4.1 Outbound Web Allowed (TCP 80/443) ---")
rc, out, err = sandbox_exec("curl -I -s --connect-timeout 5 https://1.1.1.1:443")
print("Curl 1.1.1.1:443 response headers:\n", out)
assert rc == 0 or "HTTP/" in out or "cloudflare" in out.lower(), f"Outbound 443 blocked unexpectedly: rc={rc}, err={err}"
print("✓ Outbound Web (443) is properly permitted for package/wordlist downloads!")

# 4.2 Blocked Non-Web Outbound (Port 22/3389/etc to Internet)
print("\n--- 4.2 Outbound Attacks to Internet Blocked (e.g. WAN Port 22) ---")
probe_wan = "import socket\ns=socket.socket()\ns.settimeout(3)\ns.connect(('1.1.1.1', 22))\n"
subprocess.run(f"sudo tee {rootfs}/tmp/probe_wan.py >/dev/null << 'EOF'\n{probe_wan}\nEOF", shell=True)
rc, out, err = sandbox_exec("python3 /tmp/probe_wan.py")
print(f"Connection to WAN 1.1.1.1:22 exit code: {rc} (Expect error/timeout/refused)")
assert rc != 0, "Security violation: Outbound SSH to Internet was not blocked!"
print("✓ Outbound traffic to non-80/443 WAN ports is strictly blocked!")

# 4.3 Host Protection from Sandbox Attack
print("\n--- 4.3 Host Protection (Sandbox targeting host 10.200.0.1:22) ---")
probe_host = "import socket\ns=socket.socket()\ns.settimeout(2)\ns.connect(('10.200.0.1', 22))\n"
subprocess.run(f"sudo tee {rootfs}/tmp/probe_host.py >/dev/null << 'EOF'\n{probe_host}\nEOF", shell=True)
rc, out, err = sandbox_exec("python3 /tmp/probe_host.py")
print(f"Connection from sandbox to host 10.200.0.1:22 exit code: {rc} (Expect failure)")
assert rc != 0, "Security violation: Sandbox could connect to host SSH!"
print("✓ Host services are securely protected from sandbox attack!")

# Step 5: Sysctl & Ptrace Hardening Verification
print("\n[STEP 5] Verifying kernel sysctl hardening...")
rc, out, err = sandbox_exec("strace ls")
print(f"Sandbox 'strace ls' result: rc={rc}, err={err}")
assert "Operation not permitted" in err or rc != 0, "Ptrace should be blocked by ptrace_scope=3"
print("✓ Ptrace / strace / gdb is successfully blocked by ptrace_scope=3!")

# Verify host sysctl
host_ptrace = subprocess.getoutput("cat /proc/sys/kernel/yama/ptrace_scope").strip()
host_bpf = subprocess.getoutput("cat /proc/sys/kernel/unprivileged_bpf_disabled").strip()
host_kexec = subprocess.getoutput("cat /proc/sys/kernel/kexec_load_disabled").strip()
host_io_uring = subprocess.getoutput("cat /proc/sys/kernel/io_uring_disabled").strip()
print(f"Host sysctl: ptrace_scope={host_ptrace}, bpf_disabled={host_bpf}, kexec_disabled={host_kexec}, io_uring_disabled={host_io_uring}")
assert host_ptrace == '3', f"Expected ptrace_scope=3, got {host_ptrace}"
assert host_bpf == '2', f"Expected bpf_disabled=2, got {host_bpf}"
assert host_kexec == '1', f"Expected kexec_disabled=1, got {host_kexec}"
assert host_io_uring == '2', f"Expected io_uring_disabled=2, got {host_io_uring}"
print("✓ Kernel sysctl parameters fully hardened and verified!")

# Step 6: Cleanup
print("\n[STEP 6] Cleaning up test environment...")
client('POST', '/api/env/destroy')
subprocess.run("sudo ip netns del target_lab 2>/dev/null || true", shell=True)
subprocess.run("sudo ip link del veth-tgt-h 2>/dev/null || true", shell=True)
subprocess.run("sudo pkill -f 'http.server 80' || true", shell=True)
subprocess.run("sudo pkill -f 'impacket-smbserver' || true", shell=True)
subprocess.run("sudo rm -rf /tmp/target_web /tmp/target_smb 2>/dev/null || true", shell=True)

print("\n" + "=" * 60)
print(" ALL TESTS PASSED! Stage 0 Hardening 100% Verified!")
print("=" * 60)
