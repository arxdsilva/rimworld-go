// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/application/scripting"
)

// scripts/**.wasm lands in Plugin.Wasm, sorted, bytes intact; .star stays in
// Scripts and a .wasm outside scripts/ is ignored (PLG-063).
func TestWasmScriptsCollected(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	write(t, root, "demo/scripts/a.star", "x = 1\n")
	write(t, root, "demo/scripts/b.wasm", "\x00asm-b")
	write(t, root, "demo/scripts/sub/c.wasm", "\x00asm-c")
	write(t, root, "demo/defs/x.wasm", "\x00asm-x")
	got := load(t, root)[0]
	if len(got.Scripts) != 1 || got.Scripts[0].Path != "scripts/a.star" {
		t.Fatalf("scripts %+v", got.Scripts)
	}
	if len(got.Wasm) != 2 || got.Wasm[0].Path != "scripts/b.wasm" || got.Wasm[1].Path != "scripts/sub/c.wasm" {
		t.Fatalf("wasm %+v", got.Wasm)
	}
	if string(got.Wasm[0].Data) != "\x00asm-b" || string(got.Wasm[1].Data) != "\x00asm-c" {
		t.Fatalf("bytes %q %q", got.Wasm[0].Data, got.Wasm[1].Data)
	}
	if len(got.Problems) != 0 {
		t.Fatalf("problems %+v", got.Problems)
	}
}

// A module of exactly the cap loads; one byte more is a Problem and skipped.
func TestWasmCap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	write(t, root, "demo/scripts/ok.wasm", strings.Repeat("x", scripting.MaxWasmBytes))
	write(t, root, "demo/scripts/big.wasm", strings.Repeat("x", scripting.MaxWasmBytes+1))
	got := load(t, root)[0]
	if len(got.Wasm) != 1 || got.Wasm[0].Path != "scripts/ok.wasm" || len(got.Wasm[0].Data) != scripting.MaxWasmBytes {
		t.Fatalf("wasm %d files", len(got.Wasm))
	}
	if len(got.Problems) != 1 || got.Problems[0].File != "scripts/big.wasm" {
		t.Fatalf("problems %+v", got.Problems)
	}
}

// .wasm files count in the 2,000-file trim (after scripts, before textures)
// and in the 32 MiB plugin total.
func TestWasmCountsTowardCaps(t *testing.T) {
	t.Parallel()
	t.Run("files", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		write(t, root, "demo/plugin.json", goodManifest)
		write(t, root, "demo/scripts/a.star", "x = 1\n")
		write(t, root, "demo/textures/t.png", "\x89PNG")
		for i := range maxDataFiles {
			write(t, root, fmt.Sprintf("demo/scripts/m%04d.wasm", i), "w")
		}
		got := load(t, root)[0]
		if len(got.Scripts) != 1 || len(got.Wasm) != maxDataFiles-1 || len(got.Textures) != 0 {
			t.Fatalf("scripts %d wasm %d textures %d", len(got.Scripts), len(got.Wasm), len(got.Textures))
		}
		if len(got.Problems) != 1 || !strings.Contains(got.Problems[0].Msg, "too many data files") {
			t.Fatalf("problems %+v", got.Problems)
		}
	})
	t.Run("bytes", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		write(t, root, "demo/plugin.json", goodManifest)
		n := maxPluginBytes / scripting.MaxWasmBytes
		body := strings.Repeat("x", scripting.MaxWasmBytes)
		for i := range n + 1 {
			write(t, root, fmt.Sprintf("demo/scripts/m%02d.wasm", i), body)
		}
		got := load(t, root)[0]
		if len(got.Wasm) != n {
			t.Fatalf("wasm %d, want %d", len(got.Wasm), n)
		}
		if len(got.Problems) != 1 || !strings.Contains(got.Problems[0].Msg, "MiB") {
			t.Fatalf("problems %+v", got.Problems)
		}
	})
}

// walk reports a dangling link, follows a link to a folder inside the plugin
// inside the plugin (PLG-063 quality pass).
func TestWalkLinks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	write(t, root, "demo/real/m.wasm", "w")
	if err := os.Symlink("nowhere.wasm", filepath.Join(root, "demo/scripts-dangling")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "demo/scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("nowhere.wasm", filepath.Join(root, "demo/scripts/dead.wasm")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../real", filepath.Join(root, "demo/scripts/linked")); err != nil {
		t.Fatal(err)
	}
	got := load(t, root)[0]
	if len(got.Wasm) == 0 || got.Wasm[0].Path != "scripts/linked/m.wasm" {
		t.Fatalf("wasm %+v", got.Wasm)
	}
	if !hasProblem(got, "scripts/dead.wasm", "dangling link") {
		t.Fatalf("problems %+v", got.Problems)
	}
}

func hasAnyMsg(p modding.Plugin, sub string) bool {
	for _, pr := range p.Problems {
		if strings.Contains(pr.Msg, sub) {
			return true
		}
	}
	return false
}

// Folders nested past maxDepth stop with one problem.
func TestWalkDepthIsCapped(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	deep := "demo/scripts" + strings.Repeat("/d", maxDepth+2)
	write(t, root, deep+"/m.wasm", "w")
	got := load(t, root)[0]
	if len(got.Wasm) != 0 || !hasAnyMsg(got, "nested too deeply") {
		t.Fatalf("wasm %d problems %+v", len(got.Wasm), got.Problems)
	}
}
