package maps

import (
	"rimworld-go/internal/domain/sim/entity"
)

// A mineral site may hold a threat when the caravan comes (QST-047, wiki:
// Long-range mineral scanner: unguarded, manhunters or an ambush). The
// chances are not published: a third each (ours), drawn from the map's own
// seed so no simulation stream moves. With no hostile faction the ambush is
// no threat, and a biome with no manhunting species sends no pack; the
// letter goes only when the threat stands there. The wiki's "unknown threat may approach" is the one group
// raidDays after the site is clear (hourly, QST-048). A quest site whose
// kind approaches (the stash, the refugee: quest.Def.Approach, QST-051,
// 056) rolls the same threat when it opens (questsite.go).

// The threats a mineral site may hold (OreThreat).
const (
	OreNone = iota
	OreAnimals
	OreAmbush
)

// OreThreat is the threat mineral map id on tile opens with, for the world
// seed (OreNone, OreAnimals or OreAmbush).
func OreThreat(seed uint64, tile int, id uint32) int {
	return int((SeedFor(seed, tile, id) ^ oreSalt) % 3)
}

// oreSalt keeps the site's threat roll apart from the map's other draws.
const oreSalt uint64 = 0x0e5172e

// oreThreat rolls and sends site map id's threat at caravan c, who is
// about to walk in. A quest site's guards already stand there, so the
// letter wants more hostiles than before the roll.
func (l Lifecycle) oreThreat(c *Caravan, id uint32) {
	f := l.f
	m := f.a.Meta(id)
	if m == nil || !f.h.Switch(id) {
		return
	}
	before := f.h.Census(m.Cells).Hostiles
	seed := SeedFor(f.h.Seed(), m.Tile, id) ^ oreSalt
	title, what := "", ""
	switch OreThreat(f.h.Seed(), m.Tile, id) {
	case OreAnimals:
		f.h.Hunt(seed, 2+len(c.C.Members))
		title, what = "Animals at the site", "enraged animals roam the ground"
	case OreAmbush:
		if fac := f.hostile(); fac > 0 {
			f.h.Ambush(seed, fac)
			title, what = "Ambush at the site", "raiders lie in wait"
		}
	}
	if title != "" && len(c.C.Members) > 0 && f.h.Census(m.Cells).Hostiles > before { // a barren tile breeds no pack
		f.h.Letter(true, title, entity.EntityID(c.C.Members[0]), "At %s %s.", f.h.TileName(m.Tile), what)
	}
}
