# Compose 本地构建与配套目录

状态：第四轮构建和目录支持已实现。本机真实 Docker Compose 构建／实例生命周期、独立 PostgreSQL＋模拟 Worker＋Chromium 后台流程分别通过。未连接 VPS，真实沙箱 veth 和宿主保护验收门槛仍保留，不自动启用生产新模式。

## 管理员流程

继续使用“上传 YAML／ZIP 或 GitHub 导入 → 预览 → 保存 → 节点准备 → 启用”。后台服务表展示镜像或 Dockerfile、构建上下文与可选 target，提示构建发生在准备阶段；缺 Dockerfile／附件／目录时阻止保存。

```yaml
services:
  web:
    build:
      context: ./app
      dockerfile: Dockerfile
      target: runtime           # 可选的 Dockerfile stage
      args:
        APP_VERSION: ${APP_VERSION:-1}
    volumes:
      - ./config:/etc/app:ro
      - ./data:/var/lib/app
```

也支持 `build: ./app`。context 相对 Compose 所在目录，dockerfile 相对 context；都必须落在整份部署包内。可选择包内其他 Compose 文件，不能借助绝对路径、变量路径或 `../` 逃出部署包。

构建只接受管理员管理的包。普通学员只能启动已准备产物，不接收自定义 Dockerfile、构建参数或宿主路径。

## 构建与不可变产物

`node/labcompose/build.go` 独立承担构建规划与 CLI 执行，复用现有 Prepare 的单任务通道和 30 分钟总期限，不持有实例全局锁。image-only 引擎接口不变，本地构建通过可选 `ImageBuilder` 能力接入。

1. 官方 compose-go 解析显式参数，校验 build 支持范围；记录 context、Dockerfile、target、args 和输入文件摘要。
2. 将经过校验的输入复制到专用私有临时目录，保留相对结构和 `.dockerignore`。原始 Git／上传目录不会直接交给构建器。
3. 通过参数数组调用 Compose `build --builder default --quiet`，明确使用本机默认 Docker builder。构建配置与参数写入 0600 文件，不把变量值放进进程命令行，不继承 Worker 应用密钥或宿主环境变量。
4. 平台根据输入内容和构建配置计算自己的 `attackbox-build:<hash>` 标签，既有 tag 不向用户指定的 Registry 推送，也不覆盖作者在 `image` 中指定的标签。有 build 时明确采用本地构建，不因同时填写 image 而偷偷只拉镜像。
5. 固定实际镜像 SHA256，运行 Document 去除 build，仅保留镜像 ID。学员 `up --no-build --pull never`，镜像丢失时失败并要求重新准备。

相同构建输入可复用本机缓存；相同项目内相同构建只处理一次。基础镜像的可变 tag 不会自动触发重建，建议作者固定 digest 或提交实际的来源变更；后续已新增 [强制重建](compose-preparation-management.md)，显式刷新基础镜像并跳过构建缓存。

临时构建目录准备结束后删除；Worker 重启时在独占 runtime 锁下清理遗留 `.build-*`，不删除部署产物和镜像。服务端仍只接收安全阶段记录，原始构建 stdout/stderr 不回传，避免 Dockerfile 输出变量值；失败提示区分本地构建与镜像拉取。

当前支持 context、dockerfile、args、target。远程 context、additional_contexts、dockerfile_inline、SSH、secrets、额外权限、构建网络选项、跨服务构建上下文和多平台配置明确拒绝。Dockerfile 的基础镜像下载及正常 RUN 联网仅发生在准备阶段，和运行期 internal 网络分开。

上传／Git 包当前以文件内容为准，构建上下文文件规范化为 0644、目录 0755，不保留可执行位。需要执行脚本时使用 Dockerfile `COPY --chmod=0755` 或明确 `chmod`。不要把 Git Token、Registry 密钥作为普通 build args；ARG 可能出现在构建缓存／镜像历史中，专用构建 secrets 不在本轮范围。

