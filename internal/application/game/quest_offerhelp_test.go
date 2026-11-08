package game

// quest_offerhelp_test.go: "Offer help to <name>" on the downed refugee
// (QST-055, QST-057, QST-059): a colonist on the site map is offered the
// order at the person's tile, walks over and the person joins.

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/pawn/mood"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// helpScene is the unguarded refugee site with its map in view and colonist
// doer on it, with the order's menu row at the person's tile.
func helpScene(t *testing.T) (*Game, qd.ID, entity.EntityID, *mapState, entity.EntityID) {
	t.Helper()
	g, id, guest, m := refugeeSiteWith(t, false)
	g.ViewMap(MapID(m.id))
	doer := m.pawnIDs[0]
	return g, id, guest, m, doer
}

func offerHelpOrder(g *Game, doer, guest entity.EntityID) command.Command {
	x, y := tileOf(g, guest)
	return command.Command{Kind: command.Prioritize, Order: command.OrderOfferHelp, Pawn: uint64(doer), Target: uint64(guest), X: x, Y: y}
}

func TestOfferHelpMenuAtThePersonsTile(t *testing.T) {
	t.Parallel()
	g, _, guest, _, doer := helpScene(t)
	x, y := tileOf(g, guest)
	name := g.personName(guest)
	for _, drafted := range []bool{false, true} {
		if drafted {
			g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(doer), Enabled: true})
			g.Step(1)
		}
		r := orderRow(g, doer, x, y, "Offer help to")
		if r == nil || r.Label != "Offer help to "+name || r.Cmd.Order != command.OrderOfferHelp || r.Cmd.Target != uint64(guest) || r.Cmd.Pawn != uint64(doer) {
			t.Fatalf("drafted %v: offer-help row %+v in %q", drafted, r, labelsAt(g, doer, x, y))
		}
	}
}

func TestOfferHelpWalksOverAndTheyJoin(t *testing.T) {
	t.Parallel()
	g, id, guest, m, doer := helpScene(t)
	before := len(g.Colonists())
	g.Enqueue(offerHelpOrder(g, doer, guest))
	for i := 0; i < 60 && questView(t, g, id).State == qd.Active; i++ {
		g.Step(60)
	}
	if v := questView(t, g, id); v.State != qd.Succeeded {
		t.Fatalf("not succeeded: %+v", v)
	}
	if g.visitor(guest) || len(g.Colonists()) != before+1 {
		t.Fatalf("the guest is a colonist, no second pawn walks in: visitor %v colonists %d want %d", g.visitor(guest), len(g.Colonists()), before+1)
	}
	found := false
	for _, th := range g.thoughts(guest) {
		found = found || th.Kind == mood.RescuedByColony
	}
	if !found {
		t.Fatalf("no RescuedByColony thought: %+v", g.thoughts(guest))
	}
	if onMap(g, guest) != m {
		t.Fatal("the new colonist left the site map")
	}
}

func TestOfferHelpSurvivesSaveAndLoad(t *testing.T) {
	t.Parallel()
	g, id, guest, _, doer := helpScene(t)
	g.Enqueue(offerHelpOrder(g, doer, guest))
	g.Step(2) // the order is on the doer; the walk has begun
	h := saveAndLoad(t, g)
	if h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after load")
	}
	for _, x := range []*Game{g, h} {
		for i := 0; i < 60 && questView(t, x, id).State == qd.Active; i++ {
			x.Step(60)
		}
	}
	if questView(t, g, id).State != qd.Succeeded || questView(t, h, id).State != qd.Succeeded || h.StateHash() != g.StateHash() {
		t.Fatal("a loaded game goes on as the saved one does")
	}
}

// QST-059: after joining, the downed new colonist on the site map is tended
// by a colonist there.
func TestOfferHelpNewColonistGetsTended(t *testing.T) {
	t.Parallel()
	g, id, guest, _, doer := helpScene(t)
	g.Enqueue(offerHelpOrder(g, doer, guest))
	for i := 0; i < 60 && questView(t, g, id).State == qd.Active; i++ {
		g.Step(60)
	}
	if !g.needsTending(guest) {
		t.Fatalf("the hurt new colonist needs no tending at once: status %v", vitalStatus(g, guest))
	}
	for i := 0; i < 48 && g.needsTending(guest); i++ {
		g.Step(clock.TicksPerHour)
	}
	if g.needsTending(guest) {
		t.Fatalf("nobody tended the new colonist in two days: doer %d, status %v", doer, vitalStatus(g, guest))
	}
}

func vitalStatus(g *Game, id entity.EntityID) any { s, _ := g.vital.Get(id); return s.Status }

// A caravan reformed from the site map takes the joined person along (they
// are a colonist now, downed or not).
func TestOfferHelpJoinedPersonLeavesWithTheReformedCaravan(t *testing.T) {
	t.Parallel()
	g, id, guest, m, doer := helpScene(t)
	g.Enqueue(offerHelpOrder(g, doer, guest))
	for i := 0; i < 60 && questView(t, g, id).State == qd.Active; i++ {
		g.Step(60)
	}
	g.Enqueue(command.Command{Kind: command.CaravanReform, Target: uint64(m.id), Map: m.id, MapSet: true})
	g.Step(1)
	if len(g.worldPlanet().Route(questView(t, g, id).Tile, g.colonyTile())) < 2 {
		// the test planet has no land route from this site home: the map
		// keeps everyone (nobody is left behind), the person included
		if len(g.maps) != 2 || onMap(g, guest) != m {
			t.Fatal("with no way home the reformed map must keep its people")
		}
		return
	}
	c := fleetOf(g).Of(guest)
	if c == nil || !slices.Contains(c.C.Members, uint64(guest)) {
		t.Fatalf("the new colonist is not in a caravan: %+v", c)
	}
}
