# AttackBox

AttackBox 是一个面向网络安全实训的在线学习与容器沙箱实验平台，采用 **Control（Web 控制面）** 与 **Worker（Go 容器工作节点）** 分离式架构，编译流程与部署配置完全解耦。

---

## 目录结构

```text
attackbox/
├── web/                        # Control 端：Nuxt 3 全栈 Web 与控制面服务
│   ├── components/             # 前端 UI 组件
│   ├── pages/                  # 页面路由（学习路径、教程阅读、管理后台）
│   ├── server/                 # Nitro 服务端 API、数据库迁移与集群调度中心
│   └── tests/                  # Web 与内容解析器单元测试
├── worker/                     # Worker 端：Go 沙箱运行时与终端网关守护进程
│   ├── cmd/node/               # Worker 进程入口 (main.go)
│   ├── node/                   # 容器生命周期、Btrfs 快照、Cgroups v2、网络与网关
│   ├── protocol/               # Control <-> Worker 集群通信协议定义
│   ├── go.mod                  # Go 模块定义
│   └── go.sum
├── deploy/                     # 独立拆分的服务配置模板
│   ├── control/                # Control 端配置模板
│   │   ├── web.env.example     # /etc/attackbox/web.env 参考模板
│   │   ├── attackbox-web.service
│   │   └── nginx.conf.template
│   └── worker/                 # Worker 端配置模板
│       ├── worker.yaml.example # /etc/attackbox/worker.yaml 参考模板
│       └── attackbox-worker.service
└── scripts/                    # 独立拆分的编译、配置与部署脚本
    ├── build.sh                # 纯编译脚本（支持 all | control | worker，不触碰生产配置）
    ├── setup-control.py        # 生成 Control 配置文件 (/etc/attackbox/web.env)
    ├── manage.py               # Control 数据库迁移 (migrate) 与管理员初始化 (admin)
    ├── deploy-control.sh       # 部署并启动 Control 服务 (attackbox-web + nginx)
    ├── setup-worker.py         # 生成 Worker 配置文件 (/etc/attackbox/worker.yaml)
    └── deploy-worker.sh        # 部署并启动 Worker 服务 (attackbox-worker)
```

---

## 架构与配置规范

编译产物与运行时配置文件严格分离，互不混用：

| 角色 | 源码目录 | 编译产物路径 | 运行时配置文件 | Systemd 服务单元 |
| :--- | :--- | :--- | :--- | :--- |
| **Control 控制端** | `web/` | `web/.output/` | `/etc/attackbox/web.env` | `attackbox-web.service` + `nginx.service` |
| **Worker 工作节点** | `worker/` | `dist/attackbox-worker` | `/etc/attackbox/worker.yaml` | `attackbox-worker.service` |

---

## 第一阶段：编译（Build）

`scripts/build.sh` 仅负责源码构建，不修改任何系统配置、数据库或正在运行的服务。可按节点角色选择编译目标：

```bash
# 同时编译 Control 与 Worker（同机部署推荐）
./scripts/build.sh all

# 仅编译 Control Web 控制端（产物位于 web/.output）
./scripts/build.sh control

# 仅编译 Worker 守护进程（产物位于 dist/attackbox-worker）
./scripts/build.sh worker
```

---

## 第二阶段：Control 端配置与部署

### 1. 生成 Control 配置文件 (`/etc/attackbox/web.env`)

支持交互式向导或命令行参数生成配置与安全密钥：

```bash
# 方式 A：交互式填写
sudo ./scripts/setup-control.py

# 方式 B：命令行非交互生成（适合自动化部署）
sudo ./scripts/setup-control.py \
  --database "postgres://attackbox:YOUR_DB_PASSWORD@127.0.0.1:5432/attackbox_db" \
  --origin "http://YOUR_SERVER_IP_OR_DOMAIN" \
  --force
```

执行完成后会输出自动生成的集群通信密钥 `SANDBOX_CLUSTER_SECRET`，供后续 Worker 节点配对使用。

### 2. 初始化数据库结构与管理员账号

```bash
# 执行 PostgreSQL 数据库迁移
sudo ./scripts/manage.py migrate

# 创建初始管理员账号（支持交互式或命令行传参，密码至少 12 位）
sudo ./scripts/manage.py admin \
  --email "admin@attackbox.local" \
  --password "YOUR_ADMIN_PASSWORD" \
  --name "Administrator"
```

### 3. 部署并启动 Control 服务

```bash
./scripts/deploy-control.sh
```

该脚本会校验 `web/.output` 与 `/etc/attackbox/web.env`，原子发布静态与服务端产物至 `/var/www/attackbox-web`，渲染 Nginx 配置并启动 `attackbox-web` 与 `nginx`，最后自动执行 Web 健康检查。

---

## 第三阶段：Worker 端配置与部署

Worker 节点既可以与 Control 部署在同一台服务器，也可以独立部署在多台弹性容器节点上。

### 1. 生成 Worker 配置文件 (`/etc/attackbox/worker.yaml`)

- **同机部署**：`setup-worker.py` 会自动从本机 `/etc/attackbox/web.env` 读取 `SANDBOX_CLUSTER_SECRET`，无需手动复制密钥：

```bash
sudo ./scripts/setup-worker.py --force
```

- **异机独立部署**：指定 Control 节点的连接地址与集群密钥：

```bash
sudo ./scripts/setup-worker.py \
  --node-id "node-worker-01" \
  --control-url "ws://CONTROL_HOST:3000/cluster/connect" \
  --cluster-secret "CLUSTER_SECRET_FROM_CONTROL" \
  --public-url "ws://WORKER_PUBLIC_HOST:8443" \
  --force
```

### 2. 部署并启动 Worker 服务

```bash
./scripts/deploy-worker.sh
```

该脚本会将 `dist/attackbox-worker` 安装至 `/usr/local/bin/attackbox-worker`，配置并启动 `attackbox-worker.service`，并校验本地 `:8443/health` 网关状态。

---

## 日常更新流程

代码更新后，仅需重新编译对应模块并执行对应部署脚本，**无需重新运行 `setup-*.py`**：

```bash
git pull

# 更新 Control 端
./scripts/build.sh control
sudo ./scripts/manage.py migrate
./scripts/deploy-control.sh

# 更新 Worker 端
./scripts/build.sh worker
./scripts/deploy-worker.sh
```
