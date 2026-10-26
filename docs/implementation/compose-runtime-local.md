# Compose Worker 运行器

`node/labcompose` 是唯一靶场执行器。`definition.go` 与官方 compose-go 校验来源及预算；`build.go` 在准备阶段构建本地镜像；`storage_linux.go` 保存带摘要的不可变产物；`instance_assets_linux.go` 管理用户可写副本；Manager 管理实例幂等、接线与停止责任。

运行配置只引用已准备镜像 SHA256，强制 no-build／pull never；准备失败不替换当前可用产物。每个实例有独立 Compose project、容器、网络和数据卷。资源操作核验 Node／Owner／Instance／Deployment 归属标签，不使用 prune。

实例使用固定 60 分钟租期。Worker 的一个清理循环负责到期回收和失败重试，控制端对账同步数据库；不依赖页面持续打开。停止取消该实例尚未完成的启动，确认容器／卷消失，再断开和释放网络、删除实例副本，失败保留 cleanup_pending。镜像、源包与其他实例保留。

Prepare 最多一个并行任务，总期限 30 分钟；启动最多 120 秒，清理每次 30 秒，实例操作各自串行。取消通过独立进程组终止 Docker CLI 插件，限制管道等待，不把秘密写进命令行或页面日志。

沙箱的 eth0 与默认路由不变；labnetwork 为同机实例增加实验网卡，固定 namespace Device/Inode 并通过沙箱生命周期锁接入。Docker internal 网络不等于宿主保护，INPUT 的回复方向放行和主动访问拒绝由 labnetwork 管理。

沙箱创建／销毁回调只更新身份并投递接线事件，不在沙箱生命周期锁下等待 Compose 操作。每个受影响实例使用一个可合并通知的接线任务：等待该实例的操作结束，再通过 `SandboxAccess` 在生命周期锁内读取当前沙箱身份并接线／断线；不会使用排队事件携带的旧 PID。启动收尾仍会主动接线。事件不依赖请求上下文继续存活，不增加轮询或定时器；任务处理完通知后退出，不为历史实例保留空闲 goroutine。Manager 关闭时取消并等待剩余任务退出。

锁顺序统一为实例操作锁 → 沙箱生命周期锁。Inspect 只读取服务和网络账本，不调用生命周期回调，也不承担网络修复责任。实例停止后，已排队的接线事件不能重新启用网络；接线失败写入 Worker 日志，不将查询当作隐式重试。

当前支持范围、部署及接口见 [平台说明](compose-platform-integration.md)，构建／目录细节见 [构建说明](compose-build-and-assets.md)，取消／重试见 [准备任务](compose-preparation-management.md)。不保留单容器执行器或第二套网络 fallback。
