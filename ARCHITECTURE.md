# AttackBox 系统架构规划书 (System Architecture Specification)

## 架构核心定位：纯分布式攻防靶场集群 (Distributed Cyber Range Cluster)

AttackBox 是一套专为渗透测试人员、CTF 战队和网络安全教学打造的分布式攻防靶场基础设施。系统全面采用 **“控制中心 (Control Hub) 与 牺牲品算力节点 (Sacrificial Worker Nodes)”** 的物理与逻辑彻底分离模型。

* **控制端 (Control Server)**: 部署于独立的受保护宿主机，作为系统集中管理大脑，负责 Web 控制台交付、用户认证、积分账单、算力调度与机密管理。**Control 节点永远不运行任何不受信的沙箱容器，保持绝对安全。**
* **算力节点 (Worker Agent)**: 分布式部署于一组独立的云服务器或物理宿主机上，定位为**“可牺牲的纯算力耗材 (Sacrificial Compute)”**。Worker 节点本地零数据库、无控制监听端口、不存储任何系统机密，全面放行特权调试（ptrace）、剥离关机能力（Drop CAP_SYS_BOOT），严防宿主级逃逸以守护多租户底线隔离。被打崩或受污染时，通过底层云快照秒级回滚自愈。

---

## 目录索引 (Table of Contents)

* **第 1 章：系统分布式拓扑与 C/S 通信模型**
  * 1.1 分布式集群拓扑与节点角色定位
  * 1.2 牺牲品节点安全哲学 (Sacrificial Node Philosophy)
  * 1.3 双通道解耦机制 (反向长连接控制信道 vs 经 CF 边缘直连 Worker 数据信道)
* **第 2 章：Worker 沙箱核心运行时与特权放行规范**
  * 2.1 Btrfs 原生瞬态子卷快照与毫秒级派生机制
  * 2.2 Linux Namespaces 隔离、/proc/self/exe 自引导与 CLONE_INTO_CGROUP 机制
  * 2.3 物理资源配额与防 DoS 约束 (Cgroups v2 内存/进程/IO 限额)
  * 2.4 沙箱特权放行与多租户边界约束 (ptrace / CAP_SYS_BOOT 剥离 / 明确禁止宿主逃逸)
  * 2.5 开发与调试模式专项规范 (Dev & Debug Mode)
* **第 3 章：双模交互引擎实现规范**
  * 3.1 PTY 远程命令行 (CLI) 桥接方案与自适应 Resize
  * 3.2 TigerVNC 远程桌面 (GUI) 按需加载与 Unix 套接字直通
* **第 4 章：沙箱双网桥与靶网架构**
  * 4.1 双网桥拓扑与寻址规范 (`br0` 与 `docker0`)
  * 4.2 靶场流量打通与沙箱二层端口隔离 (Bridge Port Isolation 防 ARP 欺骗与横向嗅探)
  * 4.3 WAN 出口白名单与控制面防回环规则
  * 4.4 靶标模板引擎与前端 Web 可视化管控 (Docker vs MicroVM 动态调度)
  * 4.5 预置靶场策略模板库与一键配置 (Web测试 / Pwn / 内网横向 / 逃逸)
* **第 5 章：Control 凭据认证、状态模型与安全追溯**
  * 5.1 数据持久化定位与轻量存储原则
  * 5.2 基于 Ed25519 的客户端无状态 JWT 鉴权与即时踢下线
  * 5.3 邀请激励与计费幂等游标设计
  * 5.4 攻防靶场安全责任追溯机制
* **第 6 章：分布式控制调度、数据流接入与会话生命周期**
  * 6.1 集中调度与算力负载均衡 (加权最小连接)
  * 6.2 Ed25519 跨机能力凭证 (Ticket) 与 Worker 零 DB 离线验签
  * 6.3 交互会话并发控制与状态保活 (CLI/GUI 统一单活跃抢占机制)
  * 6.4 租约与空闲自动回收 (Hard TTL 120m, Idle 10m)
  * 6.5 节点心跳遥测、熔断与离线状态对齐 (Reconciliation)
* **第 7 章：节点全生命周期管理与 Web 前端管控**
  * 7.1 节点生命周期状态机 (Online, Draining, Offline, Resetting)
  * 7.2 Web 前端管控大盘与 Cluster Secret 节点接入规范
  * 7.3 安全策略与 Feature Flags 动态热下发 (免重启 Worker)
  * 7.4 跨节点沙箱穿透监管与孤儿快照一键清理
  * 7.5 联动云厂商 / 虚拟化 API 的整机快照秒级回滚与自愈 (Cloud Reset)
* **第 8 章：系统配置与凭据隔离**
  * 8.1 静态配置文件规范
  * 8.2 核心机密单向隔离与无状态 Worker

---

## 第 1 章：系统分布式拓扑与 C/S 通信模型

```
                               ┌────────────────────────────────┐
                               │       Control 管理中心节点       │
                               │  (独立物理机 / 云 VPS, 端口:443)  │
                               │                                │
                               │ • 独占全局持久化 SQLite 库       │
                               │ • 集中托管用户、邀请与积分账单   │
                               │ • 静态金库: API Key / 签名私钥  │
                               │ • Web 交付与反向控制中心 (:443)  │
                               │ • 集中算力调度与 Web 控制台交付   │
                               └───────────────┬────────────────┘
                                               ▲
                            跨公网 WSS 反向控制信道 (带保活与心跳)
                                               │
             ┌─────────────────────────────────┼─────────────────────────────────┐
             │                                 │                                 │
             │                                 │                                 │
             ▼                                 ▼                                 ▼
  ┌───────────────────────┐         ┌───────────────────────┐         ┌───────────────────────┐
  │   Worker 算力节点 1    │         │   Worker 算力节点 2    │         │   Worker 算力节点 N    │
  │   (Sacrificial Node)  │         │   (Sacrificial Node)  │         │   (Sacrificial Node)  │
  │                       │         │                       │         │                       │
  │ • 零 DB / 零用户数据   │         │ • 零 DB / 零用户数据   │         │ • 零 DB / 零用户数据   │
  │ • 仅开数据端口 :8443  │         │ • 仅开数据端口 :8443  │         │ • 仅开数据端口 :8443  │
  │ • 放行 ptrace / 特权  │         │ • 放行 ptrace / 特权  │         │ • 放行 ptrace / 特权  │
  │ • OverlayFS + PTY/VNC │         │ • OverlayFS + PTY/VNC │         │ • OverlayFS + PTY/VNC │
  │ • 本地离线验签 Ticket  │         │ • 本地离线验签 Ticket  │         │ • 本地离线验签 Ticket  │
  └───────────┬───────────┘         └───────────┬───────────┘         └───────────┬───────────┘
              ▲                                 ▲                                 ▲
              │                                 │ 浏览器经 CF 边缘直连 Worker      │
              │                                 │ (不经 Control 中转, :8443)       │
              └─────────────────────────────────┼─────────────────────────────────┘
                                                │
                                      ┌─────────┴─────────┐
                                      │   用户浏览器终端   │
                                      │ (xterm.js / noVNC)│
                                      └───────────────────┘
```

### 1.1 分布式集群拓扑与节点角色定位
系统严格划分为两端：
* **Control 服务端 (Control Hub)**:
  * **角色定位**: 集中式有状态管理中心（Stateful Management Hub）。
  * **职责范围**:
    1. 独占 SQLite 数据库，全生命周期托管所有用户数据、登录鉴权、邀请归属、积分消费明细与操作审计日志。
    2. 集中保管系统敏感机密（Resend 邮件 Key、OAuth Client Secret、Turnstile 密钥及 Ed25519 签名私钥）。
    3. 统一交付管理员 Web 控制台与用户交互界面。
    4. 维持全集群 Worker 反向连接池，统筹全局算力负载均衡与实例调度。
    5. 支持节点全生命周期监控、策略下发与一键云快照重建。
