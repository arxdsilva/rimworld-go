package game

// quest_reinforce_test.go: a quest site's reinforcements (QST-017). Once a
// camp is cleared and its map quiet, the camp's faction sends
// reinforcements 60 hours later: they come onto the site map if anyone of
// ours is still there, and never when the caravan left first. The pack-up
// waits for them.

import (
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/cadence"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/application/game/raid"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/biome"
)

// clearedCamp is a camp quest accepted, its site opened by a visiting
// caravan and every defender fallen, stepped to the hour the site's
// reinforcement clock starts: the game, the site map's row and the camp's
// faction ref.
func clearedCamp(t *testing.T) (*Game, *maps.Meta, int) {
	t.Helper()
	g, _ := campScene(t)
	id, ok := questOf(g).Offer(qd.Camp)
	if !ok {
		t.Fatal("the camp was not offered")
	}
	v := questView(t, g, id)
	choice := slices.IndexFunc(v.Rewards, func(r quest.RewardView) bool { return len(r.Lines) > 0 })
	g.Enqueue(command.Command{Kind: command.AcceptQuest, Index: int(id), Count: max(choice, 0)})
	g.Step(1)
	c := visitingCaravan(t, g, v.Tile)
	fleetOf(g).Arrived(c, uint64(g.clock.Now()))
	g.Step(1)
	if len(g.maps) != 2 {
		t.Fatalf("no site map opened: %d maps", len(g.maps))
	}
	m := g.maps[1]
	m.climate = biome.DefOf(biome.TemperateForest).Climate // the test planet is all sea ice
	meta := g.worldS().atlas.Meta(m.id)
	if meta.Foe != v.Target || v.Target <= 0 {
		t.Fatalf("the site's foe is %d, the camp's faction ref %d", meta.Foe, v.Target)
	}
	fell(g, defenders(m)...)
	for i := 0; i < 8 && meta.Reinforce == 0; i++ {
		nextHours(g, 1)
	}
	now := uint64(g.clock.Now())
	start := cadence.Next(now-clock.TicksPerHour, clock.TicksPerHour, cadence.OffCensus, 0) // the census slot nextHours stepped over
	if questView(t, g, id).State != qd.Succeeded || meta.Reinforce != start+60*clock.TicksPerHour || meta.PackBy != 0 {
		t.Fatalf("cleared at %d: quest %v, reinforce at %d, pack-up by %d", start, questView(t, g, id).State, meta.Reinforce, meta.PackBy)
	}
	if !slices.ContainsFunc(g.letters.List(), func(l letters.Letter) bool { return l.Title == "Reinforcements on the way" }) {
		t.Fatal("no letter says the reinforcements are coming")
	}
	return g, meta, v.Target
}

// reinforcements reports an assault group of faction ref fac on map m.
func reinforcements(m *mapState, fac int) bool {
	return slices.ContainsFunc(m.raidGroups, func(gr raid.Group) bool { return gr.Plan != raid.Defend && gr.Faction == fac })
}

// soon brings site id's reinforcements to the hourly census slot after
// next (cadence.OffCensus): the 60-hour wait itself is checked by
// clearedCamp and the maps unit tests; stepping it here would cost a
// minute under -race.
func soon(g *Game, id uint32) {
	now := uint64(g.clock.Now())
	g.worldS().atlas.Meta(id).Reinforce = cadence.Next(now+1, clock.TicksPerHour, cadence.OffCensus, 0) + clock.TicksPerHour
}

// TestCampReinforcementsComeWhileWeStay: when the clock runs out the camp's
// faction raids the site map, once, and the pack-up only follows; the clock
// and the reinforcements survive save and load. (The defenders, marked
// dead by fell, keep this scene's whole-state hash from matching a loaded
// copy, so the check is on the site's row and its raid groups.)
func TestCampReinforcementsComeWhileWeStay(t *testing.T) {
	t.Parallel()
	g, m0, fac := clearedCamp(t)
	site, id := g.maps[1], m0.ID
	meta := func() *maps.Meta { return g.worldS().atlas.Meta(id) }
	if l := saveAndLoad(t, g).worldS().atlas.Meta(id); l == nil || !reflect.DeepEqual(*l, *meta()) {
		t.Fatalf("the site's row did not survive save and load: %+v, was %+v", l, meta())
	}
	soon(g, id)
	nextHours(g, 1)
	if m := meta(); reinforcements(site, fac) || m == nil || m.Reinforced || m.PackBy != 0 {
		t.Fatalf("an hour early: reinforcements %v, meta %+v", reinforcements(site, fac), m)
	}
	nextHours(g, 1)
	if m := meta(); len(g.maps) != 2 || !reinforcements(site, fac) || m == nil || !m.Reinforced || m.PackBy != 0 {
		t.Fatalf("on time: %d maps, reinforcements %v, meta %+v (groups %+v)", len(g.maps), reinforcements(site, fac), m, site.raidGroups)
	}
	if !slices.ContainsFunc(g.letters.List(), func(l letters.Letter) bool { return l.Title == "Reinforcements" }) {
		t.Fatal("no letter says the reinforcements came")
	}
	groups := len(site.raidGroups)
	nextHours(g, 2)
	if len(site.raidGroups) != groups {
		t.Fatalf("more reinforcements came: %+v", site.raidGroups)
	}
	h := saveAndLoad(t, g)
	if l := h.worldS().atlas.Meta(id); l == nil || !l.Reinforced || !reinforcements(h.maps[1], fac) {
		t.Fatal("the reinforcements' state did not survive save and load")
	}
}

// TestCampReinforcementsNeverComeAfterWeLeft: our colonist walks off the
// cleared site into a caravan (placed directly, as visitingCaravan does:
// the test world has no road from this tile), the empty map closes, and
// the reinforcements due meanwhile never come.
func TestCampReinforcementsNeverComeAfterWeLeft(t *testing.T) {
	t.Parallel()
	g, meta, _ := clearedCamp(t)
	id, tile := meta.ID, meta.Tile
	soon(g, id)
	cur := g.mapState
	g.mapState = g.maps[1]
	visitingCaravan(t, g, tile)
	g.mapState = cur
	nextHours(g, 6)
	if len(g.maps) != 1 || g.worldS().atlas.Meta(id) != nil {
		t.Fatalf("the empty site did not close: %d maps", len(g.maps))
	}
	if slices.ContainsFunc(g.letters.List(), func(l letters.Letter) bool { return l.Title == "Reinforcements" }) {
		t.Fatal("reinforcements came for a site nobody was on")
	}
}
