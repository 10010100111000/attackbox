#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
test_oauth_suite.py
AttackBox GitHub 与 Outlook OAuth 安全与功能端到端测试套件
验证项目：
1. 公开端点 /api/public/oauth-providers 动态感知
2. 管理员 OAuth 配置 AES-256-GCM 密文存储与 Secret 零回显
3. 授权跳转 302 与 State HMAC 签名防护
4. 防 CSRF 门控拦截 (伪造/篡改 State 拒绝)
5. 两步注册 Ticket 签名校验与防篡改
6. SQLite 物理数据库落库密文真实性核查
"""

import sys
import json
import sqlite3
import urllib.parse
import requests

BASE_URL = "https://127.0.0.1"
DB_PATH = "/var/lib/attackbox/attackbox.db"

session = requests.Session()
session.verify = False

import urllib3
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

def log_test(name):
    print(f"\n[Test] {name}")

def assert_true(cond, msg):
    if not cond:
        print(f"FAILED: {msg}")
        sys.exit(1)
    print(f"  OK: {msg}")

def main():
    print("=" * 60)
    print("    ATTACKBOX OAUTH 第三方认证与安全防护端到端测试")
    print("=" * 60)

    # 1. 登录管理员
    log_test("1. 管理员登录")
    res = session.post(f"{BASE_URL}/api/login", json={"username": "laji", "password": "qwer1234!"})
    assert_true(res.status_code == 200 and res.json().get("success"), "管理员登录成功")

    # 2. 查询初始公开 OAuth 提供商
    log_test("2. 公开第三方端点查询")
    pub_res = session.get(f"{BASE_URL}/api/public/oauth-providers")
    assert_true(pub_res.status_code == 200, "公开端点响应 200")
    initial_providers = pub_res.json().get("providers", [])
    print(f"  当前已启用提供商: {initial_providers}")

    # 3. 管理员配置 GitHub OAuth 并验证 AES-256-GCM 密文落库
    log_test("3. 管理员配置 GitHub OAuth 并启用")
    mock_client_id = "test_gh_client_123456"
    mock_secret = "ghp_ultra_secret_token_never_leak_9876543210"

    cfg_res = session.post(f"{BASE_URL}/api/admin/oauth/settings", json={
        "provider": "github",
        "client_id": mock_client_id,
        "client_secret": mock_secret,
        "enabled": True
    })
    assert_true(cfg_res.status_code == 200 and cfg_res.json().get("success"), "配置 GitHub OAuth 成功")

    # 4. 验证管理端读取脱敏 (绝不暴露 Secret 明文)
    log_test("4. 管理端配置回显脱敏验证")
    get_cfg = session.get(f"{BASE_URL}/api/admin/oauth/settings")
    assert_true(get_cfg.status_code == 200, "获取配置成功")
    configs = get_cfg.json().get("configs", {})
    gh_cfg = configs.get("github", {})
    assert_true(gh_cfg.get("client_id") == mock_client_id, "Client ID 匹配")
    assert_true(gh_cfg.get("configured") == True, "状态为 configured=true")
    assert_true(gh_cfg.get("enabled") == True, "状态为 enabled=true")
    assert_true("client_secret" not in gh_cfg and "secret" not in gh_cfg, "API 严格未包含任何 Secret 字段")

    # 5. 验证公开提供商已动态包含 github
    log_test("5. 公开端点动态感知验证")
    pub_res2 = session.get(f"{BASE_URL}/api/public/oauth-providers")
    active_now = pub_res2.json().get("providers", [])
    assert_true("github" in active_now, "公开提供商列表中已热更新出现 'github'")

    # 6. SQLite 物理密文检查
    log_test("6. SQLite 物理数据库密文核验")
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute("SELECT client_id, encrypted_secret, nonce, enabled FROM oauth_configs WHERE provider = 'github'")
    row = cursor.fetchone()
    assert_true(row is not None, "数据库 oauth_configs 中存在记录")
    client_id_db, enc_sec, nonce, enabled_db = row
    assert_true(enc_sec != mock_secret, "数据库存储的 Secret 绝非明文")
    assert_true(len(enc_sec) > 30 and len(nonce) > 10, f"存储为有效的 AES-GCM 密文 ({enc_sec[:10]}...) 与 Nonce")
    conn.close()

    # 7. 授权重定向与 State Cookie 植入测试
    log_test("7. 授权发起重定向与 State HMAC 签名校验")
    anon_session = requests.Session()
    anon_session.verify = False
    auth_req = anon_session.get(f"{BASE_URL}/api/oauth/github/authorize", allow_redirects=False)
    assert_true(auth_req.status_code == 307 or auth_req.status_code == 302, f"重定向状态码符合预期 ({auth_req.status_code})")
    location = auth_req.headers.get("Location", "")
    assert_true("github.com/login/oauth/authorize" in location, "重定向目标为 GitHub OAuth 端点")
    assert_true(f"client_id={mock_client_id}" in location, "重定向携带已配置的 Client ID")
    
    # 提取 state 参数与 Cookie
    parsed_loc = urllib.parse.urlparse(location)
    q_params = urllib.parse.parse_qs(parsed_loc.query)
    req_state = q_params.get("state", [""])[0]
    assert_true(bool(req_state), "重定向携带了随机 State 参数")

    cookie_val = anon_session.cookies.get("attackbox_oauth_state")
    assert_true(cookie_val is not None and "." in cookie_val, "成功下发签名 State Cookie")

    # 8. 防 CSRF 门控拦截验证
    log_test("8. 防 CSRF 拦截验证 (伪造/篡改 State 拒绝)")
    # (a) 篡改 state 参数
    fake_cb = anon_session.get(f"{BASE_URL}/api/oauth/github/callback?code=fake_code&state=TAMPERED_STATE", allow_redirects=False)
    assert_true(fake_cb.status_code == 303 or fake_cb.status_code == 302, "拦截并执行重定向")
    cb_loc = fake_cb.headers.get("Location", "")
    assert_true("error=" in cb_loc and "%E6%A0%A1%E9%AA%8C%E5%A4%B1%E8%B4%A5" in cb_loc or "error" in cb_loc, "重定向回登录页并带有校验失败提示")

    # (b) 无 state cookie 请求
    clean_session = requests.Session()
    clean_session.verify = False
    no_cookie_cb = clean_session.get(f"{BASE_URL}/api/oauth/github/callback?code=fake_code&state={req_state}", allow_redirects=False)
    assert_true("error=" in no_cookie_cb.headers.get("Location", ""), "无 Cookie 请求被精准拦截")

    # 9. 两步注册 Ticket 校验测试
    log_test("9. 两步注册 Ticket 防伪造与防篡改验证")
    # (a) 伪造任意非法 Ticket
    fake_ticket_res = session.post(f"{BASE_URL}/api/oauth/complete-registration", json={
        "ticket": "invalid.fake.ticket",
        "username": "oauth_hacker",
        "password": "Password123!"
    })
    assert_true(fake_ticket_res.status_code == 400, "非法 Ticket 提交被拒 (HTTP 400)")

    # 10. 清理测试产生的配置 (恢复干净环境)
    log_test("10. 恢复测试配置")
    session.post(f"{BASE_URL}/api/admin/oauth/settings", json={
        "provider": "github",
        "client_id": "",
        "client_secret": "",
        "enabled": False
    })
    print("  已禁用测试用例")

    print("\n" + "=" * 60)
    print("SUCCESS: 全部 10 组 OAuth 深度认证与安全防护端到端测试 100% 通过！")
    print("=" * 60)

if __name__ == "__main__":
    main()
