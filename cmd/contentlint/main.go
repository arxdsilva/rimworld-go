// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Command contentlint checks plugins without starting the game (PLG-040);
// see cli/contentlint.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/contentlint"
	"rimworld-go/internal/adapters/outbound/modloader"
	"rimworld-go/internal/platform/buildinfo"
)

func main() {
	os.Exit(contentlint.Main(os.Args[1:], os.Stdout, os.Stderr, modloader.Open, modloader.DefaultRoots(), buildinfo.Version))
}
