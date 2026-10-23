package ebiten

import (
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

func TestBackdropShowcaseBuildsAcrossFrames(t *testing.T) {
	resetSetupBackdrop()
	t.Cleanup(resetSetupBackdrop)
	b := &setupBackdrop
	calls := 0
	var worst time.Duration
	for b.discPlanet(nil) == nil {
		start := time.Now()
		calls++
		if calls > 20000 {
			t.Fatal("showcase never finished")
		}
		if d := time.Since(start); d > worst {
			worst = d
		}
	}
	if calls < 5 {
		t.Fatalf("showcase built in %d calls: not spread over frames", calls)
	}
	p := b.discPlanet(nil)
	if p.TileCount() == 0 || p.Params.SeedText != backdropShowcase.SeedText {
		t.Fatalf("showcase planet %+v", p.Params)
	}
	if b.discPlanet(nil) != p {
		t.Fatal("showcase rebuilt")
	}
	t.Logf("showcase generated in %d frames", calls)
}

func TestBackdropPrefersTheGivenPlanet(t *testing.T) {
	resetSetupBackdrop()
	t.Cleanup(resetSetupBackdrop)
	p := globeTestPlanet(t, 30)
	if got := setupBackdrop.discPlanet(p); got != p {
		t.Fatal("ignored the wizard's planet")
	}
	if setupBackdrop.showBuilder != nil {
		t.Fatal("started the showcase although a planet exists")
	}
}

func TestBackdropDrawsWithAndWithoutPlanet(t *testing.T) {
	resetSetupBackdrop()
	t.Cleanup(resetSetupBackdrop)
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	screen := ebiten.NewImage(1280, 720)
	// No diorama and no planet yet: gradient, stars and a plain sphere.
	a.drawSetupBackdrop(screen, 1280, 720, nil)
	if setupBackdrop.stars == nil || setupBackdrop.plain == nil {
		t.Fatal("stars or placeholder sphere not drawn")
	}
	// With the wizard's world: a disc rendered from it.
	p := globeTestPlanet(t, 30)
	a.drawSetupBackdrop(screen, 1280, 720, p)
	if setupBackdrop.view == nil || setupBackdrop.viewOf != p || setupBackdrop.disc == nil {
		t.Fatal("planet disc not built from the world")
	}
	lon := setupBackdrop.view.cam.Lon
	for i := 0; i < 3*backdropRenderGap; i++ {
		a.drawSetupBackdrop(screen, 1280, 720, p)
	}
	if setupBackdrop.view.cam.Lon == lon {
		t.Fatal("the planet does not drift")
	}
	// Reduced Motion freezes it.
	a.settings.Accessibility.ReducedMotion = true
	lon = setupBackdrop.view.cam.Lon
	for i := 0; i < 3*backdropRenderGap; i++ {
		a.drawSetupBackdrop(screen, 1280, 720, p)
	}
	if setupBackdrop.view.cam.Lon != lon {
		t.Fatal("planet drifts with Reduced Motion")
	}
	// A new world replaces the disc.
	q := globeTestPlanet(t, 50)
	a.drawSetupBackdrop(screen, 1280, 720, q)
	if setupBackdrop.viewOf != q {
		t.Fatal("disc not rebuilt for the new world")
	}
	// A resize rebakes the stars.
	a.drawSetupBackdrop(screen, 1600, 900, q)
	if setupBackdrop.starW != 1600 {
		t.Fatal("stars not rebaked for the new size")
	}
}

func TestBackdropDiscFitsItsRender(t *testing.T) {
	t.Parallel()
	// The halo (1.28 radii) must fit in the offscreen render.
	if r := discSilhouette(); 1.28*r > backdropDiscArea/2 {
		t.Fatalf("silhouette %.1f px: halo clipped in a %d px render", r, backdropDiscArea)
	}
}
