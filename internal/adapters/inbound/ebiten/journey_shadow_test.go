package ebiten

// journey_shadow_test.go: the STK-SH journey: only colonists outdoors cast
// a shadow, and Options › Graphics › Shadows turns it off.

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
)

// TestJourneyColonistShadowAndToggle (STK-SH): on a fresh colony at 08:00
// the one colonist, standing outdoors, casts one shadow under its feet;
// the Shadows row on Options › Graphics turns it off, and on again.
func TestJourneyColonistShadowAndToggle(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.a.renderer.art = envArt() // the world pipeline draws only with art
	st := &j.a.renderer.env.Shadow
	if st.Off {
		t.Fatal("fixture: shadows start off")
	}
	j.draw(2)
	if st.Drawn != 1 || len(st.Feet) != 1 {
		t.Fatalf("a colonist outdoors at %.1f h cast %d shadows, want 1", j.world().Hour, st.Drawn)
	}
	toggle := func() {
		j.key("Escape", 0)
		j.clickID("pause.options")
		if j.a.nav.screen() != ports.ScreenOptions {
			t.Fatalf("Options opened %s", j.a.nav.screen())
		}
		j.clickID("options.tab.graphics")
		j.clickID("options.shadows")
		for i := 0; i < 3 && j.a.nav.screen() != ports.ScreenColony; i++ {
			j.key("Escape", 0)
		}
		for i := 0; i < 2 && j.a.nav.overlay() != overlayNone; i++ {
			j.key("Escape", 0)
		}
	}
	toggle()
	if !st.Off || j.a.settings.Graphics.Shadows {
		t.Fatal("the Shadows row did not turn shadows off")
	}
	j.draw(2)
	if st.Drawn != 0 {
		t.Fatalf("shadows off still drew %d", st.Drawn)
	}
	toggle()
	j.draw(2)
	if st.Off || st.Drawn != 1 {
		t.Fatalf("shadows back on: off %v, drew %d", st.Off, st.Drawn)
	}
}
