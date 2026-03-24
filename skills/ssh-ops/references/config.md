# 配置说明

## 配置文件查找顺序

`sshctl` 会按下面顺序查找配置文件：

1. `--config /path/to/config.yaml`
2. 环境变量 `SSH_OPS_CONFIG`
3. `~/.config/ssh-ops/config.yaml`

## 建议做法

- 把真实配置放在用户目录，不要放进仓库
- 优先使用私钥，不要把明文密码写进配置文件
- 私钥口令、密码优先走环境变量
- 生产环境尽量使用 `known_hosts`
- 修改配置后先运行 `sshctl validate-config --pretty`

## 最小配置示例

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

## 字段说明

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

## 修改配置后的标准动作

每次编辑配置后，建议立刻执行：

```bash
sshctl validate-config --pretty
```

如果失败，再根据报错修正路径、认证方式或 host key 配置。
