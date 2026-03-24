package sshops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLocalFileTargetUsesDirectory(t *testing.T) {
	root := t.TempDir()

	target, err := resolveLocalFileTarget(root, "app.log")
	if err != nil {
		t.Fatalf("resolveLocalFileTarget returned error: %v", err)
	}
	if target != filepath.Join(root, "app.log") {
		t.Fatalf("expected file to land in directory, got %q", target)
	}
}

func TestLimitedBufferMarksTruncation(t *testing.T) {
	buf := newLimitedBuffer(4)
	if _, err := buf.Write([]byte("abcdef")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if !buf.Truncated() {
		t.Fatal("expected buffer to report truncation")
	}
	if buf.String() != "abcd" {
		t.Fatalf("unexpected buffer contents: %q", buf.String())
	}
}

func TestResolveLocalFileTargetAcceptsExplicitPath(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "file.txt")

	target, err := resolveLocalFileTarget(targetPath, "ignored.txt")
	if err != nil {
		t.Fatalf("resolveLocalFileTarget returned error: %v", err)
	}
	if target != targetPath {
		t.Fatalf("expected explicit path to remain unchanged, got %q", target)
	}

	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}
}
