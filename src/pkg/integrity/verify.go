package integrity

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// VerifyFile computes the SHA256 of the file at path and compares it against
// expectedSHA. Returns:
//   - StatusMissing if the file does not exist (errors.Is fs.ErrNotExist)
//   - StatusOK if SHA matches
//   - StatusModified if SHA does not match
func VerifyFile(path, expectedSHA string) VerifyStatus {
	actual, err := ComputeFileSHA256(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return StatusMissing
		}
		// Treat unreadable files (permissions, etc.) as missing so callers get
		// a definitive status rather than a raw error path.
		return StatusMissing
	}
	if actual == expectedSHA {
		return StatusOK
	}
	return StatusModified
}

// VerifyPackage verifies all files in expected against files on disk under
// installedDir. It also detects extra files present on disk that have no
// entry in expected.
//
// For each FileHash in expected:
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
	// Build expected set for quick lookup.
	expectedSet := make(map[string]struct{}, len(expected))
	for _, fh := range expected {
		expectedSet[fh.DestPath] = struct{}{}
	}

	// Verify each expected file. VerifyFile returns StatusMissing when
	// installedDir does not exist (the path will not exist either).
	results := make([]VerifyResult, 0, len(expected))
	for _, fh := range expected {
		fullPath := filepath.Join(installedDir, fh.DestPath)
		status := VerifyFile(fullPath, fh.SHA256)
		results = append(results, VerifyResult{Path: fh.DestPath, Status: status})
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
		if _, ok := expectedSet[rel]; !ok {
			extraPaths = append(extraPaths, rel)
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
