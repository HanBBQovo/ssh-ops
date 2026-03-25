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

func TestNormalizeAllowsEmptyHosts(t *testing.T) {
	cfg := Config{}

	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if cfg.Version != "1" {
		t.Fatalf("expected version 1, got %q", cfg.Version)
	}
	if cfg.Defaults.Shell != "bash" {
		t.Fatalf("expected default shell bash, got %q", cfg.Defaults.Shell)
	}
	if len(cfg.Hosts) != 0 {
		t.Fatalf("expected no hosts, got %d", len(cfg.Hosts))
	}
}

func TestNormalizeConfigPreservesInlineSecretsWithDollarSigns(t *testing.T) {
	cfg := Config{
		Hosts: []HostConfig{
			{
				ID:      "prod",
				Address: "127.0.0.1",
				User:    "root",
				Auth: AuthConfig{
					Password:   "IHBczrc!%@#$@",
					Passphrase: "key-pass$@word",
					PrivateKey: "-----BEGIN KEY-----\n$KEEP_ME\n-----END KEY-----",
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

	if cfg.Hosts[0].Auth.Password != "IHBczrc!%@#$@" {
		t.Fatalf("expected password to stay unchanged, got %q", cfg.Hosts[0].Auth.Password)
	}
	if cfg.Hosts[0].Auth.Passphrase != "key-pass$@word" {
		t.Fatalf("expected passphrase to stay unchanged, got %q", cfg.Hosts[0].Auth.Passphrase)
	}
	if cfg.Hosts[0].Auth.PrivateKey != "-----BEGIN KEY-----\n$KEEP_ME\n-----END KEY-----" {
		t.Fatalf("expected private key to stay unchanged, got %q", cfg.Hosts[0].Auth.PrivateKey)
	}
}

func TestNormalizeConfigExpandsBareAndBracedEnvInNonSecretFields(t *testing.T) {
	t.Setenv("SSH_OPS_TEST_HOST", "203.0.113.10")
	t.Setenv("SSH_OPS_TEST_USER", "deploy")
	t.Setenv("SSH_OPS_TEST_WORKDIR", "/srv/app")

	cfg := Config{
		Hosts: []HostConfig{
			{
				ID:      "prod",
				Address: "$SSH_OPS_TEST_HOST",
				User:    "${SSH_OPS_TEST_USER}",
				Auth: AuthConfig{
					PasswordEnv: "SSH_OPS_TEST_PASSWORD",
				},
				HostKey: HostKeyConfig{
					Mode: "insecure_ignore",
				},
				Defaults: HostDefaults{
					Workdir: "$SSH_OPS_TEST_WORKDIR",
				},
			},
		},
	}

	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if cfg.Hosts[0].Address != "203.0.113.10" {
		t.Fatalf("expected expanded address, got %q", cfg.Hosts[0].Address)
	}
	if cfg.Hosts[0].User != "deploy" {
		t.Fatalf("expected expanded user, got %q", cfg.Hosts[0].User)
	}
	if cfg.Hosts[0].Defaults.Workdir != "/srv/app" {
		t.Fatalf("expected expanded workdir, got %q", cfg.Hosts[0].Defaults.Workdir)
	}
}
