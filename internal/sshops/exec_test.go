package sshops

import (
	"strings"
	"testing"
)

func TestBuildShellScriptIncludesWorkdirAndEnv(t *testing.T) {
	script, err := buildShellScript("uname -a", "/srv/app", map[string]string{
		"DEPLOY_ENV": "prod",
	})
	if err != nil {
		t.Fatalf("buildShellScript returned error: %v", err)
	}

	if !strings.Contains(script, "cd '/srv/app'") {
		t.Fatalf("expected workdir change in script, got %q", script)
	}
	if !strings.Contains(script, "export DEPLOY_ENV='prod'") {
		t.Fatalf("expected env export in script, got %q", script)
	}
	if !strings.HasSuffix(script, "uname -a") {
		t.Fatalf("expected command suffix, got %q", script)
	}
}

func TestBuildShellScriptRejectsInvalidEnvKey(t *testing.T) {
	_, err := buildShellScript("uptime", "", map[string]string{
		"BAD-KEY": "x",
	})
	if err == nil {
		t.Fatal("expected invalid env key to fail")
	}
}
