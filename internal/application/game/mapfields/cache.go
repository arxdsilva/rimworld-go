// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mapfields

import (
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/site"
)

// Cache owns the regenerated field layers. A colony keeps it off the
// saved state; the live roof, fog and region layers are read each time.
type Cache struct{ generated []Layer }

// Layers returns cached generated layers followed by fresh live layers.
func (c *Cache) Layers(seed uint64, w, h int, at site.Site, roofs []roof.Cell,
	fogged, walkable func(x, y int) bool) ([]Layer, error) {
	if c.generated == nil {
		layers, err := Generate(seed, w, h, at)
		if err != nil {
			return nil, err
		}
		c.generated = layers
	}
	out := append([]Layer(nil), c.generated...)
	return append(out, Roofs(w, h, roofs), Fog(w, h, fogged), Regions(w, h, walkable)), nil
}