* **Worker 客户端 (Worker Agent / Node)**:
  * **角色定位**: 分布式无状态牺牲品执行器（Stateless Managed Executor）。
  * **职责范围**:
    1. 100% 遵从 Control 下发的调度指令，负责本地 OverlayFS 实例挂载/销毁、网桥接入、Cgroups 配额限制与沙箱生命周期管理。
    2. 负责 PTY 伪终端主从对管理、TigerVNC 桌面按需拉起及本地 Unix Domain Socket 数据流桥接。
    3. 本地持有从 Control 同步的 Ed25519 公钥，进行纯内存极速离线验签，核销 JTI 防重放。

### 1.2 牺牲品节点安全哲学 (Sacrificial Node Philosophy)
将算力节点定位为“牺牲品（Sacrificial Node）”是本架构的核心安全基石：
1. **物理隔离保护大脑**: Control 永远不与沙箱共处于同一台机器。即使沙箱被利用打穿至宿主机 root 甚至污染内核，受害范围被严格限制在该单一 Worker 物理机/虚拟机内，Control 的用户数据、数据库和系统 API Key 毫发无损。
2. **彻底解脱防御枷锁**: 不再需要单机模式下繁杂且有损攻防体验的宿主敏感文件抹除、磁盘屏蔽或系统调用封禁。Worker 是纯净可消耗的算力容器。
3. **出厂级瞬时恢复**: 一旦 Worker 被恶意篡改或测试损坏，管理员通过 Web 界面一键触发云底座 API 执行整机纯净快照回滚重建，秒级回归初始状态。

### 1.3 双通道解耦机制 (Control Channel vs Data Channel)
* **控制信道 (Control Channel - 反向长连接)**:
  * Worker 启动后作为客户端，主动通过 WebSocket (`wss://control.example.com/api/agent/connect`) 反向连接 Control 中心。
  * **安全性**: Worker 节点无需对外开放任何管理或 SSH 端口，天然免疫来自公网的直接扫描与暴破。
  * **承载流量**: 节点注册、周期心跳遥测、沙箱调度创建/销毁指令、安全策略与 Feature Flags 动态热下发。
* **数据信道 (Data Channel - 浏览器经 CF 边缘直连 Worker，不经 Control 中转)**:
  * 各 Worker 仅开放单一公网端口 `:8443`。
  * 用户浏览器从 Control 取得合法的临时访问 Ticket 后，经 Cloudflare 边缘节点直接与沙箱所在的 Worker `:8443` 建立 WebSocket 直连，全程不经 Control 中转。
  * **网络吞吐解放**: 海量高并发的 CLI 字符流与 VNC RFB 桌面图像流由 Cloudflare 边缘直接路由至各个 Worker 分散消化，Control 节点的网络带宽绝不成为系统瓶颈。

---

## 第 2 章：Worker 沙箱核心运行时与特权放行规范

### 2.1 Btrfs 原生瞬态子卷快照与毫秒级派生机制
* **核心选型考量 (为什么坚持 btrfs 原生快照而拒绝 OverlayFS)**:
  1. **杜绝双重 CoW 损耗**: 在本身就是写时复制（CoW）的 btrfs 宿主上叠挂一层 OverlayFS 会导致双重 CoW 与双重元数据寻址，拖慢 I/O 吞吐并产生大量磁盘碎片。
  2. **100% 纯正 POSIX 语义与黑客工具兼容**: OverlayFS 存在跨层目录重命名报错 `EXDEV`、inode 动态变化等非标准 POSIX 行为；而 btrfs subvolume 快照在内核中是一个原生的标准根文件系统，GDB、Wireshark、Metasploit、Ghidra 等底层攻防工具零兼容报错。
  3. **零物理磁盘冗余**: 利用 btrfs 底层 extent 引用计数，直接秒级从母机只读模板快照克隆，**创建耗时 `< 1ms`，物理磁盘占用为 0 字节**。
* **实现原理与分层拓扑**:
  1. **只读基准脱敏金盘 (Golden Base - `base_rootfs`)**:
     Worker 启动时检查 `/var/lib/attackbox/base_rootfs`；若不存在，一次性从宿主机根目录 `/` 派生基准子卷，深度清洗宿主机敏感信息（清除 SSH 密钥、宿主账户、日志与运行时文件），并强制锁定为物理只读防篡改模板：
     ```bash
     btrfs property set /var/lib/attackbox/base_rootfs ro true
     ```
  2. **毫秒级可写瞬态子卷派生 (Transient Snapshot)**:
     调度器分配沙箱时，Worker 秒级从只读基准子卷派生独立的可写子卷：
     ```bash
     btrfs subvolume snapshot /var/lib/attackbox/base_rootfs /var/lib/attackbox/instances/<sandbox_id>/rootfs
     ```
  3. **生命周期与严格注销流水线 (安全销毁防卡死原则)**:
     btrfs 本身性能极佳，历史卡死 100% 来源于未按序清理挂载与进程。新架构严格执行**逆序销毁流水线**：
     - **第 1 步 (处决进程)**: 写入 `/sys/fs/cgroup/attackbox/<id>/cgroup.kill` 触发内核原子刹停沙箱内所有进程，并向 PID 1 广播 `SIGKILL`；
     - **第 2 步 (等待退出)**: 轮询确认沙箱进程树完全清空，所有打开的文件句柄彻底释放；
     - **第 3 步 (注销网络)**: 清理网络命名空间与 veth 虚拟网卡；
     - **第 4 步 (物理销毁)**: 执行 `btrfs subvolume delete /var/lib/attackbox/instances/<sandbox_id>/rootfs`，此时子卷上无任何挂载与打开句柄，删除耗时 `< 1ms`，彻底根除卡死；
     - **第 5 步 (清理控制组)**: 移除该沙箱的 Cgroups 目录。

### 2.2 Linux Namespaces 隔离、/proc/self/exe 自引导与 CLONE_INTO_CGROUP 机制

#### 1. 核心隔离维度与非特权映射取舍
沙箱启动时强制启用以下五大核心命名空间：
* `CLONE_NEWPID`: 进程隔离，沙箱内无法观测宿主机进程，内部首进程 PID 为 1。
* `CLONE_NEWNS`: 挂载点隔离，通过 `pivot_root` 彻底将实例 rootfs 替换为根目录，隔离宿主文件系统。
* `CLONE_NEWUTS`: 隔离主机名，沙箱内拥有独立的主机标识。
* `CLONE_NEWIPC`: 隔离 IPC 消息队列与共享内存。
* `CLONE_NEWNET`: 隔离网络命名空间 (`/var/run/netns/<sandbox_id>`)。

* **明确拒绝 User Namespace (`CLONE_NEWUSER`)**:
  在牺牲品 Worker 模型下，系统显式**不使用** User Namespace：
  1. **工具链真实特权**: Raw Socket 抓包、低端口监听、Wireshark、Nmap SYN 扫描需要真实的内核 Capabilities。在 User Namespace 伪造 root 会导致大量底层黑客工具因权限虚假而报错。
  2. **避免 UID 重映射开销**: 消除 `shiftfs` / `idmapped mounts` 的复杂配置与性能损耗。
  3. **安全模型转移**: 安全性通过独立的 Worker 节点物理边界与云快照自愈来保障，而非牺牲工具的可用性。

#### 2. Go 运行时 M:N 调度陷阱与外部 Shell 性能深坑
在 Go 语言中实现轻量级命名空间隔离，必须直面两大经典致命深坑：
1. **`syscall.Setns` 导致 Go 运行时线程污染 (M:N Scheduler Trap)**:
   * Go 运行时的 Goroutine（G）在多个操作系统物理线程（M）上动态调度流转。
   * 若直接在 Go 宿主主进程内调用 `syscall.Setns` 切换到沙箱命名空间，当前系统线程（M）的命名空间被改变，但该线程随后会被 Go 调度器复用去执行其他无关的 Goroutine（如 WebSocket 监听、Control 心跳上报、RPC 消息分发）。
   * **灾难后果**: 宿主机的核心 Goroutine 意外在沙箱的 Namespace 内运行，导致心跳断开、宿主文件被污染、网络通信瘫痪。即便使用 `runtime.LockOSThread()`，在复杂的 Cgo 与标准库调用中依然极度脆弱。
2. **外部 Shell 封装脚本的性能与安全劣势**:
   * 若每次操作都通过 `os/exec.Command("sh", "-c", "ip netns exec ... chroot ...")` 调外部脚本，每次创建沙箱或打开终端都会带来数十毫秒的进程分叉开销、管道通信损耗以及字符串拼接注入风险。

