#!/usr/bin/env python3
"""Create fresh installation configuration. Never overwrite existing credentials."""
import argparse
import getpass
import json
import os
import sys
from pathlib import Path
import secrets
from urllib.parse import urlsplit

REPO = Path(__file__).resolve().parent.parent


def prompt(label, default='', secret=False):
    read = getpass.getpass if secret else input
    value = read(f'{label}' + (f' [{default}]' if default and not secret else '') + ': ').strip()
    if any(c in value for c in '\n\r\0'):
        raise ValueError('配置值不能包含换行或空字符')
    return value or default


def configuration(database, origin, resend='', mail_from=''):
    db = urlsplit(database)
    if db.scheme not in ('postgres', 'postgresql') or not db.hostname or not db.path.strip('/'):
        raise ValueError('需要有效的 PostgreSQL 连接 URL')
    site = urlsplit(origin)
    if (site.scheme not in ('http', 'https') or not site.hostname or site.username or
            site.path not in ('', '/') or site.query or site.fragment):
        raise ValueError('网站地址必须是完整 origin，例如 https://example.com')
    if site.scheme != 'https' and site.hostname not in ('localhost', '127.0.0.1', '::1'):
        raise ValueError('公网网站必须使用 HTTPS')
    if bool(resend) != bool(mail_from):
        raise ValueError('Resend Key 与发件地址必须一起配置，或一起留空')
    cluster = secrets.token_hex(32)
    # Public HTTPS requires a WSS gateway; use the site hostname and explicit TLS files.
    host = f'[{site.hostname}]' if ':' in site.hostname else site.hostname
    public = f'wss://{host}:8443' if site.scheme == 'https' else 'ws://127.0.0.1:8443'
    values = dict(NODE_ENV='production', PORT='3000', HOST='127.0.0.1', DATABASE_URL=database,
                  SANDBOX_CONTROL_ENABLED='true', SANDBOX_SCHEDULING_ENABLED='true',
                  SANDBOX_CLUSTER_SECRET=cluster, SANDBOX_SIGNING_SEED_HEX=secrets.token_hex(32),
                  SANDBOX_NODES_JSON=json.dumps([dict(id='node-primary', label='Primary Node',
                                                     publicDataUrl=public, maxSlots=16)], separators=(',', ':')),
                  CONTENT_DATA_DIR='/var/lib/attackbox-web/content', LAB_DATA_DIR='/var/lib/attackbox-web/labs',
                  CONTENT_CREDENTIALS_KEY=secrets.token_hex(32), QUIZ_ANSWER_HMAC_SECRET=secrets.token_hex(32),
                  RESEND_API_KEY=resend, MAIL_FROM=mail_from, APP_BASE_URL=origin.rstrip('/'))
    node = (REPO / 'configs/node.yaml.example').read_text()
    node = node.replace('CHANGE_ME_AT_LEAST_32_BYTES', cluster).replace('ws://127.0.0.1:8443', public)
    return values, node


def write_configuration(env_path, node_path, values, node):
    # Check both before writing either; O_EXCL also protects against concurrent initialization.
    if env_path.exists() or env_path.is_symlink() or node_path.exists() or node_path.is_symlink():
        raise ValueError('配置已存在；拒绝覆盖或重新生成密钥。请编辑现有配置。')
    env_path.parent.mkdir(parents=True, exist_ok=True)
    node_path.parent.mkdir(parents=True, exist_ok=True)
    written = []
    try:
        for path, content in [(env_path, ''.join(f'{k}={json.dumps(v, ensure_ascii=False)}\n' for k, v in values.items())), (node_path, node)]:
            fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            written.append(path)
            with os.fdopen(fd, 'w') as stream:
                stream.write(content)
    except BaseException:
        for path in written:
            path.unlink()
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()
    if not sys.stdin.isatty():
        raise ValueError("请在交互终端中运行安装向导")
    env_path, node_path = Path('/etc/attackbox/web.env'), REPO / 'configs/node.yaml'
    if env_path.exists() or node_path.exists():
        raise ValueError('配置已存在，初始化不会覆盖。')
    database = prompt('现有数据库 URL（数据库需先创建，输入隐藏）', secret=True)
    origin = prompt('网站 origin（本地测试可保留默认）', 'http://localhost')
    resend = prompt('Resend API Key（可留空，稍后配置）', secret=True)
    mail_from = prompt('发件地址') if resend else ''
    values, node = configuration(database, origin, resend, mail_from)
    if origin.startswith('https://'):
        cert = prompt('Worker TLS 证书完整路径')
        key = prompt('Worker TLS 私钥完整路径')
        if not Path(cert).is_file() or not Path(key).is_file():
            raise ValueError('Worker TLS 证书与私钥必须已存在；也可先用 localhost 完成本机部署')
        node = node.replace('tls_enabled: false', 'tls_enabled: true\n  tls_cert: ' + json.dumps(cert) + '\n  tls_key: ' + json.dumps(key))
    write_configuration(env_path, node_path, values, node)
    print('已生成 web.env 和 configs/node.yaml，权限 0600。密钥不显示。')
    if not resend:
        print('邮件未配置：管理员可先初始化，注册和找回密码需补齐邮件配置。')
    print('下一步：npm --prefix web ci，然后 sudo scripts/manage.py migrate 和 sudo scripts/manage.py admin。')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError) as error:
        # Avoid echoing database URLs or provider credentials in exception text.
        print('初始化失败：' + (str(error) if isinstance(error, ValueError) else '文件操作失败，请检查权限与路径。'))
        raise SystemExit(1)
