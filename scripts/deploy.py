#!/usr/bin/env python3
"""AttackBox role-specific configuration and deployment. Build remains separate."""
import argparse
import getpass
import fcntl
import ipaddress
import json
import os
from pathlib import Path
import pwd
import grp
import re
import secrets
import shlex
import shutil
import ssl
import subprocess
import sys
import tempfile
import time
from urllib.parse import urlsplit
from urllib.request import build_opener, ProxyHandler, HTTPSHandler
try:
    import yaml
except ImportError:
    raise SystemExit('需要 PyYAML：Debian/Ubuntu 可安装 python3-yaml，或使用 scripts/requirements.txt')

ROOT = Path(__file__).resolve().parent.parent
ENV = Path('/etc/attackbox/web.env')
WORKER = Path('/etc/attackbox/worker.yaml')
WEB = Path('/var/www/attackbox-web')
BIN = Path('/usr/local/bin/attackbox-worker')
ETC = Path('/etc')


class ConfigError(ValueError):
    pass


def run(*args, **kwargs):
    return subprocess.run([str(a) for a in args], check=True, **kwargs)


def atomic(path, text, mode=0o600, exclusive=False):
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, name = tempfile.mkstemp(dir=path.parent, prefix='.' + path.name)
    try:
        with os.fdopen(fd, 'w') as f:
            f.write(text)
            f.flush()
            os.fsync(f.fileno())
        os.chmod(name, mode)
        if exclusive:
            os.link(name, path)  # Refuse overwrites, including concurrent setup or symlinks.
        else:
            os.replace(name, path)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def read_env(path=None):
    path = path or ENV
    result = {}
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith('#'):
            continue
        key, sep, value = line.partition('=')
        key, value = key.strip(), value.strip()
        if not sep or not re.fullmatch('[A-Z][A-Z0-9_]*', key) or key in result:
            raise ConfigError('环境配置包含无效或重复字段')
        if value.startswith('"'):
            value = json.loads(value)
        elif value.startswith("'"):
            parts = shlex.split(value)
            if len(parts) != 1:
                raise ConfigError('环境配置引号无效')
            value = parts[0]
        if not isinstance(value, str) or any(c in value for c in '\n\r\0'):
            raise ConfigError('环境配置必须使用单行字符串')
        result[key] = value
    return result


def env_text(values):
    return ''.join(f'{k}={json.dumps(v, ensure_ascii=False)}\n' for k, v in values.items())


def ws_url(value):
    u = urlsplit(value)
    if u.scheme not in ('ws', 'wss') or not u.hostname or u.username or u.password or u.query or u.fragment or u.path not in ('', '/'):
        raise ConfigError('网关地址必须为 ws(s)://host[:port]')
    _ = u.port
    if u.scheme == 'ws' and u.hostname not in ('localhost', '127.0.0.1', '::1'):
        raise ConfigError('公网网关必须使用 wss://')
    return value.rstrip('/')


def endpoint(host, port):
    host = host or '0.0.0.0'
    port = int(port)
    if not 1 <= port <= 65535:
        raise ConfigError('监听端口应为 1–65535')
    if host != 'localhost':
        ipaddress.ip_address(host)
    host = {'0.0.0.0': '127.0.0.1', '::': '::1'}.get(host, host)
    return f'[{host}]:{port}' if ':' in host else f'{host}:{port}'


def worker_health(config):
    gateway = config['gateway']
    ws_url(gateway['public_data_url'])
    raw = gateway.get('listen_addr', '0.0.0.0:8443')
    host, port = raw.rsplit(':', 1)
    local = endpoint(host.strip('[]'), port)
    if gateway.get('tls_enabled'):
        u = urlsplit(ws_url(gateway['public_data_url']))
        if u.scheme != 'wss':
            raise ConfigError('TLS 网关需要 wss public_data_url')
        return f'https://{u.netloc}/health'
    return f'http://{local}/health'


