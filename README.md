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

- `setup` 隐藏输入数据库连接；集群、签名、内容加密密钥独立随机生成，不打印。
- 实际配置：`/etc/attackbox/web.env`、`/etc/attackbox/worker.yaml`，默认权限 0600；不提交 Git。
- 已有配置直接拒绝覆盖，不提供 `--force`。日常部署不重新生成密钥、不重置管理员。
- 管理员密码隐藏输入、二次确认，仅保存哈希；已有管理员或邮箱时拒绝创建。
- Resend 可稍后填写 `RESEND_API_KEY` 与 `MAIL_FROM`，邮件未配置时无法完成注册验证及密码找回。
- 同机 Worker 自动读取 Control 的集群密钥、默认节点网关和实际 Nuxt 监听端口；部署时会核对两端节点 ID、公开网关地址和密钥。

配置文件建议使用 `sudoedit` 编辑。环境文件采用单行 `KEY=value`，工具生成 JSON 双引号字符串；支持常用单／双引号，不执行变量展开或 Shell。只配置 `HOST`、`PORT`，不要额外设置 `NITRO_HOST`、`NITRO_PORT`。

## 公网 HTTPS 与 Worker 网关

同机部署且已接入 Cloudflare 的站点，优先参考 [Cloudflare 部署指南](docs/deployment/cloudflare.md)：统一 443 入口、复用已有证书、Turnstile、登录限流与源站限制。

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

`gateway.listen_addr` 是 Worker 的监听地址，`gateway.public_data_url` 是浏览器实际连接的入口；Control 的 `SANDBOX_NODES_JSON.publicDataUrl` 与后者保持一致。CLI／GUI 直接使用服务端签发的地址，不根据当前页面主机名替换它。域名大小写、默认端口和 IPv6 写法会规范化，主机与端口的含义保持不变。

HTTP 站点可以明确配置 `ws://Worker域名或IP:端口`；HTTPS 站点必须配置 `wss://`。例如 `control.sh setup --origin http://192.168.1.20` 会生成 `ws://192.168.1.20:8443`；异机 Worker 通过 `--worker-public-url` 指定自己的入口。公开地址不能使用 `0.0.0.0`／`[::]`，它们只用于监听；`127.0.0.1`／`localhost` 指向学员浏览器所在的机器。

已有 HTTP 部署若曾依赖回环地址替换，需要同时将 `web.env` 中的节点公开地址和 `worker.yaml` 中的 `gateway.public_data_url` 改为浏览器可达地址，重启两端。现有配置不会自动覆盖。Cloudflare 同域入口继续使用 `wss://microsb.de`，Worker 仍可在本机回环地址监听，由 nginx 转发。

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

当前 Control 使用单进程调度，CLI／GUI 沙箱租约保存在内存中，不支持 Web 重启后接管原沙箱。启用调度时，Web 重启后 Worker 上报的未知沙箱会被清理，因此发布 Web 也应安排沙箱使用的维护窗口。这是当前明确的回收策略，不能只凭心跳中的实例 ID 恢复用户归属与租约。账号封禁以数据库为准，不依赖这份内存状态。

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

教程标准答案与挑战 Flag 只保存一份明文，判题在服务端按题目的匹配规则执行，不依赖额外判题密钥。学员接口和 SSR 页面不返回标准答案；已完成记录不会因管理员修改答案而清除。`0002_plain_answers.sql` 只移除冗余摘要、密钥版本及对应约束，保留已有明文和学习进度；历史迁移文件保持不变。

准备任务的状态查询通过单条数据库快照读取模板和对应版本的任务，不竞争任务变更的全局锁；查询 Worker 后仍按任务标识与版本条件更新，迟到响应不能覆盖新任务。创建、取消、重建与模板变更继续使用短事务串行化，镜像准备和 Worker 通信在事务外执行。

### 请求安全与缓存

`web/server/utils/request-policy.ts` 统一维护请求来源与响应缓存策略，不再按业务功能维护路由白名单。所有 `/api` 写操作默认校验浏览器 Origin 和 Fetch Metadata；无浏览器来源信息的服务端调用仍须通过接口自身的身份与权限检查。接口中的管理员、学员及资源归属鉴权继续负责访问控制，来源校验不能替代它们。

