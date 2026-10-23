package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWanted(t *testing.T) {
	for path, want := range map[string]bool{
		"assets/generated/world/rock/granite.png":           true,
		"assets/generated/world/pawn/body_mask.png":         false,
		"assets/generated/world/rock/granite_n.png":         false,
		"assets/generated/ui/icons/axe.png":                 false,
		"assets/generated/world/rock/granite.txt":           false,
		"assets/generated/world/furniture/bed_far.png":      true,
		"assets/generated/world/terrain/soil_00.png":        false,
		"assets/generated/world/structure/floor_stone.png":  false,
		"assets/generated/world/structure/wall_wood_00.png": true,
	} {
		if got := wanted(path); got != want {
			t.Errorf("wanted(%q) = %v, want %v", path, got, want)
		}
	}
}

func writeSprite(t *testing.T, path string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 12, 12))
	for y := 2; y < 10; y++ {
		for x := 2; x < 10; x++ {
			img.SetNRGBA(x, y, color.NRGBA{180, 90, 40, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestRunWritesSkipsAndForces(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.png", "b.png", "a_mask.png"} {
		writeSprite(t, filepath.Join(dir, n))
	}
	if err := os.Mkdir(filepath.Join(dir, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSprite(t, filepath.Join(dir, "ui", "icon.png"))

	w, s, err := run([]string{dir}, false)
	if err != nil || w != 2 || s != 0 {
		t.Fatalf("first run = %d written, %d skipped, %v", w, s, err)
	}
	for _, n := range []string{"a_n.png", "b_n.png"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("%s missing: %v", n, err)
		}
	}
	for _, n := range []string{"a_mask_n.png", "ui/icon_n.png"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			t.Errorf("%s must not be written", n)
		}
	}
	// Normal maps are newer than their sprites: both up to date.
	if w, s, err = run([]string{dir}, false); err != nil || w != 0 || s != 2 {
		t.Fatalf("second run = %d written, %d skipped, %v", w, s, err)
	}
	// A sprite touched after its map is regenerated alone.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "a.png"), future, future); err != nil {
		t.Fatal(err)
	}
	if w, s, err = run([]string{dir}, false); err != nil || w != 1 || s != 1 {
		t.Fatalf("stale run = %d written, %d skipped, %v", w, s, err)
	}
	if w, _, err = run([]string{dir}, true); err != nil || w != 2 {
		t.Fatalf("-force run = %d written, %v", w, err)
	}
}

func TestRunReportsBadPNG(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.png"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run([]string{dir}, true); err == nil {
		t.Error("corrupt PNG not reported")
	}
	if _, _, err := run([]string{filepath.Join(dir, "missing")}, true); err == nil {
		t.Error("missing directory not reported")
	}
}

func TestNewerNeedsBothFiles(t *testing.T) {
	if newer("/nonexistent/a", "/nonexistent/b") {
		t.Error("missing output counted as newer")
	}
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	if err := os.WriteFile(a, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if newer(a, filepath.Join(dir, "missing")) {
		t.Error("missing input counted as older")
	}
}

func TestOneConvertsNonNRGBA(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "g.png")
	g := image.NewGray(image.Rect(0, 0, 8, 8))
	for i := range g.Pix {
		g.Pix[i] = 200
	}
	f, _ := os.Create(in)
	if err := png.Encode(f, g); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := one(in, filepath.Join(dir, "g_n.png"), settings()); err != nil {
		t.Fatal(err)
	}
	if err := one(filepath.Join(dir, "none.png"), filepath.Join(dir, "x_n.png"), settings()); err == nil {
		t.Error("missing input accepted")
	}
	if err := one(in, filepath.Join(dir, "no", "such", "x_n.png"), settings()); err == nil {
		t.Error("unwritable output accepted")
	}
}

func TestSettingsScaledForDoubleResolution(t *testing.T) {
	s := settings()
	if s.ShapeRadius != 8 || s.ShapeSoftness != 2.3 || s.DetailScale != 1.2 || s.Validate() != nil {
		t.Errorf("settings = %+v", s)
	}
}

func TestLinkedMaskFromMetadata(t *testing.T) {
	for src, want := range map[string]int{
		"autotile: rock\nmask: 6\ncorners: 2\n":                                                6,
		"tint:\n  group: stone_tint\n  mask: generated/x_mask.png\nautotile: rock\nmask: 15\n": 15,
		"mask: 0\n":         -1, // not an autotile
		"autotile: rock\n":  -1,
		"id: core:pawn/x\n": -1,
	} {
		if got := linkedMask(src); got != want {
			t.Errorf("linkedMask(%q) = %d, want %d", src, got, want)
		}
	}
}

// A west-linked sprite clipped flush at its tile edge must not get a
// bevel along that edge: the silhouette is extended past it first, so the
// normals there match the interior, while the free east edge still bevels.
func TestLinkedEdgeHasNoBevel(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 48))
	for y := 8; y < 40; y++ {
		for x := 16; x < 56; x++ { // clipped flush at x=16 (west linked)
			img.SetNRGBA(x, y, color.NRGBA{150, 150, 150, 255})
		}
	}
	plain, err := normalsFor(img, -1, settings())
	if err != nil {
		t.Fatal(err)
	}
	linked, err := normalsFor(img, 8, settings()) // W = 8
	if err != nil {
		t.Fatal(err)
	}
	if linked.Bounds() != img.Bounds() {
		t.Fatalf("bounds %v, want %v", linked.Bounds(), img.Bounds())
	}
	flat := func(c color.NRGBA) bool { return c.R >= 120 && c.R <= 136 }
	if c := plain.NRGBAAt(16, 24); flat(c) {
		t.Fatalf("unlinked west edge should bevel, normal %v", c)
	}
	if c := linked.NRGBAAt(16, 24); !flat(c) {
		t.Fatalf("linked west edge bevels: normal %v", c)
	}
	if c := linked.NRGBAAt(55, 24); flat(c) {
		t.Fatalf("free east edge lost its bevel: normal %v", c)
	}
}