def nodes(values):
    items = json.loads(values['SANDBOX_NODES_JSON'])
    if not isinstance(items, list) or not 1 <= len(items) <= 32:
        raise ConfigError('节点数量应为 1–32')
    seen = set()
    for item in items:
        if not re.fullmatch('[A-Za-z0-9_-]{1,64}', item['id']) or item['id'] in seen:
            raise ConfigError('节点 ID 无效或重复')
        if not isinstance(item.get('label'),str) or not item['label'].strip() or len(item['label'])>80 or type(item.get('maxSlots')) is not int or not 1<=item['maxSlots']<=250:
            raise ConfigError('节点名称或容量无效')
        seen.add(item['id'])
        item['publicDataUrl'] = ws_url(item['publicDataUrl'])
    return items


def match_worker(values, config):
    info = config['node']
    match = next((n for n in nodes(values) if n['id'] == info['node_id']), None)
    if not match or match['publicDataUrl'] != ws_url(config['gateway']['public_data_url']) or not secrets.compare_digest(values['SANDBOX_CLUSTER_SECRET'], info['cluster_secret']):
        raise ConfigError('两端节点 ID、集群密钥或 public_data_url 不一致；先配置 Control 节点名单')


def render_nginx(values, upstream):
    enabled = values.get('CLOUDFLARE_ENABLED', 'false')
    if enabled not in ('true', 'false'):
        raise ConfigError('CLOUDFLARE_ENABLED 必须为 true 或 false')
    if bool(values.get('TURNSTILE_SITE_KEY', '').strip()) != bool(values.get('TURNSTILE_SECRET_KEY', '').strip()):
        raise ConfigError('Turnstile Site Key 与 Secret Key 必须同时配置')
    if enabled == 'false':
        template = (ROOT/'deploy/control/nginx.conf.template').read_text()
        if template.count('@@WEB_UPSTREAM@@') != 1:
            raise ConfigError('nginx 模板占位符无效')
        return template.replace('@@WEB_UPSTREAM@@', upstream)

    origin = urlsplit(values.get('APP_BASE_URL', ''))
    domain = origin.hostname or ''
    if (origin.scheme != 'https' or origin.port not in (None, 443) or origin.username or origin.password
            or origin.path not in ('', '/') or origin.query or origin.fragment
            or not re.fullmatch(r'[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?', domain)
            or '.' not in domain):
        raise ConfigError('Cloudflare 入口需要有效的 HTTPS APP_BASE_URL，不带路径或自定义端口')
    if values.get('HOST', '127.0.0.1') not in ('127.0.0.1', '::1', 'localhost'):
        raise ConfigError('Cloudflare 同机部署的 Nuxt HOST 必须为 loopback，避免绕过入口')
    worker_raw = values.get('WORKER_PROXY_UPSTREAM', '127.0.0.1:8443')
    worker_url = urlsplit('http://' + worker_raw)
    if (worker_url.hostname not in ('127.0.0.1', '::1', 'localhost') or worker_url.username
            or worker_url.password or worker_url.path or worker_url.query or worker_url.fragment
            or not worker_url.port):
        raise ConfigError('WORKER_PROXY_UPSTREAM 必须为 loopback host:port')
    worker_upstream = endpoint(worker_url.hostname, worker_url.port)
    public = 'wss://' + domain
    entries = nodes(values)
    if len(entries) != 1 or entries[0]['publicDataUrl'] not in (public, public + ':443'):
        raise ConfigError('Cloudflare 同机模板只支持一个 Worker，publicDataUrl 应为 wss://网站域名')
    if WORKER.exists():
        config = yaml.safe_load(WORKER.read_text())
        gateway = config['gateway']
        raw_host, raw_port = gateway['listen_addr'].rsplit(':', 1)
        host = raw_host.strip('[]')
        if (host not in ('127.0.0.1', '::1', 'localhost') or gateway.get('tls_enabled')
                or endpoint(host, raw_port) != worker_upstream):
            raise ConfigError('Worker 必须在匹配的 loopback 地址监听明文，由 nginx 终止 TLS')
        if config['node']['control_url'] != f'ws://{upstream}/cluster/connect':
            raise ConfigError('Cloudflare 同机模板的 Worker control_url 必须直连本机 Nuxt /cluster/connect')
    replacements = {'WEB_UPSTREAM': upstream, 'WORKER_UPSTREAM': worker_upstream, 'DOMAIN': domain}
    for field, placeholder in [('ORIGIN_TLS_CERT', 'TLS_CERT'), ('ORIGIN_TLS_KEY', 'TLS_KEY')]:
        value = values.get(field, '')
        if not re.fullmatch(r'/[a-zA-Z0-9_./-]+', value) or not Path(value).is_file():
            raise ConfigError(f'{field} 必须为现有证书文件的绝对路径，不支持空格或特殊字符')
        replacements[placeholder] = value
    cidrs = []
    for line in (ROOT/'deploy/control/cloudflare-ips.txt').read_text().splitlines():
        line = line.strip()
        if line and not line.startswith('#'):
            cidrs.append(str(ipaddress.ip_network(line, strict=True)))
    if not cidrs or any(ipaddress.ip_network(c).prefixlen == 0 for c in cidrs):
        raise ConfigError('Cloudflare 地址范围为空或包含全网范围')
    replacements['CLOUDFLARE_GEO'] = '\n'.join(f'    {c} 1;' for c in cidrs)
    replacements['CLOUDFLARE_REAL_IP'] = '\n'.join(f'set_real_ip_from {c};' for c in cidrs)
    template = (ROOT/'deploy/control/nginx-cloudflare.conf.template').read_text()
    for key, value in replacements.items():
        template = template.replace('@@' + key + '@@', value)
    if '@@' in template:
        raise ConfigError('Cloudflare nginx 模板存在未替换占位符')
    return template


