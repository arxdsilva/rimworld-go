package game

// quest_orelump_test.go: the ore lump quest inside the game (QST-045..049,
// QST-116): the scanner's find offers it, an arrival succeeds it, the 30
// days failing it.

import (
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/domain/economy/item"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/caravan"
)

// oreFind makes the scanner find a gold site and returns the live ore lump
// quest on it.
func oreFind(t *testing.T) (*Game, QuestView) {
	t.Helper()
	g := worldGame(t)
	before := len(g.letterViews())
	if !travelOf(g).Find(item.Gold) {
		t.Fatal("no site found")
	}
	var found []QuestView
	for _, v := range g.World().Quests {
		if v.Kind == qd.OreLump {
			found = append(found, v)
		}
	}
	sites := g.World().AX.Sites
	if len(found) != 1 || len(sites) != 1 {
		t.Fatalf("one site, one quest: %+v / %+v", sites, found)
	}
	v := found[0]
	if v.State != qd.Active || v.Tile != int(sites[0].Tile) || v.Deadline != uint64(g.clock.Now())+30*clock.TicksPerDay {
		t.Fatalf("an active quest on the site with a 30-day deadline: %+v", v)
	}
	letters := g.letterViews()[before:]
	for _, l := range letters {
		if l.Title == "Mineral site found" {
			t.Fatalf("the scanner's own letter was posted: %+v", l)
		}
	}
	if len(letters) != 1 || letters[0].Quest != uint32(v.ID) {
		t.Fatalf("only the quest's letter: %+v", letters)
	}
	return g, v
}

func TestOreLumpScannerOffersActiveQuest(t *testing.T) {
	t.Parallel()
	g, v := oreFind(t)
	h := saveAndLoad(t, g)
	if h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after load")
	}
	g.Step(2 * clock.TicksPerHour)
	h.Step(2 * clock.TicksPerHour)
	if h.StateHash() != g.StateHash() {
		t.Fatal("a loaded game goes on as the saved one does")
	}
	if got := questView(t, h, v.ID); got.State != qd.Active {
		t.Fatalf("after load: %+v", got)
	}
}

func TestOreLumpArrivalSucceeds(t *testing.T) {
	t.Parallel()
	g, v := oreFind(t)
	// A caravan is placed on the tile and Fleet.Arrived is called, as the
	// road does when a real one stops there (driving the walk is slow).
	w := g.worldS()
	w.caravans = append(w.caravans, maps.Caravan{C: caravan.Caravan{ID: 999999, Tile: v.Tile, State: caravan.Arrived},
		Home: g.colonyTile()})
	fleetOf(g).Arrived(&w.caravans[len(w.caravans)-1], uint64(g.clock.Now()))
	if got := questView(t, g, v.ID); got.State != qd.Succeeded {
		t.Fatalf("an arrival on the tile succeeds it: %+v", got)
	}
}

func TestOreLumpTimesOut(t *testing.T) {
	t.Parallel()
	g, v := oreFind(t)
	questOf(g).Hourly(v.Deadline - 1)
	if got := questView(t, g, v.ID); got.State != qd.Active {
		t.Fatalf("active a tick before the deadline: %+v", got)
	}
	questOf(g).Hourly(v.Deadline)
	if got := questView(t, g, v.ID); got.State != qd.Failed {
		t.Fatalf("failed when the 30 days run out: %+v", got)
	}
}