API 响应与服务端渲染的 HTML／Nuxt payload 默认使用 `private, no-store`，合并并去重 `Vary: Cookie`；带内容哈希的 JS、CSS 等静态资源保留框架的长期缓存。此策略与浏览器内的目录短缓存相互独立。新增 API 自动纳入规则，不需要追加功能路径。使用隔离测试数据库和运行中的 Web 可执行 `npm --prefix web run test:request-policy` 验证来源拦截、鉴权、用户响应隔离及静态资源缓存。

## 学习界面说明

公共按钮、提示、徽标和空状态由 `web/components/ui` 管理外观与交互；后台页面样式使用 `scoped`，仅管理页面布局与专用元素。用户管理样式随用户页面加载，不再注入整个后台布局。

`npm --prefix web run test:ui-components` 使用本机 Chromium 验证真实 Vue 组件在两种主题下的颜色、链接禁用行为、路线保存保护与通知提示，不依赖数据库或 Worker。完整服务联调仍使用下述独立测试数据库。

学习、挑战和个人中心使用同一套桌面内容网格；个人中心主内容列对齐导航「学习」，侧栏位于左侧辅助列。窄屏按内容宽度折叠。讨论入口提供占位页，尚无发帖功能。

目录请求采用 45 秒的 Nuxt 应用内短缓存，切页不等待接口；首次读取显示骨架，过期后下次进入重新获取，不进行自动轮询。挑战缓存按用户身份区分。

数据库只保留全新安装基线 `0000_platform.sql`，使用空数据库初始化，没有旧库转换或预置分类。分类由 Git 教程内容产生，最后一个模块引用移除后自动清理。首页筛选只来自已发布模块。

本机浏览器回归可通过 `CONTENT_TEST_DATABASE_URL` 和 `CONTENT_TEST_BASE_URL` 指定隔离的 `_test` 数据库及运行中的 Web，再执行 `npm --prefix web run test:learning-shell`。

页面等待统一使用 `UiBusyIndicator`（轻量轨道图标）、`UiBusyLabel`（按钮尺寸不变）和目录骨架。指示器延迟约 180ms 展示，短操作保留原按钮文字；屏幕阅读器仍能获得状态提示，减少动态效果模式停止旋转。错误、完成结果和实际业务状态保留明确说明。学习页页脚处于正常文档流：短页靠底，长页随内容向后，不固定遮挡正文。

个人中心支持昵称与密码修改。修改密码需验证当前密码，保留本次会话、撤销其他设备会话并作废未使用的密码重置凭证。

通知与账户资料共用个人中心布局。收件箱显示发送者，删除仅隐藏自己的消息；平台通知撤回仍由管理员负责。数据结构区分平台广播和定向消息，并按接收人鉴权。当前只开放管理员发布平台通知，暂不提供会员互发接口；后台通知列表也不会列出他人的定向消息。

### 落地页内容

`/` 使用服务端渲染，展示数据库中最新发布的最多 6 个模块（首个为主卡片）；名称、简介、分类、封面、章节数和时长都来自真实内容。不配置虚拟模块，空库展示简洁空状态。发布或下架会在后续首页请求中生效，不需要重新构建。首页按钮进入 `/learn`，卡片进入公开模块详情；章节阅读仍沿用原有权限控制。构建阶段仅预渲染 `/auth`，正式部署需运行 Nuxt 服务端，不能只托管静态目录。

首页与公开模块详情返回完整 HTML，便于搜索引擎读取；登录页设置 `noindex, follow`。数据库异常时首页保留品牌与浏览入口并返回 503，避免将故障误作正常空内容。

### 站内确认交互

释放环境、停止靶机、删除数据及未保存修改的离开保护统一使用 `useConfirm()` 和全局 Reka UI AlertDialog，适配石墨／深海主题。新增操作需提供具体标题、影响说明和确认按钮名称；取消、Esc、调用组件销毁或账号切换不执行操作。不使用原生 `confirm()`／`alert()`；关闭浏览器标签页时的 `beforeunload` 提示仍由浏览器处理。

### 阅读与练习分屏

