package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
)

// TestRightColumnHiddenUnderResearch (UIS-065 journey): while Research is
// open its window spans the screen, the right column declares no global
// controls, alerts or letters and the minimap is not drawn; the speed
// hotkeys still change speed. Closing the window brings the column back.
func TestRightColumnHiddenUnderResearch(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.update(2)
	if !j.clickable("speed.1") || !hasWidget(j.a, "hud.tip.hour") {
		t.Fatal("no global controls before Research opens")
	}
	j.a.win.Show("research")
	j.update(3)
	l := j.a.hud()
	if !l.Right.Empty() || !l.Global.Empty() || !l.Minimap.Empty() {
		t.Fatalf("right column laid out under Research: right %+v global %+v minimap %+v", l.Right, l.Global, l.Minimap)
	}
	if want := float32(j.a.sw); l.Window.W != want {
		t.Fatalf("Research is %v wide on a %v screen", l.Window.W, want)
	}
	if hasWidget(j.a, "hud.tip.hour") || hasWidget(j.a, "hud.tip.date") {
		t.Error("the clock's hover cards are declared under Research")
	}
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if strings.HasPrefix(b.ID, "speed.") || strings.HasPrefix(b.ID, ids.PrefixAlert) || strings.HasPrefix(b.ID, ids.PrefixLetter) {
			t.Errorf("button %s declared under Research", b.ID)
		}
	}
	j.key("Digit2", 0)
	j.update(1)
	if j.a.renderer.speed == speedPaused {
		t.Fatal("the speed hotkey did nothing under Research")
	}
	j.a.win.Open = false
	j.update(3)
	if j.a.hud().Right.Empty() {
		t.Fatal("the right column did not come back")
	}
}
