package ebiten

import (
	"errors"
	"path/filepath"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/mainmenu"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

func testDiorama() *mainmenu.Diorama {
	base := ebiten.NewImage(320, 180)
	base.Fill(uikit.Dust)
	return &mainmenu.Diorama{Levels: []*ebiten.Image{base, ebiten.NewImage(160, 90)},
		Lamp: ebiten.NewImage(160, 90), Fire: ebiten.NewImage(160, 90), W: 320, H: 180, TilePx: 12.8,
		Fires: [][2]float64{{160, 100}}, Lamps: [][2]float64{{150, 60}}}
}

func TestMainMenuDrawsDioramaOrFallsBack(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.cfg.LoadMenuScene = func() (*mainmenu.Diorama, error) { return nil, errors.New("not generated") }
	a.finishBoot()
	screen := ebiten.NewImage(640, 360)
	if a.drawMenuDiorama(screen, 640, 360, 0) {
		t.Fatal("drew a diorama that failed to load")
	}
	a.drawMainMenu(screen, 640, 360)
	a.menuScreen.SetDiorama(testDiorama())
	a.menuTick = 777
	if !a.drawMenuDiorama(screen, 640, 360, 0.5) {
		t.Fatal("diorama not drawn")
	}
	a.drawMainMenu(screen, 640, 360)
	a.drawCredits(screen, 640, 360)
}

func TestArtCatalogSkipsMenuFamily(t *testing.T) {
	t.Parallel()
	root, err := LocateArt()
	if err != nil {
		t.Skip(err)
	}
	c, err := indexArtCatalog(root, false)
	if err != nil {
		t.Fatal(err)
	}
	for k := range c.Sprites {
		if filepath.Dir(k) == mainmenu.MetaFamily {
			t.Fatalf("menu art %q loaded as a sprite", k)
		}
	}
}

func TestMenuCaptureDoesNotStartSim(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.cfg.Capture = &Capture{Path: filepath.Join(t.TempDir(), "menu.png"), UI: "menu", Ticks: 300}
	for i := 0; i < captureFrames+3; i++ {
		a.tickCapture()
	}
	if a.renderer != nil || a.simsStarted != 0 {
		t.Fatal("menu capture started a game")
	}
	if a.nav.screen() != screenMainMenu || a.menuTick != 300 {
		t.Fatalf("screen %v tick %d, want main menu at tick 300", a.nav.screen(), a.menuTick)
	}
}
