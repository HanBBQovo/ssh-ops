package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/hanhan/ssh-ops/internal/sshops"
)

type configHostOptions struct {
	ID             string
	Name           string
	Target         string
	Address        string
	Port           int
	User           string
	Password       string
	PasswordEnv    string
	PrivateKey     string
	PrivateKeyPath string
	Passphrase     string
	PassphraseEnv  string
	HostKeyMode    string
	KnownHostsPath string
	Workdir        string
	Shell          string
}

type parsedTarget struct {
	User    string
	Address string
	Port    int
}

func runConfig(args []string) int {
	if len(args) == 0 {
		printConfigUsage(os.Stdout)
		return 0
	}

	switch args[0] {
	case "path":
		return runConfigPath(args[1:])
	case "init":
		return runConfigInit(args[1:])
	case "show":
		return runConfigShow(args[1:])
	case "add-host":
		return runConfigAddHost(args[1:])
	case "set-host":
		return runConfigSetHost(args[1:])
	case "remove-host":
		return runConfigRemoveHost(args[1:])
	case "rename-host":
		return runConfigRenameHost(args[1:])
	case "-h", "--help", "help":
		printConfigUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown config subcommand %q\n\n", args[0])
		printConfigUsage(os.Stderr)
		return 2
	}
}

func runConfigPath(args []string) int {
	fs := newFlagSet("config path")
	configPath := fs.String("config", "", "config file path")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	info, err := os.Stat(resolvedPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.path",
			Error: buildError(sshops.NewUserError("config_stat_failed", "failed to inspect config path", err), map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	result := map[string]interface{}{
		"config_path": resolvedPath,
		"exists":      err == nil,
	}
	if info != nil {
		result["size_bytes"] = info.Size()
	}

	return writeEnvelope(envelope{
		OK:     true,
		Kind:   "config.path",
		Result: result,
	}, *pretty)
}

func runConfigInit(args []string) int {
	fs := newFlagSet("config init")
	configPath := fs.String("config", "", "config file path")
	overwrite := fs.Bool("overwrite", false, "overwrite an existing config file")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	_, err := os.Stat(resolvedPath)
	if err == nil && !*overwrite {
		return writeEnvelope(envelope{
			OK:   false,
			Kind: "config.init",
			Error: &errorPayload{
				Code:    "config_exists",
				Message: "config file already exists; use --overwrite to replace it",
				Details: map[string]interface{}{"config_path": resolvedPath},
			},
		}, *pretty)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.init",
			Error: buildError(sshops.NewUserError("config_stat_failed", "failed to inspect config path", err), map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	cfg := sshops.DefaultConfig()
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.init",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	return writeEnvelope(envelope{
		OK:   true,
		Kind: "config.init",
		Result: map[string]interface{}{
			"config_path": resolvedPath,
			"overwritten": err == nil,
			"config":      cfg.Redacted(),
		},
	}, *pretty)
}

func runConfigShow(args []string) int {
	fs := newFlagSet("config show")
	configPath := fs.String("config", "", "config file path")
	revealSecrets := fs.Bool("reveal-secrets", false, "include inline secrets in output")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, exists, err := sshops.LoadConfigFileOrDefault(resolvedPath)
	if err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.show",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}
	if !*revealSecrets {
		cfg = cfg.Redacted()
	}

	return writeEnvelope(envelope{
		OK:   true,
		Kind: "config.show",
		Result: map[string]interface{}{
			"config_path": resolvedPath,
			"exists":      exists,
			"config":      cfg,
		},
	}, *pretty)
}

func runConfigAddHost(args []string) int {
	fs := newFlagSet("config add-host")
	configPath := fs.String("config", "", "config file path")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	options := bindConfigHostFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	visited := visitedFlags(fs)

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, exists, err := sshops.LoadConfigFileOrDefault(resolvedPath)
	if err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.add-host",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	host, err := buildHostConfig(nil, *options, visited)
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "config.add-host", Error: buildError(err, nil)}, *pretty)
	}
	if err := sshops.AddHost(cfg, host); err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "config.add-host", Error: buildError(err, nil)}, *pretty)
	}
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.add-host",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	return writeEnvelope(envelope{
		OK:   true,
		Kind: "config.add-host",
		Result: map[string]interface{}{
			"config_path":    resolvedPath,
			"created_config": !exists,
			"host":           host,
		},
	}, *pretty)
}

func runConfigSetHost(args []string) int {
	fs := newFlagSet("config set-host")
	configPath := fs.String("config", "", "config file path")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	options := bindConfigHostFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	visited := visitedFlags(fs)

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, exists, err := sshops.LoadConfigFileOrDefault(resolvedPath)
	if err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.set-host",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	hostID := strings.TrimSpace(options.ID)
	if hostID == "" {
		return writeEnvelope(envelope{
			OK:   false,
			Kind: "config.set-host",
			Error: &errorPayload{
				Code:    "invalid_request",
				Message: "host id is required",
			},
		}, *pretty)
	}

	existing := findHostByID(cfg.Hosts, hostID)
	host, err := buildHostConfig(existing, *options, visited)
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "config.set-host", Error: buildError(err, nil)}, *pretty)
	}
	if err := sshops.UpsertHost(cfg, host); err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "config.set-host", Error: buildError(err, nil)}, *pretty)
	}
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.set-host",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	return writeEnvelope(envelope{
		OK:   true,
		Kind: "config.set-host",
		Result: map[string]interface{}{
			"config_path":    resolvedPath,
			"created_config": !exists,
			"created_host":   existing == nil,
			"host":           host,
		},
	}, *pretty)
}

