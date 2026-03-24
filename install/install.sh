#!/usr/bin/env bash
set -euo pipefail

REPO_OWNER="${SSH_OPS_REPO_OWNER:-HanBBQovo}"
REPO_NAME="${SSH_OPS_REPO_NAME:-ssh-ops}"
VERSION="${SSH_OPS_VERSION:-}"
BIN_DIR="${SSH_OPS_BIN_DIR:-$HOME/.local/bin}"
CONFIG_DIR="${SSH_OPS_CONFIG_DIR:-$HOME/.config/ssh-ops}"
CODEX_SKILLS_DIR="${CODEX_HOME:-$HOME/.codex}/skills"
CLAUDE_SKILLS_DIR="${CLAUDE_HOME:-$HOME/.claude}/skills"

INSTALL_CODEX=false
INSTALL_CLAUDE=false
LOCAL_SOURCE=false
LOCAL_BUILD=false

usage() {
  cat <<'EOF'
ssh-ops 一键安装脚本（macOS / Linux）

用法：
  install.sh --codex
  install.sh --claude
  install.sh --all
  install.sh --codex --version v0.1.0
  install.sh --codex --local-build

参数：
  --codex         安装到 Codex
  --claude        安装到 Claude Code
  --all           同时安装到 Codex 和 Claude Code
  --version TAG   指定版本，例如 v0.1.0；默认安装最新 release
  --local-source  从当前仓库复制 skill 文件
  --local-build   从当前仓库构建 sshctl，并从当前仓库复制 skill 文件
  -h, --help      显示帮助
EOF
}

log() {
  printf '[ssh-ops] %s\n' "$*"
}

fail() {
  printf '[ssh-ops] %s\n' "$*" >&2
  exit 1
}

have_cmd() {
  command -v "$1" >/dev/null 2>&1
}

fetch() {
  local url="$1"
  local output="$2"
  if have_cmd curl; then
    curl -fsSL "$url" -o "$output"
    return 0
  fi
  if have_cmd wget; then
    wget -qO "$output" "$url"
    return 0
  fi
  fail "需要 curl 或 wget 才能下载安装包"
}

fetch_text() {
  local url="$1"
  if have_cmd curl; then
    curl -fsSL "$url"
    return 0
  fi
  if have_cmd wget; then
    wget -qO- "$url"
    return 0
  fi
  fail "需要 curl 或 wget 才能获取版本信息"
}

detect_os() {
  case "$(uname -s)" in
    Darwin) printf 'darwin' ;;
    Linux) printf 'linux' ;;
    *) fail "当前系统不受 install.sh 支持，请在 Windows 上使用 install.ps1" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) fail "当前架构不受支持：$(uname -m)" ;;
  esac
}

resolve_version() {
  if [[ "$LOCAL_SOURCE" == true && -z "$VERSION" ]]; then
    VERSION="dev"
    printf '%s\n' "$VERSION"
    return 0
  fi
  if [[ -n "$VERSION" ]]; then
    printf '%s\n' "$VERSION"
    return 0
  fi
  local api_url="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"
  local response
  response="$(fetch_text "$api_url")"
  VERSION="$(printf '%s\n' "$response" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  [[ -n "$VERSION" ]] || fail "无法解析最新 release 版本，请显式传入 --version"
  printf '%s\n' "$VERSION"
}

extract_tarball() {
  local archive="$1"
  local target_dir="$2"
  mkdir -p "$target_dir"
  tar -xzf "$archive" -C "$target_dir"
}

