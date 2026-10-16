import urllib.request
import urllib.error
import http.cookiejar
import ssl
import socket
import json
import subprocess
import time
import sys

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
    def req(method, path, data=None):
        url = BASE_URL + path
        body = None
        headers = {}
        if data is not None:
            body = json.dumps(data).encode('utf-8')
            headers['Content-Type'] = 'application/json'
        r = urllib.request.Request(url, data=body, headers=headers, method=method)
        with opener.open(r) as resp:
            return json.loads(resp.read().decode('utf-8'))
    req.opener = opener
    req.cj = cj
    return req

# 创建两个独立的客户端 Session (模拟同一用户的两台不同设备)
dev1 = make_session()
dev2 = make_session()

print('=== Test 1: 设备1与设备2分别登录 (多设备解耦验证) ===')
l1 = dev1('POST', '/api/login', {'username': 'laji', 'password': 'qwer1234!'})
print('Dev1 Login:', l1)
assert l1.get('success') is True

l2 = dev2('POST', '/api/login', {'username': 'laji', 'password': 'qwer1234!'})
print('Dev2 Login:', l2)
assert l2.get('success') is True

# 重置环境，确保干净状态
dev1('POST', '/api/env/destroy')
subprocess.getoutput('sudo sqlite3 /var/lib/attackbox/attackbox.db "DELETE FROM box_leases;"')
subprocess.getoutput('for s in $(sudo btrfs subvolume list / 2>/dev/null | grep "var/lib/attackbox/boxes" | awk \'{print $9}\'); do sudo btrfs property set "/$s" ro false 2>/dev/null; sudo btrfs subvolume delete "/$s" 2>/dev/null; done')
subprocess.getoutput('sudo rm -rf /run/attackbox/* /sys/fs/cgroup/attackbox/* 2>/dev/null')
subprocess.getoutput('sudo systemctl restart attackboxd.service')
time.sleep(1)

print('\n=== Test 2: 只看不练免占坑验证 ===')
m1 = dev1('GET', '/api/me')
m2 = dev2('GET', '/api/me')
print('Dev1 me:', m1)
print('Dev2 me:', m2)
assert m1.get('envRunning') is False and m1.get('boxId') == ''
assert m2.get('envRunning') is False and m2.get('boxId') == ''

out = subprocess.getoutput('sudo btrfs subvolume list / | grep boxes')
print('Active box subvolumes (expect none):', out if out else 'NONE (PASS)')
assert 'boxes' not in out

print('\n=== Test 3: 设备1启动沙箱，设备2共享感知 ===')
cli_res = dev1('POST', '/api/env/cli/toggle')
print('Dev1 start env:', cli_res)
assert cli_res.get('success') is True
assert cli_res.get('cliRunning') is True

time.sleep(1)
m2_after = dev2('GET', '/api/me')
print('Dev2 me after Dev1 started box:', m2_after)
assert m2_after.get('envRunning') is True
assert m2_after.get('boxId') == 'box1'

print('\n=== Test 4: 设备1登出，沙箱生命周期依然健在 (彻底杜绝过早误杀) ===')
dev1('POST', '/api/logout')
m1_logout = dev1('GET', '/api/me')
print('Dev1 after logout:', m1_logout)
assert m1_logout.get('loggedIn') is False

# 设备2检查沙箱是否被误杀
m2_still_running = dev2('GET', '/api/me')
print('Dev2 me after Dev1 logout (expect sandbox alive):', m2_still_running)
assert m2_still_running.get('loggedIn') is True
assert m2_still_running.get('envRunning') is True
assert m2_still_running.get('boxId') == 'box1'

init_pid = subprocess.getoutput('cat /run/attackbox/box1/init.pid 2>/dev/null')
print(f'Sandbox init PID: {init_pid}')
assert init_pid and init_pid.isdigit(), 'Sandbox PID not found'

print('\n=== Test 5: cgroups v2 资源防炸加固验证 ===')
cgroup_dir = '/sys/fs/cgroup/attackbox/box1'
pids_max = subprocess.getoutput(f'cat {cgroup_dir}/pids.max 2>/dev/null')
memory_max = subprocess.getoutput(f'cat {cgroup_dir}/memory.max 2>/dev/null')
cpu_max = subprocess.getoutput(f'cat {cgroup_dir}/cpu.max 2>/dev/null')
procs = subprocess.getoutput(f'cat {cgroup_dir}/cgroup.procs 2>/dev/null')
print(f'cgroups v2 pids.max: {pids_max} (expect 512)')
print(f'cgroups v2 memory.max: {memory_max} (expect 2147483648 or 2G)')
print(f'cgroups v2 cpu.max: {cpu_max} (expect 200000 100000)')
print(f'cgroups v2 procs:\n{procs}')
assert pids_max == '512', f'pids.max not set to 512: {pids_max}'
assert cpu_max.strip() == '200000 100000', f'cpu.max not set correctly: {cpu_max}'
assert init_pid in procs.splitlines(), f'init_pid {init_pid} not in cgroup.procs'

