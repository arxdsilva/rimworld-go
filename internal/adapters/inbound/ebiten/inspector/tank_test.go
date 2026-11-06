package inspector

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// TestTankPaneLinesAndPicker: the lines name the stock, growth or the ready
// state, health and the trouble in the warning colour; the picker disables
// the current stock and refused ones with their reason and offers Empty.
func TestTankPaneLinesAndPicker(t *testing.T) {
	t.Parallel()
	v := &game.TankView{Stock: "carp", StockID: 2, Stocked: true, GrowthPct: 40, HealthPct: 55, FeedPct: 30, Trouble: "No feed",
		Options: []game.TankOption{{ID: 1, Label: "tilapia"}, {ID: 2, Label: "carp"}, {ID: 3, Label: "trout", Reason: "Too warm here"}}}
	lines := strings.Join(TankLines(v), "|")
	for _, want := range []string{"Stock: carp", "Growth  40%", "Fish health  55%", "Fish dying: No feed"} {
		if !strings.Contains(lines, want) {
			t.Errorf("lines %q lack %q", lines, want)
		}
	}
	if !PaneLineWarns("Fish dying: No feed") {
		t.Error("the trouble line is not drawn as a warning")
	}
	v.Ripe = true
	if !strings.Contains(strings.Join(TankLines(v), "|"), "Ready to net") {
		t.Error("a ripe tank does not say so")
	}
	if got := TankLines(&game.TankView{}); len(got) != 1 || !strings.Contains(got[0], "none chosen") {
		t.Errorf("empty tank lines %q", got)
	}
	opts := TankStockButton(lang.English{}, game.BuildingView{ID: 9, Tank: v}).Options()
	if len(opts) != 4 || opts[0].Label != "Tilapia" || opts[0].Disabled {
		t.Fatalf("options %+v", opts)
	}
	if !opts[1].Disabled || !opts[2].Disabled || opts[2].Reason != "Too warm here" {
		t.Errorf("the current stock and the refused one stay on: %+v", opts)
	}
	if c := opts[0].Cmd; c.Kind != command.SetTankStock || c.Target != 9 || c.Count != 1 {
		t.Errorf("command %+v", c)
	}
	if c := opts[3].Cmd; opts[3].Label != "Empty" || c.Count != 0 {
		t.Errorf("empty option %+v", opts[3])
	}
}
