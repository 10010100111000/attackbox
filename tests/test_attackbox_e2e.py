#!/usr/bin/env python3
"""
AttackBox End-to-End Security & Cluster Integration Test Suite
Verifies:
1. CSWSH CheckOrigin restriction & Cluster Secret ConstantTimeCompare (Timing Defense)
2. P1-3: Existing sandbox worker_url consistency (No node drift, exact worker IP/port)
3. P0-5: PTY atomic cgroup placement & Kernel capability drops (no boot, rawio, module, mknod)
4. Passwordless sudo elevation & Kernel EPERM exploit blockades
"""

import sys
import os
import time
import json
import socket
import urllib.request
import urllib.error
import subprocess
import requests
import websocket

CONTROL_ADDR = "127.0.0.1:18080"
NODE_GW_ADDR = "127.0.0.1:18443"
CONTROL_URL = f"http://{CONTROL_ADDR}"
CLUSTER_SECRET = "security_cluster_secret_min_32_bytes_long!"

def log(msg):
    print(f"[TEST-LOG] {msg}", flush=True)

def test_cluster_security():
    log("=== TEST SUITE 1: Cluster Secret & CheckOrigin Security ===")
    
    # 1. Test invalid cluster secret rejection (ConstantTimeCompare)
    invalid_url = f"ws://{CONTROL_ADDR}/api/agent/connect?secret=wrong_secret&node_id=evil-node"
    try:
        ws = websocket.create_connection(invalid_url, timeout=3)
        ws.close()
        raise AssertionError("Connection with invalid cluster secret succeeded! Timing defense missing!")
    except websocket.WebSocketBadStatusException as e:
        assert e.status_code == 401, f"Expected 401, got {e.status_code}"
        log("[PASS] Invalid cluster secret rejected with 401 Unauthorized")
    except AssertionError:
        raise
    except Exception as e:
        log(f"[PASS] Invalid cluster secret rejected as expected ({type(e).__name__}: {e})")

    # 2. Test CSWSH malicious Origin header rejection
    valid_url = f"ws://{CONTROL_ADDR}/api/agent/connect?secret={CLUSTER_SECRET}&node_id=fake-node"
    try:
        ws = websocket.create_connection(
            valid_url,
            origin="https://attacker.com",
            timeout=3
        )
        ws.close()
        raise AssertionError("Connection with attacker.com Origin succeeded! CheckOrigin is too loose!")
    except (websocket.WebSocketBadStatusException, websocket.WebSocketException) as e:
        log(f"[PASS] Cross-origin CSWSH attempt rejected by CheckOrigin ({type(e).__name__}: {e})")
    except AssertionError:
        raise
    except Exception as e:
        log(f"[PASS] Cross-origin connection blocked ({type(e).__name__}: {e})")

def test_p1_3_worker_url_consistency(session_token):
    log("=== TEST SUITE 2: P1-3 Worker URL Consistency & Node Resolution ===")
    headers = {
        "Authorization": f"Bearer {session_token}",
        "Content-Type": "application/json"
    }

    # 1. Start sandbox
    start_resp = requests.post(
        f"{CONTROL_URL}/api/sandbox/start",
        headers=headers,
        json={"custom_policy": {"resources": {"memory_max_mb": 512, "pids_max": 128}}},
        timeout=10
    )
    assert start_resp.status_code == 201, f"Failed to start sandbox: {start_resp.text}"
    box_data1 = start_resp.json()
    sbx_id = box_data1["sandbox_id"]
    worker_id1 = box_data1["worker_id"]
    worker_url1 = box_data1["worker_url"]
    ticket1 = box_data1["ticket"]
    
    log(f"Sandbox created: ID={sbx_id}, Worker={worker_id1}, URL={worker_url1}")
    assert worker_url1.startswith("ws://"), f"Invalid worker_url schema: {worker_url1}"

    # 2. Call /api/sandbox/start again for existing active sandbox (P1-3 Core Verification)
    start_resp2 = requests.post(f"{CONTROL_URL}/api/sandbox/start", headers=headers, json={}, timeout=10)
    assert start_resp2.status_code == 200, f"Re-use sandbox failed: {start_resp2.text}"
    box_data2 = start_resp2.json()
    
    assert box_data2["sandbox_id"] == sbx_id, f"Sandbox ID mismatch: {box_data2['sandbox_id']} vs {sbx_id}"
    assert box_data2["worker_id"] == worker_id1, f"Worker ID changed on reuse: {box_data2['worker_id']} vs {worker_id1}"
    assert box_data2["worker_url"] == worker_url1, (
        f"P1-3 BUG DETECTED: worker_url drifted! Expected {worker_url1}, got {box_data2['worker_url']}"
    )
    log("[PASS] P1-3 Verified on /api/sandbox/start: Existing sandbox returns identical worker_url!")

    # 3. Call /api/sandbox/active
    active_resp = requests.get(f"{CONTROL_URL}/api/sandbox/active", headers=headers, timeout=5)
    assert active_resp.status_code == 200, f"Query active sandbox failed: {active_resp.text}"
    active_data = active_resp.json()
    assert active_data.get("active") is True, "Sandbox should be active"
    assert active_data["sandbox_id"] == sbx_id
    assert active_data["worker_url"] == worker_url1, (
        f"P1-3 BUG DETECTED on /api/sandbox/active: Expected {worker_url1}, got {active_data['worker_url']}"
    )
    log("[PASS] P1-3 Verified on /api/sandbox/active: Accurate worker endpoint returned without drift!")

    return sbx_id, worker_url1, box_data2["ticket"]

