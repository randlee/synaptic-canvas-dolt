package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// ComputeContentSHA256 returns the hex-encoded SHA256 digest of b.
// The output is identical to the digest produced by sha256sum on the same bytes.
func ComputeContentSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ComputeFileSHA256 reads the file at path and returns its hex-encoded SHA256 digest.
// Returns an error if the file cannot be read.
func ComputeFileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // Intentional: ComputeFileSHA256 reads files by design; path is provided by the caller.
	if err != nil {
		return "", fmt.Errorf("reading file %q: %w", path, err)
	}
	return ComputeContentSHA256(b), nil
}
