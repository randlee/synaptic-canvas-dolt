package operations

import "github.com/randlee/synaptic-canvas-dolt/pkg/api"

// Modifications returns non-OK file items for status/readback surfaces.
func Modifications(items []api.ValidationItem) []api.ValidationItem {
	result := make([]api.ValidationItem, 0, len(items))
	for _, item := range items {
		if item.Kind == api.ValidationKindFile && item.State != api.ValidationStateOK {
			result = append(result, item)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// Issues returns non-file validation items for status/readback surfaces.
func Issues(items []api.ValidationItem) []api.ValidationItem {
	result := make([]api.ValidationItem, 0, len(items))
	for _, item := range items {
		if item.Kind != api.ValidationKindFile {
			result = append(result, item)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
