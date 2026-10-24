# Worker Docker 单容器生命周期

状态：第二轮 Worker 代码已实现；本轮已继续接入 Nuxt 实例表、用户接口和前端按钮，详见 target-runtime-integration.md。本轮不迁移数据库，不连接 VPS。上一轮的双网桥方案见 [网络底座](worker-docker-network.md)。

## 运行边界

- Worker 使用本地 Docker Unix Socket，仅启动关联项目网络的单容器镜像。
- 每个实例使用不可变 `target_id`，命名为 `attackbox-<target_id>`。实例配置和用户归属不可在原 ID 上变更。
- 固定上限：16 个期望运行的实例，每个 512 MiB 内存／不额外使用 swap、1 CPU、256 PIDs，日志每份 10 MiB、最多两份。镜像拉取失败仍保留实例责任，可重试启动或明确停止释放名额。资源配置、镜像层及写入层磁盘配额后续另做。
- 不接受任意 Docker 参数、命令覆盖、环境变量、特权、宿主挂载、端口映射、host 网络或 Docker Socket 挂载。保留 Docker 默认能力集以兼容安全实验，不额外授予能力。
- 镜像必须明确 tag 或 SHA-256 digest。本轮支持本地缓存或公开仓库拉取；尚不接私有仓库凭据。
- 请求在独立的有界队列中串行执行，队列容量 16，单次操作最长 120 秒。长时间镜像拉取期间，后续启停请求需要等待；不是并发镜像拉取调度器。
- 不做共享、重置、TTL、空闲回收、Compose、镜像构建或个人空间。

## 持久化与恢复

归属、配置指纹及期望状态写入 Worker 本地 JSON 文件，采用临时文件、fsync、rename 和目录 fsync。默认目录位于 `runtime.instances_dir` 的同级 `docker-target-state`，默认部署为 `/var/lib/attackbox/docker-target-state`。可以通过 `network.docker_targets.state_dir` 指定；不得置于 Btrfs 实例树中。需要持久磁盘，不应挂在临时目录。

每个文件也绑定 Worker Node ID。损坏文件或 Node ID 不一致会使 Docker 管理功能不可用，不静默跳过。Worker 退出和控制连接断开均不删除运行中靶机，Docker 重启策略为 `no`；重启后可以通过启动请求恢复停止的存量容器，不会对意外退出的容器无限自动重启。

停止先落盘期望状态 `stopped`，再确认容器归属并强制删除，删除匿名卷，最后查询确认消失。失败返回 `cleanup_pending`，后续查询／心跳遇到该容器时继续处理。保留停止记录，迟到启动不能复活该 ID；再次启动业务环境时，Control 必须分配新的实例 ID。

Worker 必须核对容器上的项目实例、Owner、Node 和配置指纹标签，不操作同名但非本系统归属的容器。没有本地归属记录的项目标签容器会上报 `orphaned`，不会在重新连接时自动删除。管理员孤儿实例处理留到后续服务端接入；本轮不要手动删除状态文件。

## 控制协议

使用既有认证 WebSocket 通道。新增独立消息类型，不复用 Btrfs 的 `sandbox_id`、库存或“重启即清理”策略：

| 请求 | 响应 | 行为 |
| --- | --- | --- |
| `target_start` | `target_start_resp` | 记录归属、核验网络、准备镜像、创建或复用并启动 |
| `target_inspect` | `target_inspect_resp` | 检查归属、返回 Docker 状态与服务就绪信号 |
| `target_stop` | `target_stop_resp` | 持久停止意图、删除并确认 |

所有请求包含当前 `boot_id`、`target_id` 和 `owner_id`。启动另外包含 `image` 和 `ports: [{port, protocol}]`，最多 16 个唯一 TCP／UDP 端口。示例：

```json
{
  "id": "controller-operation-id",
  "type": "target_start",
  "payload": {
    "boot_id": "current-worker-boot-id",
    "target_id": "tgt_0123456789abcdef",
    "owner_id": "user-uuid",
    "image": "example/target:1.0",
    "ports": [{"port": 8080, "protocol": "tcp"}]
  }
}
```

停止／查询只需保留前三个身份字段。响应保留原消息 ID，返回 Worker 代次和实例 ID、`success`、脱敏错误码、可选 `instance`。旧代次请求不会执行。下一轮 Control RPC 的等待时间需要覆盖镜像准备，超时后应查询／重试同 ID，不能直接认为容器未创建。

`instance.state` 是 Docker 实际状态，或 `missing`、`stopped`、`cleanup_pending`、`orphaned`。`ready` 与运行状态分开：优先使用镜像 HEALTHCHECK；没有 HEALTHCHECK 时验证所有配置 TCP 端口。仅 UDP 或无健康信号的镜像返回 `unverified`，不会凭 Docker running 宣称服务可用。端口不是访问白名单。

心跳增加可选 `docker_targets` 清单。`available=false` 表示未知或操作忙，不表示清单为空，更不能据此清除服务端归属。清单来源于 Docker 标签，已记录但被外部删除的运行实例上报 `missing`。当前 Nuxt Hub 尚未消费该扩展，下一轮才接持久业务记录和对账。

## 本地测试

常规检查：

```bash
go test ./...
go test -race ./node ./protocol ./cmd/node
go vet ./node ./protocol ./cmd/node
```

真实 Docker 测试使用不访问外网的静态 HTTP 镜像，带内部 HEALTHCHECK：

```bash
mkdir -p /tmp/attackbox-target-smoke
CGO_ENABLED=0 go build -o /tmp/attackbox-target-smoke/server ./node/testdata/target-http
docker build -f node/testdata/target-http/Dockerfile -t attackbox-target-smoke:local /tmp/attackbox-target-smoke
ATTACKBOX_TEST_DOCKER_SOCKET=/var/run/docker.sock \
ATTACKBOX_TEST_TARGET_IMAGE=attackbox-target-smoke:local \
go test ./node -run TestDockerTargetLifecycleRealEngine -v -count=1
```

测试自动建立唯一名称的临时网络及容器，验证创建、运行、健康、重复启动、状态重新加载接管、停止和迟到请求阻断，再清理测试容器与网络。使用 `172.30.232.0/24` 测试网段，运行前确认无冲突；不改生产转发规则，不连接 Worker 生产控制通道。测试结束可以自行删除 `attackbox-target-smoke:local` 镜像。

本地 Docker 28.4.0 的真实容器生命周期测试已通过。镜像拉取失败、归属冲突、清理失败、状态损坏和代次拒绝使用模拟 Engine 单元测试覆盖；不把这些模拟断言描述成生产验收。跨桥网络互访与反连仍由上一轮网络验收覆盖，本次容器健康检查只证明容器内服务工作。

## 下一轮

Nuxt 保存业务实例归属与操作状态，解析已发布内容和关联模板，加入用户／管理员接口，扩展 Hub RPC 与 Docker 清单对账，再让学习页和挑战页共用状态卡片。浏览器不得直接提供镜像、Owner、Node、Docker 参数或访问 Worker Socket。
