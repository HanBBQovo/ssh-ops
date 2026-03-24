# 使用示例

## 先看有哪些主机

```bash
scripts/ssh_list_hosts.sh --pretty
```

## 先校验配置

```bash
scripts/ssh_validate_config.sh --pretty
```

## 查看远端系统信息

```bash
scripts/ssh_exec.sh --host prod --command "uname -a" --pretty
```

## 在指定目录执行命令

```bash
scripts/ssh_exec.sh \
  --host prod \
  --workdir /srv/app \
  --command "git rev-parse HEAD" \
  --pretty
```

## 下载日志

```bash
scripts/ssh_download.sh \
  --host prod \
  --remote /var/log/app.log \
  --local ./tmp/app.log \
  --pretty
```

## 上传构建产物

```bash
scripts/ssh_upload.sh \
  --host prod \
  --local ./dist/app.tar.gz \
  --remote /tmp/app.tar.gz \
  --pretty
```

## 适合触发这个 skill 的用户表达

- “帮我检查 prod 主机的磁盘使用情况”
- “把这个发布包传到 staging 服务器”
- “帮我下载远端 nginx 配置”
- “列出我现在配置的 SSH 主机别名”
- “登录生产机看一下当前服务目录的 git commit”

## 不适合触发这个 skill 的场景

- 纯本地 shell 操作
- 云厂商 API 管理任务
- 需要浏览器、控制台或交互式终端的任务
- 与 SSH / SFTP 无关的通用代码问题
