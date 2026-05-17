package cmd

import "github.com/randlee/synaptic-canvas-dolt/internal/output"

func writeWarning(formatter *output.Formatter, warning string) {
	formatter.Warn(warning)
}
