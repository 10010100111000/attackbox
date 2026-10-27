# AttackBox

网络安全学习与容器沙箱平台。`web/` 是 Nuxt Control，`worker/` 是 Go Worker；两端可以同机或分机部署。

## 目录与职责

| 目录 | 职责 |
| --- | --- |
| `web/` | Nuxt 页面、API、PostgreSQL 迁移、控制通道 |
| `worker/` | 独立 Go 模块：沙箱、Docker 靶场、网关和网络 |
| `deploy/control/` | Web 环境配置参考、systemd、nginx 模板 |
| `deploy/worker/` | Worker YAML 参考、systemd |
| `scripts/build.sh` | 普通用户构建：`all`、`control`、`worker` |
| `scripts/control.sh`、`worker.sh` | 稳定角色入口，按需 sudo 调用 Python |
| `scripts/deploy.py` | 配置解析、初始化、迁移、管理员创建、发布与失败回退 |
| `scripts/requirements.txt` | Python 配置工具依赖（PyYAML 6.x） |

构建不读取运行密钥、不操作线上服务；部署只使用已构建产物，不拉取 Git、不重新编译、不自动迁移数据库。

## 依赖与宿主机准备

部署模板面向使用 systemd 的 Debian/Ubuntu，采用 nginx sites-enabled 布局。需要 Python 3.10+、PyYAML、Node（满足 `web/package-lock.json` 中依赖的要求）、Go 1.23+、PostgreSQL、nginx、sudo。Worker 还需要 root、Btrfs、cgroup v2、iproute2、nftables，以及已准备好的 `/var/lib/attackbox/base_rootfs` 金盘；使用靶场时安装 Docker 与受支持的 Compose V2。

Debian/Ubuntu 可用 `sudo apt-get install python3-yaml` 安装 PyYAML。其他系统可在适当的 Python 环境中安装 `scripts/requirements.txt`，确保 sudo 使用的解释器也能导入 `yaml`。脚本不安装系统依赖、不创建数据库角色、不申请 TLS 证书。

先创建空数据库和专用数据库用户。Nuxt 使用专用系统账号 `attackbox`；构建产物归 root 所有，应用只读。内容与靶场包默认存入 `/var/lib/attackbox-web/{content,labs}`，由服务账号持有，权限 0700。自定义 Web 持久化目录须位于 `/var/lib/` 或 `/srv/`，不能是符号链接。

## 首次同机部署

以下采用本机 HTTP 联调地址；公网部署见后面的 HTTPS 配置。

```bash
./scripts/build.sh all
sudo ./scripts/control.sh setup
sudo ./scripts/control.sh migrate
sudo ./scripts/control.sh admin
sudo ./scripts/worker.sh setup
./scripts/control.sh deploy
./scripts/worker.sh deploy
```

- `setup` 隐藏输入数据库连接；集群、签名、内容加密、判题密钥独立随机生成，不打印。
- 实际配置：`/etc/attackbox/web.env`、`/etc/attackbox/worker.yaml`，默认权限 0600；不提交 Git。
- 已有配置直接拒绝覆盖，不提供 `--force`。日常部署不重新生成密钥、不重置管理员。
- 管理员密码隐藏输入、二次确认，仅保存哈希；已有管理员或邮箱时拒绝创建。
- Resend 可稍后填写 `RESEND_API_KEY` 与 `MAIL_FROM`，邮件未配置时无法完成注册验证及密码找回。
- 同机 Worker 自动读取 Control 的集群密钥、默认节点网关和实际 Nuxt 监听端口；部署时会核对两端节点 ID、公开网关地址和密钥。

配置文件建议使用 `sudoedit` 编辑。环境文件采用单行 `KEY=value`，工具生成 JSON 双引号字符串；支持常用单／双引号，不执行变量展开或 Shell。只配置 `HOST`、`PORT`，不要额外设置 `NITRO_HOST`、`NITRO_PORT`。

## 公网 HTTPS 与 Worker 网关

公网启用邮件验证时 `APP_BASE_URL` 必须为 HTTPS；未配置邮件时可先用 HTTP 完成初始部署。nginx 模板默认监听 80，不自动配置公网证书：请在 `deploy/control/nginx.conf.template` 中配置自己的 HTTPS 入口，或在外层反向代理终止 TLS；保留唯一的 `@@WEB_UPSTREAM@@` 占位符。`/cluster/connect` 已配置 WebSocket Upgrade，供异机 Worker 建立控制通道。

例如，Control 与 Worker 同机、域名相同，Worker 直接提供 TLS：

```bash
sudo ./scripts/control.sh setup --origin https://learn.example.com
sudo ./scripts/worker.sh setup \
  --tls-cert /etc/letsencrypt/live/learn.example.com/fullchain.pem \
  --tls-key /etc/letsencrypt/live/learn.example.com/privkey.pem
```

这是首次初始化示例，已有配置请编辑，不要再次初始化。Worker 默认公开地址是 `wss://learn.example.com:8443`，需要开放该端口，证书须覆盖该主机名。

