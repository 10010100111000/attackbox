# Cloudflare：同机 HTTPS、Turnstile 与源站入口

模板需要 nginx 1.19.4+ 及 http_realip 模块（部署时由 `nginx -t` 校验）。适用于 Control 与 Worker 在同一台 VPS、`microsb.de` 已在 Cloudflare 配置 DNS 和证书的部署。复用现有证书与域名，不自动申请证书、修改 DNS 或操作防火墙。

## 连接路径

```text
浏览器 → Cloudflare 橙云 HTTPS :443 → nginx HTTPS :443
                                      ├─ 页面/API → 127.0.0.1:3000 Nuxt
                                      └─ /ws/cli、/ws/gui → 127.0.0.1:8443 Worker
Worker 控制通道 → ws://127.0.0.1:3000/cluster/connect
```

浏览器只访问同域名 443，沿用现有的会话与短期 CLI/GUI 票据。WebSocket 路由不绕过 Worker 票据校验。这个 nginx 模板不公开 `/cluster/connect`；未来拆分 Worker 时，应使用相应的多节点入口与防火墙策略。

## 1. 配置同域 Worker

**仅首次安装**，初始化时指定同域的公开网关地址：

```bash
sudo ./scripts/control.sh setup \
  --origin https://microsb.de \
  --worker-public-url wss://microsb.de
sudo ./scripts/worker.sh setup \
  --tls-terminated \
  --listen-addr 127.0.0.1:8443
```

已有配置不要重新 setup。使用 `sudoedit /etc/attackbox/web.env`：

- `APP_BASE_URL=https://microsb.de`，`HOST=127.0.0.1`、`PORT=3000`。
- `SANDBOX_NODES_JSON` 保留原节点 ID、名称与容量，将唯一节点的 `publicDataUrl` 改为 `wss://microsb.de`。
- 增加以下配置（证书路径替换为服务器上已有文件的真实路径）：

```dotenv
CLOUDFLARE_ENABLED=true
ORIGIN_TLS_CERT=/etc/ssl/attackbox/origin.pem
ORIGIN_TLS_KEY=/etc/ssl/attackbox/origin.key
WORKER_PROXY_UPSTREAM=127.0.0.1:8443
```

`sudoedit /etc/attackbox/worker.yaml`，保留其他业务和密钥配置，仅核对：

```yaml
node:
  # 保留原 node_id、cluster_secret
  control_url: ws://127.0.0.1:3000/cluster/connect
gateway:
  listen_addr: 127.0.0.1:8443
  public_data_url: wss://microsb.de
  tls_enabled: false
```

上面的 YAML 是字段示意，不要用它覆盖整份文件。TLS 由 nginx 终止，Worker 只监听本机。部署脚本会校验两端地址、密钥、loopback 监听与证书文件，避免误将 3000／8443 暴露到公网。

证书须覆盖 `microsb.de`，可以是现有的受信任证书或 Cloudflare Origin CA 证书。Cloudflare 的边缘证书与 VPS 上的源站证书是两件事；只在控制台启用了边缘证书，不代表 nginx 已经有源站证书。私钥保持 root 可读，不授予 Nuxt 账号读取权限。

## 2. 启用 Turnstile

在 Cloudflare → Turnstile 创建 **Managed** widget，主机名填写 `microsb.de`。将该 widget 的 Site Key 与 Secret Key 填入 `/etc/attackbox/web.env`，不提交 Git，也不要粘贴到聊天或截图：

```dotenv
TURNSTILE_SITE_KEY=Cloudflare颁发的SiteKey
TURNSTILE_SECRET_KEY=Cloudflare颁发的SecretKey
```

这两项无法自行随机生成，也不是 Cloudflare DNS API Token。Secret Key 只在服务端使用；公开配置接口仅返回是否启用和 Site Key。登录页是预渲染页面，加载后读取运行时配置，无需为了换 Key 重新构建。

- 两项都未设置：不展示验证，仍执行登录限流，适合本地或初始安装。
- 只配置一项：认证拒绝执行，不能静默降级。
- 两项已配置：登录和注册必须先通过验证，服务端复核 `success`、本站 hostname 及 `login`／`register` action。
- 令牌由 Cloudflare 负责一次性使用与有效期验证，失败后前端重置验证；SDK 加载或服务端验证异常均不能跳过。
- 同 IP 登录与注册合计每 5 分钟最多 60 次；同 IP＋邮箱登录每 15 分钟最多 8 次，成功登录清除该组合计数。超限返回 429 与 `Retry-After`。这不是永久锁定账号，也不能替代 Cloudflare WAF。

数据库使用 `auth_rate_limits` 表。计数键存储摘要，不存储明文 IP／邮箱；请求中限量清除一天前的旧计数，不新增后台定时器。本轮沿用全新安装基线，旧数据库不会自动升级；不要直接对现有用户数据执行清库。

## 3. Cloudflare 与源站检查

在已有 DNS／证书配置基础上核对：

