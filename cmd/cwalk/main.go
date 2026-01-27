// Package main provides the entry point for the cwalk CLI tool.
//
// The cwalk command performs directory tree analysis with advanced filtering,
// aggregation modes, and flexible output formats. It provides statistics about
// file sizes, counts by type, and per-year or per-owner breakdowns.
//
// Usage:
//
//	cwalk [flags] [paths...]
//
// Examples:
//
//	cwalk .
//	cwalk --output-mode=per-year --size-min=1G /var/log
//	cwalk --type=file --name='\.log$' --output-format=json /var
//
// For more information, see cmd/cwalk/README.md or run: cwalk --help
package main

import (
	"log"
	"os"

	"github.com/otuschhoff/cwalk/cmd/cwalk/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
