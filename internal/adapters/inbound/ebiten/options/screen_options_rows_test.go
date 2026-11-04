package options

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
)

// A step row writes its value next to its own + button, whichever column
// the row is in: Max colonies once wrote its number in the first column.
func TestStepValueFollowsItsRow(t *testing.T) {
	t.Parallel()
	for _, tab := range []string{"gameplay", "graphics", "audio"} {
		rows := OptionRows(tab)
		for i, r := range rows {
			if r.kind != rowStep {
				continue
			}
			_, cx, _ := OptPos(rows, i)
			plusEnd := cx + OptBtnH + 8 + optBarW + 8 + OptBtnH
			if x := stepValueX(cx); x < plusEnd || x > plusEnd+16 {
				t.Errorf("%s/%s: value at %.0f, the + button ends at %.0f", tab, r.Key, x, plusEnd)
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