1. `microsb.de` 的 A／AAAA 记录是橙云；删除或保护指向同源站、但无需公开的灰云记录。DNS 历史记录不会因打开橙云自动消失。
2. SSL/TLS 选择 **Full (strict)**，开启 Always Use HTTPS，允许 WebSocket。不要使用 Flexible。
3. 不要给 API、登录、个人中心、后台和需登录的正文配置 Cache Everything。若存在自定义全站缓存规则，对 `/api/*`、`/auth`、`/me*`、`/admin*`、`/notifications*` 和受保护的学习页配置 bypass；公开内容缓存以后单独规划。
4. 若使用 CSP，需要允许 `challenges.cloudflare.com` 的脚本与 iframe。不要缓存或自行反向代理 Turnstile SDK。
5. VPS／云防火墙仅允许 Cloudflare 官方 IPv4 **和** IPv6 网段访问 Web 的 80／443。3000、8443、PostgreSQL 不对公网开放；SSH／Tailscale 按现有运维来源保留。不要清空宿主机规则或覆盖 Worker 的 nftables。

模板在 nginx 层也检查**实际 TCP 对端**是否为 Cloudflare，直连域名／IP 返回 444，未知 TLS SNI 拒绝握手；仅对可信 Cloudflare 对端采用 `CF-Connecting-IP`。再由 nginx 覆盖 `X-Real-IP`／`X-Forwarded-For`，Nuxt 默认只信 loopback 代理，避免伪造请求头绕过限流。

`deploy/control/cloudflare-ips.txt` 为随代码审查的官方范围快照，部署不联网更新。定期对照 https://www.cloudflare.com/ips-v4 与 https://www.cloudflare.com/ips-v6 更新该文件及云防火墙，重新 deploy 生效。nginx 拒绝请求不能替代边界防火墙；没有执行防火墙配置前，不应宣称源站已被网络层封闭。

## 4. 构建、部署和验收

```bash
./scripts/build.sh control
# 首次空库才执行 migrate；已有数据库需先确认本轮基线变更，勿盲目清库。
sudo ./scripts/control.sh migrate
./scripts/control.sh deploy
./scripts/worker.sh deploy
```

如果只更换 Key，`sudo systemctl restart attackbox-web` 即生效；更换 nginx 配置、证书路径或公开网关地址则重新 deploy。部署验证 nginx 配置失败会回退，不替换当前运行产物；已有证书自动续期后的 nginx reload 仍由原证书工具管理。使用 HTTP-01 续期时还需保留可经橙云访问的验证路径，或改用 DNS-01；模板不会自动创建 ACME 验证目录。

验收登录／注册的成功、失败、验证过期、脚本被拦截与重试；确认未通过验证不能直接 POST 登录。通过域名打开 CLI／GUI，浏览器网络面板的 WSS 地址应为 `wss://microsb.de/ws/...`；用外部网络验证直连源站失败，且 3000／8443 不可达。通过域名的正常访问需要成功，不能只测 444。

## 隐藏源站 IP 的边界

橙云隐藏 Web DNS 入口，入口限制阻止绕过 Cloudflare，但当前攻击沙箱与 Control 共用 VPS。只要允许沙箱访问外网，学员仍可能通过外部服务看到该 VPS 的公网出口 IP。完整隐藏 Control 源站需要分离 Worker 或配置独立出口；本轮不修改沙箱网络，也不承诺同机部署可完全隐藏公网 IP。

## 脚本预取与 503

若 JS 请求返回 `503`，响应头包含 `CF-Speculation-Refused: prefetch refused: not eligible`，且同一地址直接访问返回 `200`，表示 Cloudflare 拒绝了预取，不代表构建文件缺失。检查 Network 中该请求的 `Sec-Purpose`／`Purpose`，并区分普通脚本请求；普通请求也失败时，需要另查源站、缓存和部署产物。

Nuxt 构建通过 `build:manifest` 关闭额外的 `rel="prefetch"` 资源提示，保留当前页面的 `modulepreload`、正常模块导入及客户端路由预加载。重新构建并部署 Control 后生效，不需要修改 API 缓存或给后台开启全站缓存。Cloudflare Speed Brain 是独立的页面预取功能；若仍有它发出的文档预取被拒绝，可在 Cloudflare 后台关闭后复测。

## 公开参考资料的静态分块

重新执行 Control deploy，普通入口与 Cloudflare 入口会共用参考资料静态配置。公开文件默认位于 `/var/lib/attackbox-public/references`，可通过 `REFERENCE_PUBLIC_DIR` 配置独立目录。部署脚本为公开目录设置 nginx 可读权限，私有的 `CONTENT_DATA_DIR` 继续保持限制权限，不要将其整体开放。首次升级已有资料，需要在后台重新扫描、导入并发布；扫描会识别缺失的编译产物，不需要清库。

Cloudflare 缓存规则只匹配 `/static-refs/`：`meta.json` 绕过边缘缓存，带内容指纹的 HTML 块允许缓存并遵循源站的 1 小时 TTL，错误响应的边缘 TTL 设为 0。不要把 `/references/` 页面、API 或整站 HTML 一并设为 Cache Everything。HTML 与 `.gz` 预压缩文件由 nginx 直接提供；首屏 Nuxt 读取本地元数据与第一块，学习章节保持现有加载方式。

发布、下架和删除会同步切换或撤下源站的公开入口，服务启动时校准一次中断的发布。已经缓存的正文无法通过删除源站文件即时撤回；需要立即撤回时，手动清除该资料在 Cloudflare 的缓存，已下载到浏览器的内容仍无法收回。迁移与备份同时保留 `CONTENT_DATA_DIR` 下的私有构建产物和 `REFERENCE_PUBLIC_DIR`。
