# 架构设计规范：AttackBox 沙箱引擎重构（Golden Rootfs + 全命名空间）

- **日期**：2026-10-16
- **模块**：`internal/box`, `internal/gateway`, `internal/config`, `cmd/attackboxd`
- **状态**：设计已对齐，待进入实现计划

---

## 1. 背景与问题定义

现有 AttackBox 沙箱引擎存在以下关键架构隐患与 Bug：
1. **快照源污染与宿主机数据泄露**：
   - 当前 `snapshot.go` 直接执行 `btrfs subvolume snapshot / <target>`。
   - 沙箱内快照完整暴露宿主机敏感数据：`/var/lib/attackbox/attackbox.db`（全量用户密码哈希与会话）、`/etc/attackbox/config.json`、`/etc/ssl/novnc/key.pem`、`/root/.ssh` 以及其他并发槽位的运行数据。
2. **宿主机根 Mount Namespace 严重污染**：
   - `manager.go` 直接在宿主机上对每个槽位的 `rootfs` 执行 8 次挂载操作（`--rbind /dev`、`proc`、`sys`、`tmpfs`、`runDir`）。
   - 宿主机全局挂载表极其脏污，进程退出后残留大量孤儿挂载点。
3. **跨租户互杀致命 Bug**：
   - `manager.go:Kick()` 销毁沙箱时调用宿主机全局命令：`pkill -u parrot`。
   - 所有沙箱用户统一使用 `parrot` 账号，导致销毁 `box1` 时会无差别处决 `box2`、`box3`、`box4` 正在运行的渗透任务与桌面会话。
4. **命名空间隔离不完整**：
   - 现仅依靠 `ip netns` 隔离网络，配合 `chroot`。缺少 PID/UTS/IPC/Mount 命名空间隔离。
   - 沙箱内 `ps aux` 可直接窥探宿主机包括 `attackboxd` 在内的全部进程。

---

## 2. 目标与非目标

### 目标 (Goals)
1. **数据源物理脱敏**：停止对 `/` 快照，建立基准子卷 `/var/lib/attackbox/base_rootfs`，剥除一切宿主私密后设置为只读，所有沙箱均从基准子卷秒级派生。
2. **Linux 原生全命名空间隔离**：每个沙箱具备独立的 Mount、PID、Network、UTS、IPC 命名空间，宿主进程对沙箱 100% 不可见。
3. **宿主挂载零污染**：沙箱所有的虚拟设备树（`/dev`、`proc`、`sys`、`home` 等）挂载全部限制在私有 Mount Namespace 内，宿主机挂载表无任何残留。
4. **统一注入 (nsenter)**：CLI 终端与 GUI 桌面统一通过 `nsenter` 注入沙箱环境，完全替代脆弱的裸 `chroot`。
5. **内核级一键销毁 (`Kick`)**：彻底废除 `pkill -u parrot` 与 `umount -l`，直接向沙箱 PID 1 发送 `kill -9`，利用 Linux 内核 `zap_pid_ns_processes` 机制瞬间清理本沙箱所有子进程及私有挂载，秒级删除快照。

### 非目标 (Non-Goals)
- 引入 Docker / containerd / runc 外部容器运行时（AttackBox 维持 0 外部重型依赖的极简纯 Go 原生架构）。
- 修改现有的前端 Web 交互与 WebSocket 通信协议（对外 API 完全向下兼容）。

---

## 3. 详细架构设计

### 3.1 步骤一：基准子卷管理 (`base_rootfs`)

#### 3.1.1 自动初始化逻辑
在 `internal/box/snapshot.go` 中新增 `EnsureBaseRootfs(baseRootfsPath string) error`：
- 在 `cmd/attackboxd/main.go` 启动阶段调用该函数。
- 逻辑：
  1. 检查 `baseRootfsPath` 是否已存在。若存在，直接跳过。
  2. 若不存在，确保上级目录 `/var/lib/attackbox` 存在。
  3. 执行一次性宿主系统快照：
     `btrfs subvolume snapshot / /var/lib/attackbox/base_rootfs`
  4. 执行脱敏与文件清洗：
     - 清除宿主私密：`rm -rf <base>/var/lib/attackbox`、`<base>/etc/attackbox`、`<base>/etc/ssl/novnc`
     - 清除 Root 密钥与历史：`rm -rf <base>/root/*`、`<base>/root/.*`
     - 清除宿主用户目录：`rm -rf <base>/home/*`
     - 清理临时文件与日志：`rm -rf <base>/tmp/*`、`<base>/var/tmp/*`、`<base>/var/log/*`
     - 预建沙箱必要目录并纠正权限：
       `mkdir -p <base>/run/attackbox` (0777)
       `mkdir -p <base>/home/parrot` (0755, chown parrot:parrot)
       `chmod 1777 <base>/tmp`
       `mkdir -p <base>/tmp/.X11-unix && chmod 1777 <base>/tmp/.X11-unix`
  5. 锁定只读防护：
     `btrfs property set /var/lib/attackbox/base_rootfs ro true`

