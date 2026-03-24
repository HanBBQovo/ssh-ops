package main

import (
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