func runConfigRemoveHost(args []string) int {
	fs := newFlagSet("config remove-host")
	configPath := fs.String("config", "", "config file path")
	hostID := fs.String("host", "", "host id")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if strings.TrimSpace(*hostID) == "" {
		return writeEnvelope(envelope{
			OK:   false,
			Kind: "config.remove-host",
			Error: &errorPayload{
				Code:    "invalid_request",
				Message: "host is required",
			},
		}, *pretty)
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, err := sshops.LoadConfigFile(resolvedPath)
	if err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.remove-host",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	removed, err := sshops.RemoveHost(cfg, strings.TrimSpace(*hostID))
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "config.remove-host", Error: buildError(err, nil)}, *pretty)
	}
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.remove-host",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	return writeEnvelope(envelope{
		OK:   true,
		Kind: "config.remove-host",
		Result: map[string]interface{}{
			"config_path": resolvedPath,
			"removed":     removed,
		},
	}, *pretty)
}

func runConfigRenameHost(args []string) int {
	fs := newFlagSet("config rename-host")
	configPath := fs.String("config", "", "config file path")
	hostID := fs.String("host", "", "current host id")
	newID := fs.String("new-id", "", "new host id")
	name := fs.String("name", "", "new display name")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if strings.TrimSpace(*hostID) == "" || strings.TrimSpace(*newID) == "" {
		return writeEnvelope(envelope{
			OK:   false,
			Kind: "config.rename-host",
			Error: &errorPayload{
				Code:    "invalid_request",
				Message: "host and new-id are required",
			},
		}, *pretty)
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, err := sshops.LoadConfigFile(resolvedPath)
	if err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.rename-host",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	renamed, err := sshops.RenameHost(cfg, strings.TrimSpace(*hostID), strings.TrimSpace(*newID), strings.TrimSpace(*name))
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "config.rename-host", Error: buildError(err, nil)}, *pretty)
	}
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "config.rename-host",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	return writeEnvelope(envelope{
		OK:   true,
		Kind: "config.rename-host",
		Result: map[string]interface{}{
			"config_path": resolvedPath,
			"host":        renamed,
		},
	}, *pretty)
}

func bindConfigHostFlags(fs *flag.FlagSet) *configHostOptions {
	options := &configHostOptions{}
	fs.StringVar(&options.ID, "id", "", "host id")
	fs.StringVar(&options.Name, "name", "", "display name")
	fs.StringVar(&options.Target, "target", "", "shortcut target like user@example.com:22")
	fs.StringVar(&options.Address, "address", "", "hostname or IP address")
	fs.IntVar(&options.Port, "port", 0, "SSH port")
	fs.StringVar(&options.User, "user", "", "SSH username")
	fs.StringVar(&options.Password, "password", "", "inline SSH password")
	fs.StringVar(&options.PasswordEnv, "password-env", "", "environment variable containing the SSH password")
	fs.StringVar(&options.PrivateKey, "private-key", "", "inline private key contents")
	fs.StringVar(&options.PrivateKeyPath, "private-key-path", "", "path to a private key file")
	fs.StringVar(&options.Passphrase, "passphrase", "", "inline private key passphrase")
	fs.StringVar(&options.PassphraseEnv, "passphrase-env", "", "environment variable containing the private key passphrase")
	fs.StringVar(&options.HostKeyMode, "host-key-mode", "", "host key verification mode: known_hosts or insecure_ignore")
	fs.StringVar(&options.KnownHostsPath, "known-hosts-path", "", "path to the known_hosts file")
	fs.StringVar(&options.Workdir, "workdir", "", "default remote working directory")
	fs.StringVar(&options.Shell, "shell", "", "default remote shell: bash or sh")
	return options
}

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	return visited
}

