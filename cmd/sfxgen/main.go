// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Command sfxgen makes the animal voices; see
// internal/adapters/inbound/cli/sfxgen.
package main

import (
	"os"

	"rimworld-go/internal/adapters/inbound/cli/sfxgen"
)

func main() {
	os.Exit(sfxgen.Run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}
