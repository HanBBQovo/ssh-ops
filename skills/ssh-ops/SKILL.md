---
name: ssh-ops
description: Execute shell commands on remote hosts over SSH, transfer files over SFTP, and manage saved host aliases through a local `sshctl` CLI. Use when the task requires one-off SSH access via a direct target like user@host, adding a reusable host alias, remote deployment checks, log inspection, or file transfer via SSH/SFTP. Do not use for local shell tasks, cloud-provider APIs, or interactive terminal sessions.
---

# SSH Ops

通过本 skill 提供的包装脚本调用本地 `sshctl`，不要自己重新发明一套 SSH 执行逻辑。优先选择最小动作，并保持输出简洁。

## 默认工作流

1. 先确认这个任务是真的在管理 `ssh-ops` 配置，或者需要 SSH / SFTP。
2. 如果用户已经给了 `user@host[:port]` 这类直接目标，优先直接调用 `scripts/ssh_exec.sh` / `scripts/ssh_upload.sh` / `scripts/ssh_download.sh` 并透传 `--target`，不要先要求用户建配置。
3. 如果用户想保存常用机器，优先使用 `scripts/ssh_host_*.sh`。
4. 如果不确定主机别名，先运行 `scripts/ssh_host_ls.sh` 或 `scripts/ssh_list_hosts.sh`。
5. 如果配置可能有问题，先运行 `scripts/ssh_validate_config.sh`。
6. 根据任务类型只选一个入口：
   - `scripts/ssh_host_ls.sh`：列出已保存主机
   - `scripts/ssh_host_show.sh`：查看保存的主机
   - `scripts/ssh_host_add.sh`：用最短命令新增主机
   - `scripts/ssh_host_rm.sh`：删除主机
   - `scripts/ssh_host_rename.sh`：重命名主机
   - `scripts/ssh_config_path.sh`：查看配置路径
   - `scripts/ssh_config_init.sh`：初始化默认配置
   - `scripts/ssh_config_show.sh`：查看当前配置
   - `scripts/ssh_config_add_host.sh`：新增主机
   - `scripts/ssh_config_set_host.sh`：创建或更新主机字段
   - `scripts/ssh_config_remove_host.sh`：删除主机
   - `scripts/ssh_config_rename_host.sh`：重命名主机
   - `scripts/ssh_exec.sh`：远程执行命令
   - `scripts/ssh_upload.sh`：本地上传到远端
   - `scripts/ssh_download.sh`：从远端下载到本地
6. 最终答复里明确写出：
   - 使用了哪个 host id
   - 执行了什么命令或传输了什么路径
   - 是否成功
   - 是否存在后续风险

## 安全规则

- 优先做只读检查，再做写操作。
- 配置管理默认走最小变更，优先新增或补字段，不要顺手改一堆无关 host。
- 如果 `sshctl` 返回策略拒绝，不要尝试绕过 denylist。
- 除非用户明确要求，否则不要运行交互式、常驻型或后台型命令。
- 不要泄露配置里的密码、私钥内容、环境变量或远端敏感输出。
- 不要擅自关闭 host key 校验；只有本地配置已经这样写时才遵循该配置。

## 可用脚本

- `scripts/ssh_list_hosts.sh`：列出已配置主机
- `scripts/ssh_host_ls.sh`：列出已保存主机
- `scripts/ssh_host_show.sh`：查看已保存主机
- `scripts/ssh_host_add.sh`：用更短命令新增主机
- `scripts/ssh_host_rm.sh`：删除主机
- `scripts/ssh_host_rename.sh`：重命名主机
- `scripts/ssh_validate_config.sh`：校验配置是否可用
- `scripts/ssh_config_path.sh`：查看配置路径
- `scripts/ssh_config_init.sh`：初始化默认配置
- `scripts/ssh_config_show.sh`：查看当前配置
- `scripts/ssh_config_add_host.sh`：新增主机
- `scripts/ssh_config_set_host.sh`：创建或更新主机
- `scripts/ssh_config_remove_host.sh`：删除主机
- `scripts/ssh_config_rename_host.sh`：重命名主机
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
