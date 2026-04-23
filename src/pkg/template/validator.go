package template

import (
	"fmt"
	"regexp"
	"sort"
)

var templateBlockPattern = regexp.MustCompile(`(?s)\{\{.*?\}\}|\{%.*?%\}`)
var refPattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\b`)

var knownRepoKeys = map[string]struct{}{
	"name": {}, "root": {}, "primary_language": {}, "languages": {}, "frameworks": {},
	"test_frameworks": {}, "ci_system": {}, "monorepo": {}, "git_conventions": {},
}

var knownEnvKeys = map[string]struct{}{
	"synaptic_root": {}, "synaptic_channel": {}, "synaptic_agents": {},
}

// Finding represents one validator result.
type Finding struct {
	Severity string `json:"severity"`
	File     string `json:"file"`
	Variable string `json:"variable"`
	Message  string `json:"message"`
}

// Report holds validation findings.
type Report struct {
	Errors   []Finding `json:"errors"`
	Warnings []Finding `json:"warnings"`
}

// Validate checks all template variable references.
func Validate(files map[string]string, questionIDs []string) Report {
	questionSet := make(map[string]struct{}, len(questionIDs))
	for _, id := range questionIDs {
		questionSet[id] = struct{}{}
	}

	usedQuestions := make(map[string]struct{})
	report := Report{}

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		seen := map[string]struct{}{}
		for _, block := range templateBlockPattern.FindAllString(files[path], -1) {
			for _, match := range refPattern.FindAllStringSubmatch(block, -1) {
				full := match[0]
				if _, ok := seen[full]; ok {
					continue
				}
				seen[full] = struct{}{}
				namespace := match[1]
				key := match[2]

				switch namespace {
				case "answers":
					if _, ok := questionSet[key]; !ok {
						report.Errors = append(report.Errors, Finding{
							Severity: "error",
							File:     path,
							Variable: full,
							Message:  fmt.Sprintf("answers.%s has no matching package question", key),
						})
						continue
					}
					usedQuestions[key] = struct{}{}
				case "repo":
					if _, ok := knownRepoKeys[key]; !ok {
						report.Errors = append(report.Errors, Finding{
							Severity: "error",
							File:     path,
							Variable: full,
							Message:  fmt.Sprintf("repo.%s is not in the known repo schema", key),
						})
					}
				case "env":
					if _, ok := knownEnvKeys[key]; !ok {
						report.Errors = append(report.Errors, Finding{
							Severity: "error",
							File:     path,
							Variable: full,
							Message:  fmt.Sprintf("env.%s is not in the known env schema", key),
						})
					}
				default:
					report.Errors = append(report.Errors, Finding{
						Severity: "error",
						File:     path,
						Variable: full,
						Message:  fmt.Sprintf("unknown variable namespace %q", namespace),
					})
				}
			}
		}
	}

	for _, id := range questionIDs {
		if _, ok := usedQuestions[id]; ok {
			continue
		}
		report.Warnings = append(report.Warnings, Finding{
			Severity: "warning",
			Variable: "answers." + id,
			Message:  fmt.Sprintf("question %q is declared but not referenced by any template", id),
		})
	}

	return report
}
