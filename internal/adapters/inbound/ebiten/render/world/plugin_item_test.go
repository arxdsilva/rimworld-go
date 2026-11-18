// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package world

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"rimworld-go/internal/application/game"
)

// A plugin item (PLG-026) is drawn with its plugin's own texture: the
// view names it by its key, its sprite is the texture the composition
// root handed the catalog, and the stack draws it, not a stand-in.
func TestPluginItemDrawsItsTexture(t *testing.T) {
	t.Parallel()
	pink := color.NRGBA{250, 20, 200, 255}
	img := image.NewNRGBA(image.Rect(0, 0, 24, 16))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = pink.R, pink.G, pink.B, pink.A
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	w := rigWorld(16, 12)
	w.Items = []game.ItemView{{ID: 3, Kind: "frontier.copper_bell", Qty: 2, X: 4, Y: 4}}
	r := newRig(t, w, 16, 12)
	key := ItemSprite("frontier.copper_bell")
	if key != "item/frontier.copper_bell" {
		t.Fatalf("plugin item key %q", key)
	}
	if err := r.p.art().AddPNG(key, []byte("not a png")); err == nil || r.p.art().Has(key) {
		t.Fatal("a broken texture was taken")
	}
	if err := r.p.art().AddPNG(key, buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if m := r.p.art().MetaOf(key); m == nil || m.Canvas != [2]int{24, 16} || m.PxPerTile() != 24 {
		t.Fatalf("meta %+v", m)
	}
	r.p.art().Image(key, color.NRGBA{}) // the GPU image neatDraws maps back to the key
	if got := neatDraws(r); indexOf(got, key) < 0 {
		t.Fatalf("drawn %v, want the plugin texture %s", got, key)
	}
}
