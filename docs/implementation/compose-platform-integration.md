# Compose 靶场：当前平台与全新部署

平台只有一种靶场模型：一份私有部署包 → 按节点准备的不可变部署 → 用户独享的 Compose 项目。简单单镜像环境也写成单服务 Compose。Nuxt 管理鉴权、内容关联、节点选择和数据库，Worker 负责 Docker、网络、接线和回收。

## 全新部署

数据库使用唯一基线 `web/server/db/migrations/0000_compose_platform.sql`，包括当前全部平台表和 6 个初始内容分类。不提供旧数据库、旧单容器模板或旧 Worker 状态转换。

1. 准备空 PostgreSQL 数据库、Go、Node、Docker Engine 27.5+、稳定 Compose >=2.40.3 且 <3.0.0；本地构建另需 Docker Buildx 插件。
2. 按 [部署文档](deploy.md) 配置 Web 和 Worker。Web 设置 DATABASE_URL、CONTENT_CREDENTIALS_KEY（32 字节十六进制密钥）和 LAB_DATA_DIR。
3. LAB_DATA_DIR 使用 Web 服务用户拥有的专用持久目录，权限 0700，放在发布产物之外。密钥与数据库、部署包目录一起备份。
4. 执行 `sudo scripts/manage.py migrate` 或 `sudo scripts/migrate.sh`。脚本验证迁移历史身份；重复初始化当前基线安全，遇到不匹配的库拒绝执行，不修改数据。当前工作没有清空任何远程数据库。
5. 完成管理员初始化、前后端构建和服务启动。控制端与 Worker 使用 Compose 协议 2。

Worker 的 compose_labs 默认关闭，network_verified 也默认关闭。先按 [真实网络夹具](lab-network-local.md) 验证主机环境，再显式启用。网络池默认 10.202.0.0/16，按实际路由选择空闲网段；不创建共享靶机桥，不添加跨桥例外。

## 管理员流程

`/admin/target-templates` 是列表，`/admin/target-templates/new` 创建，`/admin/target-templates/:id` 编辑。不存在额外 compose 子路由或旧单镜像编辑器。

上传 YAML／ZIP 或从 GitHub 导入，预览 Compose 和附件，确认入口、变量和网络，保存未启用模板；按节点准备成功后启用。打开新建页或仅上传包不会产生空模板。

- [GitHub 来源与手动更新](lab-git-import.md)：公开／私库、固定提交、明确确认更新。
- [Dockerfile 构建与配套目录](compose-build-and-assets.md)：准备时构建、实例可写副本、只读消费者共享初始化结果。
- [准备任务管理](compose-preparation-management.md)：取消、失败重试、强制重建与保留现有可用部署。

模板保存 bundleId、settings、revision 及说明；配置与文件内容变化才增加 revision。模板没有 runtime、image、ports、sourceUrl 等冗余字段；实例没有 runtime、image、ports，必须固定 deploymentId。正文和挑战展示的 entryPort 从配置派生，实际服务地址来自 Worker，不从模板字段猜测。

源包不可变、私有存储、带内容摘要。Git Token 与部署变量加密保存不回显，Worker 下载接口不返回 Git Token。关联变更不会替换已运行实例的部署 ID 和到期时间。

## 接口

| 接口 | 用途 |
| --- | --- |
| GET/POST `/api/admin/target-templates` | 列表／首次保存 |
| GET/PUT `/api/admin/target-templates/:id` | 读取／乐观锁保存 |
| GET `/api/admin/target-templates/options` | 内容关联选择 |
| GET/POST `/api/admin/target-templates/:id/preparations` | 数据库状态／明确 prepare、status、cancel、rebuild |
| POST `/api/admin/lab-bundles` | YAML／ZIP 上传 |
| POST `/api/admin/lab-bundles/git` | GitHub 导入 |
| GET `/api/admin/lab-bundles/:id` | 安全预览 |
| POST `/api/admin/lab-bundles/:id/git` | 检查／确认更新 |
| GET `/api/worker/lab-bundles/:id` | Worker 集群认证下载 |
| GET/POST `/api/targets` | 学习／挑战归属下读取、启动、停止 |

`/api/admin/compose-templates` 和 `/:id/image` 已删除，不提供别名或兼容响应。

## 验证与后续

本地 PostgreSQL、模拟 Worker、Chromium 与真实 Docker 夹具分别验证，不把模拟接线视为真实网络通过。学习／挑战共用 TargetEnvironment.vue；没有靶场周期查询，页面进入、操作与沙箱身份变化时读取，倒计时本地计算。管理员准备状态由点击查询更新。

```bash
go test -race ./...
go vet ./...
npm --prefix web test
web/node_modules/.bin/tsc --noEmit -p web/tsconfig.json
npm --prefix web run build
```

集成测试需隔离 localhost `_test` PostgreSQL 和对应服务：CONTENT_TEST_DATABASE_URL、CONTENT_TEST_BASE_URL、NODE_TEST_CLUSTER_SECRET、LAB_DATA_DIR；Web 设置同一测试集群配置和测试密钥。`COMPOSE_BROWSER_TEST=1` 开启 Chromium。`test:compose-labs` 串行验证包、构建和准备任务；`test:baseline` 验证全新建库、重复执行和不匹配基线拒绝。

第五轮代码收敛已完成；真实 Btrfs／veth 和主机防火墙验收仍按网络夹具单独执行，不在本轮自动连接 VPS 或操作主机资源。
