# Compose 靶场：后台、Worker 与学习页面接入

状态：第三轮代码已实现。本地 PostgreSQL、模拟 Worker 与 Chromium 的端到端流程通过；真实 Docker Compose 生命周期另有本机验收。未连接 VPS，也未在当前无 NET_ADMIN 的执行环境开启真实 veth 接线。生产启用必须先完成第一轮网络验收。

## 管理员流程

后台“环境管理 → 靶场模板”默认新建 Compose 模板，旧单容器模板显式标注并保留原编辑入口，直到第五轮切换清理。

1. 上传 `.yml`／`.yaml` 或 ZIP。单文件规范化为 `compose.yml`，ZIP 保留相对目录，支持选择包内多个 Compose 文件中的一个。
2. 预览服务、镜像、网络、部署变量及缺失文件。指定主要入口服务、容器端口、沙箱接入网络；常见端口声明用于推断入口，仍可确认修改。
3. 保存未启用模板；打开新建页或仅上传包不创建空模板。
4. 对指定节点点击“准备部署”。返回已接收任务，不等待镜像下载；管理员手动查询结果和准备记录，没有页面轮询。
5. 至少一个节点准备成功后启用，学习模块／章节与挑战沿用原 `targetTemplateId` 关联。

`ports` 声明由官方 Compose 解析器识别，再转换为私有容器端口，不向宿主机发布；转换在预览与准备报告中说明。无需作者为普通端口映射手工重写文件。当前只支持 TCP／UDP，复杂范围受端口数量限制。

名称、简介、访问说明和启用状态不改变部署修订。部署包、入口配置或变量变化才产生新修订，必须重新准备；修改时先保存为未启用。已运行实例继续引用原部署与截止时间，不被替换或清除数据。内部修订用于防止错误复用，没有新增历史版本编辑器。

部署变量不回显，留空保留已有值或采用 Compose 默认值；已保存值可明确清除，避免把留空误当删除。新值通过 HTTPS 提交（localhost 联调例外），在数据库中用 AES-256-GCM 加密，AAD 绑定模板 ID。沿用 `CONTENT_CREDENTIALS_KEY` 的 32 字节环境密钥，和 Git 内容凭据使用不同 AAD。须与数据库一起备份该密钥；不要提交到 Git。

## 部署包与边界

- 压缩包／上传最大 8 MiB，最多 512 项；解压后总计 32 MiB，单文件最大 4 MiB，Compose 最大 1 MiB。
- 使用流式 ZIP 解压，拒绝目录穿越、重复路径、symlink、特殊文件和加密 ZIP；文件路径首版限 ASCII 字母、数字、点、横线、下划线和斜线。
- 服务文件引用相对 Compose 文件所在目录解析，不能超出整份部署包；不会把子目录里的 Compose 错当成根目录配置。
- 已新增 GitHub 导入，见 [Git 来源](lab-git-import.md)；本轮仍未实现源码构建。`build`、`include`、`extends`、profiles、env_file、host 网络、特权／设备、共享外部资源、可写／目录 bind 等仍明确不支持，见 [运行器支持范围](compose-runtime-local.md)。
- 服务／依赖与挂载最终由 Go 官方 compose-go 校验。网页预览提供及时提示，不作为运行权限或安全校验的替代。
- 包与准备产物私有保存，未作为 Nuxt public 或 Nginx 静态目录开放。尚无未引用包的自动 GC，管理员取消导入留下的私有包不会被误删；后续维护功能再收敛。

## Worker 下载与准备

部署包不塞进 WebSocket 帧。控制通道只传 Bundle ID、内容摘要、设置、变量和准备任务身份。

Worker 从自身配置的 Control URL 推导固定路径 `/api/worker/lab-bundles/:id`，携带既有 Bearer 集群凭据和 X-Node-ID 下载。远程仅 HTTPS，localhost 可 HTTP；不接受请求中的任意下载 URL，不跟随重定向。

