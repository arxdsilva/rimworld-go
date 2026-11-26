// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Command replay verifies a recorded colony run; see cli/replay.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/replay"
)

func main() {
	os.Exit(replay.Main(os.Args, os.Stdout, os.Stderr))
}
