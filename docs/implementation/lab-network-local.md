# 本机实验网络接入：第一轮代码与验证

状态：独立网络模块与测试夹具已实现，尚未接入现有 Worker 生产启动流程。现有单容器靶机、沙箱主网络、页面与 API 不改变。本轮不实现 Compose 执行器或源码构建。

## 代码边界

新代码位于 `node/labnetwork`，不依赖旧 `DockerTargetManager` 或旧的共享靶机网桥：

| 文件 | 职责 |
| --- | --- |
| `model.go`、`plan.go` | 网络／接线模型、命名、地址规划 |
| `namespace_linux.go` | 创建时捕获身份，接入时固定 namespace FD |
| `docker.go` | 创建／核对内部网络及 Docker IPAM 预留 |
| `links_linux.go` | 通过 netlink 将 veth 直接接入固定 namespace |
| `firewall_linux.go` | 项目网络访问宿主机的独立 INPUT 保护 |
| `store_linux.go`、`manager_linux.go` | 单写者账本、接入、断开、显式核对与清理重试 |

新增固定版本的 `vishvananda/netlink`、`vishvananda/netns` 和 `x/sys`，不自行编写底层 netlink 协议，也不使用拼接 Shell 或反复按 PID 执行 `nsenter`。

入口：

- `New(Config)`：打开本地 Docker 客户端、netlink 句柄和持久账本，不改变网络，不启动定时器。
- `CreateNetwork(ctx, NetworkSpec)`：先安装宿主机保护，分配并持久化子网，再创建或核对 Docker 网络。未确认创建的地址仍保留，避免重用。
- `Attach(ctx, networkID, Sandbox)`：核对 Owner、PID 对应的 namespace 身份，固定 FD，预先记录接线意图，再创建／核对接口。
- `Detach(ctx, attachmentID)`：先保存断开意图，只删除归属匹配的宿主 veth，内核连带删除沙箱端。
- `ReleaseNetwork(ctx, networkID)`：先断开接线，确认桥上无端口，再删除归属匹配的 Docker 网络。服务容器应由下一轮运行器先清理；本模块不停止容器或删除镜像。
- `Reconcile(ctx)`：显式核对旧账本，重试停止责任，断开 namespace 已消失／变化的接线，报告异常与孤儿，不按新 PID 重新接线。
- `Close()`：只关闭句柄和文件锁，不删除运行资源或保护规则。

所有调用都在可信 Worker 内部执行，不是允许浏览器提交 PID、namespace 或 Docker 配置的 API。

## 命名与地址

- Docker bridge：`bt_<8位摘要>`，11 字节。
- 宿主 veth：`vlh_<10位摘要>`，14 字节。
- 临时 peer：`vlg_<10位摘要>`，14 字节；直接创建在固定的沙箱 namespace 中，随后重命名。
- 沙箱网卡：`lab0` 至 `lab255`，按同一沙箱已有接线分配。关系保存在账本，前端后续可展示靶场与实验地址。
- 摘要包含完整实例／网络身份；接线还包含节点、沙箱 ID 和 namespace Device＋Inode。短名冲突时拒绝操作，不接管或删除同名接口。
- veth 使用完整归属 alias；Docker 网络使用 Node／Owner／逻辑网络身份标签。名称或前缀本身不是删除授权。

默认地址池 `10.201.0.0/16`，默认每网络 `/28`，创建时允许 `/27` 到 `/24`。以第一个 `/28` 为例：

```text
10.201.0.0       网络地址
10.201.0.1       Docker bridge 网关
10.201.0.2–13    Docker 可分配地址
10.201.0.14      沙箱接线地址，通过 AuxAddress 排除动态分配
10.201.0.15      广播地址
```

分配同时检查所有 Docker 网络、宿主机各 IPv4 路由表的非默认路由、显式排除范围和未完成清理的账本预留。池耗尽或冲突时明确失败，不自动更换到未知地址池。

现有共享靶机桥若占用整个 `10.201.0.0/16`，新模块不能同时从里面划分网络。开发夹具使用另一段空闲地址池；最终切换时先释放旧实例与旧网络，或明确配置不重叠的新池。

## 身份与隔离

沙箱创建者应在生命周期锁内调用 `CaptureNamespace(pid)`，把 Device＋Inode 与沙箱不可变 ID 一起保存。接线与沙箱销毁也要由调用方协调，下一轮再连接实际 RuntimeManager 生命周期。

`Attach` 打开 `/proc/<pid>/ns/net`，通过 `fstat` 核对 Device＋Inode，拒绝 Worker／宿主 namespace。后续 peer 迁移使用 FD，配置使用该 FD 创建的 namespace-bound netlink socket。检查后不再按 PID 查找，避免 PID 复用窗口。

主 `eth0`、默认路由和原有出口配置不变；实验端口显式关闭 bridge isolated，不复制主沙箱网桥的端口隔离设置。接口别名、peer ifindex、接入桥和实验地址可核对，错误不会伪装为接入成功。

Docker 网络固定为 IPv4、`internal=true`、关闭 masquerade，保留容器互通和 Docker DNS。自定义 nftables 仅安装独立、带 Node 所有权 comment 的 INPUT 表，在提前阻断 `iifname "bt_*"` 的宿主访问前，只放行 `ct direction reply` 且状态为 `established, related` 的回包，使宿主主动发起的 TCP／UDP 探针能收到响应；靶场主动访问宿主（包括原方向已建立流量）仍丢弃。后续宿主机防火墙链仍生效，不改旧 Worker 表、不清空 ruleset、不增加 FORWARD／DOCKER-USER 例外。

