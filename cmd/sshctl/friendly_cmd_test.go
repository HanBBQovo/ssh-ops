package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hanhan/ssh-ops/internal/sshops"
)

func TestRunAddInteractiveWizard(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	input := strings.Join([]string{
		"prod",
		"生产环境",
		"203.0.113.10",
		"deploy",
		"",
		"1",
		"~/.ssh/id_ed25519",
		"",
		"2",
		"/srv/app",
		"n",
		"",
	}, "\n")

	exitCode, stdout, stderr := captureIO(t, input, func() int {
		return run([]string{"add", "--config", configPath})
	})
	if exitCode != 0 {
		t.Fatalf("add exit code = %d, stdout = %s, stderr = %s", exitCode, stdout, stderr)
	}

	cfg, err := sshops.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}
	if len(cfg.Hosts) != 1 {
		t.Fatalf("expected one host, got %d", len(cfg.Hosts))
	}
	if cfg.Hosts[0].ID != "prod" || cfg.Hosts[0].Defaults.Workdir != "/srv/app" {
		t.Fatalf("unexpected host payload: %#v", cfg.Hosts[0])
	}
}

func TestRunHumanCommandsWithDryRun(t *testing.T) {
	exitCode, stdout, stderr := captureIO(t, "", func() int {
		return run([]string{
			"run",
			"--target", "root@192.168.1.9:22",
			"--password", "secret",
			"--host-key-mode", "insecure_ignore",
			"--dry-run",
			"df -h",
		})
	})
	if exitCode != 0 {
		t.Fatalf("run exit code = %d, stdout = %s, stderr = %s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "将执行") {
		t.Fatalf("expected dry-run output, got %q", stdout)
	}

	exitCode, stdout, stderr = captureIO(t, "", func() int {
		return run([]string{
			"test",
			"--target", "root@192.168.1.9:22",
			"--password", "secret",
			"--host-key-mode", "insecure_ignore",
			"--dry-run",
		})
	})
	if exitCode != 0 {
		t.Fatalf("test exit code = %d, stdout = %s, stderr = %s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "已解析目标") {
		t.Fatalf("expected test dry-run output, got %q", stdout)
	}
}

func TestRunEditAndRemoveInteractive(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := sshops.DefaultConfig()
	cfg.Hosts = []sshops.HostConfig{
		{
			ID:      "prod",
			Name:    "生产环境",
			Address: "203.0.113.10",
			Port:    22,
			User:    "deploy",
			Auth: sshops.AuthConfig{
				PrivateKeyPath: "~/.ssh/id_ed25519",
			},
			HostKey: sshops.HostKeyConfig{
				Mode: "insecure_ignore",
			},
			Defaults: sshops.HostDefaults{
				Workdir: "/srv/app",
			},
		},
	}
	if err := sshops.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	editInput := strings.Join([]string{
		"prod-gz",
		"",
		"",
		"",
		"",
		"1",
		"",
		"",
		"2",
		"/srv/api",
		"",
		"n",
		"",
	}, "\n")
	exitCode, stdout, stderr := captureIO(t, editInput, func() int {
		return run([]string{"edit", "--config", configPath, "prod"})
	})
	if exitCode != 0 {
		t.Fatalf("edit exit code = %d, stdout = %s, stderr = %s", exitCode, stdout, stderr)
	}

	loaded, err := sshops.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(loaded.Hosts) != 1 || loaded.Hosts[0].ID != "prod-gz" || loaded.Hosts[0].Defaults.Workdir != "/srv/api" {
		t.Fatalf("unexpected edited config: %#v", loaded.Hosts)
	}

	exitCode, stdout, stderr = captureIO(t, "y\n", func() int {
		return run([]string{"remove", "--config", configPath})
	})
	if exitCode != 0 {
		t.Fatalf("remove exit code = %d, stdout = %s, stderr = %s", exitCode, stdout, stderr)
	}
	loaded, err = sshops.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() after remove error = %v", err)
	}
	if len(loaded.Hosts) != 0 {
		t.Fatalf("expected no hosts after remove, got %#v", loaded.Hosts)
	}
}

func TestChooseHostMatchesDisplayName(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("生产环境\n"))
	var out bytes.Buffer

	host, err := chooseHost(reader, &out, []sshops.HostConfig{
		{ID: "staging", Name: "预发环境", Address: "203.0.113.11", Port: 22, User: "deploy"},
		{ID: "prod", Name: "生产环境", Address: "203.0.113.10", Port: 22, User: "deploy"},
	}, "请选择服务器")
	if err != nil {
		t.Fatalf("chooseHost() error = %v", err)
	}
	if host.ID != "prod" {
		t.Fatalf("expected prod, got %#v", host)
	}
}

