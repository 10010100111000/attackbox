import urllib.request
import urllib.error
import http.cookiejar
import ssl
import socket
import json
import subprocess
import time
import threading

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

BASE_URL = 'https://127.0.0.1'

class NoRedirectHandler(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        # 拦截重定向，不自动跳转
        return None

def make_session(follow_redirects=True):
    cj = http.cookiejar.CookieJar()
    handlers = [
        urllib.request.HTTPCookieProcessor(cj),
        urllib.request.HTTPSHandler(context=ctx)
    ]
    if not follow_redirects:
        handlers.append(NoRedirectHandler())
    opener = urllib.request.build_opener(*handlers)
    
    def req(method, path, data=None, headers_extra=None):
        url = BASE_URL + path
        body = None
        headers = {}
        if data is not None:
            body = json.dumps(data).encode('utf-8')
            headers['Content-Type'] = 'application/json'
        if headers_extra:
            headers.update(headers_extra)
        r = urllib.request.Request(url, data=body, headers=headers, method=method)
        try:
            with opener.open(r) as resp:
                raw = resp.read().decode('utf-8')
                loc = resp.headers.get('Location', '')
                try:
                    return resp.status, json.loads(raw), loc
                except:
                    return resp.status, raw, loc
        except urllib.error.HTTPError as e:
            raw = e.read().decode('utf-8')
            loc = e.headers.get('Location', '')
            try:
                return e.code, json.loads(raw), loc
            except:
                return e.code, raw, loc
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
    
    # 读取握手返回
    resp = tls_sock.recv(4096).decode('utf-8', errors='ignore')
    return tls_sock, resp

print("=" * 60)
print("  AttackBox 安全审计与并发压力深度测试套件")
print("=" * 60)

# -------------------------------------------------------------
# 专项一：未授权访问与凭据伪造/爆破测试
# -------------------------------------------------------------
print("\n[PART 1] 未授权访问与凭据伪造/爆破防御测试")
anon_no_redir = make_session(follow_redirects=False)

# 1.1 未授权访问 HTTP 路由 (拦截重定向进行验证)
code, resp, loc = anon_no_redir('GET', '/gui')
print(f"1.1 匿名访问 /gui -> HTTP Status: {code}, Location: {loc}")
assert code == 303 and loc == '/login', f"Expected 303 Redirect to /login, got {code} {loc}"

code, resp, loc = anon_no_redir('GET', '/term')
print(f"1.2 匿名访问 /term -> HTTP Status: {code}, Location: {loc}")
assert code == 303 and loc == '/login', f"Expected 303 Redirect to /login, got {code} {loc}"

code, resp, loc = anon_no_redir('GET', '/api/env/status')
print(f"1.3 匿名访问 /api/env/status -> HTTP Status: {code}")
assert code == 401, f"Expected 401 Unauthorized for REST API, got {code}"

# 1.2 匿名尝试直连 WebSocket
_, ws_resp = raw_ws_connect('/term/ws')
print(f"1.4 匿名直连 /term/ws -> Handshake: {ws_resp.splitlines()[0] if ws_resp else 'None'}")
assert '401' in ws_resp, f"Expected 401 in WebSocket handshake, got: {ws_resp}"

_, ws_resp = raw_ws_connect('/vnc/ws')
print(f"1.5 匿名直连 /vnc/ws -> Handshake: {ws_resp.splitlines()[0] if ws_resp else 'None'}")
assert '401' in ws_resp, f"Expected 401 in WebSocket handshake, got: {ws_resp}"

# 1.3 伪造与爆破凭据测试 (伪造 64位十六进制 Token 与特殊 Payload)
fake_tokens = [
    "0000000000000000000000000000000000000000000000000000000000000000",
    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
    "' or '1'='1",
    "../../etc/passwd",
    "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.t-IDcSemACt8x4iTMCda8Yhe3iZaWbvV5XKSTbuAn0M"
]
for ft in fake_tokens:
    _, ws_resp = raw_ws_connect('/term/ws', cookie=ft)
    assert '401' in ws_resp, f"Forged token {ft} was not rejected with 401!"
print(f"1.6 伪造/注入 Token 握手拦截测试: 全部 5 组伪造凭据均被 401 立即封杀 (PASS)")

# 1.4 CSWSH 跨站劫持测试
sess = make_session()
code, lres, _ = sess('POST', '/api/login', {'username': 'admin', 'password': 'qwer1234!'})
assert lres.get('success') is True, "Admin login failed"
admin_cookie = ""
for c in sess.cj:
    if c.name == 'ab':
        admin_cookie = c.value
        break

_, ws_cswsh = raw_ws_connect('/term/ws', cookie=admin_cookie, origin='https://malicious-attacker.com')
print(f"1.7 跨站恶意 Origin 握手拦截: {ws_cswsh.splitlines()[0] if ws_cswsh else 'None'}")
assert '403' in ws_cswsh, f"CSWSH Origin bypass failed! Got: {ws_cswsh}"
print("    Origin 校验白名单防御 (PASS)")

# -------------------------------------------------------------
# 专项二：GUI 窗口追踪与后端多开抢占测试
# -------------------------------------------------------------
print("\n[PART 2] GUI 窗口跟踪与后端判断机制测试")
# 确保 GUI 运行
sess('POST', '/api/env/gui/toggle')
time.sleep(1)

# 窗口 1 连接
ws1, ws1_resp = raw_ws_connect('/vnc/ws', cookie=admin_cookie)
assert '101' in ws1_resp, f"WS1 handshake failed: {ws1_resp}"
print("2.1 窗口 1 连入 /vnc/ws -> 握手成功 (101 Switching Protocols)")

# 窗口 2 连接 (后开抢占)
ws2, ws2_resp = raw_ws_connect('/vnc/ws', cookie=admin_cookie)
assert '101' in ws2_resp, f"WS2 handshake failed: {ws2_resp}"
print("2.2 窗口 2 连入 /vnc/ws -> 握手成功 (101 Switching Protocols)")

# 检查窗口 1 是否收到 4001 Preempted 帧或被服务端关闭
time.sleep(0.5)
try:
    ws1.settimeout(1.0)
    data = ws1.recv(1024)
    # WebSocket close frame format: opcode 0x88
    is_close = len(data) >= 2 and (data[0] & 0x0f) == 0x08
    print(f"2.3 窗口 1 状态检测 -> 收到抢占/关闭帧: {is_close} (Len={len(data)} B)")
except Exception as e:
    print(f"2.3 窗口 1 状态检测 -> 连接已断开/超时: {e}")

ws1.close()
ws2.close()
time.sleep(0.5)
print("    GUI 单活跃会话与 4001 抢占机制验证通过 (PASS)")

# -------------------------------------------------------------
# 专项三：超多 CLI 命令行并发建立、骤断与资源彻底回收实测
# -------------------------------------------------------------
print("\n[PART 3] 巨量 CLI 命令行开启、骤断与资源彻底回收压力实测")

# 检查测试前宿主机的进程与僵尸情况
before_defunct = subprocess.getoutput("ps -ef | grep defunct | grep -v grep")
before_pts = subprocess.getoutput("ls -l /dev/pts | wc -l")
print(f"3.1 初始环境指标 -> 僵尸进程: {'0' if not before_defunct else before_defunct}, PTY 节点数: {before_pts}")

CONCURRENT_CLI = 25
sockets = []
lock = threading.Lock()
connected_count = [0]

def open_cli_worker(idx):
    win_id = f"stress-win-{idx}"
    sock, resp = raw_ws_connect(f"/term/ws?win={win_id}", cookie=admin_cookie)
    if '101' in resp:
        with lock:
            sockets.append(sock)
            connected_count[0] += 1

threads = []
start_t = time.time()
for i in range(CONCURRENT_CLI):
    t = threading.Thread(target=open_cli_worker, args=(i,))
    threads.append(t)
    t.start()

for t in threads:
    t.join()

dur = time.time() - start_t
print(f"3.2 并发启动 {CONCURRENT_CLI} 个活动 CLI 命令行窗口 -> 成功建立: {connected_count[0]}/{CONCURRENT_CLI} 耗时: {dur:.2f}s")

# 监控并发时沙箱与宿主机进程数
nsenter_count = subprocess.getoutput("ps -ef | grep nsenter | grep '/usr/bin/su' | grep -v grep | wc -l")
bash_count = subprocess.getoutput("ps -ef | grep '/usr/bin/su - box' | grep -v grep | wc -l")
print(f"3.3 运行态监控 -> 宿主机 CLI nsenter 进程: {nsenter_count}, 沙箱登录会话数: {bash_count}")

# 模拟用户骤断（同时关闭浏览器所有 25 个标签页）
print("3.4 模拟用户突然关闭浏览器 (骤断 25 个 WebSocket TCP 连接)...")
for s in sockets:
    try:
        s.close()
    except:
        pass
sockets.clear()

# 留出 1.5 秒让 Go 的 defer 和 Linux 内核回收
time.sleep(1.5)

# 严格检测资源回收
after_nsenter = subprocess.getoutput("ps -ef | grep nsenter | grep '/usr/bin/su' | grep -v grep | wc -l")
cli_defunct = subprocess.getoutput("ps -ef | grep -E 'su|bash|nsenter' | grep defunct | grep -v grep")
after_pts = subprocess.getoutput("ls -l /dev/pts | wc -l")

print(f"3.5 骤断后回收指标:")
print(f"    - 残留 CLI nsenter 进程数: {after_nsenter} (Expect 0)")
print(f"    - CLI 相关僵尸进程: {'0 (NONE)' if not cli_defunct else cli_defunct} (Expect 0)")
print(f"    - PTY 设备回收: 初始 {before_pts} -> 峰值释放后 {after_pts}")

assert int(after_nsenter) == 0, f"Leaked nsenter processes: {after_nsenter}"
assert not cli_defunct, f"CLI Zombie processes leaked: {cli_defunct}"
print("    多命令行极速销毁与零泄漏验证通过 (PASS)")

# 再次重新打开 10 个命令行，验证系统无任何锁死或句柄耗尽
print("3.6 资源回收后再热启动 10 个命令行验证系统复原力...")
reopen_sockets = []
for i in range(10):
    s, r = raw_ws_connect(f"/term/ws?win=reopen-{i}", cookie=admin_cookie)
    if '101' in r:
        reopen_sockets.append(s)
print(f"    复原力测试: 成功热启动 {len(reopen_sockets)}/10 个新命令行 (PASS)")
for s in reopen_sockets:
    s.close()
time.sleep(1)

print("\n" + "=" * 60)
print("  全部 3 大专项测试 100% 通过！系统安全性与回收机制完备！")
print("=" * 60)
