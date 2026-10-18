# AttackBox - 轻量级 Web 远程渗透测试环境系统

AttackBox 是一个基于 Linux 原生 btrfs 写时复制（CoW）快照与网络命名空间（network namespace）构建的轻量级 Web 远程渗透测试工作台。全栈基于 Go 语言原生实现，无外部 Bash 脚本依赖、无重量级 Docker/VM 镜像开销，通过单一 443 端口交付现代 Web SPA 控制台、多虚拟命令行工作区、独立远程桌面与严格的纵深防御安全机制。

---

## 核心架构与安全特性

### 1. 瞬态快照沙箱隔离 (btrfs CoW + NetNS)
* **秒级派生**：沙箱槽位基于 btrfs 写时复制（CoW）机制，秒级派生宿主根目录快照。用户在沙箱环境内的文件修改、工具安装与操作均完全隔离于快照层。
* **纯净回收**：释放环境时直接原子卸载并删除子卷，宿主机系统 100% 保持纯净无残留。
* **纯 Go 原生调度**：底层命名空间创建、veth 对配置、虚拟文件系统挂载（`/dev/pts`, `/dev/shm`, `/proc`, `/sys`）、TigerVNC 会话管理与快照销毁均由 Go 后端原生实现，零外部脚本调用。

### 2. 纵深防御网络基线 (nftables + TrustProxy)
* **宿主网桥隔离**：后端启动时自动初始化原生网桥 `br0` (`10.200.0.1/24`) 并下发 nftables 隔离规则。
* **靶网自适应连通**：沙箱流量通过三层软路由直通本地 Docker 靶机（`172.17.0.0/16`）及远端 VPN 靶场（`10.10.0.0/16`）。
* **出向拦截**：默认阻断沙箱向外部物理网卡路由未授权流量，仅放行指定公共 DNS 服务，形成物理级防御边界。
* **防伪造真实 IP 门控**：多层代理（如 Cloudflare）下严格校验边缘 IP，拦截伪造 `X-Forwarded-For` 请求头，确保审计日志与限流 IP 真实可信。

### 3. API 凭据零落地架构 (PBKDF2 + AES-256-GCM + Memory Vault)
* **信封加密**：第三方服务密钥（如 Resend API Key）绝不以明文形式落地磁盘或写入配置文件。
* **密码派生密钥**：采用管理员登录密码基于 `PBKDF2-HMAC-SHA256`（50,000 次安全迭代）派生 256 位加密密钥，以 `AES-256-GCM` 密文形式存储于 SQLite。
* **内存动态解封**：网关冷启动时密钥库处于封锁（Sealed）状态；仅当管理员合法登入系统时在内存中动态解封（Unseal）；管理员修改密码时自动触发密钥重新派生与密文重封。
* **零明文回显**：管理端 API 读取配置时对敏感密钥实施单向掩码与零回显脱敏。

### 4. 邮箱白名单强校验与 Resend 验证码
* **受信域白名单**：注册仅放行主流权威邮箱提供商（QQ 邮箱 `@qq.com`, `@foxmail.com`、谷歌 Gmail `@gmail.com`, `@googlemail.com`、微软 Outlook `@outlook.com`, `@hotmail.com`, `@live.com`, `@msn.com`），强力屏蔽一切临时/不可信一次性邮箱。
* **发信防刷风控**：基于 Resend REST API 分发 6 位随机验证码，具备 60 秒冷却（Cooldown）限制及 10 分钟时效核验。

### 5. AFF 推广邀请机制
* **专属 8 位邀请码**：系统自动为每个用户生成剔除易混淆字符的 8 位大写邀请码（如 `FE339NQM`）。
* **推广链接自动识别**：注册页面支持 URL 参数 `?aff=CODE` 自动感知与锁定，注册成功后自动持久化上下游邀请归属链路。
* **个人中心一键推广**：在 `/account` 页面提供直观的专属推广链接展示与一键复制剪贴板支持。

### 6. OAuth 2.0 快捷认证
* **支持平台**：无缝支持 GitHub 与 Microsoft / Outlook Entra ID 第三方授权。
* **CSRF 纵深防护**：授权跳转采用 `HMAC-SHA256` 签发有时效的防伪 State 凭据并配合 `SameSite=Lax` 安全 Cookie 校验。
* **两步建号机制**：第三方初次授权成功后，强制跳转至两步确认弹窗，要求自定义或确认用户名并强制设定不少于 6 位的独立安全登录密码。

### 7. 现代前后端分离架构 (Vue 3 + Tailwind + shadcn)
* **独立工程**：前端采用 Vite + Vue 3 Composition API 构建，打包产物直出至 `web/dist/`。
* **双模访问**：控制台主页、工作区大屏、个人中心与管理后台整合为统一单页应用（SPA），兼具极佳的页面交互体验与极简的单端口部署运维。

---

## 系统拓扑图

