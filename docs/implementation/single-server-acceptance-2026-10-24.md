# 2026-10-24 真实 Worker 单机验收

结果：独立部署的 Nuxt + PostgreSQL + Go Worker 完成真实单机验收。通过 Tailscale SSH 访问 `laji@100.69.228.63`，并用 SSH 转发让本地 Chromium 连接服务器的应用和数据网关；未使用模拟 Worker 或模拟桌面。

## 验收环境

- x86_64，内核 `7.1.13+parrot7-amd64`；Node `20.19.2`、Go `1.24.4`。
- 根分区 Btrfs、Cgroup v2，宿主具备 MATE、TigerVNC 和免密 sudo。
- 专用单进程 Nuxt：`127.0.0.1:3031`；Worker 网关：`127.0.0.1:18443`。
- 专用数据库 `attackbox_acceptance_20261024`、节点 `acceptance-node`、网桥 `br-abtest`、防火墙表 `inet attackbox_acceptance_20261024`。
- 实例和金盘位于 `/var/lib/attackbox-acceptance-20261024`，Cgroup 使用相同后缀的独立根目录。金盘由已有只读金盘派生后再次脱敏并锁定，不改原金盘。
- 临时部署单元为 `attackbox-acceptance-web` 与 `attackbox-acceptance-node`，原 `attackboxd`、`attackbox-web` 未切换。

## 已通过

| 场景 | 证据 |
| --- | --- |
| 节点接入 | 真实 Go Worker 注册、心跳、Ed25519 公钥同步成功 |
| 实例启动与身份 | API 创建真实 Btrfs 快照及 namespace 进程；PTY 中 `id` 为 `uid=1001(box)` |
| 唯一实例与并发 | 同账号重复启动返回相同实例；并发上限为 1 时第二账号被拒绝 |
| 多终端 | 两个 PTY 能读写同一份 `/home/box` 文件 |
| GUI | Chromium 中真实 noVNC 解码 MATE/TigerVNC 画面；通过鼠标、键盘在桌面终端创建文件，并从 CLI 读取内容 |
| 桌面重连 | 浏览器重新加载后 VNC 进程 PID 保持不变；没有创建第二个桌面或沙箱 |
| CLI 重新进入 | 新 Shell 能读取此前 GUI 写入的文件；不将此称为恢复旧 Shell 的会话内容 |
| 控制通道重连 | 中断专用 3031 端口的已有 TCP 连接，Worker 以同 boot id 重连，现有实例与文件保留 |
| 资源限制 | 宿主读取到 `memory.max=1073741824`、`memory.swap.max=0`、`pids.max=512`、`cpu.weight=100` 及 I/O 限制；另一轮 512 MiB 策略对应实际 `memory.max=536870912` |
| 排空与新策略 | 排空保留当前实例并拒绝新用户；修改策略不改变旧实例 |
| Control 重启 | 数据库中的节点设置恢复；Worker 重连后清理 Control 无法确认归属的实例 |
| Worker 重启 | 旧实例被清理，新 boot id 注册并重新进入可调度状态 |
| 无响应与补偿 | 暂停专用 Worker，创建失败后平台保留 `cleanup_failed`；恢复 Worker 后迟到结果与补偿清理收敛，未虚报释放 |
| TTL | 设置 60 秒 TTL，同时每 7 秒发送真实终端输入，实例仍按 TTL 回收 |
| Idle | 设置 180 秒 TTL、60 秒 Idle，只轮询 API，不发送输入，实例按 Idle 回收 |
| Ticket | 真实 Worker 拒绝越权、签名篡改、重复使用、未使用但已过期的 Ticket 及外站 Origin |
| 主动释放 | 用户和管理员释放均收到真实 Worker 确认；检查实例目录、Cgroup、测试 veth 与 namespace 进程已清空 |

Go 的 `./node`、`./node/runtime`、`./protocol` 常规测试与 `go vet ./node/... ./protocol` 通过。非 root Go 测试中的内核用例仍可能跳过；上述真实生命周期证据来自本次 API、浏览器及宿主检查。另通过 80 项 Web 常规测试、TypeScript 检查和生产构建。

## 本轮修复

1. `network.firewall_table` 支持选择独立 nftables 表，默认保持 `attackbox`。转发链仅拒绝本 Worker 网桥未获准的流量，不再用全局默认 drop 影响其他宿主转发。表名在任何宿主修改前校验；增加相应 Go 测试，并在真实 nftables 中应用验证。
2. 学习页 CLI／GUI 按钮在客户端挂载完成前禁用，避免 SSR 首屏按钮看似可点击、事件尚未绑定时丢失首次操作。
3. 预检脚本同时检查 sudo PATH，避免将 `/usr/sbin/nft` 误报为未安装。服务器无需安装 npm 即可运行上传的生产构建；迁移依赖单独准备。

## 重现入口

这些脚本仅在显式设置 `RUN_REAL_SANDBOX_ACCEPTANCE=true` 且提供专用账号 fixture 路径后运行，不加入默认 `npm test`：

- `web/tests/remote-sandbox-acceptance.mjs`：真实创建、CLI、GUI、重连、用户与管理员释放。
- `web/tests/remote-sandbox-recovery.mjs`：排空、策略、重启、暂停／恢复、TTL、Idle。仅操作明确命名的测试 systemd 单元。
- `web/tests/remote-sandbox-tickets.mjs`：与恢复脚本的 60 秒 TTL 阶段配合，先检查重放／篡改／Origin，再以 `expired` 参数检查未使用的过期 Ticket。

前提是独立测试部署已按上述路径建立，SSH 主机公钥已验证，`15222` 为授权的 Tailscale CONNECT 转发；应用和网关的 SSH 转发分别为本机 `3031` 与 `18443`。fixture 包含测试账号凭据，仅存受保护的临时文件，不入库。截图 `/tmp/attackbox-real-gui.png` 仅包含专用测试桌面。

## 部署边界

本次为私有单机验收，没有切换公开站点、更新线上数据库或移除旧 Go Control。公开 HTTPS/WSS 反向代理、正式域名和正式切换回退流程仍需单独执行；本次结果不代替多节点验收。

## 收尾结果

已停止并移除临时 systemd 单元，删除专用数据库与角色、测试发布目录及私有配置、金盘副本、独立 Cgroup 根、网桥与测试防火墙表。实例目录、Cgroup、veth 和测试进程检查为空。原 `attackboxd`、`attackbox-web` 仍为 active，原金盘仍为 `ro=true`，现有数据库保留。
