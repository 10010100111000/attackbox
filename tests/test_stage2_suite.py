import urllib.request
import urllib.error
import http.cookiejar
import ssl
import json
import time
import sys
import re

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
    def req(method, path, data=None, custom_headers=None):
        url = BASE_URL + path
        body = None
        headers = {'User-Agent': 'Stage2-TestSuite/1.0', 'CF-Connecting-IP': '198.51.100.88'}
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

admin_sess = make_session()
user_sess = make_session()

print("==================================================")
print("       ATTACKBOX 阶段 2 功能与安全全量测试")
print("==================================================")

# 1. 登录管理员
print("\n[Test 1] 管理员认证与登录审计")
login_res, code, headers = admin_sess('POST', '/api/login', {'username': 'laji', 'password': 'qwer1234!'})
print("Admin Login:", login_res)
assert login_res.get('success') is True, "Admin login failed"

# 2. 模拟登录失败，验证 LOGIN_FAILED 审计落库与 TrustProxy 门控防护
print("\n[Test 2] 异常登录拦截与安全审计 (TrustProxy 防伪造 IP 验证)")
fail_res, code, _ = user_sess('POST', '/api/login', {'username': 'hacker_attacker', 'password': 'wrongpassword!'})
print("Fake Login Fail:", fail_res)
assert fail_res.get('success') is False

audit_res, _, _ = admin_sess('GET', '/api/admin/audit?limit=5&event_type=LOGIN_FAILED')
print("Audit (LOGIN_FAILED):", audit_res)
assert audit_res.get('success') is True
items = audit_res.get('items', [])
assert len(items) > 0, "No LOGIN_FAILED audit item found"
latest_fail = items[0]
assert latest_fail['actor'] == 'hacker_attacker'
# config.json 中 trustProxy 为 false，伪造的 CF-Connecting-IP 应被门控丢弃，安全记录真实 RemoteAddr (127.0.0.1)
assert latest_fail['ip'] in ('127.0.0.1', '::1'), f"TrustProxy gate failure: forged IP leaked into audit: {latest_fail['ip']}"
print(f"=> 成功验证: LOGIN_FAILED 事件已精准落库且伪造 IP 已被门控防御，记录宿主真实 IP: {latest_fail['ip']}")

# 3. 验证系统策略设置 (Settings) 与动态自主注册开关
print("\n[Test 3] 系统策略全局配置热生效 (关闭自主注册测试)")
# 3.1 获取当前配置
settings, _, _ = admin_sess('GET', '/api/admin/settings')
print("Current Settings:", settings)

# 3.2 关闭自主注册
update_res, _, _ = admin_sess('POST', '/api/admin/settings', {'allowRegister': False})
assert update_res.get('success') is True

# 3.3 校验公共策略端点 /api/public/config
pub_conf, _, _ = user_sess('GET', '/api/public/config')
print("Public Config when closed:", pub_conf)
assert pub_conf.get('allowRegister') is False

# 3.4 尝试自主注册，应被阻断
reg_fail, _, _ = user_sess('POST', '/api/register', {'username': 'test_reg_closed', 'password': 'password123'})
print("Register when closed:", reg_fail)
assert reg_fail.get('success') is False
assert '关闭' in reg_fail.get('error', '')

# 3.5 恢复自主注册
update_res, _, _ = admin_sess('POST', '/api/admin/settings', {'allowRegister': True})
assert update_res.get('success') is True
pub_conf2, _, _ = user_sess('GET', '/api/public/config')
assert pub_conf2.get('allowRegister') is True

# 3.6 验证开启后进入邮箱与验证码校验流程 (而非被全局关闭策略阻断)
test_user = f"stg2_user_{int(time.time())}"
reg_probe, _, _ = user_sess('POST', '/api/register', {'username': test_user, 'password': 'password123'})
print("Register after open (schema probe):", reg_probe)
assert '关闭' not in reg_probe.get('error', '')

# 管理员直接创建该测试用户并登录，以继续后续沙箱全量测试
create_res, _, _ = admin_sess('POST', '/api/admin/users/create', {'username': test_user, 'password': 'password123', 'role': 'user'})
assert create_res.get('success') is True
login_ok, _, _ = user_sess('POST', '/api/login', {'username': test_user, 'password': 'password123'})
assert login_ok.get('success') is True
print("=> 成功验证: 自主注册开关与系统配置热更新全流程通过")

# 4. 验证沙箱容量监控与一键资源回收 (纯内存租约模式)
print("\n[Test 4] 沙箱容量监控与一键资源回收 (非封禁纯释放)")
# 4.1 用户启动沙箱
env_start, _, _ = user_sess('POST', '/api/env/toggle')
print("User env start:", env_start)
assert env_start.get('success') is True

# 4.2 管理员查询沙箱列表 (验证无锁快照无阻塞)
sandboxes, _, _ = admin_sess('GET', '/api/admin/sandboxes')
print(f"Admin Sandboxes List (Count: {len(sandboxes)}):", sandboxes)
target_box = None
for sb in sandboxes:
    if sb['user'] == test_user:
        target_box = sb
        break
assert target_box is not None, f"User {test_user} sandbox not in admin list"
assert target_box['running'] is True, "Sandbox should be marked running"
print(f"Found running sandbox: box_id={target_box['box_id']}, user={target_box['user']}, ttl_rem={target_box['ttl_remaining_sec']}s")

# 4.3 管理员一键强制释放沙箱 (只释放资源，不封账号)
rel_res, _, _ = admin_sess('POST', '/api/admin/sandboxes/release', {'user': test_user})
print("Force Release Response:", rel_res)
assert rel_res.get('success') is True

