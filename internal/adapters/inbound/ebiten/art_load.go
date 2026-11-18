// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"log"
	"maps"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"slices"
	"sync"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

// The sprite catalog lives in art (ARCH-003). These aliases and one-line
// wrappers keep the root's call sites unchanged while its features move out
// one package at a time (SOLID_GO.md §11: no forwarder outlives the commit
// series that repoints its callers).

func loadArtCatalog(root string) (*art.Catalog, error) {
	c, err := art.Load(root)
	if c != nil {
		c.ReadBack = readBack
	}
	return c, err
}

// cpuReadBack names the sprites the renderer's bakes read back on the CPU
// (crop units and fallbacks, grass tufts); the catalog keeps their pixels.
var cpuReadBack = sync.OnceValue(func() map[string]bool {
	m := map[string]bool{}
	for _, ks := range world.PlantKeys.Crops {
		for st := range ks {
			for _, k := range ks[st] {
				m[k] = true
				for _, fb := range world.PlantKeys.Fallback[k] {
					m[fb] = true
				}
			}
		}
	}
	for _, set := range [][]string{env.GrassShortKeys, env.GrassTallKeys, env.GrassDormantKeys} {
		for _, k := range set {
			m[k] = true
		}
	}
	return m
})

func readBack(key string) bool { return cpuReadBack()[key] }

// addItemTextures adds the plugin items' textures (AppConfig.ItemTextures,
// PLG-026) to c under their sprite keys, in key order, before anything
// else reads the catalog. Build refused a texture that does not decode
// (the same decoder), so a failure here is logged, and no sprite stands in.
func addItemTextures(c *art.Catalog, tex map[string][]byte) {
	for _, k := range slices.Sorted(maps.Keys(tex)) {
		if err := c.AddPNG(world.ItemSprite(k), tex[k]); err != nil {
			log.Print(err)
		}
	}
}
