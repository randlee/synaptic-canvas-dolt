package plugin

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

type pluginManifest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version"`
	Author      map[string]string `json:"author"`
	License     string            `json:"license,omitempty"`
	Keywords    []string          `json:"keywords,omitempty"`
	Commands    []string          `json:"commands,omitempty"`
	Agents      []string          `json:"agents,omitempty"`
	Skills      []string          `json:"skills,omitempty"`
}

// Reconstruct renders plugin.json content from package metadata and file rows.
func Reconstruct(pkg *models.Package, files []models.PackageFile) (string, error) {
	if pkg == nil {
		return "", fmt.Errorf("reconstructing plugin: package is nil")
	}

	manifest := pluginManifest{
		Name:    pkg.Name,
		Version: pkg.Version,
		Author:  map[string]string{"name": "synaptic-canvas"},
	}
	if pkg.Description != nil {
		manifest.Description = *pkg.Description
	}
	if pkg.Author != nil && *pkg.Author != "" {
		manifest.Author = map[string]string{"name": *pkg.Author}
	}
	if pkg.License != nil {
		manifest.License = *pkg.License
	}
	manifest.Keywords = pkg.TagsList()

	for _, file := range files {
		path := "./" + file.DestPath
		switch file.FileType {
		case models.FileTypeCommand:
			manifest.Commands = append(manifest.Commands, path)
		case models.FileTypeAgent:
			manifest.Agents = append(manifest.Agents, path)
		case models.FileTypeSkill:
			manifest.Skills = append(manifest.Skills, path)
		}
	}

	sort.Strings(manifest.Commands)
	sort.Strings(manifest.Agents)
	sort.Strings(manifest.Skills)

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("rendering plugin json: %w", err)
	}
	return string(data) + "\n", nil
}
