package game

import (
	"math/bits"

	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// Fog of war in the read models and the orders (MAPGEN-2 §5, fog.go).

// fillFogView hands out the fog layer and leaves hidden frames,
// buildings, plants and animals out of the world view (items are skipped
// where they are counted, game.go). A no-op without fog.
func (g *Game) fillFogView(w *WorldView) {
	if g.fog.hidden == nil {
		return
	}
	w.Fog, w.FogVer = g.fogLayer(), g.fog.ver
	w.Frames = fogKeep(g, w.Frames, func(v *FrameView) (int, int) { return v.X, v.Y })
	w.Buildings = fogKeep(g, w.Buildings, func(v *BuildingView) (int, int) { return v.X, v.Y })
	w.Plants = fogKeep(g, w.Plants, func(v *PlantView) (int, int) { return v.X, v.Y })
	w.Animals = fogKeep(g, w.Animals, func(v *AnimalView) (int, int) { return v.X, v.Y })
}

// fogPawns leaves strangers (raiders, prisoners-to-be) standing in the
// dark out of the pawn snapshot; colonists always show.
func (g *Game) fogPawns(views []PawnView) []PawnView {
	if g.fog.hidden == nil {
		return views
	}
	return fogKeep(g, views, func(v *PawnView) (int, int) {
		if !g.isRaider(v.ID) {
			return -1, -1 // never fogged
		}
		return v.X, v.Y
	})
}

// fogKeep is in without the entries on hidden tiles. It returns in itself
// when nothing is hidden, else a fresh slice (in may be a shared cache).
func fogKeep[T any](g *Game, in []T, at func(*T) (int, int)) []T {
	for i := range in {
		if x, y := at(&in[i]); !g.fogged(x, y) {
			continue
		}
		out := make([]T, i, len(in)-1)
		copy(out, in[:i])
		for j := i + 1; j < len(in); j++ {
			if x, y := at(&in[j]); !g.fogged(x, y) {
				out = append(out, in[j])
			}
		}
		return out
	}
	return in
}

// fogLayer is WorldView.Fog: 1 per hidden tile, a shared read-only slice
// rebuilt into a fresh one whenever the fog changes.
func (g *Game) fogLayer() []uint8 {
	c, n := &g.fog.view, g.grid.Width()*g.grid.Height()
	if c.ok && c.ver == g.fog.ver && len(c.cells) == n {
		return c.cells
	}
	out := make([]uint8, n)
	for wi, word := range g.fog.hidden {
		for word != 0 {
			b := wi<<6 + bits.TrailingZeros64(word)
			if b < n {
				out[b] = 1
			}
			word &= word - 1
		}
	}
	c.cells, c.ver, c.ok = out, g.fog.ver, true
	return out
}

// fogRefuses reports a command aimed into the dark: a move onto a hidden
// tile. Building, orders and designations check their own tiles
// (placement.go, orders.go and the mark functions).
func (g *Game) fogRefuses(c command.Command) bool {
	return c.Kind == command.MovePawn && g.fogged(c.X, c.Y)
}

// fogOrderRefused is forcedJob's fog rule: nothing can be ordered on a
// hidden tile but mining hidden rock (so the player can dig in).
func (g *Game) fogOrderRefused(o command.Order, x, y int) bool {
	return g.fogged(x, y) && (o != command.OrderMine || !terrain.IsRockWall(g.terr.At(x, y)))
}

// fogOrders is OrdersAt on a hidden tile: nothing to do but learn why.
func (g *Game) fogOrders(pawn entity.EntityID, x, y int) []OrderOption {
	return []OrderOption{{
		Label: "Go here", Disabled: true, Reason: ReasonFogged,
		Cmd: command.Command{Kind: command.MovePawn, Pawn: uint64(pawn), X: x, Y: y},
	}}
}
