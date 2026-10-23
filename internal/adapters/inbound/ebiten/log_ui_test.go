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

func TestRecentShowsFourNewest(t *testing.T) {
	t.Parallel()
	rows := recentLogRows(logs(7), 300)
	if len(rows) != 4 || !strings.Contains(rows[0], "event A") || !strings.Contains(rows[3], "event D") {
		t.Fatalf("rows = %q", rows)
	}
}

func TestLogTabButtonAndRendering(t *testing.T) {
	a, _, id := liveColony(t, 0)
	selectPawn(t, a, id)
	if _, ok := findButton(a, "inspector.log"); !ok {
		t.Fatal("no Log tab")
	}
	r := inspectorRect()
	for _, b := range a.inspectorButtons() {
		if b.id != "pawn.draft" && b.id != "pawn.clearorders" && b.id != "inspector.close" && b.x+b.w > r.x+r.w {
			t.Fatalf("tab %s overflows the panel", b.id)
		}
	}
	click(t, a, "inspector.log")
	if a.inspectTab != "log" {
		t.Fatalf("tab = %q", a.inspectTab)
	}
	p, _ := a.selectedPawn()
	p.Log = logs(40)
	// drawing (pixels cannot be read in tests) must not panic, with or
	// without lines
	img := ebiten.NewImage(640, 480)
	a.drawPawnLog(img, p)
	empty := p
	empty.Log = nil
	a.drawPawnLog(img, empty)
	if rows := wrapLogRows(empty.Log, 300); len(rows) != 0 {
		t.Fatal("no lines, no rows (the empty message is drawn instead)")
	}
	// the wheel scrolls inside the panel, and not outside it
	x, y, _, _ := logPanelRect()
	if !a.scrollLogAt(x+5, y+5, -1) || a.logScroll == 0 {
		t.Fatalf("wheel over the log must scroll it (scroll %d)", a.logScroll)
	}
	if a.scrollLogAt(600, 460, -1) {
		t.Fatal("wheel elsewhere must not be taken")
	}
	a.logFor = 0
	a.syncLogFor(p.ID)
	if a.logScroll != 0 {
		t.Fatal("scroll resets on selection change")
	}
}

func TestAnimalLogToggle(t *testing.T) {
	a, g, _ := liveColony(t, 0)
	if _, err := g.SpawnAnimal("dustbuck", 10, 10); err != nil {
		t.Fatal(err)
	}
	a.selectAnimalAt(10, 10)
	h0 := a.animalPanel().h
	click(t, a, "animal.log")
	if !a.animalLogOpen || a.animalPanel().h <= h0 {
		t.Fatal("Log expands the panel")
	}
	screen := ebiten.NewImage(640, 480)
	a.drawAnimalInspector(screen)
	click(t, a, "animal.log")
	if a.animalLogOpen {
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
	if a.float.open {
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
	if !a.float.open {
		t.Fatal("a drafted right-click on an animal must open the order menu")
	}
}
