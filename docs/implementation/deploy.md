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
| `deploy/nginx/attackbox.conf` | 单入口 nginx 配置 |
| `deploy/web.env.example` | Web 环境变量模板(复制到 `/etc/attackbox/web.env`) |
| `scripts/deploy.sh` | 服务器上的唯一部署脚本 |
| `scripts/migrate.sh` | 显式执行数据库迁移 |
| `scripts/deploy.ps1` | Windows 侧:push 后 SSH 调 `deploy.sh` |

## 首次准备

1. PostgreSQL、Docker、Go、Node、nginx 已安装。
2. `cp deploy/web.env.example /etc/attackbox/web.env`,替换全部 `CHANGE_ME`。
3. `configs/node.yaml` 与 `/etc/attackbox/web.env` 的节点配置必须一致:
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

脚本按顺序:校验干净工作区 → `git pull --ff-only` → 编译 Worker → `npm ci && npm run build` → 安装 unit/nginx → 发布到 `/var/www/attackbox-web` → 重启 → 健康检查(3000 / 8443 / Worker 注册 / nginx)。

**注意:重启 Worker 会销毁正在运行的沙箱。** 部署应安排在维护窗口;`/var/www/attackbox-web.prev` 保留上一版,可手工回滚。

## 回滚

```bash
sudo systemctl stop attackbox-web
sudo rm -rf /var/www/attackbox-web
sudo mv /var/www/attackbox-web.prev /var/www/attackbox-web
sudo systemctl start attackbox-web
```

## 当前部署边界

CLI 和 GUI 都通过 Worker 自身的 `__enter__`／`__enter_root__` 进入沙箱，不再构建、安装或查找独立 `attackbox-enter`。升级 Worker 后，旧 `/usr/local/bin/attackbox-enter` 不再被调用，可由管理员删除。

当前示例使用浏览器直连 Worker 8443；HTTPS 站点需配置可访问的 WSS 地址，也可以正确配置反向代理。这是部署选择，不是永久禁止 nginx 代理的约束。

部署前保留工作区修改；脚本仅接受干净工作区并执行快进更新。旧 `frontend/` 的构建产物和依赖不再参与部署，根目录已忽略这些本地残留，无需为其提交或清空工作区。
