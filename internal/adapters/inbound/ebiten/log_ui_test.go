package ebiten

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func logs(n int) []game.LogLineView {
	var out []game.LogLineView
	for i := 0; i < n; i++ { // newest first
		out = append(out, game.LogLineView{Day: 3, Hour: 20 - i, Text: "event " + string(rune('A'+i))})
	}
	return out
}

func TestLogTabButtonAndRendering(t *testing.T) {
	a, _, id := liveColony(t, 0)
	selectPawn(t, a, id)
	tab, ok := paneTabByID(a.insp.Pawn().PawnPane(id), "inspector.log")
	if !ok || tab.Draw == nil {
		t.Fatal("no Log tab")
	}
	r := innerFor(1280, 720, tab)
	if r.W < hudlayout.ExpandMinW-2*inspector.PaneInset || r.W > hudlayout.ExpandMaxW {
		t.Fatalf("Log panel %+v", r)
	}
	// the tab sits inside the pane's tab strip and the panel inside the screen
	if r.X < 0 || r.Right() > 1280 || r.Y < 0 || r.Bottom() > 720 {
		t.Fatalf("Log panel %+v is off the screen", r)
	}
	a.insp.Tab = "log"
	p, _ := a.insp.SelectedPawn()
	p.Log = logs(40)
	// drawing (pixels cannot be read in tests) must not panic, with or
	// without lines
	img := ebiten.NewImage(1280, 720)
	a.insp.Pawn().DrawPawnLogRows(img, p, r)
	empty := p
	empty.Log = nil
	a.insp.Pawn().DrawPawnLogRows(img, empty, r)
	tab.Draw(img, r)
	if rows := inspector.WrapLogRows(empty.Log, 300); len(rows) != 0 {
		t.Fatal("no lines, no rows (the empty message is drawn instead)")
	}
	// the wheel scrolls inside the panel, and not outside it
	x, y, _, _ := inspector.LogBodyRect(r)
	if !a.insp.Pawn().ScrollLogIn(r, x+5, y+5, -1) || a.insp.LogScroll == 0 {
		t.Fatalf("wheel over the log must scroll it (scroll %d)", a.insp.LogScroll)
	}
	if a.insp.Pawn().ScrollLogIn(r, 1270, 710, -1) {
		t.Fatal("wheel elsewhere must not be taken")
	}
	a.insp.LogFor = 0
	a.insp.Pawn().SyncLogFor(p.ID)
	if a.insp.LogScroll != 0 {
		t.Fatal("scroll resets on selection change")
	}
}

// The animal's Log is a pane tab (PLAY-018): it opens the expanded panel
// above the pane and a second click folds it again.
func TestAnimalLogToggle(t *testing.T) {
	a, g, _ := liveColony(t, 0)
	if _, err := g.SpawnAnimal("dustbuck", 10, 10); err != nil {
		t.Fatal(err)
	}
	if !a.insp.Animal().SelectAnimalAt(10, 10) {
		t.Fatal("animal not selectable")
	}
	if !a.hud().Expanded.Empty() {
		t.Fatal("the pane starts folded")
	}
	click(t, a, "inspector.log")
	l := a.hud()
	if a.insp.PaneOpenTab() != "inspector.log" || l.Expanded.Empty() || l.Expanded.Bottom() != l.PaneTabs.Y {
		t.Fatalf("Log expands the panel above the pane: open %q, expanded %+v", a.insp.PaneOpenTab(), l.Expanded)
	}
	screen := ebiten.NewImage(1280, 720)
	a.insp.DrawPane(screen)
	click(t, a, "inspector.log")
	if a.insp.PaneOpenTab() != "" || !a.hud().Expanded.Empty() {
		t.Fatal("toggles back")
	}
}

func TestDraftedRightClick(t *testing.T) {
	a, g, id := liveColony(t, 0)
	selectPawn(t, a, id)
	a.renderer.enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	if p, _ := a.insp.SelectedPawn(); !p.Drafted {
		t.Fatal("not drafted")
	}
	// open ground: moves at once, no menu
	a.ord.RightClickTile(8, 8, 100, 100)
	if a.ord.Float.Open {
		t.Fatal("ground click must not open the menu")
	}
	g.Step(1)
	if p, _ := a.insp.SelectedPawn(); !p.HasGoal || p.GoalX != 8 || p.GoalY != 8 {
		t.Fatalf("drafted pawn must head for the ground click: %+v", p)
	}
	// an animal tile opens the order menu instead
	if _, err := g.SpawnAnimal("dustbuck", 5, 5); err != nil {
		t.Fatal(err)
	}
	a.renderer.syncPawns()
	a.ord.RightClickTile(5, 5, 100, 100)
	if !a.ord.Float.Open {
		t.Fatal("a drafted right-click on an animal must open the order menu")
	}
}