```mermaid
graph TD
    Client["用户浏览器 (Chrome / Edge / Firefox)"] -->|HTTPS / WSS (Port 443)| Gateway["attackboxd (统一安全网关)"]
    Gateway -->|静态资源交付 (Vue3 SPA)| WebDist["web/dist/ (Assets & SPA HTML)"]

    subgraph "宿主系统 (Host OS / btrfs 文件系统)"
        Gateway -->|原生网络与防火墙基线| NetFW["内核 br0 网桥与 nftables 规则"]
        Gateway -->|RBAC 鉴权与审计流水| AuthStore["SQLite 认证库 (attackbox.db)"]
        Gateway -->|信封加密与内存密钥环| Vault["Memory Vault (PBKDF2 + AES-GCM)"]
        Gateway -->|原生快照与容器生命周期| BoxMgr["Go 沙箱生命周期引擎 (internal/box)"]
        Gateway -->|原生 PTY 字节流| PTYBridge["PTY 终端桥接 (su - boxUser)"]
        Gateway -->|RFB 协议转发| VNCBridge["TigerVNC Unix Socket (:N)"]

        subgraph "网络边界与路由安全"
            NetFW -->|直通内网| DockerNet["本地 Docker 靶机 (172.17.0.0/16)"]
            NetFW -->|直通路由| VPNNet["远端 VPN 靶场 (10.10.0.0/16)"]
            NetFW -.->|nftables 拦截规则| WAN["物理网卡出口 (仅放行 DNS)"]
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
│   ├── auth/                   # 用户认证、SQLite 持久化与密码信封加密
│   │   ├── store.go            # 数据库初始化、用户模型、AFF 与审计流水
│   │   ├── crypto.go           # PBKDF2 密钥派生与 AES-256-GCM 加解密工具
│   │   └── crypto_test.go      # 密码学单元测试
│   ├── box/                    # Go 原生沙箱生命周期管理引擎
│   │   ├── manager.go          # 原生沙箱创建、状态监控、空闲释放与销毁
│   │   ├── network.go          # 原生宿主网桥 (br0) 与 nftables 出网安全基线
│   │   └── snapshot.go         # btrfs CoW 瞬态快照管理
│   ├── config/                 # 配置文件解析与参数结构 (config.go)
│   └── gateway/                # 统一 Web 网关、REST API 与 WebSocket 桥接
│       ├── server.go           # 服务核心、路由分发、中间件与安全响应头
│       ├── api.go              # 用户认证、沙箱操作与管理员 REST API
│       ├── email.go            # 邮箱白名单、内存 Vault、Resend 客户端与验证码
│       ├── oauth.go            # GitHub / Outlook OAuth 2.0 授权与两步建号
│       └── websocket.go        # PTY 与 VNC WebSocket 协议桥接
├── frontend/                   # 现代前端工程 (Vue 3 + Vite + Tailwind)
│   ├── src/
│   │   ├── views/              # 页面视图 (AuthView, WorkspaceView, AccountView, Admin*)
│   │   ├── components/         # 基础组件 (Toast, WinBox 等)
│   │   ├── api.js              # 用户端 API 客户端
│   │   └── adminApi.js         # 管理端 API 客户端
│   └── package.json            # 前端依赖配置
├── web/                        # Web 部署目录
│   └── dist/                   # 前端 Vite 编译产物 (SPA 静态资源)
├── tests/                      # 自动化测试套件
│   ├── test_email_aff_suite.py # 邮箱白名单、AFF 邀请链与 Resend 零明文测试
│   ├── test_oauth_suite.py     # OAuth 第三方认证与安全防护端到端测试
│   ├── test_stage2_suite.py    # 沙箱生命周期、容量监控与安全审计全量测试
│   └── run_regression.py       # 全量回归驱动测试
├── deploy/
│   └── systemd/                # systemd 服务单元配置文件
├── deploy_sync.sh              # 远程自动化构建、安装与服务部署脚本
├── config.json                 # 靶场运行配置文件
├── go.mod                      # Go 模块定义
└── README.md                   # 系统核心设计与运维指南
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

### 2. 前端构建与一键部署
```bash
# 本地前端构建
cd frontend && npm run build && cd ..

# 打包并同步部署至服务器
tar -czf update_sync.tar.gz cmd internal web frontend tests deploy_sync.sh
scp update_sync.tar.gz deploy_sync.sh laji@<服务器IP>:/home/laji/
ssh laji@<服务器IP> "bash /home/laji/deploy_sync.sh"
```

### 3. 自动化测试套件执行
```bash
# 邮箱白名单、AFF 邀请机制与 Resend 密文验证
python3 tests/test_email_aff_suite.py

# OAuth 第三方授权与安全防伪验证
sudo python3 tests/test_oauth_suite.py

# 沙箱生命周期与端到端回归测试
python3 tests/test_stage2_suite.py
```
