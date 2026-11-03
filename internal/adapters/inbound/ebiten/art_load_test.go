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
