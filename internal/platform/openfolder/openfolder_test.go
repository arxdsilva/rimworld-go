// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package openfolder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandPassesThePathAsOneArgument(t *testing.T) {
	const p = `/a b/c; rm -rf /`
	for _, os := range []string{"darwin", "windows", "linux"} {
		name, args, err := command(os, p)
		if err != nil || name == "" || len(args) != 1 || args[0] != p {
			t.Errorf("%s: %q %q %v", os, name, args, err)
		}
	}
	if _, _, err := command("plan9", p); err == nil {
		t.Error("an unknown OS must be an error")
	}
}

func TestOpenRefusesEmpty(t *testing.T) {
	if Open("") == nil {
		t.Fatal("empty path accepted")
	}
}

// With no opener on PATH, Start fails before anything runs, so no window opens.
func TestOpenReportsAMissingOpener(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := filepath.Join(t.TempDir(), "saves")
	err := Open(dir)
	if err == nil || !strings.Contains(err.Error(), "openfolder:") {
		t.Fatalf("Open = %v, want an openfolder error", err)
	}
	if fi, statErr := os.Stat(dir); statErr != nil || !fi.IsDir() {
		t.Errorf("folder not created: %v", statErr)
	}
}