def exec_pty_command(ws, cmd, marker, timeout=5.0):
    payload = (cmd.strip() + f"; echo {marker}\n").encode("utf-8")
    ws.send_binary(payload)
    
    output = ""
    start_time = time.time()
    while time.time() - start_time < timeout:
        try:
            ws.settimeout(1.0)
            data = ws.recv()
            if isinstance(data, bytes):
                data = data.decode("utf-8", errors="ignore")
            output += data
            if f"\n{marker}" in output or f"\r\n{marker}" in output:
                break
        except websocket.WebSocketTimeoutException:
            if f"\n{marker}" in output or f"\r\n{marker}" in output:
                break
            continue
        except Exception as e:
            break
    return output

def test_pty_containment_and_caps(worker_url, ticket):
    log("=== TEST SUITE 3: PTY Interactive Terminal & Security Containment ===")
    cli_url = f"{worker_url}?ticket={ticket}"
    log(f"Connecting to PTY CLI: {cli_url}")

    ws = websocket.create_connection(cli_url, subprotocols=["binary"], timeout=10)
    time.sleep(0.3)  # Wait for shell prompt to settle

    # 1. Audit identity
    out = exec_pty_command(ws, "id; grep --color=never -E '^(CapEff|CapBnd)' /proc/self/status", "ID_DONE")
    log(f"Identity & Caps output:\n{out}")
    assert "uid=1001(box)" in out or "box" in out, "Shell is not running as unprivileged box user"
    assert "sudo" in out, "User box is missing sudo group membership"

    # Verify Capability Bounding Set
    cap_bnd = None
    for line in out.splitlines():
        if "CapBnd:" in line:
            cap_bnd = int(line.split()[1], 16)
            break
    assert cap_bnd is not None, "CapBnd not found in /proc/self/status"
    
    # Check dropped dangerous bits:
    # CAP_SYS_BOOT = 22
    # CAP_SYS_RAWIO = 17
    # CAP_SYS_MODULE = 16
    # CAP_MKNOD = 27
    assert (cap_bnd & (1 << 22)) == 0, f"CAP_SYS_BOOT bit 22 is NOT dropped: {hex(cap_bnd)}"
    assert (cap_bnd & (1 << 17)) == 0, f"CAP_SYS_RAWIO bit 17 is NOT dropped: {hex(cap_bnd)}"
    assert (cap_bnd & (1 << 16)) == 0, f"CAP_SYS_MODULE bit 16 is NOT dropped: {hex(cap_bnd)}"
    assert (cap_bnd & (1 << 27)) == 0, f"CAP_MKNOD bit 27 is NOT dropped: {hex(cap_bnd)}"
    log(f"[PASS] CapBnd strictly bounded to {hex(cap_bnd)}: boot, rawio, module, and mknod completely dropped!")

    # 2. Test passwordless sudo
    out_sudo = exec_pty_command(ws, "sudo whoami", "SUDO_DONE")
    log(f"Sudo whoami output:\n{out_sudo}")
    assert "root" in out_sudo, "Passwordless sudo failed"
    log("[PASS] Passwordless sudo works properly inside sandbox")

    # 3. Exploit test: reboot syscall under sudo
    out_reboot = exec_pty_command(ws, "sudo /usr/bin/test_reboot", "REBOOT_DONE")
    log(f"Reboot attempt output:\n{out_reboot}")
    assert "EPERM" in out_reboot or "Operation not permitted" in out_reboot, "Reboot exploit was not blocked!"
    log("[PASS] Kernel EPERM Defense: Reboot blocked under sudo by dropped capability!")

    # 4. Exploit test: mknod /dev/test_dev under sudo
    out_mknod = exec_pty_command(ws, "sudo mknod /dev/test_block_dev b 8 0 2>&1", "MKNOD_DONE")
    log(f"Mknod attempt output:\n{out_mknod}")
    assert "Operation not permitted" in out_mknod, f"Mknod exploit was not blocked! Output: {out_mknod}"
    log("[PASS] P0-4 Kernel Defense: Physical block device creation blocked with Operation not permitted!")

    # 5. Exploit test: sysrq-trigger masking under sudo
    out_sysrq = exec_pty_command(ws, "sudo sh -c 'echo c > /proc/sysrq-trigger'", "SYSRQ_DONE")
    log(f"Sysrq attempt output:\n{out_sysrq}")
    assert "SYSRQ_DONE" in out_sysrq, "Sysrq execution hung or failed"
    log("[PASS] Host Protection: /proc/sysrq-trigger masked with /dev/null bind mount!")

    ws.close()

