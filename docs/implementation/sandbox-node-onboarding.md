# Nuxt 沙箱接入与运行

更新日期：2026-10-24。

## 已实现范围

- 第一批：Worker 反向注册、鉴权、公钥同步、心跳观测与管理员节点页。
- 第二批：稳定的用户身份映射、实例唯一性、槽位和资源分配、创建/释放 RPC、心跳核对、CLI Ticket、学习页多终端与管理员运行实例页。

浏览器通过 `/api/sandbox/*` 使用新平台登录态，由服务端决定用户、节点和策略；终端流量凭短期 Ticket 直连 Go Worker。Go Worker 协议与 RuntimeManager 保留，Go Control 尚未移除。GUI/noVNC 尚未接入；选择 GUI 不启动另一个环境。

## 部署与配置

节点接入和实例调度默认均关闭。学习平台可以独立运行。

在 `web/` 使用部署环境的 `DATABASE_URL` 执行 `npm ci`、`npm run db:migrate`、`npm run build`，然后重启应用。本批增量迁移 `0003_sandbox_identity.sql`，无需清库。新增 `sandbox_identities` 将新平台 UUID 用户映射到 Worker 所需的正整数标识，唯一约束与自增序列防止并发首次分配重复。释放、重建实例不会更换用户标识。

| 服务端环境变量 | 用途 / 默认值 |
| --- | --- |
| `SANDBOX_CONTROL_ENABLED=true` | 启用节点接入；默认关闭 |
| `SANDBOX_SCHEDULING_ENABLED=true` | 启用创建、释放及未知实例清理；默认关闭 |
| `SANDBOX_CLUSTER_SECRET` | 至少 32 字节的独立随机集群身份，与 Worker `cluster_secret` 一致 |
| `SANDBOX_SIGNING_SEED_HEX` | 32 字节 Ed25519 seed 的 64 位十六进制编码；持久保存 |
| `SANDBOX_NODES_JSON` | 节点白名单及槽位上限，见下 |
| `SANDBOX_TTL_SECONDS` | 默认 7200，范围 60–86400 秒 |
| `SANDBOX_IDLE_SECONDS` | 默认 1800，范围 60 秒至 TTL；TTL 缩短时同步设置 |
| `SANDBOX_MEMORY_MB` | 默认 2048，范围 128–65536 MB |
| `SANDBOX_PIDS_MAX` | 默认 512，范围 32–4096 |

```json
[{"id":"learning-node-1","label":"学习节点 1","publicDataUrl":"wss://worker.example.com","maxSlots":4}]
```

`maxSlots` 默认 4，范围 1–250。数据地址必须为无路径、账号、查询参数的 `wss://host[:port]`；仅本机联调允许 `ws://localhost` 或回环地址。Worker 的 `node_id`、`gateway.public_data_url` 必须与配置一致。

CPU 权重 100、Swap 上限 0、I/O 写入 20 MiB/s 和 500 IOPS、临时目录 512M、运行目录 256M，危险 proc/sys 和 sysrq 限制由服务端统一下发。本批未开放浏览器修改资源配额或节点策略的配置表单。

密钥通过部署系统或私有 `.env` 注入，不进入管理员 API、`runtimeConfig.public` 或 Git。分别使用 `openssl rand -hex 32` 生成集群身份与 seed，不重复使用联调值。

Worker 的 `control_url` 指向 `wss://platform.example.com/cluster/connect`。反向代理转发 WebSocket Upgrade，保留 `Authorization` 和 `X-Node-ID`；不能套浏览器登录跳转，也不要通过查询参数传集群密钥。Worker 数据网关需通过 `publicDataUrl` 对浏览器可达，允许前台站点 Origin，转发 `/ws/cli` WebSocket。

**先使用专用测试 Worker。** 注册会下发新平台验签公钥；启用调度后会清理无法确认归属的实例。仍由 Go Control 管理的节点不能直接同时接入两个控制端。

部署要求单个常驻 Node/Nitro 进程。不要开启多进程轮询、水平副本或 Serverless；节点连接和 API 必须使用同一份控制状态。水平扩展需要后续独立控制服务或共享消息层。

## 生命周期与恢复

运行记录保持瞬态，存于当前控制进程内；数据库只保存用户身份映射。本项目沿用原有 Control 重启不保留实例的策略，不持久化 PID、槽位历史或运行快照。

- **分配**：完整有效心跳后才可选择节点。未知实例清单核对完成前不调度。按节点槽位上限、磁盘至少 5 GB 和内存余量检查资源；为正在创建的实例扣除预留内存。用户与槽位在发送 RPC 前同步占用，并发启动不会分配第二份实例。
- **创建确认**：校验同一连接、boot id、sandbox id、实际到期时间和清单版本。客户端提供的 userId、slot、policy 不参与分配。创建失败、超时或连接断开都保留责任记录并发起幂等补偿清理。
- **释放确认**：只有 Worker 的成功响应，或不早于已确认创建版本的完整缺失清单，才清除本地占用。失败保留 `cleanup_failed`，后续心跳继续重试。停止不会被迟到的创建响应改回运行中。
- **TTL/Idle**：实际判定与回收由 Worker 执行；API 查询与页面轮询不续活。到期时间取 Worker 响应，不用浏览器计时替代服务端判定。
- **普通断线**：保留占用，不在未知状态下创建第二份实例。Worker 同一 boot 重连保留 seq/revision；旧连接及迟到心跳不能覆盖新状态。
- **Worker 重启**：Worker 负责清理残留，启动后换 boot id；首份新心跳使旧代次运行记录失效。
- **Control 重启**：运行记录丢失，Worker 重连后先销毁未知实例；等待后续完整清单确认后再分配。界面上仅“在线”不代表拥有可调度资源。

