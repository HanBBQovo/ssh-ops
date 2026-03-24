package sshops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigFileOrDefaultReturnsDefaultWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg, exists, err := LoadConfigFileOrDefault(path)
	if err != nil {
		t.Fatalf("LoadConfigFileOrDefault() error = %v", err)
	}
	if exists {
		t.Fatal("expected exists=false for a missing config file")
	}
	if cfg == nil || cfg.Version != "1" {
		t.Fatalf("expected default config, got %#v", cfg)
	}
	if len(cfg.Hosts) != 0 {
		t.Fatalf("expected no hosts, got %d", len(cfg.Hosts))
	}
}

func TestSaveConfigRoundTripsManagedHosts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := DefaultConfig()

	host := HostConfig{
		ID:      "prod",
		Name:    "Production",
		Address: "203.0.113.10",
		User:    "deploy",
		Auth: AuthConfig{
			PrivateKeyPath: "~/.ssh/id_ed25519",
			PassphraseEnv:  "SSH_OPS_PROD_KEY_PASSPHRASE",
		},
		HostKey: HostKeyConfig{
			Mode: "known_hosts",
		},
	}
	if err := AddHost(cfg, host); err != nil {
		t.Fatalf("AddHost() error = %v", err)
	}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "id: prod") {
		t.Fatalf("expected saved config to contain host id, got:\n%s", text)
	}
	if !strings.Contains(text, "private_key_path: ~/.ssh/id_ed25519") {
		t.Fatalf("expected saved config to preserve raw path, got:\n%s", text)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(loaded.Hosts) != 1 {
		t.Fatalf("expected one host after round-trip, got %d", len(loaded.Hosts))
	}
	if loaded.Hosts[0].Port != 22 {
		t.Fatalf("expected normalized port 22, got %d", loaded.Hosts[0].Port)
	}
}

func TestRenameHostAndRemoveHost(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Hosts = []HostConfig{
		{
			ID:      "prod",
			Address: "203.0.113.10",
			User:    "deploy",
			Auth: AuthConfig{
				PrivateKeyPath: "~/.ssh/id_ed25519",
			},
			HostKey: HostKeyConfig{Mode: "insecure_ignore"},
		},
	}

	renamed, err := RenameHost(cfg, "prod", "prod-gz", "广州生产")
	if err != nil {
		t.Fatalf("RenameHost() error = %v", err)
	}
	if renamed.ID != "prod-gz" {
		t.Fatalf("expected renamed id, got %q", renamed.ID)
	}

	removed, err := RemoveHost(cfg, "prod-gz")
	if err != nil {
		t.Fatalf("RemoveHost() error = %v", err)
	}
	if removed.ID != "prod-gz" {
		t.Fatalf("expected removed host id prod-gz, got %q", removed.ID)
	}
	if len(cfg.Hosts) != 0 {
		t.Fatalf("expected all hosts removed, got %d", len(cfg.Hosts))
	}
}