def prompt(value, label, default='', secret=False):
    if value is not None:
        return value
    if not sys.stdin.isatty():
        if default:
            return default
        raise ConfigError(f'非交互模式缺少 {label}')
    fn = getpass.getpass if secret else input
    return fn(label + (f' [{default}]' if default and not secret else '') + ': ') or default


def setup_control(a):
    if ENV.exists() or ENV.is_symlink():
        raise ConfigError('配置已存在，拒绝覆盖或重新生成密钥；请编辑现有配置')
    database = prompt(a.database, '数据库 URL', secret=True)
    db = urlsplit(database)
    if db.scheme not in ('postgres', 'postgresql') or not db.hostname or not db.path.strip('/'):
        raise ConfigError('数据库 URL 无效')
    origin = prompt(a.origin, '网站 origin', 'http://127.0.0.1').rstrip('/')
    u = urlsplit(origin)
    if u.scheme not in ('http', 'https') or not u.hostname or u.username or u.password or u.path or u.query or u.fragment:
        raise ConfigError('网站地址需要完整 HTTP(S) origin，不能包含路径')
    if u.scheme == 'http' and u.hostname not in ('localhost', '127.0.0.1', '::1') and a.resend_key:
        raise ConfigError('公网网站需 HTTPS，邮件验证链接也依赖 HTTPS origin')
    _ = u.port
    _ = db.port
    endpoint(a.host, a.port)
    hostname = f'[{u.hostname}]' if ':' in u.hostname else u.hostname
    if not re.fullmatch('[A-Za-z0-9_-]{1,64}', a.node_id):
        raise ConfigError('节点 ID 无效')
    public = a.worker_public_url or (f'wss://{hostname}:8443' if u.scheme == 'https' else 'ws://127.0.0.1:8443')
    secret = a.cluster_secret or secrets.token_hex(32)
    if len(secret.encode()) < 32:
        raise ConfigError('集群密钥至少 32 字节')
    if bool(a.resend_key) != bool(a.mail_from):
        raise ConfigError('Resend Key 和发件地址需同时配置或同时省略')
    values = dict(NODE_ENV='production', HOST=a.host, PORT=str(a.port), DATABASE_URL=database,
                  SANDBOX_CONTROL_ENABLED='true', SANDBOX_SCHEDULING_ENABLED='true',
                  SANDBOX_CLUSTER_SECRET=secret, SANDBOX_SIGNING_SEED_HEX=secrets.token_hex(32),
                  SANDBOX_NODES_JSON=json.dumps([dict(id=a.node_id, label=a.node_id, publicDataUrl=ws_url(public), maxSlots=16)]),
                  CONTENT_DATA_DIR='/var/lib/attackbox-web/content', LAB_DATA_DIR='/var/lib/attackbox-web/labs',
                  CONTENT_CREDENTIALS_KEY=secrets.token_hex(32), QUIZ_ANSWER_HMAC_SECRET=secrets.token_hex(32),
                  RESEND_API_KEY=a.resend_key or '', MAIL_FROM=a.mail_from or '', APP_BASE_URL=origin)
    if any(any(c in v for c in '\n\r\0') for v in values.values()):
        raise ConfigError('配置值不能包含换行')
    atomic(ENV, env_text(values), exclusive=True)
    print('Control 配置已生成，权限 0600；不显示密钥。')
    if u.scheme == 'http' and u.hostname not in ('localhost','127.0.0.1','::1'):
        print('当前为公网 HTTP 初始部署；启用邮件验证前需配置 HTTPS APP_BASE_URL。')


