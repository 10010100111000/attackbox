# 靶场 Git 来源与手动更新

状态：第四轮拆分为两个迭代。本轮完成 GitHub 来源、手动更新和 Compose 版本兼容；本地 Dockerfile 构建与目录挂载已在 [后半轮](compose-build-and-assets.md) 接入。继续使用现有部署包、节点准备、学习／挑战入口，不新增运行器或自动同步任务。

## 操作流程

1. 环境管理 → 靶场模板 → 新建靶场 → 从 GitHub 导入。
2. 填写 `owner/repository` 或标准 HTTPS GitHub 仓库链接、分支／标签、仓库内目录（留空表示根目录）。分支默认 `main`，其他默认分支请明确填写。
3. 公开仓库 Token 留空；私有仓库提供该仓库只读 Token（fine-grained PAT 需要 Contents: read）。导入后 Token 加密保存、不回显，不借用教程仓库凭据。更换仓库或 Token 时重新导入来源。
4. 服务端将分支／标签解析成 commit，再读取该 commit 的目录树与文件，生成与 ZIP 相同的私有部署包。选择 Compose、入口和变量，保存模板，然后按节点准备并启用。
5. 更新时点击“检查更新”，显示当前与新 commit；点击“确认导入更新”才下载新文件到编辑区，最后保存模板。检查、导入都由管理员明确触发，无轮询、Webhook 或自动拉取。

检查后仓库再次变化，导入返回冲突，要求重新检查，不能悄悄导入另一个提交。文件未变时可保存新的来源 commit，不改变部署修订、不清除准备记录；文件／入口／变量变更时才需要重新准备。已运行实例仍使用原产物，不被模板更新替换。

来源中的本地 `build` 与包内目录挂载现已支持，具体范围见 [构建与目录](compose-build-and-assets.md)；构建只在管理员准备阶段发生。

## 凭据与存储

`lab_bundles` 保存来源元数据与加密 Git Token，包含在全新数据库基线中。不增加模板历史编辑器。来源属于不可变包，模板仍只引用当前包。

沿用 `CONTENT_CREDENTIALS_KEY`，凭据 AAD 使用独立的 `git:<bundle UUID>` 域，与教程及部署变量分离。带 Token 的导入请求必须使用 HTTPS（localhost 联调除外）；Token 不写入 URL、下载文件、Worker 下载响应或日志。Worker 仅获取包文件，不获取 Git Token。

GitHub 首版只访问固定 `https://api.github.com`，不接受自托管 Git、SSH 克隆、任意下载地址或重定向。不执行 hooks、仓库脚本、Git LFS 或子模块。需要其他 Git 平台时另加 provider，不放宽成任意网络抓取接口。

每个导入最多 120 秒、4 路文件下载；单进程最多两个请求执行，同一管理员最多一个。文件最多 512 个、单文件 4 MiB、总量 32 MiB；拒绝截断目录树、符号链接、子模块、路径越界和非法文件大小。文件路径与 ZIP 使用相同的 ASCII 约定。仍采用现有私有持久目录和摘要校验，不对外提供原文件静态链接。

使用 GitHub REST API，需要节点能访问该域名，可能受 GitHub 限流影响；大仓库应选择具体靶场目录。Node 24 的代理环境可用 `NODE_USE_ENV_PROXY=1` 使原生 fetch 使用既有 HTTP(S)_PROXY，保留 CA 验证；普通 VPS 可正常直连，不需要代理。不要在项目代码中写入云环境代理地址。

## Compose 版本兼容

`2.40.3` 作为已验证基线和保守的最低支持版本；接受稳定的 `>=2.40.3, <3.0.0`，允许 `v` 前缀与构建标记，拒绝旧版、预发行版、未知格式和未验证的新主版本。Worker 初始化日志记录实际 CLI 版本。后续降低最低版或支持 v3 必须补充相应兼容验证，不从一次测试推断全部版本兼容。

这取代“必须完全等于 2.40.3”的检查，不改 Docker Engine 27.5+ 要求，也不改变独立的 compose-go 解析依赖。

## 验证

- 默认 `npm --prefix web test` 包含 Git 目录解析、固定 commit、路径／大小／链接限制、固定 HTTPS API、Token 与错误脱敏测试。
- `go test -race ./...` 与 `go vet ./...`，包含 Compose 版本边界。
- 原有 `test:compose-labs` 继续验证上传、准备、学员启停和权限。
- `node --test web/tests/lab-git-integration.test.mjs` 为显式联网集成测试：需隔离 localhost `_test` PostgreSQL、对应 Nuxt 服务与 `LAB_GIT_LIVE_TEST=1`、`CONTENT_TEST_DATABASE_URL`、`CONTENT_TEST_BASE_URL`、`NODE_TEST_CLUSTER_SECRET`、`LAB_DATA_DIR`。使用公开的 `10010100111000/attackbox` 下 `node/testdata/compose-lab`，依赖 GitHub 可用及当前公开目录。通过真实 GitHub API 和 Chromium 验证导入、保存、手动检查、确认更新、相同内容复用修订和提交变化冲突；Worker 仍使用模拟节点，不等同于真实沙箱网络验收。

部署执行 `npm --prefix web run db:migrate`。现有目录、密钥、Compose 开关与网络验收门槛见 [平台接入](compose-platform-integration.md)。本轮不清库、不访问 VPS、不默认启用新网络。
