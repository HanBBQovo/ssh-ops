# 配置说明

## 先看推荐工作流

对大多数用户来说，推荐工作流现在是：

1. 先 `sshctl add`
2. 日常只记 `sshctl list`、`sshctl test`、`sshctl run`
3. 一次性任务再用 `sshctl run --target ...`
4. 只有进阶场景才去看 `sshctl host ...` 和 `sshctl config ...`

也就是说，配置文件现在应该是“后台细节”，不是“上手第一步”。

最短示例：

```bash
sshctl add
```

保存之后直接：

```bash
sshctl list
sshctl test prod
sshctl run prod "df -h"
```

## 当前已经可用的入口

### 更短的人类命令

- `sshctl add`
- `sshctl list`
- `sshctl show [id]`
- `sshctl edit [id]`
- `sshctl remove [id]`
- `sshctl test [id]`
- `sshctl run [id] "command"`
- `sshctl update`

### 次级入口

- `sshctl host ls`
- `sshctl host show [id]`
- `sshctl host add <id> <target>`
- `sshctl host rm <id>`
- `sshctl host rename <old> <new>`

- `sshctl config path`
  查看当前生效的配置路径。
- `sshctl config init`
  初始化配置文件。
- `sshctl config show`
  查看当前配置内容；默认脱敏输出内联秘密值。
- `sshctl config add-host`
  新增主机；若配置文件不存在会自动创建。
- `sshctl config set-host`
  创建或更新主机字段。
- `sshctl config remove-host`
  删除主机。
- `sshctl config rename-host`
  重命名主机 id 或显示名。

底层仍然使用 YAML 文件持久化配置，但日常增删改查不需要手改 YAML。

## 配置文件查找顺序

`sshctl` 会按下面顺序查找配置文件：

1. `--config /path/to/config.yaml`
2. 环境变量 `SSH_OPS_CONFIG`
3. `~/.config/ssh-ops/config.yaml`

## 常见场景

### 临时连一台机器

直接用 `--target`，不要先创建配置：

```bash
sshctl run --target root@192.168.1.9 --password-env SSH_OPS_TEST_PASSWORD --host-key-mode insecure_ignore "uname -a"
```

### 保存一台常用机器

优先直接跑 `add` 向导：

```bash
sshctl add
```

### 修改一台常用机器

如果你不记得别名，直接运行：

```bash
sshctl edit
```

它会先列出服务器，再让你按序号、别名或显示名称选择。

### 删除一台不用的机器

```bash
sshctl remove
```

如果你已经记得别名，也可以：

```bash
sshctl remove prod
```

### 看看目前保存了哪些机器

```bash
sshctl list
sshctl show prod
```

## 管理命令说明

### `sshctl config init`

目标：

- 初始化默认配置文件
- 创建配置目录
- 写入最小可用骨架

应解决的问题：

- 新用户第一次使用时不需要手动复制模板
- 用户能快速知道配置会写到哪里

### `sshctl config path`

目标：

- 直接输出当前生效的配置路径
- 帮助定位“到底用了哪个配置文件”

适用场景：

- 排查多个环境变量或多个配置文件冲突
- 调试 CI、本地 shell 或 agent 运行时的配置来源

### `sshctl config show`

目标：

- 查看当前配置
- 查看某个 host 条目
- 查看主机列表或有效配置快照
- 默认对 `password`、`private_key`、`passphrase` 做脱敏

适用场景：

- 确认 `prod` 指向哪台机器
- 排查 host key、user、默认工作目录是否写对

### `sshctl config add-host`

目标：

- 通过参数化方式新增一个主机条目
- 避免用户手工维护 YAML 缩进和字段结构
- 支持 `--target deploy@203.0.113.10:22` 这种快捷写法

建议用途：

- 首次录入 `prod`、`staging`、`test` 等环境
- 在 skill 或脚本化流程中追加主机配置

### `sshctl config set-host`

目标：

- 创建或更新某个 host
- 适合补一个字段，例如 `workdir`、`name`、`known_hosts_path`

