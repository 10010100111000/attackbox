---
summary: 从第一条命令开始，理解 Shell、目录与文件权限。
description: 面向初次使用 Linux 的学习者。通过三个短章节，建立命令行操作的基本习惯，为后续安全实验做好准备。
category: linux
difficulty: beginner
access: authenticated
chapters:
  认识 Shell:
    access: public
    durationMinutes: 15
  目录与路径:
    durationMinutes: 15
  文件权限:
    durationMinutes: 20
learningOutcomes:
  - 使用命令查看当前用户和工作目录
  - 理解绝对路径与相对路径
  - 读懂 Linux 文件的基础权限
---

# 认识 Shell

Shell 接收你输入的命令，并将执行结果显示在终端中。你不需要记住所有命令，先学会确认自己所处的环境。

## 确认当前用户

输入以下命令，查看当前使用的账户：

```bash
whoami
```

终端可能返回 `learner`。这是账户名，不是需要继续输入的命令。

## 确认当前位置

```bash
pwd
```

输出 `/home/learner`，表示你当前位于这个目录。后面的相对路径操作都以它为起点。

> 阅读命令时，先分清“需要输入的内容”和“程序返回的结果”。

```quiz
prompt: 查看当前用户名应使用什么命令？
answer: whoami
normalization: trim_casefold
errorHint: 这条命令的意思是“我是谁”。
```

# 目录与路径

Linux 使用树状目录组织文件。路径用于描述文件或目录的位置。

## 绝对路径

以 `/` 开头的路径是绝对路径，不依赖你当前所在的目录：

```bash
cd /var/log
pwd
```

这里的 `pwd` 会输出 `/var/log`。

## 相对路径

`..` 表示上一级目录，`.` 表示当前目录：

```bash
cd ..
pwd
```

从 `/var/log` 执行后，你会来到 `/var`。

```quiz
prompt: 切换到上一级目录，需要输入什么命令？
answer: cd ..
normalization: trim
errorHint: 使用 cd，并用两个点表示上一级目录。
```

# 文件权限

Linux 的文件权限分别规定所有者、所属组和其他用户可以执行哪些操作。

## 读懂权限

执行 `ls -l` 后，你可能看到：

```text
-rw-r----- 1 learner learners 128 Oct 26 10:00 notes.txt
```

忽略开头表示文件类型的 `-`，剩下的九个字符分为三组：

| 范围 | 权限 | 含义 |
| --- | --- | --- |
| 所有者 | rw- | 可以读取和修改 |
| 所属组 | r-- | 可以读取 |
| 其他用户 | --- | 没有权限 |

其中 `r` 表示读取，`w` 表示写入，`x` 表示执行。

## 修改权限

```bash
chmod 640 notes.txt
```

这个设置让所有者可以读写、所属组可以读取，其他用户没有权限。理解权限后，再根据实际需要调整，不要习惯性地赋予所有权限。

```quiz
prompt: 在权限字符中，表示“执行”的字母是什么？
answer: x
normalization: trim_casefold
errorHint: 读取是 r，写入是 w。
```
