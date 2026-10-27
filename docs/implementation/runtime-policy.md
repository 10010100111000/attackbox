# 统一运行策略

后台「系统管理 → 运行策略」统一配置沙箱使用时长、沙箱空闲回收与靶场使用时长。默认分别为 **60、10、60 分钟**。界面使用分钟，创建沙箱时换算为秒发送给 Worker，靶场保存固定的 expiresAt 并交给 Worker 执行到期回收。

业务时限保存在 PostgreSQL runtime_policies 的单行配置中，首次未保存时使用上述默认值。不提供节点覆盖、模块覆盖或环境变量覆盖。已移除节点设置里的 TTL／Idle 输入、接口字段及数据库列；SANDBOX_TTL_SECONDS／SANDBOX_IDLE_SECONDS 不再读取。节点保留名称、容量、内存、进程数与排空设置。密钥仍留在服务端环境文件。

所有时长限制为 1–1440 分钟的整数，空闲时间不能超过沙箱使用时长。设置成功后无需重启 Web 或 Worker，新启动实例立即读取数据库中的策略；旧实例、重复启动请求保持原到期时间。没有新的定时器或前端轮询。

管理员接口 GET／PUT /api/admin/runtime-policy 提供服务端鉴权、禁止缓存、跨站写入检查、严格校验及版本冲突保护。保存失败保留表单输入，409 时可明确重新载入最新值。Worker 靶场校验已从固定一小时调整为最多 24 小时的安全边界，实例租期依然不可在启动后修改。

## 部署

```bash
npm --prefix web run db:migrate
npm --prefix web run build
# 通过项目部署流程同步编译、升级 Worker
```

新增 0002_runtime_policy：新增全局策略表并移除节点级时限列，不沿用旧节点时限，采用新默认值。需要成套部署新 Web 与新 Worker，旧 Worker 会拒绝超过一小时的靶场租期。修改策略本身不重启服务；部署重启 Worker 对当前环境的影响仍按既有部署说明处理。

## 验证

- 单元测试验证默认值、时限范围及空闲时间关系。
- 协议测试验证每个新沙箱获取策略快照、旧沙箱保持原截止时间。
- 本地 PostgreSQL、协议 Worker 夹具与 Chromium 联调验证后台保存、刷新恢复、权限、冲突、实际 RPC 秒数、三小时靶场租期与重复启动不改期。
- Go 测试验证三小时租期可用、超过 24 小时被拒绝。真实 Worker 到期清理由既有回收逻辑执行，本轮未连接 VPS 进行长时验收。

```bash
CONTENT_TEST_DATABASE_URL=postgres://.../policy_test \
CONTENT_TEST_BASE_URL=http://127.0.0.1:3000 \
NODE_TEST_CLUSTER_SECRET=local-cluster-secret \
npm --prefix web run test:runtime-policy
```

测试服务器应启用沙箱调度并配置 test-node 节点，数据库须为隔离本地空库。