def node_add(a):
    values = read_env()
    items = nodes(values)
    item = dict(id=a.node_id, label=a.label or a.node_id, publicDataUrl=ws_url(a.public_url), maxSlots=a.max_slots)
    if any(n['id'] == a.node_id for n in items):
        raise ConfigError('节点已存在；请编辑现有节点，避免覆盖')
    values['SANDBOX_NODES_JSON'] = json.dumps(items + [item])
    nodes(values)
    atomic(ENV, env_text(values))
    print('节点已加入配置；重启 attackbox-web 后生效。异机 Worker 需使用相同 ID、网关地址和集群密钥。')


def setup_worker(a):
    if WORKER.exists() or WORKER.is_symlink():
        raise ConfigError('Worker 配置已存在，拒绝覆盖')
    local = read_env() if ENV.exists() else None
    identity = a.node_id or (nodes(local)[0]['id'] if local else 'node-primary')
    if not re.fullmatch('[A-Za-z0-9_-]{1,64}',identity):
        raise ConfigError('节点 ID 无效')
    entry = next((n for n in nodes(local) if n['id'] == identity), None) if local else None
    public = ws_url(a.public_url or (entry['publicDataUrl'] if entry else 'ws://127.0.0.1:8443'))
    secret = a.cluster_secret or (local['SANDBOX_CLUSTER_SECRET'] if local else prompt(None, '集群密钥', secret=True))
    if len(secret.encode()) < 32:
        raise ConfigError('集群密钥至少 32 字节')
    control = a.control_url or ('ws://' + endpoint(local.get('HOST','127.0.0.1'),local.get('PORT','3000')) + '/cluster/connect' if local else '')
    u = urlsplit(control)
    if u.scheme not in ('ws', 'wss') or not u.hostname or u.path != '/cluster/connect' or u.query or u.fragment or u.username or u.password:
        raise ConfigError('异机部署必须明确指定 ws(s)://Control/cluster/connect')
    _ = u.port
    config = yaml.safe_load((ROOT / 'deploy/worker/worker.yaml.example').read_text())
    config['node'].update(node_id=identity, cluster_secret=secret, control_url=control)
    config['gateway'].update(listen_addr=a.listen_addr, public_data_url=public)
    if public.startswith('wss:') and not a.tls_terminated:
        if not a.tls_cert or not a.tls_key or not Path(a.tls_cert).is_file() or not Path(a.tls_key).is_file():
            raise ConfigError('WSS 需要现有 --tls-cert 和 --tls-key；由反向代理终止 TLS 时显式使用 --tls-terminated')
        config['gateway'].update(tls_enabled=True, tls_cert=a.tls_cert, tls_key=a.tls_key)
    if a.enable_compose:
        if not a.network_verified:
            raise ConfigError('Compose 必须先完成网络验收，再同时指定 --enable-compose --network-verified')
        config['compose_labs'].update(enabled=True, network_verified=True)
    config['network']['egress_interface'] = a.egress_interface
    worker_health(config)
    if local:
        match_worker(local, config)
    atomic(WORKER, yaml.safe_dump(config, allow_unicode=True, sort_keys=False), exclusive=True)
    print('Worker 配置已生成，权限 0600。')


