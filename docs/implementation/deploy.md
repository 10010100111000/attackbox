# 单机部署

状态：全新部署。Nuxt、PostgreSQL 和 Go Worker 是唯一运行路径。

## 拓扑

```
浏览器 ──HTTP:80──> nginx ──> attackbox-web (Nuxt/Nitro :3000) ──> PostgreSQL
   │
   └──WebSocket:8443──> attackbox-worker (Go) ──> 沙箱 netns / Docker 靶机
```

- nginx 只服务 Web 页面和 `/api/`,沙箱/节点的 WebSocket 不经过 nginx。
- `attackbox-worker` 以 root 运行,负责网络、cgroup、Btrfs 和 Docker。
- 没有单独的 control 服务;`attackbox-web` 的 `/cluster/connect` 就是控制面。

## 版本化文件

| 文件 | 用途 |
| --- | --- |
| `deploy/systemd/attackbox-web.service` | Nuxt 服务单元 |
| `deploy/systemd/attackbox-worker.service` | Go Worker 单元 |
| `deploy/nginx/attackbox.conf.template` | 单入口 nginx 模板，部署时生成实际转发地址 |
| `configs/node.yaml.example` | Worker 配置模板（复制为 `configs/node.yaml`，实际配置不提交 Git） |
| `deploy/web.env.example` | 配置参考；实际配置由安装向导生成 |
| `scripts/setup.py` | 首次配置与密钥生成向导 |
| `scripts/manage.py` | 使用服务端配置执行迁移或创建管理员 |
| `scripts/deploy.sh` | 服务器上的唯一部署脚本 |
| `scripts/migrate.sh` | `manage.py migrate` 的便捷入口（需 sudo） |
| `scripts/deploy.ps1` | Windows 侧:push 后 SSH 调 `deploy.sh` |

## 首次安装

依赖：Python 3、PostgreSQL、Docker、Go、Node（支持项目依赖的版本）、nginx 和 sudo。先创建空数据库及专用数据库用户，准备其 PostgreSQL URL。脚本不安装这些系统依赖，也不创建数据库角色。

在仓库根目录执行（构建和安装 npm 依赖使用普通部署用户）：

```bash
sudo scripts/setup.py
npm --prefix web ci --no-audit --no-fund
sudo scripts/manage.py migrate
sudo scripts/manage.py admin
./scripts/deploy.sh
```

首次向导会：

- 隐藏输入数据库连接；询问网站 origin，Resend Key 与发件地址可稍后配置。
- 自动生成独立的集群通信密钥、Ed25519 签名种子、内容加密密钥、判题密钥，不显示密钥。
- 写入 `/etc/attackbox/web.env` 和 `configs/node.yaml`，权限 `0600`；自动同步两端集群密钥和节点地址。实际配置不提交 Git。
- 使用默认 Nuxt 3000、Worker 8443；需要自定义时，在首次启动前编辑生成文件，见下文端口配置。
- HTTPS 站点使用直接连接 Worker 的 WSS 地址，需要提供已存在的证书与私钥（证书应覆盖该主机名）。本机测试可先用 `http://localhost`；正式域名、nginx HTTPS 和证书仍由部署者配置，向导不申请证书。

管理员命令隐藏输入并确认密码（至少 12 位），不会把密码放入参数、配置文件或日志。数据库只保存密码哈希；已有管理员或邮箱时拒绝创建，不提升或重置旧账号。缺少邮件配置仍可创建管理员，但注册验证、重发验证邮件与密码找回不可用。

初始化仅允许两个配置文件都不存在时执行。已有配置时直接拒绝，不补写或更换密钥；日常部署不调用初始化、不创建管理员、不自动迁移数据库。请将配置与数据库一起备份。

沙箱根文件系统、Btrfs、cgroup 和网桥需按 [沙箱说明](../sandbox/sandbox.md) 准备；Docker 靶场网络启用和验收见 [Compose 部署说明](compose-platform-integration.md)。安装向导不代替宿主机环境准备。

## 日常部署

```powershell
# Windows
scripts/deploy.ps1
```

或直接在服务器:

```bash
cd /home/laji/attackbox
./scripts/deploy.sh
```

