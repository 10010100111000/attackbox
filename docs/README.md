# 项目文档入口

当前结构：cmd/node 是 Go Worker 入口，web 是 Nuxt 控制平台，PostgreSQL 保存平台数据。学习与挑战共用 Compose 靶场模型，Worker 负责 Docker、Btrfs 沙箱、网络和回收。

- [部署](implementation/deploy.md)
- [Compose 平台、全新数据库与接口](implementation/compose-platform-integration.md)
- [运行器与生命周期](implementation/compose-runtime-local.md)
- [本机网络与真实验收](implementation/lab-network-local.md)
- [GitHub 靶场来源与更新](implementation/lab-git-import.md)
- [Dockerfile 构建及目录副本](implementation/compose-build-and-assets.md)
- [准备任务取消、重试与强制重建](implementation/compose-preparation-management.md)
- [架构实施状态](implementation/compose-target-migration-plan.md)
- [CLI／GUI 工作台](implementation/environment-workbench.md)
- [Resend 账户邮件](implementation/resend-mail.md)
- [账户菜单与站内通知](implementation/account-notifications.md)
- [个人中心与学习记录](implementation/personal-center.md)
- [内容分类](implementation/content-categories.md)
- [单文件教程格式](content-template/README.md)
- [教程 Git 同步](implementation/git-content-sync-usage.md)

数据库以 server/db/schema 和唯一全量基线为准，不维护旧单容器或旧控制服务的部署路径。历史实现可通过 Git 历史查阅。
