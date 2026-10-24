# 单机部署

状态:唯一部署路径。Go control(`cmd/control`、`control/`、`cmd/attackboxd`)与旧 Vue SPA(`frontend/`)已退役并归档到 `backup/`,不再构建或安装。

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
4. 数据库初始化:`scripts/migrate.sh`(或手工 `psql`)。迁移有历史漂移时先核对 `drizzle.__drizzle_migrations`。

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

脚本按顺序:校验干净工作区 → `git pull --ff-only` → 编译 Worker 与 `attackbox-enter` → `npm ci && npm run build` → 安装 unit/nginx → 发布到 `/var/www/attackbox-web` → 重启 → 健康检查(3000 / 8443 / Worker 注册 / nginx)。

**注意:重启 Worker 会销毁正在运行的沙箱。** 部署应安排在维护窗口;`/var/www/attackbox-web.prev` 保留上一版,可手工回滚。

## 回滚

```bash
sudo systemctl stop attackbox-web
sudo rm -rf /var/www/attackbox-web
sudo mv /var/www/attackbox-web.prev /var/www/attackbox-web
sudo systemctl start attackbox-web
```

## 已退役(不要部署)

- 旧 Vue SPA 与旧 Go control 已移入 `backup/`(git 忽略、仅本地归档,不在仓库中):
  `backup/frontend-current/`(原 `frontend/`)、`backup/control/`、`backup/cmd/control/`、
  `backup/scripts/apply_db_docs.sh`;
- `cmd/attackboxd`(旧控制面)只在 `backup/cmd/attackboxd/` 中;
- `/usr/local/bin/attackboxd*`、`/usr/local/sbin/attackbox-fw`、`/usr/local/sbin/attackbox-net.sh`、`/usr/local/sbin/attackbox-pool`、`/usr/local/bin/attackbox-user`;
- `attackboxd.service`、`attackbox-fw.service`(后者会重建 `table inet attackbox`,可能清掉 Worker 的防火墙);
- `/var/www/attackbox` 静态旧前端、`/usr/share/novnc`、`/usr/local/share/attackbox/xterm`。

## 不要做的事

- 不要把 `ws://<公网 IP>` 写进 `publicDataUrl`(控制器只允许 loopback 用 `ws://`);
- 不要让 nginx 代理 `/ws/`;
- 不要把 `/usr/local/bin/attackbox-enter` 删掉(宿主侧终端依赖它);
- 不要用 `git pull` 覆盖脏工作区,`deploy.sh` 会直接拒绝。