# 内容接口与事务

数据结构见 [数据模型](learning-platform-v1.md)。所有管理接口要求 active 管理员；公开接口不返回标准答案摘要。

## 管理接口

| 方法 | 路径 | 行为 |
| --- | --- | --- |
| GET | /api/admin/modules | 列表：直接返回标题、状态、章节数、分类、难度、lockVersion |
| POST | /api/admin/modules | 首次保存并创建模块，返回 `{ module }` |
| GET | /api/admin/modules/:id | 返回 `{ module, category, chapters }` |
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
  "title": "Linux 基础",
  "categoryId": null,
  "difficulty": "beginner",
  "chapters": [
    {
      "chapterId": "已有章节 ID，新章节省略",
      "title": "认识 Shell",
      "access": "public",
      "bodyMarkdown": "# Shell",
      "questions": [
        {
          "id": "已有题目 ID，新题省略",
          "promptMarkdown": "查看当前用户的命令？",
          "normalization": "trim",
          "answer": "仅新增或更换答案时传入"
        }
      ]
    }
  ]
}
```

- `chapters` 数组顺序就是章节顺序；完整数组中省略的已有章节会被删除。
- 整个 `chapters` 字段省略表示不改章节。
- 章节的 `questions` 字段省略表示不改题目；空数组表示移除全部题目。
- `answer` 省略或空字符串保留已有答案；`clearAnswer: true` 明确清除。
- ID 必须属于当前模块/章节，不能引用其他模块的内容。

### 保存事务

1. 锁住模块行，比较 `lockVersion`。
2. 更新元信息；删除移除的章节；暂移位置后依序写入章节和题目，保留已有 ID 和答案。
3. 若模块已发布，校验全部章节和题目完整性。
4. 递增 `lockVersion`，记录审计，同一事务提交。任何失败都回滚。

读取管理详情、目录、正文采用一致性读，避免并发保存过程中拼接出不同时间点的内容。编辑器一次保存全部表单；保存成功后重新读取服务端数据。

## 学习接口

| 方法 | 路径 | 行为 |
| --- | --- | --- |
| GET | /api/categories | 分类清单 |
| GET | /api/modules | 已发布模块列表 |
| GET | /api/modules/:slug | `{ module, chapters }`，目录包含登录锁提示 |
| GET | /api/modules/:slug/chapters/:chapterSlug | `{ chapter, questions }`，服务端检查章节访问权限 |
| POST | /api/modules/:slug/chapters/:chapterSlug/attempt | 提交 `{ questionId, answer }`，返回 `{ correct, errorHint }` |

答题按当前题目判定。题目已删除或模块已下架时返回 404，学习者需要刷新页面；并不维持旧题目快照。当前答题仅校验答案，不代表已实现学习完成记录和顺序解锁。
