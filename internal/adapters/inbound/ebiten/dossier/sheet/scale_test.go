package sheet

import (
	"testing"

	"rimworld-go/internal/platform/config"
)

// The docked card at each screen height: the table the plan pins, then every
// height from 564 to 1440, where the pane and its tabs never pass 30 % of the
// screen and the built-in tree fits every target.
func TestDockedForTable(t *testing.T) {
	t.Parallel()
	d := config.Defaults().HUD
	for _, c := range []struct{ sh, font, pane int }{
		{564, 9, 139}, {720, 12, 165}, {1080, 18, 247}, {1440, 24, 330},
	} {
		got := DockedFor(d, c.sh)
		if got.Text.BodySize != c.font || got.Pane.Height != c.pane {
			t.Errorf("%d tall: text %d pane %d, want %d and %d", c.sh, got.Text.BodySize, got.Pane.Height, c.font, c.pane)
		}
	}
	for sh := 564; sh <= 1440; sh++ {
		got := DockedFor(d, sh)
		if (got.Pane.Height+DockTabsH)*100 > sh*DockSharePct {
			t.Fatalf("%d tall: pane %d and tabs take more than %d %%", sh, got.Pane.Height, DockSharePct)
		}
		if got.Text.BodySize < DockMinFont {
			t.Fatalf("%d tall: text %d", sh, got.Text.BodySize)
		}
		fitsEveryTarget(t, sh, DefaultDocked(), got)
	}
}

// Under 564 (a high manual UI scale in a small window) the smallest readable
// card wins over the 30 %; nothing is cut.
func TestDockedForTinyScreens(t *testing.T) {
	t.Parallel()
	d := config.Defaults().HUD
	for _, sh := range []int{360, 480, 563} {
		got := DockedFor(d, sh)
		if got.Text.BodySize != DockMinFont || got.Pane.Height != 139 {
			t.Errorf("%d tall: text %d pane %d, want %d and 139", sh, got.Text.BodySize, got.Pane.Height, DockMinFont)
		}
		fitsEveryTarget(t, sh, DefaultDocked(), got)
	}
}

// The widths follow the text, the rest is untouched, and the call allocates
// nothing (it runs every frame through the pane). Not parallel: AllocsPerRun.
func TestDockedForCopiesValuesOnly(t *testing.T) {
	d := config.Defaults().HUD
	d.Colors = map[string]string{"ok": "#01020304"}
	got := DockedFor(d, 1080)
	if got.Pane.TabW != d.Pane.TabW*3/2 || got.Text.LabelW != d.Text.LabelW*3/2 || got.Text.ValueW != d.Text.ValueW*3/2 {
		t.Errorf("widths at 1.5x: tab %d label %d value %d", got.Pane.TabW, got.Text.LabelW, got.Text.ValueW)
	}
	if got.Pane.MinTabs != d.Pane.MinTabs || got.Card != d.Card || got.Text.BodyLinePct != d.Text.BodyLinePct || len(got.Colors) != 1 {
		t.Errorf("more than the sizes changed: %+v", got)
	}
	if d.Pane.Height != 165 || d.Text.BodySize != 12 {
		t.Error("the input changed")
	}
	if same := DockedFor(d, 720); same.Pane != d.Pane || same.Text != d.Text {
		t.Errorf("720 is the reference: %+v %+v", same.Pane, same.Text)
	}
	if none := DockedFor(d, 0); none.Pane != d.Pane || none.Text != d.Text {
		t.Error("no screen yet keeps the settings")
	}
	tree := DefaultDocked()
	d.Layouts.Docked = &tree
	if n := testing.AllocsPerRun(50, func() { _ = DockedFor(d, 600) }); n != 0 {
		t.Errorf("%v allocations a call", n)
	}
}

