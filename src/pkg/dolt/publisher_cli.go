package dolt

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/randlee/synaptic-canvas-dolt/pkg/publisher"
)

// CLIPublisher promotes branches using the local dolt CLI.
type CLIPublisher struct {
	DoltDir string
}

func NewCLIPublisher(doltDir string) *CLIPublisher {
	return &CLIPublisher{DoltDir: doltDir}
}

func (p *CLIPublisher) Merge(ctx context.Context, fromBranch, toBranch string) (*publisher.MergeResult, error) {
	query := fmt.Sprintf("CALL DOLT_MERGE(%s);", sqlString(fromBranch))
	cmd := exec.CommandContext(ctx, doltCommand, "--branch", toBranch, "sql", "-q", query, "-r", "csv") //nolint:gosec // G204: dolt binary is hardcoded constant.
	cmd.Dir = p.DoltDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("merging %s into %s: %w: %s", fromBranch, toBranch, err, strings.TrimSpace(stderr.String()))
	}
	return parseMergeCSV(stdout.String())
}

func parseMergeCSV(raw string) (*publisher.MergeResult, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimSpace(raw)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing merge csv: %w", err)
	}
	if len(records) < 2 || len(records[1]) < 4 {
		return nil, fmt.Errorf("unexpected merge output: %q", raw)
	}
	fastForward := records[1][1] == "1"
	conflicts, err := strconv.Atoi(records[1][2])
	if err != nil {
		return nil, fmt.Errorf("parsing conflict count: %w", err)
	}
	return &publisher.MergeResult{
		Hash:        records[1][0],
		FastForward: fastForward,
		Conflicts:   conflicts,
		Message:     records[1][3],
	}, nil
}
