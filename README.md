# ssh-ops

`ssh-ops` 是一个面向 Agent CLI 的开源远程运维工具集，采用 `Skill + 本地 CLI` 的形态，而不是 MCP Server。

它的目标很明确：

- 降低 token 开销：平时只暴露很小的 skill 元数据，真正需要时才加载完整说明。
- 保持执行稳定：所有实际操作都通过本地 `sshctl` 二进制完成，输出固定为 JSON，方便 agent 消费。
- 兼容主流 Agent CLI：同一套 skill 可以给 Codex CLI、Claude Code 或其他兼容 Agent Skills 标准的运行时使用。

## 为什么不用 MCP

这个项目是专门为了替代“SSH MCP Server”这种形态而写的。

对 SSH 这类本地可执行、动作边界清晰的能力来说，`skill + local CLI` 一般比 `MCP` 更合适：

- 不需要长驻 server 和额外 transport
- skill 本身上下文更轻
- CLI 的行为更稳定，也更容易做开源分发
- 对 Codex CLI、Claude Code 这类工具来说，安装和迁移成本更低

一句话说，这个项目不是“把 MCP 换个壳”，而是彻底改成 skill-first 的执行模型。

## 仓库里有什么

- `cmd/sshctl`
  `sshctl` 的命令行入口。
- `internal/sshops`
  SSH 连接、SFTP 传输、配置加载、策略控制等核心实现。
- `skills/ssh-ops`
  标准 skill 目录，包含 `SKILL.md`、包装脚本、参考文档和评测样例。
- `examples/config.example.yaml`
  配置模板。
- `install/install-codex.sh`
  安装到 Codex 的脚本。
- `install/install-claude.sh`
  安装到 Claude Code 的脚本。

## 核心能力

`sshctl` 当前提供这些子命令：

- `list-hosts`
  列出当前配置中的主机别名。
- `validate-config`
  校验配置文件、密钥路径、host key 模式和基础安全项。
- `exec`
  在远端主机执行命令。
- `upload`
  通过 SFTP 上传文件或目录。
- `download`
  通过 SFTP 下载文件或目录。
- `version`
  输出版本信息。

所有命令默认输出 JSON，适合 agent 直接解析。

## 安装

推荐直接使用一键安装脚本，不需要先 clone 仓库，也不需要本地有 Go。

### 依赖

如果你使用“一键安装脚本”，默认会从 GitHub Releases 下载预编译二进制，因此通常不需要本地 Go。

只有在这两种情况下才需要 Go：

- 你想从源码本地构建
- 你想运行 `install/install-codex.sh` 或 `install/install-claude.sh` 这类本地构建脚本

如果你确实要本地构建，再确认 Go：

```bash
go version
```

建议 Go `1.22+`。

### macOS / Linux 一键安装

安装到 Codex：

```bash
curl -fsSL https://raw.githubusercontent.com/HanBBQovo/ssh-ops/main/install/install.sh | bash -s -- --codex
```

安装到 Claude Code：

```bash
curl -fsSL https://raw.githubusercontent.com/HanBBQovo/ssh-ops/main/install/install.sh | bash -s -- --claude
```

同时安装到两者：

```bash
curl -fsSL https://raw.githubusercontent.com/HanBBQovo/ssh-ops/main/install/install.sh | bash -s -- --all
```

### Windows 一键安装

安装到 Codex：

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/HanBBQovo/ssh-ops/main/install/install.ps1 -OutFile install.ps1
.\install.ps1 -Codex
```

安装到 Claude Code：

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/HanBBQovo/ssh-ops/main/install/install.ps1 -OutFile install.ps1
.\install.ps1 -Claude
```

同时安装到两者：

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/HanBBQovo/ssh-ops/main/install/install.ps1 -OutFile install.ps1
.\install.ps1 -All
```

### 从源码仓库本地安装

在仓库根目录执行：

```bash
./install/install-codex.sh
```

默认会做两件事：

- 把 skill 安装到 `${CODEX_HOME:-$HOME/.codex}/skills/ssh-ops`
- 把 `sshctl` 安装到 `${SSH_OPS_BIN_DIR:-$HOME/.local/bin}/sshctl`

安装完成后，建议确认：

```bash
echo $PATH
which sshctl
```

如果 `sshctl` 不在 `PATH` 上，可以显式设置：

```bash
export SSH_OPS_CLI="$HOME/.local/bin/sshctl"
```

然后重启 Codex 或新开一个会话。

### 安装到 Claude Code

在仓库根目录执行：

```bash
./install/install-claude.sh
```

默认会做两件事：

- 把 skill 安装到 `${CLAUDE_HOME:-$HOME/.claude}/skills/ssh-ops`
- 把 `sshctl` 安装到 `${SSH_OPS_BIN_DIR:-$HOME/.local/bin}/sshctl`

同样建议确认 `sshctl` 在 `PATH` 上，或者设置 `SSH_OPS_CLI`。

## 配置

`sshctl` 会按下面顺序寻找配置文件：

1. `--config /path/to/config.yaml`
2. 环境变量 `SSH_OPS_CONFIG`
3. `~/.config/ssh-ops/config.yaml`

你可以直接从模板开始：

```bash
mkdir -p ~/.config/ssh-ops
cp ./examples/config.example.yaml ~/.config/ssh-ops/config.yaml
```

推荐做法：

- 优先用私钥，不要直接写明文密码
- 密码、私钥口令优先走环境变量
- 正式环境尽量使用 `known_hosts`
- 不要把真实配置文件提交到仓库

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

## 怎么用

你可以把它理解成两层：

1. `sshctl`
   真正执行 SSH/SFTP 操作的本地命令。
2. `ssh-ops skill`
   给 Agent CLI 用的说明层，告诉模型什么时候该调用哪些脚本。

### 方式一：直接用 CLI

这是最直接、也最容易调试的方式。

#### 1. 查看主机别名

```bash
sshctl list-hosts --pretty
```

#### 2. 校验配置

第一次使用前建议先跑：

```bash
sshctl validate-config --pretty
```

#### 3. 远程执行命令

```bash
sshctl exec --host prod --command "uname -a" --pretty
```

带工作目录和环境变量：

```bash
sshctl exec \
  --host prod \
  --workdir /srv/app \
  --env APP_ENV=prod \
  --command "git rev-parse HEAD" \
  --pretty
