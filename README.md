# ssh-ops

[![CI](https://github.com/HanBBQovo/ssh-ops/actions/workflows/ci.yml/badge.svg)](https://github.com/HanBBQovo/ssh-ops/actions/workflows/ci.yml)
[![Release](https://github.com/HanBBQovo/ssh-ops/actions/workflows/release.yml/badge.svg)](https://github.com/HanBBQovo/ssh-ops/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

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

`sshctl` 当前稳定提供四组能力：

### 1. 交互式添加

- `add`
  一步一步问你服务器信息，保存后还能顺手测试连接。

### 2. 人类能记住的日常命令

- `list`
  把你保存过的服务器列出来。
- `show`
  看某一台服务器的详情。
- `edit`
  不记得服务器别名也没关系，直接从列表里选一台再修改。
- `remove`
  从列表里选一台服务器删除。
- `test`
  一键测试能不能连通。
- `run`
  在服务器上执行命令；不记得别名时也可以先选列表，再输入命令。
- `update`
  查看或直接执行更新命令。

### 3. 进阶配置管理

- `config path`
  查看当前生效的配置路径，以及配置文件是否存在。
- `config init`
  初始化默认配置文件。
- `config show`
  以 JSON 查看当前配置；默认会对内联密码、私钥、口令做脱敏。
- `config add-host`
  新增一个主机条目；如果配置文件还不存在，会自动创建。
- `config set-host`
  创建或更新一个主机条目，适合补字段，例如补 `workdir`。
- `config remove-host`
  按 host id 删除条目。
- `config rename-host`
  重命名 host id 或显示名称。

### 4. 底层远端操作

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

底层命令默认输出 JSON，适合 agent 直接解析；`add/edit/remove/list/show/test/run` 这一组更短的人类命令默认输出人类可读文本。

## 最简单的用法

先别想 YAML，也别先记一堆 flags。

### 1. 第一次使用，直接交互式添加

```bash
sshctl add
```

它会一步一步问你：

- 服务器名字
- 地址或 IP
- 登录用户
- 端口
- 用私钥还是密码
- 要不要立即测试连接

### 2. 添加完之后，就记这三个命令

```bash
sshctl list
sshctl test prod
sshctl run prod "df -h"
```

### 3. 如果只是临时连一下，也可以不保存

```bash
sshctl run \
  --target root@192.168.1.9:22 \
  --password-env SSH_OPS_TEST_PASSWORD \
  --host-key-mode insecure_ignore \
  "df -h"
```

只有你需要批量调整、看底层细节、或者排障时，才需要去看 `sshctl config ...` 或 `sshctl host ...`。

其中 `target` 支持这种形式：

- `deploy@203.0.113.10`
- `deploy@203.0.113.10:22`
- `203.0.113.10:22`

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

## 更新

如果用户已经安装过 `sshctl`，后续更新直接执行：

```bash
sshctl update
```

它会直接执行更新。

如果你只是想先看看它准备执行什么命令，而不立刻更新：

```bash
sshctl update --check
```

如果你明确只想更新某个目标，也可以显式指定：

```bash
sshctl update --codex
sshctl update --claude
sshctl update --all
```

需要锁定某个版本时：

```bash
sshctl update --version v0.1.4
sshctl update --check --version v0.1.4
```

Windows 目前仍然会打印出更新命令，让你复制执行；macOS / Linux 会直接更新。

## 配置

### 推荐工作流

推荐顺序现在改成这样：

1. 新用户：先 `sshctl add`
2. 日常使用：`sshctl list` / `sshctl test` / `sshctl run`
3. 一次性任务：直接 `sshctl run --target ...`
4. 只有进阶场景再用 `sshctl host ...` 或 `sshctl config ...`

如果你只是想“把我的机器加进去”，现在最短路径就是：

```bash
sshctl add
```

保存完成后就直接：

```bash
sshctl list
sshctl test prod
sshctl run prod "df -h"
```

如果你根本不想先保存主机，直接：

```bash
sshctl run \
  --target root@192.168.1.9 \
  --password-env SSH_OPS_TEST_PASSWORD \
  --host-key-mode insecure_ignore \
  "uname -a"
```

如果你还没有配置文件，也没关系，`add` / `host add` / `config add-host` 都会自动创建默认配置文件。

### 更短的主机管理命令

#### 新增常用机器

```bash
sshctl add
```

#### 看所有机器

```bash
sshctl list
```

#### 看某一台机器

```bash
sshctl show prod
```

#### 改机器信息

```bash
sshctl edit
```

如果你已经记得别名，也可以直接：

```bash
sshctl edit prod
```

#### 测试连接

```bash
sshctl test prod
```

#### 执行命令

```bash
sshctl run prod "df -h"
```

#### 删除机器

```bash
sshctl remove
```

#### 更新工具

```bash
sshctl update
```

### 配置文件查找顺序

`sshctl` 会按下面顺序寻找配置文件：

1. `--config /path/to/config.yaml`
2. 环境变量 `SSH_OPS_CONFIG`
3. `~/.config/ssh-ops/config.yaml`

### 常用配置命令

下面这些更偏“进阶”和“排障”，不是新手第一入口。

#### 看路径

```bash
sshctl config path --pretty
```

#### 看当前配置

```bash
sshctl config show --pretty
```

如果你确实要检查内联秘密值，可以显式加：

```bash
sshctl config show --reveal-secrets --pretty
```

#### 删除主机

```bash
sshctl config remove-host --host prod --pretty
```

#### 重命名主机

```bash
sshctl config rename-host --host prod --new-id prod-gz --name "广州生产" --pretty
```

### 仍然可以手动编辑 YAML

如果你在调试底层配置，或者要做批量调整，仍然可以从模板开始：

```bash
mkdir -p ~/.config/ssh-ops
cp ./examples/config.example.yaml ~/.config/ssh-ops/config.yaml
```

推荐做法：

- 优先用私钥，不要直接写明文密码
- 密码、私钥口令优先走环境变量
- 正式环境尽量使用 `known_hosts`
- 不要把真实配置文件提交到仓库

### YAML 结构参考

下面的示例仍然有效，但应把它理解为底层格式参考，而不是首选的日常管理方式：

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

#### 1. 先交互式添加一台机器

```bash
sshctl add
```

#### 2. 看看你加了哪些机器

```bash
sshctl list
sshctl show prod
sshctl edit
```

#### 3. 测试一下能不能连

```bash
sshctl test prod
```

#### 4. 执行命令

```bash
sshctl run prod "uname -a"
```

如果你不记得服务器别名，也可以直接：

```bash
sshctl run
```

它会先让你从列表里选机器，再问你要执行什么命令。

#### 4.1 测试连接也可以直接选列表

```bash
sshctl test
```

如果你只记得“生产环境”这种显示名称，也可以在列表里按显示名称选择。

#### 4.2 修改或删除服务器

```bash
sshctl edit
sshctl remove
```

`edit` 和 `remove` 都支持：

- 直接传别名，例如 `sshctl edit prod`
- 不传参数后从列表里选
- 在列表里输入序号、别名或显示名称

#### 5. 一次性直连，不保存

```bash
sshctl run \
  --target root@192.168.1.9 \
  --password-env SSH_OPS_TEST_PASSWORD \
  --host-key-mode insecure_ignore \
  "df -h"
```

#### 6. 如果你更喜欢底层命令，也还保留着

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

#### 7. 上传文件

```bash
sshctl upload \
  --host prod \
  --local ./dist/app.tar.gz \
  --remote /tmp/app.tar.gz \
  --pretty
```

#### 8. 下载文件

```bash
sshctl download \
  --host prod \
  --remote /var/log/app.log \
  --local ./tmp/app.log \
  --pretty
```

#### 9. 更新本地安装

```bash
sshctl update
sshctl update --check
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

## 发布与开源

这个仓库以 MIT License 开源。

每次发布会在 GitHub Releases 中提供：

- macOS / Linux / Windows 的预编译 `sshctl` 压缩包
- `ssh-ops-skill.tar.gz` 与 `ssh-ops-skill.zip`
- 发布产物校验和文件，便于校验下载完整性

源码、安装脚本和 skill 目录都在仓库内，可以直接审查。

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

如果你只是想赶紧用，不一定要先管配置文件。可以直接：

```bash
sshctl add
```

或者：

```bash
sshctl run --target root@192.168.1.9 --password-env SSH_OPS_TEST_PASSWORD --host-key-mode insecure_ignore "uname -a"
```

如果你就是要查配置路径，再执行：

```bash
sshctl config path --pretty
```

如果配置文件还不存在，也可以初始化它：

```bash
sshctl config init --pretty
```

如果你只是想先把一台机器加进去，也可以直接跑 `add-host`，它会顺手创建默认配置。

再检查：

```bash
echo "$SSH_OPS_CONFIG"
ls ~/.config/ssh-ops/config.yaml
```

如果你不确定当前到底写了什么，继续执行：

```bash
sshctl config show --pretty
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
