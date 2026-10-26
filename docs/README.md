# 项目文档入口

当前运行结构：`cmd/node` 是唯一 Go 服务入口；`node` 管理沙箱和 Docker 靶机；`protocol` 定义控制通道；`web` 是 Nuxt 平台；`deploy` 与 `scripts/deploy.sh` 提供部署路径。旧前端与 Go Control 不再参与构建，历史源代码可从 Git 历史查阅。

当前使用以下文档：

- [部署](implementation/deploy.md)：安装、配置、构建和服务启动。
- [Docker 网络](implementation/worker-docker-network.md)：双网桥互通及访问边界。
- [Compose 靶场分步重构计划](implementation/compose-target-migration-plan.md)：独立实验网络、沙箱第二网卡接入、多服务部署及旧单容器路径退出的分轮安排。
- [本机实验网络第一轮](implementation/lab-network-local.md)：独立 Go 模块、namespace 身份固定、地址预留、接线恢复和本地验证；尚未切换生产路径。
- [Compose 运行器第二轮](implementation/compose-runtime-local.md)：独立多服务生命周期、固定镜像、实例数据、到期清理和真实本机 Compose 验证。
- [Compose 平台接入第三轮](implementation/compose-platform-integration.md)：上传、预览、节点准备、Worker 协议、学习／挑战联调及显式启用步骤。
- [Worker 生命周期](implementation/worker-docker-target-runtime.md)：协议、持久归属与容器管理。
- [CLI／GUI 工作台](implementation/environment-workbench.md)：独立页面、窗口操作及环境控制。
- [内容分类](implementation/content-categories.md)：分类管理、删除语义与抽屉交互。
- [前后端运行接入](implementation/target-runtime-integration.md)：使用与测试重点。
- [单文件 Markdown 与 Git 内容同步](implementation/git-content-sync-plan.md)：教程格式、后台保留功能、身份映射与分轮实施计划。

其他阶段计划、概念原型和历史验收记录用于说明背景，不是现行开发限制。数据库当前结构以 `web/server/db/schema` 和迁移为准。后续实现变化应直接更新对应操作文档，避免继续叠加相互冲突的“禁止事项”。

独立的 `attackbox-enter` 已合入 Worker 自身入口；进入 namespace、能力降权和 cgroup 归属是实际运行机制，仍需保留，不能作为旧规范删除。

- [单文件教程作者模板](content-template/README.md)：Markdown 头、章节与练习约定，以及本地校验命令。

- [手动 Git 教程同步使用说明](implementation/git-content-sync-usage.md)：仓库凭据、部署变量、导入流程与后台维护边界。

- [靶场 GitHub 导入、手动更新与版本兼容](implementation/lab-git-import.md)