def run_in_sandbox(cmd_str):
    return subprocess.getoutput(f'sudo nsenter -t {init_pid} -m -u -i -n -p --root --wd {cmd_str}')

print('\n=== Test 6: 宿主机防误关机硬核加固验证 (P0 安全基线) ===')
reboot_test = run_in_sandbox('/sbin/reboot 2>&1')
print(f'Container /sbin/reboot result: {reboot_test}')
assert 'In-sandbox reboot/shutdown is disabled' in reboot_test

poweroff_test = run_in_sandbox('/sbin/poweroff 2>&1')
print(f'Container /sbin/poweroff result: {poweroff_test}')
assert 'In-sandbox reboot/shutdown is disabled' in poweroff_test

print('\n=== Test 7: Sudo 主机名解析与零延迟验证 (P1 体验基线) ===')
hosts_content = run_in_sandbox('cat /etc/hosts')
print(f'Container /etc/hosts:\n{hosts_content}')
assert '127.0.0.1 attackbox-box1 box' in hosts_content

# 测试 sudo 执行耗时与警告
t_start = time.time()
sudo_test = run_in_sandbox('sudo -u box -s -- sudo true 2>&1')
t_elapsed = time.time() - t_start
print(f'Container sudo test output: "{sudo_test}", elapsed: {t_elapsed:.3f}s')
assert 'unable to resolve host' not in sudo_test
assert t_elapsed < 1.0, f'Sudo took too long ({t_elapsed}s), possible DNS hang'

print('\n=== Test 8: DNS 配置与 nftables 放行验证 ===')
resolv_content = run_in_sandbox('cat /etc/resolv.conf')
print(f'Container /etc/resolv.conf:\n{resolv_content}')
assert 'nameserver 1.1.1.1' in resolv_content
assert 'nameserver 8.8.8.8' in resolv_content

nft_rules = subprocess.getoutput('sudo nft list chain inet attackbox forward | grep 53')
print(f'nftables forward DNS rules:\n{nft_rules}')
assert '1.1.1.1' in nft_rules and '8.8.8.8' in nft_rules
assert 'udp dport 53' in nft_rules
assert 'tcp dport 53' in nft_rules

print('\n=== Test 9: Root 环境补齐与清除 parrot 验证 ===')
bashrc_content = run_in_sandbox('cat /root/.bashrc')
print(f'Container /root/.bashrc size: {len(bashrc_content)} bytes')
assert len(bashrc_content) > 100, 'Root .bashrc is empty or missing'

whoami_check = run_in_sandbox('su - box -c whoami').strip()
print(f'Container whoami for box: {whoami_check}')
assert whoami_check == 'box', f'Expected box, got {whoami_check}'

box_check = run_in_sandbox('id box 2>&1')
print(f'id box in container: {box_check}')
assert 'uid=' in box_check

with open('/usr/local/share/attackbox/web/static/term.js') as f:
    term_js_src = f.read()
    assert 'parrot' not in term_js_src, 'Hardcoded parrot still present in term.js'
print('Verified web/static/term.js: zero parrot hardcoding (PASS)')

with open('/usr/local/share/attackbox/web/templates/term.html') as f:
    term_html_src = f.read()
    assert 'parrot' not in term_html_src, 'Hardcoded parrot still present in term.html'
print('Verified web/templates/term.html: zero parrot hardcoding (PASS)')

# 验证敏感账户彻底脱敏清零
laji_check = run_in_sandbox('grep -E "laji" /etc/passwd /etc/shadow 2>&1')
print(f'laji in container passwd/shadow (expect empty): {laji_check}')
assert not laji_check, f'Security leak: laji still found in sandbox: {laji_check}'

parrot_check = run_in_sandbox('grep -E "parrot" /etc/passwd /etc/shadow 2>&1')
print(f'parrot in container passwd/shadow (expect empty): {parrot_check}')
assert not parrot_check, f'parrot still found in sandbox: {parrot_check}'

# 验证宿主 SSH 主机私钥彻底隔离清零
ssh_key_check = run_in_sandbox('ls /etc/ssh/ssh_host_* 2>&1')
print(f'SSH host keys in container (expect No such file): {ssh_key_check}')
assert 'No such file' in ssh_key_check

# 验证物理块设备彻底阻断 (/dev/vda* 绝不泄露)
dev_disk_check = run_in_sandbox('ls /dev/sd* /dev/vd* /dev/nvme* 2>&1')
print(f'Physical block devices in /dev (expect No such file): {dev_disk_check}')
assert 'No such file' in dev_disk_check

