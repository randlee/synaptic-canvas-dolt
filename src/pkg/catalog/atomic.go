package catalog

import "github.com/randlee/synaptic-canvas-dolt/internal/atomicfile"

func writeTOMLAtomic(path string, value any) error {
	return atomicfile.WriteTOML(path, value, 0o700)
}
