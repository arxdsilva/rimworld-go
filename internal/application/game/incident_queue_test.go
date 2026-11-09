package game

import (
	"testing"

	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
)

// TestIncidentQueueFiresAtTick (EAI-030): a queued event fires at its tick,
// not before, and entries due together fire in the order queued.
func TestIncidentQueueFiresAtTick(t *testing.T) {
	t.Parallel()
	g, _ := def4Game(t, 50, 30, 45, 25)
	now := uint64(g.clock.Now())
	g.QueueRaid(now+200, 100)
	g.QueueRaid(now+100, 100)
	g.Step(99)
	if len(g.raidGroups) != 0 || len(g.incidents.Queue) != 2 {
		t.Fatalf("nothing before the tick: %d groups, queue %v", len(g.raidGroups), g.incidents.Queue)
	}
	g.Step(2)
	if len(g.raidGroups) != 1 || len(g.incidents.Queue) != 1 || g.incidents.Queue[0].At != now+200 {
		t.Fatalf("the first fired at its tick: %d groups, queue %v", len(g.raidGroups), g.incidents.Queue)
	}
	g.Step(100)
	if len(g.raidGroups) != 2 || len(g.incidents.Queue) != 0 {
		t.Fatalf("both fired: %d groups, queue %v", len(g.raidGroups), g.incidents.Queue)
	}
}

// TestQueueSurvivesSave: the queue is saved with the incident state.
func TestQueueSurvivesSave(t *testing.T) {
	t.Parallel()
	g, _ := def4Game(t, 50, 30, 45, 25)
	now := uint64(g.clock.Now())
	g.QueueRaid(now+500, 120)
	g.QueueIncident(incident.RefugeeChased, now+300, 0)
	h := saveLoad(t, g)
	q := h.incidents.Queue
	if len(q) != 2 || q[0].ID != incident.RefugeeChased || q[1].ID != incident.RivalRaid || q[1].Points != 120 || q[1].At != now+500 {
		t.Fatalf("queue after a save: %+v", q)
	}
	if view := h.StoryDebug().Queue; len(view) != 2 {
		t.Fatalf("the debug view lists them: %v", view)
	}
}

// refugeeGame is a setup colony with a hostile faction and a refugee's plea
// on the letter stack; it returns the plea's letter.
func refugeeGame(t *testing.T) (*Game, letters.Letter) {
	t.Helper()
	g, _ := def4Game(t, 50, 30, 25, 15)
	p := difficulty.Wanderer
	g.ng.setup = &Setup{Preset: p, Difficulty: difficulty.Of(p), Storyteller: "steady"}
	g.ng.factions = []faction.Faction{{Type: faction.Drifters, Name: "Ash", Relation: faction.Hostile}}
	if !g.refugeeChased(uint64(g.clock.Now())) {
		t.Fatal("the plea is made")
	}
	list := g.letters.List()
	l := list[len(list)-1]
	if len(l.Choices) != 2 || l.Choices[0].Key != letters.Accept || l.Choices[1].Key != letters.Refuse {
		t.Fatalf("the plea offers two answers: %+v", l.Choices)
	}
	return g, l
}

// TestLetterChoiceAccept (EAI-031): taking the refugee in brings a joiner
// and queues the pursuers' raid after a delay.
func TestLetterChoiceAccept(t *testing.T) {
	t.Parallel()
	g, l := refugeeGame(t)
	before := len(g.pawnIDs)
	g.answerLetter(l.ID, 0)
	if len(g.pawnIDs) != before+1 {
		t.Fatalf("a joiner arrives: %d colonists, were %d", len(g.pawnIDs), before)
	}
	if _, ok := g.letters.Get(l.ID); ok {
		t.Fatal("the answered letter leaves the stack")
	}
	q := g.incidents.Queue
	if len(q) != 1 || q[0].ID != incident.RivalRaid || q[0].At != uint64(g.clock.Now())+refugeeDelay || q[0].Points <= 0 {
		t.Fatalf("the pursuers are queued after a delay: %+v", q)
	}
	g.answerLetter(l.ID, 0) // an answered letter cannot be answered again
	if len(g.incidents.Queue) != 1 {
		t.Fatal("answering twice queued twice")
	}
}

// TestLetterChoiceDecline: sending them away does nothing, and a choice that
// is not offered is ignored.
func TestLetterChoiceDecline(t *testing.T) {
	t.Parallel()
	g, l := refugeeGame(t)
	before := len(g.pawnIDs)
	g.answerLetter(l.ID, 5)
	if _, ok := g.letters.Get(l.ID); !ok {
		t.Fatal("an answer not offered is ignored")
	}
	g.answerLetter(l.ID, 1)
	if len(g.pawnIDs) != before || len(g.incidents.Queue) != 0 {
		t.Fatalf("refusing changes nothing: %d colonists, queue %v", len(g.pawnIDs), g.incidents.Queue)
	}
	if _, ok := g.letters.Get(l.ID); ok {
		t.Fatal("the answered letter leaves the stack")
	}
	// the plea lapses by itself as a refusal
	g2, l2 := refugeeGame(t)
	g2.Step(clock.TicksPerDay + 2*clock.TicksPerHour)
	if _, ok := g2.letters.Get(l2.ID); ok {
		t.Fatal("the plea lapses after a day")
	}
}
