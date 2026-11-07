package game

// quest_joiner_test.go: the threatened joiner inside the game (QST-2a):
// offer, accept by command, the pawn walks in, the threat arrives, the
// quest succeeds, and a save taken in between goes on identically.

import (
	"testing"

	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// joinerScene is a landed setup colony with the real kinds, one neutral
// friendly-capable human faction and one hostile human faction.
func joinerScene(t *testing.T) *Game {
	t.Helper()
	g := commitSetup(t)
	seenGiver, seenEnemy := false, false
	for i := range g.ng.factions {
		f := &g.ng.factions[i]
		if !f.Type.Human() {
			continue
		}
		switch {
		case !seenGiver && faction.DefOf(f.Type).CanBeFriendly:
			f.Relation, f.Defeated, f.Goodwill = faction.Neutral, false, 0
			seenGiver = true
		case !seenEnemy:
			f.Relation, f.Defeated, f.Goodwill = faction.Hostile, false, -80
			seenEnemy = true
		}
	}
	if !seenGiver || !seenEnemy {
		t.Fatalf("fixture: giver %v enemy %v", seenGiver, seenEnemy)
	}
	g.worldS().quests = nil
	return g
}

func TestJoinerQuestInTheGame(t *testing.T) {
	t.Parallel()
	g := joinerScene(t)
	if n := len(g.Colonists()); n >= 12 {
		t.Fatalf("fixture: %d colonists", n)
	}
	id, ok := questOf(g).Offer(qd.Joiner)
	if !ok {
		t.Fatal("the joiner is offered")
	}
	v := questView(t, g, id)
	if v.State != qd.Offered || len(v.Rewards) != 1 || v.Rewards[0].Pawn.Name == "" {
		t.Fatalf("one pawn reward: %+v", v)
	}
	want := v.Rewards[0].Pawn.Name
	before := len(g.Colonists())

	g.Enqueue(command.Command{Kind: command.AcceptQuest, Index: int(id), Count: 0})
	g.Step(1)
	v = questView(t, g, id)
	if v.State != qd.Active {
		t.Fatalf("accepted: %+v", v)
	}
	if got := len(g.Colonists()); got != before+1 {
		t.Fatalf("colonists %d, want %d", got, before+1)
	}
	found := false
	for _, c := range g.Colonists() {
		found = found || c.Name == want
	}
	if !found {
		t.Fatalf("no colonist named %q", want)
	}

	h := saveAndLoad(t, g)
	if h.StateHash() != g.StateHash() {
		t.Fatal("state hash differs after load")
	}
	lettersBefore := len(g.letterViews())
	g.Step(2 * clock.TicksPerHour)
	h.Step(2 * clock.TicksPerHour)
	if h.StateHash() != g.StateHash() {
		t.Fatal("a loaded game goes on as the saved one does")
	}
	g.Step(5 * clock.TicksPerHour)
	h.Step(5 * clock.TicksPerHour)
	if h.StateHash() != g.StateHash() {
		t.Fatal("a loaded game goes on as the saved one does, past the threat")
	}
	if v := questView(t, g, id); v.State != qd.Succeeded {
		t.Fatalf("the quest succeeds once its threat has come: %+v", v)
	}
	if hv := questView(t, h, id); hv.State != qd.Succeeded {
		t.Fatalf("loaded game: %+v", hv)
	}
	// A success letter and the threat's own letter (raid, pack or climate).
	if got := len(g.letterViews()); got < lettersBefore+2 {
		t.Fatalf("letters %d, want at least %d", got, lettersBefore+2)
	}
}
