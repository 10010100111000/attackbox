# 学习平台 v1：业务事务与接口契约

配套：[数据库与字段字典](learning-platform-v1.md)。本文为实现契约，不表示接口或迁移已上线。

## 1. 草稿编辑与发布

### 新建模块

1. 验证用户为 active admin。
2. 事务内生成 modules 稳定 ID 与不重复 slug，创建 revision_no=1 的 draft。
3. 新建只要求名称，正文与题目可以留空；不要为了发布校验阻止未完成草稿保存。
4. 返回模块 ID、草稿 revision ID 与 lock_version，页面进入编辑器。

### 保存章节与题目

1. 客户端提交完整章节编辑内容和预期 lock_version，不能提交 published_revision_id 或操纵 sealed 状态。
2. 事务中先锁 modules，再锁当前 draft module_revision，核对 lock_version。
3. 对修改的章节新建 is_sealed=false 的 chapter_revision；写正文、题目、答案 HMAC 与附件关联。
4. 校验所有题目和引用完整，在同一事务中封存正文修订；更新 draft 清单中的章节修订引用。
5. 未改动章节直接保留原引用，结构校验失败则全部回滚。未完成的题目也允许随草稿保存：提示可暂空、answer_hmac 可为 NULL，但发布校验必须拒绝缺失答案或提示的题。封存仅表示版本不可变，不代表已经达到发布要求。
6. lock_version 加一并提交。409 返回当前锁版本和冲突原因，由页面提示重新加载／比对，不使用最后写入覆盖。

这是“模块草稿可编辑，正文快照不可变”，不是为每次按键都创建完整版本。第一版显式保存即可；将来自动保存应节流并复用同一保存事务。

### 发布模块

事务顺序：

```text
锁 modules 行 FOR UPDATE
→ 锁 draft module_revision 行 FOR UPDATE，核对 lock_version
→ 校验名称、介绍、封面、章节顺序、正文、测验配置与资源归属
→ 校验每个引用的 chapter_revision 已封存且归属正确
→ 将 draft 改为 published，写 published_at
→ 更新 modules.published_revision_id
→ 追加 audit_logs
→ COMMIT
```

发布要求至少一章，章节标题与正文非空，题目 prompt、规范化规则、标准答案摘要有效；正文 access 明确。事务失败时旧发布指针不变。后台预览 draft 也必须管理员鉴权；用户接口不接受任意 revision_id 请求草稿。

发布成功后失效公开模块介绍、目录、关联路线和首页缓存。再次编辑时从发布版本复制模块元数据与清单生成新 draft，封存正文继续复用。

### 下架与恢复

- 下架清空 modules.published_revision_id，并在同一事务清空所有引用它的线上路线发布指针；保留原发布历史和学习记录。
- 归档通过 archived_at 影响可用性，不能只设置 archived_at 而让公开查询忘记过滤。
- 已发布首页配置不原地修改；首页接口过滤目标已下架的条目并失效缓存。
- 恢复时重新执行发布校验并指向有效发布修订；路线不自动恢复，管理员确认后重新发布。
- 公开接口禁止通过旧修订 ID 绕过下架。历史版本只供管理员查看，学习者看到自己的完成历史不代表能读已下架正文。

路线发布和模块下架并发时，统一先按 ID 排序锁相关 modules，再锁路线。路线发布必须重新校验锁版本和所引用模块集合；若锁定前后草稿内容发生变化，回滚重试，不拿旧集合完成发布。

## 2. 阅读授权与目录返回

目录响应可包括：模块标题、介绍、章节标题、位置、是否公开、当前用户的完成／锁定状态。不能带 Markdown 正文、答案摘要、私有源链接或私有附件直链。

正文读取决策：

```text
模块／章节不在当前发布版本或已归档 → 404
当前章 public → 返回公开正文
当前章 authenticated 且无 active 登录 → 401（不带正文）
该章已完成 → 返回正文
该章之前的所有当前章节均已完成 → 返回正文
否则 → 403 + prerequisite_required（可含缺少的前置章节 ID）
```

