package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The backdrop's own tests (the showcase, the disc) are in wizard.

func TestBackdropDrawsWithAndWithoutPlanet(t *testing.T) {
	resetSetupBackdrop()
	t.Cleanup(resetSetupBackdrop)
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	screen := ebiten.NewImage(1280, 720)
	// No diorama and no planet yet: gradient, stars and a plain sphere.
	a.drawSetupBackdrop(screen, 1280, 720, nil)
	if setupBackdrop.Stars == nil || setupBackdrop.Plain == nil {
		t.Fatal("stars or placeholder sphere not drawn")
	}
	// With the wizard's world: a disc rendered from it.
	p := globeTestPlanet(t, 30)
	a.drawSetupBackdrop(screen, 1280, 720, p)
	if setupBackdrop.View == nil || setupBackdrop.ViewOf != p || setupBackdrop.Disc == nil {
		t.Fatal("planet disc not built from the world")
	}
	lon := setupBackdrop.View.Cam.Lon
	for i := 0; i < 3*backdropRenderGap; i++ {
		a.drawSetupBackdrop(screen, 1280, 720, p)
	}
	if setupBackdrop.View.Cam.Lon == lon {
		t.Fatal("the planet does not drift")
	}
	// Reduced Motion freezes it.
	a.settings.Accessibility.ReducedMotion = true
	lon = setupBackdrop.View.Cam.Lon
	for i := 0; i < 3*backdropRenderGap; i++ {
		a.drawSetupBackdrop(screen, 1280, 720, p)
	}
	if setupBackdrop.View.Cam.Lon != lon {
		t.Fatal("planet drifts with Reduced Motion")
	}
	// A new world replaces the disc.
	q := globeTestPlanet(t, 50)
	a.drawSetupBackdrop(screen, 1280, 720, q)
	if setupBackdrop.ViewOf != q {
		t.Fatal("disc not rebuilt for the new world")
	}
	// A resize rebakes the stars.
	a.drawSetupBackdrop(ebiten.NewImage(1600, 900), 1600, 900, q)
	if setupBackdrop.StarW != 1600 {
		t.Fatal("stars not rebaked for the new size")
	}
}
