# AttackBox - 轻量高密 Web 远程渗透测试环境系统

AttackBox 是一个面向高密并发、具备原生免密 sudo 高权限、秒级创建与物理销毁的 Web 远程渗透测试工作台。系统基于 Linux 原生 btrfs 瞬态快照（写时复制）与内核三层软路由转发，免容器运行时（无 Dockerfile / 无 GB 级镜像），通过单端口 443 统一交付极简 Web 控制台、现代 PTY 命令行终端与极简远程桌面。

---

## 核心设计特性

1. **瞬态快照，物理零污染销毁**：
   沙箱槽位基于 btrfs 写时复制（CoW）机制，秒级派生宿主根目录快照。用户在沙箱内的所有新建、修改、安装与配置操作均隔离于快照层，释放环境时物理删除子卷，宿主机 100% 保持纯净无残留。

2. **方案 A 懒加载与轻量核心秒开**：
   沙箱解耦为核心层（Core）与桌面层（GUI）。用户访问 CLI 时仅启动核心层（耗时仅 **0.55 秒**，空载内存仅 **6 MB**）；仅当访问远程桌面时才按需拉起 TigerVNC 与 MATE 桌面，杜绝无意义的桌面资源空转。

3. **原生免密 sudo 提权**：
   沙箱内部以 `parrot` 用户登录，具备完整免密 `sudo` 权限，支持自由修改 `/etc/hosts`、安装新工具、调试内核网络与运行任意安全 POC / 扫描脚本。

4. **内核三层软路由与单向安全边界**：
   * 宿主机维护原生虚拟网桥 `br0` (`10.200.0.1/24`)，开启内核 `ip_forward`。
   * **靶场自适应互通**：沙箱流量由内核路由无缝自适应转发至同机 Docker 靶机（`172.17.0.0/16`）或远端 VPN 靶场（`10.10.0.0/16`）。
   * **出口单向拦截**：宿主机 nftables 强行拦截来自沙箱发往公网物理网卡（`ens3`）与宿主机管理端口（22/443）的流量，仅放行公共 DNS 解析（`1.1.1.1:53`）。

5. **严格单活跃会话互斥抢占 (Single Active Session)**：
   CLI 终端与 GUI 桌面均实行严格单活跃会话管理。新窗口接入时，网关主动向旧窗口发送 RFC 6455 状态码 `4001`（`preempted`）关闭连接；前端捕获该状态码后立即终止自动重试，展示悬浮挂起遮罩并提供“切回此窗口接管”按钮，彻底根除多窗口输入混淆与死循环互踢。

6. **原生极简解耦 PTY 终端**：
   废除复杂的 Tmux 套接字依赖与分屏状态栏，网关直接使用 Go `creack/pty` 与 WebSocket 二进制流连接沙箱内的标准登录 Shell（`su - parrot`），搭配基于 `JetBrains Mono` 字体、支持清屏与全屏的极简暗黑终端界面。

7. **轻量化远程桌面**：
   剔除官方 noVNC 臃肿侧边栏，采用自研极简客户端（`deploy/vnc.html`，~160 行代码），调优压缩比（`compressionLevel: 2`, `qualityLevel: 6`）；MATE 桌面强制关闭动画合成器，单桌面空载内存低至 ~400MB。

8. **单二进制 Go 网关与 Web 认证体系**：
   单二进制 `attackboxd` 监听 443 端口，内置 SQLite 数据库（`bcrypt` 密码加密）与 RBAC 用户体系。支持管理员实时监控 VPS 资源状态、管理用户生命周期，普通用户无需繁琐启动步骤，点击直接转圈秒开并跳转。

---

## 系统架构拓扑

```mermaid
graph TD
    Client["用户浏览器 (Chrome / Firefox / Edge)"] -->|HTTPS / WSS (Port 443)| Gateway["attackboxd (Go 单二进制网关)"]

    subgraph "宿主 Host (Parrot OS 7.4 / Btrfs 文件系统)"
        Gateway -->|Web 认证与 RBAC 管理| AuthStore["SQLite 认证库 (attackbox.db)"]
        Gateway -->|单活跃会话调度 (activeCLI / activeGUI)| SessionMgr["会话锁与抢占控制 (Code 4001)"]
        Gateway -->|原生 PTY 二进制字节流| PTYBridge["Linux 伪终端 (su - parrot)"]
        Gateway -->|RFB 二进制协议桥接| VNCBridge["TigerVNC Unix Socket (:N)"]

        subgraph "宿主内核网络与安全边界"
            Bridge["内核网桥 br0 (10.200.0.1/24)"]
            Bridge -->|内网直通 (自适应)| DockerNet["本地 Docker 靶机 (172.17.0.0/16)"]
            Bridge -->|路由直通 (自适应)| VPNNet["远端 VPN 靶场 (10.10.0.0/16)"]
            Bridge -.->|nftables 单向拦截出口| WAN["物理网卡 ens3 (仅放行 1.1.1.1:53 DNS)"]
            Bridge -.->|nftables 阻断管理面| HostMgmt["宿主管理端口 (22 / 443)"]
        end

        subgraph "沙箱槽位 (boxN 瞬态快照 /var/lib/attackbox/boxes/boxN)"
            NetNS["网络命名空间 (ab-boxN)"]
            Chroot["独立根文件系统快照 (btrfs CoW)"]
            DevPts["独立 devpts (ptmxmode=666) 与 dev/shm (1777)"]
            MateDesktop["Xtigervnc + MATE 极简桌面 (按需拉起)"]
            DirectShell["原生 Bash 交互 Shell (无 Tmux 依赖)"]
        end
    end

    SessionMgr -.->|挂起通知| Client
    PTYBridge --> DirectShell
    VNCBridge --> MateDesktop
```

