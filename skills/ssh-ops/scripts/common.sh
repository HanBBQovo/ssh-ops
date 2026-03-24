#!/usr/bin/env bash
set -euo pipefail

ssh_ops_cli() {
  if [[ -n "${SSH_OPS_CLI:-}" && -x "${SSH_OPS_CLI}" ]]; then
    printf '%s\n' "${SSH_OPS_CLI}"
    return 0
  fi

  if command -v sshctl >/dev/null 2>&1; then
    command -v sshctl
    return 0
  fi

  local here repo_root
  here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  repo_root="$(cd "${here}/../../.." && pwd)"
  if [[ -x "${repo_root}/bin/sshctl" ]]; then
    printf '%s\n' "${repo_root}/bin/sshctl"
    return 0
  fi

  echo "ssh-ops: could not find sshctl. Install it with install/install.sh, install/install.ps1, install/install-codex.sh, or install/install-claude.sh." >&2
  return 1
}
