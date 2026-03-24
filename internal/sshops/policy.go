package sshops

import (
	"fmt"
	"regexp"
	"strings"
)

var defaultDenyPatterns = []string{
	`(?i)\brm\s+-rf\s+/(?:\s|$)`,
	`(?i)\brm\s+-rf\s+\*`,
	`(?i)\brm\s+-rf\s+--no-preserve-root`,
	`(?i)\bmkfs(\.[a-z0-9]+)?\b`,
	`(?i)\bdd\b.*\bof=/dev/`,
	`(?i)\bshutdown\b|\breboot\b|\bpoweroff\b|\bhalt\b`,
	`(?i):\s*\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;`,
}

type CommandPolicy struct {
	allow []*regexp.Regexp
	deny  []*regexp.Regexp
}

func NewCommandPolicy(cfg PolicyConfig) (*CommandPolicy, error) {
	policy := &CommandPolicy{}

	for _, pattern := range cfg.AllowPatterns {
		rx, err := regexp.Compile(pattern)
		if err != nil {
			return nil, NewUserError("config_invalid", "invalid allow pattern", fmt.Errorf("%q: %w", pattern, err))
		}
		policy.allow = append(policy.allow, rx)
	}

	denyPatterns := append([]string{}, defaultDenyPatterns...)
	denyPatterns = append(denyPatterns, cfg.DenyPatterns...)
	for _, pattern := range denyPatterns {
		rx, err := regexp.Compile(pattern)
		if err != nil {
			return nil, NewUserError("config_invalid", "invalid deny pattern", fmt.Errorf("%q: %w", pattern, err))
		}
		policy.deny = append(policy.deny, rx)
	}

	return policy, nil
}

func (p *CommandPolicy) Check(command string) error {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return NewUserError("invalid_request", "command is required", nil)
	}

	if len(p.allow) > 0 {
		allowed := false
		for _, rx := range p.allow {
			if rx.MatchString(cmd) {
				allowed = true
				break
			}
		}
		if !allowed {
			return NewUserError("policy_denied", "command blocked by allowlist policy", nil)
		}
	}

	for _, rx := range p.deny {
		if rx.MatchString(cmd) {
			return NewUserError("policy_denied", "command blocked by denylist policy", nil)
		}
	}
	return nil
}