#### 3. 终极解法：单二进制 `/proc/self/exe` re-exec 极速自引导引擎
参考容器运行时工业级最佳实践，Worker 单二进制文件内置隐藏子命令引导逻辑（如 `worker __init__ <sandbox_id>` 与 `worker __enter__ <sandbox_id>`），彻底实现零外部依赖、亚毫秒级瞬时启动：

```
┌────────────────────────────────────────────────────────┐
│ Worker 主服务进程 (多线程 Go Runtime / 网络服务 / 心跳)      │
└───────────────────────────┬────────────────────────────┘
                            │
                            │ 1. 创建 cgroup v2 目录并配置配额
                            │ 2. os/exec.Command("/proc/self/exe", "__init__", sbx_id)
                            │ 3. SysProcAttr: Cloneflags + CgroupFD (物理进入目标 cgroup)
                            ▼
┌────────────────────────────────────────────────────────┐
│ 沙箱内置 PID 1 首进程 (单二进制自引导常驻守护进程)            │
│  • 执行 unix.Mount / pivot_root 切换至 btrfs snapshot  │
│  • 挂载私有 /proc, 只读 /sys, 私有 /dev tmpfs           │
│  • unix.Prctl(PR_CAPBSET_DROP) 剔除高危能力集           │
│  • 锚定 PID 命名空间，死循环轮询 waitpid 自动收割僵尸进程 │
│  • 绝不因单个 Bash 或终端退出而意外导致沙箱整体销毁        │
└────────────────────────────────────────────────────────┘
```

##### A. 主进程一步到位派生 (`CLONE_INTO_CGROUP`)
主进程创建沙箱时，先在 Cgroups v2 树中建立好目录并打开文件句柄，直接通过标准系统调用属性一步拉起隔离子进程：
```go
cgroupFile, err := os.Open(cgroupPath)
if err != nil {
    return err
}
defer cgroupFile.Close()

cmd := exec.Command("/proc/self/exe", "__init__", sandboxID)
cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWNET,
    // 利用 Linux 5.7+ 原生 CLONE_INTO_CGROUP 特性:
    CgroupFD:   int(cgroupFile.Fd()),
}
```
* **消除竞态与瞬时炸弹 (No Fork-Bomb Race Window)**:
  * 传统做法是“先 Fork 进程，拿到 PID 后再写 `/sys/fs/cgroup/.../cgroup.procs`”，存在数十微秒到毫秒级的时间窗。若恶意代码在被加入 cgroup 前高频 fork，即可瞬间绕过配额限制瘫痪宿主机。
  * `CLONE_INTO_CGROUP` 保证子进程在**诞生的第一个 CPU 时钟周期内就物理处于目标 Cgroup 内部**，配额与进程数限制即时生效，杜绝任何时间窗漏洞。

##### B. 子进程单线程初始化流水线与 PID 1 常驻收割 (`__init__`)
子进程在进入复杂逻辑之前，运行于单线程初始上下文中，按序完成底层隔离加固：
1. **挂载私有化**: `unix.Mount("", "/", "", unix.MS_REC|unix.MS_SLAVE, "")`，切断挂载事件向宿主机传播。
2. **根目录切换**: 将 btrfs 快照目录执行 `unix.PivotRoot(rootfsDir, oldRootDir)`，随后卸载原宿主根目录并脱钩。
3. **安全挂载**:
   * 挂载实例私有 `/proc` (`mount -t proc proc /proc`)。
   * **只读挂载** `/sys` 及 `/sys/fs/cgroup` (`MS_RDONLY`)，封死 `cgroup release_agent` 等逃逸路径。
   * 挂载私有 `tmpfs` 到 `/dev`，仅创建必要的设备节点（`null`, `zero`, `random`, `urandom`, `ptmx`），不挂入任何宿主机裸磁盘。
   * 挂载私有 `tmpfs` 到 `/tmp` 和 `/run`，吸收高频临时写入。
4. **能力集剥离**:
   * 调用 `unix.Prctl(unix.PR_CAPBSET_DROP, uintptr(cap), 0, 0, 0)` 从 Bounding Set 中强制剥离 `CAP_SYS_BOOT`、`CAP_SYS_MODULE`、`CAP_SYS_RAWIO`。
5. **PID 1 命名空间锚定与僵尸进程收割**:
   * Linux PID 命名空间内若 PID 1 退出，内核将强制向该命名空间内的所有进程发送 `SIGKILL` 并销毁该命名空间。
   * 为确保多终端会话（多 PTY / TigerVNC / SSH）稳定共存且任意终端退出不会击垮沙箱，`/proc/self/exe __init__` 作为 PID 1 长期驻留，建立信号监听与僵尸进程收割循环：
   ```go
   for {
       var status unix.WaitStatus
       pid, err := unix.Wait4(-1, &status, unix.WNOHANG, nil)
       if err != nil || pid <= 0 {
           time.Sleep(100 * time.Millisecond)
           continue
       }
   }
   ```
   * 彻底免除外部 `tini` / `systemd` 依赖，内存占用仅数十 KB。

##### C. 终端 PTY 与桌面动态切入 (`__enter__`)
当用户通过 Web 终端新建连接时，Worker 派生 `/proc/self/exe __enter__ <sandbox_id> [cmd...]`：
- 子进程在独立单线程中打开沙箱对应的 `/proc/<sbx_pid>/ns/*` 执行 `setns`，直接进入沙箱网络与命名空间环境。
- 在 PTY 从设备上拉起 Bash，彻底杜绝污染 Worker 主进程的多线程 Go 运行时。

### 2.3 物理资源配额与防 DoS 约束 (Cgroups v2 + I/O 限制 + 磁盘水位熔断)
沙箱拥有真实 root 特权，为防止单个租户通过 Fork 炸弹、耗尽内存或写满宿主机磁盘导致节点全盘崩溃（Noisy Neighbor / Disk DoS），必须对 CPU、内存、进程数及磁盘 I/O 执行多维硬限制：

* **进程数限制 (Cgroups v2)**:
  ```bash
  echo "512" > /sys/fs/cgroup/attackbox/<sandbox_id>/pids.max
  ```
  彻底免疫沙箱内误操作或恶意触发的 Fork 炸弹。
* **物理内存与 Swap 硬限制 (Cgroups v2)**:
  ```bash
  echo "2G" > /sys/fs/cgroup/attackbox/<sandbox_id>/memory.max
  echo "0"  > /sys/fs/cgroup/attackbox/<sandbox_id>/memory.swap.max
  ```
  硬性限制单实例最大使用 2GB 物理内存，且禁止使用 Swap，防止内存泄漏导致宿主机磁盘 Thrashing 假死。
* **CPU 调度权重 (Cgroups v2)**:
  ```bash
  echo "100" > /sys/fs/cgroup/attackbox/<sandbox_id>/cpu.weight
  ```
  实行公平轮询调度，杜绝单个实例霸占物理 CPU。
* **磁盘写入速率与防 DoS 约束 (Cgroups v2 `io.max` + 水位监控熔断)**:
  * **选型依据 (为什么弃用 btrfs qgroup 与 loop XFS)**:
    - `btrfs qgroup` 存在高频写与快照删除时的内核自旋锁与元数据卡死缺陷，禁止启用。
    - 在已有 btrfs 分区上挂载 loopback XFS 虚拟磁盘会产生严重存储碎片并预先霸占数十 GB 宝贵磁盘空间。
  * **Cgroups v2 `io.max` 速率硬压制**:
    对每个沙箱的写入带宽施加硬上限（如限制写带宽最大 20MB/s，写 IOPS 最大 500）：
    ```bash
    # 获取宿主机根设备 major:minor (如 254:0)
    echo "254:0 wbps=20971520 wiops=500" > /sys/fs/cgroup/attackbox/<sandbox_id>/io.max
    ```
    即使沙箱内恶意执行 `dd if=/dev/zero of=/largefile`，写入速度被强制压制在低位，无法瞬时冲垮物理磁盘 I/O。
  * **内存临时挂载吸收高频临时写入 (`tmpfs`)**:
    沙箱内部的 `/tmp` 和 `/run` 强制挂载为 `tmpfs`（硬限额 512MB），临时工具文件和解压过程只驻留内存，完全不产生物理磁盘 I/O。
  * **宿主机全局磁盘水位探针与紧急熔断机制 (Disk Watermark Guard)**:
    Worker 节点启动后台监控协程，每 3 秒检测一次宿主机 btrfs 根分区的真实空闲空间：
    1. **告警预警水位 (Free Space < 10GB)**: Worker 向 Control 上报健康警告，调度器将新请求引流至其他 Worker。
    2. **熔断切断水位 (Free Space < 5GB)**: Worker 立即触发本地熔断保护：
       - 拒接所有新沙箱创建请求；
       - 定位当前物理写入量异常偏高的沙箱实例，记录审计告警并安全终止其实例；
       - 彻底保障宿主机根分区永不触发 `ENOSPC` 崩溃。
    3. **瞬态沙箱自动回收**: 用户下线或会话超时后，btrfs snapshot 逆序秒级删除，所有脏数据空间即刻全额归还宿主机。

