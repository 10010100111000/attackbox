# 教师答案与参考分析

题目是固定数据分析，无动态 Flag；平台复用现有章节测验判题。
此文件不进入 Docker 构建上下文，课程 MD 中的 quiz 答案也不复制进镜像。

| 章 | 题目 | 标准答案 |
| --- | --- | --- |
| 文件内容分析 | 失败任务数量 | `3` |
| 文件内容分析 | 最大失败任务 | `JOB-1008` |
| 文件内容分析 | production 保留天数 | `14` |
| 隐藏文件调查 | 相对路径 | `.cache/.session-note` |
| 隐藏文件调查 | case_code | `CASE-HIDDEN-4821` |
| Nginx 日志分析 | 最多失败来源 | `203.0.113.77` |
| Nginx 日志分析 | 失败次数 | `7` |
| Nginx 日志分析 | 成功下载路径 | `/backup/site-2026-10-20.tar.gz` |

参考分析命令（不直接放在学员题干中）：

```bash
cd /home/student/lab/files
awk -F, 'NR > 1 && $3 == "failed" { count++ } END { print count }' jobs.csv
awk -F, 'NR > 1 && $3 == "failed" && $4+0 > max { max=$4+0; id=$1 } END { print id }' jobs.csv
sed -n '/^\[production\]/,/^\[/p' settings.ini
find /home/student/lab/hidden -type f -name '.session-note'
cat /home/student/lab/hidden/.cache/.session-note
```

日志调查可先用 `less` 查看，再用 `awk` 分组统计：

```bash
cd /home/student/lab/logs
awk '$4 >= "[20/Oct/2026:10:00:00" && $4 < "[20/Oct/2026:10:10:00" && $6 == "\"POST" && $7 == "/admin/login" && ($9 == 401 || $9 == 403) { count[$1]++ } END { for (ip in count) print count[ip], ip }' access.log | sort -nr
awk '$4 >= "[20/Oct/2026:10:00:00" && $4 < "[20/Oct/2026:10:10:00" && $1 == "203.0.113.77" && $6 == "\"GET" && $9 == 200 && $7 ~ /^\/backup\/.*\.tar\.gz$/ { print $7 }' access.log
```

生产日志跨日期分析应解析完整时间戳；上面的字符串比较只用于本题固定日期、固定时区的样本。
题目描述中的时间窗口与过滤条件是判题定义，不把这些线索隐瞒给初学者。
