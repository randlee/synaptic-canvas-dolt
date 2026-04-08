// Package integrity provides SHA256 computation and verification for Synaptic
// Canvas packages. It implements the per-file and aggregate hash model defined
// in docs/synaptic-canvas-cli.md (Integrity Model section).
package integrity

// FileHash holds a destination path and its SHA256 hex digest. It is the
// unit of input for aggregate hash computation and verification operations.
type FileHash struct {
	DestPath string
	SHA256   string
}

// VerifyStatus classifies the state of an installed file relative to the
// expected SHA256 stored in Dolt.
type VerifyStatus int

const (
	// StatusOK means the file exists and its SHA256 matches the expected value.
	StatusOK VerifyStatus = iota
	// StatusModified means the file exists but its SHA256 does not match.
	StatusModified
	// StatusMissing means the file does not exist on disk.
	StatusMissing
	// StatusUnreadable means the file exists but cannot be read (e.g. permission denied or I/O error).
	StatusUnreadable
	// StatusExtra means the file exists on disk but has no entry in Dolt.
	StatusExtra
)

// String returns the human-readable label for a VerifyStatus.
func (s VerifyStatus) String() string {
	switch s {
	case StatusOK:
		return "OK"
	case StatusModified:
		return "MODIFIED"
	case StatusMissing:
		return "MISSING"
	case StatusUnreadable:
		return "UNREADABLE"
	case StatusExtra:
		return "EXTRA"
	default:
		return "UNKNOWN"
	}
}

// VerifyResult reports the verification outcome for a single file path.
type VerifyResult struct {
	Path   string
	Status VerifyStatus
	Err    error // non-nil only for StatusUnreadable
}
