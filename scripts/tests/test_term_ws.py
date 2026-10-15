import socket
import ssl
import json
import urllib.request
import time

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

opener = urllib.request.build_opener(
    urllib.request.HTTPSHandler(context=ctx),
    urllib.request.HTTPCookieProcessor()
)
req = urllib.request.Request(
    'https://127.0.0.1/api/login',
    data=json.dumps({'username': 'laji', 'password': 'qwer1234!'}).encode(),
    headers={'Content-Type': 'application/json'}
)
resp = opener.open(req)
cookie = resp.headers.get('Set-Cookie').split(';')[0]
print('Logged in, cookie:', cookie)

raw_sock = socket.create_connection(('127.0.0.1', 443))
s = ctx.wrap_socket(raw_sock, server_hostname='127.0.0.1')

handshake = (
    "GET /term/ws HTTP/1.1\r\n"
    "Host: 127.0.0.1\r\n"
    "Upgrade: websocket\r\n"
    "Connection: Upgrade\r\n"
    "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n"
    "Sec-WebSocket-Version: 13\r\n"
    f"Cookie: {cookie}\r\n"
    "\r\n"
)
s.sendall(handshake.encode())
res = s.recv(4096).decode('utf-8', errors='ignore')
print('Handshake result:', res.split('\r\n')[0])

def send_ws(cmd_str):
    data = cmd_str.encode()
    # 0x82 for binary frame, mask bit set (0x80)
    frame = bytearray([0x82, 0x80 | len(data), 0, 0, 0, 0])
    frame.extend(data)
    s.sendall(frame)

# 发送 whoami\r
send_ws("whoami\r")
time.sleep(0.5)

out = b""
s.settimeout(2.0)
try:
    while True:
        chunk = s.recv(4096)
        if not chunk:
            break
        out += chunk
except Exception:
    pass

print(f"Received total {len(out)} bytes from terminal:")
print(repr(out))
s.close()
