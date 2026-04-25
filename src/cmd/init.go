package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/randlee/synaptic-canvas-dolt/internal/config"
	"github.com/randlee/synaptic-canvas-dolt/internal/output"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/repo"
	"github.com/spf13/cobra"
)

type initResponse struct {
	OK        bool     `json:"ok"`
	Root      string   `json:"root"`
	Created   []string `json:"created"`
	Refreshed []string `json:"refreshed"`
}

func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize .synaptic state for the current repository",
		RunE:  runInitCmd,
	}
}

func runInitCmd(cmd *cobra.Command, _ []string) error {
	cfg, err := config.NewConfigFromFlags(cmd)
	if err != nil {
		return fmt.Errorf("reading config flags: %w", err)
	}
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}
	resp, err := initializeRepo(root)
	if err != nil {
		return err
	}
	formatter := output.NewFormatter(cfg.JSON, cfg.Quiet)
	formatter.Writer = cmd.OutOrStdout()
	formatter.ErrW = cmd.ErrOrStderr()
	if cfg.JSON {
		return formatter.WriteJSON(resp)
	}
	return formatter.Table([]string{"STATE", "FILES"}, [][]string{
		{"created", joinOrDash(resp.Created)},
		{"refreshed", joinOrDash(resp.Refreshed)},
	})
}

func initializeRepo(root string) (initResponse, error) {
	resp := initResponse{OK: true, Root: root}
	stateFiles := []string{
		installer.ManifestLockPath,
		installer.RepoProfilePath,
		installer.EnvPath,
		installer.HooksRegistry,
	}
	existedBefore := map[string]bool{}
	for _, rel := range stateFiles {
		existedBefore[rel] = existed(filepath.Join(root, rel))
	}
	profile, err := repo.DetectProfile(root, time.Now().UTC())
	if err != nil {
		return initResponse{}, fmt.Errorf("detecting repo profile: %w", err)
	}
	env := map[string]string{
		"SYNAPTIC_ROOT":         filepath.Join(root, ".synaptic"),
		"SYNAPTIC_SHARED":       filepath.Join(root, ".claude", "shared"),
		"SYNAPTIC_SKILLS":       filepath.Join(root, ".claude", "skills"),
		"SYNAPTIC_PROJECT_ROOT": root,
		"SC_DOLT_BRANCH":        "main",
		"SYNAPTIC_AGENTS":       "claude",
	}

	lock, err := installer.LoadManifestLock(root)
	if err != nil {
		return initResponse{}, err
	}
	if err := installer.SaveManifestLock(root, lock); err != nil {
		return initResponse{}, err
	}
	if err := installer.SaveRepoProfile(root, profile); err != nil {
		return initResponse{}, err
	}
	if err := installer.SaveEnv(root, env); err != nil {
		return initResponse{}, err
	}
	if err := installer.SaveHookRegistry(root, installer.HookRegistry{}); err != nil {
		return initResponse{}, err
	}

	for _, rel := range stateFiles {
		if existedBefore[rel] {
			resp.Refreshed = append(resp.Refreshed, rel)
		} else {
			resp.Created = append(resp.Created, rel)
		}
	}

	return resp, nil
}

func existed(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return fmt.Sprintf("%v", values)
}
