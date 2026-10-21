package ebiten

import (
	"errors"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

func TestMenuViewCoversScreen(t *testing.T) {
	t.Parallel()
	sizes := [][2]int{{1280, 720}, {1920, 1080}, {800, 600}, {2560, 1080}, {3840, 2160}}
	for _, sz := range sizes {
		for i := 0; i < 50; i++ {
			tick := float64(i) * menuDriftPeriod / 17
			s, ox, oy := menuView(3200, 1800, sz[0], sz[1], tick)
			const eps = 1e-6
			if ox > eps || oy > eps || ox+3200*s < float64(sz[0])-eps || oy+1800*s < float64(sz[1])-eps {
				t.Fatalf("%v t=%v: scale %v offset (%v, %v) leaves the screen uncovered", sz, tick, s, ox, oy)
			}
		}
	}
}

func TestMenuViewSlowAndDeterministic(t *testing.T) {
	t.Parallel()
	for i := 0; i < 400; i++ {
		tick := float64(i * 37)
		s0, x0, y0 := menuView(3200, 1800, 1920, 1080, tick)
		s1, x1, y1 := menuView(3200, 1800, 1920, 1080, tick)
		if s0 != s1 || x0 != x1 || y0 != y1 {
			t.Fatal("menuView is not deterministic")
		}
		s2, x2, y2 := menuView(3200, 1800, 1920, 1080, tick+1)
		// the corner furthest from the zoom origin moves the most
		dx := math.Abs((x2 + 3200*s2) - (x0 + 3200*s0))
		dy := math.Abs((y2 + 1800*s2) - (y0 + 1800*s0))
		if math.Abs(x2-x0) > 0.5 || math.Abs(y2-y0) > 0.5 || dx > 0.5 || dy > 0.5 {
			t.Fatalf("t=%v: view moved (%v, %v / %v, %v) px in one tick", tick, x2-x0, y2-y0, dx, dy)
		}
	}
}

func TestMenuFlickerAndBreathRange(t *testing.T) {
	t.Parallel()
	for i := 0; i < 5000; i++ {
		if f := menuFlicker(float64(i), i%3); f < 0.5 || f > 1 {
			t.Fatalf("flicker %v out of range", f)
		}
		if b := menuLampBreath(float64(i)); b < 0.9-1e-9 || b > 1 {
			t.Fatalf("lamp breath %v out of range", b)
		}
	}
}

func TestMenuMotesDeterministic(t *testing.T) {
	t.Parallel()
	for i := 0; i < menuMotes; i++ {
		x, y, r, al := menuMote(i, 1234, 1280, 720)
		x2, y2, r2, al2 := menuMote(i, 1234, 1280, 720)
		if x != x2 || y != y2 || r != r2 || al != al2 {
			t.Fatalf("mote %d not deterministic", i)
		}
		if x < 0 || x > 1280 || y < -8 || y > 728 || al < 0 || al > 0.25 {
			t.Fatalf("mote %d = (%v, %v) alpha %v out of bounds", i, x, y, al)
		}
		if float32(x) > menuRailX(1280) && al > 0 {
			t.Fatalf("mote %d visible over the button rail", i)
		}
	}
}

func TestMenuScrimDarkensRailAndTop(t *testing.T) {
	t.Parallel()
	sw, sh := 1280, 720
	centre := menuScrimAlpha(float64(sw)/2, float64(sh)/2, sw, sh)
	rail := menuScrimAlpha(float64(menuRailX(sw))+150, float64(sh)/2, sw, sh)
	top := menuScrimAlpha(float64(sw)/2, 10, sw, sh)
	if rail <= centre+0.3 || top <= centre+0.3 {
		t.Fatalf("scrim centre %.2f rail %.2f top %.2f: chrome areas not darkened", centre, rail, top)
	}
	img := buildMenuScrim(sw, sh)
	if b := img.Bounds(); b.Dx() != sw/menuScrimDiv || b.Dy() != sh/menuScrimDiv {
		t.Fatalf("scrim %v, want %dx%d", b, sw/menuScrimDiv, sh/menuScrimDiv)
	}
}

func testDiorama() *menuDiorama {
	base := ebiten.NewImage(320, 180)
	base.Fill(uiDust)
	return &menuDiorama{levels: []*ebiten.Image{base, ebiten.NewImage(160, 90)},
		lamp: ebiten.NewImage(160, 90), fire: ebiten.NewImage(160, 90), w: 320, h: 180, tilePx: 12.8,
		fires: [][2]float64{{160, 100}}, lamps: [][2]float64{{150, 60}}}
}

func TestMainMenuDrawsDioramaOrFallsBack(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.cfg.LoadMenuScene = func() (*menuDiorama, error) { return nil, errors.New("not generated") }
	a.finishBoot()
	screen := ebiten.NewImage(640, 360)
	if a.drawMenuDiorama(screen, 640, 360, 0) {
		t.Fatal("drew a diorama that failed to load")
	}
	a.drawMainMenu(screen, 640, 360)
	a.menu = testDiorama()
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
	c, err := loadArtCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	for k := range c.sprites {
		if filepath.Dir(k) == menuMetaFamily {
			t.Fatalf("menu art %q loaded as a sprite", k)
		}
	}
}

func TestMenuDioramaAssetsDecode(t *testing.T) {
	t.Parallel()
	root, err := LocateArt()
	if err != nil {
		t.Skip(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(menuDioramaMetaRel)))
	if err != nil {
		t.Skip("menu diorama not generated (make art-menu)")
	}
	m, err := parseArtMeta(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	decode := func(rel string) (w, h int, opaque bool) {
		f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		b := img.Bounds()
		opaque = true
		for _, p := range [][2]int{{0, 0}, {b.Dx() / 2, b.Dy() / 2}, {b.Dx() - 1, b.Dy() - 1}} {
			if _, _, _, al := img.At(p[0], p[1]).RGBA(); al != 0xFFFF {
				opaque = false
			}
		}
		return b.Dx(), b.Dy(), opaque
	}
	w, h, opaque := decode(m.Image)
	if w != m.Canvas[0] || h != m.Canvas[1] || !opaque {
		t.Fatalf("base %dx%d opaque=%v, metadata canvas %v", w, h, opaque, m.Canvas)
	}
	for _, key := range []string{"glow_lamp", "glow_fire"} {
		rel := metaScalar(string(raw), key)
		if rel == "" {
			t.Fatalf("metadata has no %s", key)
		}
		if gw, gh, _ := decode(rel); gw != w/2 || gh != h/2 {
			t.Fatalf("%s %dx%d, want %dx%d", key, gw, gh, w/2, h/2)
		}
	}
	d, err := loadMenuDiorama(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.fires) == 0 || len(d.lamps) == 0 {
		t.Fatalf("fires %v lamps %v: want at least one of each", d.fires, d.lamps)
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
