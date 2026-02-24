package dolt

import (
	"context"
	"strings"

	"github.com/randlee/synaptic-canvas/pkg/models"
)

// MockClient is an in-memory implementation of Client for testing.
// It stores test data that can be populated before test execution.
type MockClient struct {
	Packages  map[string]*models.Package
	Files     map[string][]models.PackageFile
	Deps      map[string][]models.PackageDep
	Hooks     map[string][]models.PackageHook
	Questions map[string][]models.PackageQuestion
	Variants  map[string]string // key: "logicalID/agentProfile" -> variantPackageID

	// Error fields allow tests to inject errors for specific operations.
	ListErr      error
	GetErr       error
	FilesErr     error
	DepsErr      error
	HooksErr     error
	QuestionsErr error
	VariantErr   error
	CloseErr     error

	Closed bool
}

// NewMockClient creates a MockClient with initialized maps.
func NewMockClient() *MockClient {
	return &MockClient{
		Packages:  make(map[string]*models.Package),
		Files:     make(map[string][]models.PackageFile),
		Deps:      make(map[string][]models.PackageDep),
		Hooks:     make(map[string][]models.PackageHook),
		Questions: make(map[string][]models.PackageQuestion),
		Variants:  make(map[string]string),
	}
}

// AddPackage adds a package to the mock data store.
func (m *MockClient) AddPackage(p *models.Package) {
	m.Packages[p.ID] = p
}

// AddFiles adds files for a package to the mock data store.
func (m *MockClient) AddFiles(packageID string, files []models.PackageFile) {
	m.Files[packageID] = files
}

// AddDeps adds dependencies for a package to the mock data store.
func (m *MockClient) AddDeps(packageID string, deps []models.PackageDep) {
	m.Deps[packageID] = deps
}

// AddHooks adds hooks for a package to the mock data store.
func (m *MockClient) AddHooks(packageID string, hooks []models.PackageHook) {
	m.Hooks[packageID] = hooks
}

// AddQuestions adds questions for a package to the mock data store.
func (m *MockClient) AddQuestions(packageID string, questions []models.PackageQuestion) {
	m.Questions[packageID] = questions
}

// AddVariant adds a variant mapping to the mock data store.
func (m *MockClient) AddVariant(logicalID, agentProfile, variantPackageID string) {
	key := logicalID + "/" + agentProfile
	m.Variants[key] = variantPackageID
}

// ListPackages returns all packages in the mock store.
func (m *MockClient) ListPackages(_ context.Context, _ ListOptions) ([]models.Package, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	result := make([]models.Package, 0, len(m.Packages))
	for _, p := range m.Packages {
		result = append(result, *p)
	}
	return result, nil
}

// GetPackage returns a package by ID from the mock store.
func (m *MockClient) GetPackage(_ context.Context, id string) (*models.Package, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	p, ok := m.Packages[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

// GetPackageFiles returns files for a package from the mock store.
func (m *MockClient) GetPackageFiles(_ context.Context, packageID string) ([]models.PackageFile, error) {
	if m.FilesErr != nil {
		return nil, m.FilesErr
	}
	return m.Files[packageID], nil
}

// GetPackageDeps returns dependencies for a package from the mock store.
func (m *MockClient) GetPackageDeps(_ context.Context, packageID string) ([]models.PackageDep, error) {
	if m.DepsErr != nil {
		return nil, m.DepsErr
	}
	return m.Deps[packageID], nil
}

// GetPackageHooks returns hooks for a package from the mock store.
func (m *MockClient) GetPackageHooks(_ context.Context, packageID string) ([]models.PackageHook, error) {
	if m.HooksErr != nil {
		return nil, m.HooksErr
	}
	return m.Hooks[packageID], nil
}

// GetPackageQuestions returns questions for a package from the mock store.
func (m *MockClient) GetPackageQuestions(_ context.Context, packageID string) ([]models.PackageQuestion, error) {
	if m.QuestionsErr != nil {
		return nil, m.QuestionsErr
	}
	return m.Questions[packageID], nil
}

// ResolveVariant resolves a variant from the mock store.
func (m *MockClient) ResolveVariant(_ context.Context, logicalID, agentProfile string) (string, error) {
	if m.VariantErr != nil {
		return "", m.VariantErr
	}
	key := logicalID + "/" + agentProfile
	return m.Variants[key], nil
}

// Close marks the mock client as closed.
func (m *MockClient) Close() error {
	if m.CloseErr != nil {
		return m.CloseErr
	}
	m.Closed = true
	return nil
}

// Verify MockClient implements Client at compile time.
var _ Client = (*MockClient)(nil)

// NewTestPackage is a helper that creates a Package with common test defaults.
// Tags are stored as a comma-separated string (not JSON).
func NewTestPackage(id, name, version string, tags []string) *models.Package {
	p := &models.Package{
		ID:           id,
		Name:         name,
		Version:      version,
		InstallScope: "any",
	}
	if len(tags) > 0 {
		p.Tags = strings.Join(tags, ",")
	}
	return p
}
