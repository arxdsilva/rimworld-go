package ebiten

import (
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/combat/cover"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// Natural cover and its direction in the cover overlay (CBT-5 UI): trees,
// bushes, cactus, rock and stone chunks join the buildings, by the cover
// domain's values; with a hostile under the cursor the tiles around the
// selected colonist show what each gives against a shot from that hostile
// (cover.AngleFactor and RangeFactor, the sim's own).

// grownPct is the growth (percent) from which a plant gives cover (the
// sim's combatField.Plant: at least half way).
const grownPct = 50

// naturalCoverCells appends the natural cover near (x, y) from the world
// view: rock tiles, grown plants and loose stone chunks.
func naturalCoverCells(out []overlay.CoverCell, w *game.WorldView, x, y int) []overlay.CoverCell {
	rock := cover.Permille(cover.Rock) / 10
	if w.TerrainW > 0 {
		h := len(w.Terrain) / w.TerrainW
		for ty := max(0, y-coverRadius); ty <= min(h-1, y+coverRadius); ty++ {
			for tx := max(0, x-coverRadius); tx <= min(w.TerrainW-1, x+coverRadius); tx++ {
				if terrain.IsRockWall(terrain.Type(w.Terrain[ty*w.TerrainW+tx])) {
					out = append(out, overlay.CoverCell{X: tx, Y: ty, Value: rock})
				}
			}
		}
	}
	for i := range w.Plants {
		p := &w.Plants[i]
		if p.Growth < grownPct || outsideCover(p.X, p.Y, x, y) {
			continue
		}
		if sp, ok := plant.Parse(p.Species); ok {
			if v := cover.Permille(cover.OfPlant(sp, true)) / 10; v > 0 {
				out = append(out, overlay.CoverCell{X: p.X, Y: p.Y, Value: v})
			}
		}
	}
	chunk := cover.Permille(cover.Chunk) / 10
	for i := range w.Items {
		it := &w.Items[i]
		if it.Carried || outsideCover(it.X, it.Y, x, y) || !strings.HasSuffix(it.Kind, " chunk") {
			continue
		}
		out = append(out, overlay.CoverCell{X: it.X, Y: it.Y, Value: chunk})
	}
	return out
}

// outsideCover is true when (px, py) lies outside the coverRadius square around
// (x, y) (the cells to skip).
func outsideCover(px, py, x, y int) bool {
	return px < x-coverRadius || px > x+coverRadius || py < y-coverRadius || py > y+coverRadius
}

// aimCover rescales the cells touching the target (tx, ty) to what they
// give against a shot from the shooter (sx, sy): the angle and point-blank
// factors of the cover domain. Cells elsewhere keep their plain value.
func aimCover(cells []overlay.CoverCell, sx, sy, tx, ty int) {
	for i := range cells {
		c := &cells[i]
		if c.X < tx-1 || c.X > tx+1 || c.Y < ty-1 || c.Y > ty+1 || c.X == tx && c.Y == ty {
			continue
		}
		f := cover.AngleFactor(sx, sy, tx, ty, c.X, c.Y) * cover.RangeFactor(sx, sy, c.X, c.Y) / 1000
		c.Value = c.Value * f / 1000
	}
}