### 2.4 沙箱特权放行与多租户边界约束规范

在分布式牺牲品 Worker 架构下，沙箱既要保障渗透测试工具的深度特权，又要维护多租户共存的底线隔离，遵循“放行攻击能力、限制宿主关机、剥离嵌套容器、严禁宿主逃逸”四原则：

1. **放行 `ptrace` 深度调试特权**:
   * **必要性**: 渗透人员与逆向学员需要使用 GDB 动态调试二进制 Exploit、分析内存布局及执行进程注入。
   * **实现**: 不启用 Seccomp 拦截 `ptrace`，沙箱内 root 进程拥有完整的调试与跟踪能力。

2. **剥离 `CAP_SYS_BOOT` 严禁关机与重启**:
   * **背景**: 用户在沙箱内具有 root 权限，可能出于误操作或恶意执行 `shutdown -h now`、`reboot`、`poweroff` 或 `halt`，导致沙箱进程异常退出或发出无意义的系统调用。
   * **内核级阻断**: Worker 在使用 `capsh` 或 Go 运行时装配沙箱进程能力集（Capabilities）时，强制剔除 `CAP_SYS_BOOT`：
     ```go
     // 剔除系统重启与关机特权
     caps.Drop(capability.CAP_SYS_BOOT)
     ```
   * **防御效果**: 沙箱内执行任何关机/重启命令，内核均直接返回 `Operation not permitted (EPERM)`，保障实例生命周期完全由 Control 会话调度器托管。

3. **明确禁止沙箱内运行 Docker (No Docker-in-Sandbox)**:
   * **禁止理由**:
     1. **资源与复杂度冗余**: 沙箱内拉起完整的 dockerd 需要套娃式 cgroups、虚拟存储驱动（vfs/overlayfs）和嵌套网络，极度消耗 Worker 内存与 IO。
     2. **攻防心智模型解耦**: 学员的沙箱定位为**攻击机（Attacker Station）**，靶机（Target）由管理员在宿主机 `docker0` 网段统一编排和下发，学员无需也不应该在攻击机内部拉起靶机。
   * **实现机制**: 沙箱基准只读母盘（`base_rootfs`）中不启动 `dockerd.service`，且严禁将宿主机的 `/var/run/docker.sock` 挂载入沙箱。

4. **多租户隔离与逃逸边界：普通节点严禁逃逸，KVM 节点专供逃逸 (Escape Boundary)**:
   * **普通 Docker 节点严格禁逃逸（保全多租户底线）**:
     * 在普通云服务器或未开启硬件虚拟化的 Worker 节点上，多个学员共享同一个宿主内核。
     * 若在同机靶机上开放逃逸漏洞（如 `--privileged` 或挂载 `docker.sock`），一旦被逃逸至宿主机 root，攻击者即可篡改其他学员的沙箱、偷窥流量与 Flag，导致所有的 Cgroups 和能力限制全部归零。
     * 因此在普通节点上，**严禁部署任何能突破容器边界的靶标，提权考核严格收敛在靶机 Docker 内部（`www-data` -> 容器内 `root`）**。
   * **KVM 算力节点专供硬件级逃逸演练**:
     * 仅在支持硬件嵌套虚拟化（`/dev/kvm`）的专用 Worker 节点上，系统才允许派发逃逸类靶标。
     * 该靶标通过硬件级微虚拟机（如 Docker Kata 运行时）承载，逃逸破坏被物理锁死在独立虚拟客机内部，绝对无法穿透 CPU 硬件指令集触碰 Worker 宿主机，以此兼顾真实的逃逸教学与多租户底线安全。

5. **防无脑逃逸三道内核级底线限制 (Anti-Escape Baselines)**:
   针对拥有真实 root 的沙箱，必须以极小的开销堵死绝大多数已知的开箱即用逃逸手段：
   * **关键内核接口只读挂载 (Masked & Read-Only Paths)**: `/proc/sys` 与 `/sys`（特别是 `/sys/fs/cgroup`）强制挂载为**只读（ro）**，直接从根源封死经典的 `cgroup release_agent` 逃逸手法，同时阻断用户向内核写参数导致整机崩溃。
   * **隔离设备节点 (Minimal /dev)**: 沙箱内的 `/dev` 强制使用独立的私有 `tmpfs`，严禁挂入宿主机真实的裸磁盘设备（如 `/dev/sd*`、`/dev/vd*`、`/dev/nvme*`），仅按需挂入网络与伪设备（`null`, `zero`, `random`, `urandom`, `ptmx`, `pts/*`, `net/tun`），彻底杜绝直接 `dd` 宿主机磁盘。
   * **高危 Capabilities 追加剥离**: 除 `CAP_SYS_BOOT` 外，装配时强制追加剔除 `CAP_SYS_MODULE`（严禁插内核模块/Rootkit）与 `CAP_SYS_RAWIO`（严禁直接读写宿主物理内存 `/dev/mem` 和硬件 I/O 端口）。常规黑客工具（`nmap`, `wireshark`, `gdb`）不受任何影响。

### 2.5 开发与调试模式专项规范 (Dev & Debug Mode)
系统支持通过环境变量 `ATTACKBOX_ENV=dev` 或启动参数 `--debug` 进入开发调试模式。该模式专为单机开发、联调与本地测试设计，具有以下两大核心规范：

1. **沙箱绝对禁止导入 Worker 工作目录 (Strict Workspace Isolation)**:
   * **痛点背景**: 开发者通常直接在宿主机的项目源码目录（如 `/opt/attackbox` 或本地 git 仓库）下编译并运行 Worker 进程。
   * **强制隔离规则**:
     * 无论开发模式还是生产模式，新建沙箱快照时，**严禁将 Worker 所在的工作目录（Cwd）、源码文件、编译二进制以及 `.git` / 配置文件导入或暴露给沙箱**。
     * Worker 在装配沙箱挂载命名空间时，检测自身的运行目录与代码路径，若位于快照路径范围内，强制通过 `Mount Masking` 挂载只读空 `tmpfs` 进行彻底遮蔽覆盖：
       ```go
       // 强制遮蔽 Worker 自身源码与工作空间
       maskPath(sandboxRootfs, workerWorkingDir)
       maskPath(sandboxRootfs, filepath.Dir(os.Args[0]))
       ```
     * **防御效果**: 沙箱内即使用户获得 root 权限，也绝不可能触碰或读取到正在开发中的系统源码与测试凭证。

2. **凭据非安全与全透明降级 (Insecure Credentials & Mocking)**:
   * 在调试模式下，系统主动牺牲凭据安全性以换取开发便利性：
     * **固定静态开发密钥**: Ed25519 签名私钥采用硬编码的测试密钥，避免每次重启服务重新生成密钥导致前端 JWT 和 Ticket 全部失效。
     * **明文日志全打印**: Control 与 Worker 在标准输出（stdout）直接打印签发和解析的 JWT、Ticket 及明文 Claims，方便使用 `curl`、`websocat` 或 Postman 单独模拟客户端进行断点调试。
     * **第三方服务 Mock 化**: 彻底跳过 Cloudflare Turnstile 人机验证；邮箱验证码直接锁定为静态值（如 `123456`）或仅打印在终端日志中，无需配置真实的外部邮件发送凭据。
     * **允许明文通道**: 允许完全关闭 TLS，纯走本地 `http://` 与 `ws://` 极速排查通信协议故障。

