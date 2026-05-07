package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

const (
	SchemaVersion = 1
	StaleAfter    = 24 * time.Hour
)

// CatalogEntry is one immutable package asset hash visible from Dolt.
type CatalogEntry struct {
	PackageID string `toml:"package_id" json:"package_id"`
	Version   string `toml:"version" json:"version"`
	DocPath   string `toml:"doc_path" json:"doc_path"`
	SHA256    string `toml:"sha256" json:"sha256"`
}

// Catalog is the on-disk SHA catalog cache.
type Catalog struct {
	Meta    CatalogMeta    `toml:"meta" json:"meta"`
	Entries []CatalogEntry `toml:"entries" json:"entries"`
}

// CatalogMeta describes the catalog cache itself.
type CatalogMeta struct {
	Branch        string    `toml:"branch" json:"branch"`
	FetchedAt     time.Time `toml:"fetched_at" json:"fetched_at"`
	SchemaVersion int       `toml:"schema_version" json:"schema_version"`
}

// EntryKey is the immutable identity tuple for one catalog row.
type EntryKey struct {
	PackageID string
	Version   string
	DocPath   string
}

// Load reads one catalog file. Future schema versions are parsed best-effort
// and reported as warnings; invalid TOML is returned as a wrapped error.
func Load(path string) (Catalog, []string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // caller chooses catalog path.
	if err != nil {
		return Catalog{}, nil, err
	}
	var parsed Catalog
	if err := toml.Unmarshal(data, &parsed); err != nil {
		return Catalog{}, nil, fmt.Errorf("parsing catalog %s: %w", path, err)
	}
	warnings := []string{}
	if parsed.Meta.SchemaVersion == 0 {
		parsed.Meta.SchemaVersion = SchemaVersion
	}
	if parsed.Meta.SchemaVersion > SchemaVersion {
		warnings = append(warnings, fmt.Sprintf("catalog schema_version %d is newer than supported %d; using best-effort parse", parsed.Meta.SchemaVersion, SchemaVersion))
	}
	return parsed, warnings, nil
}

// Refresh merges fetched entries into the existing catalog and writes it
// atomically. Existing entries for older versions are preserved.
func Refresh(path, branch string, entries []CatalogEntry, fetchedAt time.Time) (Catalog, error) {
	existing, warnings, err := Load(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Catalog{}, err
	}
	if len(warnings) > 0 {
		// Keep refresh deterministic; warnings are preserved for Load callers.
		existing.Meta.SchemaVersion = SchemaVersion
	}

	if fetchedAt.IsZero() {
		fetchedAt = time.Now().UTC()
	}
	merged := Catalog{
		Meta: CatalogMeta{
			Branch:        branch,
			FetchedAt:     fetchedAt.UTC(),
			SchemaVersion: SchemaVersion,
		},
		Entries: mergeEntries(existing.Entries, entries),
	}
	if err := Save(path, merged); err != nil {
		return Catalog{}, err
	}
	return merged, nil
}

// Save writes a complete catalog atomically.
func Save(path string, value Catalog) error {
	if value.Meta.SchemaVersion == 0 {
		value.Meta.SchemaVersion = SchemaVersion
	}
	value.Entries = SortedEntries(value.Entries)
	return writeTOMLAtomic(path, value)
}

// ByKey returns entries indexed by their immutable identity tuple.
func ByKey(entries []CatalogEntry) map[EntryKey]CatalogEntry {
	index := make(map[EntryKey]CatalogEntry, len(entries))
	for _, entry := range entries {
		index[entry.Key()] = entry
	}
	return index
}

// Key returns the immutable catalog identity for entry.
func (e CatalogEntry) Key() EntryKey {
	return EntryKey{PackageID: e.PackageID, Version: e.Version, DocPath: e.DocPath}
}

// SortedEntries returns a stable copy sorted by package, version, and doc path.
func SortedEntries(entries []CatalogEntry) []CatalogEntry {
	sorted := make([]CatalogEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.PackageID == "" || entry.Version == "" || entry.DocPath == "" || entry.SHA256 == "" {
			continue
		}
		sorted = append(sorted, entry)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].PackageID != sorted[j].PackageID {
			return sorted[i].PackageID < sorted[j].PackageID
		}
		if sorted[i].Version != sorted[j].Version {
			return sorted[i].Version < sorted[j].Version
		}
		return sorted[i].DocPath < sorted[j].DocPath
	})
	return sorted
}

// SanitizeBranchName converts a branch name into a safe catalog filename component.
func SanitizeBranchName(branch string) string {
	if branch == "" {
		branch = "main"
	}
	var b strings.Builder
	for _, r := range branch {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "main"
	}
	return b.String()
}

// ProjectPath returns the project-scoped catalog path for branch.
func ProjectPath(root, branch string) string {
	return filepath.Join(root, ".synaptic", CatalogFilename(branch))
}

// MachinePath returns the machine-scoped catalog path for branch.
func MachinePath(branch string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	return filepath.Join(home, ".synaptic", CatalogFilename(branch)), nil
}

// CatalogFilename returns the catalog file name for branch.
func CatalogFilename(branch string) string {
	return "catalog-" + SanitizeBranchName(branch) + ".toml"
}

func mergeEntries(existing, fetched []CatalogEntry) []CatalogEntry {
	merged := ByKey(existing)
	for _, entry := range fetched {
		if entry.PackageID == "" || entry.Version == "" || entry.DocPath == "" || entry.SHA256 == "" {
			continue
		}
		merged[entry.Key()] = entry
	}
	result := make([]CatalogEntry, 0, len(merged))
	for _, entry := range merged {
		result = append(result, entry)
	}
	return SortedEntries(result)
}
