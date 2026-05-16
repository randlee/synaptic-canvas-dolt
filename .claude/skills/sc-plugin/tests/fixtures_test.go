package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/api"
)

var (
	commandBlockPattern = regexp.MustCompile("(?s)```bash\\n(.*?)\\n```")
	jsonBlockPattern    = regexp.MustCompile("(?s)```json\\n(.*?)\\n```")
	mappingRulePattern  = regexp.MustCompile(`- "([^"]+)" -> ` + "`" + `([^` + "`" + `]+)` + "`")
)

func TestSkillMappingsMatchNormativeExamples(t *testing.T) {
	mappings := readSkillMappings(t)
	for _, example := range exampleFixtures(t) {
		if got := mappings[example.utterance]; got != example.command {
			t.Fatalf("mapping mismatch for %q: got %q want %q", example.utterance, got, example.command)
		}
	}
}

func TestExampleCommandsAlwaysUseJSON(t *testing.T) {
	for _, example := range exampleFixtures(t) {
		if !strings.Contains(example.command, "--json") {
			t.Fatalf("%s command %q does not include --json", example.path, example.command)
		}
	}
}

func TestExampleJSONMatchesSharedContractTypes(t *testing.T) {
	for _, example := range exampleFixtures(t) {
		t.Run(filepath.Base(example.path), func(t *testing.T) {
			example.assertJSON(t)
		})
	}
}

type exampleFixture struct {
	path       string
	utterance  string
	command    string
	jsonBody   string
	assertJSON func(*testing.T)
}

func exampleFixtures(t *testing.T) []exampleFixture {
	t.Helper()
	return []exampleFixture{
		loadExampleFixture(t, "backend-failure-http.md", func(t *testing.T, payload string) {
			var response api.ErrorEnvelope
			decodeJSONFixture(t, payload, &response)
			if response.OK {
				t.Fatal("expected error envelope")
			}
			if response.Error.Code != api.ErrorCodeBackendUnavailable {
				t.Fatalf("unexpected error code: %s", response.Error.Code)
			}
			if response.Error.Details["client"] != "http" {
				t.Fatalf("unexpected client detail: %#v", response.Error.Details["client"])
			}
		}),
		loadExampleFixture(t, "install-beta-global.md", func(t *testing.T, payload string) {
			var response api.InstallResponse
			decodeJSONFixture(t, payload, &response)
			if !response.OK || response.Scope != "global" {
				t.Fatalf("unexpected install response: %+v", response)
			}
			if response.Package == nil || response.Package.Branch != "beta" {
				t.Fatalf("unexpected package payload: %+v", response.Package)
			}
		}),
		loadExampleFixture(t, "snapshot-ambiguous.md", func(t *testing.T, payload string) {
			var response api.ErrorEnvelope
			decodeJSONFixture(t, payload, &response)
			if response.Error.Code != api.ErrorCodeAmbiguousTarget {
				t.Fatalf("unexpected error code: %s", response.Error.Code)
			}
		}),
		loadExampleFixture(t, "status-local-global.md", func(t *testing.T, payload string) {
			var response api.StatusResponse
			decodeJSONFixture(t, payload, &response)
			if !response.OK || len(response.Packages) != 1 {
				t.Fatalf("unexpected status response: %+v", response)
			}
			pkg := response.Packages[0]
			if pkg.Global == nil || pkg.Local == nil {
				t.Fatalf("expected both scopes to be present: %+v", pkg)
			}
		}),
		loadExampleFixture(t, "uninstall-global.md", func(t *testing.T, payload string) {
			var response api.UninstallResponse
			decodeJSONFixture(t, payload, &response)
			if !response.OK || response.Removed.Scope != "global" {
				t.Fatalf("unexpected uninstall response: %+v", response)
			}
		}),
		loadExampleFixture(t, "upgrade-version-project.md", func(t *testing.T, payload string) {
			var response api.UpgradeResponse
			decodeJSONFixture(t, payload, &response)
			if !response.OK || len(response.Upgrades) != 1 {
				t.Fatalf("unexpected upgrade response: %+v", response)
			}
			if response.Upgrades[0].Scope != "project" {
				t.Fatalf("unexpected upgrade scope: %+v", response.Upgrades[0])
			}
		}),
		loadExampleFixture(t, "validate-project.md", func(t *testing.T, payload string) {
			var response api.ValidateResponse
			decodeJSONFixture(t, payload, &response)
			if !response.OK || response.Pass {
				t.Fatalf("unexpected validate response: %+v", response)
			}
			if len(response.Packages) != 1 || len(response.Packages[0].Items) != 2 {
				t.Fatalf("unexpected validation items: %+v", response.Packages)
			}
			for _, item := range response.Packages[0].Items {
				if item.Severity == "" {
					t.Fatalf("validation item missing severity: %+v", item)
				}
			}
		}),
	}
}

func loadExampleFixture(t *testing.T, name string, assertFn func(*testing.T, string)) exampleFixture {
	t.Helper()
	path := filepath.Join(skillRoot(t), "examples", name)
	body := readFile(t, path)
	return exampleFixture{
		path:       path,
		utterance:  extractSingleTextBlock(t, body),
		command:    extractSingleBlock(t, commandBlockPattern, body, "bash"),
		jsonBody:   extractSingleBlock(t, jsonBlockPattern, body, "json"),
		assertJSON: func(t *testing.T) { assertFn(t, extractSingleBlock(t, jsonBlockPattern, body, "json")) },
	}
}

func readSkillMappings(t *testing.T) map[string]string {
	t.Helper()
	body := readFile(t, filepath.Join(skillRoot(t), "SKILL.md"))
	matches := mappingRulePattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		t.Fatal("no command mappings found in SKILL.md")
	}
	result := make(map[string]string, len(matches))
	for _, match := range matches {
		result[match[1]] = strings.TrimSpace(match[2])
	}
	return result
}

func extractSingleTextBlock(t *testing.T, body string) string {
	t.Helper()
	return extractSingleBlock(t, regexp.MustCompile("(?s)```text\\n(.*?)\\n```"), body, "text")
}

func extractSingleBlock(t *testing.T, pattern *regexp.Regexp, body, blockType string) string {
	t.Helper()
	matches := pattern.FindAllStringSubmatch(body, -1)
	if len(matches) != 1 {
		t.Fatalf("expected exactly one %s block, found %d", blockType, len(matches))
	}
	return strings.TrimSpace(matches[0][1])
}

func decodeJSONFixture(t *testing.T, payload string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(payload), target); err != nil {
		t.Fatalf("unmarshal fixture JSON: %v", err)
	}
}

func skillRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(wd)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