---

## 第 3 章：双模交互引擎实现规范

### 3.1 PTY 远程命令行 (CLI) 桥接方案与自适应 Resize
* **实现基石**: 基于 Linux 原生伪终端（PTY）主从设备机制（`/dev/ptmx` 与 `/dev/pts/*`）。
* **进程进入与数据桥接流水线**:
  1. Worker 接收到建立 CLI 终端连接的 WebSocket 握手。
  2. Worker 进程通过 `creack/pty` 库打开一个宿主 PTY 主从对 (`ptmx, pts`)。
  3. 执行 `nsenter` 将子进程移入目标沙箱的 Namespaces，并将从设备（Slave PTS）重定向作为沙箱内 Shell 的 `Stdin / Stdout / Stderr`。
  4. 将启动的 Shell 进程 PID 写入该沙箱的 Cgroup (`cgroup.procs`)。
  5. 后台协程监听 `ptmx` 输出，实时以 WebSocket 二进制帧推流给前端浏览器（xterm.js）；同时将前端击键字符流写入 `ptmx`。
* **窗口动态自适应 (Resize)**:
  前端 xterm.js 窗口缩放时向 WebSocket 投递 JSON 控制帧（如 `{"type":"resize","cols":120,"rows":40}`），Worker 解析后调用底层的 `pty.Setsize` 触发沙箱内核的 `SIGWINCH` 信号，使终端布局自适应。

### 3.2 TigerVNC 远程桌面 (GUI) 按需加载与 Unix 套接字直通
* **核心选型**: 采用轻量稳定的 **TigerVNC (`Xvnc`)**。
* **惰性冷启动 (Lazy Loading)**:
  * 用户仅打开 CLI 终端时，系统绝不拉起桌面进程，节省约 300MB+ 内存开销。
  * 仅当用户在 Web 控制台点击“打开桌面”时，Worker 调用 `EnsureGUI(sandbox_id)` 按需拉起桌面环境。
* **Unix Domain Socket 直通**:
  1. 在沙箱内部启动 TigerVNC 服务，**严格禁止绑定 TCP 端口**，强制仅监听本地 Unix Domain Socket：
     ```bash
     Xvnc :1 -geometry 1280x800 -depth 24 -rfbunixpath /run/attackbox/<sandbox_id>/gui.sock -SecurityTypes None ...
     ```
  2. 宿主机 Worker 探针检测到套接字就绪后，直接将用户的 WebSocket 二进制流全双工桥接到该本地 Unix Domain Socket，前端通过 noVNC 渲染画面。
  3. **零网络端口暴露**: 全宿主机对外没有暴露任何 VNC TCP 端口（如 5901），彻底杜绝桌面端口未授权扫描与爆破。

---

## 第 4 章：沙箱双网桥与靶网架构

```
                 WAN (互联网)
                     ▲
                     │ SNAT MASQUERADE (仅 53, 80, 443)
                     │
            ┌────────┴────────┐
            │   宿主机物理网卡  │
            └────────┬────────┘
                     │
       ┌─────────────┴─────────────┐
       │ (宿主机内核路由转发启用)    │
       ▼                           ▼
┌──────────────┐            ┌──────────────┐
│  br0 网桥    │◄──────────►│ docker0 网桥 │
│ 10.200.0.1/16│ 双向流量互通 │ 172.17.0.1/16│
└──────┬───────┘            └──────┬───────┘
       │                           │
       ├─► 沙箱 A (10.200.0.2)      ├─► 靶机容器 1 (172.17.0.2)
       ├─► 沙箱 B (10.200.0.3)      ├─► 靶机容器 2 (172.17.0.3)
       └─► ...                     └─► ...
```

### 4.1 双网桥拓扑与寻址规范 (`br0` 与 `docker0`)
Worker 节点内部构建两大私有虚拟网桥（常驻），并支持 Docker 按需动态扩展：
1. **沙箱专用网桥 (`br0`)**:
   * 网段规划：`10.200.0.0/16`，宿主网关：`10.200.0.1`。
   * 每个沙箱分配一对 `veth` 虚拟以太网卡对，宿主端接入 `br0`，沙箱端命名为 `eth0` 并分配独立 IP。
2. **靶机专用网桥 (`docker0` 及动态网桥)**:
   * **标准单层靶场**: 采用标准 Docker 默认网桥网段：`172.17.0.0/16`，网关：`172.17.0.1`。
   * **多层内网穿透靶场**: **宿主机无需提前手动创建新网桥**。由 Docker 在启动靶标时（或通过 Compose）按需动态创建自定义网络（如 `dmz_net`, `internal_net`，Docker 会自动在内核生成对应的 `br-<id>` 网桥），随靶机销毁自动回收释放。

### 4.2 靶场流量打通与沙箱二层端口隔离 (Bridge Port Isolation)
* **双向打靶流量放行 (L3 Routing)**:
  宿主机开启内核 IPv4 转发 (`net.ipv4.ip_forward = 1`)，并在 `FORWARD` 链配置放行规则：
  ```bash
  iptables -A FORWARD -i br0 -o docker0 -j ACCEPT
  iptables -A FORWARD -i docker0 -o br0 -j ACCEPT
  ```
  沙箱内渗透测试工具可无阻碍对靶机容器发起攻击，靶机上的反弹 Shell 或漏洞回连 payload 也可精准回连至沙箱 `10.200.0.x`。
* **二层端口绝对隔离 (Bridge Port Isolation - 防 ARP 欺骗与横向嗅探)**:
  在多租户攻防环境下，沙箱内用户具备 `root` 权限。若不对二层通信进行约束，恶意或好奇用户只需执行一行 `arpspoof` 或 `ettercap` 即可劫持网关实施中间人攻击，瘫痪整个节点。
  **零开销内核级解决方案**:
  Worker 在将沙箱宿主端虚拟网卡 `veth_sbx_<id>` 挂载至 `br0` 网桥时，强制开启 Linux 原生**网桥端口隔离标志位 (Bridge Port Isolation)**：
  ```bash
  # 命令行等价指令:
  bridge link set dev veth_sbx_<id> isolated on

  # Go netlink 代码级直接配置 (零外部命令依赖):
  # link := &netlink.Veth{...}
  # netlink.LinkSetBridgeSlave(link, bridge)
  # netlink.LinkSetIsolated(link, true)
  ```
  * **内核行为约束**:
    1. **禁止横向直通**: 被标记为 `isolated on` 的端口发出的任何二层数据帧（ARP 广播、单播、广播等），**内核绝对禁止转发给同样标记为 `isolated on` 的其他沙箱端口**。
    2. **仅允许上行网关**: 数据包只能在 `veth_sbx_*` 与非隔离端口（即网关 `br0` 本身）之间传递。
    3. **杜绝攻击面**: 彻底免疫跨沙箱 ARP 投毒、中间人窃听、DHCP 假冒及广播风暴，同时保持零 CPU 开销与零 iptables 规则膨胀。
* **二层防御兜底规则 (Defense-in-Depth)**:
  在网桥转发层增加兜底拦截（可通过 nftables 或 ebtables）：
  ```bash
  # 禁止流量从沙箱 veth 进、又从其他沙箱 veth 出:
  ebtables -A FORWARD -i veth+ -o veth+ -j DROP
  # 或 nftables bridge 家族等价规则:
  # nft add rule bridge filter forward iifname "veth*" oifname "veth*" drop
  ```

### 4.3 WAN 出口白名单与控制面防回环规则
* **外网访问严格白名单**:
  为防止沙箱被滥用于对外发起 DDoS 攻击或挖矿，沙箱向外网（WAN）发起的出站流量通过 nftables/iptables 进行严格限制：
  * **仅放行三大核心端口**: DNS (`UDP/TCP 53`)、HTTP (`TCP 80`)、HTTPS (`TCP 443`)，保障渗透测试人员可正常执行 `apt-get`、`git clone`、`pip install` 或更新字典。
  * 其余所有外网端口一律拦截并丢弃。
