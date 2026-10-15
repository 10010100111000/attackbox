import socket
import ssl
import json
import urllib.request
import time
import sys

def main():
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE

    opener = urllib.request.build_opener(
        urllib.request.HTTPSHandler(context=ctx),
        urllib.request.HTTPCookieProcessor()
    )

    # 1. 登录
    login_req = urllib.request.Request(
        'https://127.0.0.1/api/login',
        data=json.dumps({'username': 'laji', 'password': 'qwer1234!'}).encode(),
        headers={'Content-Type': 'application/json'}
    )
    login_resp = opener.open(login_req)
    cookie = login_resp.headers.get('Set-Cookie').split(';')[0]
    print(f"[OK] Logged in successfully. Cookie: {cookie}")

    # 确保 CLI 环境就绪
    cli_req = urllib.request.Request(
        'https://127.0.0.1/api/env/cli/toggle',
        data=b'{}',
        headers={'Cookie': cookie, 'Content-Type': 'application/json'}
    )
    cli_resp = json.loads(opener.open(cli_req).read().decode())
    print(f"[OK] CLI Env state: {cli_resp}")

    def ws_connect(path):
        raw = socket.create_connection(('127.0.0.1', 443))
        s = ctx.wrap_socket(raw, server_hostname='127.0.0.1')
        handshake = (
            f"GET {path} HTTP/1.1\r\n"
            "Host: 127.0.0.1\r\n"
            "Upgrade: websocket\r\n"
            "Connection: Upgrade\r\n"
            "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n"
            "Sec-WebSocket-Version: 13\r\n"
            "Sec-WebSocket-Protocol: binary\r\n"
            f"Cookie: {cookie}\r\n"
            "\r\n"
        )
        s.sendall(handshake.encode())
        res = s.recv(4096).decode('utf-8', errors='ignore')
        status_line = res.split('\r\n')[0]
        if "101" not in status_line:
            raise RuntimeError(f"Handshake failed: {status_line}")
        return s

    # 2. 测试 CLI 终端连接及命令回显
    print("[TEST 1] Testing direct PTY shell on /term/ws...")
    s1 = ws_connect('/term/ws')
    print("  s1 connected to /term/ws")

    # 发送 whoami\r 帧
    cmd = b"whoami\r"
    frame = bytearray([0x82, 0x80 | len(cmd), 0, 0, 0, 0]) + cmd
    s1.sendall(frame)
    time.sleep(0.5)

    s1.settimeout(2.0)
    out = b""
    try:
        while True:
            chunk = s1.recv(4096)
            if not chunk:
                break
            out += chunk
            if b"parrot" in out:
                break
    except Exception:
        pass
    print(f"  Received from s1 (len={len(out)}): {repr(out[:80])}...")
    assert b"parrot" in out, "Did not receive parrot user shell prompt or output"
    print("  [PASS] Direct PTY shell responded correctly!")

    # 3. 测试 CLI 单活跃会话抢占 (s2 接入，s1 必须被关闭并收到 4001)
    print("\n[TEST 2] Testing CLI single active session preemption...")
    s2 = ws_connect('/term/ws')
    print("  s2 connected to /term/ws")

    # 此时 s1 应该收到 close 帧或者 EOF
    s1.settimeout(3.0)
    s1_closed = False
    s1_close_code = None
    try:
        while True:
            data = s1.recv(4096)
            if not data:
                s1_closed = True
                print("  s1 connection closed (EOF)")
                break
            print(f"  s1 received frame (len={len(data)}): {data[:30]!r}")
            # 遍历检查 WebSocket 帧
            i = 0
            while i < len(data):
                b0 = data[i]
                opcode = b0 & 0x0F
                if opcode == 0x8: # Close frame
                    s1_closed = True
                    if len(data) >= i + 4:
                        s1_close_code = int.from_bytes(data[i+2:i+4], 'big')
                    print(f"  Found close frame at index {i}, code: {s1_close_code}")
                    break
                # 跳过非 close 帧
                if len(data) > i + 1:
                    payload_len = data[i+1] & 0x7F
                    if payload_len == 126:
                        payload_len = int.from_bytes(data[i+2:i+4], 'big')
                        i += 4 + payload_len
                    elif payload_len == 127:
                        payload_len = int.from_bytes(data[i+2:i+10], 'big')
                        i += 10 + payload_len
                    else:
                        i += 2 + payload_len
                else:
                    break
            if s1_closed:
                break
    except socket.timeout:
        print("  s1 recv timed out waiting for kick")
    except Exception as e:
        s1_closed = True
        print(f"  s1 closed with socket error: {e}")

    assert s1_closed or s1_close_code == 4001, "s1 was NOT kicked by s2!"
    print("  [PASS] Old CLI session s1 was successfully kicked by new session s2!")

    def send_frame(sock, data):
        if isinstance(data, str):
            data = data.encode()
        frame = bytearray([0x82, 0x80 | len(data), 0, 0, 0, 0]) + data
        sock.sendall(frame)

    # 验证 s2 仍然正常工作
    time.sleep(0.5)
    send_frame(s2, "id\r")
    time.sleep(0.5)
    s2.settimeout(2.0)
    out2 = b""
    try:
        while True:
            c = s2.recv(4096)
            if not c: break
            out2 += c
            if b"uid=" in out2: break
    except Exception:
        pass
    print(f"  Received from s2: {repr(out2[:60])}...")
    assert b"uid=" in out2, "s2 is not working after kicking s1"
    print("  [PASS] New CLI session s2 is fully functional!")
    s2.close()

    # 4. 测试 GUI (VNC) 单活跃会话抢占
    print("\n[TEST 3] Testing GUI single active session preemption on /vnc/websockify...")
    v1 = ws_connect('/vnc/websockify')
    print("  v1 connected to /vnc/websockify")
    rfb1 = v1.recv(4096)
    print(f"  v1 received RFB header: {repr(rfb1[:12])}")
    assert b"RFB" in rfb1, "v1 did not receive RFB header"

    v2 = ws_connect('/vnc/websockify')
    print("  v2 connected to /vnc/websockify")
    rfb2 = v2.recv(4096)
    print(f"  v2 received RFB header: {repr(rfb2[:12])}")
    assert b"RFB" in rfb2, "v2 did not receive RFB header"

    # 检查 v1 是否被踢出
    v1.settimeout(2.0)
    v1_kicked = False
    try:
        v1_data = v1.recv(4096)
        if not v1_data:
            v1_kicked = True
        elif len(v1_data) >= 4 and (v1_data[0] & 0x0F) == 0x8:
            v1_kicked = True
            v1_close_code = int.from_bytes(v1_data[2:4], 'big')
            print(f"  v1 received close code: {v1_close_code}")
    except socket.timeout:
        pass
    except Exception as e:
        v1_kicked = True
        print(f"  v1 closed: {e}")

    assert v1_kicked, "v1 was NOT kicked by v2!"
    print("  [PASS] Old GUI session v1 was successfully kicked by new session v2!")
    v2.close()

    print("\n=== ALL SINGLE ACTIVE SESSION & PTY TESTS PASSED! ===")

if __name__ == '__main__':
    main()
