// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A plugin's data files together stop at maxPluginBytes: past it one
// problem is reported and the rest is not read (PLG-004).
func TestPluginDataTotalIsCapped(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "big")
	if err := os.MkdirAll(filepath.Join(dir, "defs"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"big","name":"Big","version":"1","game":["0.1"]}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `{"note":"` + strings.Repeat("a", maxDataFileBytes-64) + `"}`
	files := maxPluginBytes/len(body) + 2
	for i := range files {
		if err := os.WriteFile(filepath.Join(dir, "defs", fmt.Sprintf("f%02d.json", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ps, err := New(root).Plugins()
	if err != nil || len(ps) != 1 {
		t.Fatalf("plugins %d, %v", len(ps), err)
	}
	p := ps[0]
	if len(p.Defs) >= files || len(p.Defs)*len(body) > maxPluginBytes {
		t.Fatalf("read %d of %d files (%d bytes each) past the cap", len(p.Defs), files, len(body))
	}
	capped := 0
	for _, pr := range p.Problems {
		if strings.Contains(pr.Msg, "MiB") {
			capped++
		}
	}
	if capped != 1 {
		t.Fatalf("problems = %v", p.Problems)
	}
}
