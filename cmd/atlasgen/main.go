// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Command atlasgen packs one sprite family into texture atlas pages; the
// logic lives in internal/adapters/inbound/cli/atlasgen.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/atlasgen"
)

func main() { os.Exit(atlasgen.Main(os.Args[1:], os.Stdout, os.Stderr)) }
