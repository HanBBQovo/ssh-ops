---
name: ssh-ops
description: Execute shell commands on remote hosts over SSH, upload or download files over SFTP, and inspect configured SSH host aliases through a local `sshctl` CLI. Use when the task requires remote host operations, deployment checks, log inspection, or file transfer via SSH/SFTP. Do not use for local shell tasks, cloud-provider APIs, or interactive terminal sessions.
---

# SSH Ops

通过本 skill 提供的包装脚本调用本地 `sshctl`，不要自己重新发明一套 SSH 执行逻辑。优先选择最小动作，并保持输出简洁。

## 默认工作流

1. 先确认这个任务真的需要 SSH 或 SFTP。
2. 如果不确定主机别名，先运行 `scripts/ssh_list_hosts.sh`。
3. 如果配置可能有问题，先运行 `scripts/ssh_validate_config.sh`。
4. 根据任务类型只选一个入口：
   - `scripts/ssh_exec.sh`：远程执行命令
   - `scripts/ssh_upload.sh`：本地上传到远端
   - `scripts/ssh_download.sh`：从远端下载到本地
5. 最终答复里明确写出：
   - 使用了哪个 host id
   - 执行了什么命令或传输了什么路径
   - 是否成功
   - 是否存在后续风险

## 安全规则

- 优先做只读检查，再做写操作。
- 如果 `sshctl` 返回策略拒绝，不要尝试绕过 denylist。
- 除非用户明确要求，否则不要运行交互式、常驻型或后台型命令。
- 不要泄露配置里的密码、私钥内容、环境变量或远端敏感输出。
- 不要擅自关闭 host key 校验；只有本地配置已经这样写时才遵循该配置。

## 可用脚本

- `scripts/ssh_list_hosts.sh`：列出已配置主机
- `scripts/ssh_validate_config.sh`：校验配置是否可用
- `scripts/ssh_exec.sh`：执行远程命令
- `scripts/ssh_upload.sh`：上传文件或目录
- `scripts/ssh_download.sh`：下载文件或目录

这些脚本默认要求：

- `sshctl` 已经在 `PATH` 中
- 或已设置 `SSH_OPS_CLI=/path/to/sshctl`

## 参考文档索引

- `references/config.md`
  配置结构、路径规则、字段说明
- `references/safety.md`
  风险操作、策略模型、安全边界
- `references/examples.md`
  典型命令和触发示例
- `references/troubleshooting.md`
  无法连接、认证失败、找不到配置等问题

## 输出规则

- 命令输出尽量短，只保留当前任务需要的关键行。
- 如果 CLI 返回了 `truncated=true`，要明确告诉用户输出被截断。
- 最终答复里保留准确的 host id，方便用户复现。