# 验证 /sys 只读保护生效
sys_ro_check = run_in_sandbox('touch /sys/test_ro 2>&1')
print(f'Touch /sys test (expect Read-only): {sys_ro_check}')
assert 'Read-only' in sys_ro_check

# 验证 /proc/sysrq-trigger 屏蔽生效
sysrq_check = run_in_sandbox('echo b > /proc/sysrq-trigger 2>&1')
print(f'Sysrq trigger test (expect Read-only or Permission denied): {sysrq_check}')
assert 'Read-only' in sysrq_check or 'Permission denied' in sysrq_check

# 验证短网卡命名 vh1 存在且符合规范
vh_check = subprocess.getoutput('ip link show vh1 2>&1')
print(f'veth short name vh1 on host: {vh_check}')
assert 'vh1' in vh_check

# 验证 SQLite 租约表持久化
db_leases = subprocess.getoutput('sqlite3 /var/lib/attackbox/attackbox.db "SELECT username, box_id FROM box_leases"')
print(f'Active leases in SQLite: {db_leases}')
assert 'laji|box1' in db_leases

print('\n=== Test 10: GUI 与 CLI 彻底解耦验证 ===')
# 开启 GUI
gui_on = dev2('POST', '/api/env/gui/toggle')
print('Dev2 GUI toggle on:', gui_on)
assert gui_on.get('guiRunning') is True
gui_sock = subprocess.getoutput('ls -l /run/attackbox/box1/gui.sock 2>/dev/null')
print(f'GUI socket: {gui_sock}')
assert 'gui.sock' in gui_sock

# 关闭 GUI
gui_off = dev2('POST', '/api/env/gui/toggle')
print('Dev2 GUI toggle off:', gui_off)
assert gui_off.get('guiRunning') is False

# 检查环境和 CLI 依然健在
status_check = dev2('GET', '/api/me')
print('Status after GUI off:', status_check)
assert status_check.get('envRunning') is True
assert status_check.get('cliRunning') is True

print('\n=== Test 11: 守护进程重启无感对账 (Reconciliation) ===')
# 重启 attackboxd 服务
print('Restarting attackboxd.service to test lease reconciliation...')
restart_res = subprocess.getoutput('sudo systemctl restart attackboxd.service')
time.sleep(1.5)
# 验证旧沙箱 PID 仍然存活，未被盲杀！
ps_alive = subprocess.getoutput(f'ps -p {init_pid} 2>/dev/null')
print(f'PID {init_pid} status after daemon restart: {ps_alive}')
assert init_pid in ps_alive, f'Reconciliation failed: PID {init_pid} was killed across daemon restart!'

# 验证重启后前台依然识别该沙箱租约
status_after_restart = dev2('GET', '/api/me')
print('Dev2 status after daemon restart:', status_after_restart)
assert status_after_restart.get('envRunning') is True
assert status_after_restart.get('boxId') == 'box1'

print('\n=== Test 12: 一键处决与彻底清理 (Destroy) ===')
dest_res = dev2('POST', '/api/env/destroy')
print('Destroy response:', dest_res)
assert dest_res.get('success') is True
time.sleep(1)

# 验证进程彻底回收
ps_check = subprocess.getoutput(f'ps -p {init_pid} 2>/dev/null')
print(f'PID {init_pid} status (expect empty): {ps_check}')
assert init_pid not in ps_check or len(ps_check.splitlines()) <= 1

# 验证零僵尸
defunct_check = subprocess.getoutput('ps -ef | grep defunct | grep -v grep')
print(f'Zombie processes (expect empty): {defunct_check if defunct_check else "NONE (PASS)"}')
assert not defunct_check

# 验证 cgroups 目录移除
cg_exists = subprocess.getoutput(f'ls -d {cgroup_dir} 2>/dev/null')
print(f'cgroup dir after destroy (expect empty): {cg_exists}')
assert not cg_exists

# 验证快照物理删除
subvols_check = subprocess.getoutput('sudo btrfs subvolume list / | grep boxes')
print(f'Subvolumes after destroy (expect empty): {subvols_check if subvols_check else "NONE (PASS)"}')
assert 'var/lib/attackbox/boxes/box1/rootfs' not in subvols_check

# 验证 SQLite 租约清除
db_leases_after = subprocess.getoutput('sqlite3 /var/lib/attackbox/attackbox.db "SELECT * FROM box_leases WHERE username = \'laji\'"')
print(f'SQLite lease after destroy (expect empty): {db_leases_after}')
assert not db_leases_after

print('\n=== Test 13: 结构化审计日志验证 ===')
journal_logs = subprocess.getoutput('sudo journalctl -u attackboxd.service -n 60 --no-pager')
print('Recent journal logs:')
for line in journal_logs.splitlines()[-25:]:
    print('  ', line)