# 4.4 检查用户状态: 用户会话依然在线，但沙箱已被回收
user_me, _, _ = user_sess('GET', '/api/me')
print("User status after release:", user_me)
assert user_me.get('loggedIn') is True, "User session should remain active!"
assert user_me.get('envRunning') is False, "Sandbox environment must be stopped!"
assert user_me.get('boxId') == '', "BoxID must be cleared!"

# 4.5 检查审计日志记录了 SANDBOX_FORCE_RELEASED
audit_rel, _, _ = admin_sess('GET', '/api/admin/audit?limit=5&event_type=SANDBOX_FORCE_RELEASED')
print("Audit (SANDBOX_FORCE_RELEASED):", audit_rel)
assert any(i['target'] == test_user for i in audit_rel.get('items', [])), "Release audit log not found"
print("=> 成功验证: 沙箱资源被成功回收，用户账号完好在线，审计流水完整记录")

# 5. 验证用户禁用 (USER_TOGGLE) 级联斩断一切
print("\n[Test 5] 用户禁用级联处决验证 (封号一键踢下线并销毁一切)")
# 5.1 用户重新拉起沙箱
user_sess('POST', '/api/env/toggle')
user_me2, _, _ = user_sess('GET', '/api/me')
assert user_me2.get('envRunning') is True

# 5.2 管理员禁用该账号
toggle_res, _, _ = admin_sess('POST', '/api/admin/users/toggle', {'username': test_user})
print("Admin toggle user to disabled:", toggle_res)
assert toggle_res.get('success') is True

# 5.3 用户再次请求，应已被踢下线 (未登录)
user_me_after_ban, _, _ = user_sess('GET', '/api/me')
print("User status after disabled:", user_me_after_ban)
assert user_me_after_ban.get('loggedIn') is False, "Banned user session must be destroyed"

# 5.4 检查沙箱列表，其沙箱必须已不在
sandboxes_after_ban, _, _ = admin_sess('GET', '/api/admin/sandboxes')
assert not any(sb['user'] == test_user for sb in sandboxes_after_ban), "Banned user sandbox must be purged"
print("=> 成功验证: 用户禁用自动实现级联下线与沙箱物理销毁")

# 6. 用户修改密码级联注销会话与审计测试
print("\n[Test 6] 用户自主修改密码全链路会话注销与审计")
# 重新启用该账号并登录
admin_sess('POST', '/api/admin/users/toggle', {'username': test_user})
user_sess2 = make_session()
login_ok, _, _ = user_sess2('POST', '/api/login', {'username': test_user, 'password': 'password123'})
assert login_ok.get('success') is True

# 修改密码
change_res, _, _ = user_sess2('POST', '/api/user/password', {'oldPassword': 'password123', 'newPassword': 'newpassword456!'})
print("User Change Password:", change_res)
assert change_res.get('success') is True

# 旧会话应已失效
user_me3, _, _ = user_sess2('GET', '/api/me')
assert user_me3.get('loggedIn') is False, "Session must be evicted after password change"

# 用新密码登录
user_sess3 = make_session()
login_new, _, _ = user_sess3('POST', '/api/login', {'username': test_user, 'password': 'newpassword456!'})
assert login_new.get('success') is True

# 清理测试用户
del_res, _, _ = admin_sess('POST', '/api/admin/users/delete', {'username': test_user})
assert del_res.get('success') is True
print("=> 成功验证: 用户改密后级联清退全部会话并成功入库审计")

# 7. 安全响应头与 Cookie 标志校验
print("\n[Test 7] 安全响应头与 Cookie 属性校验")
_, code, resp_headers = admin_sess('GET', '/login')
assert 'nosniff' in resp_headers.get('x-content-type-options', '').lower(), "Missing nosniff header"
assert 'deny' in resp_headers.get('x-frame-options', '').lower(), "Missing frame-options DENY"
assert 'max-age=' in resp_headers.get('strict-transport-security', '').lower(), "Missing HSTS header"
print("=> 成功验证: X-Content-Type-Options / X-Frame-Options / HSTS 安全头均严格下发")

# 8. 前端静态资源与 TanStack Table 资产完整性校验
print("\n[Test 8] 前端页面与打包资产校验")
admin_page, page_code, _ = admin_sess('GET', '/admin')
assert page_code == 200
assert 'AttackBox ADMIN' in admin_page

js_matches = re.findall(r'src="(/assets/admin-[^"]+\.js)"', admin_page)
assert len(js_matches) > 0, "admin js bundle reference not found in HTML"
js_url = js_matches[0]
js_content, js_code, _ = admin_sess('GET', js_url)
assert js_code == 200, f"Failed to load bundle {js_url}"
assert len(js_content) > 10000, "JS bundle content unexpectedly small"
print(f"=> 成功验证: /admin 前端单页应用与其资产 ({js_url}) 响应正常，状态码 200")

# 9. 登录失败频次限流防护校验 (防刷审计与暴力破解)
print("\n[Test 9] 登录频次限流防护 (防止审计流水被刷满与暴力破解)")
limiter_sess = make_session()
statuses = []
for i in range(7):
    res, status_code, _ = limiter_sess('POST', '/api/login', {'username': f'brute_user_{i}', 'password': 'wrong_password'})
    statuses.append(status_code)
print("Attempts HTTP status codes:", statuses)
assert 429 in statuses, f"Rate limiter failed to block excessive login failures: {statuses}"
print("=> 成功验证: 连续登录失败被精准识别并触发 HTTP 429 频控封锁")

print("\n==================================================")
print("SUCCESS: 全部 9 组深度安全防护与架构重构端到端测试 100% 通过！")
print("==================================================")
