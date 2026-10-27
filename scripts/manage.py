#!/usr/bin/env python3
"""Explicit database migration and one-time administrator initialization."""
import argparse
import getpass
import json
import os
import sys
from pathlib import Path
import re
import shlex
import subprocess

REPO = Path(__file__).resolve().parent.parent


def read_environment(path):
    values = {}
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith('#'):
            continue
        key, sep, value = line.partition('=')
        key, value = key.strip(), value.strip()
        if not sep or not re.fullmatch(r'[A-Z][A-Z0-9_]*', key) or key in values:
            raise ValueError('配置文件存在无效或重复的配置项')
        if value.startswith('"'):
            value = json.loads(value)
        elif value.startswith("'"):
            parts = shlex.split(value)
            if len(parts) != 1:
                raise ValueError('无效的配置引号')
            value = parts[0]
        if not isinstance(value, str) or '\0' in value or '\n' in value or '\r' in value:
            raise ValueError('配置值必须是单行字符串')
        values[key] = value
    if not values.get('DATABASE_URL'):
        raise ValueError('缺少 DATABASE_URL')
    return values


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('operation', choices=['migrate', 'admin'])
    args = parser.parse_args()
    values = read_environment(Path('/etc/attackbox/web.env'))
    payload = None
    if args.operation == 'admin':
        if not sys.stdin.isatty():
            raise ValueError('请在交互终端中创建管理员')
        email = input('管理员邮箱: ').strip().lower()
        name = input('显示名称 [Administrator]: ').strip() or 'Administrator'
        password = getpass.getpass('管理员密码（至少 12 位）: ')
        if password != getpass.getpass('再次输入密码: '):
            raise ValueError('两次密码不一致')
        if not re.fullmatch(r'[^\s@]+@[^\s@]+\.[^\s@]+', email) or len(email) > 254 or len(password) < 12 or len(name) > 100:
            raise ValueError('邮箱、密码或显示名称不符合要求')
        payload = json.dumps(dict(email=email, password=password, displayName=name))
    script = 'apply-migrations.mjs' if args.operation == 'migrate' else 'init-admin.mjs'
    # No shell execution; credentials are not command-line arguments or temporary files.
    result = subprocess.run(['node', f'server/scripts/{script}'], cwd=REPO / 'web',
                            env={**os.environ, **values}, input=payload, text=True)
    raise SystemExit(result.returncode)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError):
        print('操作失败：请检查配置文件、输入信息与依赖是否已安装。')
        raise SystemExit(1)
