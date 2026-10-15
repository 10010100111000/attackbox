#!/usr/bin/env python3
import requests
import json
import sys

import urllib3
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

def main():
    s = requests.Session()
    s.verify = False
    base_url = 'https://127.0.0.1'

    print("=== 1. 用户登录 ===")
    r = s.post(f'{base_url}/api/login', json={'username': 'laji', 'password': 'qwer1234!'})
    assert r.status_code == 200, f"Login failed: {r.text}"
    print("Login:", r.json())

    print("\n=== 2. 查询并重置初始状态 ===")
    s.post(f'{base_url}/api/env/destroy')
    r = s.get(f'{base_url}/api/me')
    me = r.json()
    print("Initial Me (after reset):", json.dumps({k: me[k] for k in ['cliRunning', 'guiRunning', 'envRunning']}))
    assert me['cliRunning'] is False and me['guiRunning'] is False

    print("\n=== 3. 独立启动 CLI ===")
    r = s.post(f'{base_url}/api/env/cli/toggle')
    print("CLI Toggle Response:", r.status_code, r.json())
    assert r.json().get('success') is True

    r = s.get(f'{base_url}/api/me')
    me = r.json()
    print("State after CLI start:", json.dumps({k: me[k] for k in ['cliRunning', 'guiRunning', 'envRunning']}))
    assert me['cliRunning'] is True
    assert me['guiRunning'] is False

    print("\n=== 4. 独立启动 GUI ===")
    r = s.post(f'{base_url}/api/env/gui/toggle')
    print("GUI Toggle Response:", r.status_code, r.json())
    assert r.json().get('success') is True

    r = s.get(f'{base_url}/api/me')
    me = r.json()
    print("State after GUI start:", json.dumps({k: me[k] for k in ['cliRunning', 'guiRunning', 'envRunning']}))
    assert me['cliRunning'] is True
    assert me['guiRunning'] is True

    print("\n=== 5. 独立关闭 GUI ===")
    r = s.post(f'{base_url}/api/env/gui/toggle')
    print("GUI Toggle (stop) Response:", r.status_code, r.json())
    assert r.json().get('success') is True

    r = s.get(f'{base_url}/api/me')
    me = r.json()
    print("State after GUI stop:", json.dumps({k: me[k] for k in ['cliRunning', 'guiRunning', 'envRunning']}))
    assert me['cliRunning'] is True
    assert me['guiRunning'] is False

    print("\n=== 6. 一键全局释放销毁 ===")
    r = s.post(f'{base_url}/api/env/destroy')
    print("Destroy Response:", r.status_code, r.json())
    assert r.json().get('success') is True

    r = s.get(f'{base_url}/api/me')
    me = r.json()
    print("State after Destroy:", json.dumps({k: me[k] for k in ['cliRunning', 'guiRunning', 'envRunning']}))
    assert me['cliRunning'] is False
    assert me['guiRunning'] is False
    assert me['envRunning'] is False

    print("\n>>> 全部生命周期与独立启停测试顺利通过！")

if __name__ == '__main__':
    main()
