# Resend 账户邮件

注册验证、重新发送验证邮件与找回密码使用 Resend HTTP API。没有控制台邮件替代实现，也没有浏览器可读取的验证／重置 Token 字段。旧的公开 Token 自动化测试入口已移除。

## 服务端配置

在 Resend 验证发送域名（按控制台配置 SPF／DKIM），创建仅允许发送邮件的 API Key，优先限制为指定发送域名。在 `/etc/attackbox/web.env` 配置：

```dotenv
RESEND_API_KEY=re_replace_with_your_key
MAIL_FROM=AttackBox <noreply@your-domain.example>
APP_BASE_URL=https://your-site.example
```

`APP_BASE_URL` 必须是完整站点 origin，不包含路径、查询参数、用户名或密码；正式地址要求 HTTPS，只有本地开发允许 loopback HTTP。链接不使用请求 Host 或浏览器传入的域名。

配置由现有 systemd `EnvironmentFile` 注入 Nuxt 服务端，不放进 Nuxt public 配置、Git、数据库或前端表单。示例文件只保留占位值。配置文件可由 root 持有、权限设为 `0600`（系统级 systemd 读取后向服务进程注入环境）。修改后重启 `attackbox-web`。

本轮不新增数据库迁移。未配置邮件服务时返回明确的服务不可用错误，注册不会因此创建新账号。

## 发送与账户状态

- 每次请求向固定的 `https://api.resend.com/emails` 发送一封邮件，包含 HTML 和纯文本；超时 10 秒，不自动重试 POST。Token 派生的幂等键避免同一凭证重复投递。
- 验证链接有效 24 小时；密码重置链接有效 30 分钟。数据库只保存 Token 哈希，正常日志不打印链接、Token、API Key 或供应商原始错误。
- 每个账号、每种邮件用途至少间隔 60 秒，每小时最多 5 次成功提交；通过数据库行锁与已有 Token 记录限制并发，重启不会清空限制。这是收件人维度的限制，不替代将来的入口 IP／全站发送预算。
- 发送成功后才使同用途的旧凭证失效；发送失败回滚新凭证。注册已创建的 pending 账号保留，可以重新申请验证邮件，不需要重建账号。
- 找回密码对不存在、未激活、停用、冷却中的账号返回相同结果。投递失败也不暴露账号是否存在，服务端仅记无敏感信息的错误；用户可以间隔后重新申请。
- 重置密码时，在同一事务内锁定用户并重新检查 Token，仅允许一次成功消费，同时使其他未消费的密码重置凭证与旧登录会话失效。停用账号不能通过重置密码恢复访问。

## 本地测试

```bash
npm --prefix web test
# 先为隔离本地测试库执行 db:migrate；集成测试自行启动临时 Nuxt 服务。
CONTENT_TEST_DATABASE_URL=postgres://.../account_test npm --prefix web run test:auth-mail
```

`test:auth-mail` 通过测试进程 preload 将固定 Resend 请求转入本地邮件捕获器，应用没有可修改的供应商 API 地址或公共“获取测试 Token”接口。覆盖注册、未激活登录拒绝、真实邮件链接激活、重新发送、冷却、小时额度、供应商失败回滚、过期凭证、并发重置、旧会话撤销及前端重发操作。

本地测试不使用真实 Resend Key，也不向真实收件人发送邮件。正式域名验证和实际投递需部署者配置后确认。

API 参考：https://resend.com/docs/api-reference/emails/send-email
