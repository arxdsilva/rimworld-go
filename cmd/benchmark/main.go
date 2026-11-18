// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Command benchmark runs the engine-scale fixtures; see cli/benchmark.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/benchmark"
)

func main() {
	os.Exit(benchmark.Main(os.Args[1:], os.Stdout, os.Stderr))
}
