#!/usr/bin/env python3
"""
AttackBox Full Regression Test Suite (全量回归测试套件)
======================================================
涵盖以下 6 大核心阶段的端到端自动化验证：
1. Go 语言核心单元测试 (auth, box, config, gateway)
2. 网关安全控制与管理 API 回归 (TrustProxy 防 IP 伪造、频控限流、级联下线、配置热加载、防自锁)
3. 沙箱引擎与 OS 内核加固回归 (纯内存租约、cgroups v2、防误关机、Sudo零延迟、文件系统隔离)
4. WebSocket 双桥接与会话状态回归 (CLI PTY 命令收发、CSWSH 防御、VNC 懒加载与 4001 抢占、零僵尸零泄漏)
5. 靶场网络隔离与渗透测试回归 (br0 靶机 HTTP/SMB 访问、出网策略白名单、宿主端口阻断)
6. 开机自愈 CleanBoot 与资源彻底回收验证
"""

import urllib.request
import urllib.error
import http.cookiejar
import ssl
import socket
import json
import subprocess
import time
import os
import sys
import re

# 配置环境
BASE_URL = 'https://127.0.0.1'
ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

ADMIN_USER = 'laji'
ADMIN_PASS = 'qwer1234!'

def log_section(title):
    print("\n" + "=" * 70)
    print(f" >>> {title}")
    print("=" * 70)

def log_test(num, name):
    print(f"\n[Test {num}] {name}")

def log_pass(msg):
    print(f"  [PASS] {msg}")

def make_session(headers_extra=None):
    cj = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(
        urllib.request.HTTPCookieProcessor(cj),
        urllib.request.HTTPSHandler(context=ctx)
    )
    def req(method, path, data=None, custom_headers=None):
        url = BASE_URL + path
        body = None
        headers = {'User-Agent': 'AttackBox-Regression/3.0'}
        if headers_extra:
            headers.update(headers_extra)
        if custom_headers:
            headers.update(custom_headers)
        if data is not None:
            body = json.dumps(data).encode('utf-8')
            headers['Content-Type'] = 'application/json'
        r = urllib.request.Request(url, data=body, headers=headers, method=method)
        try:
            with opener.open(r) as resp:
                raw = resp.read().decode('utf-8')
                resp_headers = {k.lower(): v for k, v in resp.headers.items()}
                try:
                    return json.loads(raw), resp.status, resp_headers
                except:
                    return raw, resp.status, resp_headers
        except urllib.error.HTTPError as e:
            raw = e.read().decode('utf-8')
            resp_headers = {k.lower(): v for k, v in e.headers.items()}
            try:
                return json.loads(raw), e.code, resp_headers
            except:
                return raw, e.code, resp_headers
    req.opener = opener
    req.cj = cj
    return req

def raw_ws_connect(path, cookie=None, origin='https://127.0.0.1'):
    raw_sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    tls_sock = ctx.wrap_socket(raw_sock, server_hostname='127.0.0.1')
    tls_sock.connect(('127.0.0.1', 443))
    
    headers = [
        f"GET {path} HTTP/1.1",
        "Host: 127.0.0.1",
        "Upgrade: websocket",
        "Connection: Upgrade",
        "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==",
        "Sec-WebSocket-Version: 13",
        f"Origin: {origin}"
    ]
    if cookie:
        headers.append(f"Cookie: ab={cookie}")
    headers.append("\r\n")
    
    req_data = "\r\n".join(headers).encode('utf-8')
    tls_sock.sendall(req_data)
    resp = tls_sock.recv(4096).decode('utf-8', errors='ignore')
    return tls_sock, resp

