// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mapgen

import (
	"flag"
	"fmt"
	"io"
)

// Main runs the tool with the command-line args and returns the exit code.
func Main(args []string, out, errw io.Writer) int {
	o, err := ParseArgs(args, errw)
	if err != nil {
		if err != flag.ErrHelp {
			fmt.Fprintln(errw, "mapgen:", err)
		}
		return 2
	}
	if err := Run(o, out); err != nil {
		fmt.Fprintln(errw, "mapgen:", err)
		return 1
	}
	return 0
}
