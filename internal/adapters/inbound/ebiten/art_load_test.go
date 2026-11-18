// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

func indexArtCatalog(root string, preferFiles bool) (*art.Catalog, error) {
	c, err := art.Index(root, preferFiles)
	if c != nil {
		c.ReadBack = readBack
	}
	return c, err
}