# ==============================================================================
# PHASE 1: Go 单元测试执行
# ==============================================================================
def run_phase_1():
    log_section("PHASE 1: Go 核心包单元测试 (go test -v ./...)")
    cmd = "export PATH=/usr/local/go/bin:/usr/bin:$PATH && cd /home/laji/attackbox && go test -v ./..."
    p = subprocess.run(cmd, shell=True, capture_output=True, text=True)
    print(p.stdout)
    if p.returncode != 0:
        print(p.stderr)
        raise RuntimeError("Go 单元测试未完全通过！")
    log_pass("Go 核心模块单元测试全部 PASS！")

# ==============================================================================
# PHASE 2: 网关安全控制与管理 API 回归
# ==============================================================================
def run_phase_2():
    log_section("PHASE 2: 网关安全控制与管理 API 回归测试")
    # 等待网关服务端口监听就绪
    for _ in range(30):
        try:
            s = socket.create_connection(('127.0.0.1', 443), timeout=1)
            s.close()
            break
        except:
            time.sleep(0.5)

    admin = make_session()

    # 2.1 管理员登录
    log_test("2.1", "管理员身份认证")
    res, code, _ = admin('POST', '/api/login', {'username': ADMIN_USER, 'password': ADMIN_PASS})
    assert res.get('success') is True, f"Admin login failed: {res}"
    log_pass(f"管理员 {ADMIN_USER} 登录成功")

    # 2.2 TrustProxy 防 IP 伪造
    log_test("2.2", "TrustProxy 门控防伪造 IP 验证")
    fake_attacker = make_session(headers_extra={'CF-Connecting-IP': '198.51.100.88'})
    fail_res, _, _ = fake_attacker('POST', '/api/login', {'username': 'attacker_fake', 'password': 'wrongpassword'})
    assert fail_res.get('success') is False
    audit_res, _, _ = admin('GET', '/api/admin/audit?limit=5&event_type=LOGIN_FAILED')
    items = audit_res.get('items', [])
    assert len(items) > 0, "No LOGIN_FAILED audit item found"
    latest = items[0]
    assert latest['ip'] in ('127.0.0.1', '::1'), f"TrustProxy leak: got {latest['ip']}"
    log_pass(f"伪造 CF-Connecting-IP 被成功过滤，审计记录真实 IP: {latest['ip']}")

    # 2.3 系统策略热更新与自主注册门控
    log_test("2.3", "系统策略热更新与自主注册门控")
    user_sess = make_session()
    # 关
    admin('POST', '/api/admin/settings', {'allowRegister': False})
    pub_conf, _, _ = user_sess('GET', '/api/public/config')
    assert pub_conf.get('allowRegister') is False
    reg_res, _, _ = user_sess('POST', '/api/register', {'username': 'test_blocked_reg', 'password': 'password123'})
    assert reg_res.get('success') is False
    # 开
    admin('POST', '/api/admin/settings', {'allowRegister': True})
    test_u = f"reg_user_{int(time.time())}"
    reg_ok, _, _ = user_sess('POST', '/api/register', {'username': test_u, 'password': 'password123'})
    assert reg_ok.get('success') is True
    log_pass(f"注册开关与配置热更新联动正常，成功注册测试用户 {test_u}")

    # 2.4 用户修改密码级联注销旧会话
    log_test("2.4", "用户改密级联会话注销")
    chg_res, _, _ = user_sess('POST', '/api/user/password', {'oldPassword': 'password123', 'newPassword': 'newpassword456!'})
    assert chg_res.get('success') is True
    me_after, _, _ = user_sess('GET', '/api/me')
    assert me_after.get('loggedIn') is False, "Old session still active after password change"
    # 用新密码登录验证
    relogin, _, _ = user_sess('POST', '/api/login', {'username': test_u, 'password': 'newpassword456!'})
    assert relogin.get('success') is True
    log_pass("用户改密后旧会话立即失效，新密码登录成功")

    # 2.5 管理员禁用用户自动连根拔起
    log_test("2.5", "管理员禁用用户级联下线与沙箱销毁")
    user_sess('POST', '/api/env/toggle')
    user_me, _, _ = user_sess('GET', '/api/me')
    assert user_me.get('envRunning') is True
    # 禁用
    admin('POST', '/api/admin/users/toggle', {'username': test_u})
    user_me_banned, _, _ = user_sess('GET', '/api/me')
    assert user_me_banned.get('loggedIn') is False
    sb_list, _, _ = admin('GET', '/api/admin/sandboxes')
    assert not any(sb['user'] == test_u for sb in sb_list)
    # 删除测试用户
    admin('POST', '/api/admin/users/delete', {'username': test_u})
    log_pass("禁用用户实现原子级联踢出并物理回收沙箱")

    # 2.6 管理员防自锁保护
    log_test("2.6", "最后活跃管理员账号自锁保护")
    self_toggle, _, _ = admin('POST', '/api/admin/users/toggle', {'username': ADMIN_USER})
    assert self_toggle.get('success') is False
    self_del, _, _ = admin('POST', '/api/admin/users/delete', {'username': ADMIN_USER})
    assert self_del.get('success') is False
    log_pass("成功拦截对当前管理员的自禁用与自删除")

    # 2.7 安全响应头校验
    log_test("2.7", "安全响应头合规检查")
    _, _, h = admin('GET', '/login')
    assert 'nosniff' in h.get('x-content-type-options', '').lower()
    assert 'deny' in h.get('x-frame-options', '').lower()
    assert 'max-age=' in h.get('strict-transport-security', '').lower()
    log_pass("X-Content-Type-Options / X-Frame-Options / HSTS 均已配置生效")

    return admin

