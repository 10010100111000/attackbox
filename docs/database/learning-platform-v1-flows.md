# 内容接口与事务

数据结构见 [数据模型](learning-platform-v1.md)。所有管理接口要求 active 管理员；公开接口不返回标准答案原文或摘要。

## 管理接口

| 方法 | 路径 | 行为 |
| --- | --- | --- |
| GET | /api/admin/modules | 列表：直接返回标题、状态、章节数、分类、难度、lockVersion |
| POST | /api/admin/modules | 首次保存并创建模块，返回 `{ module }` |
| GET | /api/admin/modules/:id | 返回 `{ module, category, chapters, prerequisites }` |
| PUT | /api/admin/modules/:id | 保存元信息及完整章节清单，返回 `{ module }` |
| POST | /api/admin/modules/:id/publish | 校验完整性后发布 |
| POST | /api/admin/modules/:id/unpublish | 隐藏模块，保留内容 |
| DELETE | /api/admin/modules/:id | 永久删除模块及下属内容 |
| POST | /api/admin/categories | 新增分类 |

更新、发布、下架、删除均需提交当前 `lockVersion`；缺失或不匹配返回 409。不存在的模块返回 404。输入无效、发布不完整返回 400。重复访问标识返回 409。

不再提供 `/draft`、单章节草稿、独立章节排序或移除草稿接口；章节编排统一通过模块 PUT 保存。

### 保存请求

```json
{
  "lockVersion": 1,
  "intent": "publish",
  "prerequisiteModuleIds": [],
  "learningOutcomes": ["查看当前用户与文件权限"],
  "title": "Linux 基础",
  "categoryId": null,
  "difficulty": "beginner",
  "chapters": [
    {
      "chapterId": "已有章节 ID，新章节省略",
      "title": "认识 Shell",
      "access": "public",
      "bodyMarkdown": "# Shell",
      "durationMinutes": 20,
      "questions": [
        {
          "id": "已有题目 ID，新题省略",
          "promptMarkdown": "查看当前用户的命令？",
          "normalization": "trim",
          "answer": "whoami"
        }
      ]
    }
  ]
}
```

- `intent: "draft"` 保存并下架；`intent: "publish"` 保存并发布，包括新建模块。编辑器只发送一次 POST 或 PUT，不再串行保存与发布。省略 intent 的旧客户端仍按当前状态保存。
- `prerequisiteModuleIds` 为有序模块 ID 数组，空数组清除，省略保持；最多 12 个，不能成环。管理详情返回 `prerequisites`（含发布状态），公开详情只返回已发布项。
- 章节 `durationMinutes` 是可选整数（1–1440），显式 null 清空，省略保留；公开目录与章节正文均返回此值。
- `learningOutcomes` 为可选学习目标列表；空数组清空，省略保持原值。
- `chapters` 数组顺序就是章节顺序；完整数组中省略的已有章节会被删除。
- 整个 `chapters` 字段省略表示不改章节。
- 章节的 `questions` 字段省略表示不改题目；空数组表示移除全部题目。
- 管理详情返回 `answer` 原文，管理员可直接修改；`answer` 省略时保留原值，空字符串表示清空。修改匹配方式自动使用现有答案重新计算判题摘要。`clearAnswer: true` 仅保留为旧客户端兼容参数，编辑器不再使用。
- ID 必须属于当前模块/章节，不能引用其他模块的内容。

### 保存事务

1. 锁住模块行，比较 `lockVersion`。
2. 更新元信息；删除移除的章节；暂移位置后依序写入章节和题目，保留已有 ID 和答案。
3. 根据 intent 确定目标公开状态；若目标为发布，校验全部章节和题目完整性；同一事务写入公开状态。
4. 递增 `lockVersion`，记录审计，同一事务提交。任何失败都回滚。

读取管理详情、目录、正文采用一致性读，避免并发保存过程中拼接出不同时间点的内容。编辑器一次保存全部表单；保存成功后重新读取服务端数据。

## 学习接口

| 方法 | 路径 | 行为 |
| --- | --- | --- |
| GET | /api/categories | 分类清单 |
| GET | /api/modules | 已发布模块列表 |
| GET | /api/modules/:slug | `{ module, chapters, prerequisites, progress }`，目录包含 lockReason 与完成时间 |
| GET | /api/modules/:slug/chapters/:chapterSlug | `{ chapter, questions, progress }`，服务端检查章节访问权限 |
| POST | /api/modules/:slug/chapters/:chapterSlug/attempt | 提交 `{ questionId, answer }`，返回 `{ correct, errorHint, progress }` |
| POST | /api/modules/:slug/chapters/:chapterSlug/visit | 登录后记录最近访问，不返回正文 |
| POST | /api/modules/:slug/chapters/:chapterSlug/complete | 登录后完成章节；有题章节仍需全部通过 |
| GET | /api/me/learning | 当前用户最近 20 个已访问且仍发布的模块及实时进度 |

目录的 `lockReason` 为 `login`、`prerequisite` 或 null；`isLocked` 与之保持一致。需要登录返回 401，未完成前章返回 403；题目已删除或模块下架返回 404。公开预览不受顺序限制，已完成章节可回顾。

章节 GET 和 attempt 响应中的 `progress` 为 `{ completedAt, passedQuestionIds, isSaved }`。访客返回空通过记录、`isSaved: false`；已登录用户的数据来自自己的记录。答对会幂等写入题目通过记录，全部题目答对时自动完成。显式 complete 不接受客户端传入的已通过状态；未通过全部练习返回 409。无题章节可直接 complete。

模块 progress 为 `{ completedChapters, totalChapters, percent, isCompleted, resumeChapterSlug }`，访客为 null。继续学习优先返回最近访问的可读未完成章节，否则选择第一个可读未完成章节；全部完成则返回回顾入口。

读取接口不写进度。前端打开已登录用户的可读章节后发送 visit；写入失败显示重试入口。响应均禁止共享缓存；SSR 保持公开目录和权限提示，不输出受限正文或标准答案。
