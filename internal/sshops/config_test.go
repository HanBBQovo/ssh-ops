package sshops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeConfigExpandsEnvAndDefaults(t *testing.T) {
	t.Setenv("SSH_OPS_TEST_PASSWORD", "secret")

	cfg := Config{
		Hosts: []HostConfig{
			{
				ID:      "prod",
				Address: "127.0.0.1",
				User:    "root",
				Auth: AuthConfig{
					PasswordEnv: "SSH_OPS_TEST_PASSWORD",
				},
				HostKey: HostKeyConfig{
					Mode: "insecure_ignore",
				},
			},
		},
	}

	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if cfg.Hosts[0].Port != 22 {
		t.Fatalf("expected default port 22, got %d", cfg.Hosts[0].Port)
	}
	if cfg.Defaults.Shell != "bash" {
		t.Fatalf("expected default shell bash, got %q", cfg.Defaults.Shell)
	}
	if cfg.Hosts[0].Auth.Password != "secret" {
		t.Fatalf("expected password to be loaded from env")
	}
}

func TestResolveConfigPathPrefersExplicitThenEnv(t *testing.T) {
	t.Setenv(ConfigEnvVar, filepath.Join("from", "env.yaml"))

	got := ResolveConfigPath("from/flag.yaml")
	if got != expandPath("from/flag.yaml") {
		t.Fatalf("expected explicit path to win, got %q", got)
	}

	got = ResolveConfigPath("")
	if got != expandPath(filepath.Join("from", "env.yaml")) {
		t.Fatalf("expected env path to win, got %q", got)
	}
}

func TestNormalizeRejectsInvalidHostID(t *testing.T) {
	cfg := Config{
		Hosts: []HostConfig{
			{
				ID:      "Prod Name",
				Address: "127.0.0.1",
				User:    "root",
				Auth: AuthConfig{
					Password: "secret",
				},
				HostKey: HostKeyConfig{
					Mode: "insecure_ignore",
				},
			},
		},
	}

	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected invalid host id error")
	}
}

func TestDefaultConfigPathUsesHomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("home dir unavailable")
	}
	got := defaultConfigPath()
	want := filepath.Join(home, ".config", "ssh-ops", "config.yaml")
	if got != want {
		t.Fatalf("defaultConfigPath() = %q, want %q", got, want)
	}
}
