package options

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
)

// A step row's two buttons frame its bar and value: the + button ends where
// the control column ends, on the - button's line, with room between them.
func TestStepButtonsFrameTheValue(t *testing.T) {
	t.Parallel()
	_, o := newFake()
	for _, tab := range []string{"gameplay", "graphics", "audio"} {
		o.Tab = tab
		for page := 0; page < 4; page++ {
			o.rowOff = page * 6
			bs := o.Buttons(1280, 720)
			for _, dec := range bs {
				base, ok := strings.CutSuffix(dec.ID, ".dec")
				if !ok {
					continue
				}
				for _, inc := range bs {
					if inc.ID != base+".inc" {
						continue
					}
					if inc.Y != dec.Y || inc.X+inc.W-dec.X != optCtrlW || inc.X-(dec.X+dec.W) < 40 {
						t.Errorf("%s/%s: - at %.0f, + at %.0f..%.0f", tab, base, dec.X, inc.X, inc.X+inc.W)
					}
				}
			}
		}
	}
}

// TestHiDPIRowNamesTheRenderCap (PERF-003): the HiDPI row says the size
// the render cap holds the frame to while it is holding it, and plain On
// or Off otherwise.
func TestHiDPIRowNamesTheRenderCap(t *testing.T) {
	t.Parallel()
	f, o := newFake()
	o.Tab = "graphics"
	value := func() string {
		for _, b := range o.Buttons(1280, 720) {
			if b.ID == ids.PrefixOptions+"hidpi" {
				return b.Label
			}
		}
		t.Fatal("no hidpi button")
		return ""
	}
	if got := value(); got != "On" {
		t.Fatalf("without a limit: %q", got)
	}
	f.env.RenderLimit = func() (int, int, bool) { return 5120, 2880, true }
	if got := value(); got != "On, limited to 5120x2880" {
		t.Fatalf("capped: %q", got)
	}
	f.settings.Graphics.HiDPI = false
	if got := value(); got != "Off" {
		t.Fatalf("HiDPI off: %q", got)
	}
	f.settings.Graphics.HiDPI = true
	f.env.RenderLimit = func() (int, int, bool) { return 2560, 1440, false }
	if got := value(); got != "On" {
		t.Fatalf("under the cap: %q", got)
	}
}
