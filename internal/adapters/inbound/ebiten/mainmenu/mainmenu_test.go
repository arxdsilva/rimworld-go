package mainmenu

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

func TestMenuViewCoversScreen(t *testing.T) {
	t.Parallel()
	sizes := [][2]int{{1280, 720}, {1920, 1080}, {800, 600}, {2560, 1080}, {3840, 2160}}
	for _, sz := range sizes {
		for i := 0; i < 50; i++ {
			tick := float64(i) * driftPeriod / 17
			s, ox, oy := view(3200, 1800, sz[0], sz[1], tick)
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
		s0, x0, y0 := view(3200, 1800, 1920, 1080, tick)
		s1, x1, y1 := view(3200, 1800, 1920, 1080, tick)
		if s0 != s1 || x0 != x1 || y0 != y1 {
			t.Fatal("menuView is not deterministic")
		}
		s2, x2, y2 := view(3200, 1800, 1920, 1080, tick+1)
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
		if f := flicker(float64(i), i%3); f < 0.5 || f > 1 {
			t.Fatalf("flicker %v out of range", f)
		}
		if b := lampBreath(float64(i)); b < 0.9-1e-9 || b > 1 {
			t.Fatalf("lamp breath %v out of range", b)
		}
	}
}

func TestMenuMotesDeterministic(t *testing.T) {
	t.Parallel()
	for i := 0; i < MoteCount; i++ {
		x, y, r, al := mote(i, 1234, 1280, 720)
		x2, y2, r2, al2 := mote(i, 1234, 1280, 720)
		if x != x2 || y != y2 || r != r2 || al != al2 {
			t.Fatalf("mote %d not deterministic", i)
		}
		if x < 0 || x > 1280 || y < -8 || y > 728 || al < 0 || al > 0.25 {
			t.Fatalf("mote %d = (%v, %v) alpha %v out of bounds", i, x, y, al)
		}
		if float32(x) > RailX(1280) && al > 0 {
			t.Fatalf("mote %d visible over the button rail", i)
		}
	}
}

func TestMenuScrimDarkensRailAndTop(t *testing.T) {
	t.Parallel()
	sw, sh := 1280, 720
	centre := scrimAlpha(float64(sw)/2, float64(sh)/2, sw, sh)
	rail := scrimAlpha(float64(RailX(sw))+150, float64(sh)/2, sw, sh)
	top := scrimAlpha(float64(sw)/2, 10, sw, sh)
	if rail <= centre+0.3 || top <= centre+0.3 {
		t.Fatalf("scrim centre %.2f rail %.2f top %.2f: chrome areas not darkened", centre, rail, top)
	}
	img := buildScrim(sw, sh)
	if b := img.Bounds(); b.Dx() != sw/scrimDiv || b.Dy() != sh/scrimDiv {
		t.Fatalf("scrim %v, want %dx%d", b, sw/scrimDiv, sh/scrimDiv)
	}
}

func TestMenuDioramaAssetsDecode(t *testing.T) {
	t.Parallel()
	root, err := art.Locate()
	if err != nil {
		t.Skip(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(DioramaMetaRel)))
	if err != nil {
		t.Skip("menu diorama not generated (make art-menu)")
	}
	m, err := art.ParseMeta(string(raw))
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
	d, err := LoadDiorama(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Fires) == 0 || len(d.Lamps) == 0 {
		t.Fatalf("fires %v lamps %v: want at least one of each", d.Fires, d.Lamps)
	}
}

func TestMenuTitleCentered(t *testing.T) {
	t.Parallel()
	title := "rimworld-go"
	sw := 1280
	x := TitleX(sw, title)
	mid := sw / 2
	textMid := x + int(gfx.TextW(title, gfx.Huge))/2
	if textMid < mid-1 || textMid > mid+1 {
		t.Fatalf("title midpoint %d, want ~%d (x=%d)", textMid, mid, x)
	}
}

func TestMenuSplashPNGDecodes(t *testing.T) {
	t.Parallel()
	path, err := LocateMenuSplash()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() < 64 || b.Dy() < 64 {
		t.Fatalf("splash size %dx%d, want a usable bust", b.Dx(), b.Dy())
	}
	if b.Dy() <= b.Dx() {
		t.Fatalf("splash %dx%d, want portrait", b.Dx(), b.Dy())
	}
	transparent := false
	for y := b.Min.Y; y < b.Max.Y && !transparent; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				transparent = true
				break
			}
		}
	}
	if !transparent {
		t.Fatal("splash PNG should have transparent pixels so the menu backdrop shows through")
	}
	lastY := b.Max.Y - 1
	bottomOpaque := false
	for x := b.Min.X; x < b.Max.X; x++ {
		_, _, _, a := img.At(x, lastY).RGBA()
		if a != 0 {
			bottomOpaque = true
			break
		}
	}
	if !bottomOpaque {
		t.Fatal("splash PNG last row should meet the bust, not empty transparency")
	}
}

// TestMenuSplashKeepsItsRightEdgeFree (ART-029): the splash is anchored
// bottom-left, so a figure may run off the left and bottom edges but not
// the right: a cut there shows as a straight edge against the backdrop.
// The last columns of the PNG must be fully transparent.
func TestMenuSplashKeepsItsRightEdgeFree(t *testing.T) {
	t.Parallel()
	path, err := LocateMenuSplash()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	for x := b.Max.X - 2; x < b.Max.X; x++ {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 8<<8 {
				t.Fatalf("splash column %d (of %d) is opaque at y=%d: the figure is cut on the right; extend the source with margin", x, b.Max.X, y)
			}
		}
	}
}
