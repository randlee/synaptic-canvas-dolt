package catalog

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// NOTE: duplicate of tracking.writeTOMLAtomic; a future refactor may extract this to src/internal/atomicfile/.
func writeTOMLAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating parent dir: %w", err)
	}
	data, err := toml.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding toml: %w", err)
	}

	tmpName, err := tempName(path)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpName) }()

	file, err := os.OpenFile(tmpName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // temp path is generated in the catalog directory.
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(filepath.Clean(tmpName), filepath.Clean(path)); err != nil { //nolint:gosec // both paths are in the catalog directory.
		return fmt.Errorf("atomic replace %s: %w", path, err)
	}
	return nil
}

func tempName(path string) (string, error) {
	var token [8]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("generating temp file token: %w", err)
	}
	return path + "." + hex.EncodeToString(token[:]) + ".tmp", nil
}