消息正文上限 1 MiB；注册与首份心跳期限 10 秒，正常心跳超时 15 秒，巡检周期 3 秒，RPC 默认等待 30 秒。业务拒绝消息不会回显 Worker 原始内部错误。

## API 与权限

| 接口 | 职责 |
| --- | --- |
| `GET /api/sandbox/status` | 本人实例状态与是否已启用调度 |
| `POST /api/sandbox/start` | 幂等启动或复用本人实例；不接受客户端资源策略 |
| `POST /api/sandbox/stop` | 携带 `sandboxId` 释放本人实例 |
| `POST /api/sandbox/ticket` | 携带 `sandboxId` 获取本人运行实例的 PTY Ticket |
| `GET /api/admin/nodes` | 管理员查看节点及 Worker 上报清单 |
| `GET /api/admin/instances` | 管理员查看平台已知实例、用户归属与资源状态 |
| `POST /api/admin/instances/:id/stop` | 管理员释放指定实例 |

沙箱接口要求 active 登录账号；管理员接口额外检查角色。Ticket 绑定 Worker 数字用户标识、实例和 `pty` 流，最长 60 秒且不超过实例实际到期时间；使用 Ed25519 签名与随机 JTI，由 Worker 验签并一次性核销。节点离线、实例未就绪或已到期时不签发。没有 VNC Ticket 或浏览器可读的签名密钥。

所有实例、Ticket 及管理员响应为 `private, no-store`。写操作拒绝跨站 Origin / `Sec-Fetch-Site: cross-site`。不能通过修改地址、请求体用户 ID 或前端按钮绕过权限。Worker 所有者身份不由浏览器指定。

## 界面行为

学习正文保留唯一的 CLI/GUI 入口。首次使用明确点击“启动环境”；已运行时直接获取新 Ticket 连接原实例。多个终端各用独立 win id 和 PTY，共用 home、网络和配额。

同一学习页面中切换终端、收起或关闭面板、切换 CLI/GUI 均保留现有终端连接，不释放实例。关闭某个终端页签会结束该 Shell；离开章节页面、刷新或断网后当前 Worker 也不能保留原 PTY。重连只获取 Ticket，不隐式创建沙箱，并明确提示“新 Shell；文件仍保留”。连接失败按退避自动重试，接管关闭不自动抢回。

管理员“运行实例”显示当前进程已知的用户、实例、节点、IP、状态、启动/到期与最近活动；提供释放和失败重试。不展示虚构累计时长、历史实例或使用量画像。未知实例在节点清单中核对，不伪造用户归属。

## 验证

```bash
cd web
npm test
npm run test:content
npm run build
npx tsc --noEmit -p tsconfig.json
```

数据库和浏览器集成用例为显式启用：`CONTENT_TEST_DATABASE_URL` 必须指向本地、名字以 `_test` 结尾的独立库，`CONTENT_TEST_BASE_URL` 指向本地构建服务，`NODE_TEST_CLUSTER_SECRET` 与服务一致；测试自行创建和清理账号与模块。

- `npm run test:nodes-browser`：**观察模式**服务，节点 `test-node`，数据地址 `ws://127.0.0.1:18443`；验证注册、心跳、后台权限和状态页。
- `npm run test:sandbox-browser`：**调度模式**服务，同样配置测试节点，18443 端口空闲。测试程序提供本地 Worker 协议和网关，模拟终端输出，不执行真实沙箱命令；验证身份持久化、并发幂等启动、越权/跨站拒绝、一次性 Ticket、多终端输入输出、断线重连、面板保活、用户与管理员释放。可设置 `SANDBOX_SCREENSHOT_DIR` 保存截图。
- 上述 CLI 浏览器测试可传 `GO_TEST_BINARY` 指向 Go 工具链，通过私有临时文件将短期测试 Ticket 交给仓库实际 `protocol.VerifyTicket` 验证，临时文件随后删除。
- `NUXT_CONTROL_TEST_URL=ws://127.0.0.1:18916/cluster/connect NODE_TEST_CLUSTER_SECRET=… go test ./protocol -run TestNuxtNodeControlCompatibility -count=1`：使用现有 Go 注册和心跳结构验证互通，不与其他同 id 节点测试并行运行。

本地协议与浏览器联调不能代替真实 Worker 生命周期验收。本批未部署或切换生产节点，未宣称已验证 Btrfs、Cgroup、网络命名空间与实际 PTY。

## 下一批

1. 在专用 Worker 上验收创建、真实命令、限额、并发、清理失败、TTL/Idle、控制断线与重启收敛。
2. 同实例接入 GUI/noVNC，增加节点排空与运行策略配置。
3. 完成等价验收后再迁移节点并移除 Go Control；每个节点迁移期间始终只有一个控制端。
