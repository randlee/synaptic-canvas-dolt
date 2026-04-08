package integrity

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeFile is a helper that creates a file with the given content inside dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	return path
}

// ---- VerifyFile tests -------------------------------------------------------

func TestVerifyFile(t *testing.T) {
	dir := t.TempDir()

	content := "hello, world"
	correctSHA := ComputeContentSHA256([]byte(content))
	wrongSHA := ComputeContentSHA256([]byte("different content"))

	existingFile := writeFile(t, dir, "file.txt", content)

	tests := []struct {
		name        string
		path        string
		expectedSHA string
		want        VerifyStatus
	}{
		{
			name:        "status_ok_matching_sha",
			path:        existingFile,
			expectedSHA: correctSHA,
			want:        StatusOK,
		},
		{
			name:        "status_modified_wrong_sha",
			path:        existingFile,
			expectedSHA: wrongSHA,
			want:        StatusModified,
		},
		{
			name:        "status_missing_absent_file",
			path:        filepath.Join(dir, "does_not_exist.txt"),
			expectedSHA: correctSHA,
			want:        StatusMissing,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := VerifyFile(tc.path, tc.expectedSHA)
			if got != tc.want {
				t.Errorf("VerifyFile(%q, %q) = %v; want %v", tc.path, tc.expectedSHA, got, tc.want)
			}
		})
	}
}

// TestVerifyFile_Unreadable exercises the non-IsNotExist error branch
// (file exists but cannot be read due to permissions).
func TestVerifyFile_Unreadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission model differs on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root: permission restrictions do not apply")
	}

	dir := t.TempDir()
	path := writeFile(t, dir, "secret.txt", "contents")

	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	got := VerifyFile(path, "anyhash")
	if got != StatusMissing {
		t.Errorf("VerifyFile on unreadable file = %v; want StatusMissing", got)
	}
}

// ---- VerifyPackage tests ----------------------------------------------------

func TestVerifyPackage(t *testing.T) {
	t.Run("all_ok", func(t *testing.T) {
		dir := t.TempDir()
		content := "package content"
		sha := ComputeContentSHA256([]byte(content))
		writeFile(t, dir, "a.txt", content)
		writeFile(t, dir, "b.txt", content)

		expected := []FileHash{
			{DestPath: "a.txt", SHA256: sha},
			{DestPath: "b.txt", SHA256: sha},
		}

		results, err := VerifyPackage(expected, dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("len(results) = %d; want 2", len(results))
		}
		for _, r := range results {
			if r.Status != StatusOK {
				t.Errorf("file %q: status = %v; want StatusOK", r.Path, r.Status)
			}
		}
	})

	t.Run("mix_ok_modified_missing", func(t *testing.T) {
		dir := t.TempDir()

		okContent := "ok content"
		okSHA := ComputeContentSHA256([]byte(okContent))
		modContent := "modified content"
		modSHA := ComputeContentSHA256([]byte("original content")) // stored SHA for "original"

		writeFile(t, dir, "ok.txt", okContent)
		writeFile(t, dir, "modified.txt", modContent) // exists but SHA won't match modSHA

		expected := []FileHash{
			{DestPath: "ok.txt", SHA256: okSHA},
			{DestPath: "modified.txt", SHA256: modSHA},
			{DestPath: "missing.txt", SHA256: okSHA},
		}

		results, err := VerifyPackage(expected, dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("len(results) = %d; want 3", len(results))
		}

		want := []VerifyStatus{StatusOK, StatusModified, StatusMissing}
		for i, r := range results {
			if r.Status != want[i] {
				t.Errorf("results[%d] (%q): status = %v; want %v", i, r.Path, r.Status, want[i])
			}
		}
	})

	t.Run("extra_files_detected", func(t *testing.T) {
		dir := t.TempDir()

		content := "data"
		sha := ComputeContentSHA256([]byte(content))
		writeFile(t, dir, "known.txt", content)
		writeFile(t, dir, "extra1.txt", "extra one")
		writeFile(t, dir, "extra2.txt", "extra two")

		expected := []FileHash{
			{DestPath: "known.txt", SHA256: sha},
		}

		results, err := VerifyPackage(expected, dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 1 expected + 2 extra
		if len(results) != 3 {
			t.Fatalf("len(results) = %d; want 3", len(results))
		}
		if results[0].Status != StatusOK {
			t.Errorf("results[0]: want StatusOK, got %v", results[0].Status)
		}
		if results[1].Status != StatusExtra {
			t.Errorf("results[1]: want StatusExtra, got %v", results[1].Status)
		}
		if results[2].Status != StatusExtra {
			t.Errorf("results[2]: want StatusExtra, got %v", results[2].Status)
		}
		// Extra files are sorted.
		if results[1].Path > results[2].Path {
			t.Errorf("extra files not sorted: %q > %q", results[1].Path, results[2].Path)
		}
	})

	t.Run("empty_expected_with_files_on_disk", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "unexpected.txt", "data")

		results, err := VerifyPackage(nil, dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("len(results) = %d; want 1", len(results))
		}
		if results[0].Status != StatusExtra {
			t.Errorf("results[0]: want StatusExtra, got %v", results[0].Status)
		}
	})

	t.Run("nonexistent_installeddir_expected_files_are_missing", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "no_such_dir")

		expected := []FileHash{
			{DestPath: "a.txt", SHA256: "abc"},
			{DestPath: "b.txt", SHA256: "def"},
		}
		results, err := VerifyPackage(expected, missing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("len(results) = %d; want 2", len(results))
		}
		for _, r := range results {
			if r.Status != StatusMissing {
				t.Errorf("file %q: status = %v; want StatusMissing", r.Path, r.Status)
			}
		}
	})

	t.Run("nonexistent_installeddir_empty_expected_returns_empty", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "no_such_dir")

		results, err := VerifyPackage([]FileHash{}, missing)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("len(results) = %d; want 0", len(results))
		}
	})

	t.Run("unicode_dest_paths", func(t *testing.T) {
		dir := t.TempDir()

		unicodeName := "файл_données_日本語.txt"
		content := "unicode content"
		sha := ComputeContentSHA256([]byte(content))
		writeFile(t, dir, unicodeName, content)

		expected := []FileHash{
			{DestPath: unicodeName, SHA256: sha},
		}

		results, err := VerifyPackage(expected, dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("len(results) = %d; want 1", len(results))
		}
		if results[0].Status != StatusOK {
			t.Errorf("unicode file: status = %v; want StatusOK", results[0].Status)
		}
		if results[0].Path != unicodeName {
			t.Errorf("path = %q; want %q", results[0].Path, unicodeName)
		}
	})

	t.Run("unicode_dest_path_missing", func(t *testing.T) {
		dir := t.TempDir()

		unicodeName := "manquant_αβγ_☃.txt"
		sha := ComputeContentSHA256([]byte("some content"))

		expected := []FileHash{
			{DestPath: unicodeName, SHA256: sha},
		}

		results, err := VerifyPackage(expected, dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("len(results) = %d; want 1", len(results))
		}
		if results[0].Status != StatusMissing {
			t.Errorf("unicode missing file: status = %v; want StatusMissing", results[0].Status)
		}
	})

	t.Run("walk_error_propagated", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("permission model differs on Windows")
		}
		if os.Getuid() == 0 {
			t.Skip("running as root: permission restrictions do not apply")
		}

		dir := t.TempDir()
		// Create a subdirectory that Walk cannot read.
		subdir := filepath.Join(dir, "locked")
		if err := os.MkdirAll(subdir, 0o750); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		writeFile(t, subdir, "inner.txt", "data")
		if err := os.Chmod(subdir, 0o000); err != nil {
			t.Fatalf("Chmod: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(subdir, 0o750) }) //nolint:gosec // G302: restoring directory permissions in test cleanup; 0o750 is appropriate for a directory, not a file.

		_, err := VerifyPackage(nil, dir)
		if err == nil {
			t.Error("expected error from unreadable subdir, got nil")
		}
	})
}

