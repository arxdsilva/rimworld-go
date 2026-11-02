package orders

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/plant"
)

// Claim (AX-1, wiki: Orders): the Claim order makes buildings that are
// not the colony's its own at once, with no work (wiki): map-generation
// ruins (Building.Ruin) and ancient or raider-built pieces
// (Building.Ancient). Claimed, they count as colony wealth, join the Home
// area, are repaired and rebuilt, and give their full refund. A natural
// feature (a steam vent) is never anyone's.
type Claim struct{ h Host }

// Claimable reports a building the Claim order takes.
func Claimable(b construction.Building) bool {
	return (b.Ruin || b.Ancient) && !construction.DefOf(b.Kind).Natural
}

// Mark claims every claimable building whose origin lies in the
// rectangle and reports how many it claimed.
func (c *Claim) Mark(minX, minY, maxX, maxY int) int {
	h := c.h
	n := 0
	for _, id := range h.BuildingIDs() {
		b, ok := h.Building(id)
		if !ok || !in(b.X, b.Y, minX, minY, maxX, maxY) || !h.Seen(b.X, b.Y) || !Claimable(b) {
			continue
		}
		b.Ruin, b.Ancient = false, false
		h.SetBuilding(id, b)
		h.Claimed(id)
		n++
	}
	return n
}

// ChopWoodPct is the least growth (percent) a tree must have for Chop
// wood (wiki: Orders, 40 %); a younger tree only falls to Cut plants.
const ChopWoodPct = 40

// Choppable reports a plant the Chop wood order marks: a tree (a plant
// with a trunk, the saguaro included) grown at least ChopWoodPct.
func Choppable(p plant.Wild) bool { return Tree(p) && p.Percent() >= ChopWoodPct }

// Tree reports a plant with a trunk (Chop wood's unmark takes any tree).
func Tree(p plant.Wild) bool { return plant.DefOf(p.Species).Trunk }

// Slaughterable reports an animal the Slaughter order marks: a living
// tame animal (its mark is the hunt mark, herd/slaughter.go).
func Slaughterable(a animal.State) bool { return a.Tame && !a.Dead() }

// FloorRefund is what taking floor kind up gives back (wiki: Floors,
// agreed with A3 through the lead): half of its material, rounded down,
// and nothing for flagstone. Smoothed stone is ground, not a floor, so the
// order never reaches it; this game has no burned floors.
func FloorRefund(k construction.Kind) int {
	if k == construction.Flagstone {
		return 0
	}
	return construction.DefOf(k).Wood / 2
}

// FloorExtraRefund is the secondary material (construction.Def.Extra)
// slot i of floor kind k gives back: half, rounded down; none for
// flagstone or for anything but a floor.
func FloorExtraRefund(k construction.Kind, i int) int {
	d := construction.DefOf(k)
	if d.Layer != construction.LayerFloor || k == construction.Flagstone || i < 0 || i >= len(d.Extra) {
		return 0
	}
	return construction.DefOf(k).Extra[i].N / 2
}