控制端校验集群凭据、已配置节点和无浏览器 Origin。包响应 private/no-store，原始文件仅对受信 Worker 提供。登录学员或普通管理员 Cookie 都不能替代 Worker 身份。

双方按排序后的 `路径 + NUL + 文件 SHA256 + 换行` 计算包摘要。Worker 验证每份内容、总大小、路径和完整摘要，写入专用临时目录，准备后删除临时文件；不可变运行产物继续由第二轮运行器保存。

每节点一个准备任务，不持有实例生命周期全局锁。任务重复提交按身份去重，内容不能在同一任务 ID 上变化。任务状态与有限准备记录可手动查询；记录只含阶段和安全错误说明，不回传原始 CLI stderr、变量或完整 Compose 内容。进程重启后可由已有部署 ID 检查持久产物，未完成任务允许明确重试。

镜像默认先检查本机，缺失时在管理员准备阶段拉取，再固定镜像 ID。不会在学员启动时 pull/build。要更新已经缓存的相同 tag，当前需先在节点更新镜像，或在来源中采用新 tag／digest；尚未增加网页强制拉取或 Registry 凭据管理。

## 启用与部署

Nuxt：

```dotenv
LAB_DATA_DIR=/var/lib/attackbox/labs
CONTENT_CREDENTIALS_KEY=<保留既有的64位十六进制密钥>
```

`LAB_DATA_DIR` 应由 Web 服务用户拥有，专用目录权限 0700，置于发布 `.output` 之外。部署时执行 `npm --prefix web run db:migrate`，新增 `0013_compose_labs`：部署包、按节点准备记录、模板运行模式与修订、实例部署身份。旧模板默认仍为 container，不在线转换运行中的实例。

仓库默认 Web systemd 用户为 laji，首次部署可执行 `sudo install -d -m 0700 -o laji -g laji /var/lib/attackbox/labs`；自定义服务用户或路径时对应调整，不要把现有公共目录直接充作部署包目录。

Worker 的 `compose_labs` 默认关闭：

```yaml
compose_labs:
  enabled: false
  network_verified: false
  state_dir: /var/lib/attackbox/compose-labs
  socket_path: /var/run/docker.sock
  pool: 10.202.0.0/16
```

