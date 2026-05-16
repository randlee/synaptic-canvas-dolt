package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var snapshotNow = func() time.Time { return time.Now().UTC() }
var snapshotGitRemoteURL = gitRemoteURL

func scopeDisplay(branch, version string) string {
	if version == "" {
		return ""
	}
	if branch == "" || branch == "main" {
		return version
	}
	return version + " " + branch
}

func sanitizePathComponent(value string) string {
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	cleaned := strings.Trim(b.String(), "-")
	if cleaned == "" {
		return "unknown"
	}
	return cleaned
}

func repoKey(path string) string {
	base := sanitizePathComponent(filepath.Base(path))
	sum := sha256.Sum256([]byte(path))
	return base + "-" + hex.EncodeToString(sum[:4])
}

func gitRemoteURL(path string) string {
	if path == "" {
		return ""
	}
	cmd := exec.Command("git", "-C", path, "remote", "get-url", "origin") //nolint:gosec // git command and args are fixed.
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
