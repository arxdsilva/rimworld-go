// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"path/filepath"
	"testing"

	"rimworld-go/internal/adapters/outbound/modloader"
)

// The example plugin's item hands its own PNG to the presentation
// adapter, keyed as its item views name it; core content hands none.
func TestItemTexturesFromEnabledPlugin(t *testing.T) {
	src := modloader.New(filepath.Join("..", "..", "..", "mods"))
	on := buildContent(src, []string{"example_frontier"}, "0.1.35").itemTextures()
	png := on["example_frontier.trail_bell"]
	if len(on) != 1 || !bytes.HasPrefix(png, []byte("\x89PNG")) {
		t.Fatalf("textures %d, bell %d bytes", len(on), len(png))
	}
	if off := buildContent(src, nil, "0.1.35").itemTextures(); off != nil {
		t.Fatalf("core hands textures: %d", len(off))
	}
}
