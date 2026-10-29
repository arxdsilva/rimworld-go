package inspector

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestPrisonerCommandsEmitCaptureAndArrest(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.hostiles = []game.PawnView{{ID: 9, Name: "Raider", Status: "downed", X: 3, Y: 3}, {ID: 8, Name: "Far", Status: "downed", X: 30, Y: 30}}
	btns := New(f).Tabs().prisonerCommands(game.PawnView{ID: 1, Name: "Ann", Status: "alive", X: 2, Y: 2})
	if len(btns) != 2 || btns[0].Disabled {
		t.Fatalf("buttons %+v", btns)
	}
	if want := (command.Command{Kind: command.Capture, Pawn: 1, Target: 9}); !slices.Equal(btns[0].Cmds, []command.Command{want}) {
		t.Fatalf("capture cmds %+v", btns[0].Cmds)
	}
	if want := (command.Command{Kind: command.Arrest, Target: 1}); !slices.Equal(btns[1].Cmds, []command.Command{want}) {
		t.Fatalf("arrest cmds %+v", btns[1].Cmds)
	}
}

func TestPrisonerCommandsDisabledAndHidden(t *testing.T) {
	t.Parallel()
	tabs := New(newFake()).Tabs()
	btns := tabs.prisonerCommands(game.PawnView{ID: 1, Status: "alive"})
	if len(btns) != 2 || !btns[0].Disabled || btns[0].Reason == "" || len(btns[0].Cmds) != 0 {
		t.Fatalf("no downed enemy disables Capture: %+v", btns[0])
	}
	for _, p := range []game.PawnView{{Prisoner: true, Status: "alive"}, {Hostile: true}, {Embodied: true}, {Status: "dead"}} {
		if tabs.prisonerCommands(p) != nil {
			t.Errorf("%+v gets no buttons", p)
		}
	}
}

func TestPawnLinesPrintsQueuedOrderLabels(t *testing.T) {
	t.Parallel()
	p := game.PawnView{Status: "alive", QueuedOrders: []game.QueuedOrderView{{Label: "Rescue Bo"}, {Label: "Arrest Cy"}}}
	lines := New(newFake()).Pawn().PawnLines(p)
	for _, want := range []string{"1. Rescue Bo", "2. Arrest Cy"} {
		if !slices.Contains(lines, want) {
			t.Errorf("missing %q in %v", want, lines)
		}
	}
}
