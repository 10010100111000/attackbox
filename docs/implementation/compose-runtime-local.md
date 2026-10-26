# Compose 靶场运行器：第二轮代码与本地验证

状态：独立 `node/labcompose` 包与多服务夹具已实现。本机真实 Docker Compose 生命周期测试通过；现有 Worker 的控制协议、沙箱生命周期、后台与学习／挑战页面尚未切换。第一轮真实 veth／宿主机 INPUT 隔离验收仍待具备权限的环境执行。

本文保留第二轮的运行器边界；第三轮已补充可选 Worker／后台接入与端口转换，当前操作以 [平台接入说明](compose-platform-integration.md) 为准，新模式仍需显式启用。

## 本轮范围

管理员网页上传、Git 拉取、数据库接口、源码构建留在后续轮次。当前由可信 Go 调用方提供包目录、Compose 文件、显式变量和入口，先验证完整运行模型。浏览器不能提交这些本机路径或 Docker 参数。

新包与旧 `DockerTargetManager` 独立：

| 文件 | 职责 |
| --- | --- |
| `definition.go` | 官方 compose-go 解析、支持范围检查、入口、网络、挂载和整套资源预算 |
| `storage_linux.go` | 准备不可变产物、文件快照、摘要校验与实例配置生成 |
| `engine.go` | 固定版本 Docker Compose CLI、镜像准备、归属核验、状态和清理 |
| `manager_linux.go` | 实例生命周期、并发、停止墓碑、到期核对与沙箱接线接口 |
| `model.go` | 部署、实例、服务状态与执行／网络接口 |

没有增加数据库表、控制消息、定时器、前端查询或隐藏的旧执行器回退。

## 依赖与运行条件

- Linux，Go 1.23+。
- 官方 `compose-go/v2` 固定为 `v2.4.9`。
- Docker Engine 27.5+；CLI 检查服务端版本。
- Docker Compose 固定为 **2.40.3**，启动时校验，其他版本明确拒绝。后续变更需重新验证，不自动接受本机任意插件版本。
- Worker 使用本机 Unix Socket，Docker CLI 可执行文件及 Docker 凭据目录来自可信节点配置。
- 完整网络接入使用第一轮 `labnetwork.Manager`，要求 Worker 与 Docker 位于同一宿主机并具备相应网络权限。真实 Compose 测试使用仅供测试的 internal 网络适配器，不修改宿主机防火墙或伪造 veth 验收。

CLI 使用参数数组，不拼接 Shell；不继承应用密钥、`DOCKER_HOST`、`COMPOSE_FILE` 等环境。它显式指定本机 Socket、私有配置文件、项目目录及空 env-file，不读取部署目录的 `.env`。Compose 文件中的变量只来自 `Bundle.Variables`，初次解析后的字面 `$` 在 CLI 再解析时转义，避免第二次插值。

Registry 认证由显式 DockerConfig 提供，镜像下载使用 Docker daemon 的网络配置。执行错误不返回可能包含密码的原始 stderr；本轮尚未提供管理员日志查看界面。

## 支持的部署定义

支持常用 `image`、命令／入口、显式环境变量、依赖及健康检查、项目内 bridge 网络、命名卷、匿名卷及包内只读文件。初始化服务通过其他服务的 `service_completed_successfully` 依赖识别，成功退出不被误报为靶场失败。

处理原则：

- 上游固定 `container_name` 移除，容器名由独立 Compose project 产生。
- 上游网络和卷名称由平台替换；网络保持服务归属与别名，均使用 `internal` 网络，沙箱仅接入选定入口网络。
- 匿名挂载转换为项目内命名卷，避免把一次实例数据变成难以追踪的共享资源。
- 识别 Compose 端口声明并转换为私有容器端口，不发布到宿主机；入口使用容器 IP 和管理员确认的服务端口。端口不是访问白名单。
- 包内 bind 只接受已存在、无 symlink、范围内的只读普通文件。准备阶段复制成持久快照，不运行时引用作者工作目录。容器可读的文件权限与私有父目录配合，不要求容器以 root 读取配置。
- 文件每份最多 4 MiB，最多 128 份、合计 32 MiB；Compose 文件最多 1 MiB。
- 默认整套预算为 1 GiB、2 CPU、256 PIDs、最多 8 个服务，按服务分配，不给每个服务复制整套额度。调用方可设置节点预算；每服务日志最多 10 MiB × 2。
- 强制 `pull_policy: never`、`restart: no`、有限停止宽限，避免运行期间隐式拉取或无限重启。规范化变更通过 `Definition.Report` 返回。

首版明确拒绝：`build`、`include`、`extends`、profiles、YAML aliases、env_file、external 共享资源、host 网络、特权／设备／Socket、额外驱动与 IPAM、自定义静态地址、宿主目录和可写 bind。目录形式的包内挂载、secrets/configs 也暂不支持。未支持字段报错，不静默忽略或宣称兼容任意开源 Compose。第三轮补充端口声明转换，仍禁止实际宿主机端口发布。

## 准备产物

`Prepare(ctx, Bundle)` 在独立有界准备通道中执行：

1. 读取、校验、规范化包配置，确认缺失文件和变量。
2. 检查本机镜像；缺失时仅在这里拉取。相同引用在本次准备中只解析一次。
3. 固定实际本机镜像 ID，复制只读文件并记录内容摘要。
4. 写入不可变 `dep_<SHA256>` 目录；重复准备复用同一规范化产物。

