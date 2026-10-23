package ebiten

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
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

// syntheticAtlas writes a packed family "dummy" of n sprites, one per page,
// into a fresh assets root: sprite i has a (4+i)x3 block at (i,2) in a
// 24x16 canvas, packed at (1,1) of its page. It returns the root.
func syntheticAtlas(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "palettes"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := "version: 1\ntints:\n  skin:  {pale: \"#f0d3be\"}\n"
	if err := os.WriteFile(filepath.Join(root, "palettes", "base.yaml"), []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	var y strings.Builder
	fmt.Fprintf(&y, "version: 1\nfamily: dummy\nmax: 64\npages: %d\nsprites:\n", n)
	for i := 0; i < n; i++ {
		w := 4 + i
		page := image.NewNRGBA(image.Rect(0, 0, 40, 20))
		mask := image.NewGray(image.Rect(0, 0, 40, 20))
		norm := image.NewNRGBA(image.Rect(0, 0, 40, 20))
		for k := 0; k < len(norm.Pix); k += 4 {
			norm.Pix[k], norm.Pix[k+1], norm.Pix[k+2], norm.Pix[k+3] = 128, 128, 255, 255
		}
		for py := 0; py < 3; py++ {
			for px := 0; px < w; px++ {
				page.SetNRGBA(1+px, 1+py, color.NRGBA{uint8(10 * i), uint8(px), uint8(py), 255})
				mask.SetGray(1+px, 1+py, color.Gray{uint8(100 + px)})
				norm.SetNRGBA(1+px, 1+py, color.NRGBA{128, uint8(i), 255, 255}) // pages are opaque RGB
			}
		}
		writeTestPNG(t, filepath.Join(root, "generated", "atlas", fmt.Sprintf("dummy_%d.png", i)), page)
		writeTestPNG(t, filepath.Join(root, "generated", "atlas", fmt.Sprintf("dummy_%d_mask.png", i)), mask)
		writeTestPNG(t, filepath.Join(root, "generated", "atlas", fmt.Sprintf("dummy_%d_n.png", i)), norm)
		fmt.Fprintf(&y, "  s%d:\n    page: %d\n    rect: [1, 1, %d, 3]\n    trim: [%d, 2]\n    normal: %t\n", i, i, w, i, i%2 == 0)
		fmt.Fprintf(&y, "    id: \"core:dummy/s%d\"\n    image: generated/atlas/dummy_%d.png\n    scale: 2\n", i, i)
		fmt.Fprintf(&y, "    canvas: [24, 16]\n    footprint: [1, 1]\n    origin_px: [3, 4]\n")
		fmt.Fprintf(&y, "    tint:\n      group: skin\n      mask: generated/atlas/dummy_%d_mask.png\n", i)
		fmt.Fprintf(&y, "    attachments:\n      hand: [1.5, 2.5]\n")
	}
	mp := filepath.Join(root, "metadata", "atlas", "dummy.yaml")
	if err := os.MkdirAll(filepath.Dir(mp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mp, []byte(y.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestParseSpriteAtlas(t *testing.T) {
	t.Parallel()
	src := `# generated
version: 1
family: pawn
max: 4096
pages: 2
sprites:
  a:
    page: 1
    rect: [5, 6, 7, 8]
    trim: [1, 2]
    normal: true
    id: "core:pawn/a"
    image: generated/atlas/pawn_1.png
    canvas: [20, 30]
    tint:
      group: skin
      mask: generated/atlas/pawn_1_mask.png
  b:
    page: 0
    rect: [0, 0, 1, 1]
    trim: [0, 0]
    normal: false
    id: "core:pawn/b"
    image: generated/atlas/pawn_0.png
    canvas: [2, 2]
`
	fam, pages, es, err := parseSpriteAtlas(src)
	if err != nil {
		t.Fatal(err)
	}
	if fam != "pawn" || pages != 2 || len(es) != 2 {
		t.Fatalf("got %q %d %d", fam, pages, len(es))
	}
	a := es[0]
	if a.key != "a" || a.page != 1 || a.rect != image.Rect(5, 6, 12, 14) || a.trim != image.Pt(1, 2) || !a.normal {
		t.Fatalf("entry a = %+v", a)
	}
	m, err := parseArtMeta(a.meta)
	if err != nil {
		t.Fatalf("embedded metadata does not parse: %v\n%s", err, a.meta)
	}
	if m.ID != "core:pawn/a" || m.Image != "generated/atlas/pawn_1.png" || m.Canvas != [2]int{20, 30} ||
		m.TintGroup != "skin" || m.TintMask != "generated/atlas/pawn_1_mask.png" {
		t.Fatalf("meta = %+v", m)
	}
	if es[1].normal || es[1].page != 0 {
		t.Fatalf("entry b = %+v", es[1])
	}
	for _, bad := range []string{
		"pages: x\n",
		"sprites:\n  a:\n    page: x\n",
		"sprites:\n  a:\n    rect: [1, 2]\n",
		"sprites:\n  a:\n    trim: [1]\n",
		"sprites:\n  a:\n    rect: 1,2,3,4\n",
		"sprites:\n  a:\n    rect: [1, 2, 3, z]\n",
	} {
		if _, _, _, err := parseSpriteAtlas(bad); err == nil {
			t.Errorf("parseSpriteAtlas(%q) accepted bad input", bad)
		}
	}
}

func TestLoadAtlasCutsSpritesLikeFiles(t *testing.T) {
	t.Parallel()
	root := syntheticAtlas(t, 7) // more pages than atlasPageCache: eviction is exercised
	c, err := loadArtCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.sprites) != 7 {
		t.Fatalf("%d sprites, want 7", len(c.sprites))
	}
	for pass := 0; pass < 2; pass++ { // the second pass re-reads evicted pages
		for i := 0; i < 7; i++ {
			key := fmt.Sprintf("dummy/s%d", i)
			if !c.has(key) {
				t.Fatalf("%s missing", key)
			}
			sp := c.sprite(key)
			if sp.pix.Bounds().Size() != (image.Point{24, 16}) {
				t.Fatalf("%s: canvas %v, want 24x16", key, sp.pix.Bounds().Size())
			}
			if sp.meta.Origin != [2]float64{3, 4} || sp.meta.Attach["hand"] != [2]float64{1.5, 2.5} || sp.meta.TintGroup != "skin" {
				t.Fatalf("%s: metadata changed: %+v", key, sp.meta)
			}
			w := 4 + i
			for y := 0; y < 16; y++ {
				for x := 0; x < 24; x++ {
					inBlock := x >= i && x < i+w && y >= 2 && y < 5
					wantPix, wantMask := color.NRGBA{}, color.NRGBA{0, 0, 0, 255}
					if inBlock {
						px, py := x-i, y-2
						wantPix = color.NRGBA{uint8(10 * i), uint8(px), uint8(py), 255}
						g := uint8(100 + px)
						wantMask = color.NRGBA{g, g, g, 255}
					}
					if got := sp.pix.NRGBAAt(x, y); got != wantPix {
						t.Fatalf("%s pix (%d,%d) = %v, want %v", key, x, y, got, wantPix)
					}
					if got := sp.mask.NRGBAAt(x, y); got != wantMask {
						t.Fatalf("%s mask (%d,%d) = %v, want %v", key, x, y, got, wantMask)
					}
				}
			}
			n := sp.src.normalImage(sp)
			if (i%2 == 0) != (n != nil) {
				t.Fatalf("%s: normal map present = %v", key, n != nil)
			}
			if n != nil {
				// RGB from the page, alpha from the sprite; transparent flat blue elsewhere
				if n.Bounds().Size() != sp.pix.Bounds().Size() || n.NRGBAAt(i, 2) != (color.NRGBA{128, uint8(i), 255, 255}) ||
					n.NRGBAAt(0, 0) != (color.NRGBA{128, 128, 255, 0}) {
					t.Fatalf("%s: normal map wrong: %v %v", key, n.NRGBAAt(i, 2), n.NRGBAAt(0, 0))
				}
			}
		}
	}
	if len(c.sprites["dummy/s0"].src.atlas.cache) > atlasPageCache {
		t.Fatalf("page cache holds %d pages, want at most %d", len(c.sprites["dummy/s0"].src.atlas.cache), atlasPageCache)
	}
}

func TestLoadAtlasPrefersPagesOverIndividualFiles(t *testing.T) {
	t.Parallel()
	root := syntheticAtlas(t, 1)
	// an individual file for the same family that must not be read
	mp := filepath.Join(root, "metadata", "dummy", "stray.yaml")
	if err := os.MkdirAll(filepath.Dir(mp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mp, []byte("id: \"core:dummy/stray\"\nimage: generated/world/dummy/missing.png\ncanvas: [2, 2]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := loadArtCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	if c.has("dummy/stray") || !c.has("dummy/s0") {
		t.Fatal("a packed family must be served from its atlas only")
	}
}

func TestLoadAtlasRejectsBrokenAtlases(t *testing.T) {
	t.Parallel()
	for name, edit := range map[string]func(root string){
		"missing page": func(root string) { os.Remove(filepath.Join(root, "generated", "atlas", "dummy_0.png")) },
		"rect outside page": func(root string) {
			p := filepath.Join(root, "metadata", "atlas", "dummy.yaml")
			b, _ := os.ReadFile(p)
			os.WriteFile(p, bytes.Replace(b, []byte("rect: [1, 1, 4, 3]"), []byte("rect: [39, 1, 4, 3]"), 1), 0o644)
		},
		"trim beyond canvas": func(root string) {
			p := filepath.Join(root, "metadata", "atlas", "dummy.yaml")
			b, _ := os.ReadFile(p)
			os.WriteFile(p, bytes.Replace(b, []byte("trim: [0, 2]"), []byte("trim: [22, 2]"), 1), 0o644)
		},
		"wrong family": func(root string) {
			p := filepath.Join(root, "metadata", "atlas", "dummy.yaml")
			b, _ := os.ReadFile(p)
			os.WriteFile(p, bytes.Replace(b, []byte("family: dummy"), []byte("family: other"), 1), 0o644)
		},
		"corrupt page": func(root string) {
			os.WriteFile(filepath.Join(root, "generated", "atlas", "dummy_0.png"), []byte("not a png"), 0o644)
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := syntheticAtlas(t, 1)
			edit(root)
			if _, err := loadArtCatalog(root); err == nil {
				t.Fatal("want a load error")
			}
		})
	}
}

// A page that goes bad after load leaves a blank sprite, not a panic.
func TestAtlasSpriteSurvivesUnreadablePage(t *testing.T) {
	t.Parallel()
	root := syntheticAtlas(t, 1)
	c, err := loadArtCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"dummy_0.png", "dummy_0_mask.png", "dummy_0_n.png"} {
		os.WriteFile(filepath.Join(root, "generated", "atlas", f), []byte("garbage"), 0o644)
	}
	sp := c.sprite("dummy/s0")
	if sp == nil || sp.pix == nil || sp.mask == nil || sp.pix.Bounds().Size() != (image.Point{24, 16}) {
		t.Fatalf("sprite = %+v", sp)
	}
	if n := sp.src.normalImage(sp); n != nil {
		t.Fatal("an unreadable normal page must give no normal map")
	}
}

// TestAtlasCoversKeys checks the generated atlas of every packed family:
// each sprite key resolves in the catalog (including every key that still
// has individual metadata), and a sample of sprites, masks and normal
// maps cut from the pages are byte-identical to their individual files.
func TestAtlasCoversKeys(t *testing.T) {
	root, err := LocateArt()
	if err != nil {
		t.Skip("generated art not present:", err)
	}
	idxs, _ := filepath.Glob(filepath.Join(root, artMetadataRel, atlasMetaDir, "*.yaml"))
	if len(idxs) == 0 {
		t.Skip("no atlas generated (go run ./cmd/atlasgen)")
	}
	c, err := loadArtCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, idx := range idxs {
		family := strings.TrimSuffix(filepath.Base(idx), ".yaml")
		raw, err := os.ReadFile(idx)
		if err != nil {
			t.Fatal(err)
		}
		_, _, entries, err := parseSpriteAtlas(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) == 0 {
			t.Fatalf("%s: empty atlas", family)
		}
		for _, e := range entries {
			if !c.has(family + "/" + e.key) {
				t.Errorf("atlas key %s/%s does not resolve", family, e.key)
			}
		}
		// every sprite that still has its own metadata is in the atlas
		if files, err := os.ReadDir(filepath.Join(root, artMetadataRel, family)); err == nil {
			for _, f := range files {
				if k, ok := strings.CutSuffix(f.Name(), ".yaml"); ok && !c.has(family+"/"+k) {
					t.Errorf("%s/%s has metadata but is not served by the catalog", family, k)
				}
			}
		}
		// pixels from the pages equal the individual files
		step := max(1, len(entries)/16)
		compared := 0
		for i := 0; i < len(entries); i += step {
			e := entries[i]
			sp := c.sprite(family + "/" + e.key)
			file := filepath.Join(root, "generated", "world", family, e.key+".png")
			if _, err := os.Stat(file); err != nil {
				continue // individual files pruned: nothing to compare with
			}
			want, err := decodeNRGBA(file)
			if err != nil {
				t.Fatal(err)
			}
			if sp.pix.Bounds() != want.Bounds() || !bytes.Equal(sp.pix.Pix, want.Pix) {
				t.Errorf("%s/%s: atlas pixels differ from %s", family, e.key, file)
			}
			if sp.mask != nil {
				wm, err := decodeNRGBA(strings.TrimSuffix(file, ".png") + "_mask.png")
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(sp.mask.Pix, wm.Pix) {
					t.Errorf("%s/%s: atlas mask differs from its file", family, e.key)
				}
			}
			nf := strings.TrimSuffix(file, ".png") + "_n.png"
			if wn, err := decodeNRGBA(nf); err == nil {
				n := sp.src.normalImage(sp)
				if n == nil || n.Bounds() != wn.Bounds() {
					t.Errorf("%s/%s: atlas normal map missing or the wrong size", family, e.key)
				} else if !sameVisible(n, wn) {
					t.Errorf("%s/%s: atlas normal map differs from its file", family, e.key)
				}
			} else if sp.src.entry.normal {
				t.Errorf("%s/%s: atlas has a normal map the file lacks", family, e.key)
			}
			compared++
		}
		t.Logf("%s: %d sprites, %d compared with their files", family, len(entries), compared)
	}
}

// sameVisible reports whether a and b agree on alpha and, where a pixel is
// not fully transparent, on colour: the RGB of a transparent pixel is never
// drawn (Ebiten premultiplies on upload).
func sameVisible(a, b *image.NRGBA) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for i := 0; i < len(a.Pix); i += 4 {
		if a.Pix[i+3] != b.Pix[i+3] || (a.Pix[i+3] != 0 && !bytes.Equal(a.Pix[i:i+3], b.Pix[i:i+3])) {
			return false
		}
	}
	return true
}
