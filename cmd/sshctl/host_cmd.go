package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hanhan/ssh-ops/internal/sshops"
)

func runHost(args []string) int {
	if len(args) == 0 {
		printHostUsage(os.Stdout)
		return 0
	}

	switch args[0] {
	case "ls", "list":
		return runListHosts(args[1:])
	case "show":
		return runHostShow(args[1:])
	case "add":
		return runHostAdd(args[1:])
	case "rm", "remove":
		return runHostRemove(args[1:])
	case "rename":
		return runHostRename(args[1:])
	case "-h", "--help", "help":
		printHostUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown host subcommand %q\n\n", args[0])
		printHostUsage(os.Stderr)
		return 2
	}
}

func runHostShow(args []string) int {
	fs := newFlagSet("host show")
	configPath := fs.String("config", "", "config file path")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	revealSecrets := fs.Bool("reveal-secrets", false, "include inline secrets in output")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	hostID := ""
	if fs.NArg() > 0 {
		hostID = strings.TrimSpace(fs.Arg(0))
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, exists, err := sshops.LoadConfigFileOrDefault(resolvedPath)
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.show", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
	}
	if err := cfg.Normalize(); err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.show", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
	}
	if !*revealSecrets {
		cfg = cfg.Redacted()
	}

	result := map[string]interface{}{
		"config_path": resolvedPath,
		"exists":      exists,
	}
	if hostID == "" {
		result["hosts"] = cfg.Hosts
	} else if host := findHostByID(cfg.Hosts, hostID); host != nil {
		result["host"] = host
	} else {
		return writeEnvelope(envelope{OK: false, Kind: "host.show", Error: &errorPayload{Code: "unknown_host", Message: "unknown host id"}}, *pretty)
	}

	return writeEnvelope(envelope{OK: true, Kind: "host.show", Result: result}, *pretty)
}

func runHostAdd(args []string) int {
	fs := newFlagSet("host add")
	configPath := fs.String("config", "", "config file path")
	name := fs.String("name", "", "display name")
	user := fs.String("user", "", "override user from target")
	address := fs.String("address", "", "override address from target")
	port := fs.Int("port", 0, "override port from target")
	password := fs.String("password", "", "inline password")
	passwordEnv := fs.String("password-env", "", "environment variable with password")
	privateKeyPath := fs.String("private-key-path", "", "private key path")
	passphrase := fs.String("passphrase", "", "inline private key passphrase")
	passphraseEnv := fs.String("passphrase-env", "", "environment variable with private key passphrase")
	hostKeyMode := fs.String("host-key-mode", "", "known_hosts or insecure_ignore")
	knownHostsPath := fs.String("known-hosts-path", "", "known_hosts path")
	workdir := fs.String("workdir", "", "default remote workdir")
	shell := fs.String("shell", "", "default remote shell")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 2 {
		return writeEnvelope(envelope{OK: false, Kind: "host.add", Error: &errorPayload{Code: "invalid_request", Message: "usage: sshctl host add <id> <target> [flags]"}}, *pretty)
	}

	cfg, resolvedPath, _, err := loadRuntimeConfig(*configPath, newLogger(false))
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.add", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
	}

	host, err := buildHostConfig(nil, configHostOptions{
		ID:             fs.Arg(0),
		Name:           *name,
		Target:         fs.Arg(1),
		User:           *user,
		Address:        *address,
		Port:           *port,
		Password:       *password,
		PasswordEnv:    *passwordEnv,
		PrivateKeyPath: *privateKeyPath,
		Passphrase:     *passphrase,
		PassphraseEnv:  *passphraseEnv,
		HostKeyMode:    *hostKeyMode,
		KnownHostsPath: *knownHostsPath,
		Workdir:        *workdir,
		Shell:          *shell,
	}, map[string]bool{
		"id":               true,
		"target":           true,
		"name":             strings.TrimSpace(*name) != "",
		"user":             strings.TrimSpace(*user) != "",
		"address":          strings.TrimSpace(*address) != "",
		"port":             *port != 0,
		"password":         *password != "",
		"password-env":     strings.TrimSpace(*passwordEnv) != "",
		"private-key-path": strings.TrimSpace(*privateKeyPath) != "",
		"passphrase":       *passphrase != "",
		"passphrase-env":   strings.TrimSpace(*passphraseEnv) != "",
		"host-key-mode":    strings.TrimSpace(*hostKeyMode) != "",
		"known-hosts-path": strings.TrimSpace(*knownHostsPath) != "",
		"workdir":          strings.TrimSpace(*workdir) != "",
		"shell":            strings.TrimSpace(*shell) != "",
	})
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.add", Error: buildError(err, nil)}, *pretty)
	}

	if err := sshops.AddHost(cfg, host); err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.add", Error: buildError(err, nil)}, *pretty)
	}
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.add", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
	}

	return writeEnvelope(envelope{OK: true, Kind: "host.add", Host: host.ID, Result: map[string]interface{}{"config_path": resolvedPath, "host": host}}, *pretty)
}