跨网络和直接公网出口由 Docker internal 网络负责；必须通过真实网络测试确认所用 Docker／内核配置的行为。沙箱可主动代理流量不属于本轮阻止目标。手动接入的沙箱不自动获得 Docker 内置 DNS，第一版使用实验 IP。

`bt_` 是本模块保留的桥名前缀，宿主机不要将无关网络命名为该前缀。保护表保留到显式维护，不在退出时删除，避免留下未保护的存量资源。

新建 veth 不仅依赖创建请求中的 Alias：独占创建成功后，先核对固定 namespace 中的双向 peer ifindex，再显式 SETLINK 写入宿主别名并重读验证。失败时仅回滚已确认的新建链路；已有接口仍须原有归属标记匹配，不接管空别名接口。

## 持久化与恢复

`StateDir` 必须是专用绝对路径、权限 0700，不放进 Btrfs 沙箱实例目录。状态文件 0600，写入经过临时文件、fsync、rename 和目录 fsync；文件锁阻止两个进程同时分配地址。

测试使用 `filepath.Join(t.TempDir(), "state")` 作为状态目录，由模块按 0700 创建。`t.TempDir()` 返回的子目录权限受 umask 影响，不直接作为私有账本目录；不放宽已有目录的生产权限检查。

账本绑定节点 ID，启动验证派生名称、地址和互斥预留；损坏或冲突不静默跳过。写盘失败后停止接受新的修改，需重新打开模块恢复，以免在落盘状态未知时继续创建或删除。

失败回滚也先保存断开意图，清理失败继续占用地址和接口槽位。`Reconcile` 可由下一轮运行器的启动核对及现有清理循环调用；本模块不另建定时器。未知归属的网络、网卡只报告，不自动删除。

## 常规检查

```bash
go test ./...
go test -race ./node/labnetwork ./node ./node/runtime ./protocol
go vet ./...
```

模拟测试覆盖：分配与冲突、命名长度、多实验网卡、重复接入、Owner／namespace 拒绝、保护规则失败、回滚失败持久化、重启重试、停止墓碑、未知桥端口、孤儿报告、文件锁和并发接入。

Docker 配置检查只需要显式选择本机 Socket，不修改 Worker 网络规则，不运行靶机：

```bash
ATTACKBOX_TEST_DOCKER_SOCKET=/var/run/docker.sock \
  go test ./node/labnetwork -run '^TestDockerNetworkRealEngine$' -v -count=1
```

该测试使用 `172.29.240.0/24` 内无 Docker 冲突的 `/28`，仅创建／核对／删除自己的带标签网络，证明真实 internal、IPAM AuxAddress 和重复创建语义；不是访问隔离验收。

## 真实通信夹具

需要一次性本地 Linux 验证主机，Worker 与 Docker daemon 在同一宿主网络 namespace，具备 NET_ADMIN、SYS_ADMIN、nft、Docker Engine 27.5+，以及符合现有支持范围的 Docker iptables 后端。开启测试后缺少条件应失败，不静默跳过。

构建镜像无需下载基础镜像或依赖，使用仓库中的静态 Go 服务：

```bash
mkdir -p /tmp/attackbox-lab-network-fixture
CGO_ENABLED=0 go build -trimpath \
  -o /tmp/attackbox-lab-network-fixture/fixture ./node/testdata/lab-network
docker --host=unix:///var/run/docker.sock build \
  -f node/testdata/lab-network/Dockerfile \
  -t attackbox-lab-network-fixture:local /tmp/attackbox-lab-network-fixture

# 必须选当前主机空闲、且不与 VPN／局域网冲突的测试池。
ATTACKBOX_TEST_LAB_NETWORK=1 \
ATTACKBOX_TEST_DOCKER_SOCKET=/var/run/docker.sock \
ATTACKBOX_TEST_LAB_IMAGE=attackbox-lab-network-fixture:local \
ATTACKBOX_TEST_LAB_POOL=172.29.240.0/24 \
  go test ./node/labnetwork -run '^TestLocalAttachmentAcceptance$' -v -count=1
```

夹具不使用真实学员沙箱：创建一个独立 namespace 及带默认路由的测试 `eth0`，启动 TCP／UDP 接收端；在专属网络中启动两个服务容器。控制使用管道，不依赖实验网络。

检查沙箱至靶机 TCP／UDP、靶机反连、Docker 服务名解析、实验地址预留、主网卡和默认路由不变、宿主机监听端口阻断、跨用户项目阻断、namespace 身份错误拒绝、同名接口不误删、断开、重开账本及沙箱退出清理。另做 TEST-NET 地址的出口失败探测；该探测不替代生产环境公网策略验收。

夹具仅清理自身创建的容器、接线、网络与带匹配 comment 的保护表；镜像保留。失败时输出待清理的资源名。实际 Btrfs 沙箱的原有公网访问回归、更多 Compose 拓扑及资源上限由后续联调验证，不能据此宣称已完成。

## 本轮验证记录

- 无特权本地 Go 单元／模拟测试与 race 检查已通过，真实 Docker internal／AuxAddress 配置检查已通过。
- 当前执行进程没有 NET_ADMIN／SYS_ADMIN，且缺少 nft，未执行真实 namespace 接线与通信测试；不连接 VPS 绕过此限制。
- 第一轮代码交付完成，真实通信验收待上述具备权限的本地环境执行；尚未启用生产切换。

第二轮 [Compose 运行器](compose-runtime-local.md) 已独立实现并完成本机 Compose 生命周期验证；后续业务接入以 [分步重构计划](compose-target-migration-plan.md) 第三轮为边界。完整网络验收仍是生产切换门槛。
