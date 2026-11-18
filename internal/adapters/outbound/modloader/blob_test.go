// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modloader

import (
	"strings"
	"testing"
)

// Scripts (scripts/**.star) and textures (textures/**.png) are read as
// they are, sorted, other files beside them skipped, each under its cap.
func TestScriptsAndTexturesAreReadRaw(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	write(t, root, "demo/scripts/main.star", "def on_day(ctx):\n    pass\n")
	write(t, root, "demo/scripts/lib/a.star", "x = 1\n")
	write(t, root, "demo/scripts/notes.txt", "skip")
	write(t, root, "demo/textures/items/brick.png", "\x89PNG")
	write(t, root, "demo/textures/big.png", strings.Repeat("x", maxTextureBytes+1))
	got := load(t, root)[0]
	var scripts, textures []string
	for _, b := range got.Scripts {
		scripts = append(scripts, b.Path)
	}
	for _, b := range got.Textures {
		textures = append(textures, b.Path)
	}
	if strings.Join(scripts, ",") != "scripts/lib/a.star,scripts/main.star" || string(got.Scripts[1].Data) != "def on_day(ctx):\n    pass\n" {
		t.Fatalf("scripts %v", scripts)
	}
	if strings.Join(textures, ",") != "textures/items/brick.png" {
		t.Fatalf("textures %v", textures)
	}
	if len(got.Problems) != 1 || got.Problems[0].File != "textures/big.png" {
		t.Fatalf("problems %+v, want the oversized texture", got.Problems)
	}
}
