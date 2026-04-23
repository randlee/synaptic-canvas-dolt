package dolt

import (
	"testing"
)

func TestParseMergeCSV(t *testing.T) {
	t.Parallel()

	got, err := parseMergeCSV("hash,fast_forward,conflicts,message\nabc,1,0,merge successful\n")
	if err != nil {
		t.Fatalf("parseMergeCSV() error = %v", err)
	}
	if got.Hash != "abc" || !got.FastForward || got.Conflicts != 0 || got.Message != "merge successful" {
		t.Fatalf("unexpected merge result: %#v", got)
	}
}
