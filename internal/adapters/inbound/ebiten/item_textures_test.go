// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

// The composition root's plugin item textures (AppConfig.ItemTextures,
// PLG-026) land in the catalog under the sprite key the item's view
// draws; a broken one adds nothing.
func TestItemTexturesReachTheCatalog(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	c := &art.Catalog{}
	addItemTextures(c, map[string][]byte{"frontier.trail_bell": buf.Bytes(), "frontier.broken": []byte("x")})
	if !c.Has("item/frontier.trail_bell") || c.Has("item/frontier.broken") || len(c.Sprites) != 1 {
		t.Fatalf("sprites %v", c.Sprites)
	}
}
