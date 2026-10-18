import urllib.request
import http.cookiejar
import ssl
import json
import socket

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

BASE_URL = 'https://77.67.1.106'

def test():
    cj = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(
        urllib.request.HTTPCookieProcessor(cj),
        urllib.request.HTTPSHandler(context=ctx)
    )

    # 1. 登录
    login_data = json.dumps({'username': 'laji', 'password': 'qwer1234!'}).encode('utf-8')
    req = urllib.request.Request(f'{BASE_URL}/api/login', data=login_data, headers={'Content-Type': 'application/json'})
    with opener.open(req) as resp:
        res = json.loads(resp.read().decode('utf-8'))
        assert res.get('success'), f"Login failed: {res}"
        print("1. Admin 登录成功")

    # 2. 访问 /term
    req_term = urllib.request.Request(f'{BASE_URL}/term')
    with opener.open(req_term) as resp:
        html = resp.read().decode('utf-8')
        assert resp.status == 200, f"Expected 200, got {resp.status}"
        assert '/xterm/xterm.js' in html, "xterm.js not loaded in /term"
        assert 'assets/index-' in html, "Vue SPA bundle not served for /term"
        print("2. /term 页面正确返回 Vue SPA 产物并包含 xterm 同步脚本")

    # 3. 访问 /gui
    req_gui = urllib.request.Request(f'{BASE_URL}/gui')
    with opener.open(req_gui) as resp:
        html = resp.read().decode('utf-8')
        assert resp.status == 200, f"Expected 200, got {resp.status}"
        assert 'assets/index-' in html, "Vue SPA bundle not served for /gui"
        print("3. /gui 页面正确返回 Vue SPA 产物")

    print("\nALL FRONTEND ENDPOINT VERIFICATIONS PASSED!")

if __name__ == '__main__':
    test()
