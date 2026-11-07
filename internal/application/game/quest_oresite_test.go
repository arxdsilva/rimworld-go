package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
)

// TestOreThreatRollsAThirdEach: over many sites the three threats each
// come about a third of the time (QST-047, ours), from the seed alone.
func TestOreThreatRollsAThirdEach(t *testing.T) {
	t.Parallel()
	var n [3]int
	for tile := range 3000 {
		k := maps.OreThreat(0x5eed, tile, uint32(1+tile%7))
		if k != maps.OreThreat(0x5eed, tile, uint32(1+tile%7)) {
			t.Fatal("the roll is not a function of its inputs")
		}
		n[k]++
	}
	for k, c := range n {
		if c < 850 || c > 1150 {
			t.Fatalf("threat %d came %d times in 3000 (counts %v)", k, c, n)
		}
	}
}

// TestOreSiteOpensWithItsThreat: the mineral map holds the threat its roll
// names when the caravan walks in, with a letter for any threat; the raid
// clock waits until no hostile stands. Each threat runs once, on a world
// seed picked so the first map rolls it.
func TestOreSiteOpensWithItsThreat(t *testing.T) {
	t.Parallel()
	for _, want := range []int{maps.OreNone, maps.OreAnimals, maps.OreAmbush} {
		g := worldGame(t)
		tile := plainNear(t, g)
		for maps.OreThreat(g.streams.seed, tile, 1) != want {
			g.streams.seed++
		}
		w := g.worldS()
		w.ax9 = travel.Import(&travel.Save{Sites: []travel.Site{{Tile: tile, Mineral: item.Steel, Qty: 200, Expires: 1 << 40}}})
		arriveAt(t, g, tile, g.pawnIDs[0], g.pawnIDs[1])
		g.Step(1)
		if len(g.maps) != 2 || w.atlas.Maps[0].ID != 1 {
			t.Fatalf("threat %d: no first site map (%d maps)", want, len(g.maps))
		}
		m, meta := g.maps[1], &w.atlas.Maps[0]
		manhunters := 0
		for _, id := range m.animalIDs {
			if a := g.animals.Ref(id); a != nil && a.Manhunter > uint64(g.clock.Now()) {
				manhunters++
			}
		}
		hostile := slices.ContainsFunc(g.ng.factions, func(f faction.Faction) bool {
			return f.Type.Human() && f.Relation == faction.Hostile && !f.Defeated
		})
		threat := manhunters > 0 || len(m.raidGroups) > 0
		switch {
		case want == maps.OreNone && threat:
			t.Fatalf("an unguarded site holds %d manhunters, %d raid groups", manhunters, len(m.raidGroups))
		case want == maps.OreAnimals && ((manhunters > 0) != huntable(g, m) || len(m.raidGroups) != 0):
			t.Fatalf("an animal site holds %d manhunters (its biome has hunters: %v), %d raid groups", manhunters, huntable(g, m), len(m.raidGroups))
		case want == maps.OreAmbush && hostile && len(m.raidGroups) == 0:
			t.Fatal("an ambush site holds no raiders")
		}
		warned := slices.ContainsFunc(g.letters.List(), func(l letters.Letter) bool {
			return l.Title == "Animals at the site" || l.Title == "Ambush at the site"
		})
		if warned != threat {
			t.Fatalf("threat %d: warned %v with a threat %v", want, warned, threat)
		}
		g.Step(clock.TicksPerHour)
		if threat == (meta.Clear != 0) {
			t.Fatalf("threat %d: clear at %d while hostiles stand %v", want, meta.Clear, threat)
		}
	}
}

// huntable: site map m's biome has a species that can turn manhunter.
func huntable(g *Game, m *mapState) bool {
	prev, rng := g.mapState, g.rng
	g.mapState = m
	_, ok := g.pickSpecies(manhunterSpecies)
	g.mapState, g.rng = prev, rng
	return ok
}
