package integrity

import (
	"testing"
)

func TestComputeAggregateSHA256(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		files    []FileHash
		wantHash string
	}{
		{
			name:  "empty slice returns SHA256 of empty string",
			files: []FileHash{},
			// SHA256("") = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
			wantHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "nil slice returns SHA256 of empty string",
			files:    nil,
			wantHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "single file produces correct hash",
			files: []FileHash{
				{DestPath: "scripts/hello.sh", SHA256: "abc123def456"},
			},
			// SHA256("scripts/hello.sh:abc123def456")
			wantHash: "c4255b501caafaf1676fc4c1b814cc7af43aca3cce38aba52490d8c4d734b55a",
		},
		{
			name: "multiple files in sorted order",
			files: []FileHash{
				{DestPath: "a.txt", SHA256: "aaa"},
				{DestPath: "b.txt", SHA256: "bbb"},
			},
			// SHA256("a.txt:aaa\nb.txt:bbb")
			wantHash: "e1913340ea015b31cb13bf39393232f5f8432d9813e7bd269fccc703406ea83b",
		},
		{
			name: "same files in reverse order produce same hash",
			files: []FileHash{
				{DestPath: "b.txt", SHA256: "bbb"},
				{DestPath: "a.txt", SHA256: "aaa"},
			},
			// must match the sorted-order case above
			wantHash: "e1913340ea015b31cb13bf39393232f5f8432d9813e7bd269fccc703406ea83b",
		},
		{
			name: "unicode dest path",
			files: []FileHash{
				{DestPath: "café/script.sh", SHA256: "deadbeef"},
			},
			// SHA256("café/script.sh:deadbeef")
			wantHash: "9fd0dcfe99ead6596df9cd743954f01b6ab3ee3844e0a0002e0c45bfb06192e0",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ComputeAggregateSHA256(tc.files)
			if got != tc.wantHash {
				t.Errorf("ComputeAggregateSHA256(%v) = %q, want %q", tc.files, got, tc.wantHash)
			}
		})
	}
}

// TestComputeAggregateSHA256_DoesNotMutateInput verifies that the input slice
// is not reordered by the function.
func TestComputeAggregateSHA256_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	files := []FileHash{
		{DestPath: "z.txt", SHA256: "zzz"},
		{DestPath: "a.txt", SHA256: "aaa"},
		{DestPath: "m.txt", SHA256: "mmm"},
	}

	// Capture original order.
	originalOrder := make([]string, len(files))
	for i, f := range files {
		originalOrder[i] = f.DestPath
	}

	ComputeAggregateSHA256(files)

	for i, f := range files {
		if f.DestPath != originalOrder[i] {
			t.Errorf("input slice was mutated: index %d changed from %q to %q", i, originalOrder[i], f.DestPath)
		}
	}
}