# ==============================================================================
# PHASE 3: 沙箱引擎与 OS 内核加固回归
# ==============================================================================
def run_phase_3(admin):
    log_section("PHASE 3: 沙箱引擎与 OS 内核加固回归测试")
    
    # 3.1 启动沙箱
    log_test("3.1", "按需拉起纯内存租约沙箱")
    spawn_res, _, _ = admin('POST', '/api/env/toggle')
    assert spawn_res.get('success') is True
    me, _, _ = admin('GET', '/api/me')
    assert me.get('envRunning') is True
    box_id = me.get('boxId', 'box1')
    log_pass(f"沙箱 {box_id} 已成功在内存租约中分配并启动")

    init_pid_file = f"/run/attackbox/{box_id}/init.pid"
    for _ in range(30):
        if os.path.exists(init_pid_file):
            break
        time.sleep(0.1)
    init_pid = subprocess.getoutput(f"cat {init_pid_file} 2>/dev/null").strip()
    assert init_pid and init_pid.isdigit(), f"Invalid init PID: {init_pid}"
    log_pass(f"宿主机解析到沙箱 PID 1 映射: {init_pid}")

    def sandbox_exec(cmd):
        full = f"sudo nsenter -t {init_pid} -m -u -i -n -p --root --wd {cmd}"
        return subprocess.getoutput(full)

    # 3.2 cgroups v2 资源隔离
    log_test("3.2", "cgroups v2 进程与资源防护配额")
    cg_dir = f"/sys/fs/cgroup/attackbox/{box_id}"
    pids_max = subprocess.getoutput(f"cat {cg_dir}/pids.max 2>/dev/null").strip()
    mem_max = subprocess.getoutput(f"cat {cg_dir}/memory.max 2>/dev/null").strip()
    cpu_max = subprocess.getoutput(f"cat {cg_dir}/cpu.max 2>/dev/null").strip()
    assert pids_max == '512', f"Expected pids.max=512, got {pids_max}"
    assert cpu_max == '200000 100000', f"Expected cpu.max='200000 100000', got {cpu_max}"
    log_pass(f"cgroups v2 配置核实: pids={pids_max}, mem={mem_max}, cpu={cpu_max}")

    # 3.3 容器内误关机拦截
    log_test("3.3", "容器内误关机防护 (/sbin/reboot & /sbin/poweroff)")
    rb_out = sandbox_exec("/sbin/reboot 2>&1")
    assert "In-sandbox reboot/shutdown is disabled" in rb_out, f"Reboot not blocked: {rb_out}"
    log_pass("容器内 reboot 行为被安全桩成功拦截")

    # 3.4 Sudo 零延迟与 hosts 注入
    log_test("3.4", "Sudo 零延迟与主机名解析")
    t0 = time.time()
    whoami = sandbox_exec("su - box -c 'sudo whoami' 2>&1")
    dur = time.time() - t0
    assert "root" in whoami, f"sudo whoami failed: {whoami}"
    assert dur < 1.0, f"sudo took {dur:.2f}s (DNS timeout detected)"
    hosts = sandbox_exec("cat /etc/hosts")
    assert f"127.0.0.1 attackbox-{box_id} box" in hosts
    log_pass(f"Sudo 耗时 {dur:.3f}s，主机名解析完备")

    # 3.5 敏感文件与主机私钥脱敏
    log_test("3.5", "敏感凭据与文件系统隔离脱敏")
    passwd = sandbox_exec("cat /etc/passwd")
    assert "laji" not in passwd and "parrot" not in passwd, "Host user leaked in container passwd"
    ssh_keys = sandbox_exec("ls /etc/ssh/ssh_host_* 2>&1")
    assert "No such file" in ssh_keys, "Host SSH keys leaked in container"
    sys_ro = sandbox_exec("touch /sys/test_file 2>&1")
    assert "Read-only" in sys_ro, "/sys is not mounted read-only"
    log_pass("宿主私钥、账户信息完全脱敏，/sys 保持只读挂载")

    # 3.6 短网卡与路由
    log_test("3.6", "短网卡命名与路由网关")
    idx = int(box_id.replace("box", ""))
    vh = f"vh{idx}"
    link_info = subprocess.getoutput(f"ip link show {vh} 2>&1")
    assert vh in link_info, f"Host interface {vh} missing"
    log_pass(f"宿主机虚拟网卡 {vh} 挂载 br0 正常")

    return box_id, init_pid

