import urllib.request
import json
import ssl
import time

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

opener = urllib.request.build_opener(
    urllib.request.HTTPSHandler(context=ctx),
    urllib.request.HTTPCookieProcessor()
)

# 1. 登录验证
req = urllib.request.Request(
    'https://127.0.0.1/api/login',
    data=json.dumps({'username': 'laji', 'password': 'qwer1234!'}).encode(),
    headers={'Content-Type': 'application/json'}
)
resp = opener.open(req)
print('Login status:', resp.status, resp.read().decode())

# 2. 测量启动沙箱时间 (方案 A Core 毫秒级)
t0 = time.time()
req2 = urllib.request.Request('https://127.0.0.1/api/env/toggle', data=b'', headers={'Content-Type': 'application/json'})
resp2 = opener.open(req2)
dur = time.time() - t0
print(f'Toggle start (Core): {resp2.status} in {dur:.3f}s, body: {resp2.read().decode()}')

# 3. 验证 /term
req3 = urllib.request.Request('https://127.0.0.1/term')
resp3 = opener.open(req3)
print('Term page HTTP:', resp3.status)

# 4. 测量关闭释放沙箱时间
t1 = time.time()
resp4 = opener.open(req2)
dur4 = time.time() - t1
print(f'Toggle stop: {resp4.status} in {dur4:.3f}s, body: {resp4.read().decode()}')
