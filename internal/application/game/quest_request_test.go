package game

// quest_request_test.go: the trade request through the game (QST-2g):
// offer, accept by command, a caravan visiting the settlement, Fulfil by
// command, the goods filter (QST-071), the reward into the caravan, the
// goodwill (QST-095) and the held visit (QST-118, decision 14).

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/application/game/quest/kinds"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// askTuque is the real Request kind asking for one tuque instead of a
// stackable good, so the quality and taint filters can be exercised.
type askTuque struct{ kinds.Request }

func (k askTuque) Build(c quest.Ctx, q *qd.Quest) bool {
	if !k.Request.Build(c, q) {
		return false
	}
	q.Parts[0].Lines = []trade.Line{{Kind: item.Tuque, Qty: 1}}
	return true
}

// requestScene is a landed world colony whose only willing giver is one
// human faction (index gi) with its single settlement on the returned
// tile; every other human faction is hostile.
func requestScene(t *testing.T) (g *Game, gi, tile int) {
	t.Helper()
	g = worldGame(t)
	tile = nearDest(t, g)
	gi = -1
	for i := range g.ng.factions {
		f := &g.ng.factions[i]
		switch {
		case gi < 0 && f.Type.Human() && faction.DefOf(f.Type).CanBeFriendly:
			gi = i
			f.Relation, f.Defeated, f.Goodwill, f.Settlements = faction.Neutral, false, 0, []int{tile}
		case f.Type.Human():
			f.Relation = faction.Hostile
		}
	}
	if gi < 0 {
		t.Fatal("fixture: no friendly-capable human faction")
	}
	g.worldS().quests = quest.New(questHost{g}.roles(), []quest.Kind{askTuque{}})
	return g, gi, tile
}

// visitingCaravan puts a caravan with one colonist on tile, Visiting for its
// normal day. Placed directly: travelling there would take days.
func visitingCaravan(t *testing.T, g *Game, tile int) *maps.Caravan {
	t.Helper()
	w, home := g.worldS(), g.colonyTile()
	cid := g.entities.Next()
	w.caravans = append(w.caravans, maps.Caravan{C: caravan.Caravan{ID: uint64(cid), Tile: tile, Dest: tile,
		State: caravan.Visiting, VisitUntil: uint64(g.clock.Now()) + clock.TicksPerDay, FoodDays: maps.FoodDays},
		Home: home, Out: testPlanet().Route(home, tile)})
	id := g.pawnIDs[0]
	if !g.stashPawn(id, cid) {
		t.Fatal("could not stash the colonist")
	}
	c := fleetOf(g).At(cid)
	c.C.Members = append(c.C.Members, uint64(id))
	return c
}

func countOf(g *Game, cid uint64, k item.Kind) (n int) {
	for _, st := range travelOf(g).Goods(cid) {
		if st.Kind == k {
			n += st.Qty
		}
	}
	return n
}

func TestRequestFulfilThroughCommands(t *testing.T) {
	t.Parallel()
	g, gi, tile := requestScene(t)
	id, ok := questOf(g).Offer(qd.Request)
	if !ok {
		t.Fatal("fixture: the request is offered")
	}
	if v := questView(t, g, id); v.Giver != gi+1 || v.Tile != tile || v.Detail[0].Key != "quest.request.item" {
		t.Fatalf("offer: %+v", v)
	}
	g.Enqueue(command.Command{Kind: command.AcceptQuest, Index: int(id), Count: 0})
	g.Step(1)
	v := questView(t, g, id)
	if v.State != qd.Active {
		t.Fatalf("not accepted: %+v", v)
	}
	if h := saveAndLoad(t, g); h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after save and load")
	}

	c := visitingCaravan(t, g, tile)
	cid := c.C.ID
	until := c.C.VisitUntil
	fulfil := func() QuestView {
		g.Enqueue(command.Command{Kind: command.FulfilRequest, Index: int(id), Target: cid})
		g.Step(1)
		return questView(t, g, id)
	}
	travelOf(g).Carry(cid, item.Stack{Kind: item.Tuque, Qty: 1, Quality: quality.Stored(quality.Poor)})
	travelOf(g).Carry(cid, item.Stack{Kind: item.Tuque, Qty: 1, Quality: quality.Stored(quality.Normal), Of: 7})
	if v := fulfil(); v.State != qd.Active || countOf(g, cid, item.Tuque) != 2 || g.ng.factions[gi].Goodwill != 0 {
		t.Fatalf("poor or tainted goods were taken: %+v", v)
	}

	// The visit is held past its normal day while the request waits.
	worldForward(g, int(until-uint64(g.clock.Now()))+200)
	if c = fleetOf(g).At(entity.EntityID(cid)); c == nil || c.C.State != caravan.Visiting || uint64(g.clock.Now()) <= until {
		t.Fatalf("the visit was not held past its day: %+v", c)
	}

	travelOf(g).Carry(cid, item.Stack{Kind: item.Tuque, Qty: 1, Quality: quality.Stored(quality.Normal)})
	v = fulfil()
	if v.State != qd.Succeeded {
		t.Fatalf("not fulfilled: %+v", v)
	}
	if countOf(g, cid, item.Tuque) != 2 {
		t.Fatalf("only the good tuque should have left: %d left", countOf(g, cid, item.Tuque))
	}
	for _, st := range travelOf(g).Goods(cid) {
		if st.Kind == item.Tuque && st.Quality == quality.Stored(quality.Normal) && st.Of == 0 {
			t.Fatal("the good tuque is still in the cargo")
		}
	}
	opt := v.Rewards[v.Chosen]
	for _, l := range opt.Lines {
		if countOf(g, cid, l.Kind) < l.Qty {
			t.Fatalf("reward %+v did not land in the caravan: %+v", l, travelOf(g).Goods(cid))
		}
	}
	if got := g.ng.factions[gi].Goodwill; got != 12+opt.Goodwill {
		t.Fatalf("goodwill %d, want %d", got, 12+opt.Goodwill)
	}

	// With the request done the visit ends on its own.
	worldForward(g, 300)
	if c = fleetOf(g).At(entity.EntityID(cid)); c != nil && c.C.State == caravan.Visiting {
		t.Fatalf("still visiting after the request: %+v", c.C)
	}
	if slices.ContainsFunc(g.World().Quests, func(q QuestView) bool { return q.State == qd.Failed }) {
		t.Fatal("a quest failed")
	}
}