- public 只豁免阅读锁；进度保存、测验判定和章节完成必须登录。
- active 管理员可以通过独立管理预览接口查看任何草稿，不把管理员角色伪装成用户已完成。
- 同一模块的新章节可能改变后续未完成章节的前置集合；已经完成的章节仍可读。
- 受限正文、锁定正文和个性化目录不能共用公共缓存。SSR 不等于公开：服务端先鉴权，再决定是否把正文写入 HTML／Nuxt payload。
- 预取、搜索索引、Open Graph、Markdown 下载、图片附件都遵守同一内容边界。

## 3. 答题事务与并发

请求字段：`question_id`、`module_revision_id`、`answer`、`idempotency_key`。请求体限制大小；user_id 必须来自 Session，不能信任客户端提交。

1. 验证 active Session、CSRF／同源要求、接口限流。
2. 如已有相同 `(user_id, idempotency_key)`，用服务端请求 HMAC 校验题目、模块版本、输入一致；一致返回原判定，不重复写入，不一致返回 409。
3. 对目标 modules 行加 FOR SHARE 锁，读取当前发布指针；请求版本过期返回 `409 content_updated`，提示刷新章节，不能悄悄按旧题判定。
4. 检查题目位于当前发布版本、章节所属模块正确，并执行与正文相同的阅读／顺序授权。
5. `INSERT user_learning_activity ... ON CONFLICT DO NOTHING`，然后锁该用户该模块的活动行 FOR UPDATE。所有答题与手动完成事务都使用这把锁，串行化同一用户同一模块的进度写入。
6. 服务端规范化输入，使用该题密钥版本计算 HMAC 并进行常量时间比较。
7. 插入 answer_attempts。通过 UNIQUE 处理并发重复 idempotency_key；若冲突，读取原行并核验请求 HMAC。
8. 查询本章当前 chapter_revision 的所有题是否均有该用户的正确 attempt。
9. 全部通过则 `INSERT chapter_completions ... ON CONFLICT DO NOTHING`，保留第一次完成证据与时间。
10. 更新 user_learning_activity，提交。返回本题是否正确、是否完成本章及下一章 ID；不返回标准答案。

第 5 步是必要的：两道题同时提交时，不能各自因看不到另一事务尚未提交的正确记录而都漏掉“完成章节”。模块行的共享锁则让发布、下架与判题具有明确的先后顺序。

已经完成的章节仍可复习答题，第一次完成记录不改。对新版本的再次作答仅追加新的 attempt。新的完成判定以当前发布题目为准，不能用同一 question_key 的旧版本正确结果替代。

## 4. 无测验章节完成

`POST .../complete`：校验 active 用户、当前发布版本、阅读资格；按上述模块 → 用户模块活动行顺序加锁，重新确认当前章确实没有题，再插入 method=manual_mark 的完成记录。

如果章节有测验，接口返回 `409 quiz_required`。不能相信客户端传来的 `is_complete=true`，也不能把打开页面、停留时间或浏览器 localStorage 当作完成依据。

## 5. 账号事务

- 注册创建 pending 用户与验证 token 摘要。邮件包含原始随机 token，数据库只存摘要。发送失败允许重新请求，不允许激活未验证账号。
- 发送邮件在事务提交后执行；若要求可靠重试，应在认证实现时加持久任务／邮件 outbox，不能持有数据库事务等待邮件供应商。
- 验证邮箱：锁 token 与用户，检查用途、目标邮箱与有效期；更新 email_verified_at、pending→active，消费 token。suspended 用户不能借验证邮件恢复 active。
- 登录：验证密码及状态后生成 Session；返回 HttpOnly Cookie，不返回可供 localStorage 保存的长期 token。登录失败提示不区分邮箱不存在和密码错误。
- 重置密码：锁 token 与用户，修改哈希、消费所有同用途有效 token、撤销会话，事务提交后要求重新登录。已有封禁状态不变。
- 封禁与角色变更：获取专用事务级顾问锁 admin_guard，再锁目标用户并检查 active admin 数量，防止并发操作一起移除最后一位 active admin；变更、会话撤销、audit_logs 同事务。
- OAuth 只有启用提供方后才开放；验证 state，OIDC 验证 nonce／issuer／audience／签名，身份关联不能只按邮箱合并。