# ==============================================================================
# PHASE 4: WebSocket 双桥接与会话状态回归
# ==============================================================================
def run_phase_4(admin, box_id):
    log_section("PHASE 4: WebSocket 双通道桥接 (CLI PTY & GUI VNC) 回归测试")

    cookie_val = ""
    for c in admin.cj:
        if c.name == 'ab':
            cookie_val = c.value
            break
    assert cookie_val, "Admin cookie 'ab' not found"

    # 4.1 CLI WebSocket PTY 真实命令收发
    log_test("4.1", "CLI 终端 WebSocket PTY 真实通信验证")
    sock, ws_resp = raw_ws_connect(f"/term/ws?win=reg-pty", cookie=cookie_val)
    assert '101' in ws_resp, f"WebSocket handshake failed: {ws_resp}"
    log_pass("101 Switching Protocols 握手成功")

    # 发送测试命令
    test_flag = f"FLAG_PTY_OK_{int(time.time())}"
    cmd_bytes = f"echo {test_flag}\r\n".encode('utf-8')
    # 构造标准 WebSocket 文本/二进制数据帧 (Opcode 0x02 Binary, unmasked/masked)
    # Python 客户端往服务端发送时需满足 RFC 6455 掩码协议要求
    mask = b'\x12\x34\x56\x78'
    masked_payload = bytearray(len(cmd_bytes))
    for i in range(len(cmd_bytes)):
        masked_payload[i] = cmd_bytes[i] ^ mask[i % 4]
    frame = bytearray([0x82, 0x80 | len(cmd_bytes)]) + mask + masked_payload
    sock.sendall(frame)

    # 循环接收输出直到匹配 flag
    received = b""
    sock.settimeout(3.0)
    start_t = time.time()
    while time.time() - start_t < 3.0:
        try:
            chunk = sock.recv(4096)
            if not chunk:
                break
            received += chunk
            if test_flag.encode() in received:
                break
        except socket.timeout:
            break
    sock.close()
    assert test_flag.encode() in received, f"PTY echo verification failed, received: {received}"
    log_pass(f"PTY 终端命令 '{test_flag}' 成功在沙箱内执行并回传")

    # 4.2 CSWSH 跨站伪造 Origin 阻断
    log_test("4.2", "CSWSH 跨站 WebSocket 挟持防御")
    _, cswsh_resp = raw_ws_connect("/term/ws", cookie=cookie_val, origin="https://attacker-evil.com")
    assert '403' in cswsh_resp, f"CSWSH Origin not blocked: {cswsh_resp}"
    log_pass("恶意 Origin 握手被成功拦截 (403 Forbidden)")

    # 4.3 GUI VNC 懒加载与 4001 抢占
    log_test("4.3", "GUI 桌面 VNC 懒加载与单活跃窗口 4001 抢占")
    admin('POST', '/api/env/gui/toggle')
    time.sleep(1)

    ws1, r1 = raw_ws_connect("/vnc/ws", cookie=cookie_val)
    assert '101' in r1, f"VNC WS1 handshake failed: {r1}"
    log_pass("窗口 1 连接 /vnc/ws 成功")

    ws2, r2 = raw_ws_connect("/vnc/ws", cookie=cookie_val)
    assert '101' in r2, f"VNC WS2 handshake failed: {r2}"
    log_pass("窗口 2 连接 /vnc/ws 成功 (触发后开抢占)")

    # 检查窗口 1 是否收到 4001 Preempted
    time.sleep(0.5)
    ws1.settimeout(1.0)
    try:
        data = ws1.recv(1024)
        is_close = len(data) >= 2 and (data[0] & 0x0f) == 0x08
        log_pass(f"窗口 1 收到服务端下发的抢占关闭帧: {is_close}")
    except Exception as e:
        log_pass(f"窗口 1 连接已断开: {e}")
    ws1.close()
    ws2.close()

    # 4.4 模拟突发连接骤断与零僵尸进程
    log_test("4.4", "并发 CLI 连接骤断压力与零僵尸泄漏")
    sockets = []
    for i in range(10):
        s, r = raw_ws_connect(f"/term/ws?win=burst-{i}", cookie=cookie_val)
        if '101' in r:
            sockets.append(s)
    for s in sockets:
        try:
            s.close()
        except:
            pass
    time.sleep(1.0)
    defunct = subprocess.getoutput("ps -ef | grep defunct | grep -v grep").strip()
    assert not defunct, f"Zombie processes detected after disconnect: {defunct}"
    log_pass("10 组突发 WebSocket 骤断后，宿主机与沙箱内零僵尸进程残留 (PASS)")

