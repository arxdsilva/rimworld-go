package ebiten

import (
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

func indexArtCatalog(root string, preferFiles bool) (*artpkg.Catalog, error) {
	c, err := art.Index(root, preferFiles)
	if c != nil {
		c.ReadBack = readBack
	}
	return c, err
}
