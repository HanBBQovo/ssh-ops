# 故障排查

## `sshctl` 找不到

优先检查下面几项：

1. 是否已经执行过安装脚本
2. `${SSH_OPS_BIN_DIR:-$HOME/.local/bin}` 是否在 `PATH` 中
3. 是否设置了 `SSH_OPS_CLI`

可执行的排查命令：

```bash
which sshctl
echo "$SSH_OPS_CLI"
```

如果还没有构建，可以手动执行：

```bash
go build -o ./bin/sshctl ./cmd/sshctl
```

## 找不到配置文件

先直接执行：

```bash
sshctl config path --pretty
```

如果配置文件还不存在，直接初始化：

```bash
sshctl config init --pretty
```

如果你想一步到位把机器加进去，也可以直接执行 `sshctl config add-host ...`，它会自动创建默认配置文件。

再按现有规则排查：

```bash
echo "$SSH_OPS_CONFIG"
ls ~/.config/ssh-ops/config.yaml
```

如果你仍然想从模板开始，也可以复制：

```bash
mkdir -p ~/.config/ssh-ops
cp ./examples/config.example.yaml ~/.config/ssh-ops/config.yaml
```

## 不确定当前配置里到底有什么

先直接执行：

```bash
sshctl config show --pretty
```

如果还需要继续排查：

- 运行 `sshctl list-hosts --pretty`
- 直接查看当前配置文件
- 再执行 `sshctl validate-config --pretty`

## `validate-config` 失败

建议直接运行：

```bash
sshctl validate-config --pretty
```

重点看这些字段：

- `errors`
- `warnings`
- `hosts[*].errors`
- `hosts[*].warnings`

常见原因：

- 私钥路径不存在
- `password_env` / `passphrase_env` 没有导出
- `known_hosts_path` 不存在
- `host_key.mode` 写错
- host id 不符合规则

## Host key 相关错误

如果使用的是：

```yaml
host_key:
  mode: "known_hosts"
```

那么要确认目标主机已经存在于对应的 `known_hosts` 文件里。

常见处理方式：

- 用系统自带 `ssh` 先连一次
- 按你的正常 SSH 管理流程更新 `known_hosts`

不要为了省事直接把生产环境改成 `insecure_ignore`。

## 认证失败

优先检查：

- `user`
- `port`
- `address`
- `auth.private_key_path`
- `auth.password_env`
- `auth.passphrase_env`

如果用私钥：

- 确认文件存在
- 确认当前用户可读
- 确认口令已正确导出

如果用密码：

- 确认环境变量名正确
- 确认变量已经导出到当前 shell

如果只是想修正一个字段，优先使用 `sshctl config set-host`。

## 命令被策略拒绝

如果 `sshctl exec` 返回 `policy_denied`：

- 说明命令被 denylist 或 allowlist 拦住了
- 正确做法是缩小命令范围
- 不要尝试拼接、变形或绕过策略

通常应该先改成只读检查命令，再让用户确认下一步。

## 主机别名改错或想删除旧条目

优先使用配置管理命令完成：

- `sshctl config rename-host`
- `sshctl config remove-host`

只有在命令不覆盖你的场景时，才考虑手动修改 YAML，并在修改后立刻执行：

```bash
sshctl validate-config --pretty
```
