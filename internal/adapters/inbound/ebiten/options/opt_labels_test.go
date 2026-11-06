package options

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// A row's label must end before its control: 204 px in the first column,
// 112 px in the second (the label sits at x=600, its control at 720).
func TestRowLabelsFitBeforeTheirControl(t *testing.T) {
	t.Parallel()
	for _, tab := range []string{"audio", "graphics", "gameplay", "access", "general", "interface", "developer"} {
		for _, r := range OptionRows(tab) {
			room := float64(OptCtrlX - 56 - 8)
			if r.Col == 1 {
				room = float64(optCol2CtrlX - optCol2LabelX - 8)
			}
			if w, _ := gfx.Measure(r.Label, gfx.Body); w > room {
				t.Errorf("%s/%s: %q is %.0f px, %.0f fit", tab, r.Key, r.Label, w, room)
			}
		}
	}
}
