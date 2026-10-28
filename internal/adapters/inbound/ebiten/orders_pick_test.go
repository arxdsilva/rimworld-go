package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/orders"
	"testing"

	"rimworld-go/internal/application/game"
)

// The float menu as a command's options list (PLAY-018): entries carry
// labels and reasons only, a pick callback runs the chosen one.

func pickEntries() []game.OrderOption {
	return []game.OrderOption{{Label: "Ada"}, {Label: "Bo", Disabled: true, Reason: "asleep"}, {Label: "Cy"}}
}

func TestFloatPickCallsBackAndCloses(t *testing.T) {
	t.Parallel()
	a, _, _ := inputColony(t, [2]int{3, 3})
	picked := -1
	a.openPick(300, 300, false, pickEntries(), func(i int) { picked = i })
	if !a.ord.Float.Open {
		t.Fatal("menu not open")
	}
	click(t, a, "float.2")
	if picked != 2 || a.ord.Float.Open {
		t.Fatalf("picked=%d open=%v", picked, a.ord.Float.Open)
	}
}

func TestFloatPickDisabledEntryShowsItsReason(t *testing.T) {
	t.Parallel()
	a, _, _ := inputColony(t, [2]int{3, 3})
	picked := -1
	a.openPick(300, 300, false, pickEntries(), func(i int) { picked = i })
	click(t, a, "float.1")
	if picked != -1 || !a.ord.Float.Open || a.hint != "asleep" {
		t.Fatalf("picked=%d open=%v hint=%q", picked, a.ord.Float.Open, a.hint)
	}
}

func TestFloatPickHasNoQueueFooter(t *testing.T) {
	t.Parallel()
	a, _, _ := inputColony(t, [2]int{3, 3})
	a.openPick(300, 300, false, pickEntries(), func(int) {})
	if r := a.floatRect(); r.H != float32(3*floatRowH) {
		t.Fatalf("a pick menu is only its rows (no Shift-queue footer): h=%v", r.H)
	}
	a.closeFloat()
	a.ord.Float = orders.Float{Open: true, X: 300, Y: 300, Opts: pickEntries()}
	if r := a.floatRect(); r.H != float32(3*floatRowH+floatHintH) {
		t.Fatalf("the order menu keeps its footer: h=%v", r.H)
	}
}

func TestFloatOpensUpwardFromTheAnchor(t *testing.T) {
	t.Parallel()
	a, _, _ := inputColony(t, [2]int{3, 3})
	a.openPick(300, 400, true, pickEntries(), func(int) {})
	r := a.floatRect()
	if r.Y+r.H != 400 {
		t.Fatalf("an upward menu ends at its anchor: %+v", r)
	}
}

func TestFloatStaysAboveTheTabStrip(t *testing.T) {
	t.Parallel()
	a, _, _ := inputColony(t, [2]int{3, 3})
	tabs := a.hud().Tabs
	for _, up := range []bool{false, true} {
		a.openPick(300, tabs.Y-4, up, pickEntries(), func(int) {})
		if up {
			a.ord.Float.Y = tabs.Y + 100 // an anchor below the strip
		}
		if r := a.floatRect(); r.Y+r.H > tabs.Y {
			t.Fatalf("up=%v: menu %+v overlaps the tab strip at y=%v", up, r, tabs.Y)
		}
	}
}