学习正文与挑战详情共用练习面板布局。工作区可用宽度达到 1000px 时默认右侧分屏，支持左右拖拽；也可在面板标题栏切换到底部停靠并调整高度。布局与尺寸保存在浏览器本地，窄屏临时回到底部，恢复宽屏后继续使用原选择。分隔线支持方向键和 Home／End，正文与练习区独立滚动；切换布局及收起恢复保留 CLI／GUI 连接。远程桌面自适应面板，服务端支持时同步调整桌面分辨率，否则使用缩放适配。

### 章节小节与剪贴板

学习目录按 Markdown `#` 章节及当前章节正文的 `##` 小节组织，小节可以收起并随正文滚动高亮；点击更新 URL 锚点，不请求其他章节正文。阅读位置按账号和章节保存在本机，显式锚点优先于位置恢复。目录不加载锁定章节的小节。

教程正文与题目不显示代码复制按钮，并阻止普通复制／剪切事件；仍允许选中文本，不做防绕过处理。答案输入、靶机操作和 CLI／GUI 不受限制。CLI 选中文本后可用 Ctrl+Shift+C（macOS 使用 Command+C），粘贴沿用终端原生行为。GUI 标题栏提供文本剪贴板，显式读取／写入浏览器剪贴板及发送至远程桌面；HTTP 或浏览器权限限制下，可以在文本框中手动复制、粘贴。剪贴板内容只保留在当前页面内存中。

参考资料已实现独立的公开资料库：顶级导航负责浏览，教程通过普通 Markdown 链接引用同一资料及小节。资料没有测验、解锁或沙箱。模块全文搜索尚未实现。

### 用户反馈

登录后从头像菜单打开「问题反馈」，选择遇到问题、内容纠错或功能建议，填写内容后提交。弹窗关闭后保留未提交文字，成功后显示确认；支持石墨／深海主题。只附上当前页面路径，不保存查询参数、URL 锚点或额外的邮箱快照。

后台「账户管理 → 用户反馈」可查看提交者和相关页面，按未处理、处理中、已解决筛选并更新状态。不自动轮询，不向前台公开其他用户的反馈；每个账号每 10 分钟最多提交 5 条。第一版不提供附件、公开评论或多轮工单。

本轮新增 `0001_feedback` 建表迁移，部署新版前执行 `sudo ./scripts/control.sh migrate`，再运行部署命令；匹配当前基线的数据库会保留已有数据，无需清库。

### 参考资料的 Git 发布流程

后台「参考资料 → 仓库设置」配置 GitHub 仓库、分支、资料目录和可选的只读 Token。私有仓库 Token 使用现有 `CONTENT_CREDENTIALS_KEY` 加密，与教程的仓库配置分别保存；公开仓库可以不填 Token。两个目录不得互相包含，例如同一个仓库中分别使用 `learning/` 和 `references/`，避免资料被当成教程解析。

```text
my-content/
  learning/
    Linux 基础.md
  references/
    网络协议笔记.md
    images/
      handshake.png
```

一份 `.md` 对应一篇资料，标题取文件名。正文使用普通 Markdown，`##` / `###` 为目录；可选配置头仅含 `summary`，最多 500 字，不需要手写 ID、权限或进度字段：

```markdown
---
summary: TCP 握手、状态与排障的阅读笔记。
---

## 建立连接

在此编写正文。

![握手示意](images/handshake.png)
```

后台可下载完整 MD 示例。支持子目录；忽略 README 和隐藏文件；不支持符号链接、子模块或仓库脚本执行。单篇正文最多 2 MiB，最多 64 张本地相对路径图片，每张最多 10 MiB、合计最多 64 MiB。图片必须在所选资料目录内，支持 PNG、JPEG、WebP、GIF、AVIF，不直接引用 GitHub 私有图片地址。普通链接使用站内绝对路径、锚点或 HTTP(S) 地址；暂不将仓库中的相对 `.md` 链接自动转换为本站资料链接。

点击「Git 导入」扫描提交，选择文件导入预览，再手动发布。导入更新保留资料 ID 与公开链接，替换单份内容并转为未发布；内容未改变时保留发布状态。导入时校验扫描提交、仓库配置版本及资料版本，冲突需重新扫描。源文件被移除时保留本地资料，由管理员决定下架或删除。改名／移动文件视为新资料，不自动推断身份，也不自动删除旧资料。

