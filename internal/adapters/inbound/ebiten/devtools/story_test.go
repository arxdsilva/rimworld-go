package devtools

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/incident"
)

type storyGame struct{ v game.StoryDebugView }

func (s storyGame) StoryDebug() game.StoryDebugView { return s.v }

// TestStoryPanelLists (EAI-034): the story panel lists the threat points,
// the wealth, the next threat's tick and the queue; the debug menu toggles
// it.
func TestStoryPanelLists(t *testing.T) {
	t.Parallel()
	f := &fakeShell{layout: devLayouts()["idle 1280x720"]}
	f.env = Env{On: true, Game: storyGame{game.StoryDebugView{
		Queue:      []incident.Queued{{ID: incident.RivalRaid, At: 90, Points: 240}, {ID: incident.RefugeeChased, At: 400}},
		NextThreat: 1000, Points: 180, Wealth: 5300,
	}}}
	p := New(f)
	var toggle *Action
	acts := p.Actions(1, 1)
	for i := range acts {
		if strings.HasPrefix(acts[i].Label, "Story panel") {
			toggle = &acts[i]
		}
	}
	if toggle == nil {
		t.Fatal("the debug menu has a story panel entry")
	}
	if !p.Stack().Story.Empty() {
		t.Fatal("hidden until toggled")
	}
	toggle.Run(nil)
	if p.Stack().Story.Empty() {
		t.Fatal("shown once toggled")
	}
	joined := strings.Join(p.storyLines(), "\n")
	for _, want := range []string{"threat points 180  wealth 5300", "next threat: t1000", "queue: 2", "t90  rival_raid  240 pts", "t400  refugee_chased"} {
		if !strings.Contains(joined, want) {
			t.Errorf("story panel lacks %q:\n%s", want, joined)
		}
	}
	if !strings.Contains(p.Actions(1, 1)[len(acts)-2].Label, "hide") {
		t.Error("the entry reads hide while the panel is up")
	}
}
