package sshops

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"
)

type HostSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	User    string `json:"user"`
	Port    int    `json:"port"`
}

type Service struct {
	cfg    *Config
	policy *CommandPolicy
	hosts  map[string]*HostConfig
	logger *log.Logger
}

func LoadService(configPath string, logger *log.Logger) (*Service, string, error) {
	resolvedPath := ResolveConfigPath(configPath)
	cfg, err := LoadConfig(resolvedPath)
	if err != nil {
		return nil, resolvedPath, err
	}
	service, err := NewService(cfg, logger)
	if err != nil {
		return nil, resolvedPath, err
	}
	return service, resolvedPath, nil
}

func NewService(cfg *Config, logger *log.Logger) (*Service, error) {
	policy, err := NewCommandPolicy(cfg.Policy)
	if err != nil {
		return nil, err
	}

	hosts := make(map[string]*HostConfig, len(cfg.Hosts))
	for i := range cfg.Hosts {
		host := &cfg.Hosts[i]
		hosts[host.ID] = host
	}

	return &Service{
		cfg:    cfg,
		policy: policy,
		hosts:  hosts,
		logger: logger,
	}, nil
}

func (s *Service) Config() *Config {
	return s.cfg
}

func (s *Service) Host(id string) (*HostConfig, error) {
	if host, ok := s.hosts[id]; ok {
		return host, nil
	}
	return nil, NewUserError("unknown_host", "unknown host id", fmt.Errorf("%q", id))
}

func (s *Service) ListHosts() []HostSummary {
	ids := make([]string, 0, len(s.hosts))
	for id := range s.hosts {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]HostSummary, 0, len(ids))
	for _, id := range ids {
		host := s.hosts[id]
		out = append(out, HostSummary{
			ID:      host.ID,
			Name:    host.Name,
			Address: host.Address,
			User:    host.User,
			Port:    host.Port,
		})
	}
	return out
}

func (s *Service) operationContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = time.Duration(s.cfg.Defaults.OperationTimeoutSec) * time.Second
	}
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}
