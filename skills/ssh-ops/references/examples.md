# 使用示例

## 当前可用的操作示例

这些示例对应当前已经稳定存在的脚本和命令。

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

## 配置管理示例

下面这些命令已经可用，优先通过它们管理配置，而不是让用户手改 YAML。

## 查看当前配置路径

```bash
scripts/ssh_config_path.sh --pretty
```

## 初始化默认配置

```bash
scripts/ssh_config_init.sh --pretty
```

## 查看当前配置

```bash
scripts/ssh_config_show.sh --pretty
```

## 新增一台机器

```bash
scripts/ssh_config_add_host.sh \
  --id prod \
  --target deploy@203.0.113.10:22 \
  --private-key-path ~/.ssh/id_ed25519 \
  --host-key-mode known_hosts \
  --pretty
```

## 给已有主机补充默认目录

```bash
scripts/ssh_config_set_host.sh \
  --id prod \
  --workdir /srv/app \
  --name "生产环境" \
  --pretty
```

## 删除主机

```bash
scripts/ssh_config_remove_host.sh --host prod --pretty
```

## 重命名主机

```bash
scripts/ssh_config_rename_host.sh \
  --host prod \
  --new-id prod-gz \
  --name "广州生产" \
  --pretty
```

适合这类意图的用户表达：

- “帮我初始化 ssh-ops 配置，不想自己创建 YAML”
- “看看 ssh-ops 现在到底读取的是哪个配置文件”
- “给我新增一个 `prod` 主机配置”
- “把 `staging-old` 改名成 `staging`”
- “把 `prod` 的默认工作目录更新一下”
- “删除不再使用的测试主机”

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
