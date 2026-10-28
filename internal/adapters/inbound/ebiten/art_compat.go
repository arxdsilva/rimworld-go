package ebiten

import (
	"image"
	"image/color"
	"sync"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

// The sprite catalog lives in art (ARCH-003). These aliases and one-line
// wrappers keep the root's call sites unchanged while its features move out
// one package at a time (SOLID_GO.md §11: no forwarder outlives the commit
// series that repoints its callers).

type (
	deflater    = art.Deflater
	artCatalog  = art.Catalog
	artSprite   = art.Sprite
	artMeta     = art.Meta
	spriteAtlas = art.Atlas
	facing      = art.Facing
	animalExt   = art.AnimalExt
	artMemStats = art.MemSnapshot
	tintKey     = art.TintKey
)

const (
	artTilePx          = art.TilePx
	artMetadataRel     = art.MetadataRel
	faceSouth          = art.FaceSouth
	faceEast           = art.FaceEast
	faceNorth          = art.FaceNorth
	faceWest           = art.FaceWest
	wallN              = art.WallN
	wallE              = art.WallE
	wallS              = art.WallS
	wallW              = art.WallW
	atlasMetaDir       = art.AtlasMetaDir
	atlasLayers        = art.AtlasLayers
	atlasPageCache     = art.AtlasPageCache
	animalSpanFallback = art.AnimalSpanFallback
)

func facingFor(dx, dy int, prev facing) facing { return art.FacingFor(dx, dy, prev) }
func mix64(x uint64) uint64                    { return art.Mix64(x) }

func loadArtCatalog(root string) (*artCatalog, error) {
	c, err := art.Load(root)
	if c != nil {
		c.ReadBack = readBack
	}
	return c, err
}

func indexArtCatalog(root string, preferFiles bool) (*artCatalog, error) {
	c, err := art.Index(root, preferFiles)
	if c != nil {
		c.ReadBack = readBack
	}
	return c, err
}

func artOrigin(m artMeta, sx, sy, tile float64) (x, y, scale float64) {
	return art.Origin(m, sx, sy, tile)
}

// LocateArt finds the assets root (assets/metadata above the working
// directory or the executable).
func LocateArt() (string, error) { return art.Locate() }

// cpuReadBack names the sprites the renderer's bakes read back on the CPU
// (crop units and fallbacks, grass tufts); the catalog keeps their pixels.
var cpuReadBack = sync.OnceValue(func() map[string]bool {
	m := map[string]bool{}
	for _, ks := range plantKeys.Crops {
		for st := range ks {
			for _, k := range ks[st] {
				m[k] = true
				for _, fb := range plantKeys.Fallback[k] {
					m[fb] = true
				}
			}
		}
	}
	for _, set := range [][]string{grassShortKeys, grassTallKeys, grassDormantKeys} {
		for _, k := range set {
			m[k] = true
		}
	}
	return m
})

func readBack(key string) bool { return cpuReadBack()[key] }

func parseArtMeta(src string) (artMeta, error) { return art.ParseMeta(src) }

func parseTints(src string) (map[string]map[string]color.NRGBA, error) {
	return art.ParseTints(src)
}

func decodeNRGBA(path string) (*image.NRGBA, error) { return art.DecodeNRGBA(path) }
