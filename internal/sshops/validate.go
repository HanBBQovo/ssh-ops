package sshops

import "os"

type ValidationReport struct {
	OK         bool             `json:"ok"`
	ConfigPath string           `json:"config_path"`
	Version    string           `json:"version"`
	Errors     []string         `json:"errors,omitempty"`
	Warnings   []string         `json:"warnings,omitempty"`
	Hosts      []HostValidation `json:"hosts"`
}

type HostValidation struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	OK       bool     `json:"ok"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func ValidateConfig(cfg *Config, configPath string) ValidationReport {
	report := ValidationReport{
		OK:         true,
		ConfigPath: configPath,
		Version:    cfg.Version,
		Hosts:      make([]HostValidation, 0, len(cfg.Hosts)),
	}

	if _, err := NewCommandPolicy(cfg.Policy); err != nil {
		report.OK = false
		report.Errors = append(report.Errors, ErrorMessage(err))
	}

	for _, host := range cfg.Hosts {
		validation := HostValidation{
			ID:   host.ID,
			Name: host.Name,
			OK:   true,
		}

		if host.Address == "" {
			validation.OK = false
			validation.Errors = append(validation.Errors, "address is required")
		}
		if host.User == "" {
			validation.OK = false
			validation.Errors = append(validation.Errors, "user is required")
		}
		if normalizeShell(host.Defaults.Shell) == "" && host.Defaults.Shell != "" {
			validation.OK = false
			validation.Errors = append(validation.Errors, "defaults.shell must be bash or sh")
		}

		hasAuth := false
		if host.Auth.Password != "" {
			hasAuth = true
		}
		if host.Auth.Password == "" && host.Auth.PasswordEnv != "" {
			if _, ok := os.LookupEnv(host.Auth.PasswordEnv); ok {
				hasAuth = true
			} else {
				validation.OK = false
				validation.Errors = append(validation.Errors, "password_env is set but not exported")
			}
		}
		if host.Auth.PrivateKey != "" {
			hasAuth = true
		}
		if host.Auth.PrivateKeyPath != "" {
			if _, err := os.Stat(host.Auth.PrivateKeyPath); err != nil {
				validation.OK = false
				validation.Errors = append(validation.Errors, "private_key_path does not exist or is not readable")
			} else {
				hasAuth = true
			}
		}
		if !hasAuth {
			validation.OK = false
			validation.Errors = append(validation.Errors, "at least one auth method must be configured")
		}
		if host.Auth.PassphraseEnv != "" {
			if _, ok := os.LookupEnv(host.Auth.PassphraseEnv); !ok {
				validation.Warnings = append(validation.Warnings, "passphrase_env is set but not exported")
			}
		}

		switch host.HostKey.Mode {
		case "known_hosts":
			if host.HostKey.KnownHostsPath == "" {
				validation.OK = false
				validation.Errors = append(validation.Errors, "known_hosts_path is required when host_key.mode=known_hosts")
			} else if _, err := os.Stat(host.HostKey.KnownHostsPath); err != nil {
				validation.OK = false
				validation.Errors = append(validation.Errors, "known_hosts_path does not exist or is not readable")
			}
		case "insecure_ignore":
			validation.Warnings = append(validation.Warnings, "host key verification is disabled")
		default:
			validation.OK = false
			validation.Errors = append(validation.Errors, "host_key.mode must be known_hosts or insecure_ignore")
		}

		if !validation.OK {
			report.OK = false
		}
		report.Hosts = append(report.Hosts, validation)
	}

	return report
}
