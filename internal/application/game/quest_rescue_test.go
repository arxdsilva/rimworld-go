package game

// quest_rescue_test.go: the prisoner rescue through the game (QST-065..069,
// decision 13): a caravan opens the quest's site, the camp stands with its
// guards and a walled cell, the prisoner is held in it; once the guards are
// down the cell door is claimed, a colonist is ordered to free the prisoner
// and the quest succeeds with the pawn a colonist.
//
// It needs the care_captives glue (care.Roles.Captives): until the lead has
// applied the patch from the QST-2f report, the free order is refused.

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/mapgen"
)

// rescueSite offers the quest against a hostile settlement faction, sends a
// two-pawn caravan to the camp tile and steps once: the site is laid.
func rescueSite(t *testing.T) (g *Game, id qd.ID, guest entity.EntityID, m *mapState, cell mapgen.Cell) {
	t.Helper()
	g = worldGame(t)
	for i := range g.ng.factions {
		if f := &g.ng.factions[i]; f.Type.Human() && len(f.Settlements) > 0 {
			f.Relation, f.Defeated = faction.Hostile, false
			break
		}
	}
	id, ok := questOf(g).Offer(qd.Rescue)
	if !ok {
		t.Fatal("the rescue was not offered")
	}
	v := questView(t, g, id)
	if v.State != qd.Active || v.Tile < 0 || len(v.Rewards) != 1 || !v.Rewards[0].HasPawn {
		t.Fatalf("an active quest on a site with a pawn reward: %+v", v)
	}
	arriveAt(t, g, v.Tile, g.pawnIDs[0], g.pawnIDs[1])
	g.Step(1)
	n, _ := questOf(g).Get(id)
	guest = entity.EntityID(n.Parts[1].Pawn)
	if guest == 0 || n.Parts[0].Map == 0 {
		t.Fatalf("the site was not laid: %+v", n.Parts)
	}
	m = mapByID(g, n.Parts[0].Map)
	if m == nil || onMap(g, guest) != m {
		t.Fatal("the prisoner is not on the site map")
	}
	tech := faction.Neolithic
	if n.Parts[0].Layout == rescueIndustrialLayout {
		tech = faction.Industrial
	}
	_, cell = mapgen.CellCamp(mapgen.CompoundConfig{W: m.grid.Width(), H: m.grid.Height(), Tech: tech, Seed: n.Parts[0].Seed})
	g.ViewMap(MapID(m.id))
	return g, id, guest, m, cell
}

// rescueIndustrialLayout is the kind's industrial layout (kinds.rescueIndustrial).
const rescueIndustrialLayout = 1

// clearGuards removes the camp's guards from m but not the prisoner, who is
// listed with the raiders (a guest is a visitor).
func clearGuards(g *Game, m *mapState, prisoner entity.EntityID) {
	g.mapState = m
	for _, id := range slices.Clone(m.raiderIDs) {
		if id != prisoner {
			g.removeRaider(id)
		}
	}
	g.mapState = g.view
}

// claimCamp claims every door and turret of the camp (the player's Claim
// order over the whole map): the guards are down, so the doors are ours.
func claimCamp(g *Game, m *mapState) {
	g.Enqueue(command.Command{Kind: command.DesignateClaim, X: 0, Y: 0, X2: m.grid.Width() - 1, Y2: m.grid.Height() - 1})
}

// freeOrder orders colonist doer to free guest.
func freeOrder(g *Game, doer, guest entity.EntityID) {
	g.Enqueue(command.Command{Kind: command.Prioritize, Order: command.OrderFreePrisoner, Pawn: uint64(doer), Target: uint64(guest)})
}

func TestRescueSiteHoldsPrisonerInWalledCell(t *testing.T) {
	t.Parallel()
	g, id, guest, m, cell := rescueSite(t)
	if pos, _ := m.positions.Get(guest); pos.X != cell.Inside.X || pos.Y != cell.Inside.Y {
		t.Fatalf("the prisoner stands at %v, the cell's inside is %v", pos, cell.Inside)
	}
	if !g.visitor(guest) {
		t.Fatal("the prisoner is a guest")
	}
	door := g.layerBldgAt(cell.Door.X, cell.Door.Y, construction.LayerObject)
	if b, ok := m.bldgs.Get(door); !ok || !b.Kind.IsDoor() || !b.Hostile {
		t.Fatalf("the cell's door is the camp's: %+v", b)
	}
	guards := 0
	for _, grp := range m.raidGroups {
		guards += grp.Size
	}
	if guards == 0 {
		t.Fatal("no guards stand in the camp")
	}
	if d := questView(t, g, id).Detail; d[len(d)-1].Key != "quest.rescue.held" {
		t.Fatalf("detail %+v", d)
	}
	if h := saveAndLoad(t, g); h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after save and load")
	}
}

func TestRescueFreeThePrisoner(t *testing.T) {
	t.Parallel()
	g, id, guest, m, _ := rescueSite(t)
	doer := m.pawnIDs[0]
	before := len(g.pawnIDs)
	// With the guards standing, the order cannot be carried out.
	clearGuards(g, m, guest)
	g.Step(1)
	// The camp's doors, the cell's included, are the enemy's until the
	// player claims them (decision 13).
	freeOrder(g, doer, guest)
	g.Step(600)
	if v := questView(t, g, id); v.State != qd.Active {
		t.Fatalf("freed through the enemy's door: %+v", v)
	}
	claimCamp(g, m)
	g.Step(1)
	freeOrder(g, doer, guest)
	for i := 0; i < 60 && questView(t, g, id).State == qd.Active; i++ {
		g.Step(60)
	}
	v := questView(t, g, id)
	if v.State != qd.Succeeded {
		t.Fatalf("not rescued: %+v", v)
	}
	if g.visitor(guest) || len(g.pawnIDs) != before+1 {
		t.Fatalf("the prisoner is a colonist, no second pawn walks in: visitor %v pawns %d want %d", g.visitor(guest), len(g.pawnIDs), before+1)
	}
	if s, _ := g.vital.Get(guest); s.Status != health.Alive {
		t.Fatalf("the prisoner is %v", s.Status)
	}
	if d := v.Detail; d[len(d)-1].Key != "quest.rescue.freed" {
		t.Fatalf("detail %+v", d)
	}
}

func TestRescuePrisonerDyingFailsIt(t *testing.T) {
	t.Parallel()
	g, id, guest, _, _ := rescueSite(t)
	g.hurt(guest, 100000, 0)
	g.Step(2)
	if v := questView(t, g, id); v.State != qd.Failed || v.Reason != qd.PawnLost {
		t.Fatalf("a dead prisoner fails the quest: %+v", v)
	}
}

func TestRescueSurvivesSaveAndLoad(t *testing.T) {
	t.Parallel()
	g, id, guest, m, _ := rescueSite(t)
	clearGuards(g, m, guest)
	claimCamp(g, m)
	g.Step(1)
	h := saveAndLoad(t, g)
	if h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after load")
	}
	doer := m.pawnIDs[0]
	for _, x := range []*Game{g, h} {
		freeOrder(x, doer, guest)
		for i := 0; i < 60 && questView(t, x, id).State == qd.Active; i++ {
			x.Step(60)
		}
	}
	if h.StateHash() != g.StateHash() || questView(t, h, id).State != qd.Succeeded {
		t.Fatal("a loaded game goes on as the saved one does")
	}
}