学员启动只检查准备好的镜像 ID 是否存在，不下载、不构建、不追踪变化的 tag。镜像被外部删除时明确失败，需管理员重新准备。

部署产物保存规范化配置和只读文件，可能包含运行所需的环境变量，必须备份并保持私有。尚无公开下载接口或历史版本编辑器；运行实例引用的产物不会被自动删除。

## 实例生命周期

可信调用方创建 `Request{ID, OwnerID, DeploymentID, ExpiresAt}`。新租期必须在未来一小时内，已分配实例的 Owner、部署和到期时间不可修改。

```text
Prepare → Start
             ├── 预写实例归属、网络责任
             ├── 创建各实验网络，生成私有 Compose 配置
             ├── 预写 start_issued → compose up --no-build --pull never
             └── 返回服务实际状态及主要入口

Stop／到期 → 预写停止意图并取消同实例启动
             → 断开沙箱接线
             → 核对归属，compose down --volumes
             → 确认容器与项目卷消失
             → 释放实验网络 → 确认已释放
```

重复 Start 不重新执行 `up`，避免再次运行初始化服务；只返回状态并在需要时接线。Worker 在预写启动意图后崩溃、导致服务不完整时，不猜测是否可以安全重跑初始化；报告 `incomplete`，由明确释放后新建恢复。

状态与健康分开：无 healthcheck 的运行服务返回 `unverified`；健康检查不通过显示 degraded，初始化非零退出或服务退出显示 failed。缺服务不虚报 ready。停止或清理失败保留 `cleanup_pending`，不提前释放名额。

项目名由节点与实例完整身份派生。每个容器、项目卷带 Node／Owner／Instance／Deployment 标签。修改前检查实际项目资源，同时检查预期卷名是否已被无标签资源占用；不靠名称接管资源。禁止 prune；释放不删除镜像、部署包、攻击沙箱或其他用户实例。

## 沙箱与租期接入

当前提供给下一轮 Worker 集成的接口：

- `SandboxAvailable(ctx, capturedSandbox)`：记录当前沙箱并为已有实例接入入口网络；后启动的实例也会使用该沙箱，因此支持两种启动顺序。
- `SandboxGone(ctx, sandboxID)`：只断开接线，不清除靶场。重新创建沙箱时使用新 namespace 身份，先断开旧接线。
- `Reap(ctx)`：到期或上次清理未完成时调用同一 Stop 路径。它不创建定时器，应接入 Worker 现有清理循环。

这些接口已经在模拟生命周期中验证，但尚未挂到旧 RuntimeManager／控制通道。没有实际调用 Reap 的独立运行器，不会自行执行到期清理；本轮也没有承诺 Worker 生产环境已经改用它。

实例操作按实例串行，默认允许 4 个启动并发，清理使用独立有界通道。停止先持久化并取消该实例的长启动，不等待节点全局生命周期锁。启动最长 120 秒，清理每次 30 秒，镜像准备最长 30 分钟。关闭运行器取消启动／准备，保留已运行实例和持久责任；不因控制连接断开删除环境。

## 持久目录

```text
compose-runtime/                    # 0700，独立于 Btrfs 实例与 Nuxt .output
├── runtime.lock
├── deployments/dep_<digest>/
│   ├── deployment.json             # 0600
│   └── files/                      # 私有目录中的只读挂载快照
└── instances/<instance-id>/
    ├── record.json                 # 原子持久意图及资源归属
    └── compose.json                # 0600，仅由固定部署产物生成
```

原子写使用 fsync／rename／目录 fsync；持有进程文件锁。重开校验 Node、项目名、部署摘要、文件摘要及卷／网络归属。实例写盘失败后拒绝继续修改该实例，重新打开核对；未知结果不会被当成没有资源。

## 验证

```bash
go test -race ./...
go vet ./...
```

测试包括字段和文件边界、主机变量隔离、字面美元转义、整套预算、卷名碰撞、两种启动顺序、沙箱重建、重复启动、停止取消长启动、清理失败重开、到期、初始化与健康状态、快照篡改拒绝。

真实 Compose 夹具包含初始化服务与 Web、共享的项目卷、健康检查。先按 [网络夹具构建步骤](lab-network-local.md#真实通信夹具) 重新构建当前静态测试镜像，再执行：

```bash
ATTACKBOX_TEST_COMPOSE_ENGINE=1 \
ATTACKBOX_TEST_DOCKER_SOCKET=/var/run/docker.sock \
ATTACKBOX_TEST_LAB_IMAGE=attackbox-lab-network-fixture:local \
  go test ./node/labcompose -run '^TestComposeRealEngineLifecycle$' -v -count=1
```

测试使用显式本机 Socket，创建独立带测试标签的 Docker internal 网络，不做主机接线。验证两用户独立容器／数据、初始化依赖、健康检查、重复启动不重建、重开识别、自动到期调用路径、释放后镜像仍在，并清理测试资源。

本轮验证：Docker 28.4.0＋Compose 2.40.3 的真实生命周期测试通过；全仓库 Go race 与 vet 通过。此结论不包含第一轮真实 veth、宿主机防火墙、实际 Btrfs 沙箱公网访问回归或生产单机部署验收；没有连接 VPS。

下一轮按 [分步迁移计划](compose-target-migration-plan.md) 第三轮接后台导入、准备任务、协议及学习／挑战组件；生产切换仍以前述网络验收为前提。

版本检查已由后续 [Git 来源与兼容性迭代](lab-git-import.md#compose-版本兼容) 改为稳定版 >=2.40.3、<3.0.0；本文 2.40.3 指验收所用版本。
