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
| `deploy/web.env.example` | Web 环境变量模板(复制到 `/etc/attackbox/web.env`) |
| `scripts/deploy.sh` | 服务器上的唯一部署脚本 |
| `scripts/migrate.sh` | 显式执行数据库迁移 |
| `scripts/deploy.ps1` | Windows 侧:push 后 SSH 调 `deploy.sh` |

## 首次准备

1. PostgreSQL、Docker、Go、Node、nginx 已安装。
2. `cp deploy/web.env.example /etc/attackbox/web.env`,替换全部 `CHANGE_ME`。
3. 创建 Worker 本机配置并填写实际值：

   ```bash
   install -m 0600 configs/node.yaml.example configs/node.yaml
   ${EDITOR:-vi} configs/node.yaml
   ```

   `configs/node.yaml` 已加入 Git 忽略。Worker 从部署目录读取它，systemd 显式指定该路径。与 `/etc/attackbox/web.env` 对应关系：
   - `node.node_id` == `SANDBOX_NODES_JSON[].id`，`node.cluster_secret` == `SANDBOX_CLUSTER_SECRET`；
   - `gateway.public_data_url` == `SANDBOX_NODES_JSON[].publicDataUrl`;
   - `ws://` 只允许 loopback;浏览器会把 host 换成页面 host,端口保持 8443。
4. 空数据库初始化：`scripts/migrate.sh` 或 `npm --prefix web run db:migrate`，使用唯一全量基线；不提供旧库转换，不手工逐份执行旧 SQL。靶场环境、持久目录与网络启用见 [Compose 部署说明](compose-platform-integration.md)。

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

- Nuxt：在 `/etc/attackbox/web.env` 设置 `HOST`、`PORT`（默认 `127.0.0.1`、`3000`）。请使用单行 `KEY=value`，可加单引号或双引号；不要另设 `NITRO_HOST`、`NITRO_PORT`。
- Worker：在 `configs/node.yaml` 设置 `gateway.listen_addr`。浏览器连接地址由 `gateway.public_data_url` 与 Web 的 `SANDBOX_NODES_JSON[].publicDataUrl` 对应设置。
- Worker 的 `node.control_url` 指向 Nuxt 的实际监听地址及 `/cluster/connect`。
- nginx 的网站入口端口由 `deploy/nginx/attackbox.conf.template` 的 `listen` 设置，默认 80；入口端口或协议改变时，同步调整域名与 `APP_BASE_URL`。

部署脚本读取上述本机监听配置，生成 nginx upstream 与 Web／Worker 健康检查地址，无需手工修改检查端口。通配监听地址转换为对应的 loopback 地址；Worker 启用 TLS 时，通过 `public_data_url` 对应的 HTTPS 地址检查，保持证书校验。生成文件位于临时目录，部署结束删除，不包含邮件或数据库密钥。
