package hud

import (
	"image/color"
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

func TestBarStatuses(t *testing.T) {
	t.Parallel()
	alive := game.PawnView{Status: "alive"}
	with := func(f func(p *game.PawnView)) game.PawnView {
		p := alive
		f(&p)
		return p
	}
	for _, tc := range []struct {
		name   string
		p      game.PawnView
		onFire bool
		idle   bool
		want   []string
	}{
		{"nothing", alive, false, false, nil},
		{"drafted", with(func(p *game.PawnView) { p.Drafted = true }), false, false, []string{"drafted"}},
		{"asleep", with(func(p *game.PawnView) { p.Lying = true }), false, false, []string{"sleeping"}},
		{"mental break", with(func(p *game.PawnView) { p.MentalBreak = "wander" }), false, false, []string{"mental"}},
		{"downed", with(func(p *game.PawnView) { p.Status, p.Lying = "downed", true }), false, false, []string{"downed"}},
		{"idle", alive, false, true, []string{"idle"}},
		{"fighting (melee)", with(func(p *game.PawnView) { p.Action = game.ActionMelee }), false, false, []string{"fighting"}},
		{"fighting (aim)", with(func(p *game.PawnView) { p.Action = game.ActionAim }), false, false, []string{"fighting"}},
		{"fleeing", with(func(p *game.PawnView) { p.Fleeing = true }), false, false, []string{"fleeing"}},
		{"burning", alive, true, false, []string{"burning"}},
		{"drafted and fighting", with(func(p *game.PawnView) { p.Drafted, p.Action = true, game.ActionAim }), false, false, []string{"fighting", "drafted"}},
		{"downed and burning", with(func(p *game.PawnView) { p.Status, p.Lying = "downed", true }), true, false, []string{"downed", "burning"}},
		{"broken, fleeing, drafted", with(func(p *game.PawnView) { p.MentalBreak, p.Fleeing, p.Drafted = "x", true, true }), false, false, []string{"mental", "fleeing", "drafted"}},
		// a downed colonist is not "sleeping" and not "idle"
		{"downed is not asleep or idle", with(func(p *game.PawnView) { p.Status, p.Lying = "downed", true }), false, true, []string{"downed"}},
	} {
		var got []string
		for _, s := range BarStatuses(tc.p, tc.onFire, tc.idle) {
			got = append(got, s.Key)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: statuses %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestBarMoodTokens(t *testing.T) {
	t.Parallel()
	for pct, want := range map[int]color.RGBA{0: uikit.MoodBad, 29: uikit.MoodBad, 30: uikit.MoodMid, 54: uikit.MoodMid, 55: uikit.MoodGood, 100: uikit.MoodGood} {
		if got := MoodToken(pct); got != want {
			t.Errorf("moodToken(%d) = %v, want %v", pct, got, want)
		}
	}
}

// Negative temperatures round away from zero like the rest of the HUD.
func TestFmtDegrees(t *testing.T) {
	t.Parallel()
	for in, want := range map[int]string{0: "0 °C", 94: "9 °C", 95: "10 °C", 215: "22 °C", -4: "0 °C", -5: "-1 °C", -94: "-9 °C", -95: "-10 °C"} {
		if got := FmtDegrees(in); got != want {
			t.Errorf("FmtDegrees(%d) = %q, want %q", in, got, want)
		}
	}
}

// More than eight toggles wrap into rows of eight, right-aligned, upward
// from the bottom; the measured height grows by a row each time.
func TestLayoutGlobalWrapsToggleRows(t *testing.T) {
	t.Parallel()
	g := gfx.Rect{X: 1040, Y: 400, W: 232, H: 0}
	var prev float32
	for _, n := range []int{8, 9, 16, 17} {
		h := GlobalStackHeight(GlobalPlan{}, n)
		g.H = h
		g.Y = 692 - hudlayout.HudMargin - h
		rows := LayoutGlobal(g, GlobalPlan{}, n)
		if len(rows.Toggles) != n {
			t.Fatalf("n=%d: %d toggle rects", n, len(rows.Toggles))
		}
		for i, r := range rows.Toggles {
			if r.X < g.X || r.Right() > g.Right() || r.Y < g.Y || r.Bottom() > g.Bottom() {
				t.Errorf("n=%d toggle %d %+v outside %+v", n, i, r, g)
			}
			if i >= GlobalPerRow && r.Y <= rows.Toggles[i-GlobalPerRow].Y {
				t.Errorf("n=%d: toggle %d %+v should be below toggle %d %+v (rows run top to bottom)", n, i, r, i-GlobalPerRow, rows.Toggles[i-GlobalPerRow])
			}
		}
		if last := rows.Toggles[n-1]; g.Right()-GlobalPad != last.Right() {
			t.Errorf("n=%d: the last toggle ends at %v, want the inner right edge %v", n, last.Right(), g.Right()-GlobalPad)
		}
		if h < prev {
			t.Errorf("n=%d: height %v shrank from %v", n, h, prev)
		}
		prev = h
	}
	if h8, h9 := GlobalStackHeight(GlobalPlan{}, 8), GlobalStackHeight(GlobalPlan{}, 9); h9-h8 != GlobalIconPx+GlobalIconGap {
		t.Errorf("a second toggle row adds %v, want %v", h9-h8, GlobalIconPx+GlobalIconGap)
	}
}

func TestResourceIconKeys(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]string{
		"wood":           "ui_icon/wood",
		"granite blocks": "ui_icon/granite_blocks",
		"cloth carpet":   "ui_icon/cloth_carpet",
	} {
		if got := ResourceIconKey(kind); got != want {
			t.Errorf("resourceIconKey(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestStackSplit(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		total, rows, limit int
		shown, folded      int
	}{
		{0, 5, 8, 0, 0},
		{3, 0, 8, 0, 3},  // no room at all
		{3, 5, 8, 3, 0},  // everything fits
		{9, 9, 8, 8, 1},  // over the limit: the cap and a +N row
		{10, 4, 8, 3, 7}, // cramped: the count takes a row
		{5, 1, 5, 1, 0},  // one row has no room for a count
		{12, 20, 99, 12, 0},
	} {
		shown, folded := StackSplit(c.total, c.rows, c.limit)
		if shown != c.shown || folded != c.folded {
			t.Errorf("stackSplit(%d, %d, %d) = %d, %d; want %d, %d", c.total, c.rows, c.limit, shown, folded, c.shown, c.folded)
		}
	}
}
