package cmd

import (
	"context"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/operations"
)

type trackedInstall = operations.TrackedInstall

type validatedInstall = api.ValidatedInstall
type ValidationSeverity = api.ValidationSeverity
type ValidationKind = api.ValidationKind
type ValidationState = api.ValidationState
type validatedItem = api.ValidationItem

const (
	ValidationSeverityInfo     ValidationSeverity = api.ValidationSeverityInfo
	ValidationSeverityWarn     ValidationSeverity = api.ValidationSeverityWarn
	ValidationSeverityError    ValidationSeverity = api.ValidationSeverityError
	ValidationSeverityCritical ValidationSeverity = api.ValidationSeverityCritical

	ValidationKindFile       ValidationKind = api.ValidationKindFile
	ValidationKindDependency ValidationKind = api.ValidationKindDependency
	ValidationKindHook       ValidationKind = api.ValidationKindHook
	ValidationKindTemplate   ValidationKind = api.ValidationKindTemplate
	ValidationKindAggregate  ValidationKind = api.ValidationKindAggregate
	ValidationKindContext    ValidationKind = api.ValidationKindContext

	ValidationStateOK         ValidationState = api.ValidationStateOK
	ValidationStateModified   ValidationState = api.ValidationStateModified
	ValidationStateMissing    ValidationState = api.ValidationStateMissing
	ValidationStateUnreadable ValidationState = api.ValidationStateUnreadable
	ValidationStateExtra      ValidationState = api.ValidationStateExtra
)

func validateTrackedInstall(ctx context.Context, record installer.InstallRecord) (validatedInstall, error) {
	expected, warnings, err := operations.ResolveExpectedHashes(ctx, record, operations.ExpectedHashOptions{
		ResolveRepoRoot:    currentRepoRoot,
		FetchCatalog:       validateCatalogFetch,
		WriteCatalogCaches: writeCatalogCaches,
		Now:                snapshotNow,
		DisplayCatalogPath: displayCatalogPath,
	})
	if err != nil {
		return validatedInstall{}, err
	}
	return operations.ValidateTrackedInstall(ctx, record, expected, warnings, stateRootForScope)
}

func severityForValidationState(state ValidationState) ValidationSeverity {
	return operations.SeverityForValidationState(state)
}
