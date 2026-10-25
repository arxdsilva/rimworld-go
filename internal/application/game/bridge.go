package game

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// Bridges (MAPGEN-3 §3.2): a wooden foundation in its own layer over
// water, marsh or mud. The terrain underneath never changes; a finished
// bridge makes the cell dry, walkable footing (pathing, spawning, drops,
// stockpiles) that holds up light things (wooden walls, doors, fences,
// furniture) but not stone or steel walls, floors or carpets. The open
// sea is never bridged: a generated colony keeps an ocean bitset (saved);
// a legacy map has none, so every deep water there counts as river.

// bridgeState is the open-sea bitset (nil: no sea known).
type bridgeState struct {
	ocean []uint64
}

// cellWalkable is whether pawns can stand on (x, y): the terrain is
// walkable or a finished bridge covers it, no building blocks it and no
// filled grave mounds it. Every site writing the grid's walkable bit uses
// it.
func (g *Game) cellWalkable(x, y int) bool {
	if !g.terr.At(x, y).Walkable() && !g.bridged(x, y) {
		return false
	}
	if g.layerBlockerAt(x, y) != 0 {
		return false
	}
	return !g.filledGraveAt(x, y)
}

// filledGraveAt reports a filled grave covering (x, y): a mound that
// blocks movement (burial.go).
func (g *Game) filledGraveAt(x, y int) bool {
	if len(g.graves) == 0 {
		return false
	}
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	b, ok := g.bldgs.Get(bid)
	return ok && b.Kind == construction.Grave && g.graveFilled(bid)
}

// bridgeAt is the finished foundation (a bridge) covering (x, y), 0 for
// none. Only bridgeable ground can carry one, so dry land answers at once.
func (g *Game) bridgeAt(x, y int) entity.EntityID {
	if !g.grid.InBounds(x, y) || !terrain.Bridgeable(g.terr.At(x, y)) {
		return 0
	}
	bid := g.layerBldgAt(x, y, construction.LayerFoundation)
	if b, ok := g.bldgs.Get(bid); ok && construction.DefOf(b.Kind).Foundation {
		return bid
	}
	return 0
}

// bridged reports a finished bridge on (x, y).
func (g *Game) bridged(x, y int) bool { return g.bridgeAt(x, y) != 0 }

// supportAt is what (x, y) holds up: the higher of the ground's own
// support and its finished foundation's.
func (g *Game) supportAt(x, y int) terrain.Support {
	s := terrain.GroundSupport(g.terr.At(x, y))
	if bid := g.bridgeAt(x, y); bid != 0 {
		b, _ := g.bldgs.Get(bid)
		s = max(s, construction.Provides(b.Kind))
	}
	return s
}

// bridgePlacement is the one-cell rule for a foundation: bridgeable
// ground (water, marsh, mud) that is not the open sea.
func (g *Game) bridgePlacement(x, y int) (bool, string) {
	if !terrain.Bridgeable(g.terr.At(x, y)) {
		return false, ReasonNotBridgeable
	}
	if g.isOcean(x, y) {
		return false, ReasonOpenSea
	}
	return true, ""
}

// supportPlacement is the load rule for kind k built from stuff on
// (x, y): the cell must hold up what k needs. A dug piece (a grave) also
// needs the ground itself, never a deck. Without a material (noStuff) the
// kind's default material decides.
func (g *Game) supportPlacement(k construction.Kind, stuff item.Kind, x, y int) (bool, string) {
	if stuff == noStuff {
		stuff = construction.DefaultStuff(k)
	}
	s := g.supportAt(x, y)
	if construction.Requires(k, stuff) > s {
		return false, ReasonNoSupport
	}
	if construction.DefOf(k).Dug && terrain.GroundSupport(g.terr.At(x, y)) < terrain.SupportHeavy {
		return false, ReasonNoSupport
	}
	return true, ""
}

// deckOutOfReach reports a planned bridge hauler id can't get beside yet
// (further out over deep water than the finished span): its material
// waits until the cell before it is built, so a span grows from the
// shore and nobody carries wood to the water's edge and back.
func (g *Game) deckOutOfReach(id entity.EntityID, fr construction.Frame) bool {
	return construction.DefOf(fr.Kind).Foundation && !g.reachable(id, fr.X, fr.Y)
}

// --- the open sea -----------------------------------------------------------

// isOcean reports open sea on (x, y) (never bridged).
func (g *Game) isOcean(x, y int) bool {
	if g.br.ocean == nil || !g.grid.InBounds(x, y) {
		return false
	}
	i := uint(y*g.grid.Width() + x)
	return g.br.ocean[i>>6]&(1<<(i&63)) != 0
}

// setOcean keeps the generator's sea mask as a bitset; a mask of another
// size, or one with no sea, is no sea.
func (g *Game) setOcean(mask []bool) {
	g.br.ocean = nil
	n := g.grid.Width() * g.grid.Height()
	if len(mask) != n || !slices.Contains(mask, true) {
		return
	}
	bits := make([]uint64, (n+63)/64)
	for i, on := range mask {
		if on {
			bits[i>>6] |= 1 << (uint(i) & 63)
		}
	}
	g.br.ocean = bits
}

// exportOcean is the sea bitset for a save (nil: no sea).
func (g *Game) exportOcean() []uint64 { return slices.Clone(g.br.ocean) }

// importOcean restores a saved sea bitset; an absent (older save) or
// mis-sized one is no sea, so every deep water stays bridgeable.
func (g *Game) importOcean(bits []uint64) {
	g.br = bridgeState{}
	n := g.grid.Width() * g.grid.Height()
	if len(bits) == 0 || len(bits) != (n+63)/64 {
		return
	}
	g.br.ocean = slices.Clone(bits)
	if r := n & 63; r != 0 {
		g.br.ocean[len(bits)-1] &= 1<<uint(r) - 1
	}
}
