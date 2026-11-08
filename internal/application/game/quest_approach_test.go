package game

// quest_approach_test.go: an item stash's site may draw an unknown threat
// when its map opens (QST-051; the refugee's is the same roll, QST-056):
// the mineral site's roll, from the map's own seed, on top of the guards.

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
)

// TestStashSiteOpensWithItsApproachRoll: each threat once, on a world seed
// picked so the stash's site map rolls it: the guards stand, and beyond
// them the roll's manhunters or ambush, announced by letter only when they
// came. The state hash holds across save and load, and a loaded game goes
// on as the saved one does.
func TestStashSiteOpensWithItsApproachRoll(t *testing.T) {
	t.Parallel()
	for _, want := range []int{maps.OreNone, maps.OreAnimals, maps.OreAmbush} {
		g := worldGame(t)
		for i := range g.ng.factions {
			if f := &g.ng.factions[i]; f.Type.Human() && len(f.Settlements) > 0 {
				f.Relation, f.Defeated = faction.Hostile, false
				break
			}
		}
		id, ok := questOf(g).Offer(qd.Stash)
		if !ok {
			t.Fatal("the stash was not offered")
		}
		v := questView(t, g, id)
		for maps.OreThreat(g.streams.seed, v.Tile, 1) != want {
			g.streams.seed++
		}
		c := visitingCaravan(t, g, v.Tile)
		fleetOf(g).Arrived(c, uint64(g.clock.Now()))
		g.Step(1)
		if len(g.maps) != 2 || g.worldS().atlas.Maps[0].ID != 1 {
			t.Fatalf("threat %d: no first site map (%d maps)", want, len(g.maps))
		}
		m := g.maps[1]
		if meta := g.worldS().atlas.Meta(m.id); meta.Foe != v.Target {
			t.Fatalf("threat %d: the site's foe is %d, want the guards' %d", want, meta.Foe, v.Target)
		}
		guards := slices.ContainsFunc(m.raidGroups, func(gr raid.Group) bool { return gr.Plan == raid.Defend && gr.Faction == v.Target })
		ambush := slices.ContainsFunc(m.raidGroups, func(gr raid.Group) bool { return gr.Plan != raid.Defend })
		manhunters := 0
		for _, aid := range m.animalIDs {
			if a := m.animals.Ref(aid); a != nil && a.Manhunter > uint64(g.clock.Now()) {
				manhunters++
			}
		}
		switch {
		case !guards:
			t.Fatalf("threat %d: no guards of faction ref %d: %+v", want, v.Target, m.raidGroups)
		case want == maps.OreNone && (ambush || manhunters > 0):
			t.Fatalf("an unthreatened stash holds %d manhunters, groups %+v", manhunters, m.raidGroups)
		case want == maps.OreAnimals && ((manhunters > 0) != huntable(g, m) || ambush):
			t.Fatalf("an animal stash holds %d manhunters (hunters here: %v), groups %+v", manhunters, huntable(g, m), m.raidGroups)
		case want == maps.OreAmbush && !ambush:
			t.Fatalf("an ambush stash holds no raiders beyond its guards: %+v", m.raidGroups)
		}
		warned := slices.ContainsFunc(g.letters.List(), func(l letters.Letter) bool {
			return l.Title == "Animals at the site" || l.Title == "Ambush at the site"
		})
		if warned != (ambush || manhunters > 0) {
			t.Fatalf("threat %d: warned %v, ambush %v, %d manhunters", want, warned, ambush, manhunters)
		}
		g.Step(1) // the site's rooms and roofs settle
		h := saveAndLoad(t, g)
		if h.StateHash() != g.StateHash() {
			t.Fatalf("threat %d: state hash differs after save and load", want)
		}
		nextHours(g, 2)
		nextHours(h, 2)
		if h.StateHash() != g.StateHash() {
			t.Fatalf("threat %d: a loaded game went another way", want)
		}
	}
}
