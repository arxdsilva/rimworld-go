package sheet

import (
	"testing"

	"rimworld-go/internal/platform/config"
)

func TestDefaultsAreValid(t *testing.T) {
	t.Parallel()
	for name, n := range map[string]config.HUDNode{"docked": DefaultDocked(), "floating": DefaultFloating()} {
		if ps := Validate(n); len(ps) != 0 {
			t.Errorf("%s: %v", name, ps)
		}
		if nodes, depth := Count(n); nodes > config.HUDMaxNodes || depth > config.HUDMaxDepth {
			t.Errorf("%s: %d nodes depth %d", name, nodes, depth)
		}
	}
}

// The docked body fits the pane's default height with room for its header and
// tabs; the floating card fits its default height less an inset.
func TestDefaultLayoutsFitPaneH(t *testing.T) {
	t.Parallel()
	d := config.Defaults().HUD
	m := MetricsFor(d.Text)
	const dockedBody = 300 // what the pane leaves for the card at Height 360
	const inset = 16
	for w := WhoColonist; w <= WhoWild; w++ {
		if h := Measure(Default(true, w), m, 0); h > dockedBody || h <= 0 {
			t.Errorf("docked, target %c: %v tall, room %d", w.Letter(), h, dockedBody)
		}
		if h := Measure(Default(false, w), m, 0); h > float32(d.Card.H-inset) || h <= 0 {
			t.Errorf("floating, target %c: %v tall, room %d", w.Letter(), h, d.Card.H-inset)
		}
	}
	if float32(d.Pane.Height) < dockedBody+60 {
		t.Errorf("the default pane (%d) leaves less than 60 for header and tabs", d.Pane.Height)
	}
}

func TestDefaultTargetsShowTheRightRows(t *testing.T) {
	t.Parallel()
	has := func(n config.HUDNode, key string) bool {
		var found bool
		walk(n, func(x config.HUDNode) config.HUDNode {
			if x.Kind == key {
				found = true
			}
			return x
		})
		return found
	}
	col, str, tame, wild := Default(false, WhoColonist), Default(false, WhoStranger), Default(false, WhoPrisoner), Default(false, WhoWild)
	for _, k := range []string{"skill", "hour_pick", "fun_bar", "traits", "mood_bar"} {
		if !has(col, k) {
			t.Errorf("colonist lacks %s", k)
		}
	}
	if has(col, "train") || has(col, "keeper") || has(col, "captive") {
		t.Error("a colonist has no training, master or captive rows")
	}
	if has(str, "skill") || has(str, "hour_pick") || !has(str, "captive") || !has(str, "mood_bar") {
		t.Error("strangers: no skills, no schedule, a captive row, a mood bar")
	}
	if !has(tame, "train") || !has(tame, "keeper") || !has(tame, "area_pick") || has(tame, "mood_bar") || has(tame, "skill") {
		t.Error("a tame animal: training, master, area; no mood bar, no skills")
	}
	if has(wild, "train") || has(wild, "area_pick") || has(wild, "diet_pick") || !has(wild, "hunger_bar") || !has(wild, "doing") {
		t.Error("a wild animal: food and doing only")
	}
	if has(DefaultDocked(), "title") || !has(DefaultFloating(), "title") {
		t.Error("the pane header carries the name when docked; the floating card needs a title")
	}
}

// Every skill appears once, and no row of the skill block holds a lone entry.
func TestDefaultSkillsPairUp(t *testing.T) {
	t.Parallel()
	seen := map[string]int{}
	for _, row := range skillBlock().Kids {
		if len(row.Kids) != 2 {
			t.Errorf("row of %d", len(row.Kids))
		}
		for _, k := range row.Kids {
			if k.Kind == "skill" {
				seen[k.Def]++
			}
		}
	}
	for _, d := range Defs("skill") {
		if seen[d.Key] != 1 {
			t.Errorf("skill %s shown %d times", d.Key, seen[d.Key])
		}
	}
}

func TestEachBuiltInFitsItsOwnSize(t *testing.T) {
	t.Parallel()
	for _, p := range BuiltIns() {
		m := MetricsFor(*p.Text)
		for w := WhoColonist; w <= WhoWild; w++ {
			if h := Measure(Prune(*p.Docked, w), m, 0); h > float32(p.DockH-60) {
				t.Errorf("%s docked %c: %v in %d", p.Name, w.Letter(), h, p.DockH)
			}
			if h := Measure(Prune(*p.Floating, w), m, 0); h > float32(p.FloatH-16) {
				t.Errorf("%s floating %c: %v in %d", p.Name, w.Letter(), h, p.FloatH)
			}
		}
	}
}
