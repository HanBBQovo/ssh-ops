package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/hanhan/ssh-ops/internal/sshops"
)

func runAdd(args []string) int {
	fs := newFlagSet("add")
	configPath := fs.String("config", "", "config file path")
	noTest := fs.Bool("no-test", false, "skip the connection test after saving")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() >= 2 {
		rebuilt := append([]string{"--config", *configPath}, fs.Args()...)
		return runHostAdd(rebuilt)
	}

	host, testAfterSave, err := runAddWizard(os.Stdin, os.Stdout, addWizardOptions{NoTest: *noTest})
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "add", Error: buildError(err, nil)}, *pretty)
	}

	cfg, resolvedPath, _, err := loadRuntimeConfig(*configPath, newLogger(false))
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "add", Error: buildError(err, nil)}, *pretty)
	}
	if err := sshops.UpsertHost(cfg, host); err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "add", Error: buildError(err, nil)}, *pretty)
	}
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "add", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
	}

	fmt.Fprintf(os.Stdout, "\n已保存服务器 %q。\n", host.ID)
	fmt.Fprintf(os.Stdout, "目标: %s@%s:%d\n", host.User, host.Address, host.Port)
	if host.Defaults.Workdir != "" {
		fmt.Fprintf(os.Stdout, "默认目录: %s\n", host.Defaults.Workdir)
	}
	fmt.Fprintf(os.Stdout, "配置文件: %s\n", resolvedPath)

	if testAfterSave {
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintln(os.Stdout, "正在测试连接...")
		logger := newLogger(false)
		service, _, _, _, _, err := runtimeServiceForHost(*configPath, logger, host.ID, runtimeHostOptions{})
		if err != nil {
			return writeEnvelope(envelope{OK: false, Kind: "add", Error: buildError(err, nil)}, *pretty)
		}
		result, err := service.Test(context.Background(), sshops.TestRequest{HostID: host.ID})
		if err != nil {
			fmt.Fprintf(os.Stdout, "连接测试失败: %s\n", sshops.ErrorMessage(err))
			fmt.Fprintf(os.Stdout, "你仍然可以稍后执行: sshctl test %s\n", host.ID)
			return 0
		}
		fmt.Fprintf(os.Stdout, "连接成功: %s@%s:%d (%dms)\n", result.User, result.Address, result.Port, result.DurationMS)
	}

	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintf(os.Stdout, "下次直接这样用:\n  sshctl run %s \"df -h\"\n  sshctl test %s\n", host.ID, host.ID)
	return 0
}

