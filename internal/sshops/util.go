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
	if value == "" {
		return value
	}

	var out strings.Builder
	out.Grow(len(value))

	for i := 0; i < len(value); {
		if value[i] != '$' {
			out.WriteByte(value[i])
			i++
			continue
		}

		if i+1 >= len(value) {
			out.WriteByte(value[i])
			break
		}

		switch value[i+1] {
		case '{':
			keyStart := i + 2
			keyEnd := strings.IndexByte(value[keyStart:], '}')
			if keyEnd < 0 {
				out.WriteByte(value[i])
				i++
				continue
			}
			keyEnd += keyStart
			key := value[keyStart:keyEnd]
			if isValidEnvKey(key) {
				out.WriteString(os.Getenv(key))
			} else {
				out.WriteString(value[i : keyEnd+1])
			}
			i = keyEnd + 1
		default:
			if !isEnvKeyStart(value[i+1]) {
				out.WriteByte(value[i])
				i++
				continue
			}

			keyEnd := i + 2
			for keyEnd < len(value) && isEnvKeyContinue(value[keyEnd]) {
				keyEnd++
			}

			out.WriteString(os.Getenv(value[i+1 : keyEnd]))
			i = keyEnd
		}
	}

	return out.String()
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

func isEnvKeyStart(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isEnvKeyContinue(value byte) bool {
	return isEnvKeyStart(value) || value >= '0' && value <= '9'
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
