package plugin

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/randlee/synaptic-canvas-dolt/pkg/models"
)

type pluginManifest struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Version     string       `json:"version"`
	Author      string       `json:"author,omitempty"`
	License     string       `json:"license,omitempty"`
	Keywords    []string     `json:"keywords,omitempty"`
	Commands    []PluginItem `json:"commands,omitempty"`
	Agents      []PluginItem `json:"agents,omitempty"`
	Skills      []PluginItem `json:"skills,omitempty"`
}

type PluginItem struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Reconstruct renders plugin.json content from package metadata and file rows.
func Reconstruct(pkg *models.Package, files []models.PackageFile) (string, error) {
	if pkg == nil {
		return "", fmt.Errorf("reconstructing plugin: package is nil")
	}

	manifest := pluginManifest{
		Name:    pkg.Name,
		Version: pkg.Version,
		Author:  "synaptic-canvas",
	}
	if pkg.Description != nil {
		manifest.Description = *pkg.Description
	}
	if pkg.Author != nil && *pkg.Author != "" {
		manifest.Author = *pkg.Author
	}
	if pkg.License != nil {
		manifest.License = *pkg.License
	}
	manifest.Keywords = pkg.TagsList()

	for _, file := range files {
		item := PluginItem{
			Name:        pluginItemName(file),
			Description: pluginItemDescription(file),
		}
		switch file.FileType {
		case models.FileTypeCommand:
			manifest.Commands = append(manifest.Commands, item)
		case models.FileTypeAgent:
			manifest.Agents = append(manifest.Agents, item)
		case models.FileTypeSkill:
			manifest.Skills = append(manifest.Skills, item)
		}
	}

	sort.Slice(manifest.Commands, func(i, j int) bool { return manifest.Commands[i].Name < manifest.Commands[j].Name })
	sort.Slice(manifest.Agents, func(i, j int) bool { return manifest.Agents[i].Name < manifest.Agents[j].Name })
	sort.Slice(manifest.Skills, func(i, j int) bool { return manifest.Skills[i].Name < manifest.Skills[j].Name })

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("rendering plugin json: %w", err)
	}
	return string(data) + "\n", nil
}

func pluginItemName(file models.PackageFile) string {
	if file.FMName != nil && *file.FMName != "" {
		return *file.FMName
	}
	return file.DestPath
}

func pluginItemDescription(file models.PackageFile) string {
	if file.FMDescription != nil {
		return *file.FMDescription
	}
	return ""
}
