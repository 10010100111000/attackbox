---
summary: 从导出记录、隐藏文件和 Nginx 访问日志中寻找证据，完成八道基础调查题。
category: Linux 基础
difficulty: beginner
access: authenticated
learningOutcomes:
  - 根据条件筛选文本与 CSV 记录，区分配置文件中的不同环境。
  - 查找隐藏目录中的文件，准确报告相对路径。
  - 根据时间、来源地址、请求路径及响应状态分析访问日志。
chapters:
  文件内容分析:
    durationMinutes: 10
  隐藏文件调查:
    durationMinutes: 10
  Nginx 日志分析:
    durationMinutes: 15
---

# 文件内容分析

## 进入调查环境

启动本章关联的实验靶机，在平台 CLI 中通过 SSH 登录：

```bash
ssh student@靶机IP
```

将“靶机IP”替换为页面展示的实际地址。用户名为 `student`，密码为 `learn123`，SSH 端口为 `22`。
首次连接确认目标地址后接受主机指纹。镜像重新创建后，主机指纹也会变化。

本章的调查目录为 `/home/student/lab/files`。这里有一份交接说明、一份任务记录和一份配置文件。
你可以使用 `pwd`、`ls`、`cd`、`cat`、`less`、`grep`、`awk` 等工具完成调查。

## 调查任务

每条 CSV 数据行代表一个任务，首行为表头。`bytes` 是十进制字节数。
只有 `status` 为 `failed` 的任务计入失败；`pending` 不计入。
请根据文件内容作答，无需修改文件。

```quiz
prompt: jobs.csv 中共有多少个失败任务？只提交十进制整数。
answer: "3"
normalization: trim
errorHint: 检查 status 列；不要把表头、pending 或包含其他状态的记录计入。
```

```quiz
prompt: 在所有失败任务中，bytes 最大的任务，其 job_id 是什么？提交完整编号，例如 JOB-0001。
answer: JOB-1008
normalization: trim_casefold
errorHint: 先筛选失败记录，再按 bytes 的数值比较；成功任务不参与比较。
```

```quiz
prompt: settings.ini 的 production 环境保留数据多少天？只提交十进制整数。
answer: "14"
normalization: trim
errorHint: 同一个配置项出现在多个环境中，注意所属的配置段。
```

# 隐藏文件调查

## 调查范围

调查目录为 `/home/student/lab/hidden`，包括它的所有隐藏子目录。
其中有一个名为 `.session-note` 的调查记录文件。普通的 `ls` 默认不展示以点开头的名称。

你可以使用 `ls` 的相关选项或 `find`。不要只搜索当前目录，也不需要搜索整个系统。

```quiz
prompt: 找出 .session-note 相对于 /home/student/lab/hidden 的路径。保留名称中的点，使用 / 分隔，不要添加前导 ./ 或完整绝对路径。
answer: .cache/.session-note
normalization: trim
errorHint: 文件位于隐藏子目录中，答案应同时包含目录名与文件名。
```

```quiz
prompt: 读取该文件，提交 case_code 字段的完整值。
answer: CASE-HIDDEN-4821
normalization: trim_casefold
errorHint: 提交等号右侧的内容，不要包含字段名。
```

# Nginx 日志分析

## 日志说明

日志位于 `/home/student/lab/logs/access.log`，是固定的 Nginx combined 格式教学样本。
不需要启动 Web 服务或发送网络请求，所有答案均在这一个文件中。

一行的主要结构为：

```text
来源IP - - [时间 时区] "方法 路径 协议" 状态码 响应字节数 "来源页面" "客户端标识"
```

本章只分析 **2026-10-20，UTC 10:00:00（包含）至 10:10:00（不包含）** 的记录。
日志中的时区为 `+0000`，不需要换算时区。

本题将同时满足以下条件的请求定义为一次“登录失败”：

- 请求方法为 `POST`。
- 请求路径恰好是 `/admin/login`。
- 状态码为 `401` 或 `403`。

其他路径、其他状态码及时间范围之外的请求都不计入。

```quiz
prompt: 在指定时间范围内，登录失败次数最多的来源 IPv4 地址是什么？只提交 IP。
answer: 203.0.113.77
normalization: trim
errorHint: 同时限定方法、路径、状态码和时间，再按日志首列分组计数。
```

```quiz
prompt: 上一题的来源 IP，在指定范围内共有多少次符合定义的登录失败？只提交十进制整数。
answer: "7"
normalization: trim
errorHint: 403 也算失败；10:10:00 的记录不在本题范围内。
```

```quiz
prompt: 同一来源 IP 在上述时间范围内，通过 GET 成功下载（状态码 200）的 /backup/ 下 .tar.gz 文件，其完整请求路径是什么？只提交以 / 开头的路径。
answer: /backup/site-2026-10-20.tar.gz
normalization: trim
errorHint: 排除状态码 404 的探测请求，以及时间范围外的下载记录。
```