copy_skill_to() {
  local source_dir="$1"
  local target_root="$2"
  local target_dir="${target_root}/ssh-ops"
  mkdir -p "$target_root"
  rm -rf "$target_dir"
  cp -R "$source_dir" "$target_dir"
  chmod +x "$target_dir"/scripts/*.sh
}

copy_example_config() {
  local example_file="$1"
  mkdir -p "$CONFIG_DIR"
  if [[ ! -f "${CONFIG_DIR}/config.yaml" ]]; then
    cp "$example_file" "${CONFIG_DIR}/config.yaml"
    log "已创建默认配置：${CONFIG_DIR}/config.yaml"
  else
    log "已保留现有配置：${CONFIG_DIR}/config.yaml"
  fi
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
LOCAL_SKILL_DIR="${REPO_ROOT}/skills/ssh-ops"
LOCAL_CONFIG_EXAMPLE="${REPO_ROOT}/examples/config.example.yaml"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --codex)
      INSTALL_CODEX=true
      ;;
    --claude)
      INSTALL_CLAUDE=true
      ;;
    --all)
      INSTALL_CODEX=true
      INSTALL_CLAUDE=true
      ;;
    --version)
      shift
      [[ $# -gt 0 ]] || fail "--version 需要一个值"
      VERSION="$1"
      ;;
    --local-source)
      LOCAL_SOURCE=true
      ;;
    --local-build)
      LOCAL_SOURCE=true
      LOCAL_BUILD=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "未知参数：$1"
      ;;
  esac
  shift
done

if [[ "$INSTALL_CODEX" == false && "$INSTALL_CLAUDE" == false ]]; then
  INSTALL_CODEX=true
fi

if [[ "$LOCAL_SOURCE" == true && ! -d "$LOCAL_SKILL_DIR" ]]; then
  fail "--local-source/--local-build 需要在仓库目录中运行"
fi

OS="$(detect_os)"
ARCH="$(detect_arch)"
VERSION="$(resolve_version)"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

BIN_TARGET="${BIN_DIR}/sshctl"
SKILL_SOURCE_DIR=""
CONFIG_EXAMPLE_FILE=""

if [[ "$LOCAL_BUILD" == true ]]; then
  have_cmd go || fail "--local-build 需要本地安装 Go"
  mkdir -p "$BIN_DIR"
  (
    cd "$REPO_ROOT"
    go build -ldflags "-X main.version=${VERSION}" -o "$BIN_TARGET" ./cmd/sshctl
  )
  SKILL_SOURCE_DIR="$LOCAL_SKILL_DIR"
  CONFIG_EXAMPLE_FILE="$LOCAL_CONFIG_EXAMPLE"
else
  mkdir -p "$BIN_DIR"
  BINARY_ASSET="ssh-ops_${OS}_${ARCH}.tar.gz"
  BINARY_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/${BINARY_ASSET}"
  BINARY_ARCHIVE="${TMP_DIR}/${BINARY_ASSET}"
  fetch "$BINARY_URL" "$BINARY_ARCHIVE"
  extract_tarball "$BINARY_ARCHIVE" "$TMP_DIR/bin"
  cp "${TMP_DIR}/bin/sshctl" "$BIN_TARGET"
  chmod +x "$BIN_TARGET"

  if [[ "$LOCAL_SOURCE" == true ]]; then
    SKILL_SOURCE_DIR="$LOCAL_SKILL_DIR"
    CONFIG_EXAMPLE_FILE="$LOCAL_CONFIG_EXAMPLE"
  else
    SKILL_ASSET="ssh-ops-skill.tar.gz"
    SKILL_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/${SKILL_ASSET}"
    SKILL_ARCHIVE="${TMP_DIR}/${SKILL_ASSET}"
    fetch "$SKILL_URL" "$SKILL_ARCHIVE"
    extract_tarball "$SKILL_ARCHIVE" "$TMP_DIR/skill"
    SKILL_SOURCE_DIR="${TMP_DIR}/skill/ssh-ops-skill/skills/ssh-ops"
    CONFIG_EXAMPLE_FILE="${TMP_DIR}/skill/ssh-ops-skill/examples/config.example.yaml"
  fi
fi

[[ -x "$BIN_TARGET" ]] || fail "sshctl 安装失败"
[[ -d "$SKILL_SOURCE_DIR" ]] || fail "skill 文件不存在"
[[ -f "$CONFIG_EXAMPLE_FILE" ]] || fail "示例配置不存在"

if [[ "$INSTALL_CODEX" == true ]]; then
  copy_skill_to "$SKILL_SOURCE_DIR" "$CODEX_SKILLS_DIR"
  log "已安装到 Codex：${CODEX_SKILLS_DIR}/ssh-ops"
fi

if [[ "$INSTALL_CLAUDE" == true ]]; then
  copy_skill_to "$SKILL_SOURCE_DIR" "$CLAUDE_SKILLS_DIR"
  log "已安装到 Claude Code：${CLAUDE_SKILLS_DIR}/ssh-ops"
fi

copy_example_config "$CONFIG_EXAMPLE_FILE"

cat <<EOF

安装完成

- 版本：${VERSION}
- 二进制：${BIN_TARGET}
- 配置目录：${CONFIG_DIR}

下一步：
1. 确保 ${BIN_DIR} 在 PATH 中；如果没有，请手动加入。
2. 编辑 ${CONFIG_DIR}/config.yaml
3. 运行：sshctl validate-config --pretty
4. 重启 Codex / Claude Code，或开启一个新会话

EOF