#### 3.1.2 快照派生与销毁
- `SnapshotManager.Create(slot string)`：
  `btrfs subvolume snapshot /var/lib/attackbox/base_rootfs <target>`
  *(btrfs 从只读子卷派生默认生成可读写的目标子卷)*
- `SnapshotManager.Destroy(slot string)`：
  `btrfs subvolume delete <target>`

---

### 3.2 步骤二：常驻 PID 1 守护进程 (`unshare` + `tini`)

在 `internal/box/manager.go:startCore` 中实现沙箱核心拉起：

1. **网络配置**：
   维持原有的 `ip netns add ab-<slot>` 及网桥 `br0` / `veth` 对分配。
2. **拉起 unshare 守护进程**：
   ```bash
   ip netns exec ab-<slot> unshare \
     --mount \
     --uts \
     --ipc \
     --pid \
     --fork \
     --kill-child \
     /bin/bash -c '<init_script>'
   ```
3. **私有挂载与 tini 容器标准 Init 逻辑 (`<init_script>`)**：
   ```bash
   # 1. 挂载命名空间彻底私有化，禁止反向传播到宿主机
   mount --make-rprivate /

   # 2. 挂载虚拟文件系统与设备树
   mount --bind "$ROOTFS" "$ROOTFS"
   mount --make-rprivate "$ROOTFS"
   mount --rbind /dev "$ROOTFS/dev"
   mount --make-rprivate "$ROOTFS/dev"
   mount -t devpts devpts "$ROOTFS/dev/pts" -o rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=666
   mount -t tmpfs tmpfs "$ROOTFS/dev/shm" -o rw,nosuid,nodev,mode=1777
   chmod 666 "$ROOTFS/dev/ptmx" 2>/dev/null || true

   # 挂载属于本沙箱独立 PID namespace 的 proc 与 sys
   mount -t proc proc "$ROOTFS/proc"
   mount -t sysfs sys "$ROOTFS/sys"

   # 宿主机-沙箱通信套接字目录
   mount --bind "$RUNDIR" "$ROOTFS/run/attackbox"

   # 独立 tmpfs home 空间
   mount -t tmpfs tmpfs "$ROOTFS/home" -o rw,nosuid,nodev,mode=0755
   mkdir -p "$ROOTFS/home/parrot"
   if [ -d /home/parrot ]; then
     cp -a /home/parrot/. "$ROOTFS/home/parrot/" 2>/dev/null || true
   else
     cp -r /etc/skel/. "$ROOTFS/home/parrot/" 2>/dev/null || true
   fi
   chown -R parrot:parrot "$ROOTFS/home/parrot"
   chmod 0755 "$ROOTFS/home/parrot"

   # 3. chroot 并以容器工业标准 tini / dumb-init 作为 PID 1 长期驻留
   exec chroot "$ROOTFS" /bin/bash -c "
     hostname attackbox-${BOX_ID}
     touch /run/attackbox/core.ready

     # 优先寻找容器工业标准 init: tini 或 dumb-init
     INIT_BIN=\"\"
     for b in /usr/bin/tini /bin/tini /usr/bin/dumb-init /bin/dumb-init; do
       if [ -x \"\$b\" ]; then
         INIT_BIN=\"\$b\"
         break
       fi
     done

     if [ -n \"\$INIT_BIN\" ]; then
       # 工业级标准：tini 负责高效收割孤儿僵尸进程与信号分发
       exec \"\$INIT_BIN\" -s -- sleep infinity
     else
       # 优雅防御回退：若未预装 tini，自动降级为原生 Shell 收割循环
       set -m
       trap 'while wait -n 2>/dev/null; do :; done' CHLD
       while true; do
         sleep 86400 &
         wait \$! 2>/dev/null || true
       done
     fi
   "
   ```
4. **获取并固化 PID 1 宿主 PID**：
   `cmd.Start()` 后得到 `unshare` 在宿主机的 PID（存入 `unshare.pid`）。
   通过 `pgrep -P <unsharePID>` 或读取 `/proc/<unsharePID>/task/<unsharePID>/children` 获取其派生子进程（即在沙箱内充当 PID 1 的 `tini` 进程）在宿主机的真实 PID，保存至 `/run/attackbox/<slot>/init.pid`。

