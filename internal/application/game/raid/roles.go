package raid

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// DEF-4 roles (Roles): what drop pods, thieves, burners, siege mortars,
// rival factions and the combat log need from the game. Value-only, like
// Host's.

// Arrive lands hostile drop pods.
type Arrive interface {
	// PodSpot: a pod may come down on (x, y): on the map, dry ground, no
	// thick rock overhead (wiki: Drop pod) and room for its shell.
	PodSpot(x, y int) bool
	// Land brings pod p of group gi down: the roof it breaks falls on
	// whoever is beneath, its shell stays as debris and its raider steps
	// out beside it (0: no room to step out).
	Land(gi int, p Pod) entity.EntityID
	// Reach: (tx, ty) can be reached from (x, y) by breaking through walls
	// and doors.
	Reach(x, y, tx, ty int) bool
}

// Mischief is what thieves, burners and rival raiders reach for.
type Mischief interface {
	// Loot is the nearest stack worth stealing within r tiles of (x, y)
	// and its tile (ok false: none).
	Loot(x, y, r int) (stack entity.EntityID, sx, sy int, ok bool)
	// Steal takes stack off the map into thief id's hands.
	Steal(id, stack entity.EntityID) (k item.Kind, n int, ok bool)
	// Flammable is the nearest unburning thing worth setting alight (a
	// wooden building or a crop) within r tiles of (x, y).
	Flammable(x, y, r int) (tx, ty int, ok bool)
	// Ignite sets (x, y) alight (false: nothing there burns).
	Ignite(x, y int) bool
	// Rival: the raid groups of factions fa and fb (1 + faction index)
	// fight each other.
	Rival(fa, fb int) bool
}

// SiegeWorks are a siege's mortars.
type SiegeWorks interface {
	// BuildMortar raises a raiders' mortar near (x, y), loaded with
	// shells (0: no room).
	BuildMortar(x, y int) entity.EntityID
	// Crew is who mans mortar m (0: nobody); ok false when it is gone.
	Crew(m entity.EntityID) (crew entity.EntityID, ok bool)
	// Man sends raider id to crew mortar m.
	Man(id, m entity.EntityID) bool
}

// Chronicle is the combat log (fights and threats, for the UI).
type Chronicle interface {
	Log(text string)
}
