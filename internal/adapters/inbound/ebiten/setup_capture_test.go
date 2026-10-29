package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"rimworld-go/internal/platform/input"
)

func TestSetupCapturesOpenTheirScreens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ui     string
		screen ports.Screen
		modal  string
	}{
		{"worldgen", ports.ScreenWorldGen, ""},
		{"seed", ports.ScreenWorldGen, ""},
		{"worldgen-progress", ports.ScreenWorldGen, wizard.WorldProgress},
		{"landing", ports.ScreenLanding, ""},
		{"picker", ports.ScreenLanding, ""},
		{"landing-advanced", ports.ScreenLanding, "land.advanced"},
		{"landing-factions", ports.ScreenLanding, wizard.FacModalID},
		{"pawns", ports.ScreenPawns, ""},
		{"pawns-reroll", ports.ScreenPawns, ""},
	}
	for _, c := range cases {
		a, f, _ := ngTest(t, nil)
		a.cfg.Capture = &Capture{Path: t.TempDir() + "/x.png", UI: c.ui}
		for i := 0; i < captureFrames+20; i++ {
			f.Frame(t, a)
		}
		if a.nav.screen() != c.screen || a.uiModalTop() != c.modal || a.renderer != nil {
			t.Errorf("%s: screen=%s modal=%q renderer=%v", c.ui, a.nav.screen(), a.uiModalTop(), a.renderer != nil)
		}
		if c.ui == "worldgen-progress" {
			ng := a.ng
			if ng.Builder == nil || ng.GenFrac < 0.4 || ng.GenFrac > 0.8 {
				t.Errorf("the progress capture freezes part-way, frac %v", ng.GenFrac)
			}
		}
		if c.ui == "landing" && (a.ng.Planet == nil || a.ng.Tile < 0 || a.ng.Planet.Params.SeedText != "amber-hollow") {
			t.Errorf("landing capture: planet=%v tile=%d", a.ng.Planet != nil, a.ng.Tile)
		}
	}
}

func TestEscapeOrderOnTheLaterSteps(t *testing.T) {
	t.Parallel()
	// Landing: Esc closes the Advanced dialog, then goes Back to Create World.
	a, f := landingApp(t)
	click(t, a, "land.advanced")
	f.Frame(t, a)
	f.Key(t, a, "Escape", 0)
	if a.uiSt().ModalOpen() || a.nav.screen() != ports.ScreenLanding {
		t.Fatalf("first Esc closes the modal: modal=%q screen=%s", a.uiModalTop(), a.nav.screen())
	}
	f.Key(t, a, "Escape", 0)
	if a.nav.screen() != ports.ScreenWorldGen || a.ng == nil || a.ng.Planet == nil {
		t.Fatalf("second Esc goes Back and keeps the planet: screen=%s", a.nav.screen())
	}
	// Create World: a text field in edit takes Esc first.
	f.Frame(t, a)
	press(t, a, f, "world.seed")
	f.Key(t, a, "Escape", 0)
	if a.nav.screen() != ports.ScreenWorldGen || a.uiSt().Editing() {
		t.Fatalf("Esc ends the edit only: screen=%s editing=%v", a.nav.screen(), a.uiSt().Editing())
	}
	// Characters: Back returns to the landing site with the tile kept.
	b, fb := pawnsApp(t)
	tile := b.ng.Tile
	f2 := fb
	f2.Key(t, b, "Escape", 0)
	f2.Frame(t, b)
	if b.nav.screen() != ports.ScreenLanding || b.ng.Tile != tile {
		t.Fatalf("screen=%s tile=%d want %d", b.nav.screen(), b.ng.Tile, tile)
	}
	_ = input.Cancel
}
