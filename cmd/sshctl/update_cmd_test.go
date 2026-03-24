package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestBuildUpdateCommand(t *testing.T) {
	command, label := buildUpdateCommand(installTargets{Codex: true, Claude: true}, "v0.1.4")
	if label != "Codex + Claude Code" {
		t.Fatalf("unexpected label: %q", label)
	}
	if runtime.GOOS == "windows" {
		if !strings.Contains(command, "-All") || !strings.Contains(command, "-Version v0.1.4") {
			t.Fatalf("unexpected windows command: %q", command)
		}
		return
	}
	if !strings.Contains(command, "--all") || !strings.Contains(command, "--version v0.1.4") {
		t.Fatalf("unexpected unix command: %q", command)
	}
}

func TestResolveInstallTargets(t *testing.T) {
	targets := resolveInstallTargets(installTargets{}, false, false, false)
	if targets.Codex || targets.Claude {
		t.Fatalf("expected empty targets, got %#v", targets)
	}

	targets = resolveInstallTargets(installTargets{}, false, true, false)
	if !targets.Claude || targets.Codex {
		t.Fatalf("expected Claude override, got %#v", targets)
	}

	targets = resolveInstallTargets(installTargets{Codex: true}, false, false, false)
	if !targets.Codex || targets.Claude {
		t.Fatalf("expected detected Codex target, got %#v", targets)
	}
}
