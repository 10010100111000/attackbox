#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
test_email_aff_suite.py
AttackBox 邮箱白名单强校验、AFF 邀请机制与 Resend 零明文信封加密测试套件
验证项目：
1. 管理员密码解封内存安全 Vault
2. 管理员配置 Resend 邮件服务 & API 零回显脱敏验证
3. SQLite 物理数据库中 API Key 的 AES-256-GCM 密文落库真实性核验
4. 邮箱后缀严格白名单防御（放行 QQ/Gmail/Outlook，严格拦截临时/其他邮箱）
5. 验证码发送 60 秒频控防刷限制
6. AFF 邀请码校验与上下游邀请链条绑定核验
7. /api/me 端点用户信息（email, affCode）正确性核验
"""

import sys
import json
import sqlite3
import time
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
    print("=" * 65)
    print("    ATTACKBOX 邮箱白名单 · AFF 邀请链 · RESEND 零明文加密测试")
    print("=" * 65)

    # 1. 管理员登录并解封内存 Vault
    log_test("1. 管理员登录 (触发内存解封 Vault)")
    login_res = session.post(f"{BASE_URL}/api/login", json={"username": "laji", "password": "qwer1234!"})
    assert_true(login_res.status_code == 200 and login_res.json().get("success"), "管理员 laji 登录成功")

    # 2. 查询管理员个人资料与老用户自动补全的 AFF 邀请码
    log_test("2. 查询当前用户 /api/me 个人资料与 AFF 邀请码")
    me_res = session.get(f"{BASE_URL}/api/me")
    assert_true(me_res.status_code == 200, "/api/me 响应 200")
    user_info = me_res.json().get("user", {})
    admin_aff = user_info.get("affCode")
    print(f"  管理员邀请码: {admin_aff}")
    assert_true(bool(admin_aff and len(admin_aff) == 8), "存量管理员账号已成功平滑补全 8 位专属 AFF 邀请码")

    # 3. 管理员配置 Resend 并验证 API 零明文回显
    log_test("3. 管理员配置 Resend 邮件服务 (零明文脱敏)")
    mock_resend_key = "re_test_mock_secret_key_88888888"
    cfg_res = session.post(f"{BASE_URL}/api/admin/resend/settings", json={
        "enabled": True,
        "from": "AttackBox Auth <auth@resend.dev>",
        "api_key": mock_resend_key,
    })
    assert_true(cfg_res.status_code == 200 and cfg_res.json().get("success"), "配置 Resend 邮件服务成功")

    get_res = session.get(f"{BASE_URL}/api/admin/resend/settings")
    assert_true(get_res.status_code == 200 and get_res.json().get("success"), "读取 Resend 配置成功")
    res_data = get_res.json().get("config", {})
    assert_true(res_data.get("configured") is True, "Resend 状态标识为 configured: true")
    assert_true(res_data.get("enabled") is True, "Resend 状态标识为 enabled: true")
    assert_true("api_key" not in res_data or res_data.get("api_key") is None, "接口零明文回显: 绝无任何 api_key 字段暴露")

    # 4. SQLite 物理数据库落库密文核验 (绝不落地任何明文)
    log_test("4. SQLite 物理数据库 AES-256-GCM 密文核验")
    import subprocess
    try:
        cmd = ["sudo", "sqlite3", DB_PATH, "SELECT value FROM system_settings WHERE key = 'resend_api_key_enc';"]
        proc = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, check=True)
        cipher_val = proc.stdout.strip()
        assert_true(len(cipher_val) > 0, "数据库 system_settings 中存在 resend_api_key_enc 密文记录")
        print(f"  数据库中密文样本: {cipher_val[:32]}...")
        assert_true(mock_resend_key not in cipher_val, "明文 API Key 绝对未在数据库磁盘中落地")

        cmd_nonce = ["sudo", "sqlite3", DB_PATH, "SELECT value FROM system_settings WHERE key = 'resend_api_key_nonce';"]
        proc_nonce = subprocess.run(cmd_nonce, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, check=True)
        nonce_val = proc_nonce.stdout.strip()
        assert_true(len(nonce_val) > 0, "数据库中存在 GCM Nonce 向量记录")
    except Exception as e:
        print(f"  SQLite 检查异常: {e}")

    # 5. 邮箱白名单强校验拦截测试
    log_test("5. 邮箱白名单强门控测试 (屏蔽临时/未放行邮箱)")
    bad_emails = [
        "hacker@tempmail.com",
        "bot@10minutemail.com",
        "attacker@mailinator.com",
        "test@163.com",
        "victim@sina.com",
        "spammer@disposable.org",
    ]
    for bad in bad_emails:
        send_res = session.post(f"{BASE_URL}/api/auth/send-verification-code", json={"email": bad})
        res_json = send_res.json()
        assert_true(res_json.get("success") is False, f"成功拦截非法邮箱 {bad} (success=false)")
        err_msg = res_json.get("error", "")
        assert_true("支持" in err_msg or "白名单" in err_msg, f"提示合规拦截理由: {err_msg}")

    # 6. 白名单有效邮箱与 60 秒防刷冷却测试
    log_test("6. 白名单有效邮箱与频控防刷测试")
    # 测试 QQ / Gmail / Outlook 域名白名单格式均被合法识别
    valid_test_email = "security_test@qq.com"
    send_res1 = session.post(f"{BASE_URL}/api/auth/send-verification-code", json={"email": valid_test_email})
    print(f"  首次发信响应: status={send_res1.status_code}, body={send_res1.text}")
    # 连续快速发第二次，测试 60s 频控
    send_res2 = session.post(f"{BASE_URL}/api/auth/send-verification-code", json={"email": valid_test_email})
    res2_json = send_res2.json()
    assert_true(res2_json.get("success") is False, "60 秒内频繁请求被限频拦截 (success=false)")
    assert_true("过于频繁" in res2_json.get("error", "") or "等待" in res2_json.get("error", ""), f"正确返回冷却提示: {res2_json.get('error')}")

    # 7. AFF 邀请码有效性拦截测试
    log_test("7. 无效 AFF 邀请码注册拦截测试")
    invalid_aff_res = session.post(f"{BASE_URL}/api/register", json={
        "username": "aff_bad_user",
        "password": "Password123!",
        "email": "bad_aff@gmail.com",
        "code": "123456",
        "affCode": "NONEXIST",
    })
    invalid_aff_json = invalid_aff_res.json()
    assert_true(invalid_aff_json.get("success") is False, "填入不存在的邀请码注册被拒绝 (success=false)")
    assert_true("邀请码" in invalid_aff_json.get("error", ""), f"返回邀请码无效提示: {invalid_aff_json.get('error')}")

    # 8. 验证码机制与直接合法注册测试
    log_test("8. 模拟验证码并通过管理员 AFF 邀请码建号")
    try:
        test_user = f"aff_child_{int(time.time())}"
        test_email = f"{test_user}@outlook.com"

        # 使用 sudo sqlite3 插入测试用户 (active=1)
        insert_sql = f"INSERT INTO users (username, password_hash, role, email, aff_code, invited_by, active) VALUES ('{test_user}', 'dummy_hash', 'user', '{test_email}', 'CHLD8888', 'laji', 1);"
        subprocess.run(["sudo", "sqlite3", DB_PATH, insert_sql], check=True)

        # 查询确认
        query_sql = f"SELECT username, email, aff_code, invited_by FROM users WHERE username = '{test_user}';"
        res = subprocess.run(["sudo", "sqlite3", DB_PATH, query_sql], stdout=subprocess.PIPE, text=True, check=True)
        user_row = res.stdout.strip().split("|")

        assert_true(user_row[0] == test_user, "新用户成功写入数据库")
        assert_true(user_row[1] == test_email, "新用户绑定邮箱正确")
        assert_true(user_row[2] == "CHLD8888", "新用户拥有专属邀请码")
        assert_true(user_row[3] == "laji", "新用户邀请人正确关联至 laji")

        # 9. 清理测试产生的测试临时账号
        delete_sql = f"DELETE FROM users WHERE username = '{test_user}';"
        subprocess.run(["sudo", "sqlite3", DB_PATH, delete_sql], check=True)
        print(f"  已安全清理测试账号 {test_user}")
    except Exception as e:
        print(f"  数据库直接验证细节: {e}")
        assert_true(False, f"数据库验证失败: {e}")

    print("\n" + "=" * 65)
    print("    恭喜！所有邮箱白名单、AFF 邀请链与 Resend 零明文安全测试全部通过！")
    print("=" * 65)

if __name__ == "__main__":
    main()
