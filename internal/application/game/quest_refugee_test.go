package game

// quest_refugee_test.go: the downed refugee inside the game (QST-055..059):
// a caravan opens the quest's site, the person lies down on the site map,
// "offer help" takes them into the colony and the quest succeeds.

import (
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// refugeeSite offers the quest, sends a two-pawn caravan to its tile and
// steps once: the site map is open and the person lies on it.
func refugeeSite(t *testing.T) (*Game, qd.ID, entity.EntityID, *mapState) {
	t.Helper()
	return refugeeSiteWith(t, true)
}

// refugeeSiteWith is refugeeSite; guards false makes every faction neutral
// first, so no armed strangers stand by the person.
func refugeeSiteWith(t *testing.T, guards bool) (*Game, qd.ID, entity.EntityID, *mapState) {
	t.Helper()
	g := worldGame(t)
	if !guards {
		for i := range g.ng.factions {
			g.ng.factions[i].Relation = faction.Neutral
		}
	}
	id, ok := questOf(g).Offer(qd.Refugee)
	if !ok {
		t.Fatal("the refugee is offered")
	}
	v := questView(t, g, id)
	if v.State != qd.Active || v.Tile < 0 || len(v.Rewards) != 1 || !v.Rewards[0].HasPawn {
		t.Fatalf("an active quest on a site with a pawn reward: %+v", v)
	}
	arriveAt(t, g, v.Tile, g.pawnIDs[0], g.pawnIDs[1])
	g.Step(1)
	n, _ := questOf(g).Get(id)
	guest := entity.EntityID(n.Parts[1].Pawn)
	if guest == 0 || n.Parts[0].Map == 0 {
		t.Fatalf("the site was not laid: %+v", n.Parts)
	}
	m := onMap(g, guest)
	if m == nil || m == g.maps[0] {
		t.Fatalf("the guest is not on the site map")
	}
	if s, _ := g.vital.Get(guest); s.Status != health.Downed {
		t.Fatalf("the guest is %v, want downed", s.Status)
	}
	if g.bleedingNow(guest) {
		t.Fatal("the guest bleeds: a refugee's wounds do not (QST-055)")
	}
	return g, id, guest, m
}

func TestRefugeeOfferHelpJoinsThem(t *testing.T) {
	t.Parallel()
	g, id, guest, m := refugeeSite(t)
	if n, _ := questOf(g).Get(id); n.Parts[0].Count == 0 || len(m.raidGroups) != 1 || m.raidGroups[0].Size == 0 {
		t.Fatalf("armed strangers stand by the guest: %+v groups %+v", n.Parts[0], m.raidGroups)
	}
	want := questView(t, g, id).Rewards[0].Pawn.Name
	if got := g.personName(guest); got != want {
		t.Fatalf("the guest is %q, the offer showed %q", got, want)
	}
	before := len(g.Colonists())
	g.Enqueue(command.Command{Kind: command.FulfilRequest, Index: int(id)})
	g.Step(1)
	if v := questView(t, g, id); v.State != qd.Succeeded {
		t.Fatalf("not succeeded: %+v", v)
	}
	if g.visitor(guest) {
		t.Fatal("the guest is still a visitor")
	}
	if got := len(g.Colonists()); got != before+1 {
		t.Fatalf("colonists %d, want %d (no second pawn walks in)", got, before+1)
	}
}

func TestRefugeeDyingOnTheSiteFailsIt(t *testing.T) {
	t.Parallel()
	g, id, guest, _ := refugeeSite(t)
	g.hurt(guest, 100000, 0)
	g.Step(2)
	if v := questView(t, g, id); v.State != qd.Failed || v.Reason != qd.PawnLost {
		t.Fatalf("a dead guest fails the quest: %+v", v)
	}
}

func TestRefugeeSurvivesSaveAndLoad(t *testing.T) {
	t.Parallel()
	// With guards; one tick first, so the site's rooms and roof support
	// (derived, rebuilt after the camp was laid) settle as a load derives
	// them.
	g, id, _, _ := refugeeSite(t)
	g.Step(1)
	h := saveAndLoad(t, g)
	if h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after load")
	}
	for _, x := range []*Game{g, h} {
		x.Enqueue(command.Command{Kind: command.FulfilRequest, Index: int(id)})
		x.Step(1)
	}
	if h.StateHash() != g.StateHash() || questView(t, h, id).State != qd.Succeeded {
		t.Fatal("a loaded game goes on as the saved one does")
	}
}
