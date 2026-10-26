# 第五轮第一步：退出旧单容器执行路径

状态：代码完成运行路径收敛；这是第五轮的第一步，不宣称数据库结构和旧主机资源已经全部清理。没有连接 VPS，没有删除任何服务器容器、镜像、网桥、状态文件或数据库记录。真实网络验收仍是部署新路径的门槛。

## 已收敛的路径

- 删除旧 `DockerTargetManager`、镜像准备、租期扫描、共享靶机网络及其测试，删除 `handle_target_commands.go` 和 `protocol/docker_targets.go`。
- Worker 只处理 Compose 靶场协议；心跳不再发布 docker_targets，控制端移除旧 target RPC 和能力集合，不保留自动 fallback。
- 移除 `network.docker_targets` 配置及默认值，不再创建 `attackbox-targets`／`br-ab-targets`，不再添加旧跨桥转发例外。沙箱 br-attackbox、eth0、隔离、受控公网出口保持原职责；Compose bt_* 网络与额外 lab 网卡独立管理。
- 后台旧单镜像编辑器改为跳转 Compose 编辑入口，旧 POST／PUT／image API 返回 410，并保留管理员认证与 CSRF 校验；没有把旧字段暗中转换成可运行模板。
- 列表及关联选择只显示 Compose 模板。旧关联在学习／挑战中显示不可用，不能新建旧模板关联；遗留实例停止请求报 410，不会把未知清理责任标记为 stopped。
- 学员启动、查看和停止只走 `composeTargetOperation`。发布产物、手动准备、取消／重试／强制重建与固定租期逻辑保留。

## 升级前必须完成的实际清理

已经运行旧模式的主机，先保持升级前 Worker 运行，暂停旧实例新建，释放旧实例并确认容器与卷已清理。不要先删除旧程序或改写 cleaned 字段来绕过保护。

新 Worker 会只读检查 runtime.instances_dir 同级的默认 `docker-target-state`：有未确认 cleaned 的 JSON、无效记录或读取失败时拒绝启动。自定义旧 state_dir 需要由部署者另行确认；此检查不是对 Docker 实际库存的完整验收，不能代替容器／卷／网络归属核查。

旧 YAML 的 network.docker_targets 会因严格配置解析而报错；清理完成后使用仓库新配置。代码不自动删除旧 Docker 网桥、Docker 过滤规则或状态目录，不执行 prune。旧实例与旧网桥无引用后，主机清理由部署者在维护窗口核查归属再做。

Compose 开关和 network_verified 默认仍关闭，不因删除旧代码而自动开启。准备产物可重新导入／准备，不能在网络未验收时宣称生产切换已完成。

## 暂时保留的结构

数据库 runtime、image、ports、source_url 等旧字段暂时保留，供识别与显式拒绝遗留记录；运行阶段不再从它们构造单容器命令。新模板仍由 Compose 服务端接口写入 runtime=compose。没有新增数据库迁移，也没有静默删除旧记录。

下一步按已约定的全新部署方式收敛数据库与共享类型：去掉这些旧字段、清理历史迁移／规范和过渡路由，并同步更新学习／挑战视图与夹具。无需承诺在线迁移旧运行实例。完成这一步后再称第五轮完整收尾。

## 验证

- Go race／vet，含真实本机 Docker Compose 生命周期；增加沙箱防火墙保留、旧 YAML 拒绝及旧状态清理保护测试。
- 前端默认测试删除已经退出的单镜像验证契约，保留 Compose 包解析、权限、绑定、学习与界面测试。
- 将学习、挑战、内容同步与租期测试夹具切换为 Compose，不再通过旧创建接口或模拟旧 target RPC。
- 新退役回归验证旧 API 返回 410、列表隐藏旧模板、旧实例不能被误记为已释放；原 Compose 上传／构建／准备任务管理浏览器链路继续回归。

当前执行环境缺少 NET_ADMIN，未声称完成真实 veth／宿主机 INPUT 验收；该项仍按 [本机网络夹具](lab-network-local.md) 在具备权限的主机执行。
