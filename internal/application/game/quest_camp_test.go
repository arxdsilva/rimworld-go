package game

// quest_camp_test.go: the hostile camp through the game (QST-060..064):
// offered by a friendly faction against a hostile one, accepted by
// command; a caravan reaching the tile opens the site map with the camp's
// walls and defenders; once more than half of them have fallen the next
// hourly census succeeds it, the reward pod is sent home and the giver
// gains its 8 goodwill.

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/biome"
)

// campScene is a landed world colony with one faction able to ask for the
// camp (index gi, at its type's baseline goodwill, so the daily drift
// leaves it be) and every other friendly-capable human faction with a
// settlement hostile.
func campScene(t *testing.T) (g *Game, gi int) {
	t.Helper()
	g = worldGame(t)
	gi = -1
	for i := range g.ng.factions {
		f := &g.ng.factions[i]
		d := faction.DefOf(f.Type)
		if !f.Type.Human() || !d.CanBeFriendly || len(f.Settlements) == 0 {
			continue
		}
		f.Defeated, f.Besieged = false, false
		if gi < 0 {
			gi = i
			f.Relation, f.Goodwill = faction.Neutral, d.Baseline
			continue
		}
		f.Relation, f.Goodwill = faction.Hostile, -80
	}
	if gi < 0 {
		t.Fatal("fixture: no friendly-capable human faction with a settlement")
	}
	return g, gi
}

func TestCampClearedThroughTheGame(t *testing.T) {
	t.Parallel()
	g, gi := campScene(t)
	id, ok := questOf(g).Offer(qd.Camp)
	if !ok {
		t.Fatal("the camp was not offered")
	}
	v := questView(t, g, id)
	if v.State != qd.Offered || v.Giver != gi+1 || v.Target <= 0 || v.Tile < 0 || len(v.Rewards) == 0 {
		t.Fatalf("offer: %+v", v)
	}
	choice := slices.IndexFunc(v.Rewards, func(r quest.RewardView) bool { return len(r.Lines) > 0 })
	if choice < 0 {
		t.Fatalf("no items option: %+v", v.Rewards)
	}
	g.Enqueue(command.Command{Kind: command.AcceptQuest, Index: int(id), Count: choice})
	g.Step(1)
	if v = questView(t, g, id); v.State != qd.Active || v.Deadline == 0 {
		t.Fatalf("not accepted: %+v", v)
	}
	if h := saveAndLoad(t, g); h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after save and load")
	}

	// A caravan is placed on the tile and Fleet.Arrived is called, as the
	// road does when a real one stops there (walking it takes days).
	home := pieces(g.maps[0])
	c := visitingCaravan(t, g, v.Tile)
	fleetOf(g).Arrived(c, uint64(g.clock.Now()))
	g.Step(1) // the open request is flushed at the tick's end
	if len(g.maps) != 2 {
		t.Fatalf("no site map opened: %d maps", len(g.maps))
	}
	m := g.maps[1]
	m.climate = biome.DefOf(biome.TemperateForest).Climate // the test planet is all sea ice
	if m.tile != v.Tile {
		t.Fatalf("the site map is on tile %d, want %d", m.tile, v.Tile)
	}
	// The camp was laid while the site map was current: its pieces stand
	// there, and home gained none.
	count := pieces(m)
	if got := pieces(g.maps[0]); !reflect.DeepEqual(got, home) {
		t.Fatalf("home's buildings changed when the camp was laid: %v, was %v", got, home)
	}
	if count[construction.Wall] < 40 || count[construction.Door] != 1 || count[construction.Bed] < 3 {
		t.Fatalf("camp pieces %v", count)
	}
	ds := defenders(m)
	if len(ds) == 0 || !slices.ContainsFunc(m.raidGroups, func(gr raid.Group) bool { return gr.Faction == v.Target && gr.Size == len(ds) }) {
		t.Fatalf("%d defenders, groups %+v, want them of faction ref %d", len(ds), m.raidGroups, v.Target)
	}
	n, _ := questOf(g).Get(id)
	if n.Parts[0].Count != len(ds) {
		t.Fatalf("the quest counted %d defenders, %d stand", n.Parts[0].Count, len(ds))
	}
	g.Step(1) // the rooms and roofs the camp's walls make are worked out on the next tick
	if h := saveAndLoad(t, g); h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after save and load, camp laid")
	}

	gw0 := g.ng.factions[gi].Goodwill
	fell(g, ds[:len(ds)/2+1]...) // more than half fall
	for i := 0; i < 3 && questView(t, g, id).State == qd.Active; i++ {
		nextHours(g, 1)
	}
	v = questView(t, g, id)
	if v.State != qd.Succeeded {
		t.Fatalf("not cleared with %d of %d down: %+v", len(ds)/2+1, len(ds), v)
	}
	if got := g.ng.factions[gi].Goodwill - gw0; got != 8 {
		t.Fatalf("giver goodwill moved by %d, want 8", got)
	}
	if !slices.ContainsFunc(g.ng.factions[gi].Events, func(e string) bool { return strings.Contains(e, "+8 goodwill") }) {
		t.Fatalf("no goodwill event: %v", g.ng.factions[gi].Events)
	}
	opt := v.Rewards[choice]
	if !slices.ContainsFunc(g.trade.pods, func(p tradePod) bool { return reflect.DeepEqual(p.Items, opt.Lines) }) {
		t.Fatalf("no reward pod with %v: %+v", opt.Lines, g.trade.pods)
	}
}

// pieces counts map m's buildings by kind.
func pieces(m *mapState) map[construction.Kind]int {
	count := map[construction.Kind]int{}
	for _, bid := range m.bldgIDs {
		b, _ := m.bldgs.Get(bid)
		count[b.Kind]++
	}
	return count
}
