package sshops

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func expandEnv(value string) string {
	return os.ExpandEnv(value)
}

func expandPath(value string) string {
	value = strings.TrimSpace(expandEnv(value))
	if value == "" {
		return value
	}
	if strings.HasPrefix(value, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			value = filepath.Join(home, strings.TrimPrefix(value, "~"))
		}
	}
	return value
}

func shellEscape(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func isValidEnvKey(key string) bool {
	return envKeyPattern.MatchString(key)
}

func normalizeShell(shell string) string {
	switch strings.TrimSpace(shell) {
	case "", "bash":
		if strings.TrimSpace(shell) == "" {
			return ""
		}
		return "bash"
	case "sh":
		return "sh"
	default:
		return ""
	}
}

type limitedBuffer struct {
	buf       bytes.Buffer
	remaining int64
	truncated bool
}

func newLimitedBuffer(limit int64) *limitedBuffer {
	return &limitedBuffer{remaining: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if int64(len(p)) > b.remaining {
		_, _ = b.buf.Write(p[:int(b.remaining)])
		b.remaining = 0
		b.truncated = true
		return len(p), nil
	}
	_, err := b.buf.Write(p)
	b.remaining -= int64(len(p))
	return len(p), err
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}

func (b *limitedBuffer) Truncated() bool {
	return b.truncated
}

func joinRemotePath(base, rel string) string {
	if base == "" {
		return rel
	}
	if rel == "" {
		return base
	}
	base = strings.TrimSuffix(base, "/")
	rel = strings.TrimPrefix(rel, "/")
	return base + "/" + rel
}
