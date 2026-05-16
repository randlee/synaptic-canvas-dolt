package cmd

import (
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/operations"
)

func stateRootForScope(repoRoot, scope string) (string, error) {
	return operations.StateRootForScope(repoRoot, scope)
}

func rollbackInstallSummary(repoRoot string, summary installer.Summary) error {
	return operations.RollbackInstallSummary(repoRoot, summary)
}

func currentProfileSnapshot(repoRoot string) map[string]any {
	return operations.CurrentProfileSnapshot(repoRoot, snapshotNow())
}