```

#### 4. 上传文件

```bash
sshctl upload \
  --host prod \
  --local ./dist/app.tar.gz \
  --remote /tmp/app.tar.gz \
  --pretty
```

#### 5. 下载文件

```bash
sshctl download \
  --host prod \
  --remote /var/log/app.log \
  --local ./tmp/app.log \
  --pretty
```

### 方式二：在 Codex CLI 里用 skill

正常情况下，不需要每次都显式说“用 ssh-ops”。

安装完 skill 并重启 Codex 之后，直接自然语言描述任务就可以，模型应该会在合适时自动命中这个 skill，例如：

```text
检查 prod 主机磁盘空间
```

```text
把这个发布包上传到 staging 服务器
```

```text
下载远端 nginx 配置文件
```

只有在下面这些情况，才建议显式写 `$ssh-ops`：

- 你想强制命中这个 skill
- 当前上下文里有多个 skill 可能冲突
- 你在调试 skill 是否触发

例如：

```text
用 $ssh-ops 帮我检查 prod 主机的磁盘使用情况
```

典型适用场景：

- “帮我查看生产机磁盘空间”
- “把这个构建产物传到 staging”
- “下载远端 nginx 配置文件”
- “列出我当前配置了哪些 SSH 主机”

### 方式三：在 Claude Code 里用 skill

Claude Code 也是一样，优先自然语言触发：

```text
检查 prod 主机上的 Docker 容器状态
```

```text
把这个文件上传到生产机的 /tmp 目录
```

如果你想强制使用 skill，也可以显式说：

```text
使用 ssh-ops 检查 prod 主机上的 Docker 容器状态
```

## Skill 目录说明

`skills/ssh-ops` 是这个仓库的 canonical skill：

- [SKILL.md](./skills/ssh-ops/SKILL.md)
  skill 主说明
- [skills/ssh-ops/agents/openai.yaml](./skills/ssh-ops/agents/openai.yaml)
  Codex 侧 UI 元数据
- `skills/ssh-ops/scripts/*.sh`
  skill 实际调用的包装脚本
- `skills/ssh-ops/references/*.md`
  按需加载的参考文档
- `skills/ssh-ops/evals/*.json`
  用于 forward-test 的样例

注意：`SKILL.md` 的 frontmatter 描述保留英文，是为了尽量保证跨运行时的 skill 触发质量；正文和仓库文档已经改成中文。

## 安全设计

项目默认是偏保守的：

- 默认阻止明显危险的命令模式
- 默认不覆盖已存在文件，除非显式加 `--overwrite`
- 默认走 `known_hosts`
- 默认限制命令输出大小，避免把大量日志直接塞进 agent 上下文

如果命令被策略拦截，正确做法是缩小命令范围，而不是绕过 denylist。

## 开发

### 本地测试

```bash
go test ./...
```

### 本地构建

```bash
go build -o ./bin/sshctl ./cmd/sshctl
```

### 校验 skill

```bash
make validate-skill
```

## 常见问题

### 1. `sshctl` 找不到

- 确认已经运行过安装脚本
- 确认 `${SSH_OPS_BIN_DIR:-$HOME/.local/bin}` 在 `PATH` 里
- 或手动设置 `SSH_OPS_CLI`

### 2. 配置文件找不到

先检查：

```bash
echo "$SSH_OPS_CONFIG"
ls ~/.config/ssh-ops/config.yaml
```

### 3. 校验失败

先执行：

```bash
sshctl validate-config --pretty
```

看返回 JSON 里的 `errors` 和 `warnings`。

### 4. host key 报错

如果配置是 `known_hosts`，请确认目标主机已经存在于对应的 `known_hosts` 文件中。

## 仓库结构

```text
cmd/sshctl/             CLI 入口
internal/sshops/        SSH/SFTP/配置/策略核心实现
examples/               配置示例
install/                Codex / Claude 安装脚本
skills/ssh-ops/         canonical skill 目录
```

## License

MIT，见 [`LICENSE`](./LICENSE)。
