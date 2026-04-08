package integrity

import (
	"os"
	"path/filepath"
	"testing"
)

// TestComputeContentSHA256 covers known vectors, empty input, and binary content.
func TestComputeContentSHA256(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{
			name:  "empty slice",
			input: []byte{},
			want:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:  "nil treated as empty",
			input: nil,
			want:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:  "known vector: hello world",
			input: []byte("hello world"),
			want:  "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:  "known vector: abc",
			input: []byte("abc"),
			want:  "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			// binary content: every byte value 0x00–0xFF
			name: "binary 0x00-0xFF",
			input: func() []byte {
				b := make([]byte, 256)
				for i := range b {
					b[i] = byte(i)
				}
				return b
			}(),
			want: "40aff2e9d2d8922e47afd4648e6967497158785fbd1da870e7110266bf944880",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ComputeContentSHA256(tc.input)
			if got != tc.want {
				t.Errorf("ComputeContentSHA256(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestComputeFileSHA256_MatchesContent verifies that ComputeFileSHA256 returns
// the same digest as ComputeContentSHA256 for the same bytes.
func TestComputeFileSHA256_MatchesContent(t *testing.T) {
	t.Parallel()

	content := []byte("synaptic canvas integrity test\n")
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile.txt")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	wantDigest := ComputeContentSHA256(content)
	gotDigest, err := ComputeFileSHA256(path)
	if err != nil {
		t.Fatalf("ComputeFileSHA256(%q) unexpected error: %v", path, err)
	}
	if gotDigest != wantDigest {
		t.Errorf("ComputeFileSHA256 = %q, want %q", gotDigest, wantDigest)
	}
}

// TestComputeFileSHA256_EmptyFile verifies that an empty file produces the
// well-known SHA256 of zero bytes.
func TestComputeFileSHA256_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatalf("writing empty temp file: %v", err)
	}

	const wantEmpty = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	got, err := ComputeFileSHA256(path)
	if err != nil {
		t.Fatalf("ComputeFileSHA256 unexpected error: %v", err)
	}
	if got != wantEmpty {
		t.Errorf("ComputeFileSHA256(empty) = %q, want %q", got, wantEmpty)
	}
}

// TestComputeFileSHA256_UnicodeFilename verifies that a file whose name contains
// Unicode characters can be read and hashed without error.
func TestComputeFileSHA256_UnicodeFilename(t *testing.T) {
	t.Parallel()

	content := []byte("unicode filename test: 日本語 café")
	dir := t.TempDir()
	// Unicode filename: Japanese + accented characters
	path := filepath.Join(dir, "テスト_café_file.md")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("writing unicode-named temp file: %v", err)
	}

	want := ComputeContentSHA256(content)
	got, err := ComputeFileSHA256(path)
	if err != nil {
		t.Fatalf("ComputeFileSHA256(%q) unexpected error: %v", path, err)
	}
	if got != want {
		t.Errorf("ComputeFileSHA256(unicode path) = %q, want %q", got, want)
	}
}

// TestComputeFileSHA256_Fixture verifies that ComputeFileSHA256 on the shared
// test fixture produces the expected SHA256 digest.
func TestComputeFileSHA256_Fixture(t *testing.T) {
	t.Parallel()

	// Fixture is at test/fixtures/integrity/hello.txt relative to the repo root.
	// This test file lives at src/pkg/integrity/, so the fixture is three levels up.
	fixturePath := filepath.Join("..", "..", "..", "test", "fixtures", "integrity", "hello.txt")

	const wantSHA = "f1a902bd3d0e85ca9c075b36a9733d08c1a5a072937a1c6e57046e030924916e"
	got, err := ComputeFileSHA256(fixturePath)
	if err != nil {
		t.Fatalf("ComputeFileSHA256(%q) unexpected error: %v", fixturePath, err)
	}
	if got != wantSHA {
		t.Errorf("ComputeFileSHA256(fixture) = %q; want %q", got, wantSHA)
	}
}

// TestComputeFileSHA256_Nonexistent verifies that a missing file returns a
// wrapped error rather than panicking or returning an empty digest.
func TestComputeFileSHA256_Nonexistent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.txt")

	got, err := ComputeFileSHA256(path)
	if err == nil {
		t.Errorf("ComputeFileSHA256(nonexistent) expected error, got digest %q", got)
	}
	if got != "" {
		t.Errorf("ComputeFileSHA256(nonexistent) digest should be empty on error, got %q", got)
	}
}

// TestComputeFileSHA256_BinaryContent verifies that ComputeFileSHA256 agrees
// with ComputeContentSHA256 for binary data (all byte values 0x00–0xFF).
func TestComputeFileSHA256_BinaryContent(t *testing.T) {
	t.Parallel()

	// Build 256-byte slice with all possible byte values.
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "binary.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("writing binary temp file: %v", err)
	}

	wantDigest := ComputeContentSHA256(data)
	gotDigest, err := ComputeFileSHA256(path)
	if err != nil {
		t.Fatalf("ComputeFileSHA256(%q) unexpected error: %v", path, err)
	}
	if gotDigest != wantDigest {
		t.Errorf("ComputeFileSHA256(binary) = %q; want %q", gotDigest, wantDigest)
	}
}