## 文件和目录挂载

`package_assets.go` 负责包内扫描与限额，`instance_assets_linux.go` 负责实例副本，生命周期继续由 Manager 管理：

- 只读文件／目录在准备时快照，运行时以 `read_only` 绑定，不引用作者源目录。
- 可写文件／目录从快照创建实例独立副本。每个学员独立；同一实例中的服务共享该副本，保留初始化与应用之间的协作。
- 如果同一来源或重叠目录同时被只读和可写服务使用，只读消费者也接入这一实例的副本，并保持只读挂载，能够观察初始化写入。完全静态的来源继续共享不可变快照。
- 副本只在首次启动前创建；重复 Start、检查状态、Worker 重启和模板更新都不重置已有修改。中途初始化失败保留停止责任，不能返回已运行。
- 释放先确认 Docker 容器已移除、网络已释放，再删除本实例副本与遗留 `.assets-*`。失败保持 cleanup_pending，不假装释放成功。不会删除共享镜像、部署包、其他用户数据或沙箱。
- 实例父目录保持 0700。可写内容使用 0666 文件、0777 可写目录，让不同非 root 容器 UID 可以使用各自副本；只读语义由容器挂载强制执行。部署文件 0444、目录 0755，外层持久目录私有。
- 清理不跟随副本内的符号链接；测试确认副本内链接不能使释放删除外部文件。

包和准备快照仍限制最多 512 文件、单文件 4 MiB、合计 32 MiB，拒绝源 symlink／特殊文件／越界引用。Git 无法保存空目录，现有包也不持久化独立空目录条目；空数据目录优先用 Compose 命名卷，或在来源中放入占位文件。运行中新增数据和构建层不属于这 32 MiB 初始来源限额；本轮未引入磁盘配额或自动镜像 prune。

## 验证与部署

实际构建要求节点安装 Docker Buildx 插件（官方 Docker 软件源的 `docker-buildx-plugin`）；版本检查不精确锁死 Buildx，缺插件时准备记录明确提示。仅使用已存在镜像的模板不新增此依赖。

无需新增数据库迁移或页面轮询。控制端与 Worker 一起升级；来源仍由既有 GitHub／ZIP 导入机制管理，节点准备与模板修订机制不变。基础部署依赖和网络启用门槛见 [平台接入](compose-platform-integration.md)。

- `go test -race ./...`、`go vet ./...`：本地 build 范围、路径、参数隔离、准备去重、不可变运行配置、副本隔离、初始化共享、幂等和清理。
- `npm --prefix web test`、TypeScript、生产构建：预览本地 context／Dockerfile、缺失项、目录和不支持选项。
- `web/tests/compose-builds.test.mjs`：隔离 localhost `_test` 数据库、模拟 Worker 和真实 Chromium，验证缺 Dockerfile、ZIP 上传、构建预览、保存、准备、手动查询和启用。使用与 `compose-labs.test.mjs` 相同的环境变量，两个测试按 `--test-concurrency=1` 执行，避免占用同一个模拟节点端口。
- `ATTACKBOX_TEST_COMPOSE_ENGINE=1 ATTACKBOX_TEST_DOCKER_SOCKET=/var/run/docker.sock ATTACKBOX_TEST_LAB_IMAGE=attackbox-lab-network-fixture:local go test -race ./node/labcompose`：包含 `node/testdata/compose-build`，真实构建镜像、运行初始化服务、非 root 写入、只读保护、同实例共享与多用户隔离，确认释放保留共享镜像。

真实构建测试复用已离线构建的夹具基础镜像，无外部镜像拉取。它不等于真实 Btrfs 沙箱＋veth 接线验收。

准备任务的管理员取消／强制重建已在 [维护迭代](compose-preparation-management.md) 完成；旧模式删除仍按第五轮安排，并以网络验收和旧资源清理完成为前提。
