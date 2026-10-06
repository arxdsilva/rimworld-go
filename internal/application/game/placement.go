package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Placement reasons are player-facing strings shown by the build preview.
const (
	ReasonOutside  = "outside the map"
	ReasonBlocked  = "blocked by a wall"
	ReasonOccupied = "already built here"
	ReasonPlanned  = "already planned here"
	ReasonItem     = "items in the way"
	ReasonUnknown  = "unknown building"
)

// BuildableView is the read model of one buildable definition.
type BuildableView struct {
	ID        uint8 // value for command.Command.Build
	Kind      string
	Wood      int
	Work      int
	Rotatable bool
	Floor     bool // floor layer: can sit under furniture
	Blocks    bool // completed building blocks movement
}

// Buildables lists every buildable in catalog order.
func Buildables() []BuildableView {
	defs := construction.All()
	out := make([]BuildableView, 0, len(defs))
	for _, d := range defs {
		out = append(out, BuildableView{
			ID: uint8(d.Kind), Kind: d.Name, Wood: d.Wood, Work: d.Work, Rotatable: d.Rotatable,
			Floor: d.Layer == construction.LayerFloor, Blocks: d.BlocksMove,
		})
	}
	return out
}

// CanPlace reports whether a blueprint of kind could be placed at (x, y)
// now, and why not. The Architect preview and DesignateBuild share it.
func (g *Game) CanPlace(kind string, x, y int) (bool, string) {
	k, ok := construction.Parse(kind)
	if !ok {
		return false, ReasonUnknown
	}
	return g.canPlace(k, x, y)
}

func (g *Game) canPlace(k construction.Kind, x, y int) (bool, string) {
	if !g.grid.InBounds(x, y) {
		return false, ReasonOutside
	}
	layer := construction.DefOf(k).Layer
	if g.layerBldgAt(x, y, layer) != 0 {
		return false, ReasonOccupied
	}
	if g.layerFrameAt(x, y, layer) != 0 {
		return false, ReasonPlanned
	}
	if !g.grid.Walkable(x, y) {
		return false, ReasonBlocked
	}
	if construction.DefOf(k).BlocksMove && g.itemAt(x, y) != 0 {
		return false, ReasonItem
	}
	return true, ""
}

func (g *Game) placeFrame(kind construction.Kind, x, y int) {
	g.placeFrameFacing(kind, construction.South, x, y)
}

func (g *Game) placeFrameFacing(kind construction.Kind, f construction.Facing, x, y int) {
	if !kind.Valid() {
		return
	}
	if ok, _ := g.canPlace(kind, x, y); !ok {
		return
	}
	if !construction.DefOf(kind).Rotatable {
		f = construction.South
	}
	id := g.entities.Next()
	g.frameIDs = append(g.frameIDs, id)
	g.frames.Set(id, construction.Frame{
		Kind:     kind,
		Facing:   f,
		X:        x,
		Y:        y,
		Need:     construction.WoodCost(kind),
		WorkLeft: construction.WorkTicks(kind),
	})
}

func (g *Game) layerFrameAt(x, y int, layer construction.Layer) entity.EntityID {
	for _, id := range g.frameIDs {
		fr, ok := g.frames.Get(id)
		if ok && fr.X == x && fr.Y == y && construction.DefOf(fr.Kind).Layer == layer {
			return id
		}
	}
	return 0
}

func (g *Game) layerBldgAt(x, y int, layer construction.Layer) entity.EntityID {
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if ok && b.X == x && b.Y == y && construction.DefOf(b.Kind).Layer == layer {
			return id
		}
	}
	return 0
}

// ejectPawns moves anyone standing on a tile that just became blocked to
// the first walkable neighbour (fixed order, so deterministic).
func (g *Game) ejectPawns(x, y int) {
	for _, id := range g.pawnIDs {
		pos, _ := g.positions.Get(id)
		if pos.X != x || pos.Y != y {
			continue
		}
		for _, d := range [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
			nx, ny := x+d[0], y+d[1]
			if g.grid.Walkable(nx, ny) {
				g.positions.Set(id, Position{X: nx, Y: ny})
				break
			}
		}
	}
}

func (g *Game) markDeconstruct(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := normalizeRect(x1, y1, x2, y2)
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok || b.X < minX || b.X > maxX || b.Y < minY || b.Y > maxY {
			continue
		}
		if b.Deconstruct == on {
			continue
		}
		b.Deconstruct = on
		if on {
			b.WorkLeft = construction.DeconstructTicks(b.Kind)
		}
		g.bldgs.Set(id, b)
	}
}

func normalizeRect(x1, y1, x2, y2 int) (int, int, int, int) {
	if x2 < x1 {
		x1, x2 = x2, x1
	}
	if y2 < y1 {
		y1, y2 = y2, y1
	}
	return x1, y1, x2, y2
}

// removeBuilding deletes a completed building, refunds wood and frees
// its tile and bed claim.
func (g *Game) removeBuilding(id entity.EntityID) {
	b, ok := g.bldgs.Get(id)
	if !ok {
		return
	}
	g.bldgs.Delete(id)
	kept := g.bldgIDs[:0]
	for _, bid := range g.bldgIDs {
		if bid != id {
			kept = append(kept, bid)
		}
	}
	g.bldgIDs = kept
	if construction.DefOf(b.Kind).BlocksMove {
		g.grid.SetWalkable(b.X, b.Y, true)
	}
	if b.Claimant != 0 {
		delete(g.sleptAt, entity.EntityID(b.Claimant))
	}
	if r := construction.Refund(b.Kind); r > 0 {
		if sid, err := g.SpawnStack(item.Wood, r, b.X, b.Y); err == nil {
			if st, ok := g.stacks.Get(sid); ok {
				g.mergeOrSet(sid, st)
			}
		}
	}
}
