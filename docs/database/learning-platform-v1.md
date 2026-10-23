# AttackBox 数据模型

当前版本面向全新数据库。内容直接编辑、保存后生效；不提供历史版本、草稿副本或回滚功能。

## 已实现的数据表

| 表 | 用途 |
| --- | --- |
| users | 账号、密码摘要、角色、状态、邮箱验证 |
| auth_sessions | 登录会话，保存令牌摘要 |
| email_tokens | 邮箱验证与找回密码令牌 |
| audit_logs | 操作人、动作、目标及时间，不保存正文历史 |
| content_categories | 内容方向分类，与学习难度独立 |
| modules | 模块元信息、公开状态、并发修改计数 |
| chapters | 模块的有序章节、Markdown 正文和访问权限 |
| chapter_questions | 章节题目、判题规则和标准答案 HMAC |
| assets | 文件存储索引与资源属性 |
| chapter_assets | 章节与附件的关联 |

## 模块

`modules` 直接保存 `title`、`slug`、`summary`、`description_markdown`、`difficulty`、`category_id`、`cover_asset_id`。

- `status` 只有 `unpublished` 和 `published`。下架回到 `unpublished`，内容不变。
- `lock_version` 从 1 开始，每次修改或发布状态变更递增，用于阻止过期页面覆盖内容。它不是内容历史版本。
- `created_by`、`created_at`、`updated_at` 记录创建与更新时间。
- `published_at` 记录首次发布时刻，下架不清除。
- `slug` 全局唯一，创建后不可修改。
- 删除模块时真实删除其章节、题目和附件关联；附件文件本身不连带删除。

默认提供 Web 安全、Linux 基础、网络安全、云原生安全、逆向分析、容器安全六种分类。管理员可添加分类。未指定分类显示“未分类”。难度为 `beginner`、`intermediate`、`advanced`。

## 章节和题目

`chapters` 直接属于模块，保存 `position`、`title`、`slug`、`access`、`body_markdown`。模块内访问标识与位置分别唯一，位置为正整数。

`chapter_questions` 直接属于章节，保存稳定的 `id`、`position`、`prompt_markdown`、`error_hint`、`normalization`、`answer_hmac`、`answer_key_version`。

- 普通编辑与排序保留章节和题目 ID，不复制记录。
- 新建题目产生新 ID；删除题目真实删除记录。
- 答案仅存 HMAC。编辑器读取 `hasAnswer`；留空表示保留，明确勾选清除才删除答案。
- 规则支持 `exact`、`trim`、`trim_casefold`。更改已有答案的规则需要重新输入答案。
- 公开接口通过字段白名单返回数据，不返回答案摘要或内部管理字段。

## 公开与鉴权

模块介绍和目录在模块发布后公开。`public` 章节直接可读；`authenticated` 章节要求有效且已激活的账号。鉴权在正文及答题接口执行。

顺序解锁、学习进度持久化尚未实现。未来完成记录应关联稳定的章节 ID，普通正文更新默认保留已完成状态，不需要重建版本系统。

## 保存与发布

- 新建页只维护本地表单，首次保存才创建模块，初始为未发布。
- 保存包含整个模块、章节清单、题目和顺序，在一个事务内完成。
- 已发布模块的保存立即生效，同时执行发布完整性校验。
- 未发布模块允许保存不完整正文和未设置答案的题目。
- 发布要求至少一个章节、各章正文完整、已有题目均有题干和标准答案；允许章节无题。
- 下架仅改变状态，所有已保存内容继续可编辑。

数据库只保留当前内容。故障恢复依靠数据库备份，不以内容历史表替代备份。