* **阻断宿主及 Control 控制信道回环**:
  严格阻断沙箱访问 Worker 本地环回地址（`127.0.0.1/8`），并阻断沙箱向 Control 管理端口发起内部探测。

### 4.4 靶标模板引擎与前端 Web 可视化管控 (Target Template Engine & Web Control)

为彻底摆脱管理员登录服务器手工敲命令行启动靶机的落后运维方式，系统引入**“声明式模板规范（Lab Manifest）”**与**“双轨运行时自适应调度”**，全流程由前端 Web 控制台一体化管控：

```
                ┌─────────────────────────────────┐
                │      管理员 Web 控制台           │
                │ (在线编辑 YAML / 一键下发靶机)    │
                └────────────────┬────────────────┘
                                 │ HTTP / REST API
                                 ▼
                ┌─────────────────────────────────┐
                │       Control 集中管理中心       │
                │  (模板仓库 + 智能节点能力调度)    │
                └────────────────┬────────────────┘
                                 │ RPC 下发 (控制长连接信道)
          ┌──────────────────────┴──────────────────────┐
          ▼                                             ▼
┌──────────────────────────────┐              ┌──────────────────────────────┐
│  Worker A (普通 VPS, 无 KVM)  │              │ Worker B (嵌套虚拟化, 含 KVM) │
│  Capabilities: has_kvm=false │              │ Capabilities: has_kvm=true   │
├──────────────────────────────┤              ├──────────────────────────────┤
│ 运行时: runtime: runc (默认)  │              │ 运行时: runtime: kata        │
│  - 纯容器靶场 (Web/Pwn/内网)   │              │  - Docker Kata 硬件微虚拟机  │
│  - 严格安全容器基线          │              │  - 硬件级 VMX 隔离保障多租户 │
└──────────────────────────────┘              └──────────────────────────────┘
```

#### 1. 靶标声明式模板规范 (YAML Lab Manifest)
靶标不再散乱，全部以声明式 YAML 文件统一保存在 Control 节点的模板目录中（`/etc/attackbox/templates/*.yaml`），Web 前端支持在线读取、编辑与保存，物理落盘永不丢失，不依赖数据库表：
* **普通标准业务靶机 (默认 runc 容器)**:
  ```yaml
  apiVersion: attackbox.io/v1
  kind: TargetTemplate
  metadata:
    id: "web-sqli-lab"
    name: "OWASP SQL 注入实战靶场"
    category: "web"
  spec:
    runtime: "runc" # 默认标准 Docker 容器
    image: "vuln-targets/sqli-app:v1"
    resources:
      memory_mb: 256
      cpu_weight: 100
    network:
      bridge: "docker0" # 挂接在标准靶网，IP 由 Docker IPAM 动态分配
      exposed_ports: [80]
    flag:
      type: "dynamic" # 支持会话级动态 Flag 注入
      env_key: "GZCTF_FLAG"
  ```
* **高危/逃逸演练靶机 (基于 Docker Kata 运行时，需 KVM)**:
  ```yaml
  apiVersion: attackbox.io/v1
  kind: TargetTemplate
  metadata:
    id: "escape-runc-lab"
    name: "CVE-2024-21626 容器逃逸专练"
    category: "escape"
  spec:
    runtime: "kata" # 统一复用 Docker 官方 Kata 运行时 (基于 KVM 硬件隔离)
    image: "vuln-targets/escape-runc:v1" # 仍采用标准 Docker 镜像，无需维护 ext4/内核文件
    requirements:
      kvm: true # 调度强约束：必须调度至具备硬件虚拟化的 Worker
    network:
      bridge: "docker0" # IP 同样由 Docker IPAM 动态分配
  ```
  > **多租户防冲突与动态 IPAM 规范**:
  > 1. **严禁在模板中写死固定 IP (`fixed_ip`)**: 靶机 IP 统一交由 Docker 自带的 IPAM 动态分配（如分配 `172.17.0.5`、`172.17.0.6`）。Worker 创建容器后从 Docker API 提取实际分配的 IP 回传给 Control，前端实时向学员展示对应的靶机独立 IP。
  > 2. **容器唯一命名规范**: 容器启动时强制附加沙箱前缀：`tgt_<sandbox_id>_<template_id>`（如 `tgt_sbx9a1_sqli`），天然避免容器同名冲突。
  > 3. **架构极简红利**: Worker 内部的靶标管理器 `worker/target/` **100% 仅调用标准 Docker Engine API**。无论是普通容器（`runc`）还是逃逸微虚拟机（`kata`），镜像拉取、网络网桥分配、容器生命周期全由 Docker 自身完成，**严禁在 Go 里自行编写代码管理 QEMU 进程、vmlinux 内核文件或 TAP 网卡**，彻底消除过度设计。

#### 2. Worker 硬件能力自省与自适应上报 (Capability Telemetry)
* Worker 节点启动并与 Control 建立反向长连接时，主动探查宿主机底层能力：
  * 检测 `/dev/kvm` 设备文件是否存在且具备 `rw` 权限。
  * 检测本地 `docker.service` 运行状态。
  * 提取物理 CPU 核心数与当前可用物理内存。
* Worker 在握手包中主动上报能力字典：
  ```json
  {
    "node_id": "worker-sg-01",
    "capabilities": {
      "has_overlayfs": true,
      "has_kvm": true,
      "has_docker": true,
      "cpu_cores": 8,
      "memory_free_mb": 12400
    }
  }
  ```

#### 3. Control 智能调度与前端 Web 统一管控
1. **模板库统一管理**: 管理员在 Web 控制台拥有“靶标管理”大盘，可在线创建、在线编辑、上传 YAML 或从官方模板库一键导入。
2. **自适应调度与能力感知约束**:
   * 当管理员在前端点击“启动靶机”时，Control 解析模板的 `spec.runtime`：
     * 若为 `runc`（或默认）：根据节点内存与负载，调度至任意在线 Worker 节点拉起标准 Docker 容器。
     * 若为 `kata`：Control 调度器**强制过滤仅保留 `has_kvm: true` 的 Worker 节点**，通过 Docker 的 Kata 运行时拉起硬件级隔离微虚拟机。若当前集群无可用 KVM 节点，前端立即提示“当前无支持硬件虚拟化的算力节点”，杜绝错误派发导致的节点异常。
3. **可视化全生命周期管理**:
   * 前端大盘实时展示各 Worker 上运行的靶机状态、靶机 IP（如 `172.17.0.10`）、健康状况与运行时长。
   * 支持管理员在前端对任一靶标执行“一键重启”、“重置状态”、“修改环境变量/Flag”及“一键销毁”。

### 4.5 预置靶场策略模板库与一键配置 (Preset Policy Profiles)

为大幅降低管理员配置门槛，前端 Web 控制台提供预置的**场景化策略模板库（Policy Profiles）**。管理员在创建靶机时，无需手动拼装底层参数，只需在前端下拉框选择策略，系统自动套用最佳实践：

```
                ┌──────────────────────────────────────────────┐
                │        前端策略预设 (一键快速选用)              │
                ├──────────────────────────────────────────────┤
                │ [●] 1. Web 渗透基准策略 (默认: 容器内提权即可)  │
                │ [ ] 2. Pwn 二进制与逆向调试策略               │
                │ [ ] 3. 内网横向与穿透打洞策略                 │
                │ [ ] 4. 容器逃逸与底层内核高危策略 (需 KVM)     │
                └──────────────────────────────────────────────┘
```

#### 1. Web 渗透测试基准策略 (`profile: web-pentest` - 推荐默认)
* **适用场景**: Web 漏洞利用（SQLi, RCE, 文件上传, 反序列化）、内网信息搜集、常规 CTF Web 题目。
* **攻防边界标准**:
  * 靶机采用**标准 Docker 容器**部署于 `docker0` 网段（`172.17.0.x`）。
  * **容器严格基线**: 严禁特权（`--privileged=false`）、严禁挂载宿主机套接字 (`docker.sock`)、严禁暴露宿主机系统目录。
  * **考核终点**: 学员从应用低权限账号（如 `www-data`）利用提权漏洞（SUID 二进制、sudo 规则错配、本地提权 PoC）提升为**靶机 Docker 内部的 `root`**，读取 `/root/flag.txt` 即判定通关。**不设置逃逸点，也严禁向宿主机逃逸**。

