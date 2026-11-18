// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Command mapgen renders the MAPGEN-2 field generator's debug images; the
// logic lives in internal/adapters/inbound/cli/mapgen.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/mapgen"
)

func main() { os.Exit(mapgen.Main(os.Args[1:], os.Stdout, os.Stderr)) }