不迁移旧账号也意味着不验证旧 bcrypt、不复用旧 Cookie；生产切换时为新 Session 使用独立 Cookie 名称与签名／摘要密钥，避免旧服务误识别。

## 6. 最小接口契约

| 场景 | 接口建议 | 权限和返回边界 |
| --- | --- | --- |
| 首页内容 | GET /api/home | 只含当前有效公开介绍与公开封面 |
| 模块列表 | GET /api/modules | 仅当前发布、未归档模块；分页 |
| 模块详情与目录 | GET /api/modules/:slug | 介绍公开；个性化状态不共享缓存 |
| 章节正文 | GET /api/modules/:slug/chapters/:chapterSlug | 按章节访问／顺序规则授权 |
| 单题提交 | POST /api/learning/answers | active 登录，当前发布版本，幂等判题 |
| 无测验完成 | POST /api/learning/chapters/:id/complete | active 登录且章节无题 |
| 我的学习 | GET /api/me/learning | 当前用户活动与完成记录 |
| 路线列表／详情 | GET /api/paths、GET /api/paths/:slug | 只展示可用的发布路线 |
| 注册／邮箱验证 | POST /api/auth/register、POST /api/auth/verify-email | 服务端校验，凭证只用一次 |
| 登录／退出／当前身份 | POST /api/auth/login、POST /api/auth/logout、GET /api/auth/me | Cookie Session |
| 忘记／重置密码 | POST /api/auth/forgot-password、POST /api/auth/reset-password | 不暴露账号是否存在 |
| 创建／保存模块 | POST /api/admin/modules、PUT /api/admin/modules/:id/draft | active admin，保存检查 lock_version |
| 发布／下架 | POST /api/admin/modules/:id/publish、POST /api/admin/modules/:id/unpublish | active admin，事务加审计 |
| 管理正文／题目 | 随模块 draft 保存，不开放任意 revision 写入 | active admin，创建并封存章节修订 |
| 路线与首页 | /api/admin/paths、/api/admin/homepage 的 draft/publish 操作 | 与模块相同的草稿／发布契约 |
| 私有附件 | GET /api/chapters/:id/assets/:assetId | 当前发布关联与正文阅读权限 |

默认所有写操作验证 CSRF／Origin，不因使用 Cookie 就忽略跨站请求。列表分页、输入长度、错误码需在实现时固定；不要让客户端提交任意数据库字段更新 users 或 published 指针。

## 7. 实现阶段必须通过的验证

1. 保存草稿不改变访客看到的标题、排序、正文、题目和图片。
2. 非管理员不能按 revision UUID 读取草稿、题目摘要或私有来源信息。
3. published／sealed 记录及附属关系在数据库层拒绝原地修改与删除。
4. 章节不能关联到另一个模块的版本，完成证据不能引用不存在的章节清单。
5. 访客可直接读后面的公开章节，但读不到后面的 authenticated 章节。
6. 登录用户必须完成当前所有前置章节才能读未完成的受限章节。
7. 全部题目分多次答对仍完成章节；之后答错不撤销完成。
8. 两题并发答对不丢失完成；同一幂等请求重放不重复计数，换请求复用 key 冲突。
9. 发布更新与判题并发时，按锁顺序判定结果，不记录不存在的发布证据。
10. 已完成章节改题后仍已完成；未完成章节的旧版本部分答题不能拼接到新版本。
11. 新增章节只影响当前模块完成比例和未完成章节的前置判定，不抹掉已有完成记录。
12. 下架模块同时下架受影响路线，首页不再链接到该目标，缓存不继续返回正文。
13. 无题章节可显式完成，有题章节不能绕过判题标记完成。
14. 已封禁用户即使有旧 Session 或已完成记录也无法访问受限内容。
15. 并发停用管理员仍至少保留一个正常管理员。
16. 注册、邮件验证、密码重置与 Session 撤销可跨服务重启保持正确状态，不靠内存定时器维持凭证有效期。

这些是后续迁移／接口测试的验收项，本轮只完成设计，没有运行尚不存在的数据库服务或伪造测试通过结果。