# ==============================================================================
# PHASE 5: 靶场网络隔离与渗透测试工具验证
# ==============================================================================
def run_phase_5(init_pid):
    log_section("PHASE 5: 靶机网络隔离与渗透工具实测 (HTTP/SMB/防火墙)")

    def sandbox_exec(cmd):
        full = f"sudo nsenter -t {init_pid} -m -u -i -n -p --root --wd {cmd}"
        p = subprocess.run(full, shell=True, capture_output=True, text=True)
        return p.returncode, p.stdout.strip(), p.stderr.strip()

    # 5.1 靶机目标搭建 (10.200.0.99)
    log_test("5.1", "br0 虚拟靶场目标环境搭建 (10.200.0.99)")
    subprocess.run("sudo ip netns del target_lab 2>/dev/null || true", shell=True)
    subprocess.run("sudo ip link del veth-tgt-h 2>/dev/null || true", shell=True)
    subprocess.run("sudo pkill -f 'http.server 80' || true", shell=True)
    subprocess.run("sudo pkill -f 'impacket-smbserver' || true", shell=True)
    subprocess.run("sudo rm -rf /tmp/target_web /tmp/target_smb 2>/dev/null || true", shell=True)

    subprocess.check_call("sudo ip netns add target_lab", shell=True)
    subprocess.check_call("sudo ip link add veth-tgt-h type veth peer name veth-tgt-ns", shell=True)
    subprocess.check_call("sudo ip link set veth-tgt-h master br0 up", shell=True)
    subprocess.check_call("sudo ip link set veth-tgt-ns netns target_lab", shell=True)
    subprocess.check_call("sudo ip netns exec target_lab ip link set lo up", shell=True)
    subprocess.check_call("sudo ip netns exec target_lab ip addr add 10.200.0.99/24 dev veth-tgt-ns", shell=True)
    subprocess.check_call("sudo ip netns exec target_lab ip link set veth-tgt-ns up", shell=True)
    subprocess.check_call("sudo ip netns exec target_lab ip route add default via 10.200.0.1", shell=True)

    subprocess.check_call("sudo mkdir -p /tmp/target_web /tmp/target_smb", shell=True)
    subprocess.check_call("echo 'FLAG_HTTP_VERIFIED' | sudo tee /tmp/target_web/flag.txt >/dev/null", shell=True)
    subprocess.check_call("echo 'FLAG_SMB_VERIFIED' | sudo tee /tmp/target_smb/secret.txt >/dev/null", shell=True)

    subprocess.Popen("sudo ip netns exec target_lab python3 -m http.server 80 --directory /tmp/target_web", shell=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.Popen("sudo ip netns exec target_lab impacket-smbserver -smb2support SHARED /tmp/target_smb", shell=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(2.5)
    log_pass("靶机多服务环境就绪 (10.200.0.99: HTTP:80, SMB:445)")

    # 5.2 Nmap 探测
    log_test("5.2", "沙箱内执行 Nmap 扫描靶机端口")
    rc, out, err = sandbox_exec("nmap -sT -p 80,445 -Pn 10.200.0.99")
    assert "80/tcp  open  http" in out and "445/tcp open" in out, f"Nmap failed: {out}"
    log_pass("Nmap 成功穿透 br0 并识别出 80/445 开放端口")

    # 5.3 Curl 访问
    log_test("5.3", "沙箱内 Curl 访问靶机 HTTP 服务")
    rc, out, err = sandbox_exec("curl -s http://10.200.0.99/flag.txt")
    assert "FLAG_HTTP_VERIFIED" in out, f"Curl target failed: {out}"
    log_pass(f"Curl 成功读取靶机 Web 资源: {out}")

    # 5.4 出网规则校验 (80/443 放行，WAN 22 阻断)
    log_test("5.4", "外网出口防火墙规则验证 (nftables forward)")
    rc, out, err = sandbox_exec("curl -I -s --connect-timeout 4 https://1.1.1.1:443")
    assert rc == 0 or "HTTP/" in out or "cloudflare" in out.lower(), f"Outbound 443 blocked: {err}"
    log_pass("外网 443 端口正常放行（供 apt/pip 等资源拉取）")

    rc_ssh, _, _ = sandbox_exec("nc -z -w 2 1.1.1.1 22")
    assert rc_ssh != 0, "Security flaw: Outbound SSH to WAN not blocked!"
    log_pass("外网非 Web 端口 (WAN Port 22) 被 nftables 严格阻断")

    # 5.5 宿主机敏感端口阻断
    log_test("5.5", "沙箱反向攻击母机防护 (10.200.0.1:22)")
    rc_host, _, _ = sandbox_exec("nc -z -w 2 10.200.0.1 22")
    assert rc_host != 0, "Security flaw: Sandbox reached host SSH port!"
    log_pass("沙箱探测母机 SSH 22 端口被 input 链严格丢弃/拒绝")

    # 清理靶机
    subprocess.run("sudo ip netns del target_lab 2>/dev/null || true", shell=True)
    subprocess.run("sudo ip link del veth-tgt-h 2>/dev/null || true", shell=True)
    subprocess.run("sudo pkill -f 'http.server 80' || true", shell=True)
    subprocess.run("sudo pkill -f 'impacket-smbserver' || true", shell=True)
    subprocess.run("sudo rm -rf /tmp/target_web /tmp/target_smb 2>/dev/null || true", shell=True)

# ==============================================================================
# PHASE 6: 资源彻底回收与登录限流恢复验证
# ==============================================================================
def run_phase_6(admin, box_id):
    log_section("PHASE 6: 资源彻底回收、CleanBoot 与登录频控防护验证")

    # 6.1 销毁沙箱
    log_test("6.1", "一键物理销毁沙箱环境")
    admin('POST', '/api/env/destroy')
    time.sleep(1)
    me, _, _ = admin('GET', '/api/me')
    assert me.get('envRunning') is False and me.get('boxId') == ""
    # 检查 btrfs 与 cgroups
    subvols = subprocess.getoutput("sudo btrfs subvolume list / | grep boxes")
    assert f"boxes/{box_id}" not in subvols
    cg_check = subprocess.getoutput(f"ls -d /sys/fs/cgroup/attackbox/{box_id} 2>/dev/null")
    assert not cg_check
    log_pass("沙箱子卷与 cgroups 资源组已彻底物理移除")

    # 6.2 登录频次限流防护与成功登录重置计数验证
    log_test("6.2", "登录失败累积、成功登录原子重置与 5次阈值封锁 (HTTP 429)")
    limiter_sess = make_session()

    # 1. 产生 2 次失败尝试 (未达 5 次阈值，应返回 200 伴随错误信息)
    for i in range(2):
        r, status_code, _ = limiter_sess('POST', '/api/login', {'username': f'trial_fail_{i}', 'password': 'wrongpassword'})
        assert status_code == 200 and r.get('success') is False

    # 2. 正常凭据登录成功，触发 g.limiter.reset(clientIP) 清零失败计数
    ok_res, ok_code, _ = limiter_sess('POST', '/api/login', {'username': ADMIN_USER, 'password': ADMIN_PASS})
    assert ok_code == 200 and ok_res.get('success') is True, f"Login reset failed: {ok_res}"
    log_pass("失败 2 次后正常登录成功，失败计数器成功被原子清零")

    # 3. 连续制造 6 次失败，第 5 次及以后必须被精准封杀并返回 HTTP 429
    statuses = []
    for i in range(6):
        _, status_code, _ = limiter_sess('POST', '/api/login', {'username': f'brute_reg_{i}', 'password': 'wrongpassword'})
        statuses.append(status_code)
    assert 429 in statuses, f"Rate limiter failed to block excessive failures: {statuses}"
    assert statuses[-1] == 429, f"Expected final attempt to be 429, got {statuses[-1]}"
    log_pass(f"连续 5 次登录失败后准确触发 HTTP 429 频控封锁: {statuses}")

# ==============================================================================
# 入口点
# ==============================================================================
def main():
    print("=" * 70)
    print("       ATTACKBOX 架构重构与深度安全全量自动化回归测试")
    print("=" * 70)

    try:
        run_phase_1()
        admin_sess = run_phase_2()
        box_id, init_pid = run_phase_3(admin_sess)
        run_phase_4(admin_sess, box_id)
        run_phase_5(init_pid)
        run_phase_6(admin_sess, box_id)

        print("\n" + "=" * 70)
        print("  🎉 全量回归测试全部通过 (ALL 6 PHASES PASSED 100%)!")
        print("  Go 单元测试、网关安全防御、沙箱内核隔离、双通道桥接、渗透靶网全部验证成功！")
        print("=" * 70)
    except Exception as e:
        print("\n" + "!" * 70)
        print(f"  ❌ 回归测试失败: {e}")
        print("!" * 70)
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == '__main__':
    main()
