package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func runUpdate(args []string) int {
	fs := newFlagSet("update")
	apply := fs.Bool("apply", false, "run the update command immediately")
	versionFlag := fs.String("version", "", "install a specific version tag")
	codex := fs.Bool("codex", false, "update the Codex install")
	claude := fs.Bool("claude", false, "update the Claude Code install")
	all := fs.Bool("all", false, "update both Codex and Claude Code installs")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	detected := detectInstallTargets()
	targets := resolveInstallTargets(detected, *codex, *claude, *all)
	command, targetLabel := buildUpdateCommand(targets, strings.TrimSpace(*versionFlag))
	if !*apply {
		fmt.Fprintf(os.Stdout, "当前版本: %s\n", version)
		fmt.Fprintf(os.Stdout, "检测到的安装目标: %s\n", describeInstallTargets(detected))
		fmt.Fprintf(os.Stdout, "建议更新目标: %s\n", targetLabel)
		if !targets.Codex && !targets.Claude {
			fmt.Fprintln(os.Stdout, "没有检测到 skill 安装目录，默认会给出 Codex 更新命令。需要改成 Claude Code 可加 --claude。")
		}
		fmt.Fprintln(os.Stdout, "执行下面的命令即可更新：")
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintln(os.Stdout, command)
		if runtime.GOOS != "windows" {
			fmt.Fprintln(os.Stdout, "")
			fmt.Fprintln(os.Stdout, "如果想直接开始更新，也可以执行：")
			fmt.Fprintln(os.Stdout, "sshctl update --apply")
		}
		return 0
	}

	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "Windows 下暂不支持 `sshctl update --apply`。请复制下面的命令手动执行：")
		fmt.Fprintln(os.Stderr, command)
		return 1
	}

	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "更新失败: %v\n", err)
		return 1
	}
	return 0
}

type installTargets struct {
	Codex  bool
	Claude bool
}

func resolveInstallTargets(detected installTargets, codex, claude, all bool) installTargets {
	switch {
	case all || (codex && claude):
		return installTargets{Codex: true, Claude: true}
	case claude:
		return installTargets{Claude: true}
	case codex:
		return installTargets{Codex: true}
	default:
		return detected
	}
}

func detectInstallTargets() installTargets {
	home, _ := os.UserHomeDir()
	codexHome := os.Getenv("CODEX_HOME")
	if strings.TrimSpace(codexHome) == "" {
		codexHome = filepath.Join(home, ".codex")
	}
	claudeHome := os.Getenv("CLAUDE_HOME")
	if strings.TrimSpace(claudeHome) == "" {
		claudeHome = filepath.Join(home, ".claude")
	}

	targets := installTargets{}
	if statExists(filepath.Join(codexHome, "skills", "ssh-ops")) {
		targets.Codex = true
	}
	if statExists(filepath.Join(claudeHome, "skills", "ssh-ops")) {
		targets.Claude = true
	}
	return targets
}

func statExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func buildUpdateCommand(targets installTargets, versionTag string) (string, string) {
	mode := "--codex"
	label := "Codex（默认）"
	switch {
	case targets.Codex && targets.Claude:
		mode = "--all"
		label = "Codex + Claude Code"
	case targets.Claude:
		mode = "--claude"
		label = "Claude Code"
	case targets.Codex:
		mode = "--codex"
		label = "Codex"
	}

	if runtime.GOOS == "windows" {
		command := "Invoke-WebRequest https://raw.githubusercontent.com/HanBBQovo/ssh-ops/main/install/install.ps1 -OutFile install.ps1; .\\install.ps1"
		switch mode {
		case "--all":
			command += " -All"
		case "--claude":
			command += " -Claude"
		default:
			command += " -Codex"
		}
		if versionTag != "" {
			command += " -Version " + versionTag
		}
		return command, label
	}

	command := "curl -fsSL https://raw.githubusercontent.com/HanBBQovo/ssh-ops/main/install/install.sh | bash -s -- " + mode
	if versionTag != "" {
		command += " --version " + versionTag
	}
	return command, label
}

func describeInstallTargets(targets installTargets) string {
	switch {
	case targets.Codex && targets.Claude:
		return "Codex + Claude Code"
	case targets.Codex:
		return "Codex"
	case targets.Claude:
		return "Claude Code"
	default:
		return "未检测到"
	}
}