先安装并核对 Docker Engine 27.5+、Compose 稳定版 **>=2.40.3, <3.0.0**（已验证基线 2.40.3），在该主机完成 [第一轮真实网络验收](lab-network-local.md#真实通信夹具)。通过后才将 enabled、network_verified 设为 true。该标记是部署者的明确配置，不是程序自动推定网络已经验收。

迁移期间旧共享 Docker 网络占用 10.201.0.0/16，新模式默认使用不重叠的 10.202.0.0/16；仍会检查沙箱、已有 Docker、宿主路由和显式排除范围。按实际 VPN／局域网选择空闲池，不合并两套地址范围。

Worker 初始化核对 CLI 和持久账本；失败时不上报可用 Compose 能力，不偷偷回退旧单容器执行器。前端仍可管理包和模板，但不能对不兼容节点准备或启动。

## 学员与沙箱生命周期

学习和挑战继续使用同一 `/api/targets` 与 `TargetEnvironment.vue`。控制端从有权限的章节／已发布挑战解析模板，选择与攻击沙箱同机、对应修订已准备且 Boot ID 已确认的节点。Worker 重启后，管理员需手动确认准备状态以更新节点代次。

实例记录固定运行模式、部署 ID 与绝对租期，后续模板编辑不影响 Stop。重复启动复用原 ID／截止时间，Worker 不重复运行初始化服务。释放先核实归属；即使最初 Start 被容量限制拒绝，也可通过完整身份登记停止墓碑并确认项目资源不存在。

沙箱创建时记录平台 Owner UUID 与网络 namespace Device/Inode，在启动 Wait 回收子进程之前捕获身份，避免 PID 复用窗口。生命周期钩子用于接线和断开；控制端启动／查询靶场时也在沙箱生命周期锁内核对并接入当前沙箱。忙碌实例不阻塞 CLI 创建或销毁。

同一个 Worker 5 秒租期循环处理旧模式和 Compose；没有按容器、网卡新增定时器。网络核对在启动、沙箱修订变化或上次核对未完成时进行。Worker 与控制连接断开仍回收，到期数据库对账继续复用既有 15 秒循环。

启动与读取采用有界命令通道；停止另有保留容量，不会因启动通道已满而失去取消能力。未知节点、清理失败和没有账本都不等同于“已释放”。停止成功必须确认容器、项目卷与网络清理完成，镜像保留。

学员卡片显示主要入口、到期时间及可展开服务状态；接入后提供 lab 网卡的回连地址，避免误用 eth0 地址设置 LHOST。区分服务运行与沙箱尚未接入，不提供逐容器启停。无靶场周期查询，只在进入页面、明确操作及沙箱身份变化时读取；倒计时本地计算。账户切换会清理旧状态并丢弃旧身份的迟到响应。

## 接口

| 接口 | 权限／用途 |
| --- | --- |
| POST `/api/admin/lab-bundles` | 管理员上传二进制 ZIP／YAML，`X-Lab-Filename` 为 URI 编码文件名 |
| GET `/api/admin/lab-bundles/:id?file=...` | 管理员读取安全预览，不返回文件正文与变量值 |
| POST `/api/admin/compose-templates` | 首次保存模板 |
| PUT `/api/admin/compose-templates/:id` | 乐观锁保存，部署变化使准备修订失效 |
| GET `/api/admin/compose-templates/:id/preparations` | 只读数据库准备状态和节点能力，不自动查询 Worker |
| POST 同上 | 对指定节点执行 prepare／status |
| GET `/api/worker/lab-bundles/:id` | Worker 认证下载，不公开托管 |
| `/api/targets` | 复用学员权限、scope、启动／读取／释放入口 |

控制消息新增 `compose_prepare`、`compose_prepare_status`、`compose_start`、`compose_inspect`、`compose_stop`。响应严格绑定连接、Boot、消息 ID、实例和 Owner；控制端白名单投影响应，避免运行定义或变量因额外字段进入页面。

## 验证与后续

```bash
go test -race ./...
go vet ./...
npm --prefix web test
web/node_modules/.bin/tsc --noEmit -p web/tsconfig.json
npm --prefix web run build
```

集成测试入口 `npm --prefix web run test:compose-labs`：只允许 localhost 独立 `_test` PostgreSQL，需 CONTENT_TEST_DATABASE_URL、CONTENT_TEST_BASE_URL、NODE_TEST_CLUSTER_SECRET；服务器另设同一测试集群配置、LAB_DATA_DIR 和测试用 CONTENT_CREDENTIALS_KEY。`COMPOSE_BROWSER_TEST=1` 可启用 Chromium 流程。

已覆盖真实文件上传／预览、变量加密与不回显、已保存变量明确清除、Worker 下载认证、准备后启用、JSONB 字段顺序不影响修订、设置变更失效、所有权、不可变实例租期、旧部署释放、显式手动查询，以及浏览器上传／保存和先靶场后 CLI 的回连地址显示。旧单容器模板、学习关联和租期集成测试同样通过。

Go 测试覆盖包摘要、路径、重定向、TLS 限制、停止保留容量、启动失败后的取消、嵌套 Compose、宿主端口转换与原有生命周期。真实 Compose 夹具证明多用户数据独立、初始化依赖、宿主端口没有发布、重启与释放；这与模拟控制通道验收分别记录，不宣称已在真实 Btrfs＋veth 环境完成联调。

GitHub 目录来源与手动更新已在 [第四轮前半段](lab-git-import.md) 接入；下一轮做 Dockerfile 构建与更完整的配套目录支持。第五轮再安排旧单容器模板转换和旧网络／协议／表单退出，不把旧路径永久作为隐藏 fallback。
