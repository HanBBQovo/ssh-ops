package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/hanhan/ssh-ops/internal/sshops"
)

type runtimeHostOptions struct {
	Target         string
	User           string
	Address        string
	Port           int
	Password       string
	PasswordEnv    string
	PrivateKeyPath string
	Passphrase     string
	PassphraseEnv  string
	HostKeyMode    string
	KnownHostsPath string
	SaveHost       string
	SaveName       string
}

func loadRuntimeConfig(configPath string, logger *log.Logger) (*sshops.Config, string, bool, error) {
	resolvedPath := sshops.ResolveConfigPath(configPath)
	cfg, exists, err := sshops.LoadConfigFileOrDefault(resolvedPath)
	if err != nil {
		return nil, resolvedPath, false, err
	}
	if err := cfg.Normalize(); err != nil {
		return nil, resolvedPath, exists, err
	}
	return cfg, resolvedPath, exists, nil
}

func runtimeServiceForHost(configPath string, logger *log.Logger, hostID string, options runtimeHostOptions) (*sshops.Service, *sshops.HostConfig, *sshops.Config, string, bool, error) {
	cfg, resolvedPath, exists, err := loadRuntimeConfig(configPath, logger)
	if err != nil {
		return nil, nil, nil, resolvedPath, false, err
	}

	if strings.TrimSpace(hostID) != "" {
		service, err := sshops.NewService(cfg, logger)
		if err != nil {
			return nil, nil, nil, resolvedPath, exists, err
		}
		host, err := service.Host(strings.TrimSpace(hostID))
		if err != nil {
			return nil, nil, nil, resolvedPath, exists, err
		}
		return service, host, cfg, resolvedPath, exists, nil
	}

	host, err := buildRuntimeHost(options)
	if err != nil {
		return nil, nil, nil, resolvedPath, exists, err
	}
	cfg.Hosts = append(cfg.Hosts, host)

	service, err := sshops.NewService(cfg, logger)
	if err != nil {
		return nil, nil, nil, resolvedPath, exists, err
	}
	resolvedHost, err := service.Host(host.ID)
	if err != nil {
		return nil, nil, nil, resolvedPath, exists, err
	}
	return service, resolvedHost, cfg, resolvedPath, exists, nil
}

func buildRuntimeHost(options runtimeHostOptions) (sshops.HostConfig, error) {
	configOptions := configHostOptions{
		ID:             "adhoc",
		Target:         options.Target,
		Address:        options.Address,
		Port:           options.Port,
		User:           options.User,
		Password:       options.Password,
		PasswordEnv:    options.PasswordEnv,
		PrivateKeyPath: options.PrivateKeyPath,
		Passphrase:     options.Passphrase,
		PassphraseEnv:  options.PassphraseEnv,
		HostKeyMode:    options.HostKeyMode,
		KnownHostsPath: options.KnownHostsPath,
	}

	visited := map[string]bool{
		"id": true,
	}
	if strings.TrimSpace(options.Target) != "" {
		visited["target"] = true
	}
	if strings.TrimSpace(options.Address) != "" {
		visited["address"] = true
	}
	if strings.TrimSpace(options.User) != "" {
		visited["user"] = true
	}
	if options.Port != 0 {
		visited["port"] = true
	}
	if options.Password != "" {
		visited["password"] = true
	}
	if options.PasswordEnv != "" {
		visited["password-env"] = true
	}
	if options.PrivateKeyPath != "" {
		visited["private-key-path"] = true
	}
	if options.Passphrase != "" {
		visited["passphrase"] = true
	}
	if options.PassphraseEnv != "" {
		visited["passphrase-env"] = true
	}
	if options.HostKeyMode != "" {
		visited["host-key-mode"] = true
	}
	if options.KnownHostsPath != "" {
		visited["known-hosts-path"] = true
	}

	host, err := buildHostConfig(nil, configOptions, visited)
	if err != nil {
		return sshops.HostConfig{}, err
	}
	host.Name = ""
	if strings.TrimSpace(options.SaveHost) != "" {
		host.ID = strings.TrimSpace(options.SaveHost)
		if strings.TrimSpace(options.SaveName) != "" {
			host.Name = strings.TrimSpace(options.SaveName)
		}
	}
	return host, nil
}

func maybePersistRuntimeHost(resolvedPath string, cfg *sshops.Config, source *sshops.HostConfig, options runtimeHostOptions) error {
	if source == nil || strings.TrimSpace(options.SaveHost) == "" {
		return nil
	}

	host := *source
	host.ID = strings.TrimSpace(options.SaveHost)
	if strings.TrimSpace(options.SaveName) != "" {
		host.Name = strings.TrimSpace(options.SaveName)
	}
	if err := sshops.UpsertHost(cfg, host); err != nil {
		return err
	}
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		return fmt.Errorf("save host %q: %w", host.ID, err)
	}
	return nil
}