---

### 3.3 步骤三：CLI 与 GUI 统一通过 `nsenter` 注入

#### 3.3.1 CLI 终端注入 (`websocket.go:termWS`)
- 废弃原 `ip netns exec ... chroot ...`。
- 读取 `init.pid`，通过 `nsenter` 同时穿透 5 大命名空间并切换根目录：
  ```go
  cmd := exec.Command("/usr/bin/nsenter",
      "--target", strconv.Itoa(initPID),
      "--mount", "--uts", "--ipc", "--net", "--pid",
      "--root", "--wd",
      "/usr/bin/su", "-", "parrot")
  cmd.Env = []string{
      "TERM=xterm-256color",
      "LANG=en_US.UTF-8",
  }
  ```
- 接入现有 PTY 双向桥接。

#### 3.3.2 GUI 桌面注入 (`manager.go:startGUI`)
- 启动 GUI 桌面会话：
  ```go
  cmd := exec.Command("/usr/bin/nsenter",
      "--target", strconv.Itoa(initPID),
      "--mount", "--uts", "--ipc", "--net", "--pid",
      "--root", "--wd",
      "/bin/bash", "-c", guiScript)
  ```
- TigerVNC 输出 socket 直接落地到 `/run/attackbox/gui.sock`，宿主机通过 bind-mount 的 `b.RunDir/gui.sock` 正常代理。
- `StopGUI` 操作：使用 `nsenter` 在沙箱 PID Namespace 内部执行 `pkill -f Xtigervnc` 和 `pkill -f mate-session`，完全不干扰其他槽位。

---

### 3.4 步骤四：极简销毁逻辑 (`Kick`)

在 `manager.go:Kick` 中彻底去除原有 7 个 `umount -l` 和全局 `pkill -u parrot`：
1. **停止 GUI（若运行）**：
   调用局部清理，停止本槽位 GUI 进程。
2. **终结 PID 1**：
   读取 `init.pid` 与 `unshare.pid`：
   ```go
   if initPID > 0 {
       _ = exec.CommandContext(ctx, "kill", "-9", strconv.Itoa(initPID)).Run()
   }
   if unsharePID > 0 {
       _ = exec.CommandContext(ctx, "kill", "-9", strconv.Itoa(unsharePID)).Run()
   }
   ```
   **内核机制**：内核收到 PID 1 的 `SIGKILL` 后，自动以 `SIGKILL` 杀死本 PID Namespace 内的一切子孙进程。当全部进程死亡后，私有 Mount Namespace 自动销毁释放。
3. **销毁网络资源**：
   `ip netns del ab-<slot>`
   `ip link del veth-<slot>-br`
4. **物理删除快照**：
   `m.Snapshots.Destroy(b.ID)`（此时没有任何挂载或进程占用，绝对不会触发 busy 错误）。
5. **清理运行时标记**：
   删除 `init.pid`、`unshare.pid`、`core.ready`、`gui.sock`。

---

## 4. 影响评估与兼容性

| 影响面 | 评估结论 |
| :--- | :--- |
| **外部 REST API** | 完全兼容，`/api/env/*`、`/api/me`、`/api/admin/*` 无需任何变动。 |
| **前端页面与 WebSocket** | 完全兼容，xterm.js 与 noVNC 对接逻辑无需修改。 |
| **配置文件 `config.json`** | 新增可选字段 `"baseRootfs": "/var/lib/attackbox/base_rootfs"`，具备安全默认值。 |
| **性能表现** | 快照派生与 `unshare` 启动保持在 100ms 级别；`Kick()` 销毁由多次同步 umount 转为一次性 kill，耗时大幅降低。 |

---

## 5. 验证标准

1. **宿主隔离性验证**：
   进入沙箱后查看 `/var/lib/attackbox` 与 `/etc/attackbox`，确认不存在或无宿主敏感文件；运行 `ps aux` 确认只能看到沙箱内部进程。
2. **宿主挂载表干净度验证**：
   启动多个沙箱后，宿主机运行 `grep -i attackbox /proc/mounts`，确认宿主机无任何泄露的沙箱绑定挂载点。
3. **跨租户互杀规避验证**：
   同时启动 `box1` 与 `box2`，在两边同时运行 `parrot` 用户的耗时任务；执行 `box1` 销毁，验证 `box2` 的进程与终端完全不受干扰。
4. **快照干净销毁验证**：
   执行 `Kick()` 后，检查 `btrfs subvolume list /`，确认瞬态快照被干净删除且无报错。
