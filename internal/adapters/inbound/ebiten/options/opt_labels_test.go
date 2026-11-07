package options

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// A row's label must end before its control: 255 px in the window (the
// label starts at contentX, the control at OptCtrlX), less the room a row's
// second button takes.
func TestRowLabelsFitBeforeTheirControl(t *testing.T) {
	t.Parallel()
	for _, tab := range []string{"audio", "graphics", "gameplay", "access", "general", "interface", "developer"} {
		for _, r := range OptionRows(tab) {
			room := float64(OptCtrlX - contentX - 8 - extraRoom(r))
			if w, _ := gfx.Measure(r.Label, gfx.Body); w > room {
				t.Errorf("%s/%s: %q is %.0f px, %.0f fit", tab, r.Key, r.Label, w, room)
			}
		}
	}
}