#### 2. 二进制漏洞与逆向策略 (`profile: pwn-re`)
* **适用场景**: 栈溢出、堆利用、格式化字符串、逆向工程分析。
* **预置特性**:
  * 沙箱端保障 `ptrace` 权限生效，预置 GDB、pwntools、pwndbg 环境。
  * 靶机支持自定义 ASLR 开关（通过容器启动参数控制），开放特定端口暴露受害二进制服务（如通过 `socat` 监听）。

#### 3. 内网横向与多级穿透策略 (`profile: intranet-pivoting`)
* **适用场景**: 真实企业内网模拟、内网穿透（FRP / Chisel / EW）、域渗透与跨网段路由跳板。
* **预置特性**:
  * 自动分配双网段（DMZ 区 `172.17.1.0/24` + 核心内网区 `172.17.2.0/24`）。
  * 边界跳板机配置双虚拟网卡，沙箱允许建立 SOCKS5 代理隧道实现多级跳板打靶。

#### 4. 容器逃逸专练策略 (`profile: container-escape` - 需 KVM)
* **适用场景**: runC CVE 逃逸、特权容器挂载利用、底层内核提权教学。
* **预置特性**:
  * Control 调度器自动将该策略路由至拥有 `has_kvm: true` 的 Worker 算力节点。
  * 后端通过 Docker 的 Kata 运行时拉起轻量微虚拟机（依然是标准 Docker 镜像），逃逸破坏被硬件物理锁死在虚拟客机中，保障多租户物理安全且 Worker 零额外管理代码。

---

## 第 5 章：Control 凭据认证、状态模型与安全追溯

### 5.1 数据持久化定位与轻量存储原则
Control 节点独占一个极简的 SQLite 数据库（`attackbox.db`），强制启用 WAL 模式并发优化：
```
sqlite3://attackbox.db?_journal_mode=WAL&_busy_timeout=5000
```
本架构对数据持久化实行**极简与轻量约束原则**，去除过度设计的陈旧建模，具体底层库表在项目功能完全跑通并定型后再行固化：
1. **只持久化高价值凭据与资产**：数据库仅收敛托管用户认证核心身份、密码哈希与邀请归属。第三方服务 API Key 等静态机密严格收敛于配置文件中，物理不落库。
2. **沙箱状态纯内存管理 (In-Memory State)**：沙箱实例定位为瞬态消耗品（Ephemeral），当前谁在运行、运行于哪个节点，完全由 Control 内存状态机与 Worker 实时心跳同步对齐，用完即焚，不持久化繁琐的历史打靶过程。

### 5.2 基于 Ed25519 的客户端无状态 JWT 鉴权与即时踢下线
* **彻底取消 Session 表**: 消除对数据库 Session 记录的高频写入与过期锁竞争。
* **Ed25519 签名复用**: JWT 直接复用 Control 内存中的 **Ed25519 签名私钥**，公钥验签。
* **JWT Claims 核心载荷**:
  * `uid`: 用户自增 ID。
  * `username`: 用户名。
  * `role`: 角色（`admin` 或 `user`）。
  * `ver`: 令牌版本号（用于改密或封禁时即时失效）。
  * `exp`: 过期时间（默认 24 小时）。
* **即时失效机制**:
  当管理员封禁用户或用户修改密码时，服务端仅需对用户令牌版本号递增（`token_version + 1`），中间件拦截比对版本号，旧 JWT 立即失效，兼顾客户端无状态的轻量与主动踢下线的安全。

### 5.3 邀请激励与计费幂等游标设计
1. **邀请关系与奖励**: 新用户注册时绑定专属邀请码（`invite_code`），系统记录邀请人归属并触发拉新赠分。
2. **计费幂等游标 (Idempotency Cursor)**:
   * 状态可以纯内存，但资产与扣费绝不走纯内存。为防止 Control 重启、网络重试或定时计费协程重复执行导致的错扣，系统引入**幂等游标机制**。
   * 每次计费抵扣均绑定唯一业务游标（如 `sandbox_id:hour_N`），扣减通过底层原子事务提交，冲突即拦截跳过，从数学上彻底杜绝重启与重试导致的重复扣费。

### 5.4 攻防靶场安全责任追溯机制
在具备真实 root 特权的攻防靶场中，安全溯源是底线保障，系统确立四维防护边界：
1. **Identification (标识)**: 用户分配全局唯一 `uid`；每个沙箱派生全局唯一 `sandbox_id`；数据流分配一次性 `jti`。
2. **Authentication (认证)**: 控制信道由 bcrypt + Ed25519 JWT 保护；数据信道由 Ed25519 Ticket 离线验签保护；节点通信通过 Pre-Shared Key (`cluster_secret`) 保护。
3. **Authorization (授权)**: RBAC 划分管理员与普通用户；强制单用户单活跃沙箱；用户仅能访问属于自己 `uid` 映射的沙箱。
4. **Accountability (责任溯源)**:
   * **安全责任记录**: 针对登录鉴权、沙箱派发与终止、违规行为拦截、管理员封禁与快照重置等关键安全事件执行持久化记录，满足靶场运维安全合规与违规追责需求。
   * **底层运行流水**: 普通通信细节、心跳帧、PTY 字符流水与常规调试走系统标准日志（`journalctl` / `stdout`），不污染核心数据库。

---

## 第 6 章：分布式控制调度、数据流接入与会话生命周期

### 6.1 集中调度与算力负载均衡
* Control 内存中实时维护全集群 Worker 节点的健康状态池与槽位利用率。
* 用户申请沙箱时，Control 调度器根据**加权最小连接算法（Weighted Least Connections）**选择最优节点（优先调度到 CPU/内存占用最低且可用槽位最多的 Worker 节点）。
* **Control 侧防刷与滥用限制 (Anti-Abuse Limits)**:
  * **单用户单实例硬限制**: 单个用户账号在同一时刻**最多仅允许运行 1 个沙箱**，杜绝多开实例霸占算力。
  * **创建冷却保护**: 每次创建或销毁沙箱强制附加 15 秒冷却时间，拦截自动化脚本的高频并发轰炸。
  * **控制信道绝对阻断**: Worker 网桥防火墙对沙箱发出的所有流量生效，**严禁沙箱网络主动连接 Control 的内部通信或管理端口**，杜绝将沙箱作为肉鸡跳板反向渗透攻击 Control 大脑。

### 6.2 Ed25519 跨机能力凭证 (Ticket) 与 Worker 零 DB 离线验签
* **Ticket 载荷数据结构 (Claims)**:
  * `jti`: 票据唯一 UUID（用于防重放核销）。
  * `sub`: 所属用户 ID。
  * `sbx`: 绑定的沙箱唯一标识 ID。
  * `typ`: 授权的数据流类型（`pty` 终端 或 `vnc` 桌面）。
  * `exp`: 过期时间戳（签发后固定 60 秒有效；仅在建立 WebSocket 握手时校验，握手成功后的长连接持续有效不受此时间限制）。
  * `cip`: 可选的客户端请求公网 IP。
* **Worker 本地无状态验签流水线**:
  1. 浏览器经 Cloudflare 边缘节点直连目标 Worker `:8443` 端口发起 WebSocket 握手（不经 Control 中转）。
  2. Worker 利用启动时从 Control 同步的 **Ed25519 公钥** 进行数学验签。
  3. **零 DB / 零网络 I/O**: 纯在 CPU 内存中执行（耗时 `< 0.1ms`），不调用数据库，亦不向 Control 回查。
  4. **防重放控制 (Replay Defense)**: Worker 维护带 TTL 的已核销 JTI 哈希表（有效周期 120 秒）。若票据合法但 `jti` 已在表中或已过期，立即切断 TCP 连接。