若 Worker 前方的反向代理负责 TLS，可显式使用 `--tls-terminated`，并按代理拓扑配置 `--public-url` 和 `--listen-addr`。该选项只声明已有代理，不创建代理规则。TLS 网关健康检查使用公开 HTTPS 地址并校验证书；明文／代理后网关使用本机实际监听地址。不会关闭证书验证。

## 异机与多 Worker

Control 和 Worker 必须使用相同节点 ID、公开网关地址及集群密钥。首次只有一个远程节点时，Control setup 可以指定 `--node-id` 与 `--worker-public-url`。

为已有 Control 添加另一个节点：

```bash
sudo ./scripts/control.sh node add \
  --node-id node-worker-01 \
  --public-url wss://worker.example.com:8443 \
  --max-slots 16
sudo systemctl restart attackbox-web
```

此命令只增加节点配置，重复 ID 拒绝覆盖。集群密钥通过安全的运维渠道复制到 Worker 的权限 0600 凭据文件，不在终端打印或放进脚本日志。

在远程 Worker 上构建或放置与该节点匹配的 `dist/attackbox-worker` 后：

```bash
sudo ./scripts/worker.sh setup \
  --node-id node-worker-01 \
  --control-url wss://learn.example.com/cluster/connect \
  --public-url wss://worker.example.com:8443 \
  --cluster-secret-file /root/attackbox-cluster-secret \
  --tls-cert /path/to/fullchain.pem \
  --tls-key /path/to/privkey.pem
./scripts/worker.sh deploy
```

Worker 本地健康检查通过仅表示网关可用，还需在 Control 后台确认节点已注册在线。异机脚本无法读取远程 Control 配置，因此节点名单由部署者先添加。

Compose 默认不启用。先完成节点上沙箱／靶场互通与隔离验收，再初始化时同时传入 `--enable-compose --network-verified`，或明确编辑现有配置；普通启用开关不会替你宣布网络已验收。

## 非交互安装

推荐通过权限 0600 的普通文件传递凭据，避免命令历史与进程参数暴露：

```bash
sudo ./scripts/control.sh setup \
  --database-file /root/attackbox-database-url \
  --origin https://learn.example.com
sudo ./scripts/control.sh admin \
  --email admin@example.com \
  --password-file /root/attackbox-admin-password \
  --name Administrator
```

也支持 `--cluster-secret-file`、`--resend-key-file`。凭据文件仅在执行时读取，不复制到发布产物；管理员密码不会保存在配置中。直接值参数仍可使用，但不推荐用于秘密信息。

## 日常发布与回退

```bash
git pull --ff-only
./scripts/build.sh control
sudo ./scripts/control.sh migrate
./scripts/control.sh deploy

./scripts/build.sh worker
./scripts/worker.sh deploy
```

Control 先暂存产物、检查 nginx 配置，再停止服务切换目录并重启；首次部署会显式启动 nginx。健康检查失败时恢复上一版产物、nginx 配置及服务单元。成功后保留 `/var/www/attackbox-web.prev`，下一次发布会替换这个备份。本方案有短暂重启窗口，不承诺零停机。

Worker 安装二进制前先执行只读 `-check-config`，替换采用临时文件重命名。失败时恢复旧二进制和服务单元；成功后保留 `/usr/local/bin/attackbox-worker.prev`。Worker 重启会影响正在运行的沙箱，应安排维护窗口。

回退只处理运行产物和服务配置，不回滚 PostgreSQL 迁移、不恢复运行中的沙箱会话。升级数据库前应备份数据；内部密钥须和数据库一起持久保存。自定义 nginx 配置修改模板后再发布，直接编辑生成文件会被下一次部署覆盖。

## 本地检查

```bash
python3 -m unittest discover -s scripts/tests -v
bash -n scripts/build.sh scripts/control.sh scripts/worker.sh
(cd worker && go test ./...)
npm --prefix web test
```

部署回退测试在临时目录中模拟 systemd 与失败的健康检查，不连接服务器、不操作本机系统服务。

## 学习界面说明

学习、挑战和个人中心使用同一套桌面内容网格；个人中心主内容列对齐导航「学习」，侧栏位于左侧辅助列。窄屏按内容宽度折叠。讨论入口提供占位页，尚无发帖功能。

目录请求采用 45 秒的 Nuxt 应用内短缓存，切页不等待接口；首次读取显示骨架，过期后下次进入重新获取，不进行自动轮询。挑战缓存按用户身份区分。

初始化数据库仍包含六个历史预置分类，本轮不自动删除它们。学习首页筛选仅根据已发布模块的实际分类生成，不请求完整分类表，不展示空分类。

本机浏览器回归可通过 `CONTENT_TEST_DATABASE_URL` 和 `CONTENT_TEST_BASE_URL` 指定隔离的 `_test` 数据库及运行中的 Web，再执行 `npm --prefix web run test:learning-shell`。
