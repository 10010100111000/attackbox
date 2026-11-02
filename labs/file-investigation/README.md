# Linux 文件调查靶机

一个 SSH 容器，三章八题：文件内容分析、隐藏文件查找、Nginx 访问日志分析。
只有静态教学文件，不运行 Nginx、不依赖数据库或外部服务。

- 学员账户：`student`，密码：`learn123`。
- SSH 端口：`22`；目录：`/home/student/lab`。
- 学员无 sudo，禁止 root SSH 登录；允许修改自己的练习文件。
- SSH 主机密钥在每个容器首次启动时生成，不烘焙到镜像中。
- `content/Linux 文件调查.md`：符合当前学习模块解析格式的三章八题，可放进私有内容仓库的学习目录同步。
- `instructor/answers.md`：教师答案与参考分析，仅供维护者使用。
- 镜像只复制 `student-files`、SSH 配置和入口脚本；课程、教师答案与验证脚本不进入镜像。

## 接入 AttackBox

1. 按下面的打包命令生成 ZIP（只含运行文件，保留隐藏素材），导入靶场模板。也可以先构建、推送镜像，再提供使用该镜像的 Compose 文件。
2. 入口服务选择 `investigation`，入口网络选择 `lab`，入口端口填写 `22`。
3. 在后台完成镜像准备／构建，确认准备成功后，再启用模板供课程关联。
4. 导入课程 MD 后，将模块的默认靶场设为此模板，三个章节继承默认环境。课程 MD 内无需写模板 UUID。
5. 学员启动独享靶机，在攻击沙箱 CLI 中执行 `ssh student@靶机IP`，连接页面显示的实际地址。

该 Compose 不映射宿主机端口，不挂载宿主机目录，也不申请 privileged。
Worker 管理独享网络及沙箱接入；三章使用同一套镜像，按现有环境关联与复用规则运行。
题目和日志使用固定答案。每个用户独立创建容器，可写层互不影响。
停止再启动会保留修改；释放后重新创建才恢复镜像中的初始文件。
公开练习密码仅用于受控教学网络，不要将 SSH 直接暴露到公网。

制作上传 ZIP 时，在本目录运行（包含隐藏素材，排除课程和教师文件）：

```bash
zip -r /tmp/file-investigation.zip Dockerfile compose.yml sshd_config entrypoint.sh .dockerignore student-files
```

部署包路径目前仅接受 ASCII 字符，课程中文文件名只放入内容仓库，不放入靶场 ZIP。

## 本地试用

```bash
docker build -t attackbox/file-investigation:1.0 .
docker run --rm -d --name file-investigation \
  -p 127.0.0.1:2222:22 attackbox/file-investigation:1.0
ssh -p 2222 student@127.0.0.1
```

本地示例只向回环地址映射端口；平台部署使用无 `ports` 的 compose.yml。
重新创建靶机后若 SSH 提示主机指纹变化，先确认容器确实由你重新创建，随后仅删除对应地址的旧记录：

```bash
ssh-keygen -R '[127.0.0.1]:2222'
```

验证素材和标准答案：

```bash
python3 instructor/verify.py
```
