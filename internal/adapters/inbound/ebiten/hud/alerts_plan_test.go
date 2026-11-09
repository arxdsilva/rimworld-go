package hud

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/domain/sim/command"
)

// TestRaidAlertNamesPlan (EAI-016): the raid alert names the stage, steal and
// burn plans as it does siege and sappers.
func TestRaidAlertNamesPlan(t *testing.T) {
	t.Parallel()
	for plan, want := range map[string]string{
		"assault":        "Raid: 2 raiders",
		"siege":          "Siege: 2 raiders",
		"sappers":        "Sappers: 2 raiders",
		"staged assault": "Staging: 2 raiders",
		"steal":          "Thieves: 2 raiders",
		"burn":           "Arsonists: 2 raiders",
	} {
		pawns := []game.PawnView{
			{Name: "R1", Hostile: true, RaidPlan: plan, X: 1, Y: 1},
			{Name: "R2", Hostile: true, RaidPlan: plan, X: 2, Y: 2},
		}
		var got string
		for _, a := range BuildAlerts(pawns, game.WorldView{}) {
			if a.Key == "raid" {
				got = a.Label
			}
		}
		if got != want {
			t.Errorf("plan %q: alert %q, want %q", plan, got, want)
		}
	}
}

// TestLetterChoiceButtons (EAI-031): a letter with choices shows them in the
// footer, each sending AnswerLetter with its index.
func TestLetterChoiceButtons(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	l := game.LetterView{ID: 9, X: -1, Y: -1, Choices: letters.ShelterChoices()}
	got := c.Right().letterFooter(l)
	if len(got) != 2 || got[0].Label != "Take them in" || got[1].Label != "Send them away" {
		t.Fatalf("the footer: %+v", got)
	}
	got[1].OnClick()
	got[0].OnClick()
	if len(f.cmds) != 2 || f.cmds[0].Kind != command.AnswerLetter || f.cmds[0].Target != 9 || f.cmds[0].Mode != 1 || f.cmds[1].Mode != 0 {
		t.Fatalf("commands: %+v", f.cmds)
	}
}

// TestMessageShowsRepeat (EAI-032): a merged notice reads "×N".
func TestMessageShowsRepeat(t *testing.T) {
	t.Parallel()
	m := game.MessageView{Text: "The stove is cold."}
	if got := MessageLine(m); got != "The stove is cold." {
		t.Fatalf("plain: %q", got)
	}
	m.Repeat = 2
	if got := MessageLine(m); got != "The stove is cold. ×3" {
		t.Fatalf("merged: %q", got)
	}
}

// TestSparkingTurretAlert (MAP-21): a danger alert names a turret with a lit
// fuse and is gone once the fuse ends.
func TestSparkingTurretAlert(t *testing.T) {
	t.Parallel()
	find := func(w game.WorldView) *Alert {
		for _, a := range BuildAlerts(nil, w) {
			if a.Key == "sparking" {
				return &a
			}
		}
		return nil
	}
	w := game.WorldView{Buildings: []game.BuildingView{{Kind: "turret", X: 4, Y: 5, Sparking: true}}}
	a := find(w)
	if a == nil || !a.Danger || a.Label != "Turret about to explode" || len(a.Targets) != 1 || a.Targets[0] != [2]int{4, 5} {
		t.Fatalf("alert %+v", a)
	}
	w.Buildings = append(w.Buildings, game.BuildingView{Kind: "turret", X: 9, Y: 9, Sparking: true})
	if a := find(w); a == nil || a.Label != "2 turrets about to explode" || len(a.Targets) != 2 {
		t.Fatalf("two: %+v", a)
	}
	w.Buildings[0].Sparking, w.Buildings[1].Sparking = false, false
	if find(w) != nil {
		t.Fatal("the alert stays after the fuse ends")
	}
}
