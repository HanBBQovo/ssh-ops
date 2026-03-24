package sshops

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func DefaultConfig() *Config {
	return &Config{
		Version: "1",
		Defaults: DefaultsConfig{
			ConnectTimeoutSec:   10,
			OperationTimeoutSec: 120,
			MaxOutputBytes:      1 << 20,
			Shell:               "bash",
		},
		Policy: PolicyConfig{
			AllowPatterns: []string{},
			DenyPatterns:  []string{},
		},
		Hosts: []HostConfig{},
	}
}

func LoadConfigFile(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, NewUserError("config_read_failed", "failed to read config file", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, NewUserError("config_parse_failed", "failed to parse config file", err)
	}
	return &cfg, nil
}

func LoadConfigFileOrDefault(path string) (*Config, bool, error) {
	cfg, err := LoadConfigFile(path)
	if err == nil {
		return cfg, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return DefaultConfig(), false, nil
	}
	return nil, false, err
}

func SaveConfig(path string, cfg *Config) error {
	if cfg == nil {
		return NewUserError("config_invalid", "config is required", nil)
	}

	clone := cfg.Clone()
	if err := clone.Normalize(); err != nil {
		return err
	}

	if clone.Policy.AllowPatterns == nil {
		cfg.Policy.AllowPatterns = []string{}
	}
	if clone.Policy.DenyPatterns == nil {
		cfg.Policy.DenyPatterns = []string{}
	}
	if cfg.Hosts == nil {
		cfg.Hosts = []HostConfig{}
	}

	sortHosts(cfg.Hosts)

	raw, err := marshalConfigYAML(cfg)
	if err != nil {
		return NewUserError("config_write_failed", "failed to encode config file", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return NewUserError("config_write_failed", "failed to create config directory", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return NewUserError("config_write_failed", "failed to write config file", err)
	}
	return nil
}

func marshalConfigYAML(cfg *Config) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		_ = encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	clone := *c
	clone.Policy.AllowPatterns = append([]string(nil), c.Policy.AllowPatterns...)
	clone.Policy.DenyPatterns = append([]string(nil), c.Policy.DenyPatterns...)
	clone.Hosts = append([]HostConfig(nil), c.Hosts...)
	return &clone
}

func (c *Config) Redacted() *Config {
	clone := c.Clone()
	if clone == nil {
		return nil
	}
	for i := range clone.Hosts {
		redactSecret(&clone.Hosts[i].Auth.Password)
		redactSecret(&clone.Hosts[i].Auth.PrivateKey)
		redactSecret(&clone.Hosts[i].Auth.Passphrase)
	}
	return clone
}

func AddHost(cfg *Config, host HostConfig) error {
	if cfg == nil {
		return NewUserError("config_invalid", "config is required", nil)
	}
	host.ID = strings.TrimSpace(host.ID)
	if host.ID == "" {
		return NewUserError("invalid_request", "host id is required", nil)
	}
	if findHostIndex(cfg.Hosts, host.ID) >= 0 {
		return NewUserError("duplicate_host", "host id already exists", fmt.Errorf("%q", host.ID))
	}
	cfg.Hosts = append(cfg.Hosts, host)
	sortHosts(cfg.Hosts)
	return nil
}

func UpsertHost(cfg *Config, host HostConfig) error {
	if cfg == nil {
		return NewUserError("config_invalid", "config is required", nil)
	}
	host.ID = strings.TrimSpace(host.ID)
	if host.ID == "" {
		return NewUserError("invalid_request", "host id is required", nil)
	}

	if index := findHostIndex(cfg.Hosts, host.ID); index >= 0 {
		cfg.Hosts[index] = host
		sortHosts(cfg.Hosts)
		return nil
	}

	cfg.Hosts = append(cfg.Hosts, host)
	sortHosts(cfg.Hosts)
	return nil
}

func RemoveHost(cfg *Config, hostID string) (*HostConfig, error) {
	if cfg == nil {
		return nil, NewUserError("config_invalid", "config is required", nil)
	}

	index := findHostIndex(cfg.Hosts, hostID)
	if index < 0 {
		return nil, NewUserError("unknown_host", "unknown host id", fmt.Errorf("%q", hostID))
	}

	removed := cfg.Hosts[index]
	cfg.Hosts = append(cfg.Hosts[:index], cfg.Hosts[index+1:]...)
	return &removed, nil
}

func RenameHost(cfg *Config, hostID, newID, newName string) (*HostConfig, error) {
	if cfg == nil {
		return nil, NewUserError("config_invalid", "config is required", nil)
	}
	index := findHostIndex(cfg.Hosts, hostID)
	if index < 0 {
		return nil, NewUserError("unknown_host", "unknown host id", fmt.Errorf("%q", hostID))
	}
	if strings.TrimSpace(newID) == "" {
		return nil, NewUserError("invalid_request", "new host id is required", nil)
	}
	if hostID != newID && findHostIndex(cfg.Hosts, newID) >= 0 {
		return nil, NewUserError("duplicate_host", "host id already exists", fmt.Errorf("%q", newID))
	}

	cfg.Hosts[index].ID = strings.TrimSpace(newID)
	if strings.TrimSpace(newName) != "" {
		cfg.Hosts[index].Name = strings.TrimSpace(newName)
	}
	sortHosts(cfg.Hosts)
	updated := cfg.Hosts[findHostIndex(cfg.Hosts, strings.TrimSpace(newID))]
	return &updated, nil
}

func findHostIndex(hosts []HostConfig, hostID string) int {
	for i := range hosts {
		if hosts[i].ID == hostID {
			return i
		}
	}
	return -1
}

func sortHosts(hosts []HostConfig) {
	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].ID < hosts[j].ID
	})
}

func redactSecret(value *string) {
	if value == nil {
		return
	}
	if strings.TrimSpace(*value) != "" {
		*value = "***"
	}
}
