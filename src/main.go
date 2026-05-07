package main

import (
	"os"

	"github.com/randlee/synaptic-canvas-dolt/cmd"
)

// version, commit, and date are set via ldflags at build time.
//
//nolint:gochecknoglobals // these are set via ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := cmd.Execute(version, commit, date); err != nil {
		if cmd.IsJSONCmdError(err) {
			os.Exit(1)
		}
		os.Exit(1)
	}
}
