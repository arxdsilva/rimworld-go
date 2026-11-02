package construction

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
)

// plannedA3 is A3's kinds not live yet (floors; AX-0 contract): delete a
// case when its kind works end to end. This file is A3's, and so are any
// predicates it adds here for its own kinds. Every A3 floor is live (AX-3).
func plannedA3(Kind) bool { return false }

// FixedWork reports a floor whose construct work is the wiki's whatever it
// is made of (AX-3): every A3 floor. wiki: Floors (stone tile 1100 and
// flagstone 500 in any stone, concrete 100 and the tiles 800 in their
// metal). The wood floor and the rest scale with their material
// (StuffStats).
func FixedWork(k Kind) bool { return k >= StrawMatting && k <= CarpetFloor }

// FloorFor is the floor kind a floor of stuff is laid as (AX-3): stone
// blocks make a stone tile, anything else the wood floor. Map generation
// lays ruin and vault floors through it.
func FloorFor(stuff item.Kind) Kind {
	if item.IsStoneBlocks(stuff) || stuff == item.Stone {
		return StoneTile
	}
	return Floor
}

// MigrateStoneFloor is the save v11 step for one building or blueprint
// (AX-3): a wood floor built from stone blocks (or the legacy "stone")
// becomes a stone tile of those blocks (legacy stone is granite); any other
// piece is returned as it is.
func MigrateStoneFloor(k Kind, stuff item.Kind) (Kind, item.Kind) {
	if k != Floor || FloorFor(stuff) != StoneTile {
		return k, stuff
	}
	if stuff == item.Stone {
		stuff = item.GraniteBlocks
	}
	return StoneTile, stuff
}

// MigrateFloors is the save v11 step (AX-3): MigrateStoneFloor over every
// stored building and blueprint.
func MigrateFloors(bs *component.Store[Building], fs *component.Store[Frame]) {
	bs.AnyRef(func(_ entity.EntityID, b *Building) bool {
		b.Kind, b.Stuff = MigrateStoneFloor(b.Kind, b.Stuff)
		return false
	})
	fs.AnyRef(func(_ entity.EntityID, f *Frame) bool {
		f.Kind, f.Stuff = MigrateStoneFloor(f.Kind, f.Stuff)
		return false
	})
}

// Walk speed on floors (AX-3). wiki: Floors and Terrain: walk speed is
// 13/(13+path cost); soil, our plain ground, costs 2 (87 %), a built floor
// 0 (100 %) and straw matting 1 (93 %). A step's ticks on a floor are the
// plain-ground ticks times WalkPct/100: 13/15 on a floor, 14/15 on straw
// matting.
const (
	floorWalkPct = 87 // 13/15
	strawWalkPct = 93 // 14/15
)

// WalkPct is the step time on a floor of kind k in percent of plain
// ground (100 for anything but a floor).
func WalkPct(k Kind) int {
	switch {
	case k == StrawMatting:
		return strawWalkPct
	case DefOf(k).Layer == LayerFloor:
		return floorWalkPct
	}
	return 100
}

// FloorStepPct is WalkPct of floor building b when ok (the floor-layer
// lookup's results), 100 when there is none.
func FloorStepPct(b Building, ok bool) int {
	if !ok {
		return 100
	}
	return WalkPct(b.Kind)
}

// Replaces reports a new piece of kind want may be laid in place of a
// standing have by an in-place swap (PLAY-011): the same kind in another
// material, or any floor over another floor (AX-3; wiki: Floors, a floor
// laid over a floor replaces it once built).
func Replaces(have, want Kind) bool {
	return have == want || DefOf(have).Layer == LayerFloor && DefOf(want).Layer == LayerFloor
}

// SameFloor reports a swap of have in stuff by want in stuff would change
// nothing: the same kind in the same material, except a carpet floor,
// which is laid again to repaint it (its colour rides the blueprint).
func SameFloor(have Kind, haveStuff item.Kind, want Kind, stuff item.Kind) bool {
	return have == want && haveStuff == stuff && want != CarpetFloor
}

// SwapTo is what a swap turns a have into, for its label: the new
// material's word (stuffWord) for the same kind, else the new kind's name.
func SwapTo(have, want Kind, stuffWord string) string {
	if have == want && want != CarpetFloor {
		return stuffWord
	}
	return DefOf(want).Name
}

// SwapToName is SwapTo for a building view's kind name, with the new
// material's full name (the pane's "Changing to …").
func SwapToName(have string, want Kind, stuff item.Kind) string {
	h, _ := Parse(have)
	return SwapTo(h, want, stuff.String())
}

// OnlyStuff reports k is some kind's one material (Def.Only: hay for straw
// matting, silver and gold for their tiles, cloth for the carpet floor),
// so the interface may name it as a building material.
func OnlyStuff(k item.Kind) bool {
	for _, d := range defs {
		if d.Only != 0 && d.Only == k {
			return true
		}
	}
	return false
}
