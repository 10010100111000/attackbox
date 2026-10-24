# Worker Docker 靶机网络

## 拓扑

```
沙箱 10.200.0.0/16 ─ br-attackbox ─ 宿主机路由 ─ br-ab-targets ─ 靶机 10.201.0.0/16
```

Worker 保留原有沙箱网络，创建并管理项目专用 Docker 网络；无需管理员逐个容器接线。这里只完成网络底座，还没有学习／挑战启动接口。

## 配置

`configs/node.yaml` 的 `network.docker_targets` 默认关闭。安装 Docker Engine 27.5+、iproute2、iptables、nftables 后，在 Worker 的实际配置文件中启用 `enabled: true`。示例中的两个网段必须避开部署主机的其他容器、局域网及 VPN 网段。运行 Worker 需要宿主网络管理权限和 Docker Socket 访问权。

- `socket_path`：本机 Docker Unix Socket，默认 `/var/run/docker.sock`，不接受远程 Docker 端点。
- `name`：项目 Docker 网络名，默认 `attackbox-targets`。
- `bridge_name`：独立 Linux bridge，默认 `br-ab-targets`。
- `subnet_cidr`、`gateway_ip`：独立 IPv4 网段和网关。

Worker 启动时创建网络并打归属标签，后续启动只核验并复用。遇到重名未归属网络、地址重叠、网关或选项变化时拒绝接管，不删除已有网络。配置在运行中保持不变；变更网段前先停止并清理该项目的靶机，不能直接修改配置期待自动迁移容器。

## 转发策略

- 沙箱到靶机、靶机到沙箱：不限协议或端口，保留源 IP。
- 沙箱之间：保留已有 bridge isolation 和同桥转发限制。
- 两侧环境到宿主机：保留管理面保护；沙箱已有 DNS／ping 例外不变。
- 靶机公网出口：本轮不开放。靶机模板端口不是防火墙白名单，也不是公网映射。
- 沙箱公网出口：保留原有 TCP 53/80/443、UDP 53。显式兼容 Docker 把 FORWARD 默认策略改成 DROP 的情况，不修改主机全局策略。

Docker 网络使用 `enable_ip_masquerade=false`，避免靶机回连时地址被改写；`gateway_mode_ipv4=nat-unprotected` 允许未发布端口的路由访问，实际转发范围由 Worker 的 nftables 规则限制。Docker 的 DOCKER-USER 链增加带项目注释的精确双向放行规则，重复初始化不重复插入；Worker 专属 nftables 表在事务中替换，不清空其他表。

当前适配 Docker **iptables 后端**，包括 iptables-nft。Docker **原生 nftables 后端**暂未适配，缺少 DOCKER-USER 转发入口时明确失败，不关闭 Docker 防火墙。Docker 重启或外部防火墙工具重载规则后，应重新启动 Worker 完成网络核验；后续生命周期轮次再接持续健康检查。

Worker 退出不删除共用网络，避免破坏仍在运行的靶机。现有 Worker 启动流程会清理沙箱残留接口，因此更新与重启应安排在无运行中实验时。

## 本地验证

```bash
go test ./node ./cmd/node
go vet ./node ./cmd/node
ATTACKBOX_TEST_DOCKER_SOCKET=/var/run/docker.sock go test ./node -run TestDockerTargetNetworkRealEngine -v -count=1
```

已验证配置冲突、归属检查、网络复用、防火墙范围、重复请求、错误反馈，以及真实 Docker 28.4.0 网络创建／检查／删除。真实测试使用唯一名称的临时网络，结束时仅删除该测试标签归属的网络，不改主机转发规则。

当前开发环境没有 NET_ADMIN 和 nft 命令，尚未实际加载规则或完成跨桥通信测试。后续具备权限的本地测试环境必须确认：沙箱可访问靶机任意测试端口、靶机可主动反连、对端观察到真实源 IP、宿主管理端口不可达、沙箱原有公网访问不回归。此状态不能作为生产网络连通性已验收的依据。

## 实现收敛

保留两张网桥和宿主机自动生成的直连路由。代码不新增静态路由，不增加代理、网络命名空间或网络管理服务。Worker 防火墙由单一模板生成，不再通过匹配注释文字二次插入规则；Docker 转发入口只检查一次。Docker 网络归属及配置冲突检查保留，避免接管不属于项目的网络。

两处转发规则仍有必要：Worker 自己的阻断规则需要放行实验流量，Docker 的 FORWARD 默认策略也可能阻断该流量。因此保留精确的 DOCKER-USER 例外，而非关闭 Docker 防火墙或改变全局默认策略。靶机流量先经过明确的实验访问许可及出口阻断，再处理其他 established 连接，避免已有公网连接绕过靶机出口限制。

这次是代码收敛及规则顺序修正，不是新的网络架构；尚未在实际主机加载规则，真实跨桥通信测试仍待部署反馈。