正文写入 PostgreSQL，图片复用本地不可变资源存储。访客只读取已发布资料与其图片；未发布内容只能通过管理员预览接口读取。`/references` 和 `/references/:slug` 使用 SSR，正文不依赖浏览器执行 JavaScript；更新发布不需要重新构建。

发布后复制后台显示的公开路径，即可在教程中写 `[网络协议笔记](/references/reference-实际ID#建立连接)`，也可引用外部资料。备份及换机时应同时保留数据库、`CONTENT_DATA_DIR` 与内容加密密钥；Git 仍为内容源。此轮数据库按单一全新安装基线初始化，不转换旧数据库。

### 账户资料与登录方式

账户资料使用单一设置面板，个人信息和登录安全保持相同字段布局。通过仅本人可读的 `/api/me/profile` 获取 `hasPassword` 能力标记，不返回密码哈希；只有设置了本地密码的账号显示修改入口。无本地密码不等同于已绑定某个第三方身份。

当前只实现邮箱密码登录；GitHub OAuth 和通用单点登录尚未接入，登录页不展示不可用的占位按钮。`web.env` 当前没有生效的 GitHub OAuth 配置项，Git 教程／资料同步的只读 Token 也不能用于用户登录。后续接入需要完整的 GitHub OAuth App、服务端回调和身份绑定流程，届时再增加仅服务端读取的 Client ID／Client Secret 配置。

### 注册邮箱白名单

新注册默认仅允许以下精确域名：`gmail.com`、`googlemail.com`、`outlook.com`、`hotmail.com`、`live.com`、`msn.com`、`icloud.com`、`me.com`、`mac.com`、`qq.com`、`foxmail.com`、`163.com`、`126.com`、`yeah.net`、`yahoo.com`、`aliyun.com`。邮箱验证仍然必需。白名单覆盖注册、重发激活邮件和激活确认；已有账号的登录与密码找回、CLI 初始化管理员不受影响。未通过验证的账号会按当前白名单重新检查。

无需新增安装配置即可使用默认名单。若要修改，在 `/etc/attackbox/web.env` 中添加 `AUTH_REGISTRATION_EMAIL_DOMAINS="gmail.com,outlook.com,qq.com,163.com"` 并重新部署／重启 Nuxt；配置值**替换**默认名单，不是追加。企业自有域名需明确列入。只支持逗号分隔的域名，不允许通配符或子域名自动继承；空值或无效值会关闭注册能力并返回配置错误，不会放开限制。

注册页从运行中的服务端读取公开名单，因此自定义域名在服务重启后生效，无需重新构建预渲染的登录页面。未来第三方登录创建新账号时也需要复用此策略；当前尚未接入第三方登录。

登录成功后使用经过站内路径校验的 `returnTo` 返回原页面；未指定或无效时进入 `/learn`，不增加登录成功中间页。主题只使用 `attackbox-appearance` 一个浏览器存储键。

`0003_content_cleanup` 删除废弃的 Git 导入 Preview 表，并将章节、题目与挑战阶段的位置唯一约束改为 `DEFERRABLE INITIALLY DEFERRED`。排序在原事务内直接写入最终位置，提交时仍拒绝重复位置，不再临时平移。Drizzle 未提供此约束属性的声明 API，因此延迟校验属性由 SQL 迁移明确维护，后续替换这些约束时需保留。

Web 运行时必须显式配置 `DATABASE_URL`；缺失或空白时拒绝启动，不回退到公网数据库或 PostgreSQL 默认环境配置。仅登录页预渲染允许无数据库构建，预渲染期间意外访问数据库也会明确报错。

### 靶场停止记录与恢复

Compose 停止按「资源释放 → 已停止状态原子落盘 → 确认删除网络账本 → 精简实例记录」执行。网络已释放但实例清理尚未提交时，网络记录仍保存在磁盘，Worker 重启可继续清理；确认删除操作可重复执行。停止后实例目录只保留 `record.json`，记录实例身份、原租约截止时间及停止状态，不保留可写文件或运行配置，也不删除镜像。原租约到期后由现有 Worker 回收循环删除停止记录和目录，不新增定时器。回放已停止实例不会重新创建容器；新的启动仍使用新实例 ID。持久化或清理失败会返回错误并保留恢复依据，不假报完成。
