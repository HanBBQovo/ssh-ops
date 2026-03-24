package sshops

import (
	"context"
	"time"
)

type TestRequest struct {
	HostID  string
	Timeout time.Duration
	DryRun  bool
}

type TestResult struct {
	HostID     string `json:"host"`
	Address    string `json:"address"`
	User       string `json:"user"`
	Port       int    `json:"port"`
	Connected  bool   `json:"connected"`
	DurationMS int64  `json:"duration_ms"`
	DryRun     bool   `json:"dry_run,omitempty"`
}

func (s *Service) Test(ctx context.Context, req TestRequest) (TestResult, error) {
	host, err := s.Host(req.HostID)
	if err != nil {
		return TestResult{}, err
	}

	result := TestResult{
		HostID:    host.ID,
		Address:   host.Address,
		User:      host.User,
		Port:      host.Port,
		Connected: false,
		DryRun:    req.DryRun,
	}
	if req.DryRun {
		return result, nil
	}

	timeoutSec := s.cfg.Defaults.ConnectTimeoutSec
	if req.Timeout > 0 {
		timeoutSec = int(req.Timeout / time.Second)
	}
	if timeoutSec <= 0 {
		timeoutSec = 10
	}

	startedAt := time.Now()
	client, err := dialSSH(host, timeoutSec)
	if err != nil {
		return result, err
	}
	_ = client.Close()

	select {
	case <-ctx.Done():
		return TestResult{}, NewUserError("timeout", "connection test timed out", ctx.Err())
	default:
	}

	result.Connected = true
	result.DurationMS = time.Since(startedAt).Milliseconds()
	return result, nil
}
