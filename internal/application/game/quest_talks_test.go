package game

// quest_talks_test.go: the peace talks through the game (QST-075..079,
// QST-096, QST-119): offered by a hostile faction that could be friends,
// active at once on a talk tile with no deadline; held by the Negotiate
// command or a caravan's arrival (Social XP, goodwill, the gift); failed
// when the host loses its last settlement.

import (
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/roster"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/skill"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/caravan"
)

// talksScene is a landed world colony whose only talks host is one human,
// friendly-capable faction (index hi), hostile at -40 goodwill (still
// inside the hostile band's calm-down margin, so no outcome clamps); every
// other friendly-capable faction is neutral. The talks are offered at once.
// With settlements, every faction's settlements are cleared and the host
// gets those.
func talksScene(t *testing.T, g *Game, settlements []int) (hi int, id qd.ID, tile int) {
	t.Helper()
	hi = -1
	for i := range g.ng.factions {
		f := &g.ng.factions[i]
		if settlements != nil {
			f.Settlements, f.SettlementNames = nil, nil
		}
		switch {
		case !f.Type.Human() || !faction.DefOf(f.Type).CanBeFriendly:
		case hi < 0:
			hi = i
			f.Relation, f.Goodwill, f.Defeated, f.Besieged = faction.Hostile, -40, false, false
			if settlements != nil {
				f.Settlements = settlements
			}
		default:
			f.Relation, f.Goodwill = faction.Neutral, 0
		}
	}
	if hi < 0 {
		t.Fatal("fixture: no friendly-capable human faction")
	}
	id, ok := questOf(g).Offer(qd.Talks)
	if !ok {
		t.Fatal("the talks were not offered")
	}
	v := questView(t, g, id)
	if v.State != qd.Active || v.Giver != hi+1 || v.Tile < 0 || v.Deadline != 0 || len(v.Rewards) != 0 {
		t.Fatalf("offer: %+v, want active for faction %d on a tile, no deadline, no rewards", v, hi+1)
	}
	return hi, id, v.Tile
}

// arrivedCaravan puts a caravan with colonist id on tile in the Arrived
// state. Placed directly, and Fleet.Arrived is not called: travelling to a
// talk tile 6-25 tiles away would take days.
func arrivedCaravan(t *testing.T, g *Game, tile int, id entity.EntityID) uint64 {
	t.Helper()
	w, home := g.worldS(), g.colonyTile()
	cid := g.entities.Next()
	w.caravans = append(w.caravans, maps.Caravan{C: caravan.Caravan{ID: uint64(cid), Tile: tile, Dest: tile,
		State: caravan.Arrived, FoodDays: maps.FoodDays}, Home: home, Out: testPlanet().Route(home, tile)})
	if !g.stashPawn(id, cid) {
		t.Fatal("could not stash the colonist")
	}
	c := fleetOf(g).At(cid)
	c.C.Members = append(c.C.Members, uint64(id))
	return uint64(cid)
}

// checkTalksHeld: quest id succeeded, the negotiator gained the talks' XP,
// the host's goodwill moved by the recorded change and a gift is in the
// caravan's cargo.
func checkTalksHeld(t *testing.T, g *Game, id qd.ID, hi int, cid uint64, who entity.EntityID, xp0 int64, gw0 int) {
	t.Helper()
	n, ok := questOf(g).Get(id)
	if !ok || n.State != qd.Succeeded || len(n.Parts) != 1 || !n.Parts[0].Done {
		t.Fatalf("talks not held: %+v", n)
	}
	if got := roster.TotalXP(g.skillOf(who, skill.Social)) - xp0; got != qd.TalkXP {
		t.Fatalf("social xp rose by %d, want %d", got, qd.TalkXP)
	}
	if got := g.ng.factions[hi].Goodwill - gw0; got != n.Parts[0].Points {
		t.Fatalf("goodwill moved by %d, want the recorded %d", got, n.Parts[0].Points)
	}
	for _, l := range n.Parts[0].Lines {
		if countOf(g, cid, l.Kind) < l.Qty {
			t.Fatalf("gift %+v not in the cargo: %+v", l, travelOf(g).Goods(cid))
		}
	}
}

func TestTalksHeldByNegotiateCommand(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	hi, id, tile := talksScene(t, g, nil)
	if h := saveAndLoad(t, g); h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after save and load")
	}
	who := g.pawnIDs[0]
	cid := arrivedCaravan(t, g, tile, who)
	xp0, gw0 := roster.TotalXP(g.skillOf(who, skill.Social)), g.ng.factions[hi].Goodwill
	g.Enqueue(command.Command{Kind: command.Negotiate, Index: int(id), Target: cid})
	g.Step(1)
	checkTalksHeld(t, g, id, hi, cid, who, xp0, gw0)
	if h := saveAndLoad(t, g); h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after save and load, talks held")
	}
}

func TestTalksHeldOnArrival(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	hi, id, tile := talksScene(t, g, nil)
	who := g.pawnIDs[0]
	xp0, gw0 := roster.TotalXP(g.skillOf(who, skill.Social)), g.ng.factions[hi].Goodwill
	// arriveAt calls Fleet.Arrived on the talk tile, as the road does when a
	// real caravan stops there.
	cid := arriveAt(t, g, tile, who)
	checkTalksHeld(t, g, id, hi, uint64(cid), who, xp0, gw0)
}

func TestTalksFailWhenTheHostFalls(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	base := plainNear(t, g)
	hi, id, _ := talksScene(t, g, []int{base})
	// The real path: a caravan attacks the host's last settlement, every
	// defender falls, the settlement is destroyed (maps settle.go destroy →
	// SettlementLost → SigSiteLost and SigFactionDefeated).
	cid := arriveAt(t, g, base, g.pawnIDs[0])
	if v := questView(t, g, id); v.State != qd.Active {
		t.Fatalf("an arrival at the host's settlement ended the talks: %+v", v)
	}
	g.Enqueue(command.Command{Kind: command.AttackSettlement, Target: uint64(cid)})
	g.Step(1)
	if len(g.maps) != 2 {
		t.Fatalf("no settlement map opened: %d maps", len(g.maps))
	}
	m := g.maps[1]
	m.climate = biome.DefOf(biome.TemperateForest).Climate // the test planet is all sea ice
	fell(g, defenders(m)...)
	nextHours(g, 4)
	if !g.ng.factions[hi].Defeated {
		t.Fatalf("the host was not defeated: %+v", g.ng.factions[hi])
	}
	if v := questView(t, g, id); v.State != qd.Failed || v.Reason != qd.SiteLost {
		t.Fatalf("talks with a fallen host: %+v, want failed (site lost)", v)
	}
}
