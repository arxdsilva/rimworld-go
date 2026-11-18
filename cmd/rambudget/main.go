// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Command rambudget prints the parallel-agent budget; see cli/rambudget.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/rambudget"
)

func main() {
	os.Exit(rambudget.Main(os.Stdout, os.Stderr))
}
