package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// ComputeAggregateSHA256 computes a deterministic package-level SHA256 from a
// slice of FileHash values. The algorithm:
//  1. Sort entries by DestPath (lexicographic ascending)
//  2. Build a string: join "dest_path:sha256" pairs with newline (\n)
//  3. Return SHA256(that string) as hex
//
// An empty slice returns the SHA256 of an empty string.
func ComputeAggregateSHA256(files []FileHash) string {
	// Sort a copy so we do not mutate the caller's slice.
	sorted := make([]FileHash, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].DestPath < sorted[j].DestPath
	})

	pairs := make([]string, len(sorted))
	for i, f := range sorted {
		pairs[i] = f.DestPath + ":" + f.SHA256
	}

	input := strings.Join(pairs, "\n")
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
