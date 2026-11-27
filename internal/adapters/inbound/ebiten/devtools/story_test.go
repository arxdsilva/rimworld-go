// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package devtools

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
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

// TestStoryPanelLastThreat: the story panel's last-threat line names the
// event, its day, its point minimum and its spend, or says none.
func TestStoryPanelLastThreat(t *testing.T) {
	t.Parallel()
	lines := func(v game.StoryDebugView) []string {
		f := &fakeShell{layout: devLayouts()["idle 1280x720"]}
		f.env = Env{On: true, Game: storyGame{v}}
		return New(f).storyLines()
	}
	if got := lines(game.StoryDebugView{}); got[3] != "last threat: none" {
		t.Errorf("no threat yet: %q", got)
	}
	d := incident.DefOf(incident.PredatorPack)
	got := lines(game.StoryDebugView{LastThreat: d, LastThreatAt: 3*clock.TicksPerDay + 17})
	want := fmt.Sprintf("last threat: %s day 3 · min %d pts · spends %d%%", d.Name, d.MinPoints, max(d.PointsPct, 100))
	if got[3] != want {
		t.Errorf("last threat line %q, want %q", got[3], want)
	}
	half := incident.Def{Key: "half_pack", Name: "Half pack", MinPoints: 300, PointsPct: 50}
	if got := lines(game.StoryDebugView{LastThreat: half, LastThreatAt: 5 * clock.TicksPerDay}); got[3] != "last threat: Half pack day 5 · min 300 pts · spends 50%" {
		t.Errorf("a set share shows as is: %q", got[3])
	}
	long := make([]incident.Queued, maxStoryRows+3)
	if n := len(lines(game.StoryDebugView{Queue: long})); n != 4+1+maxStoryRows+1 {
		t.Errorf("queue rows stay capped: %d lines", n)
	}
}
