# AttackBox - 轻量级 Web 远程渗透测试环境系统

AttackBox 是一个基于 Linux 原生 btrfs 写时复制（CoW）快照与网络命名空间（network namespace）构建的轻量级 Web 远程渗透测试工作台。全栈基于 Go 语言原生实现，无外部 Bash 脚本依赖、无重量级 Docker/VM 镜像开销，通过单一 443 端口提供完整的 Web 管理控制台、多命令行虚拟终端空间及独立远程桌面。

---

## 核心架构设计

1. **btrfs 瞬态快照隔离**：
   沙箱槽位基于 btrfs 写时复制（CoW）机制，秒级派生宿主根目录快照。用户在沙箱环境内的文件修改、工具安装和网络配置均隔离于快照层。释放环境时直接删除子卷，宿主机系统 100% 保持纯净无残留。

2. **纯 Go 原生底层调度 (零 Shell 脚本)**：
   完全废除外部 Bash 脚本。沙箱的核心网络命名空间创建、veth 虚拟网卡对配置、虚拟设备文件树挂载（`/dev/pts`, `/dev/shm`, `/proc`, `/sys`）、Xtigervnc 桌面会话管理以及快照物理销毁，全部由 Go 代码（`internal/box`）原生调用系统底层接口完成。

3. **宿主网络自适应与出向安全隔离**：
   网关启动时自动在宿主机初始化原生网桥 `br0` (`10.200.0.1/24`) 并下发 nftables 隔离规则：
   * 沙箱流量通过三层软路由自适应直通本地 Docker 靶机（`172.17.0.0/16`）及远端 VPN 靶场（`10.10.0.0/16`）。
   * 默认阻断沙箱向外部物理网卡路由未授权流量，仅放行指定公共 DNS 服务，形成严密的安全边界。

4. **单会话互斥管理**：
   CLI 终端与 GUI 桌面均支持基于 WebSocket 的会话互斥。若同一槽位在其他页面建立新会话，网关主动发送 RFC 6455 状态码 `4001`（`preempted`）挂起旧窗口，避免并发连接导致状态混淆。

5. **Web 虚拟终端空间与独立桌面**：
   * **Web 虚拟终端空间 (`/term`)**：以独立浏览器新窗口打开，提供桌面级多命令行窗口工作区。支持在画布内按需新建多个命令行窗口，支持自由拖拽排布、边缘拉伸调节，通过 `ResizeObserver` 自动触发 `xterm.js` 与底层 Linux PTY 窗口大小同步。
   * **独立远程桌面 (`/gui`)**：以独立浏览器新窗口全屏打开，前端基于 noVNC 核心库（`core/rfb.js`）直连 WebSocket 网桥，自适应画布缩放。
   * **控制台主页 (`/`)**：纯净的管理入口，仅负责用户认证、沙箱状态监测、环境销毁及靶机路由信息展示。

6. **前后端解耦与单端口网关**：
   `attackboxd` 后端统一基于 443 端口交付 HTTPS REST API、WebSocket 协议桥接与 SQLite RBAC 鉴权；前端 Web 资源作为独立 `web/` 目录挂载运行，修改前端模板与脚本保存后 F5 刷新即生效，无需重新编译二进制。

---

## 系统拓扑结构

```mermaid
graph TD
    Client["用户浏览器 (Chrome / Edge / Firefox)"] -->|HTTPS / WSS (Port 443)| Gateway["attackboxd (统一网关服务)"]
    Gateway -->|静态资源与模板读取 (F5 实时生效)| WebDir["web/ 目录 (templates & static)"]

    subgraph "宿主系统 (Host OS / btrfs 文件系统)"
        Gateway -->|原生网络与防火墙初始化| NetFW["内核 br0 网桥与 nftables 规则"]
        Gateway -->|用户认证与 RBAC 管理| AuthStore["SQLite 认证库 (attackbox.db)"]
        Gateway -->|原生快照与容器调度| BoxMgr["Go 沙箱生命周期引擎 (internal/box)"]
        Gateway -->|原生 PTY 字节流| PTYBridge["PTY 终端桥接 (su - boxUser)"]
        Gateway -->|RFB 协议转发| VNCBridge["TigerVNC Unix Socket (:N)"]

        subgraph "网络边界与路由策略"
            NetFW -->|内网路由| DockerNet["本地 Docker 靶机 (172.17.0.0/16)"]
            NetFW -->|路由互通| VPNNet["远端 VPN 靶场 (10.10.0.0/16)"]
            NetFW -.->|nftables 拦截规则| WAN["物理网卡出口 (仅允许 DNS)"]
        end

        subgraph "沙箱槽位 (boxN 快照目录)"
            BoxMgr -->|Go 原生配置| NetNS["网络命名空间 (ab-boxN)"]
            BoxMgr -->|btrfs 原生命令| Chroot["文件系统快照 (btrfs CoW)"]
            BoxMgr -->|Go 原生挂载| DevPts["隔离 devpts / dev / proc / sys"]
            PTYBridge --> DirectShell["交互式 Shell 进程"]
            VNCBridge --> MateDesktop["TigerVNC + MATE 桌面 (按需)"]
        end
    end
```

---

## 项目工程结构

```
attackbox/
├── cmd/
│   └── attackboxd/             # 网关主程序入口 (main.go)
├── internal/                   # 纯 Go 后端核心逻辑
│   ├── auth/                   # SQLite 用户持久化与 bcrypt 认证模块 (store.go)
│   ├── box/                    # Go 原生沙箱生命周期管理引擎
│   │   ├── manager.go          # 原生核心沙箱与桌面会话创建、状态检查与销毁
│   │   ├── network.go          # 原生宿主网桥 (br0) 与 nftables 出网安全基线管理
│   │   └── snapshot.go         # btrfs CoW 瞬态快照管理
│   ├── config/                 # 配置文件解析与参数结构 (config.go)
│   └── gateway/                # 统一 Web 网关、REST API 与 WebSocket 桥接
│       ├── server.go           # 网关服务核心、路由注册、中间件与页面渲染
│       ├── api.go              # 用户认证、沙箱控制、管理员运维 REST API 与工具函数
│       └── websocket.go        # PTY 与 VNC WebSocket 协议桥接
├── web/                        # 前端 Web 资源目录 (独立于二进制，F5 刷新即生效)
│   ├── templates/              # HTML 页面模板 (dash.html, term.html, gui.html)
│   └── static/                 # 扁平化前端静态资源 (dash.css, dash.js, term.css, term.js)
├── deploy/
│   └── systemd/                # systemd 服务单元配置文件
│       └── attackboxd.service  # 网关守护进程服务 (开机自启)
├── config.json                 # 靶场运行配置文件
└── README.md
```

---

## 运维管理指南

### 1. 服务控制
```bash
# 查看网关服务状态
sudo systemctl status attackboxd

# 重启网关服务
sudo systemctl restart attackboxd

# 查看网关运行日志
sudo journalctl -u attackboxd -f -n 50

# 查看当前 nftables 过滤规则
sudo nft list table inet attackbox
```

### 2. 初始访问与凭据
* 访问入口：`https://<服务器地址>/`
* 默认管理员账号：`admin`
* 默认管理员密码：`qwer1234!`
