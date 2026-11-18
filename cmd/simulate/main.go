// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Command simulate runs a headless colony; see cli/simulate.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/simulate"
)

func main() {
	os.Exit(simulate.Main(os.Args, os.Stdout, os.Stderr))
}
