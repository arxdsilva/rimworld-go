// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package art

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

// AddPNG adds a plugin item's texture (PLG-026) to the catalog under key:
// the PNG's pixels, as they are, filling one tile (its longer side is a
// tile; the shorter one centred). The plugin pipeline already refused a
// texture that does not decode; one that fails here is an error and adds
// nothing, never a stand-in sprite.
func (c *Catalog) AddPNG(key string, data []byte) error {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("plugin texture %s: %w", key, err)
	}
	pix := image.NewNRGBA(image.Rect(0, 0, src.Bounds().Dx(), src.Bounds().Dy()))
	draw.Draw(pix, pix.Bounds(), src, src.Bounds().Min, draw.Src)
	w, h := pix.Bounds().Dx(), pix.Bounds().Dy()
	side := float64(max(w, h))
	if c.Sprites == nil {
		c.Sprites = map[string]*Sprite{}
	}
	c.Sprites[key] = &Sprite{Pix: pix, Meta: Meta{ID: "plugin:" + key, Canvas: [2]int{w, h}, Footprint: [2]int{1, 1},
		Origin: [2]float64{-(side - float64(w)) / 2, -(side - float64(h)) / 2}, PPT: side}}
	return nil
}