// A player's tree taller than the built-in one gets the largest text that
// lets it fit, down to the floor; the pane does not grow for it.
func TestDockedForShrinksTextForAPlayersTree(t *testing.T) {
	t.Parallel()
	d := config.Defaults().HUD
	tall := withTraits(DefaultDocked()) // one row more
	d.Layouts.Docked = &tall
	plain := DockedFor(config.Defaults().HUD, 640)
	got := DockedFor(d, 640)
	if got.Pane.Height != plain.Pane.Height {
		t.Fatalf("the pane grew for the tree: %d, built-in %d", got.Pane.Height, plain.Pane.Height)
	}
	if got.Text.BodySize >= plain.Text.BodySize {
		t.Fatalf("text %d, built-in %d: one row more needs smaller text", got.Text.BodySize, plain.Text.BodySize)
	}
	fitsEveryTarget(t, 640, tall, got)
}

// The constant row count DockedFor uses for the built-in tree is the tree's.
func TestDockRowsMatchTheDefault(t *testing.T) {
	t.Parallel()
	text := config.Defaults().HUD.Text
	for size := 7; size <= 30; size++ {
		text.BodySize = size
		m := MetricsFor(text)
		want := dockLines*m.BodyH + dockRules*m.GapH
		var tallest float32
		for w := WhoColonist; w <= WhoWild; w++ {
			tallest = max(tallest, Measure(Default(true, w), m, 0))
		}
		if tallest != want || Tallest(DefaultDocked(), m) != want {
			t.Fatalf("size %d: the built-in tree is %v (Tallest %v), DockedFor counts %v", size, tallest, Tallest(DefaultDocked(), m), want)
		}
	}
}

// Tallest agrees with measuring each pruned tree, for every built-in tree.
func TestTallestIsThePrunedMeasure(t *testing.T) {
	t.Parallel()
	m := MetricsFor(config.Defaults().HUD.Text)
	for _, p := range BuiltIns() {
		for _, n := range []config.HUDNode{*p.Docked, *p.Floating} {
			var want float32
			for w := WhoColonist; w <= WhoWild; w++ {
				want = max(want, Measure(Prune(n, w), m, 0))
			}
			if got := Tallest(n, m); got != want {
				t.Errorf("%s: Tallest %v, pruned %v", p.Name, got, want)
			}
		}
	}
}

func fitsEveryTarget(t *testing.T, sh int, tree config.HUDNode, h config.HUD) {
	t.Helper()
	m := MetricsFor(h.Text)
	room := float32(h.Pane.Height - DockMargin)
	for w := WhoColonist; w <= WhoWild; w++ {
		if got := Measure(Prune(tree, w), m, 0); got > room {
			t.Fatalf("%d tall, target %c: the tree is %v in %v (text %d)", sh, w.Letter(), got, room, h.Text.BodySize)
		}
	}
}

// Each built-in, loaded, fits every target on every screen from 564 to 1440
// tall, the pane within 30 % wherever its smallest readable text allows. Under
// that (Roomy's taller lines need 617, Snug with traits' ninth row 607) the
// smallest text wins over the 30 % for the built-in rows, and a ninth row past
// them is hidden, with the card's warning.
func TestEachBuiltInFitsEveryScreen(t *testing.T) {
	t.Parallel()
	for _, p := range BuiltIns() {
		h := config.Defaults().HUD
		Apply(p, &h)
		tree := *p.Docked
		for sh := 564; sh <= 1440; sh++ {
			got := DockedFor(h, sh)
			share := sh*DockSharePct/100 - DockTabsH
			if dockNeed(&tree, DockMinFont, h.Text) <= share {
				fitsEveryTarget(t, sh, tree, got)
			}
			if dockNeed(nil, DockMinFont, h.Text) <= share && got.Pane.Height > share {
				t.Fatalf("%s at %d tall: pane %d, more than 30 %%", p.Name, sh, got.Pane.Height)
			}
		}
		for _, sh := range []int{617, 720, 1080} {
			fitsEveryTarget(t, sh, tree, DockedFor(h, sh))
		}
	}
}
