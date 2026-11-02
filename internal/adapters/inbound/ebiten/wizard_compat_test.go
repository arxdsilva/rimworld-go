package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/options"

	"rimworld-go/internal/adapters/inbound/ebiten/worldinfo"
	"rimworld-go/internal/domain/world/planet"
)

// buildTileInfo is the landing panel's info for tile i of p with a side x
// side map on a computer with total bytes of memory (0 unknown).
func buildTileInfo(p *planet.Planet, i, side int, total uint64) worldinfo.TileInfo {
	return worldinfo.BuildTileInfo(p, i, options.MapSizeNote(side, total))
}