// TestVerifyPackage_PathTraversal verifies that VerifyPackage returns an error
// for malicious or malformed DestPath values and does not touch the filesystem.
func TestVerifyPackage_PathTraversal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		destPath string
	}{
		{name: "single_dotdot", destPath: ".."},
		{name: "dotdot_escape", destPath: "../escape"},
		{name: "deep_escape", destPath: "../../.ssh/config"},
		{name: "absolute_path", destPath: "/absolute/path"},
		{name: "empty_string", destPath: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Use a non-existent dir to confirm no filesystem access is needed
			// to detect the error — validation must happen before any I/O.
			installedDir := "/nonexistent/should/never/be/accessed"

			expected := []FileHash{
				{DestPath: tc.destPath, SHA256: "deadbeef"},
			}
			_, err := VerifyPackage(expected, installedDir)
			if err == nil {
				t.Errorf("VerifyPackage with DestPath=%q: expected error, got nil", tc.destPath)
			}
		})
	}
}

// TestVerifyPackage_NestedPaths verifies that nested files with slash-delimited
// DestPaths are classified as StatusOK (not StatusExtra) on all platforms.
func TestVerifyPackage_NestedPaths(t *testing.T) {
	dir := t.TempDir()

	content := "nested content"
	sha := ComputeContentSHA256([]byte(content))
	writeFile(t, dir, filepath.Join("subdir", "nested", "file.txt"), content)

	expected := []FileHash{
		{DestPath: "subdir/nested/file.txt", SHA256: sha},
	}

	results, err := VerifyPackage(expected, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d; want 1", len(results))
	}
	if results[0].Status != StatusOK {
		t.Errorf("nested file: status = %v; want StatusOK", results[0].Status)
	}
}

// TestVerifyStatusString verifies the String() method on VerifyStatus for all
// defined values and the default (unknown) branch.
func TestVerifyStatusString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status VerifyStatus
		want   string
	}{
		{StatusOK, "OK"},
		{StatusModified, "MODIFIED"},
		{StatusMissing, "MISSING"},
		{StatusExtra, "EXTRA"},
		{VerifyStatus(99), "UNKNOWN"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := tc.status.String()
			if got != tc.want {
				t.Errorf("VerifyStatus(%d).String() = %q; want %q", tc.status, got, tc.want)
			}
		})
	}
}
