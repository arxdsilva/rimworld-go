package game

import (
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/terrain"
)

// Water and wet ground in play (NG-1 §5.6): nothing is built, dropped or
// spawned on water, wading rinses boots, mud and wet ground soil them.

// ReasonWater is the placement refusal for any water cell in a footprint.
const ReasonWater = "Can't build on water"

// isWaterAt reports shallow or deep water on tile (x, y).
func (g *Game) isWaterAt(x, y int) bool {
	return g.grid.InBounds(x, y) && terrain.IsWater(g.terr.At(x, y))
}

// dryWalkable is a walkable tile that is not water: where things are
// spawned, dropped and entered from (animals and raiders cross shallow
// water on their way but never appear in it).
func (g *Game) dryWalkable(x, y int) bool {
	return g.grid.Walkable(x, y) && !terrain.IsWater(g.terr.At(x, y))
}

// wadeWash is the dirt left on boots after a step through shallow water:
// half of it washes off.
func wadeWash(c int) int { return c - c/2 }

// stepGain is the dirt picked up by a step (or a work beat) base on tile
// (x, y): the wetter of the weather and the ground, doubled on mud.
func (g *Game) stepGain(x, y, base int, grass bool) int {
	t := g.terr.At(x, y)
	wet := max(g.wetPm(), t.Wetness())
	gain := filth.Gain(base, wet, grass)
	if t == terrain.Mud {
		gain *= 2
	}
	return gain
}

// waterPlacement is the water rule of canPlace: ok is false, with
// ReasonWater, for a water cell (shallow or deep).
func (g *Game) waterPlacement(x, y int) (bool, string) {
	if g.isWaterAt(x, y) {
		return false, ReasonWater
	}
	return true, ""
}

// groundExtra is the path cost added for entering (x, y) on slow ground
// (marsh, mud, sand, ice, shallow water): (WalkCost-100)/10, none on
// floored tiles or ground at least as fast as plain.
func (g *Game) groundExtra(x, y int) int {
	c := g.terr.At(x, y).WalkCost()
	if c <= 100 || !g.grid.InBounds(x, y) || g.treadAt(y*g.grid.Width()+x)&treadFloor != 0 {
		return 0
	}
	return (c - 100) / 10
}
