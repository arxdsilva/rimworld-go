package options

import (
	"testing"

	"rimworld-go/internal/platform/config"
)

// The Pane height row (RHUD-003) steps through the compact pane's range in
// six presses, shows the share of the screen and stops at both ends.
func TestPaneHeightRow(t *testing.T) {
	var row OptionRow
	for _, r := range cardRows() {
		if r.Key == "cardheight" {
			row = r
		}
	}
	if row.apply == nil {
		t.Fatal("no Pane height row")
	}
	s := config.Defaults()
	if got := row.value(s); got != "27%" {
		t.Errorf("the default pane reads %q", got)
	}
	for range 10 {
		row.apply(&s, 1)
	}
	if s.HUD.Pane.Height != config.HUDPaneMaxH || row.value(s) != "30%" || row.frac(s) != 1 {
		t.Errorf("the top: %d %q %v", s.HUD.Pane.Height, row.value(s), row.frac(s))
	}
	presses := 0
	for ; s.HUD.Pane.Height > config.HUDPaneMinH; presses++ {
		row.apply(&s, -1)
	}
	if presses != 6 || row.frac(s) != 0 || row.value(s) != "20%" {
		t.Errorf("%d presses to the bottom, %q at %v", presses, row.value(s), row.frac(s))
	}
}