脚本按顺序:校验干净工作区 → `git pull --ff-only` → 编译 Worker → `npm ci && npm run build` → 安装 unit/nginx → 发布到 `/var/www/attackbox-web` → 重启 → 健康检查（实际 Web／Worker 地址、Worker 注册、nginx 服务状态）。

**注意:重启 Worker 会销毁正在运行的沙箱。** 部署应安排在维护窗口;`/var/www/attackbox-web.prev` 保留上一版,可手工回滚。

## 回滚

```bash
sudo systemctl stop attackbox-web
sudo rm -rf /var/www/attackbox-web
sudo mv /var/www/attackbox-web.prev /var/www/attackbox-web
sudo systemctl start attackbox-web
```

## 当前部署边界

CLI 和 GUI 都通过 Worker 自身的 `__enter__`／`__enter_root__` 进入沙箱。

当前示例使用浏览器直连 Worker 8443；HTTPS 站点需配置可访问的 WSS 地址，也可以正确配置反向代理。这是部署选择，不是永久禁止 nginx 代理的约束。

部署脚本仅接受干净工作区并执行快进更新；已忽略的实际配置不影响检查。

## 配置职责

| 位置 | 职责 |
| --- | --- |
| `configs/node.yaml` | Worker 节点身份、连接地址、网关、存储路径、磁盘保护、网桥和 Docker 本机配置 |
| `/etc/attackbox/web.env` | Web 服务部署参数与密钥，包括数据库连接、集群密钥、邮件服务 Key |
| 管理员后台 | 「运行策略」配置沙箱时长、空闲回收与靶场时长；「节点管理」配置节点容量与资源限额 |

`compose_labs.enabled` 和 `network_verified` 在本机网络验收通过后开启，具体见 [Compose 部署说明](compose-platform-integration.md)。

## 端口配置

- Nuxt：在 `/etc/attackbox/web.env` 设置 `HOST`、`PORT`（默认 `127.0.0.1`、`3000`）。向导生成带双引号的单行 `KEY=value`；手工编辑时保留格式；不要另设 `NITRO_HOST`、`NITRO_PORT`。
- Worker：在 `configs/node.yaml` 设置 `gateway.listen_addr`。浏览器连接地址由 `gateway.public_data_url` 与 Web 的 `SANDBOX_NODES_JSON[].publicDataUrl` 对应设置。
- Worker 的 `node.control_url` 指向 Nuxt 的实际监听地址及 `/cluster/connect`。
- nginx 的网站入口端口由 `deploy/nginx/attackbox.conf.template` 的 `listen` 设置，默认 80；入口端口或协议改变时，同步调整域名与 `APP_BASE_URL`。

部署脚本读取上述本机监听配置，生成 nginx upstream 与 Web／Worker 健康检查地址，无需手工修改检查端口。通配监听地址转换为对应的 loopback 地址；Worker 启用 TLS 时，通过 `public_data_url` 对应的 HTTPS 地址检查，保持证书校验。生成文件位于临时目录，部署结束删除，不包含邮件或数据库密钥。

## Nuxt 服务身份与权限

构建由执行部署脚本的用户完成。脚本创建专用系统账号 `attackbox`（无登录 shell），Nuxt 的 systemd 服务以该账号运行；Worker 以 root 运行。

- `/var/www/attackbox-web`：发布文件归 root 所有，Nuxt 只读。
- `/var/lib/attackbox-web`：systemd 自动创建并交给 `attackbox`，权限 `0700`。教程与靶场包默认分别存放在其 `content`、`labs` 子目录，不与 Worker 的 `/var/lib/attackbox` 混用。
- `/etc/attackbox/web.env`：root 所有、权限 `0600`，由 systemd 读取并传入 Nuxt。部署渲染工具通过 sudo 读取，只输出连接地址和 nginx 配置，不输出密钥。

如自行修改 `CONTENT_DATA_DIR` 或 `LAB_DATA_DIR`，需预先创建对应目录并授予 `attackbox` 写权限；目录须放在发布产物、用户 home 和系统只读路径之外。

学习首页推荐区配置见 [学习首页管理](learn-home.md)；首次部署此功能需先执行 `sudo scripts/manage.py migrate`，再发布服务。
