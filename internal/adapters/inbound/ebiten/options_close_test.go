package ebiten

import (
	"strconv"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/platform/config"
)

// UIS-075: the Options window's X closes it exactly as Back/OK does.
func TestOptionsCloseXSendsBack(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	optOpen(j, "general")
	if j.a.nav.screen() != ports.ScreenOptions {
		t.Fatalf("screen = %s, want options", j.a.nav.screen())
	}
	if !j.clickID("options.close") {
		t.Fatal("the X is disabled")
	}
	j.update(1)
	if j.a.nav.screen() != ports.ScreenColony || j.a.nav.overlay() != overlayPause {
		t.Fatalf("X left %s/%s, want colony/pause like Back", j.a.nav.screen(), j.a.nav.overlay())
	}
}

// UIS-008: the UI scale row opens a drop list under the row: Auto and the
// steps 29 px apart, at least 70 wide, a refused step disabled; a pick
// changes the scale and asks keep or revert.
func TestScaleDropListPicksAStep(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	optOpen(j, "access")
	j.a.opt.Scale.Choose(0)
	j.clickID("options.uiscale")
	j.update(1)
	if j.a.uiSt().Top() != "options.uiscale.pick" {
		t.Fatalf("top = %q, want the drop list", j.a.uiSt().Top())
	}
	row, _ := j.a.uiSt().Find("options.uiscale.pick.0")
	next, ok := j.a.uiSt().Find("options.uiscale.pick.1")
	if !ok {
		t.Fatal("no second entry")
	}
	if next.R.Y-row.R.Y != 29 || row.R.W < 70 {
		t.Fatalf("rows %v apart, %v wide; want 29 and at least 70", next.R.Y-row.R.Y, row.R.W)
	}
	n := len(config.UIScalePresets)
	big, ok := j.a.uiSt().Find("options.uiscale.pick." + strconv.Itoa(n))
	if !ok || !big.Disabled || big.Reason == "" {
		t.Fatalf("the largest step must be refused with a reason: %+v", big)
	}
	step100 := 0
	for i, p := range config.UIScalePresets {
		if p == 100 {
			step100 = i + 1
		}
	}
	j.clickID("options.uiscale.pick." + strconv.Itoa(step100))
	j.update(1)
	if ac := j.a.settings.Accessibility; ac.UIScaleAuto || ac.UIScale != 100 {
		t.Fatalf("after the pick: %+v", ac)
	}
	if j.a.uiSt().Top() != "options.uiscale.keep" {
		t.Fatalf("top = %q, want the keep/revert dialog", j.a.uiSt().Top())
	}
}
