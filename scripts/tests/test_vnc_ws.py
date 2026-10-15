#!/usr/bin/env python3
import socket
import ssl
import json
import urllib.request
import time

def main():
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE

    # 1. 登录
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

    # 2. 测试直接连接 unix socket
    print('\n2. 测试直接连接 gui.sock...')
    try:
        usock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        usock.connect('/run/attackbox/box1/gui.sock')
        banner = usock.recv(12)
        print('Direct gui.sock RFB Banner:', repr(banner))
        usock.close()
    except Exception as e:
        print('Direct gui.sock connection error:', e)

    # 3. 测试通过网关 /vnc/websockify
    print('\n3. 测试通过网关 /vnc/websockify...')
    raw_sock = socket.create_connection(('127.0.0.1', 443))
    s = ctx.wrap_socket(raw_sock, server_hostname='127.0.0.1')

    handshake = (
        "GET /vnc/websockify HTTP/1.1\r\n"
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
    res = s.recv(4096)
    print('WS Handshake response:', repr(res[:200]))

    # 等待 RFB 版本号
    s.settimeout(3.0)
    try:
        rfb_data = s.recv(4096)
        print('Received from RFB via WS:', repr(rfb_data))
    except Exception as e:
        print('Recv error:', e)

if __name__ == '__main__':
    main()
