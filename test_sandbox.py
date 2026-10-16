import urllib.request
import urllib.error
import http.cookiejar
import ssl
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
subprocess.getoutput('for s in $(sudo btrfs subvolume list / 2>/dev/null | grep "var/lib/attackbox/boxes" | awk \'{print $9}\'); do sudo btrfs property set "/$s" ro false 2>/dev/null; sudo btrfs subvolume delete "/$s" 2>/dev/null; done')
subprocess.getoutput('sudo rm -rf /run/attackbox/* /sys/fs/cgroup/attackbox/* 2>/dev/null')
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
procs = subprocess.getoutput(f'cat {cgroup_dir}/cgroup.procs 2>/dev/null')
print(f'cgroups v2 pids.max: {pids_max} (expect 512)')
print(f'cgroups v2 memory.max: {memory_max} (expect 4294967296 or max)')
print(f'cgroups v2 procs:\n{procs}')
assert pids_max == '512', f'pids.max not set to 512: {pids_max}'
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
assert '127.0.0.1 attackbox-box1 box1' in hosts_content

# 测试 sudo 执行耗时与警告
t_start = time.time()
sudo_test = run_in_sandbox('sudo -u box1 -s -- sudo true 2>&1')
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

whoami_check = run_in_sandbox('su - box1 -c whoami').strip()
print(f'Container whoami for box1: {whoami_check}')
assert whoami_check == 'box1', f'Expected box1, got {whoami_check}'

box1_check = run_in_sandbox('id box1 2>&1')
print(f'id box1 in container: {box1_check}')
assert 'uid=' in box1_check

with open('/usr/local/share/attackbox/web/static/term.js') as f:
    term_js_src = f.read()
    assert 'parrot' not in term_js_src, 'Hardcoded parrot still present in term.js'
print('Verified web/static/term.js: zero parrot hardcoding (PASS)')

with open('/usr/local/share/attackbox/web/templates/term.html') as f:
    term_html_src = f.read()
    assert 'parrot' not in term_html_src, 'Hardcoded parrot still present in term.html'
print('Verified web/templates/term.html: zero parrot hardcoding (PASS)')

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

print('\n=== Test 11: 一键处决与彻底清理 (Destroy) ===')
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

print('\n=== Test 12: 结构化审计日志验证 ===')
journal_logs = subprocess.getoutput('sudo journalctl -u attackboxd.service -n 60 --no-pager')
print('Recent journal logs:')
for line in journal_logs.splitlines()[-25:]:
    print('  ', line)
assert '[AUTH]' in journal_logs
assert '[ENV]' in journal_logs
assert '[GUI]' in journal_logs

print('\n======================================================')
print('SUCCESS! ALL 12 TEST SUITES PASSED FLAWLESSLY!')
print('沙箱引擎所有架构重构、安全加固及用户体验缺陷全量验证通过！')
print('======================================================')