---

## 核心实现技术细节

### 1. 方案 A 沙箱懒加载与分级启停
沙箱生命周期控制脚本位于 [`scripts/box-launch.sh`](file:///F:/deepseek/attackbox/scripts/box-launch.sh)：
* **Core 核心模式**：执行 `btrfs subvolume snapshot` 派生子卷，创建 `ab-boxN` 虚拟网卡对并连接 `br0`，挂载独立的 `/dev/pts`、`/dev/shm` 与 `tmpfs`。启动耗时 ~0.5s，内存占用 ~6MB。
* **GUI 桌面模式**：在 Core 基础之上，按需启动 `Xtigervnc` 监听独立 Unix 套接字（`gui.sock`），并调优加载轻量 MATE 桌面会话。
* **销毁模式**：终止槽位 cgroup 内所有进程，清理网络命名空间与 veth 设备，卸载所有临时挂载点，物理执行 `btrfs subvolume delete` 销毁快照。

### 2. 原生解耦 PTY 终端架构
网关 WebSocket 终端桥接位于 [`internal/gate/ws.go`](file:///F:/deepseek/attackbox/internal/gate/ws.go)：
* 丢弃传统的 Tmux IPC 包装层，直接调用 Go `creack/pty.StartWithSize`：
  ```go
  cmd := exec.Command("/usr/bin/ip", "netns", "exec", netns,
      "/usr/sbin/chroot", rootfs,
      "/usr/bin/su", "-", "parrot")
  ```
* 终端尺寸动态监听：前端 xterm 缩放时向服务端发送 `{"cols": X, "rows": Y}` JSON 数据包，网关自动调用 `pty.Setsize` 实时同步内核终端窗口大小。

### 3. 单活跃会话互斥锁与抢占协议
在网关结构体 `Gate` 中维护并发互斥字典：
```go
type Gate struct {
    cliMu     sync.Mutex
    activeCLI map[string]*websocket.Conn // boxID -> current active CLI connection

    guiMu     sync.Mutex
    activeGUI map[string]*websocket.Conn // boxID -> current active GUI connection
}
```
* **抢占断开**：当同槽位建立新连接时，服务端调用 `oldConn.WriteControl` 发送带有自定义代码 `4001` 的 Close 帧，随后关闭底层连接；
* **前端防护**：前端通过 Monkey-patch 监控原生 WebSocket 关闭事件，捕获 `4001` 后切断重试定时器，展示沉浸式挂起提示，并激活“切回此窗口接管”手动唤醒按键。

### 4. 网络隔离与 nftables 规则
网络初始化脚本位于 [`scripts/attackbox-net.sh`](file:///F:/deepseek/attackbox/scripts/attackbox-net.sh)：
```nft
table inet attackbox {
    chain forward {
        type filter hook forward priority 0; policy drop;
        # 允许已建立的连接与沙箱间/靶机间互通
        ct state established,related accept
        iifname "br0" oifname "br0" accept
        # 允许直通本地 Docker 网桥与 VPN 隧道
        iifname "br0" oifname "docker0" accept
        iifname "br0" oifname "tun*" accept
        iifname "br0" oifname "wg*" accept
        # 仅放行指定公共 DNS 解析
        iifname "br0" ip daddr 1.1.1.1 udp dport 53 accept
        # 坚决拍死通往物理网卡的一切公网流量
        iifname "br0" oifname "ens3" reject
    }
}
```

---

## 项目工程结构

```
attackbox/
├── cmd/
│   └── attackboxd/             # 单二进制网关主程序入口 (main.go)
├── internal/
│   ├── auth/                   # SQLite 用户持久化与 bcrypt 认证模块 (store.go)
│   ├── box/                    # btrfs 瞬态快照管理与槽位租约池 (snapshot.go, pool.go)
│   ├── cfg/                    # 配置文件解析与默认参数注入 (cfg.go)
│   └── gate/                   # HTTP 路由、Web 页面渲染与 WebSocket 桥接
│       ├── gate.go             # 核心路由、用户 CRUD 与系统监控 API
│       ├── ws.go               # PTY 与 VNC WebSocket 桥接及单活跃会话管理
│       └── templates/          # Go 嵌入式前端模板
│           ├── dash.html       # 极简卡片式仪表盘、登录注册与管理员后台
│           └── term.html       # 现代极简 JetBrains Mono 终端界面
├── deploy/
│   ├── systemd/                # systemd 服务单元文件
│   │   ├── attackboxd.service  # 网关主守护进程
│   │   └── attackbox-fw.service# 启动自动加载软路由与防火墙服务
│   └── vnc.html                # 极简轻量版 noVNC 远程桌面客户端 (部署至 /usr/share/novnc)
└── scripts/
    ├── attackbox-net.sh        # 宿主机网络初始化与 nftables 防火墙脚本
    └── box-launch.sh           # 沙箱生命周期引导脚本 (core / gui / stop / clean)
```

---

## 运维与管理指南

### 1. 服务控制与日常运维
网关与防火墙均已配置为 systemd 守护进程：
```bash
# 查看网关实时运行状态
sudo systemctl status attackboxd

# 重启网关服务
sudo systemctl restart attackboxd

# 实时查看网关访问与调度日志
sudo journalctl -u attackboxd -f -n 50

# 查看当前 nftables 安全规则
sudo nft list table inet attackbox
```

### 3. 用户与管理员初始凭据
* 默认管理员账号：`laji` 或 `admin`
* 默认初始密码：`qwer1234!`
* 平台访问入口：`https://microsb.de/` (自动重定向至登录页)
