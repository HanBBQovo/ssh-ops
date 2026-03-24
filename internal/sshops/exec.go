package sshops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type ExecRequest struct {
	HostID         string
	Command        string
	Workdir        string
	Env            map[string]string
	Timeout        time.Duration
	Shell          string
	Stdin          io.Reader
	DryRun         bool
	MaxOutputBytes int64
}

type ExecResult struct {
	HostID     string `json:"host"`
	Command    string `json:"command"`
	Workdir    string `json:"workdir,omitempty"`
	Shell      string `json:"shell"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	DryRun     bool   `json:"dry_run,omitempty"`
}

func (s *Service) Exec(ctx context.Context, req ExecRequest) (ExecResult, error) {
	host, err := s.Host(req.HostID)
	if err != nil {
		return ExecResult{}, err
	}
	if err := s.policy.Check(req.Command); err != nil {
		return ExecResult{}, err
	}

	workdir := resolveWorkdir(req.Workdir, host)
	shell := resolveShell(req.Shell, host, s.cfg)
	if shell == "" {
		return ExecResult{}, NewUserError("invalid_request", "unsupported shell", fmt.Errorf("%q", req.Shell))
	}

	script, err := buildShellScript(req.Command, workdir, req.Env)
	if err != nil {
		return ExecResult{}, err
	}

	if req.DryRun {
		return ExecResult{
			HostID:   host.ID,
			Command:  req.Command,
			Workdir:  workdir,
			Shell:    shell,
			ExitCode: 0,
			DryRun:   true,
		}, nil
	}

	runCtx, cancel := s.operationContext(ctx, req.Timeout)
	defer cancel()

	limit := req.MaxOutputBytes
	if limit <= 0 {
		limit = s.cfg.Defaults.MaxOutputBytes
	}

	startedAt := time.Now()
	client, err := dialSSH(host, s.cfg.Defaults.ConnectTimeoutSec)
	if err != nil {
		return ExecResult{}, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return ExecResult{}, NewUserError("ssh_session_failed", "failed to create SSH session", err)
	}
	defer session.Close()

	if req.Stdin != nil {
		session.Stdin = req.Stdin
	}

	stdoutBuf := newLimitedBuffer(limit)
	stderrBuf := newLimitedBuffer(limit)
	session.Stdout = stdoutBuf
	session.Stderr = stderrBuf

	command := shell + " -lc " + shellEscape(script)
	errCh := make(chan error, 1)
	go func() {
		errCh <- session.Run(command)
	}()

	var runErr error
	select {
	case <-runCtx.Done():
		_ = session.Close()
		return ExecResult{}, NewUserError("timeout", "remote command timed out", runCtx.Err())
	case runErr = <-errCh:
	}

	result := ExecResult{
		HostID:     host.ID,
		Command:    req.Command,
		Workdir:    workdir,
		Shell:      shell,
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		Truncated:  stdoutBuf.Truncated() || stderrBuf.Truncated(),
		DurationMS: time.Since(startedAt).Milliseconds(),
	}

	if runErr == nil {
		return result, nil
	}

	var exitErr *ssh.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitStatus()
		return result, nil
	}

	return ExecResult{}, NewUserError("ssh_exec_failed", "failed to execute remote command", runErr)
}

func buildShellScript(command, workdir string, env map[string]string) (string, error) {
	var lines []string
	if workdir != "" {
		lines = append(lines, "cd "+shellEscape(workdir))
	}
	for key, value := range env {
		if !isValidEnvKey(key) {
			return "", NewUserError("invalid_request", "invalid environment variable name", fmt.Errorf("%q", key))
		}
		lines = append(lines, "export "+key+"="+shellEscape(value))
	}
	lines = append(lines, strings.TrimSpace(command))
	return strings.Join(lines, "\n"), nil
}

func resolveWorkdir(workdir string, host *HostConfig) string {
	if strings.TrimSpace(workdir) != "" {
		return workdir
	}
	return host.Defaults.Workdir
}

func resolveShell(shell string, host *HostConfig, cfg *Config) string {
	if normalized := normalizeShell(shell); normalized != "" {
		return normalized
	}
	if host != nil {
		if normalized := normalizeShell(host.Defaults.Shell); normalized != "" {
			return normalized
		}
	}
	return normalizeShell(cfg.Defaults.Shell)
}
