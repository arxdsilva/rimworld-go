package ebiten

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
)

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
	c := atlasArt(t)
	for _, idx := range idxs {
		family := strings.TrimSuffix(filepath.Base(idx), ".yaml")
		raw, err := os.ReadFile(idx)
		if err != nil {
			t.Fatal(err)
		}
		_, _, entries, err := art.ParseAtlas(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) == 0 {
			t.Fatalf("%s: empty atlas", family)
		}
		for _, e := range entries {
			if !c.Has(family + "/" + e.Key) {
				t.Errorf("atlas key %s/%s does not resolve", family, e.Key)
			}
		}
		// every sprite that still has its own metadata is in the atlas
		if files, err := os.ReadDir(filepath.Join(root, artMetadataRel, family)); err == nil {
			for _, f := range files {
				if k, ok := strings.CutSuffix(f.Name(), ".yaml"); ok && !c.Has(family+"/"+k) {
					t.Errorf("%s/%s has metadata but is not served by the catalog", family, k)
				}
			}
		}
		// pixels from the pages equal the individual files
		step := max(1, len(entries)/16)
		compared := 0
		for i := 0; i < len(entries); i += step {
			e := entries[i]
			sp := c.Sprite(family + "/" + e.Key)
			file := filepath.Join(root, "generated", "world", family, e.Key+".png")
			if _, err := os.Stat(file); err != nil {
				continue // individual files pruned: nothing to compare with
			}
			want, err := decodeNRGBA(file)
			if err != nil {
				t.Fatal(err)
			}
			if sp.Pix.Bounds() != want.Bounds() || !bytes.Equal(sp.Pix.Pix, want.Pix) {
				t.Errorf("%s/%s: atlas pixels differ from %s", family, e.Key, file)
			}
			if sp.Mask != nil {
				wm, err := decodeNRGBA(strings.TrimSuffix(file, ".png") + "_mask.png")
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(sp.Mask.Pix, wm.Pix) {
					t.Errorf("%s/%s: atlas mask differs from its file", family, e.Key)
				}
			}
			nf := strings.TrimSuffix(file, ".png") + "_n.png"
			if wn, err := decodeNRGBA(nf); err == nil {
				n := sp.Src.NormalImage(sp)
				if n == nil || n.Bounds() != wn.Bounds() {
					t.Errorf("%s/%s: atlas normal map missing or the wrong size", family, e.Key)
				} else if !sameVisible(n, wn) {
					t.Errorf("%s/%s: atlas normal map differs from its file", family, e.Key)
				}
			} else if sp.Src.Entry.Normal {
				t.Errorf("%s/%s: atlas has a normal map the file lacks", family, e.Key)
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
