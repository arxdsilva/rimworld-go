// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"strings"

	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/domain/economy/item"
)

// itemTextures are the run's plugin items' textures (PLG-026): each
// plugin kind's PNG bytes from its own plugin's textures/, by the key
// its item views carry, for the presentation adapter to decode
// (AppConfig.ItemTextures). Build refused an item whose texture is
// missing or broken, so every plugin kind has one.
func (c runContent) itemTextures() map[string][]byte { return itemTextures(c.found, c.catalog.Items) }

// itemTextures is runContent.itemTextures over the installed plugins
// found and the run's items (nil when it has no plugin kind).
func itemTextures(found []modding.Plugin, items item.Book) map[string][]byte {
	var out map[string][]byte
	for _, k := range items.Plugins() {
		key := items.Key(k)
		plugin, _, _ := strings.Cut(key, ".")
		path := items.Def(k).Texture
		for _, p := range found {
			if p.Manifest.ID != plugin {
				continue
			}
			for _, b := range p.Textures {
				if b.Path == path {
					if out == nil {
						out = map[string][]byte{}
					}
					out[items.Name(k)] = b.Data
				}
			}
		}
	}
	return out
}
