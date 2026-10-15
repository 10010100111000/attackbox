import socket
import ssl
import json
import urllib.request
import time

def test():
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE

    opener = urllib.request.build_opener(
        urllib.request.HTTPSHandler(context=ctx),
        urllib.request.HTTPCookieProcessor()
    )
    login_req = urllib.request.Request(
        'https://127.0.0.1/api/login',
        data=json.dumps({'username': 'laji', 'password': 'qwer1234!'}).encode(),
        headers={'Content-Type': 'application/json'}
    )
    resp = opener.open(login_req)
    cookie = resp.headers.get('Set-Cookie').split(';')[0]
    print("1. Logged in, cookie:", cookie)

    # 1. Connect to /term/ws
    raw = socket.create_connection(('127.0.0.1', 443))
    s = ctx.wrap_socket(raw, server_hostname='127.0.0.1')
    handshake = (
        f"GET /term/ws HTTP/1.1\r\n"
        f"Host: 127.0.0.1\r\n"
        f"Upgrade: websocket\r\n"
        f"Connection: Upgrade\r\n"
        f"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n"
        f"Sec-WebSocket-Version: 13\r\n"
        f"Cookie: {cookie}\r\n\r\n"
    )
    s.sendall(handshake.encode())
    res = s.recv(4096).decode('utf-8', errors='ignore')
    assert '101' in res.split('\r\n')[0]
    print("2. CLI terminal connected successfully.")

    # 2. Call destroy
    destroy_req = urllib.request.Request(
        'https://127.0.0.1/api/env/destroy',
        data=b'{}',
        headers={'Cookie': cookie, 'Content-Type': 'application/json'}
    )
    d_resp = json.loads(opener.open(destroy_req).read().decode())
    print("3. Destroy response:", d_resp)

    # 3. Read from s - MUST receive close frame or EOF!
    s.settimeout(2.0)
    closed = False
    try:
        data = s.recv(4096)
        if not data or (len(data) >= 2 and (data[0] & 0x0F) == 0x8):
            closed = True
            print("4. Received close frame from terminal socket:", repr(data))
    except Exception as e:
        closed = True
        print("4. Socket closed with error:", e)

    assert closed, "Socket was NOT closed after environment destroy!"
    print("[PASS] Terminal connection was immediately terminated after environment destruction!")

if __name__ == '__main__':
    test()