def database_action(a):
    values = read_env()
    if not values.get('DATABASE_URL'):
        raise ConfigError('缺少数据库连接')
    payload = None
    if a.action == 'admin':
        email = prompt(a.email, '管理员邮箱').strip().lower()
        name = prompt(a.name, '显示名称', 'Administrator').strip()
        password = prompt(a.password, '管理员密码（至少 12 位）', secret=True)
        if a.password is None and password != getpass.getpass('再次输入密码: '):
            raise ConfigError('两次密码不一致')
        payload = json.dumps(dict(email=email, displayName=name, password=password))
    script = 'init-admin.mjs' if a.action == 'admin' else 'apply-migrations.mjs'
    run('node', 'server/scripts/' + script, cwd=ROOT/'web', env={**os.environ, **values}, input=payload, text=True)


def health(url, attempts=30):
    # Local deployment probes must not inherit an outbound HTTP proxy. Verify TLS normally.
    opener = build_opener(ProxyHandler({}), HTTPSHandler(context=ssl.create_default_context()))
    for _ in range(attempts):
        try:
            with opener.open(url, timeout=2) as response:
                if response.status == 200:
                    return
        except (OSError, ValueError):
            pass
        time.sleep(1)
    raise RuntimeError('服务健康检查超时，请检查 systemd 日志')


def prepare_account(values):
    try:
        grp.getgrnam('attackbox')
    except KeyError:
        run('groupadd','--system','attackbox')
    try:
        account = pwd.getpwnam('attackbox')
    except KeyError:
        run('useradd','--system','--gid','attackbox','--home-dir','/var/lib/attackbox-web','--no-create-home','--shell','/usr/sbin/nologin','attackbox')
        account = pwd.getpwnam('attackbox')
    if account.pw_uid == 0:
        raise ConfigError('attackbox 不能是 root')
    gid = grp.getgrnam('attackbox').gr_gid
    for path in ['/var/lib/attackbox-web', values.get('CONTENT_DATA_DIR','/var/lib/attackbox-web/content'), values.get('LAB_DATA_DIR','/var/lib/attackbox-web/labs')]:
        p = Path(path)
        if not p.is_absolute() or p.is_symlink() or not (str(p).startswith('/var/lib/') or str(p).startswith('/srv/')):
            raise ConfigError('Web 持久化目录必须位于 /var/lib 或 /srv，且不能是符号链接')
        p.mkdir(parents=True, exist_ok=True)
        os.chown(p, account.pw_uid, gid)
        os.chmod(p,0o700)


def remember(path):
    if path.is_symlink():
        return ('link',os.readlink(path))
    if path.exists():
        return ('file',path.read_bytes(),path.stat().st_mode & 0o777)
    return None


def restore(path, state):
    if path.exists() or path.is_symlink():
        path.unlink()
    if state:
        if state[0] == 'link':
            path.symlink_to(state[1])
        else:
            atomic(path,state[1].decode(),state[2])