func TestRunAndTestInteractiveSelection(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := sshops.DefaultConfig()
	cfg.Hosts = []sshops.HostConfig{
		{
			ID:      "prod",
			Name:    "生产环境",
			Address: "203.0.113.10",
			Port:    22,
			User:    "deploy",
			Auth: sshops.AuthConfig{
				PrivateKeyPath: "~/.ssh/id_ed25519",
			},
			HostKey: sshops.HostKeyConfig{
				Mode: "insecure_ignore",
			},
		},
	}
	if err := sshops.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	exitCode, stdout, stderr := captureIO(t, "\n", func() int {
		return run([]string{"test", "--config", configPath, "--dry-run"})
	})
	if exitCode != 0 {
		t.Fatalf("test exit code = %d, stdout = %s, stderr = %s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "已解析目标") {
		t.Fatalf("expected interactive dry-run test output, got %q", stdout)
	}

	exitCode, stdout, stderr = captureIO(t, "df -h\n", func() int {
		return run([]string{"run", "--config", configPath, "--dry-run"})
	})
	if exitCode != 0 {
		t.Fatalf("run exit code = %d, stdout = %s, stderr = %s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "将执行") {
		t.Fatalf("expected interactive dry-run run output, got %q", stdout)
	}
}

func TestRunEditRejectsDuplicateHostID(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := sshops.DefaultConfig()
	cfg.Hosts = []sshops.HostConfig{
		{
			ID:      "prod",
			Name:    "生产环境",
			Address: "203.0.113.10",
			Port:    22,
			User:    "deploy",
			Auth: sshops.AuthConfig{
				PrivateKeyPath: "~/.ssh/id_ed25519",
			},
			HostKey: sshops.HostKeyConfig{
				Mode: "insecure_ignore",
			},
		},
		{
			ID:      "staging",
			Name:    "预发环境",
			Address: "203.0.113.11",
			Port:    22,
			User:    "deploy",
			Auth: sshops.AuthConfig{
				PrivateKeyPath: "~/.ssh/id_ed25519",
			},
			HostKey: sshops.HostKeyConfig{
				Mode: "insecure_ignore",
			},
		},
	}
	if err := sshops.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	input := strings.Join([]string{
		"staging",
		"",
		"",
		"",
		"",
		"1",
		"",
		"",
		"",
		"",
		"",
		"n",
		"",
	}, "\n")
	exitCode, stdout, stderr := captureIO(t, input, func() int {
		return run([]string{"edit", "--config", configPath, "prod"})
	})
	if exitCode == 0 {
		t.Fatalf("expected duplicate edit to fail, stdout = %s, stderr = %s", stdout, stderr)
	}
	if !strings.Contains(stderr, "已经存在服务器") {
		t.Fatalf("expected duplicate host warning, got stdout = %s, stderr = %s", stdout, stderr)
	}

	loaded, err := sshops.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(loaded.Hosts) != 2 {
		t.Fatalf("expected both hosts to remain, got %#v", loaded.Hosts)
	}
}

func TestPromptChoiceRetriesOnInvalidInput(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("9\n2\n"))
	var out bytes.Buffer

	value, err := promptChoice(reader, &out, "登录方式", []string{
		"1) 私钥文件",
		"2) 密码环境变量",
	}, "1")
	if err != nil {
		t.Fatalf("promptChoice() error = %v", err)
	}
	if value != "2" {
		t.Fatalf("expected second choice, got %q", value)
	}
	if !strings.Contains(out.String(), "请输入列表里的序号") {
		t.Fatalf("expected retry hint, got %q", out.String())
	}
}

func captureIO(t *testing.T, stdinText string, fn func() int) (int, string, string) {
	t.Helper()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe error = %v", err)
	}
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe error = %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe error = %v", err)
	}

	os.Stdin = stdinReader
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	if stdinText != "" {
		_, _ = io.WriteString(stdinWriter, stdinText)
	}
	_ = stdinWriter.Close()

	exitCode := fn()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdin = oldStdin
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	stdoutData, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	stderrData, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("ReadAll(stderr) error = %v", err)
	}
	return exitCode, string(stdoutData), string(stderrData)
}
