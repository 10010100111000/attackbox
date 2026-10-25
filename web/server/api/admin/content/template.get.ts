import {defineEventHandler,setResponseHeaders} from 'h3'
import {requireAdmin} from '../../../utils/admin'
export default defineEventHandler(e=>{requireAdmin(e);setResponseHeaders(e,{'Content-Type':'text/markdown; charset=utf-8','Content-Disposition':"attachment; filename*=UTF-8''%E6%95%99%E7%A8%8B%E6%A8%A1%E6%9D%BF.md"});return `---
summary: 模块简介
description: 模块详细介绍
difficulty: beginner
access: authenticated
learningOutcomes:
  - 学习目标
chapters:
  第一章:
    access: public
    durationMinutes: 15
---

# 第一章

在这里编写教程。模块名取上传到仓库后的 MD 文件名。

\`\`\`quiz
prompt: 在这里填写问题
answer: "标准答案"
normalization: trim
\`\`\`

# 第二章

继续编写教程。
`})