def deploy_control():
    values = read_env()
    upstream = endpoint(values.get('HOST','127.0.0.1'),values.get('PORT','3000'))
    nodes(values)
    if 'NITRO_HOST' in values or 'NITRO_PORT' in values:
        raise ConfigError('请统一使用 HOST 和 PORT，不要另设 NITRO_HOST/NITRO_PORT')
    if WORKER.exists():
        match_worker(values,yaml.safe_load(WORKER.read_text()))
    nginx_config = render_nginx(values, upstream)
    artifact = ROOT/'web/.output'
    if not (artifact/'server/index.mjs').is_file():
        raise ConfigError('先运行 scripts/build.sh control')
    prepare_account(values)
    WEB.parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(dir=WEB.parent,prefix='.attackbox-stage-'))
    previous = WEB.with_name(WEB.name+'.prev')
    config = ETC/'nginx/sites-available/attackbox'
    link = ETC/'nginx/sites-enabled/attackbox'
    default = ETC/'nginx/sites-enabled/default'
    unit = ETC/'systemd/system/attackbox-web.service'
    saved = {p:remember(p) for p in [config,link,default,unit]}
    web_active = subprocess.run(['systemctl','is-active','--quiet','attackbox-web']).returncode == 0
    nginx_active = subprocess.run(['systemctl','is-active','--quiet','nginx']).returncode == 0
    had_web, switched, stopped = WEB.exists(), False, False
    try:
        shutil.copytree(artifact,stage/'.output',symlinks=True)
        # Explicit permissions: deployment user's umask must not make root-owned output unreadable.
        for directory, dirs, files in os.walk(stage,followlinks=False):
            os.chmod(directory,0o755)
            for name in files:
                p=Path(directory)/name
                if not p.is_symlink():
                    os.chmod(p,0o755 if p.stat().st_mode & 0o111 else 0o644)
        atomic(config,nginx_config,0o644)
        restore(link,('link',str(config)))
        restore(default,None)
        atomic(unit,(ROOT/'deploy/control/attackbox-web.service').read_text(),0o644)
        run('nginx','-t')
        run('systemctl','daemon-reload')
        run('systemctl','stop','attackbox-web'); stopped=True
        if had_web:
            if previous.exists():
                shutil.rmtree(previous)
            os.replace(WEB,previous)
        try:
            os.replace(stage,WEB); switched=True
        except BaseException:
            if had_web:
                os.replace(previous,WEB)
            raise
        run('systemctl','enable','attackbox-web','nginx')
        run('systemctl','restart','attackbox-web')
        run('systemctl','start','nginx')
        run('systemctl','reload','nginx')
        health('http://'+upstream+'/')
        print('Control 部署成功；上一版保留在 '+str(previous))
    except BaseException:
        if switched:
            run('systemctl','stop','attackbox-web')
            shutil.rmtree(WEB)
            if had_web:
                os.replace(previous,WEB)
        for p,state in saved.items():
            restore(p,state)
        run('systemctl','daemon-reload')
        if had_web and stopped and web_active:
            run('systemctl','restart','attackbox-web')
        if nginx_active:
            run('systemctl','reload','nginx')
        else:
            run('systemctl','stop','nginx')
        raise
    finally:
        if stage.exists():
            shutil.rmtree(stage)


def deploy_worker():
    cfg = yaml.safe_load(WORKER.read_text())
    url = worker_health(cfg)
    if ENV.exists():
        values = read_env()
        match_worker(values,cfg)
        if values.get('CLOUDFLARE_ENABLED') == 'true':
            render_nginx(values, endpoint(values.get('HOST', '127.0.0.1'), values.get('PORT', '3000')))
    artifact = ROOT/'dist/attackbox-worker'
    if not artifact.is_file():
        raise ConfigError('先运行 scripts/build.sh worker')
    run(artifact,'-check-config','-config',WORKER)
    for p in [cfg.get('runtime',{}).get('instances_dir','/var/lib/attackbox/instances'),cfg['compose_labs']['state_dir']]:
        Path(p).mkdir(parents=True,exist_ok=True,mode=0o700)
    previous = BIN.with_name(BIN.name+'.prev')
    had_binary = BIN.exists()
    if had_binary:
        shutil.copy2(BIN,previous)
    unit = ETC/'systemd/system/attackbox-worker.service'
    old_unit = remember(unit)
    fd,name=tempfile.mkstemp(dir=BIN.parent,prefix='.attackbox-worker-')
    os.close(fd)
    try:
        shutil.copyfile(artifact,name);os.chmod(name,0o755)
        os.replace(name,BIN)
        atomic(unit,(ROOT/'deploy/worker/attackbox-worker.service').read_text(),0o644)
        run('systemctl','daemon-reload')
        run('systemctl','enable','attackbox-worker')
        run('systemctl','restart','attackbox-worker')
        health(url)
        print('Worker 本地网关健康检查通过；还需在 Control 节点管理中确认注册在线。')
    except BaseException:
        run('systemctl','stop','attackbox-worker')
        if had_binary:
            os.replace(previous,BIN)
        else:
            BIN.unlink(missing_ok=True)
        restore(unit,old_unit)
        run('systemctl','daemon-reload')
        if had_binary and old_unit:
            run('systemctl','restart','attackbox-worker')
        raise
    finally:
        Path(name).unlink(missing_ok=True)


