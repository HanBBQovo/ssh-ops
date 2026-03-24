package sshops

import "testing"

func TestPolicyDeniesDangerousCommand(t *testing.T) {
	policy, err := NewCommandPolicy(PolicyConfig{})
	if err != nil {
		t.Fatalf("NewCommandPolicy() error = %v", err)
	}

	if err := policy.Check("rm -rf /"); err == nil {
		t.Fatal("expected denylist error")
	}
}

func TestPolicyAllowlistRestrictsCommands(t *testing.T) {
	policy, err := NewCommandPolicy(PolicyConfig{
		AllowPatterns: []string{`^uptime$`},
	})
	if err != nil {
		t.Fatalf("NewCommandPolicy() error = %v", err)
	}

	if err := policy.Check("uptime"); err != nil {
		t.Fatalf("expected uptime to be allowed, got %v", err)
	}
	if err := policy.Check("hostname"); err == nil {
		t.Fatal("expected hostname to be blocked by allowlist")
	}
}