func runList(args []string) int {
	fs := newFlagSet("list")
	configPath := fs.String("config", "", "config file path")
	jsonOutput := fs.Bool("json", false, "output JSON instead of a table")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	verbose := fs.Bool("verbose", false, "write debug logs to stderr")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *jsonOutput {
		rebuilt := maybeAppendFlag(nil, "config", *configPath)
		if *pretty {
			rebuilt = append(rebuilt, "--pretty")
		}
		if *verbose {
			rebuilt = append(rebuilt, "--verbose")
		}
		return runListHosts(rebuilt)
	}

	logger := newLogger(*verbose)
	cfg, resolvedPath, exists, err := loadRuntimeConfig(*configPath, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}
	service, err := sshops.NewService(cfg, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载服务器列表失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}

	hosts := service.ListHosts()
	if !exists || len(hosts) == 0 {
		fmt.Fprintf(os.Stdout, "还没有保存任何服务器。先运行 `sshctl add`。\n配置文件: %s\n", resolvedPath)
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTARGET\tNAME")
	for _, host := range hosts {
		target := fmt.Sprintf("%s@%s:%d", host.User, host.Address, host.Port)
		fmt.Fprintf(w, "%s\t%s\t%s\n", host.ID, target, host.Name)
	}
	_ = w.Flush()
	return 0
}

func runEdit(args []string) int {
	fs := newFlagSet("edit")
	configPath := fs.String("config", "", "config file path")
	noTest := fs.Bool("no-test", false, "skip the connection test after saving")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, err := sshops.LoadConfigFile(resolvedPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}
	if err := cfg.Normalize(); err != nil {
		fmt.Fprintf(os.Stderr, "配置无效: %s\n", sshops.ErrorMessage(err))
		return 1
	}

	reader := bufio.NewReader(os.Stdin)
	var selected *sshops.HostConfig
	if fs.NArg() > 0 {
		selected = findHostByID(cfg.Hosts, strings.TrimSpace(fs.Arg(0)))
		if selected == nil {
			fmt.Fprintf(os.Stderr, "找不到服务器 %q。\n", fs.Arg(0))
			return 1
		}
	} else {
		selected, err = chooseHost(reader, os.Stdout, cfg.Hosts, "请输入要编辑的服务器序号、别名或显示名称")
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", sshops.ErrorMessage(err))
			return 1
		}
	}

	edited, testAfterSave, err := runEditWizard(reader, os.Stdout, *selected, editWizardOptions{NoTest: *noTest})
	if err != nil {
		fmt.Fprintf(os.Stderr, "编辑失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}

	if edited.ID != selected.ID && findHostByID(cfg.Hosts, edited.ID) != nil {
		fmt.Fprintf(os.Stderr, "已经存在服务器 %q，请换一个名字。\n", edited.ID)
		return 1
	}

	if edited.ID != selected.ID {
		if _, err := sshops.RemoveHost(cfg, selected.ID); err != nil {
			fmt.Fprintf(os.Stderr, "更新服务器名字失败: %s\n", sshops.ErrorMessage(err))
			return 1
		}
	}
	if err := sshops.UpsertHost(cfg, edited); err != nil {
		fmt.Fprintf(os.Stderr, "保存失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "写入配置失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}

	fmt.Fprintf(os.Stdout, "\n已更新服务器 %q。\n", edited.ID)
	if testAfterSave {
		logger := newLogger(false)
		service, _, _, _, _, err := runtimeServiceForHost(*configPath, logger, edited.ID, runtimeHostOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "加载服务器失败: %s\n", sshops.ErrorMessage(err))
			return 1
		}
		result, err := service.Test(context.Background(), sshops.TestRequest{HostID: edited.ID})
		if err != nil {
			fmt.Fprintf(os.Stdout, "连接测试失败: %s\n", sshops.ErrorMessage(err))
			fmt.Fprintf(os.Stdout, "你可以稍后执行: sshctl test %s\n", edited.ID)
			return 0
		}
		fmt.Fprintf(os.Stdout, "连接成功: %s@%s:%d (%dms)\n", result.User, result.Address, result.Port, result.DurationMS)
	}
	return 0
}

func runRemove(args []string) int {
	fs := newFlagSet("remove")
	configPath := fs.String("config", "", "config file path")
	force := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, err := sshops.LoadConfigFile(resolvedPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}
	if err := cfg.Normalize(); err != nil {
		fmt.Fprintf(os.Stderr, "配置无效: %s\n", sshops.ErrorMessage(err))
		return 1
	}

	reader := bufio.NewReader(os.Stdin)
	var selected *sshops.HostConfig
	if fs.NArg() > 0 {
		selected = findHostByID(cfg.Hosts, strings.TrimSpace(fs.Arg(0)))
		if selected == nil {
			fmt.Fprintf(os.Stderr, "找不到服务器 %q。\n", fs.Arg(0))
			return 1
		}
	} else {
		selected, err = chooseHost(reader, os.Stdout, cfg.Hosts, "请输入要删除的服务器序号、别名或显示名称")
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", sshops.ErrorMessage(err))
			return 1
		}
	}

	if !*force {
		answer, promptErr := promptOptional(reader, os.Stdout, fmt.Sprintf("确认删除 %s 吗？[y/N]", selected.ID), "N")
		if promptErr != nil {
			fmt.Fprintf(os.Stderr, "读取确认失败: %v\n", promptErr)
			return 1
		}
		if !normalizeYesNo(answer, false) {
			fmt.Fprintln(os.Stdout, "已取消。")
			return 0
		}
	}

	removed, err := sshops.RemoveHost(cfg, selected.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "删除失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}
	if err := sshops.SaveConfig(resolvedPath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "写入配置失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}

	fmt.Fprintf(os.Stdout, "已删除服务器 %q。\n", removed.ID)
	return 0
}

func runShow(args []string) int {
	fs := newFlagSet("show")
	configPath := fs.String("config", "", "config file path")
	jsonOutput := fs.Bool("json", false, "output JSON")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	revealSecrets := fs.Bool("reveal-secrets", false, "include inline secrets in output")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	hostID := ""
	if fs.NArg() > 0 {
		hostID = strings.TrimSpace(fs.Arg(0))
	}

	if *jsonOutput {
		rebuilt := maybeAppendFlag(nil, "config", *configPath)
		if *revealSecrets {
			rebuilt = append(rebuilt, "--reveal-secrets")
		}
		if *pretty {
			rebuilt = append(rebuilt, "--pretty")
		}
		if hostID != "" {
			return runHostShow(append(rebuilt, hostID))
		}
		return runHostShow(rebuilt)
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, exists, err := sshops.LoadConfigFileOrDefault(resolvedPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}
	if err := cfg.Normalize(); err != nil {
		fmt.Fprintf(os.Stderr, "配置无效: %s\n", sshops.ErrorMessage(err))
		return 1
	}
	if !*revealSecrets {
		cfg = cfg.Redacted()
	}

	if !exists || len(cfg.Hosts) == 0 {
		fmt.Fprintf(os.Stdout, "还没有保存任何服务器。先运行 `sshctl add`。\n配置文件: %s\n", resolvedPath)
		return 0
	}

	if hostID == "" {
		return runList([]string{"--config", *configPath})
	}

	host := findHostByID(cfg.Hosts, hostID)
	if host == nil {
		fmt.Fprintf(os.Stderr, "找不到服务器 %q。\n", hostID)
		return 1
	}

	fmt.Fprintf(os.Stdout, "ID: %s\n", host.ID)
	if host.Name != "" {
		fmt.Fprintf(os.Stdout, "名称: %s\n", host.Name)
	}
	fmt.Fprintf(os.Stdout, "地址: %s\n", host.Address)
	fmt.Fprintf(os.Stdout, "端口: %d\n", host.Port)
	fmt.Fprintf(os.Stdout, "用户: %s\n", host.User)
	if host.Defaults.Workdir != "" {
		fmt.Fprintf(os.Stdout, "默认目录: %s\n", host.Defaults.Workdir)
	}
	if host.Auth.PrivateKeyPath != "" {
		fmt.Fprintf(os.Stdout, "私钥: %s\n", host.Auth.PrivateKeyPath)
	}
	if host.Auth.PasswordEnv != "" {
		fmt.Fprintf(os.Stdout, "密码环境变量: %s\n", host.Auth.PasswordEnv)
	}
	if host.Auth.Password != "" && *revealSecrets {
		fmt.Fprintf(os.Stdout, "密码: %s\n", host.Auth.Password)
	}
	fmt.Fprintf(os.Stdout, "Host Key 校验: %s\n", host.HostKey.Mode)
	if host.HostKey.KnownHostsPath != "" {
		fmt.Fprintf(os.Stdout, "known_hosts: %s\n", host.HostKey.KnownHostsPath)
	}
	return 0
}

func runTest(args []string) int {
	fs := newFlagSet("test")
	configPath := fs.String("config", "", "config file path")
	host := fs.String("host", "", "saved host id")
	target := fs.String("target", "", "direct target like user@host:22")
	user := fs.String("user", "", "SSH username for direct target mode")
	address := fs.String("address", "", "host or IP for direct target mode")
	port := fs.Int("port", 0, "port for direct target mode")
	password := fs.String("password", "", "inline password")
	passwordEnv := fs.String("password-env", "", "environment variable with password")
	privateKeyPath := fs.String("private-key-path", "", "private key path")
	passphrase := fs.String("passphrase", "", "inline private key passphrase")
	passphraseEnv := fs.String("passphrase-env", "", "environment variable with private key passphrase")
	hostKeyMode := fs.String("host-key-mode", "", "known_hosts or insecure_ignore")
	knownHostsPath := fs.String("known-hosts-path", "", "known_hosts path")
	saveHost := fs.String("save-host", "", "save this direct target after a successful test")
	saveName := fs.String("save-name", "", "optional display name used with --save-host")
	timeoutSec := fs.Int("timeout", 0, "connection timeout in seconds")
	dryRun := fs.Bool("dry-run", false, "resolve only without dialing")
	jsonOutput := fs.Bool("json", false, "output JSON")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	verbose := fs.Bool("verbose", false, "write debug logs to stderr")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() > 0 && strings.TrimSpace(*host) == "" {
		*host = fs.Arg(0)
	}
	if strings.TrimSpace(*host) == "" && strings.TrimSpace(*target) == "" && strings.TrimSpace(*address) == "" {
		if *jsonOutput {
			return writeEnvelope(envelope{OK: false, Kind: "test", Error: &errorPayload{Code: "invalid_request", Message: "host or target is required"}}, *pretty)
		}
		cfg, _, exists, err := loadRuntimeConfig(*configPath, newLogger(*verbose))
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取配置失败: %s\n", sshops.ErrorMessage(err))
			return 1
		}
		if !exists || len(cfg.Hosts) == 0 {
			fmt.Fprintln(os.Stderr, "还没有保存任何服务器。先运行 `sshctl add`。")
			return 1
		}
		reader := bufio.NewReader(os.Stdin)
		selected, err := chooseHost(reader, os.Stdout, cfg.Hosts, "请输入要测试的服务器序号、别名或显示名称")
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", sshops.ErrorMessage(err))
			return 1
		}
		*host = selected.ID
	}

	runtimeHost := runtimeHostOptions{
		Target:         *target,
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
		SaveHost:       *saveHost,
		SaveName:       *saveName,
	}

	logger := newLogger(*verbose)
	service, resolvedHost, cfg, resolvedPath, _, err := runtimeServiceForHost(*configPath, logger, *host, runtimeHost)
	if err != nil {
		if *jsonOutput {
			return writeEnvelope(envelope{OK: false, Kind: "test", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
		}
		fmt.Fprintf(os.Stderr, "测试连接失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}

	request := sshops.TestRequest{HostID: resolvedHost.ID, DryRun: *dryRun}
	if *timeoutSec > 0 {
		request.Timeout = time.Duration(*timeoutSec) * time.Second
	}
	result, err := service.Test(context.Background(), request)
	if err != nil {
		if *jsonOutput {
			return writeEnvelope(envelope{OK: false, Kind: "test", Host: resolvedHost.ID, Error: buildError(err, nil)}, *pretty)
		}
		fmt.Fprintf(os.Stderr, "连接失败: %s@%s:%d\n", resolvedHost.User, resolvedHost.Address, resolvedHost.Port)
		fmt.Fprintf(os.Stderr, "原因: %s\n", sshops.ErrorMessage(err))
		return 1
	}

	if !*dryRun {
		if err := maybePersistRuntimeHost(resolvedPath, cfg, resolvedHost, runtimeHost); err != nil {
			if *jsonOutput {
				return writeEnvelope(envelope{OK: false, Kind: "test", Host: resolvedHost.ID, Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
			}
			fmt.Fprintf(os.Stderr, "连接成功，但保存服务器失败: %s\n", err)
			return 1
		}
	}

	if *jsonOutput {
		return writeEnvelope(envelope{OK: true, Kind: "test", Host: result.HostID, Result: result, Meta: &metaPayload{DurationMS: result.DurationMS}}, *pretty)
	}

	if result.DryRun {
		fmt.Fprintf(os.Stdout, "已解析目标: %s@%s:%d\n", result.User, result.Address, result.Port)
		return 0
	}
	fmt.Fprintf(os.Stdout, "连接成功: %s@%s:%d (%dms)\n", result.User, result.Address, result.Port, result.DurationMS)
	return 0
}

func runRun(args []string) int {
	fs := newFlagSet("run")
	configPath := fs.String("config", "", "config file path")
	host := fs.String("host", "", "saved host id")
	target := fs.String("target", "", "direct target like user@host:22")
	user := fs.String("user", "", "SSH username for direct target mode")
	address := fs.String("address", "", "host or IP for direct target mode")
	port := fs.Int("port", 0, "port for direct target mode")
	password := fs.String("password", "", "inline password")
	passwordEnv := fs.String("password-env", "", "environment variable with password")
	privateKeyPath := fs.String("private-key-path", "", "private key path")
	passphrase := fs.String("passphrase", "", "inline private key passphrase")
	passphraseEnv := fs.String("passphrase-env", "", "environment variable with private key passphrase")
	hostKeyMode := fs.String("host-key-mode", "", "known_hosts or insecure_ignore")
	knownHostsPath := fs.String("known-hosts-path", "", "known_hosts path")
	saveHost := fs.String("save-host", "", "save this direct target after success")
	saveName := fs.String("save-name", "", "optional display name used with --save-host")
	workdir := fs.String("workdir", "", "remote working directory")
	shell := fs.String("shell", "", "remote shell: bash or sh")
	timeoutSec := fs.Int("timeout", 0, "operation timeout in seconds")
	maxOutputBytes := fs.Int64("max-output-bytes", 0, "stdout/stderr capture limit")
	readStdin := fs.Bool("stdin", false, "pipe local stdin to the remote process")
	dryRun := fs.Bool("dry-run", false, "resolve only without executing")
	jsonOutput := fs.Bool("json", false, "output JSON")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	verbose := fs.Bool("verbose", false, "write debug logs to stderr")
	var envFlags keyValueFlags
	fs.Var(&envFlags, "env", "environment variable assignment (repeatable KEY=VALUE)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	reader := bufio.NewReader(os.Stdin)
	commandText, hostID := parseRunPositionals(*host, *target, fs.Args())
	if strings.TrimSpace(hostID) != "" {
		*host = hostID
	}
	if strings.TrimSpace(*host) == "" && strings.TrimSpace(*target) == "" && strings.TrimSpace(*address) == "" {
		if *jsonOutput {
			return writeEnvelope(envelope{OK: false, Kind: "exec", Error: &errorPayload{Code: "invalid_request", Message: "host or target is required"}}, *pretty)
		}
		cfg, _, exists, err := loadRuntimeConfig(*configPath, newLogger(*verbose))
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取配置失败: %s\n", sshops.ErrorMessage(err))
			return 1
		}
		if !exists || len(cfg.Hosts) == 0 {
			fmt.Fprintln(os.Stderr, "还没有保存任何服务器。先运行 `sshctl add`。")
			return 1
		}
		selected, err := chooseHost(reader, os.Stdout, cfg.Hosts, "请输入要执行命令的服务器序号、别名或显示名称")
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", sshops.ErrorMessage(err))
			return 1
		}
		*host = selected.ID
	}
	if strings.TrimSpace(commandText) == "" {
		if *jsonOutput {
			return writeEnvelope(envelope{OK: false, Kind: "exec", Error: &errorPayload{Code: "invalid_request", Message: "command is required"}}, *pretty)
		}
		value, err := promptRequired(reader, os.Stdout, "要执行的命令", "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取命令失败: %s\n", sshops.ErrorMessage(err))
			return 1
		}
		commandText = value
	}

	if *jsonOutput {
		rebuilt := maybeAppendFlag(nil, "config", *configPath)
		rebuilt = maybeAppendFlag(rebuilt, "host", *host)
		rebuilt = maybeAppendFlag(rebuilt, "target", *target)
		rebuilt = maybeAppendFlag(rebuilt, "user", *user)
		rebuilt = maybeAppendFlag(rebuilt, "address", *address)
		rebuilt = maybeAppendIntFlag(rebuilt, "port", *port)
		rebuilt = maybeAppendFlag(rebuilt, "password", *password)
		rebuilt = maybeAppendFlag(rebuilt, "password-env", *passwordEnv)
		rebuilt = maybeAppendFlag(rebuilt, "private-key-path", *privateKeyPath)
		rebuilt = maybeAppendFlag(rebuilt, "passphrase", *passphrase)
		rebuilt = maybeAppendFlag(rebuilt, "passphrase-env", *passphraseEnv)
		rebuilt = maybeAppendFlag(rebuilt, "host-key-mode", *hostKeyMode)
		rebuilt = maybeAppendFlag(rebuilt, "known-hosts-path", *knownHostsPath)
		rebuilt = maybeAppendFlag(rebuilt, "save-host", *saveHost)
		rebuilt = maybeAppendFlag(rebuilt, "save-name", *saveName)
		rebuilt = maybeAppendFlag(rebuilt, "workdir", *workdir)
		rebuilt = maybeAppendFlag(rebuilt, "shell", *shell)
		rebuilt = maybeAppendIntFlag(rebuilt, "timeout", *timeoutSec)
		rebuilt = maybeAppendInt64Flag(rebuilt, "max-output-bytes", *maxOutputBytes)
		for _, item := range envFlags {
			rebuilt = append(rebuilt, "--env", item)
		}
		if *readStdin {
			rebuilt = append(rebuilt, "--stdin")
		}
		if *dryRun {
			rebuilt = append(rebuilt, "--dry-run")
		}
		if *pretty {
			rebuilt = append(rebuilt, "--pretty")
		}
		rebuilt = append(rebuilt, "--command", commandText)
		return runExec(rebuilt)
	}

	envMap, err := parseEnvFlags(envFlags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "命令参数无效: %s\n", sshops.ErrorMessage(err))
		return 1
	}

	logger := newLogger(*verbose)
	runtimeHost := runtimeHostOptions{
		Target:         *target,
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
		SaveHost:       *saveHost,
		SaveName:       *saveName,
	}
	service, resolvedHost, cfg, resolvedPath, _, err := runtimeServiceForHost(*configPath, logger, *host, runtimeHost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "准备连接失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}

	request := sshops.ExecRequest{
		HostID:         resolvedHost.ID,
		Command:        commandText,
		Workdir:        *workdir,
		Env:            envMap,
		Shell:          *shell,
		DryRun:         *dryRun,
		MaxOutputBytes: *maxOutputBytes,
	}
	if *timeoutSec > 0 {
		request.Timeout = time.Duration(*timeoutSec) * time.Second
	}
	if *readStdin {
		request.Stdin = os.Stdin
	}

	result, err := service.Exec(context.Background(), request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "执行失败: %s\n", sshops.ErrorMessage(err))
		return 1
	}

	if !*dryRun {
		if err := maybePersistRuntimeHost(resolvedPath, cfg, resolvedHost, runtimeHost); err != nil {
			fmt.Fprintf(os.Stderr, "执行成功，但保存服务器失败: %s\n", err)
			return 1
		}
	}

	if result.Stdout != "" {
		_, _ = io.WriteString(os.Stdout, result.Stdout)
		if !strings.HasSuffix(result.Stdout, "\n") {
			_, _ = io.WriteString(os.Stdout, "\n")
		}
	}
	if result.Stderr != "" {
		_, _ = io.WriteString(os.Stderr, result.Stderr)
		if !strings.HasSuffix(result.Stderr, "\n") {
			_, _ = io.WriteString(os.Stderr, "\n")
		}
	}
	if result.DryRun {
		fmt.Fprintf(os.Stdout, "将执行: %s@%s:%d -> %s\n", resolvedHost.User, resolvedHost.Address, resolvedHost.Port, result.Command)
		return 0
	}
	if result.ExitCode != 0 {
		fmt.Fprintf(os.Stderr, "远端命令退出码: %d\n", result.ExitCode)
		return 1
	}
	if result.Truncated {
		fmt.Fprintln(os.Stdout, "[输出已截断]")
	}
	return 0
}

type addWizardOptions struct {
	NoTest bool
}

type editWizardOptions struct {
	NoTest bool
}

func runAddWizard(in io.Reader, out io.Writer, options addWizardOptions) (sshops.HostConfig, bool, error) {
	reader := bufio.NewReader(in)
	hostID, err := promptRequired(reader, out, "给这台服务器起个名字（比如 prod / test）", "")
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	displayName, err := promptOptional(reader, out, "显示名称（可留空）", "")
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	target, err := promptRequired(reader, out, "服务器地址或 IP（可写成 user@host，也可以只写 host）", "")
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	parsedTarget, err := parseTarget(target)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	loginUser, err := promptRequired(reader, out, "登录用户", parsedTarget.User)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	defaultPort := "22"
	if parsedTarget.Port > 0 {
		defaultPort = strconv.Itoa(parsedTarget.Port)
	}
	portText, err := promptRequired(reader, out, "端口", defaultPort)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	port, convErr := strconv.Atoi(strings.TrimSpace(portText))
	if convErr != nil || port <= 0 {
		return sshops.HostConfig{}, false, sshops.NewUserError("invalid_request", "端口必须是正整数", convErr)
	}

	authMethod, err := promptChoice(reader, out, "登录方式", []string{
		"1) 私钥文件",
		"2) 密码环境变量",
		"3) 直接保存密码",
	}, "1")
	if err != nil {
		return sshops.HostConfig{}, false, err
	}

	optionsForHost := configHostOptions{
		ID:      hostID,
		Name:    displayName,
		Target:  target,
		User:    loginUser,
		Port:    port,
		Workdir: "",
	}
	visited := map[string]bool{
		"id":     true,
		"target": true,
		"user":   true,
		"name":   strings.TrimSpace(displayName) != "",
	}
	visited["port"] = true

	switch authMethod {
	case "1":
		keyPath, promptErr := promptRequired(reader, out, "私钥路径", "~/.ssh/id_ed25519")
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		optionsForHost.PrivateKeyPath = keyPath
		visited["private-key-path"] = true

		passphraseEnv, promptErr := promptOptional(reader, out, "私钥口令环境变量（可留空）", "")
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		if strings.TrimSpace(passphraseEnv) != "" {
			optionsForHost.PassphraseEnv = passphraseEnv
			visited["passphrase-env"] = true
		}
	case "2":
		passwordEnv, promptErr := promptRequired(reader, out, "密码环境变量名", "SSH_OPS_PASSWORD")
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		optionsForHost.PasswordEnv = passwordEnv
		visited["password-env"] = true
	case "3":
		passwordValue, promptErr := promptRequired(reader, out, "密码（会保存到本地配置文件）", "")
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		optionsForHost.Password = passwordValue
		visited["password"] = true
	default:
		return sshops.HostConfig{}, false, sshops.NewUserError("invalid_request", "不支持的登录方式", nil)
	}

	hostKeyMode, err := promptChoice(reader, out, "Host Key 校验方式", []string{
		"1) known_hosts（推荐）",
		"2) insecure_ignore（跳过校验）",
	}, "1")
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	if hostKeyMode == "1" {
		optionsForHost.HostKeyMode = "known_hosts"
		visited["host-key-mode"] = true
		knownHosts, promptErr := promptOptional(reader, out, "known_hosts 路径", "~/.ssh/known_hosts")
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		if strings.TrimSpace(knownHosts) != "" {
			optionsForHost.KnownHostsPath = knownHosts
			visited["known-hosts-path"] = true
		}
	} else {
		optionsForHost.HostKeyMode = "insecure_ignore"
		visited["host-key-mode"] = true
	}

	workdir, err := promptOptional(reader, out, "默认工作目录（可留空）", "")
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	if strings.TrimSpace(workdir) != "" {
		optionsForHost.Workdir = workdir
		visited["workdir"] = true
	}

	host, err := buildHostConfig(nil, optionsForHost, visited)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}

	testAfterSave := false
	if !options.NoTest {
		answer, promptErr := promptOptional(reader, out, "保存后立即测试连接？[Y/n]", "Y")
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		testAfterSave = normalizeYesNo(answer, true)
	}
	return host, testAfterSave, nil
}

func runEditWizard(reader *bufio.Reader, out io.Writer, host sshops.HostConfig, options editWizardOptions) (sshops.HostConfig, bool, error) {
	fmt.Fprintf(out, "正在编辑 %s。\n\n", host.ID)

	newID, changed, err := promptEditable(reader, out, "服务器名字", host.ID, false)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	if changed {
		host.ID = newID
	}

	displayName, changed, err := promptEditable(reader, out, "显示名称", host.Name, true)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	if changed {
		host.Name = displayName
	}

	address, changed, err := promptEditable(reader, out, "服务器地址或 IP", host.Address, false)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	if changed {
		host.Address = address
	}

	user, changed, err := promptEditable(reader, out, "登录用户", host.User, false)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	if changed {
		host.User = user
	}

	portText, changed, err := promptEditable(reader, out, "端口", strconv.Itoa(host.Port), false)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	if changed {
		port, convErr := strconv.Atoi(strings.TrimSpace(portText))
		if convErr != nil || port <= 0 {
			return sshops.HostConfig{}, false, sshops.NewUserError("invalid_request", "端口必须是正整数", convErr)
		}
		host.Port = port
	}

	currentMethod := "1"
	if host.Auth.PasswordEnv != "" {
		currentMethod = "2"
	}
	if host.Auth.Password != "" {
		currentMethod = "3"
	}
	authMethod, err := promptChoice(reader, out, "登录方式", []string{
		"1) 私钥文件",
		"2) 密码环境变量",
		"3) 直接保存密码",
	}, currentMethod)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}

	switch authMethod {
	case "1":
		keyPath, _, promptErr := promptEditable(reader, out, "私钥路径", host.Auth.PrivateKeyPath, false)
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		host.Auth.PrivateKeyPath = keyPath
		host.Auth.Password = ""
		host.Auth.PasswordEnv = ""
		host.Auth.PrivateKey = ""

		passphraseEnv, _, promptErr := promptEditable(reader, out, "私钥口令环境变量", host.Auth.PassphraseEnv, true)
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		host.Auth.PassphraseEnv = passphraseEnv
		host.Auth.Passphrase = ""
	case "2":
		passwordEnv, _, promptErr := promptEditable(reader, out, "密码环境变量名", host.Auth.PasswordEnv, false)
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		host.Auth.PasswordEnv = passwordEnv
		host.Auth.Password = ""
		host.Auth.PrivateKey = ""
		host.Auth.PrivateKeyPath = ""
		host.Auth.Passphrase = ""
		host.Auth.PassphraseEnv = ""
	case "3":
		passwordValue, _, promptErr := promptEditable(reader, out, "密码", host.Auth.Password, false)
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		host.Auth.Password = passwordValue
		host.Auth.PasswordEnv = ""
		host.Auth.PrivateKey = ""
		host.Auth.PrivateKeyPath = ""
		host.Auth.Passphrase = ""
		host.Auth.PassphraseEnv = ""
	default:
		return sshops.HostConfig{}, false, sshops.NewUserError("invalid_request", "不支持的登录方式", nil)
	}

	hostKeyModeDefault := "1"
	if host.HostKey.Mode == "insecure_ignore" {
		hostKeyModeDefault = "2"
	}
	hostKeyMode, err := promptChoice(reader, out, "Host Key 校验方式", []string{
		"1) known_hosts（推荐）",
		"2) insecure_ignore（跳过校验）",
	}, hostKeyModeDefault)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	if hostKeyMode == "1" {
		host.HostKey.Mode = "known_hosts"
		knownHosts, _, promptErr := promptEditable(reader, out, "known_hosts 路径", host.HostKey.KnownHostsPath, false)
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		host.HostKey.KnownHostsPath = knownHosts
	} else {
		host.HostKey.Mode = "insecure_ignore"
		host.HostKey.KnownHostsPath = ""
	}

	workdir, changed, err := promptEditable(reader, out, "默认工作目录", host.Defaults.Workdir, true)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	if changed {
		host.Defaults.Workdir = workdir
	}

	shell, changed, err := promptEditable(reader, out, "默认 shell", host.Defaults.Shell, true)
	if err != nil {
		return sshops.HostConfig{}, false, err
	}
	if changed {
		host.Defaults.Shell = shell
	}

	validator := sshops.DefaultConfig()
	validator.Hosts = []sshops.HostConfig{host}
	if err := validator.Normalize(); err != nil {
		return sshops.HostConfig{}, false, err
	}
	host = validator.Hosts[0]

	testAfterSave := false
	if !options.NoTest {
		answer, promptErr := promptOptional(reader, out, "保存后立即测试连接？[Y/n]", "Y")
		if promptErr != nil {
			return sshops.HostConfig{}, false, promptErr
		}
		testAfterSave = normalizeYesNo(answer, true)
	}
	return host, testAfterSave, nil
}

func promptRequired(reader *bufio.Reader, out io.Writer, label, defaultValue string) (string, error) {
	for {
		value, err := promptOptional(reader, out, label, defaultValue)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
		fmt.Fprintln(out, "这个字段不能为空。")
	}
}

func promptOptional(reader *bufio.Reader, out io.Writer, label, defaultValue string) (string, error) {
	if strings.TrimSpace(defaultValue) != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}

func promptChoice(reader *bufio.Reader, out io.Writer, label string, choices []string, defaultValue string) (string, error) {
	fmt.Fprintln(out, label)
	for _, choice := range choices {
		fmt.Fprintf(out, "  %s\n", choice)
	}
	allowed := make(map[string]struct{}, len(choices))
	for _, choice := range choices {
		token := strings.TrimSpace(choice)
		if before, _, found := strings.Cut(token, ")"); found {
			token = strings.TrimSpace(before)
		}
		allowed[token] = struct{}{}
	}
	for {
		value, err := promptOptional(reader, out, "请选择", defaultValue)
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(value)
		if _, ok := allowed[value]; ok {
			return value, nil
		}
		fmt.Fprintln(out, "请输入列表里的序号。")
	}
}

func normalizeYesNo(value string, defaultValue bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return defaultValue
	}
}

func parseRunPositionals(hostFlag, targetFlag string, args []string) (string, string) {
	if len(args) == 0 {
		return "", strings.TrimSpace(hostFlag)
	}
	if strings.TrimSpace(targetFlag) != "" || strings.TrimSpace(hostFlag) != "" {
		return strings.Join(args, " "), strings.TrimSpace(hostFlag)
	}
	if len(args) == 1 {
		return args[0], ""
	}
	return strings.Join(args[1:], " "), strings.TrimSpace(args[0])
}

func maybeAppendFlag(args []string, name, value string) []string {
	if strings.TrimSpace(value) == "" {
		return args
	}
	return append(args, "--"+name, value)
}

func maybeAppendIntFlag(args []string, name string, value int) []string {
	if value == 0 {
		return args
	}
	return append(args, "--"+name, strconv.Itoa(value))
}

func maybeAppendInt64Flag(args []string, name string, value int64) []string {
	if value == 0 {
		return args
	}
	return append(args, "--"+name, strconv.FormatInt(value, 10))
}