def parser():
    p=argparse.ArgumentParser(description=__doc__)
    roles=p.add_subparsers(dest='role',required=True)
    control=roles.add_parser('control').add_subparsers(dest='action',required=True)
    setup=control.add_parser('setup')
    for name in ['database','origin','cluster-secret','worker-public-url','resend-key','mail-from']:
        setup.add_argument('--'+name)
    setup.add_argument('--node-id',default='node-primary')
    setup.add_argument('--host',default='127.0.0.1');setup.add_argument('--port',type=int,default=3000)
    control.add_parser('migrate');admin=control.add_parser('admin')
    for name in ['email','password','name']:
        admin.add_argument('--'+name)
    node=control.add_parser('node').add_subparsers(dest='node_action',required=True).add_parser('add')
    node.add_argument('--node-id',required=True);node.add_argument('--public-url',required=True);node.add_argument('--label');node.add_argument('--max-slots',type=int,choices=range(1,251),default=16)
    for name in ['database','cluster-secret','resend-key']:
        setup.add_argument('--'+name+'-file')
    admin.add_argument('--password-file')
    control.add_parser('deploy')
    worker=roles.add_parser('worker').add_subparsers(dest='action',required=True)
    setup=worker.add_parser('setup')
    for name in ['node-id','control-url','cluster-secret','public-url','tls-cert','tls-key']:
        setup.add_argument('--'+name)
    setup.add_argument('--listen-addr',default='0.0.0.0:8443');setup.add_argument('--egress-interface',default='')
    for name in ['tls-terminated','enable-compose','network-verified']:
        setup.add_argument('--'+name,action='store_true')
    setup.add_argument('--cluster-secret-file')
    worker.add_parser('deploy')
    return p


def secret_files(a):
    for key, path in list(vars(a).items()):
        if key.endswith('_file') and path:
            target=key[:-5]
            if getattr(a,target,None):
                raise ConfigError('密钥值和密钥文件参数不能同时提供')
            p=Path(path)
            if p.is_symlink() or not p.is_file() or p.stat().st_mode & 0o077 or p.stat().st_size>16384:
                raise ConfigError('凭据文件须为普通文件、权限 0600，且不超过 16 KiB')
            setattr(a,target,p.read_text().rstrip('\r\n'))


def dispatch(a):
    secret_files(a)
    if a.role=='control':
        if a.action=='setup':setup_control(a)
        elif a.action=='node':node_add(a)
        elif a.action in ('migrate','admin'):database_action(a)
        else:deploy_control()
    elif a.action=='setup':setup_worker(a)
    else:deploy_worker()


def main():
    a=parser().parse_args()
    if os.geteuid()!=0:
        raise ConfigError('配置和部署需 root；请使用 sudo scripts/control.sh 或 worker.sh。构建不需要 sudo。')
    ENV.parent.mkdir(parents=True,exist_ok=True)
    with open(ENV.parent/'.deploy.lock','a') as lock:
        os.chmod(lock.name,0o600)
        fcntl.flock(lock,fcntl.LOCK_EX)
        dispatch(a)


if __name__=='__main__':
    try:
        main()
    except ConfigError as error:
        print(str(error),file=sys.stderr)
        sys.exit(1)
    except (ValueError,KeyError,OSError,RuntimeError,yaml.YAMLError,subprocess.CalledProcessError):
        # Do not print exceptions containing provider keys or database URLs.
        print('操作失败。请检查参数、两端配置、依赖和 systemd 日志。',file=sys.stderr)
        sys.exit(1)