// The extension is only for the bevel: the written map keeps the
// sprite's own alpha (the validator and the lit shader rely on it).
func TestLinkedNormalsKeepSpriteAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 48))
	for y := 8; y < 40; y++ {
		for x := 16; x < 56; x++ {
			img.SetNRGBA(x, y, color.NRGBA{150, 150, 150, uint8(128 + x)})
		}
	}
	n, err := normalsFor(img, 8|1, settings())
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			if got, want := n.NRGBAAt(x, y).A, img.NRGBAAt(x, y).A; got != want {
				t.Fatalf("alpha at %d,%d = %d, want %d", x, y, got, want)
			}
		}
	}
}

func TestFlatNormalFlag(t *testing.T) {
	for meta, want := range map[string]bool{
		"id: x\nflat_normal: true\n":    true,
		"flat_normal: false\n":          false,
		"  flat_normal: true\n":         false, // nested keys do not count
		"species: grass\n":              false,
		"flat_normal: true # comment\n": true,
	} {
		if got := flatNormal(meta); got != want {
			t.Errorf("flatNormal(%q) = %v, want %v", meta, got, want)
		}
	}
}

func TestFlatNormalsKeepAlphaAndPointUp(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 6, 6))
	src.SetNRGBA(2, 2, color.NRGBA{10, 200, 30, 255})
	src.SetNRGBA(3, 3, color.NRGBA{10, 200, 30, 90})
	n := flatNormals(src)
	if got := n.NRGBAAt(2, 2); got != (color.NRGBA{128, 128, 255, 255}) {
		t.Errorf("opaque pixel = %v, want flat up normal", got)
	}
	if got := n.NRGBAAt(3, 3); got.A != 90 || got.B != 255 {
		t.Errorf("translucent pixel = %v, want alpha 90 and up normal", got)
	}
	if got := n.NRGBAAt(0, 0); got != (color.NRGBA{}) {
		t.Errorf("transparent pixel = %v, want zero", got)
	}
}

func TestRunWritesFlatNormalForFlaggedSprite(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "generated", "world", "plant")
	metaDir := filepath.Join(root, "metadata", "plant")
	for _, d := range []string{dir, metaDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeSprite(t, filepath.Join(dir, "grass.png"))
	writeSprite(t, filepath.Join(dir, "bush.png"))
	if err := os.WriteFile(filepath.Join(metaDir, "grass.yaml"), []byte("flat_normal: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run([]string{dir}, true); err != nil {
		t.Fatal(err)
	}
	read := func(name string) *image.NRGBA {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		im, err := png.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		return im.(*image.NRGBA)
	}
	flat, dome := read("grass_n.png"), read("bush_n.png")
	if got := flat.NRGBAAt(2, 5); got != (color.NRGBA{128, 128, 255, 255}) {
		t.Errorf("flagged sprite edge = %v, want flat normal", got)
	}
	if got := dome.NRGBAAt(2, 5); got == (color.NRGBA{128, 128, 255, 255}) {
		t.Errorf("unflagged sprite edge is flat: %v", got)
	}
	if flat.NRGBAAt(0, 0).A != 0 {
		t.Error("flat map must keep transparent pixels transparent")
	}
}