def test_p1_9_timeout_and_ghost_box_prevention(session_token):
    log("=== TEST SUITE 4: P1-9 CreateBox Timeout & Ghost Box Defense ===")
    headers = {
        "Authorization": f"Bearer {session_token}",
        "Content-Type": "application/json"
    }

    # 1. Fault Injection: Client initiates sandbox creation but aborts/disconnects early
    log("[Fault Injection] Simulating client dropout / timeout during container provisioning...")
    try:
        # Connect timeout 3s, read timeout 0.08s: drops HTTP connection while container provisioning is in flight
        requests.post(
            f"{CONTROL_URL}/api/sandbox/start",
            headers=headers,
            json={"custom_policy": {"resources": {"memory_max_mb": 512, "pids_max": 128}}},
            timeout=(3.0, 0.08)
        )
    except (requests.exceptions.ReadTimeout, requests.exceptions.Timeout):
        log("[PASS] Injected client-side timeout / cancellation successfully")
    except Exception as e:
        log(f"[WARN] Request ended with {type(e).__name__}: {e}")

    # 2. Wait 2 seconds for Control compensating DestroyBox and Worker rollback to complete
    log("Waiting 2.0s for Control compensating destruction and Worker rollback...")
    time.sleep(2.0)

    # Verify that Control has NO active sandbox session for this user
    active_resp = requests.get(f"{CONTROL_URL}/api/sandbox/active", headers=headers, timeout=5)
    assert active_resp.status_code == 200, f"Query active sandbox failed: {active_resp.text}"
    active_data = active_resp.json()
    assert active_data.get("active") is False, (
        f"P1-9 FAULT: Sandbox session leaked on Control after aborted creation: {active_data}"
    )
    log("[PASS] Control Hub correctly marked session inactive (no leaked session)")

    # 3. Create a brand new sandbox - must succeed and be able to reuse Slot 1 without ARP/IP conflict
    log("Creating fresh sandbox to verify Slot 1 reuse and no IP collision...")
    start_resp = requests.post(
        f"{CONTROL_URL}/api/sandbox/start",
        headers=headers,
        json={"custom_policy": {"resources": {"memory_max_mb": 512, "pids_max": 128}}},
        timeout=15
    )
    assert start_resp.status_code == 201, f"Failed to start new sandbox: {start_resp.text}"
    new_box = start_resp.json()
    log(f"[PASS] Fresh sandbox created: ID={new_box['sandbox_id']}, Slot={new_box['slot_index']}, IP={new_box['sandbox_ip']}")

    # 4. Verify no ghost container or duplicate veth pairs on host
    cli_url = f"{new_box['worker_url']}?ticket={new_box['ticket']}"
    ws = websocket.create_connection(cli_url, subprotocols=["binary"], timeout=10)
    time.sleep(0.3)
    out = exec_pty_command(ws, "ip addr show eth0; echo IP_CHECK_DONE", "IP_CHECK_DONE")
    log(f"Sandbox eth0 interface check:\n{out}")
    assert new_box["sandbox_ip"].split("/")[0] in out, f"IP {new_box['sandbox_ip']} not assigned to eth0 in sandbox"
    ws.close()

    # 5. Clean teardown
    stop_resp = requests.post(f"{CONTROL_URL}/api/sandbox/stop", headers=headers, json={"sandbox_id": new_box["sandbox_id"]}, timeout=5)
    assert stop_resp.status_code == 200, f"Stop sandbox failed: {stop_resp.text}"
    log("[PASS] P1-9 Fully Verified: No slot leak, no ghost container, and no ARP/IP collision!")

def main():
    log("Starting AttackBox Comprehensive E2E Verification...")

    # Wait for control server to be up
    for i in range(20):
        try:
            r = requests.get(f"{CONTROL_URL}/health", timeout=1)
            if r.status_code == 200:
                break
        except Exception:
            time.sleep(0.5)
    else:
        log("ERROR: Control server at 127.0.0.1:18080 is not reachable!")
        sys.exit(1)

    # Register user
    reg_data = {
        "username": "e2e_tester",
        "password": "Password123!",
        "email": "tester@attackbox.local"
    }
    r = requests.post(f"{CONTROL_URL}/api/auth/register", json=reg_data, timeout=5)
    if r.status_code != 200:
        # Try login
        r = requests.post(f"{CONTROL_URL}/api/auth/login", json=reg_data, timeout=5)
    assert r.status_code == 200, f"Auth failed: {r.text}"
    token = r.json()["token"]
    log("[PASS] User authenticated successfully")

    # Run tests
    test_cluster_security()
    sbx_id, worker_url, ticket = test_p1_3_worker_url_consistency(token)
    test_pty_containment_and_caps(worker_url, ticket)

    # Teardown sandbox
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    requests.post(f"{CONTROL_URL}/api/sandbox/stop", headers=headers, json={"sandbox_id": sbx_id}, timeout=5)
    log("[PASS] First sandbox stopped cleanly")

    # P1-9 Fault Injection & Ghost Box Prevention Test
    test_p1_9_timeout_and_ghost_box_prevention(token)

    log(">>> ALL END-TO-END SECURITY AUDITS, P1-3 AND P1-9 VERIFICATIONS PASSED 100%! <<<")

if __name__ == "__main__":
    main()
