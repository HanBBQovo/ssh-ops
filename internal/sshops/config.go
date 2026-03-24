package sshops

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const ConfigEnvVar = "SSH_OPS_CONFIG"

var hostIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

type Config struct {
	Version  string         `yaml:"version"`
	Defaults DefaultsConfig `yaml:"defaults"`
	Policy   PolicyConfig   `yaml:"policy"`
	Hosts    []HostConfig   `yaml:"hosts"`
}

type DefaultsConfig struct {
	ConnectTimeoutSec   int    `yaml:"connect_timeout_sec"`
	OperationTimeoutSec int    `yaml:"operation_timeout_sec"`
	MaxOutputBytes      int64  `yaml:"max_output_bytes"`
	Shell               string `yaml:"shell"`
}

type PolicyConfig struct {
	AllowPatterns []string `yaml:"allow_patterns"`
	DenyPatterns  []string `yaml:"deny_patterns"`
}

type HostConfig struct {
	ID       string        `yaml:"id"`
	Name     string        `yaml:"name,omitempty"`
	Address  string        `yaml:"address,omitempty"`
	Port     int           `yaml:"port,omitempty"`
	User     string        `yaml:"user,omitempty"`
	Auth     AuthConfig    `yaml:"auth,omitempty"`
	HostKey  HostKeyConfig `yaml:"host_key,omitempty"`
	Defaults HostDefaults  `yaml:"defaults,omitempty"`
}

type AuthConfig struct {
	Password       string `yaml:"password,omitempty"`
	PasswordEnv    string `yaml:"password_env,omitempty"`
	PrivateKey     string `yaml:"private_key,omitempty"`
	PrivateKeyPath string `yaml:"private_key_path,omitempty"`
	Passphrase     string `yaml:"passphrase,omitempty"`
	PassphraseEnv  string `yaml:"passphrase_env,omitempty"`
}

type HostKeyConfig struct {
	Mode           string `yaml:"mode,omitempty"`
	KnownHostsPath string `yaml:"known_hosts_path,omitempty"`
}

type HostDefaults struct {
	Workdir string `yaml:"workdir,omitempty"`
	Shell   string `yaml:"shell,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	cfg, err := LoadConfigFile(path)
	if err != nil {
		return nil, err
	}
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func ResolveConfigPath(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return expandPath(explicit)
	}
	if envPath := strings.TrimSpace(os.Getenv(ConfigEnvVar)); envPath != "" {
		return expandPath(envPath)
	}
	return defaultConfigPath()
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "ssh-ops", "config.yaml")
	}
	return filepath.Join(home, ".config", "ssh-ops", "config.yaml")
}

func (c *Config) Normalize() error {
	c.Version = strings.TrimSpace(expandEnv(c.Version))
	if c.Version == "" {
		c.Version = "1"
	}
	if c.Version != "1" {
		return NewUserError("config_invalid", "unsupported config version", fmt.Errorf("version %q", c.Version))
	}

	if c.Defaults.ConnectTimeoutSec <= 0 {
		c.Defaults.ConnectTimeoutSec = 10
	}
	if c.Defaults.OperationTimeoutSec <= 0 {
		c.Defaults.OperationTimeoutSec = 120
	}
	if c.Defaults.MaxOutputBytes <= 0 {
		c.Defaults.MaxOutputBytes = 1 << 20
	}
	c.Defaults.Shell = normalizeShell(c.Defaults.Shell)
	if c.Defaults.Shell == "" {
		c.Defaults.Shell = "bash"
	}

	seen := make(map[string]struct{}, len(c.Hosts))
	for i := range c.Hosts {
		host := &c.Hosts[i]
		host.ID = strings.TrimSpace(expandEnv(host.ID))
		host.Name = strings.TrimSpace(expandEnv(host.Name))
		host.Address = strings.TrimSpace(expandEnv(host.Address))
		host.User = strings.TrimSpace(expandEnv(host.User))

		if host.ID == "" {
			return NewUserError("config_invalid", "host id is required", nil)
		}
		if !hostIDPattern.MatchString(host.ID) {
			return NewUserError("config_invalid", "host id must use lowercase letters, digits, dots, underscores, or hyphens", fmt.Errorf("host %q", host.ID))
		}
		if _, exists := seen[host.ID]; exists {
			return NewUserError("config_invalid", "duplicate host id", fmt.Errorf("host %q", host.ID))
		}
		seen[host.ID] = struct{}{}

		if host.Name == "" {
			host.Name = host.ID
		}
		if host.Port == 0 {
			host.Port = 22
		}

		host.Auth.Password = expandEnv(host.Auth.Password)
		host.Auth.PasswordEnv = strings.TrimSpace(expandEnv(host.Auth.PasswordEnv))
		host.Auth.PrivateKey = expandEnv(host.Auth.PrivateKey)
		host.Auth.PrivateKeyPath = expandPath(host.Auth.PrivateKeyPath)
		host.Auth.Passphrase = expandEnv(host.Auth.Passphrase)
		host.Auth.PassphraseEnv = strings.TrimSpace(expandEnv(host.Auth.PassphraseEnv))

		if host.Auth.Password == "" && host.Auth.PasswordEnv != "" {
			host.Auth.Password = os.Getenv(host.Auth.PasswordEnv)
		}
		if host.Auth.Passphrase == "" && host.Auth.PassphraseEnv != "" {
			host.Auth.Passphrase = os.Getenv(host.Auth.PassphraseEnv)
		}

		host.HostKey.Mode = strings.TrimSpace(expandEnv(host.HostKey.Mode))
		if host.HostKey.Mode == "" {
			host.HostKey.Mode = "known_hosts"
		}
		host.HostKey.KnownHostsPath = expandPath(host.HostKey.KnownHostsPath)
		if host.HostKey.Mode == "known_hosts" && host.HostKey.KnownHostsPath == "" {
			host.HostKey.KnownHostsPath = expandPath("~/.ssh/known_hosts")
		}

		host.Defaults.Workdir = strings.TrimSpace(expandEnv(host.Defaults.Workdir))
		host.Defaults.Shell = normalizeShell(host.Defaults.Shell)
	}

	return nil
}
