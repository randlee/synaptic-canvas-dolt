package integrity

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// VerifyFile computes the SHA256 of the file at path and compares it against
// expectedSHA. Returns a VerifyResult with:
//   - StatusMissing if the file does not exist (errors.Is fs.ErrNotExist)
//   - StatusUnreadable if the file exists but cannot be read; Err holds the cause
//   - StatusOK if SHA matches
//   - StatusModified if SHA does not match
func VerifyFile(path, expectedSHA string) VerifyResult {
	actual, err := ComputeFileSHA256(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return VerifyResult{Path: path, Status: StatusMissing}
		}
		return VerifyResult{Path: path, Status: StatusUnreadable, Err: err}
	}
	if actual == expectedSHA {
		return VerifyResult{Path: path, Status: StatusOK}
	}
	return VerifyResult{Path: path, Status: StatusModified}
}

// validateDestPath rejects dest paths that could escape the install root:
//   - empty string
//   - absolute paths (filepath.IsAbs)
//   - paths whose cleaned, slash-normalised form begins with ".." or equal ".."
func validateDestPath(destPath string) error {
	if destPath == "" {
		return errors.New("dest_path must not be empty")
	}
	if filepath.IsAbs(destPath) {
		return fmt.Errorf("dest_path must be relative, got absolute path %q", destPath)
	}
	cleaned := filepath.ToSlash(filepath.Clean(destPath))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("dest_path %q escapes the install root", destPath)
	}
	return nil
}

// VerifyPackage verifies all files in expected against files on disk under
// installedDir. It also detects extra files present on disk that have no
// entry in expected.
//
// For each FileHash in expected:
//   - Validates fh.DestPath with validateDestPath (returns error on bad path)
//   - Resolves the full path as filepath.Join(installedDir, fh.DestPath)
//   - Calls VerifyFile and records the result
//
// For extra files: walks installedDir, reports any file whose path (relative
// to installedDir) is not in the expected set as StatusExtra.
// If installedDir does not exist, the walk is skipped (no extra files to find).
//
// Returns a slice of VerifyResult, one per file processed (expected + extra).
// Order: expected files first (in input order), extra files after (sorted).
func VerifyPackage(expected []FileHash, installedDir string) ([]VerifyResult, error) {
	// Build expected set for quick lookup. Canonicalise to forward slashes so
	// the lookup works identically on all platforms (filepath.Rel returns
	// backslashes on Windows for nested paths).
	expectedSet := make(map[string]struct{}, len(expected))
	for _, fh := range expected {
		expectedSet[filepath.ToSlash(fh.DestPath)] = struct{}{}
	}

	// Verify each expected file. VerifyFile returns StatusMissing when
	// installedDir does not exist (the path will not exist either).
	results := make([]VerifyResult, 0, len(expected))
	for _, fh := range expected {
		if err := validateDestPath(fh.DestPath); err != nil {
			return nil, fmt.Errorf("invalid dest_path %q: %w", fh.DestPath, err)
		}
		fullPath := filepath.Join(installedDir, fh.DestPath)
		r := VerifyFile(fullPath, fh.SHA256)
		r.Path = fh.DestPath
		results = append(results, r)
	}

	// Walk the installed directory to detect extra files.
	// If installedDir does not exist, skip the walk — there are no extra files.
	if _, statErr := os.Stat(installedDir); errors.Is(statErr, fs.ErrNotExist) {
		return results, nil
	}

	var extraPaths []string
	err := filepath.Walk(installedDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		// filepath.Rel cannot fail when both paths are absolute and well-formed.
		rel, _ := filepath.Rel(installedDir, path)
		// Canonicalise to forward slashes so the lookup matches the expectedSet
		// keys built above (important on Windows where filepath.Rel uses backslashes).
		relSlash := filepath.ToSlash(rel)
		if _, ok := expectedSet[relSlash]; !ok {
			extraPaths = append(extraPaths, relSlash)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking dir %q: %w", installedDir, err)
	}

	sort.Strings(extraPaths)
	for _, p := range extraPaths {
		results = append(results, VerifyResult{Path: p, Status: StatusExtra})
	}

	return results, nil
}
