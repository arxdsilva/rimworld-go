package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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

func TestLogLineTextAndWrap(t *testing.T) {
	t.Parallel()
	if got := logLineText(game.LogLineView{Day: 2, Hour: 7, Text: "Ate a meal"}); got != "Day 2, 07h  Ate a meal" {
		t.Fatalf("text = %q", got)
	}
	rows := wrapLogRows([]game.LogLineView{{Day: 1, Hour: 1, Text: strings.Repeat("x", 60)}}, 200)
	if w, _ := measure(rows[0], logStyle); len(rows) < 2 || !strings.HasPrefix(rows[1], "  ") || w > 200 {
		t.Fatalf("rows = %q", rows)
	}
	if got := wrapLogRows(logs(3), 300); len(got) != 3 || !strings.Contains(got[0], "event A") {
		t.Fatalf("newest first: %q", got)
	}
	if clampLogScroll(99, 10, 4) != 6 || clampLogScroll(-1, 10, 4) != 0 {
		t.Fatal("scroll clamp")
	}
}

func TestLogTabButtonAndRendering(t *testing.T) {
	a, _, id := liveColony(t, 0)
	selectPawn(t, a, id)
	tab, ok := paneTabByID(a.pawnPane(id), "inspector.log")
	if !ok || tab.Draw == nil {
		t.Fatal("no Log tab")
	}
	r := innerFor(1280, 720, tab)
	if r.W < expandMinW-2*paneInset || r.W > expandMaxW {
		t.Fatalf("Log panel %+v", r)
	}
	// the tab sits inside the pane's tab strip and the panel inside the screen
	if r.X < 0 || r.Right() > 1280 || r.Y < 0 || r.Bottom() > 720 {
		t.Fatalf("Log panel %+v is off the screen", r)
	}
	a.inspectTab = "log"
	p, _ := a.selectedPawn()
	p.Log = logs(40)
	// drawing (pixels cannot be read in tests) must not panic, with or
	// without lines
	img := ebiten.NewImage(1280, 720)
	a.drawPawnLogRows(img, p, r)
	empty := p
	empty.Log = nil
	a.drawPawnLogRows(img, empty, r)
	tab.Draw(img, r)
	if rows := wrapLogRows(empty.Log, 300); len(rows) != 0 {
		t.Fatal("no lines, no rows (the empty message is drawn instead)")
	}
	// the wheel scrolls inside the panel, and not outside it
	x, y, _, _ := logBodyRect(r)
	if !a.scrollLogIn(r, x+5, y+5, -1) || a.logScroll == 0 {
		t.Fatalf("wheel over the log must scroll it (scroll %d)", a.logScroll)
	}
	if a.scrollLogIn(r, 1270, 710, -1) {
		t.Fatal("wheel elsewhere must not be taken")
	}
	a.logFor = 0
	a.syncLogFor(p.ID)
	if a.logScroll != 0 {
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
	if !a.selectAnimalAt(10, 10) {
		t.Fatal("animal not selectable")
	}
	if !a.hud().Expanded.Empty() {
		t.Fatal("the pane starts folded")
	}
	click(t, a, "inspector.log")
	l := a.hud()
	if a.paneOpenTab() != "inspector.log" || l.Expanded.Empty() || l.Expanded.Bottom() != l.PaneTabs.Y {
		t.Fatalf("Log expands the panel above the pane: open %q, expanded %+v", a.paneOpenTab(), l.Expanded)
	}
	screen := ebiten.NewImage(1280, 720)
	a.drawPane(screen)
	click(t, a, "inspector.log")
	if a.paneOpenTab() != "" || !a.hud().Expanded.Empty() {
		t.Fatal("toggles back")
	}
}

func TestDraftedRightClick(t *testing.T) {
	a, g, id := liveColony(t, 0)
	selectPawn(t, a, id)
	a.renderer.enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	if p, _ := a.selectedPawn(); !p.Drafted {
		t.Fatal("not drafted")
	}
	// open ground: moves at once, no menu
	a.rightClickTile(8, 8, 100, 100)
	if a.ord.Float.Open {
		t.Fatal("ground click must not open the menu")
	}
	g.Step(1)
	if p, _ := a.selectedPawn(); !p.HasGoal || p.GoalX != 8 || p.GoalY != 8 {
		t.Fatalf("drafted pawn must head for the ground click: %+v", p)
	}
	// an animal tile opens the order menu instead
	if _, err := g.SpawnAnimal("dustbuck", 5, 5); err != nil {
		t.Fatal(err)
	}
	a.renderer.syncPawns()
	a.rightClickTile(5, 5, 100, 100)
	if !a.ord.Float.Open {
		t.Fatal("a drafted right-click on an animal must open the order menu")
	}
}