func buildHostConfig(existing *sshops.HostConfig, options configHostOptions, visited map[string]bool) (sshops.HostConfig, error) {
	var host sshops.HostConfig
	if existing != nil {
		host = *existing
	}

	if visited["id"] {
		host.ID = strings.TrimSpace(options.ID)
	}
	if visited["name"] {
		host.Name = strings.TrimSpace(options.Name)
	}
	if visited["target"] {
		target, err := parseTarget(options.Target)
		if err != nil {
			return sshops.HostConfig{}, err
		}
		if !visited["user"] && target.User != "" {
			host.User = target.User
		}
		if !visited["address"] {
			host.Address = target.Address
		}
		if !visited["port"] && target.Port > 0 {
			host.Port = target.Port
		}
	}
	if visited["address"] {
		host.Address = strings.TrimSpace(options.Address)
	}
	if visited["port"] {
		host.Port = options.Port
	}
	if visited["user"] {
		host.User = strings.TrimSpace(options.User)
	}
	if visited["password"] {
		host.Auth.Password = options.Password
	}
	if visited["password-env"] {
		host.Auth.PasswordEnv = strings.TrimSpace(options.PasswordEnv)
	}
	if visited["private-key"] {
		host.Auth.PrivateKey = options.PrivateKey
	}
	if visited["private-key-path"] {
		host.Auth.PrivateKeyPath = strings.TrimSpace(options.PrivateKeyPath)
	}
	if visited["passphrase"] {
		host.Auth.Passphrase = options.Passphrase
	}
	if visited["passphrase-env"] {
		host.Auth.PassphraseEnv = strings.TrimSpace(options.PassphraseEnv)
	}
	if visited["host-key-mode"] {
		host.HostKey.Mode = strings.TrimSpace(options.HostKeyMode)
	}
	if visited["known-hosts-path"] {
		host.HostKey.KnownHostsPath = strings.TrimSpace(options.KnownHostsPath)
	}
	if visited["workdir"] {
		host.Defaults.Workdir = strings.TrimSpace(options.Workdir)
	}
	if visited["shell"] {
		host.Defaults.Shell = strings.TrimSpace(options.Shell)
	}

	if strings.TrimSpace(host.ID) == "" {
		return sshops.HostConfig{}, sshops.NewUserError("invalid_request", "host id is required", nil)
	}
	if strings.TrimSpace(host.Address) == "" {
		return sshops.HostConfig{}, sshops.NewUserError("invalid_request", "host address is required", nil)
	}
	if strings.TrimSpace(host.User) == "" {
		return sshops.HostConfig{}, sshops.NewUserError("invalid_request", "host user is required", nil)
	}
	if host.Port < 0 {
		return sshops.HostConfig{}, sshops.NewUserError("invalid_request", "port must be positive", nil)
	}

	if strings.TrimSpace(host.Auth.Password) == "" &&
		strings.TrimSpace(host.Auth.PasswordEnv) == "" &&
		strings.TrimSpace(host.Auth.PrivateKey) == "" &&
		strings.TrimSpace(host.Auth.PrivateKeyPath) == "" {
		return sshops.HostConfig{}, sshops.NewUserError("invalid_request", "configure at least one auth method", nil)
	}

	validator := sshops.DefaultConfig()
	validator.Hosts = []sshops.HostConfig{host}
	if err := validator.Normalize(); err != nil {
		return sshops.HostConfig{}, err
	}
	return validator.Hosts[0], nil
}

func parseTarget(value string) (parsedTarget, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return parsedTarget{}, sshops.NewUserError("invalid_request", "target is required", nil)
	}

	target := parsedTarget{}
	hostPart := value
	if before, after, found := strings.Cut(value, "@"); found {
		target.User = strings.TrimSpace(before)
		hostPart = after
	}

	if hostPart == "" {
		return parsedTarget{}, sshops.NewUserError("invalid_request", "target address is required", nil)
	}

	if strings.Count(hostPart, ":") == 1 {
		address, portText, _ := strings.Cut(hostPart, ":")
		portText = strings.TrimSpace(portText)
		if portText != "" {
			port, err := strconv.Atoi(portText)
			if err != nil || port <= 0 {
				return parsedTarget{}, sshops.NewUserError("invalid_request", "target port must be a positive integer", err)
			}
			target.Port = port
			hostPart = address
		}
	}

	target.Address = strings.TrimSpace(hostPart)
	if target.Address == "" {
		return parsedTarget{}, sshops.NewUserError("invalid_request", "target address is required", nil)
	}
	return target, nil
}

func findHostByID(hosts []sshops.HostConfig, hostID string) *sshops.HostConfig {
	for i := range hosts {
		if hosts[i].ID == hostID {
			host := hosts[i]
			return &host
		}
	}
	return nil
}

func printConfigUsage(w io.Writer) {
	fmt.Fprintln(w, "Manage the local ssh-ops configuration without editing YAML by hand.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  sshctl config <subcommand> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  path         Print the resolved config file path")
	fmt.Fprintln(w, "  init         Create a starter config file")
	fmt.Fprintln(w, "  show         Print the current config as JSON")
	fmt.Fprintln(w, "  add-host     Add a new SSH host entry")
	fmt.Fprintln(w, "  set-host     Create or update a host entry")
	fmt.Fprintln(w, "  remove-host  Remove a host entry by id")
	fmt.Fprintln(w, "  rename-host  Rename a host id or display name")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  sshctl config init --pretty")
	fmt.Fprintln(w, "  sshctl config add-host --id prod --target deploy@203.0.113.10:22 --private-key-path ~/.ssh/id_ed25519 --pretty")
	fmt.Fprintln(w, "  sshctl config set-host --id prod --workdir /srv/app --pretty")
}
