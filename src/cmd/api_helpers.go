package cmd

import (
	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
	"github.com/randlee/synaptic-canvas-dolt/pkg/installer"
	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func apiInstallScope(scope models.InstallScope) api.InstallScope {
	return api.InstallScope(scope)
}

func apiDependencyType(depType models.DepType) api.DependencyType {
	return api.DependencyType(depType)
}

func apiInstallAnswers(values map[string]any) *api.InstallAnswers {
	if len(values) == 0 {
		return nil
	}
	copied := make(map[string]any, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return &api.InstallAnswers{Values: copied}
}

func apiInstallHookEntries(entries []installer.HookEntry) []api.InstallHookEntry {
	if len(entries) == 0 {
		return nil
	}
	result := make([]api.InstallHookEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, api.InstallHookEntry{
			Event:    entry.Event,
			Matcher:  entry.Matcher,
			Skill:    entry.Skill,
			Scope:    entry.Scope,
			Script:   entry.Script,
			Priority: entry.Priority,
			Blocking: entry.Blocking,
		})
	}
	return result
}

func apiInstallPlannedFiles(files []installer.PlannedFile) []api.InstallPlannedFile {
	if len(files) == 0 {
		return nil
	}
	result := make([]api.InstallPlannedFile, 0, len(files))
	for _, file := range files {
		result = append(result, api.InstallPlannedFile{
			Path:       file.Path,
			IsTemplate: file.IsTemplate,
			Preview:    file.Preview,
		})
	}
	return result
}

func apiInstallSummary(summary installer.Summary) api.InstallSummary {
	return api.InstallSummary{
		PackageID:                  summary.PackageID,
		Version:                    summary.Version,
		Branch:                     summary.Branch,
		Scope:                      summary.Scope,
		InstallRoot:                summary.InstallRoot,
		FilesWritten:               summary.FilesWritten,
		Dependencies:               summary.Dependencies,
		DependencyWarnings:         summary.DependencyWarnings,
		HooksRegistered:            apiInstallHookEntries(summary.HooksRegistered),
		TemplateValidationWarnings: summary.TemplateValidationWarnings,
		Warnings:                   append([]string(nil), summary.Warnings...),
		Files:                      apiInstallPlannedFiles(summary.Files),
		Answers:                    apiInstallAnswers(summary.Answers),
	}
}

func apiInstallSummaries(summaries []installer.Summary) []api.InstallSummary {
	if len(summaries) == 0 {
		return nil
	}
	result := make([]api.InstallSummary, 0, len(summaries))
	for _, summary := range summaries {
		result = append(result, apiInstallSummary(summary))
	}
	return result
}
