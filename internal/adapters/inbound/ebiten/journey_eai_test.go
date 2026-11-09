package ebiten

import (
	"fmt"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/command"
)

// TestJourneyEAILetterChoice (EAI-031): a refugee's plea arrives as a letter
// with two answers; opening it and pressing "Take them in" sends
// AnswerLetter 0, the refugee joins and their pursuers are queued.
func TestJourneyEAILetterChoice(t *testing.T) {
	t.Parallel()
	g := ax4RiverColony(t)
	g.Step(4000)
	if !g.FireIncident("refugee_chased") {
		t.Fatal("the plea did not fit this colony")
	}
	var plea game.LetterView
	for _, l := range g.World().Letters {
		if len(l.Choices) > 0 {
			plea = l
		}
	}
	if plea.ID == 0 || len(plea.Choices) != 2 || plea.Choices[0].Key != letters.Accept {
		t.Fatalf("no plea with two answers: %+v", g.World().Letters)
	}
	before := colonistCount(g)
	j := newJourney(t, jGame(g))
	j.update(2)
	if !j.clickID(fmt.Sprintf("letter.%d", plea.ID)) {
		t.Fatal("the plea is not in the stack")
	}
	j.update(2)
	if !j.clickID(fmt.Sprintf("modal.letter.%d.choice.accept", plea.ID)) {
		t.Fatalf("no Take them in button; top modal %q", j.a.uiModalTop())
	}
	var sent *command.Command
	for i := range j.cmds {
		if j.cmds[i].Kind == command.AnswerLetter {
			sent = &j.cmds[i]
		}
	}
	if sent == nil || sent.Target != plea.ID || sent.Mode != 0 {
		t.Fatalf("commands: %+v", j.cmds)
	}
	g.Step(2)
	if got := colonistCount(g); got != before+1 {
		t.Fatalf("the refugee joins: %d colonists, were %d", got, before)
	}
	q := g.StoryDebug().Queue
	if len(q) != 1 || q[0].ID != incident.RivalRaid {
		t.Fatalf("the pursuers are queued: %+v", q)
	}
	for _, l := range g.World().Letters {
		if l.ID == plea.ID {
			t.Fatal("the answered plea is still in the stack")
		}
	}
}

// colonistCount is the colonists the read model lists (no raiders).
func colonistCount(g *game.Game) int {
	n := 0
	for _, p := range g.Snapshot() {
		if !p.Hostile {
			n++
		}
	}
	return n
}
