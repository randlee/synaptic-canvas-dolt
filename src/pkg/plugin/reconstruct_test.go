package plugin

import (
	"encoding/json"
	"testing"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

func TestReconstruct(t *testing.T) {
	t.Parallel()

	desc := "Manage Synaptic Canvas packages."
	author := "randlee"
	license := "MIT"
	pkg := &models.Package{
		ID:          "sc-manage",
		Name:        "sc-manage",
		Version:     "0.9.0",
		Description: &desc,
		Author:      &author,
		License:     &license,
		Tags:        "management,packages",
	}
	files := []models.PackageFile{
		{DestPath: "commands/sc-manage.md", FileType: models.FileTypeCommand, FMName: stringPtr("sc-manage"), FMDescription: stringPtr("Manage packages")},
		{DestPath: "agents/sc-packages-list.md", FileType: models.FileTypeAgent, FMName: stringPtr("sc-packages-list"), FMDescription: stringPtr("List packages")},
		{DestPath: "skills/managing-sc-packages/SKILL.md", FileType: models.FileTypeSkill, FMName: stringPtr("managing-sc-packages"), FMDescription: stringPtr("Skill docs")},
	}

	got, err := Reconstruct(pkg, files)
	if err != nil {
		t.Fatalf("Reconstruct() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded["name"] != "sc-manage" {
		t.Fatalf("name = %v", decoded["name"])
	}
	if decoded["author"] != "randlee" {
		t.Fatalf("author = %#v", decoded["author"])
	}
	commands, ok := decoded["commands"].([]any)
	if !ok || len(commands) != 1 {
		t.Fatalf("commands = %#v", decoded["commands"])
	}
	command, ok := commands[0].(map[string]any)
	if !ok || command["name"] != "sc-manage" || command["description"] != "Manage packages" {
		t.Fatalf("command = %#v", commands[0])
	}
}

func stringPtr(v string) *string {
	return &v
}