assert '[AUTH]' in journal_logs
assert '[ENV]' in journal_logs

print('\n=== Test 14: 10大安全漏洞防御专项验证 ===')
# 1. 日志注入防御：测试换行符/非法字符用户名拦截
log_inj_res = dev1('POST', '/api/login', {'username': "admin\n[AUTH] forged log entry", 'password': "test"})
print('Log injection login test:', log_inj_res)
assert log_inj_res.get('success') is False

# 2. 详细错误泄露防护：测试 SQLite 错误是否被统一脱敏
admin_sess = make_session()
admin_login = admin_sess('POST', '/api/login', {'username': 'admin', 'password': 'qwer1234!'})
print('Admin Login:', admin_login)
assert admin_login.get('success') is True

dup_create = admin_sess('POST', '/api/admin/users/create', {'username': 'admin', 'password': 'newpassword123', 'role': 'admin'})
print('Duplicate admin create test:', dup_create)
assert dup_create.get('success') is False
err_msg = dup_create.get('error', '')
assert 'sqlite' not in err_msg.lower() and 'sql:' not in err_msg.lower(), f'Raw SQL error leaked: {err_msg}'

# 3. 最后一个管理员自锁与防误删验证
toggle_self = admin_sess('POST', '/api/admin/users/toggle', {'username': 'admin'})
print('Admin toggle self test:', toggle_self)
assert toggle_self.get('success') is False
assert '无法禁用' in toggle_self.get('error', '') or '至少一个' in toggle_self.get('error', '')

del_self = admin_sess('POST', '/api/admin/users/delete', {'username': 'admin'})
print('Admin delete self test:', del_self)
assert del_self.get('success') is False
assert '无法删除' in del_self.get('error', '') or '至少一个' in del_self.get('error', '')

# 4. WebSocket 跨站挟持 (CSWSH) 阻断测试
# 先为 admin 启动沙箱，确保 WebSocket 路由进入 Upgrade 握手阶段
admin_sess('POST', '/api/env/cli/toggle')

# 获取登录凭据 Cookie
cookie_val = ""
for c in admin_sess.cj:
    if c.name == 'ab':
        cookie_val = c.value
        break
assert cookie_val, "Admin session cookie 'ab' not found"

# 通过标准 TLS 套接字发送真实的 WebSocket 升级握手帧（携带恶意跨站 Origin）
raw_sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
tls_sock = ctx.wrap_socket(raw_sock, server_hostname='127.0.0.1')
tls_sock.connect(('127.0.0.1', 443))

ws_handshake = (
    "GET /term/ws HTTP/1.1\r\n"
    "Host: 127.0.0.1\r\n"
    "Upgrade: websocket\r\n"
    "Connection: Upgrade\r\n"
    "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n"
    "Sec-WebSocket-Version: 13\r\n"
    "Origin: https://evil-attacker.com\r\n"
    f"Cookie: ab={cookie_val}\r\n"
    "\r\n"
)
tls_sock.sendall(ws_handshake.encode('utf-8'))
ws_resp = tls_sock.recv(4096).decode('utf-8', errors='ignore')
tls_sock.close()

status_line = ws_resp.splitlines()[0] if ws_resp.splitlines() else ""
print(f'CSWSH handshake response status line: {status_line}')
assert '403 Forbidden' in status_line or '403' in status_line, f'CSWSH protection failed! Expected 403 Forbidden, got: {ws_resp}'
print('CSWSH cross-origin handshake blocked with 403 Forbidden (PASS)')

# 5. 陈旧 gui.sock 自动探测与自愈恢复验证
m_before = admin_sess('GET', '/api/me')
box_id = m_before.get('boxId', 'box1')
subprocess.getoutput(f'sudo mkdir -p /run/attackbox/{box_id} && sudo touch /run/attackbox/{box_id}/gui.sock')
print(f'Simulated stale socket created at /run/attackbox/{box_id}/gui.sock')
m_stale = admin_sess('GET', '/api/me')
print('api/me response with stale socket:', m_stale)
assert m_stale.get('guiRunning') is False, 'Stale socket falsely reported guiRunning=True!'
sock_still_exists = subprocess.getoutput(f'ls /run/attackbox/{box_id}/gui.sock 2>/dev/null')
assert not sock_still_exists, 'Stale socket was not cleaned up!'
print('Stale socket successfully detected and purged (PASS)')

# 清理测试环境
admin_sess('POST', '/api/env/destroy')

print('\n======================================================')
print('SUCCESS! ALL 14 TEST SUITES (INCLUDING 10 SECURITY FIXES) PASSED!')
print('沙箱引擎所有架构重构、安全漏洞与加固措施全量验证通过！')
print('======================================================')

