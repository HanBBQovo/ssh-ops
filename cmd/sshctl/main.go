package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hanhan/ssh-ops/internal/sshops"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

type metaPayload struct {
	DurationMS int64 `json:"duration_ms,omitempty"`
	ExitCode   int   `json:"exit_code,omitempty"`
	Truncated  bool  `json:"truncated,omitempty"`
}

type errorPayload struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type envelope struct {
	OK      bool          `json:"ok"`
	Kind    string        `json:"kind"`
	Host    string        `json:"host,omitempty"`
	Request interface{}   `json:"request,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *errorPayload `json:"error,omitempty"`
	Meta    *metaPayload  `json:"meta,omitempty"`
}

type keyValueFlags []string

func (f *keyValueFlags) String() string {
	return strings.Join(*f, ",")
}

func (f *keyValueFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return 2
	}

	switch args[0] {
	case "add":
		return runAdd(args[1:])
	case "edit":
		return runEdit(args[1:])
	case "config":
		return runConfig(args[1:])
	case "host":
		return runHost(args[1:])
	case "list":
		return runList(args[1:])
	case "remove", "rm":
		return runRemove(args[1:])
	case "show":
		return runShow(args[1:])
	case "test":
		return runTest(args[1:])
	case "update":
		return runUpdate(args[1:])
	case "run":
		return runRun(args[1:])
	case "list-hosts":
		return runListHosts(args[1:])
	case "exec":
		return runExec(args[1:])
	case "upload":
		return runUpload(args[1:])
	case "download":
		return runDownload(args[1:])
	case "validate-config":
		return runValidateConfig(args[1:])
	case "version":
		return runVersion(args[1:])
	case "-h", "--help", "help":
		printUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", args[0])
		printUsage(os.Stderr)
		return 2
	}
}

func runListHosts(args []string) int {
	fs := newFlagSet("list-hosts")
	configPath := fs.String("config", "", "config file path")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	verbose := fs.Bool("verbose", false, "write debug logs to stderr")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	logger := newLogger(*verbose)
	startedAt := time.Now()
	cfg, resolvedPath, exists, err := loadRuntimeConfig(*configPath, logger)
	if err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "list-hosts",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}
	service, err := sshops.NewService(cfg, logger)
	if err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "list-hosts",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	return writeEnvelope(envelope{
		OK:   true,
		Kind: "list-hosts",
		Result: map[string]interface{}{
			"config_path": resolvedPath,
			"exists":      exists,
			"hosts":       service.ListHosts(),
		},
		Meta: &metaPayload{DurationMS: time.Since(startedAt).Milliseconds()},
	}, *pretty)
}

func runValidateConfig(args []string) int {
	fs := newFlagSet("validate-config")
	configPath := fs.String("config", "", "config file path")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedPath := sshops.ResolveConfigPath(*configPath)
	cfg, exists, err := sshops.LoadConfigFileOrDefault(resolvedPath)
	if err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "validate-config",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}
	if err := cfg.Normalize(); err != nil {
		return writeEnvelope(envelope{
			OK:    false,
			Kind:  "validate-config",
			Error: buildError(err, map[string]interface{}{"config_path": resolvedPath}),
		}, *pretty)
	}

	report := sshops.ValidateConfig(cfg, resolvedPath)
	if !exists {
		report.Warnings = append([]string{"config file does not exist yet; showing defaults"}, report.Warnings...)
	}
	env := envelope{
		OK:     report.OK,
		Kind:   "validate-config",
		Result: report,
	}
	if !report.OK {
		env.Error = &errorPayload{
			Code:    "config_invalid",
			Message: "config validation failed",
		}
	}
	return writeEnvelope(env, *pretty)
}

func runExec(args []string) int {
	fs := newFlagSet("exec")
	configPath := fs.String("config", "", "config file path")
	host := fs.String("host", "", "host id")
	command := fs.String("command", "", "remote shell command")
	workdir := fs.String("workdir", "", "remote working directory")
	timeoutSec := fs.Int("timeout", 0, "operation timeout in seconds")
	shell := fs.String("shell", "", "remote shell to use (bash or sh)")
	maxOutputBytes := fs.Int64("max-output-bytes", 0, "stdout/stderr capture limit")
	readStdin := fs.Bool("stdin", false, "pipe local stdin to the remote process")
	target := fs.String("target", "", "ad-hoc target like user@example.com:22")
	user := fs.String("user", "", "SSH username for ad-hoc target mode")
	address := fs.String("address", "", "host or IP for ad-hoc target mode")
	port := fs.Int("port", 0, "port for ad-hoc target mode")
	password := fs.String("password", "", "inline password for ad-hoc target mode")
	passwordEnv := fs.String("password-env", "", "environment variable with password for ad-hoc target mode")
	privateKeyPath := fs.String("private-key-path", "", "private key path for ad-hoc target mode")
	passphrase := fs.String("passphrase", "", "inline private key passphrase")
	passphraseEnv := fs.String("passphrase-env", "", "environment variable with private key passphrase")
	hostKeyMode := fs.String("host-key-mode", "", "known_hosts or insecure_ignore for ad-hoc target mode")
	knownHostsPath := fs.String("known-hosts-path", "", "known_hosts path for ad-hoc target mode")
	saveHost := fs.String("save-host", "", "save the ad-hoc target as a reusable host id after success")
	saveName := fs.String("save-name", "", "optional display name used with --save-host")
	dryRun := fs.Bool("dry-run", false, "print the resolved action without executing")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	verbose := fs.Bool("verbose", false, "write debug logs to stderr")
	var envFlags keyValueFlags
	fs.Var(&envFlags, "env", "environment variable assignment (repeatable KEY=VALUE)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*host) == "" && strings.TrimSpace(*target) == "" && strings.TrimSpace(*address) == "" {
		return writeEnvelope(envelope{OK: false, Kind: "exec", Error: &errorPayload{Code: "invalid_request", Message: "host or target is required"}}, *pretty)
	}
	if strings.TrimSpace(*command) == "" {
		return writeEnvelope(envelope{OK: false, Kind: "exec", Error: &errorPayload{Code: "invalid_request", Message: "command is required"}}, *pretty)
	}

	envMap, err := parseEnvFlags(envFlags)
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "exec", Error: buildError(err, nil)}, *pretty)
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
	service, resolvedHost, cfg, resolvedPath, cfgExists, err := runtimeServiceForHost(*configPath, logger, *host, runtimeHost)
	if err != nil {
		return writeEnvelope(envelope{OK: false, Kind: "exec", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath, "config_exists": cfgExists})}, *pretty)
	}

	request := sshops.ExecRequest{
		HostID:         resolvedHost.ID,
		Command:        *command,
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
		return writeEnvelope(envelope{
			OK:      false,
			Kind:    "exec",
			Host:    resolvedHost.ID,
			Request: buildExecRequestPayload(request, *readStdin),
			Error:   buildError(err, nil),
		}, *pretty)
	}

	if !*dryRun {
		if err := maybePersistRuntimeHost(resolvedPath, cfg, resolvedHost, runtimeHost); err != nil {
			return writeEnvelope(envelope{
				OK:      false,
				Kind:    "exec",
				Host:    result.HostID,
				Request: buildExecRequestPayload(request, *readStdin),
				Error:   buildError(err, map[string]interface{}{"config_path": resolvedPath}),
			}, *pretty)
		}
	}

	env := envelope{
		OK:      result.ExitCode == 0,
		Kind:    "exec",
		Host:    result.HostID,
		Request: buildExecRequestPayload(request, *readStdin),
		Result:  result,
		Meta: &metaPayload{
			DurationMS: result.DurationMS,
			ExitCode:   result.ExitCode,
			Truncated:  result.Truncated,
		},
	}
	if result.ExitCode != 0 {
		env.Error = &errorPayload{
			Code:    "remote_exit_nonzero",
			Message: "remote command exited with a non-zero status",
		}
	}
	return writeEnvelope(env, *pretty)
}

func runUpload(args []string) int {
	fs := newFlagSet("upload")
	configPath := fs.String("config", "", "config file path")
	host := fs.String("host", "", "host id")
	localPath := fs.String("local", "", "local file or directory")
	remotePath := fs.String("remote", "", "remote destination path")
	timeoutSec := fs.Int("timeout", 0, "operation timeout in seconds")
	overwrite := fs.Bool("overwrite", false, "overwrite existing remote files")
	preserveMode := fs.Bool("preserve-mode", false, "preserve file modes")
	target := fs.String("target", "", "ad-hoc target like user@example.com:22")
	user := fs.String("user", "", "SSH username for ad-hoc target mode")
	address := fs.String("address", "", "host or IP for ad-hoc target mode")
	port := fs.Int("port", 0, "port for ad-hoc target mode")
	password := fs.String("password", "", "inline password for ad-hoc target mode")
	passwordEnv := fs.String("password-env", "", "environment variable with password for ad-hoc target mode")
	privateKeyPath := fs.String("private-key-path", "", "private key path for ad-hoc target mode")
	passphrase := fs.String("passphrase", "", "inline private key passphrase")
	passphraseEnv := fs.String("passphrase-env", "", "environment variable with private key passphrase")
	hostKeyMode := fs.String("host-key-mode", "", "known_hosts or insecure_ignore for ad-hoc target mode")
	knownHostsPath := fs.String("known-hosts-path", "", "known_hosts path for ad-hoc target mode")
	saveHost := fs.String("save-host", "", "save the ad-hoc target as a reusable host id after success")
	saveName := fs.String("save-name", "", "optional display name used with --save-host")
	dryRun := fs.Bool("dry-run", false, "print the resolved action without executing")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	verbose := fs.Bool("verbose", false, "write debug logs to stderr")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if (strings.TrimSpace(*host) == "" && strings.TrimSpace(*target) == "" && strings.TrimSpace(*address) == "") || strings.TrimSpace(*localPath) == "" || strings.TrimSpace(*remotePath) == "" {
		return writeEnvelope(envelope{
			OK:   false,
			Kind: "upload",
			Error: &errorPayload{
				Code:    "invalid_request",
				Message: "host or target, local, and remote are required",
			},
		}, *pretty)
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
		return writeEnvelope(envelope{OK: false, Kind: "upload", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
	}

	request := sshops.UploadRequest{
		HostID:       resolvedHost.ID,
		LocalPath:    *localPath,
		RemotePath:   *remotePath,
		Overwrite:    *overwrite,
		PreserveMode: *preserveMode,
		DryRun:       *dryRun,
	}
	if *timeoutSec > 0 {
		request.Timeout = time.Duration(*timeoutSec) * time.Second
	}

	result, err := service.Upload(context.Background(), request)
	if err != nil {
		return writeEnvelope(envelope{
			OK:      false,
			Kind:    "upload",
			Host:    resolvedHost.ID,
			Request: request,
			Error:   buildError(err, nil),
		}, *pretty)
	}

	if !*dryRun {
		if err := maybePersistRuntimeHost(resolvedPath, cfg, resolvedHost, runtimeHost); err != nil {
			return writeEnvelope(envelope{
				OK:      false,
				Kind:    "upload",
				Host:    result.HostID,
				Request: request,
				Error:   buildError(err, map[string]interface{}{"config_path": resolvedPath}),
			}, *pretty)
		}
	}

	return writeEnvelope(envelope{
		OK:      true,
		Kind:    "upload",
		Host:    result.HostID,
		Request: request,
		Result:  result,
		Meta:    &metaPayload{DurationMS: result.DurationMS},
	}, *pretty)
}

func runDownload(args []string) int {
	fs := newFlagSet("download")
	configPath := fs.String("config", "", "config file path")
	host := fs.String("host", "", "host id")
	remotePath := fs.String("remote", "", "remote file or directory")
	localPath := fs.String("local", "", "local destination path")
	timeoutSec := fs.Int("timeout", 0, "operation timeout in seconds")
	overwrite := fs.Bool("overwrite", false, "overwrite existing local files")
	preserveMode := fs.Bool("preserve-mode", false, "preserve file modes")
	target := fs.String("target", "", "ad-hoc target like user@example.com:22")
	user := fs.String("user", "", "SSH username for ad-hoc target mode")
	address := fs.String("address", "", "host or IP for ad-hoc target mode")
	port := fs.Int("port", 0, "port for ad-hoc target mode")
	password := fs.String("password", "", "inline password for ad-hoc target mode")
	passwordEnv := fs.String("password-env", "", "environment variable with password for ad-hoc target mode")
	privateKeyPath := fs.String("private-key-path", "", "private key path for ad-hoc target mode")
	passphrase := fs.String("passphrase", "", "inline private key passphrase")
	passphraseEnv := fs.String("passphrase-env", "", "environment variable with private key passphrase")
	hostKeyMode := fs.String("host-key-mode", "", "known_hosts or insecure_ignore for ad-hoc target mode")
	knownHostsPath := fs.String("known-hosts-path", "", "known_hosts path for ad-hoc target mode")
	saveHost := fs.String("save-host", "", "save the ad-hoc target as a reusable host id after success")
	saveName := fs.String("save-name", "", "optional display name used with --save-host")
	dryRun := fs.Bool("dry-run", false, "print the resolved action without executing")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	verbose := fs.Bool("verbose", false, "write debug logs to stderr")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if (strings.TrimSpace(*host) == "" && strings.TrimSpace(*target) == "" && strings.TrimSpace(*address) == "") || strings.TrimSpace(*localPath) == "" || strings.TrimSpace(*remotePath) == "" {
		return writeEnvelope(envelope{
			OK:   false,
			Kind: "download",
			Error: &errorPayload{
				Code:    "invalid_request",
				Message: "host or target, remote, and local are required",
			},
		}, *pretty)
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
		return writeEnvelope(envelope{OK: false, Kind: "download", Error: buildError(err, map[string]interface{}{"config_path": resolvedPath})}, *pretty)
	}

	request := sshops.DownloadRequest{
		HostID:       resolvedHost.ID,
		RemotePath:   *remotePath,
		LocalPath:    *localPath,
		Overwrite:    *overwrite,
		PreserveMode: *preserveMode,
		DryRun:       *dryRun,
	}
	if *timeoutSec > 0 {
		request.Timeout = time.Duration(*timeoutSec) * time.Second
	}

	result, err := service.Download(context.Background(), request)
	if err != nil {
		return writeEnvelope(envelope{
			OK:      false,
			Kind:    "download",
			Host:    resolvedHost.ID,
			Request: request,
			Error:   buildError(err, nil),
		}, *pretty)
	}

	if !*dryRun {
		if err := maybePersistRuntimeHost(resolvedPath, cfg, resolvedHost, runtimeHost); err != nil {
			return writeEnvelope(envelope{
				OK:      false,
				Kind:    "download",
				Host:    result.HostID,
				Request: request,
				Error:   buildError(err, map[string]interface{}{"config_path": resolvedPath}),
			}, *pretty)
		}
	}

	return writeEnvelope(envelope{
		OK:      true,
		Kind:    "download",
		Host:    result.HostID,
		Request: request,
		Result:  result,
		Meta:    &metaPayload{DurationMS: result.DurationMS},
	}, *pretty)
}

func runVersion(args []string) int {
	fs := newFlagSet("version")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	return writeEnvelope(envelope{
		OK:   true,
		Kind: "version",
		Result: map[string]string{
			"version": version,
			"commit":  commit,
			"date":    date,
		},
	}, *pretty)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	return fs
}

func parseEnvFlags(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	result := make(map[string]string, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, sshops.NewUserError("invalid_request", "env flags must use KEY=VALUE syntax", fmt.Errorf("%q", value))
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

func buildExecRequestPayload(request sshops.ExecRequest, stdin bool) map[string]interface{} {
	payload := map[string]interface{}{
		"command":          request.Command,
		"workdir":          request.Workdir,
		"env":              request.Env,
		"shell":            request.Shell,
		"dry_run":          request.DryRun,
		"stdin":            stdin,
		"max_output_bytes": request.MaxOutputBytes,
	}
	if request.Timeout > 0 {
		payload["timeout_sec"] = int(request.Timeout / time.Second)
	}
	return payload
}

func buildError(err error, details interface{}) *errorPayload {
	return &errorPayload{
		Code:    sshops.ErrorCode(err),
		Message: sshops.ErrorMessage(err),
		Details: details,
	}
}

func writeEnvelope(env envelope, pretty bool) int {
	encoder := json.NewEncoder(os.Stdout)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(env); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode response: %v\n", err)
		return 1
	}
	if env.OK {
		return 0
	}
	return 1
}

func newLogger(verbose bool) *log.Logger {
	if !verbose {
		return log.New(io.Discard, "", 0)
	}
	return log.New(os.Stderr, "sshctl: ", log.LstdFlags)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "sshctl 是一个面向 Agent CLI 的本地 SSH / SFTP 工具，不再依赖 MCP。")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "用法：")
	fmt.Fprintln(w, "  sshctl <subcommand> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "最常用的命令：")
	fmt.Fprintln(w, "  add              交互式添加一台服务器")
	fmt.Fprintln(w, "  edit             从列表里选一台服务器进行编辑")
	fmt.Fprintln(w, "  list             列出已保存的服务器")
	fmt.Fprintln(w, "  remove           从列表里选一台服务器并删除")
	fmt.Fprintln(w, "  show             查看某一台服务器")
	fmt.Fprintln(w, "  test             测试一台服务器能不能连通")
	fmt.Fprintln(w, "  update           查看或执行更新命令")
	fmt.Fprintln(w, "  run              在一台服务器上执行命令")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "进阶子命令：")
	fmt.Fprintln(w, "  config           管理本地 ssh-ops 配置文件")
	fmt.Fprintln(w, "  host             用更短的参数化命令管理常用服务器")
	fmt.Fprintln(w, "  list-hosts       以 JSON 列出所有服务器")
	fmt.Fprintln(w, "  exec             执行远端命令")
	fmt.Fprintln(w, "  upload           通过 SFTP 上传文件或目录")
	fmt.Fprintln(w, "  download         通过 SFTP 下载文件或目录")
	fmt.Fprintln(w, "  validate-config  检查配置文件是否可用")
	fmt.Fprintln(w, "  version          查看版本信息")
}