func runHostRemove(args []string) int {
	fs := newFlagSet("host remove")
	configPath := fs.String("config", "", "config file path")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		return writeEnvelope(envelope{OK: false, Kind: "host.remove", Error: &errorPayload{Code: "invalid_request", Message: "usage: sshctl host rm <id>"}}, *pretty)
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, err := sshops.LoadConfigFile(resolvedPath)
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.remove", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
	}
	removed, err := sshops.RemoveHost(cfg, strings.TrimSpace(fs.Arg(0)))
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.remove", Error: buildError(err, nil)}, *pretty)
	}
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.remove", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
	}
	return writeEnvelope(envelope{OK: true, Kind: "host.remove", Host: removed.ID, Result: map[string]interface{}{"config_path": resolvedPath, "removed": removed}}, *pretty)
}

func runHostRename(args []string) int {
	fs := newFlagSet("host rename")
	configPath := fs.String("config", "", "config file path")
	name := fs.String("name", "", "new display name")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 2 {
		return writeEnvelope(envelope{OK: false, Kind: "host.rename", Error: &errorPayload{Code: "invalid_request", Message: "usage: sshctl host rename <old-id> <new-id> [--name ...]"}}, *pretty)
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, err := sshops.LoadConfigFile(resolvedPath)
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.rename", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
	}
	host, err := sshops.RenameHost(cfg, strings.TrimSpace(fs.Arg(0)), strings.TrimSpace(fs.Arg(1)), strings.TrimSpace(*name))
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.rename", Error: buildError(err, nil)}, *pretty)
	}
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "host.rename", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
	}
	return writeEnvelope(envelope{OK: true, Kind: "host.rename", Host: host.ID, Result: map[string]interface{}{"config_path": resolvedPath, "host": host}}, *pretty)
}

func printHostUsage(w io.Writer) {
	fmt.Fprintln(w, "Manage saved hosts with short, human-friendly commands.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  sshctl host ls")
	fmt.Fprintln(w, "  sshctl host show [id]")
	fmt.Fprintln(w, "  sshctl host add <id> <target> [flags]")
	fmt.Fprintln(w, "  sshctl host rm <id>")
	fmt.Fprintln(w, "  sshctl host rename <old-id> <new-id> [--name ...]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  sshctl host add prod deploy@203.0.113.10 --private-key-path ~/.ssh/id_ed25519 --host-key-mode known_hosts")
	fmt.Fprintln(w, "  sshctl host add test root@192.168.1.9 --password-env SSH_OPS_TEST_PASSWORD --host-key-mode insecure_ignore")
	fmt.Fprintln(w, "  sshctl exec --target root@192.168.1.9 --password-env SSH_OPS_TEST_PASSWORD --command 'df -h'")
}
