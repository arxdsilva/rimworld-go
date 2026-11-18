// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Command audiogen renders the game's procedural sounds and music; see
// internal/adapters/inbound/cli/audiogen.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/audiogen"
)

func main() {
	os.Exit(audiogen.Run(os.Args[1:], os.Stdout, os.Stderr))
}
