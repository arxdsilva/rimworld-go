package ebiten

import (
	"testing"

	"rimworld-go/internal/platform/input"
)

func TestSetupCapturesOpenTheirScreens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ui     string
		screen screenID
		modal  string
	}{
		{"worldgen", screenWorldGen, ""},
		{"seed", screenWorldGen, ""},
		{"worldgen-progress", screenWorldGen, worldProgress},
		{"landing", screenLanding, ""},
		{"picker", screenLanding, ""},
		{"landing-advanced", screenLanding, "land.advanced"},
		{"landing-factions", screenLanding, facModalID},
		{"pawns", screenPawns, ""},
		{"pawns-reroll", screenPawns, ""},
	}
	for _, c := range cases {
		a, f, _ := ngTest(t, nil)
		a.cfg.Capture = &Capture{Path: t.TempDir() + "/x.png", UI: c.ui}
		for i := 0; i < captureFrames+20; i++ {
			f.frame(t, a)
		}
		if a.nav.screen() != c.screen || a.uiModalTop() != c.modal || a.renderer != nil {
			t.Errorf("%s: screen=%s modal=%q renderer=%v", c.ui, a.nav.screen(), a.uiModalTop(), a.renderer != nil)
		}
		if c.ui == "worldgen-progress" {
			ng := a.ng
			if ng.builder == nil || ng.genFrac < 0.4 || ng.genFrac > 0.8 {
				t.Errorf("the progress capture freezes part-way, frac %v", ng.genFrac)
			}
		}
		if c.ui == "landing" && (a.ng.planet == nil || a.ng.tile < 0 || a.ng.planet.Params.SeedText != "amber-hollow") {
			t.Errorf("landing capture: planet=%v tile=%d", a.ng.planet != nil, a.ng.tile)
		}
	}
}

func TestEscapeOrderOnTheLaterSteps(t *testing.T) {
	t.Parallel()
	// Landing: Esc closes the Advanced dialog, then goes Back to Create World.
	a, f := landingApp(t)
	click(t, a, "land.advanced")
	f.frame(t, a)
	f.key(t, a, "Escape", 0)
	if a.uiModalOpen() || a.nav.screen() != screenLanding {
		t.Fatalf("first Esc closes the modal: modal=%q screen=%s", a.uiModalTop(), a.nav.screen())
	}
	f.key(t, a, "Escape", 0)
	if a.nav.screen() != screenWorldGen || a.ng == nil || a.ng.planet == nil {
		t.Fatalf("second Esc goes Back and keeps the planet: screen=%s", a.nav.screen())
	}
	// Create World: a text field in edit takes Esc first.
	f.frame(t, a)
	press(t, a, f, "world.seed")
	f.key(t, a, "Escape", 0)
	if a.nav.screen() != screenWorldGen || a.uiTextEditing() {
		t.Fatalf("Esc ends the edit only: screen=%s editing=%v", a.nav.screen(), a.uiTextEditing())
	}
	// Characters: Back returns to the landing site with the tile kept.
	b, fb := pawnsApp(t)
	tile := b.ng.tile
	f2 := fb
	f2.key(t, b, "Escape", 0)
	f2.frame(t, b)
	if b.nav.screen() != screenLanding || b.ng.tile != tile {
		t.Fatalf("screen=%s tile=%d want %d", b.nav.screen(), b.ng.tile, tile)
	}
	_ = input.Cancel
}
