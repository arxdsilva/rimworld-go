package ebiten

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// fileArtSprite is one sprite of syntheticFiles.
type fileArtSprite struct {
	key    string // "family/name"
	mask   bool   // has a tint mask (coverage on its middle two rows)
	normal bool   // has a normal map
}

// syntheticFiles writes a file-backed art root (no atlas): every sprite
// is a 24x16 canvas with an opaque 8x6 block at (4, 5), so its opaque
// height is 6 px.
func syntheticFiles(t *testing.T, sprites ...fileArtSprite) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "palettes"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := "version: 1\ntints:\n  skin:  {pale: \"#f0d3be\", dark: \"#6a4a3a\", red: \"#c03020\"}\n"
	if err := os.WriteFile(filepath.Join(root, "palettes", "base.yaml"), []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	for i, s := range sprites {
		fam, name, _ := strings.Cut(s.key, "/")
		img := filepath.ToSlash(filepath.Join("generated", "world", fam, name+".png"))
		pix := image.NewNRGBA(image.Rect(0, 0, 24, 16))
		for y := 5; y < 11; y++ {
			for x := 4; x < 12; x++ {
				pix.SetNRGBA(x, y, color.NRGBA{uint8(20 + 10*i), uint8(x * 9), uint8(y * 13), 255})
			}
		}
		writeTestPNG(t, filepath.Join(root, filepath.FromSlash(img)), pix)
		var y strings.Builder
		fmt.Fprintf(&y, "id: \"core:%s\"\nimage: %s\ncanvas: [24, 16]\nfootprint: [1, 1]\norigin_px: [3, 4]\n", s.key, img)
		if s.mask {
			mp := strings.TrimSuffix(img, ".png") + "_mask.png"
			m := image.NewGray(image.Rect(0, 0, 24, 16))
			for y := 7; y < 9; y++ {
				for x := 4; x < 12; x++ {
					m.SetGray(x, y, color.Gray{uint8(120 + x)})
				}
			}
			writeTestPNG(t, filepath.Join(root, filepath.FromSlash(mp)), m)
			fmt.Fprintf(&y, "tint:\n  group: skin\n  mask: %s\n", mp)
		}
		if s.normal {
			n := image.NewNRGBA(image.Rect(0, 0, 24, 16))
			for k := 0; k < len(n.Pix); k += 4 {
				n.Pix[k], n.Pix[k+1], n.Pix[k+2], n.Pix[k+3] = 128, uint8(k), 255, pix.Pix[k+3]
			}
			writeTestPNG(t, filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(img, ".png")+"_n.png")), n)
		}
		mp := filepath.Join(root, artMetadataRel, fam, name+".yaml")
		if err := os.MkdirAll(filepath.Dir(mp), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(mp, []byte(y.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

var memTestSprites = []fileArtSprite{
	{key: "rock/ore_a", mask: true, normal: true},
	{key: "terrain/soil_0", normal: true},
	{key: "plant/grass_short_v0"},           // read back on the CPU by the grass bake
	{key: "plant/bush_mature_v0"},           // not read back
	{key: "overlay/tilled_1"},               // read back by the crop bake (furrows)
	{key: "animal/deer_east", normal: true}, // left to first use
}

// The game's catalog indexes the art and decodes nothing until a sprite
// is used.
func TestLoadArtCatalogDecodesOnFirstUse(t *testing.T) {
	t.Parallel()
	root := syntheticFiles(t, memTestSprites...)
	c, err := loadArtCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	if !c.freeCPU {
		t.Fatal("the game's catalog must release CPU copies after upload")
	}
	for k, sp := range c.sprites {
		if sp.pix != nil || sp.mask != nil {
			t.Fatalf("%s decoded at load", k)
		}
	}
	sp := c.sprite("rock/ore_a")
	if sp.pix == nil || sp.mask == nil {
		t.Fatal("first use did not decode")
	}
	if s := c.memStats(); s.ColdFills != 1 || s.WarmFills != 0 {
		t.Fatalf("fills: %+v", s)
	}
	if idx, err := indexArtCatalog(root, true); err != nil || idx.freeCPU {
		t.Fatalf("the tests' index keeps CPU copies (err %v)", err)
	}
}

// The background prewarm holds deflated copies of the families every
// colony shows (not animals); a sprite filled from one equals its files.
func TestPrewarmFilesHoldsDeflatedCopies(t *testing.T) {
	t.Parallel()
	root := syntheticFiles(t, memTestSprites...)
	plain, err := indexArtCatalog(root, false)
	if err != nil {
		t.Fatal(err)
	}
	c, err := loadArtCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	<-c.prewarmAsync()
	if c.memStats().WarmHeld <= 0 {
		t.Fatal("prewarm held nothing")
	}
	if c.sprites["animal/deer_east"].file.warm[layerPix].z != nil {
		t.Fatal("animals are left to first use")
	}
	for k := range c.sprites {
		if strings.HasPrefix(k, "animal/") {
			continue
		}
		if c.sprites[k].file.warm[layerPix].z == nil {
			t.Fatalf("%s not prewarmed", k)
		}
		got, want := c.sprite(k), plain.sprite(k)
		if !bytes.Equal(got.pix.Pix, want.pix.Pix) || got.pix.Rect != want.pix.Rect {
			t.Fatalf("%s: prewarmed pixels differ from the file", k)
		}
		if (got.mask == nil) != (want.mask == nil) || got.mask != nil && !bytes.Equal(got.mask.Pix, want.mask.Pix) {
			t.Fatalf("%s: prewarmed mask differs from the file", k)
		}
		if got.normalPath != "" {
			n := got.file.normalNRGBA(got)
			wn, err := decodeNRGBA(got.normalPath)
			if err != nil || n == nil || !bytes.Equal(n.Pix, wn.Pix) {
				t.Fatalf("%s: prewarmed normal differs from the file", k)
			}
		}
	}
	s := c.memStats()
	if s.WarmFills != int64(len(c.sprites)-1) || s.ColdFills != 0 {
		t.Fatalf("fills %+v, want %d warm, 0 cold", s, len(c.sprites)-1)
	}
	if s.WarmHeld != 0 {
		t.Fatalf("%d deflated bytes left after every layer was taken", s.WarmHeld)
	}
}

// Draws racing the prewarm (run with -race) get the files' pixels, and no
// copy is published for a layer a draw already took.
func TestPrewarmFilesRacingDraws(t *testing.T) {
	t.Parallel()
	root := syntheticFiles(t, memTestSprites...)
	plain, err := indexArtCatalog(root, false)
	if err != nil {
		t.Fatal(err)
	}
	c, err := loadArtCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	done := c.prewarmAsync()
	keys := []string{"terrain/soil_0", "rock/ore_a", "plant/bush_mature_v0"}
	for _, k := range keys {
		if img := c.image(k, c.tint("skin", "red")); img == nil {
			t.Fatalf("%s: no image", k)
		}
	}
	<-done
	for _, k := range keys {
		for layer, w := range c.sprites[k].file.warm {
			if w.z != nil {
				t.Fatalf("%s: a copy of layer %d was published after it was taken", k, layer)
			}
		}
	}
	for k := range c.sprites {
		if got, want := c.sprite(k), plain.sprite(k); got.pix != nil && !bytes.Equal(got.pix.Pix, want.pix.Pix) {
			t.Fatalf("%s: pixels differ while prewarming", k)
		}
	}
}

// Once a sprite's GPU image exists the game frees its CPU pixels, except
// for the sprites CPU bakes read back (grass tufts, crop units, furrows);
// the tests' index keeps them all.
func TestImageReleasesCPUPixels(t *testing.T) {
	t.Parallel()
	root := syntheticFiles(t, memTestSprites...)
	c, err := loadArtCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"plant/bush_mature_v0", "terrain/soil_0", "animal/deer_east"} {
		sp := c.sprite(k)
		if c.image(k, color.NRGBA{}) == nil {
			t.Fatalf("%s: no image", k)
		}
		if sp.pix != nil {
			t.Fatalf("%s: CPU pixels kept after upload", k)
		}
		if h := sp.opaqueHeightPx(); h != 6 {
			t.Fatalf("%s: opaque height %v after release, want 6", k, h)
		}
		if sp.normalPath != "" && c.normalOf(sp.img) == nil {
			t.Fatalf("%s: normal map not linked before release", k)
		}
	}
	for _, k := range []string{"plant/grass_short_v0", "overlay/tilled_1"} {
		c.image(k, color.NRGBA{})
		if c.sprites[k].pix == nil {
			t.Fatalf("%s: a CPU bake reads it back, its pixels must stay", k)
		}
	}
	s := c.memStats()
	const px = 24 * 16 * 4
	if s.Freed != 3 || s.FreedBytes != 3*px || s.Kept != 2 || s.KeptBytes != 2*px {
		t.Fatalf("stats %+v", s)
	}
	if !strings.Contains(s.String(), "freed 3") || !strings.Contains(s.String(), "kept 2") || s.AtlasHeld != 0 {
		t.Fatalf("report %q", s.String())
	}

	idx, err := indexArtCatalog(root, true)
	if err != nil {
		t.Fatal(err)
	}
	idx.image("plant/bush_mature_v0", color.NRGBA{})
	if idx.sprites["plant/bush_mature_v0"].pix == nil {
		t.Fatal("the tests' index must keep CPU pixels")
	}
}

// A masked sprite keeps only its mask's coverage box on the CPU after
// release, still tints the same, and bakes new tints without CPU pixels.
func TestReleasedMaskedSpriteStillTints(t *testing.T) {
	t.Parallel()
	root := syntheticFiles(t, memTestSprites...)
	c, err := loadArtCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	sp := c.sprite("rock/ore_a")
	pix := image.NewNRGBA(sp.pix.Rect)
	copy(pix.Pix, sp.pix.Pix)
	full := sp.mask
	red := c.tint("skin", "red")
	img := c.image("rock/ore_a", red)
	if img == nil || img == sp.img || img.Bounds() != sp.img.Bounds() {
		t.Fatalf("tinted image %v (base %v)", img, sp.img)
	}
	if sp.pix != nil || sp.mask == nil {
		t.Fatalf("pix kept %v, mask %v", sp.pix != nil, sp.mask != nil)
	}
	if sp.mask.Rect != image.Rect(4, 7, 12, 9) {
		t.Fatalf("mask kept as %v, want its coverage box", sp.mask.Rect)
	}
	if !bytes.Equal(tintNRGBA(pix, sp.mask, red).Pix, tintNRGBA(pix, full, red).Pix) {
		t.Fatal("the cropped mask tints differently")
	}
	if c.image("rock/ore_a", red) != img {
		t.Fatal("a tint bake is cached")
	}
	dark := c.image("rock/ore_a", c.tint("skin", "dark"))
	if dark == nil || dark == img || dark.Bounds() != sp.img.Bounds() {
		t.Fatal("a new tint after release must bake on the GPU")
	}
	if c.normalOf(dark) == nil {
		t.Fatal("a tint bake shares its sprite's normal map")
	}
}

// The tint bakes are bounded: past the budget the least recently used
// bake goes (from the index light.go scans and the normal links) and is
// deallocated; the order is the use order, so eviction is deterministic.
func TestTintBakesEvictLeastRecentlyUsed(t *testing.T) {
	t.Parallel()
	root := syntheticFiles(t, memTestSprites...)
	c, err := loadArtCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	const bake = 24 * 16 * 4
	c.tintBudget = 2 * bake
	tint := func(i int) color.NRGBA { return color.NRGBA{uint8(10 * i), 50, 90, 255} }
	a := c.image("rock/ore_a", tint(1))
	b := c.image("rock/ore_a", tint(2))
	if c.image("rock/ore_a", tint(1)) != a { // a is now the most recent
		t.Fatal("cache miss on a fresh bake")
	}
	c.image("rock/ore_a", tint(3)) // evicts b
	if _, ok := c.tinted[tintKey{"rock/ore_a", tint(2)}]; ok {
		t.Fatal("least recently used bake kept")
	}
	if c.normalOf(b) != nil {
		t.Fatal("an evicted bake's normal link kept")
	}
	if c.tinted[tintKey{"rock/ore_a", tint(1)}] != a {
		t.Fatal("recently used bake evicted")
	}
	s := c.memStats()
	if s.Tints != 2 || s.TintBytes != 2*bake || s.TintEvicted != 1 {
		t.Fatalf("tint stats %+v", s)
	}
	// a literal catalog (tests) bakes with no budget set and no maps made
	lit := &artCatalog{sprites: map[string]*artSprite{"x/y": {
		pix: image.NewNRGBA(image.Rect(0, 0, 4, 4)), mask: image.NewNRGBA(image.Rect(0, 0, 4, 4))}}}
	if lit.image("x/y", tint(1)) == nil || len(lit.tinted) != 1 {
		t.Fatal("literal catalog bake")
	}
}

// tint.kage is tintNRGBA on the GPU: its Go twin agrees with the CPU bake
// to two levels (the CPU bake floors twice, the GPU rounds once), and the
// shader compiles.
func TestTintShaderTwin(t *testing.T) {
	t.Parallel()
	if _, err := tintShader(); err != nil {
		t.Fatal(err)
	}
	src := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	mask := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for i := range 256 {
		o := 4 * i
		src.Pix[o], src.Pix[o+1], src.Pix[o+2], src.Pix[o+3] = uint8(i), uint8(255-i), uint8(i*7), 255
		mask.Pix[o], mask.Pix[o+1], mask.Pix[o+2], mask.Pix[o+3] = uint8(i*13), uint8(i*13), uint8(i*13), 255
	}
	for _, tc := range []color.NRGBA{{240, 211, 190, 255}, {192, 48, 32, 255}, {0, 0, 0, 255}, {255, 255, 255, 255}} {
		cpu := tintNRGBA(src, mask, tc)
		for i := range 256 {
			o := 4 * i
			for ch, tv := range [3]uint8{tc.R, tc.G, tc.B} {
				got := tintTexel(src.Pix[o+ch], mask.Pix[o], tv)
				if d := int(got) - int(cpu.Pix[o+ch]); d < -2 || d > 2 {
					t.Fatalf("tint %v texel %d ch %d: twin %d, CPU %d", tc, i, ch, got, cpu.Pix[o+ch])
				}
			}
		}
	}
}

// maskImage keeps the coverage box as an opaque grey image (red =
// coverage), clipped to the sprite.
func TestMaskImageCoverageBox(t *testing.T) {
	t.Parallel()
	m := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	for i := 3; i < len(m.Pix); i += 4 {
		m.Pix[i] = 255
	}
	if img, box := maskImage(m, image.Rect(0, 0, 10, 10)); img != nil || !box.Empty() {
		t.Fatal("an empty mask needs no GPU image")
	}
	m.SetNRGBA(2, 3, color.NRGBA{9, 9, 9, 255})
	m.SetNRGBA(8, 9, color.NRGBA{200, 200, 200, 255})
	cov := maskCoverage(m)
	if cov.Rect != image.Rect(2, 3, 9, 10) || cov.NRGBAAt(8, 9) != (color.NRGBA{200, 200, 200, 255}) || cov.NRGBAAt(3, 3).A != 255 {
		t.Fatalf("coverage %v", cov.Rect)
	}
	img, box := maskImage(m, image.Rect(0, 0, 6, 6))
	if box != image.Rect(2, 3, 6, 6) || img == nil || img.Bounds().Size() != box.Size() {
		t.Fatalf("clipped box %v", box)
	}
	var _ *ebiten.Image = img
}

// The report counts the atlas sprites' deflated copies until taken.
func TestMemStatsCountsAtlasCopies(t *testing.T) {
	t.Parallel()
	c, err := loadArtCatalog(syntheticAtlas(t, 3))
	if err != nil {
		t.Fatal(err)
	}
	held := c.prewarmAtlases()
	if s := c.memStats(); held == 0 || s.AtlasHeld != int64(held) {
		t.Fatalf("atlas held %d, prewarm reported %d", s.AtlasHeld, held)
	}
	for k := range c.sprites {
		c.image(k, c.tint("skin", "pale"))
	}
	if s := c.memStats(); s.AtlasHeld != 0 || s.Freed != 3 || s.Tints != 3 {
		t.Fatalf("after drawing every sprite: %+v", s)
	}
}