当前边界：

- 适合“补充/覆盖字段”
- 暂不提供显式清空单个字段的专门参数；需要时可删除并重建，或手工编辑 YAML

### `sshctl config remove-host`

目标：

- 删除一个不再使用的 host 条目
- 避免 YAML 残留条目导致误用

### `sshctl config rename-host`

目标：

- 安全地重命名 host id
- 避免用户手动修改后遗漏引用关系或命名规范

## 推荐顺序

如果你就是想尽快把一台机器加进去，优先这样做：

```bash
sshctl add
sshctl list
sshctl test
sshctl run
```

如果是已有机器想补字段、改地址、换登录方式，优先这样做：

```bash
sshctl edit
```

只有你需要批量改字段、脚本化写配置、或者排查底层配置结构时，才去使用：

- `sshctl host ...`
- `sshctl config ...`
- 手动编辑 YAML

## 建议做法

- 把真实配置放在用户目录，不要放进仓库
- 优先使用私钥，不要把明文密码写进配置文件
- 私钥口令、密码优先走环境变量
- 生产环境尽量使用 `known_hosts`
- 修改配置后先运行 `sshctl validate-config --pretty`

## YAML 结构参考

YAML 仍然是底层格式，因此保留字段说明，供以下场景使用：

- 调试配置加载问题
- 手工修复异常条目
- 与未来 `sshctl config show` / `update` 的行为做比对

### 最小配置示例

```yaml
version: "1"

defaults:
  connect_timeout_sec: 10
  operation_timeout_sec: 120
  max_output_bytes: 1048576
  shell: "bash"

policy:
  allow_patterns: []
  deny_patterns: []

hosts:
  - id: "prod"
    name: "生产环境"
    address: "203.0.113.10"
    port: 22
    user: "deploy"
    auth:
      private_key_path: "~/.ssh/id_ed25519"
      passphrase_env: "SSH_OPS_PROD_KEY_PASSPHRASE"
    host_key:
      mode: "known_hosts"
      known_hosts_path: "~/.ssh/known_hosts"
    defaults:
      workdir: "/srv/app"
```

### 顶层字段

- `version`
  当前配置版本，现阶段使用 `"1"`。
- `defaults`
  全局默认行为。
- `policy`
  命令放行/拒绝规则。
- `hosts`
  主机列表。

### defaults

- `connect_timeout_sec`
  SSH 建连超时秒数。
- `operation_timeout_sec`
  单次操作超时秒数。
- `max_output_bytes`
  `exec` 时允许捕获的最大输出字节数。
- `shell`
  默认远端 shell，目前建议使用 `bash` 或 `sh`。

### policy

- `allow_patterns`
  可选。配置后，只有匹配到 allowlist 的命令才允许执行。
- `deny_patterns`
  可选。会叠加到默认 denylist 之后。

### hosts

每个 host 至少应包含：

- `id`
  稳定的命令行标识。建议只用小写字母、数字和 `-` / `_` / `.`。
- `name`
  面向人的显示名称，可选。
- `address`
  主机地址。
- `port`
  SSH 端口，默认 `22`。
- `user`
  登录用户。

### auth

至少提供一种认证方式：

- `auth.password`
- `auth.password_env`
- `auth.private_key`
- `auth.private_key_path`

如果私钥有口令，可以提供：

- `auth.passphrase`
- `auth.passphrase_env`

### host_key

- `host_key.mode`
  可选 `known_hosts` 或 `insecure_ignore`。
- `host_key.known_hosts_path`
  当 `mode=known_hosts` 时建议显式指定。

### host defaults

- `defaults.workdir`
  当前主机默认工作目录。
- `defaults.shell`
  当前主机默认 shell，会覆盖全局默认值。

## 推荐的 host id 命名方式

建议保持稳定、简短、可脚本化，例如：

- `prod`
- `staging`
- `hk-edge-01`
- `aliyun-gz`

不建议把中文、空格或说明性长句直接放进 `id`。