### 6.3 交互会话并发控制与状态保活 (统一单活跃抢占机制)
针对渗透测试交互的高敏感性，系统杜绝并发冲突，对终端与桌面长连接严格执行以下统一会话管控机制：
1. **严格单活跃连接互斥与抢占 (Single-Active Preemption)**:
   * **桌面 (GUI) 会话互斥**: Worker 维护 `activeGUI[sandbox_id]` 字典，记录当前沙箱正持有的 WebSocket 连接与底层 Unix Socket (`gui.sock`) 桥接。每个沙箱在任意时刻**仅允许存在 1 个活跃桌面连接**。若检测到新的桌面连接接入，Worker 立即向旧连接发送 WebSocket 关闭控制帧：
     ```go
     oldConn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "preempted"), time.Now().Add(time.Second))
     ```
     随后断开旧连接并关闭旧的 Unix Socket 桥接，由新连接接管桌面操作，彻底消除键鼠冲突与 RFB 状态机错乱。
   * **终端 (CLI) 会话互斥**: Worker 维护 `activeCLI[sandbox_id + ":" + winID]` 字典，针对同一沙箱的指定 PTY 窗口同样执行严格的 `4001 Preempted` 互斥抢占，保障单个 PTY 窗口输入输出绝对同步，杜绝并发输入混乱。
2. **反向代理保活机制 (Keepalive Ping Ticker)**:
   * Worker 后台协程每 25 秒向客户端发送一次底层 `websocket.PingMessage`（超时 5 秒），防止 Cloudflare 等反向代理在静止时切断连接。
3. **交互事件感知与空闲刷新 (Activity Touch)**:
   * Worker 实时捕获 PTY 按键及 VNC 鼠标点击事件，原子调用 `touchBoxActivity(sandbox_id)` 刷新时间戳，推后空闲回收倒计时。

### 6.4 租约与空闲自动回收 (Hard TTL 120m, Idle 10m)
* **最大存活时长 (Hard TTL)**: 单次沙箱分配硬性上限为 120 分钟，到期由后台协程强制触发销毁释放。
* **空闲自动回收 (Idle Timeout)**: 连续 10 分钟未感知到任何交互，沙箱被标记为 Idle，系统自动卸载对应 OverlayFS 实例、清理目录并释放物理内存。

### 6.5 节点心跳遥测、熔断与离线状态对齐 (Reconciliation)
* **心跳周期**: Worker 每 3 秒上报一次实时负载（CPU、内存、磁盘剩余空间、槽位利用率）。
* **熔断机制**: Control 连续 10 秒未收到心跳将该 Worker 标记为 `Offline`，暂停分发新任务，存量连接不受影响。
* **状态自愈对齐 (Reconciliation)**: Worker 断线重连后上报本地活跃快照列表，Control 比对数据库，下发指令清理离线期间废弃的孤儿沙箱。

---

## 第 7 章：节点全生命周期管理与 Web 前端管控

### 7.1 节点生命周期状态机 (Online, Draining, Offline, Resetting)
* `Online`: 节点心跳正常（每 3 秒），处于正常调度池，可接收新沙箱创建。
* `Draining`: 节点处于排空维护期。**禁止分发新沙箱**，存量沙箱继续运行，等待存量沙箱全部自然消亡后可安全下线。
* `Offline`: 连续 10 秒无心跳，系统自动将节点标记为离线告警，并将其移出调度队列。
* `Resetting`: 节点正在执行底层云快照回滚或系统重构。

### 7.2 Web 前端管控大盘与 Cluster Secret 节点接入规范
1. **集群遥测大盘**:
   * 管理员前端直观呈现各 Worker 物理负载（CPU 利用率、内存水位、磁盘空间利用率、在线沙箱数/最大槽位数、网络出入流量）。
2. **极简预共享密钥接入 (Cluster Secret / PSK)**:
   * 彻底摒弃动态发 Token 燃烧与数据库拉黑等过度设计，采用最简单可靠的集群预共享密钥（PSK）：
   * **Control 服务端**: `/etc/attackbox/control.yaml` 中配置静态高熵 `cluster_secret: "..."`。
   * **Worker 客户端**: `/etc/attackbox/worker.yaml` 中配置相同的 `cluster_secret`，并指定本节点标识 `node_id: "node-1"`。
   * **握手认证**: Worker 主动向 Control 建立 WebSocket 反向连接时，通过 `Authorization: Bearer <cluster_secret>` 认证，并上报自己的 `node_id` 与硬件能力。
3. **断线自愈与重启无状态**:
   * Control 服务端重启后，Worker 自动通过内置的指数退避重连机制秒级重新接入，带着 `cluster_secret` 即可完成握手，无需依赖数据库持久化节点 Token，避免因系统重启导致合法 Worker 失联。
4. **管理端排空与应急处置**:
   * 管理员在 Web 控制台可将异常或需维护的节点一键设为 `Draining`（停止分派新沙箱，等待存量自然销毁）。
   * 若节点系统受污染，直接在 Web 控制台触发整机云快照重置（Cloud Reset），秒级恢复出厂纯净状态。

### 7.3 安全策略与 Feature Flags 动态热下发 (免重启 Worker)
* **可视化安全控制**: 管理员在 Web 端勾选修改节点的安全策略（如动态调整 Cgroups CPU/内存配额、开关 ptrace 调试特权等）。
* **内存热更新**:
  修改保存后，Control 通过现有的反向控制 WebSocket 向 Worker 推送更新 RPC。Worker 在内存中即刻替换全局配置，**无需重启 Worker 进程**，后续新建的沙箱立即生效新配置。

### 7.4 跨节点沙箱穿透监管与孤儿快照一键清理
* **下钻监管**: 管理员可在 Web 大盘点击任意节点，下钻查看该节点上运行的所有沙箱实例、归属用户、打靶靶场及实时流量。
* **孤儿快照清理**: 若 Worker 汇报的快照在 Control 数据库中不存在（如 Control 异常重启导致），管理员在 Web 端点击“清理孤儿沙箱”，Control 下发销毁 RPC，Worker 强制清理脏快照。

### 7.5 联动云厂商 / 虚拟化 API 的整机快照秒级回滚与自愈 (Cloud Reset)
当多机牺牲品节点的宿主环境遭受高级红队攻击破坏（如内核被植入 Rootkit、文件系统被篡改）时：
1. 管理员在 Web 控制台点击“节点整机重置”。
2. Control 自动调用底层基础设施 API：
   * **私有云环境**: 调用 Proxmox VE / ESXi API 触发虚拟机秒级回滚至初始纯净快照。
   * **公有云环境**: 调用 AWS / DigitalOcean / Hetzner API 触发整机快照重建（Rebuild from Snapshot）。
3. 节点秒级重启后，使用内置的 cluster_secret 自动反向重连入网，彻底恢复至出厂无毒状态。

---

## 第 8 章：系统配置与凭据隔离

### 8.1 静态配置文件规范
系统两端均采用最简的本地 YAML 配置文件启动，常规的 TLS 证书路径直接作为配置项传入即可，不引入额外的复杂设计：
* **Control 配置文件 (`/etc/attackbox/control.yaml`, 权限 `0600`)**:
  * 静态包含：Web 监听端口（`:443`）、TLS 证书与私钥路径、Ed25519 签名私钥、集群通信密钥 `cluster_secret`、SQLite 数据库路径、以及第三方服务凭证（Resend 邮件 Key、OAuth Client Secret、Turnstile 密钥）。
  * 数据库（`attackbox.db`）只存业务数据（用户、积分与审计日志），不存放任何系统 API Key，防止 SQL 注入拖库。
* **Worker 配置文件 (`/etc/attackbox/worker.yaml`, 权限 `0600`)**:
  * 静态包含：数据端口监听（`:8443`）、TLS 证书路径、Control 反向连接地址（`wss://...`）、Ed25519 验签公钥、集群通信密钥 `cluster_secret` 与本节点标识 `node_id`。

### 8.2 核心机密单向隔离与无状态 Worker
* **私钥留底，公钥下发**:
  * Control 独占持有 Ed25519 **签名私钥**；Worker 仅持有 **验签公钥** 用于离线校验 Ticket。Worker 节点即便被逆向或 dump 内存，也无法伪造 Ticket 票单。
* **Worker 物理零机密、零数据库**:
  * Worker 本地没有数据库、没有用户密码，亦无第三方 API Key。
  * 学员无论如何在沙箱中破坏或在靶机中测试，受害面严格锁死在该 Worker 算力节点内，Control 核心资产物理隔离。
