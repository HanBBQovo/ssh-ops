package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hanhan/ssh-ops/internal/sshops"
)

func TestParseTarget(t *testing.T) {
	target, err := parseTarget("deploy@203.0.113.10:2222")
	if err != nil {
		t.Fatalf("parseTarget() error = %v", err)
	}
	if target.User != "deploy" || target.Address != "203.0.113.10" || target.Port != 2222 {
		t.Fatalf("unexpected parsed target: %#v", target)
	}
}

func TestBuildHostConfigFromTarget(t *testing.T) {
	options := configHostOptions{
		ID:             "prod",
		Target:         "deploy@203.0.113.10:2222",
		PrivateKeyPath: "~/.ssh/id_ed25519",
		HostKeyMode:    "insecure_ignore",
	}
	visited := map[string]bool{
		"id":               true,
		"target":           true,
		"private-key-path": true,
		"host-key-mode":    true,
	}

	host, err := buildHostConfig(nil, options, visited)
	if err != nil {
		t.Fatalf("buildHostConfig() error = %v", err)
	}
	if host.User != "deploy" || host.Address != "203.0.113.10" || host.Port != 2222 {
		t.Fatalf("unexpected host: %#v", host)
	}
}

func TestRunConfigInitAndAddHost(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")

	exitCode, output := captureStdout(t, func() int {
		return run([]string{"config", "init", "--config", configPath})
	})
	if exitCode != 0 {
		t.Fatalf("config init exit code = %d, output = %s", exitCode, output)
	}

	exitCode, output = captureStdout(t, func() int {
		return run([]string{
			"config", "add-host",
			"--config", configPath,
			"--id", "prod",
			"--target", "deploy@203.0.113.10:22",
			"--private-key-path", "~/.ssh/id_ed25519",
			"--host-key-mode", "insecure_ignore",
		})
	})
	if exitCode != 0 {
		t.Fatalf("config add-host exit code = %d, output = %s", exitCode, output)
	}

	cfg, err := sshops.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}
	if len(cfg.Hosts) != 1 {
		t.Fatalf("expected one host, got %d", len(cfg.Hosts))
	}
	if cfg.Hosts[0].ID != "prod" {
		t.Fatalf("expected host id prod, got %q", cfg.Hosts[0].ID)
	}
}

func captureStdout(t *testing.T, fn func() int) (int, string) {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = writer

	exitCode := fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	os.Stdout = oldStdout

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return exitCode, string(data)
}
