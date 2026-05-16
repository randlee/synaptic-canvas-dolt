package catalog

import (
	"errors"
	"fmt"
)

// MissingCatalogError reports that a required cached catalog is absent.
type MissingCatalogError struct {
	Branch string
	Path   string
}

func (e MissingCatalogError) Error() string {
	return fmt.Sprintf("catalog not found for branch %s at %s; run: sc catalog update", e.Branch, e.Path)
}

// NewMissingCatalogError returns the typed catalog-missing sentinel.
func NewMissingCatalogError(path, branch string) error {
	return MissingCatalogError{Branch: branch, Path: path}
}

// MissingCatalogDetails unwraps a typed missing-catalog error when present.
func MissingCatalogDetails(err error) (MissingCatalogError, bool) {
	var target MissingCatalogError
	if !errors.As(err, &target) {
		return MissingCatalogError{}, false
	}
	return target, true
}
