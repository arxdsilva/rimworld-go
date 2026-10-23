package ebiten

import (
	"sync"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/pawn/look"
)

var (
	realArtOnce sync.Once
	realArtCat  *artCatalog
	realArtErr  error
)

// realArt loads the generated art catalog once, or skips.
func realArt(t *testing.T) *artCatalog {
	t.Helper()
	realArtOnce.Do(func() {
		root, err := LocateArt()
		if err != nil {
			realArtErr = err
			return
		}
		realArtCat, realArtErr = loadArtCatalog(root)
	})
	if realArtErr != nil {
		t.Skip(realArtErr)
	}
	return realArtCat
}

func TestPortraitWithoutRenderer(t *testing.T) {
	t.Parallel()
	art := realArt(t)
	dst := ebiten.NewImage(200, 240)
	for _, sex := range []look.Sex{look.Male, look.Female} {
		for _, build := range []look.Build{look.Average, look.Thin, look.Heavy} {
			l := look.Look{Sex: sex, Build: build, Skin: 2, Hair: 1, HairStyle: 3, Beard: 1, Shirt: 1, Pants: 2, Vest: true, VestCloth: 4}
			layers := portraitLayers(art, l)
			if len(layers) < 5 {
				t.Fatalf("%v: %d layers", l.BuildKey(), len(layers))
			}
			for _, ly := range layers {
				if !art.has(ly.key) && !(ly.key[:11] == "pawn/beard_" && sex == look.Female) {
					t.Errorf("%s: no art for %q", l.BuildKey(), ly.key)
				}
			}
			if !drawLookPortrait(dst, art, l, 20, 20, 160, 200) {
				t.Errorf("%s: nothing drawn", l.BuildKey())
			}
		}
	}
	if drawLookPortrait(dst, nil, look.Look{}, 0, 0, 10, 10) {
		t.Error("no art, no portrait: the caller draws a stand-in")
	}
}

func TestPortraitKeysFollowTheBuild(t *testing.T) {
	t.Parallel()
	art := realArt(t)
	keys := func(l look.Look) map[string]bool {
		m := map[string]bool{}
		for _, ly := range portraitLayers(art, l) {
			m[ly.key] = true
		}
		return m
	}
	m := keys(look.Look{Sex: look.Male, Build: look.Average, HairStyle: 0})
	if !m["pawn/body_south"] || !m["pawn/pants_basic_south"] || !m["pawn/shirt_basic_south"] || !m["pawn/hair_short_south"] {
		t.Errorf("male average uses the unsuffixed keys: %v", m)
	}
	f := keys(look.Look{Sex: look.Female, Build: look.Heavy, HairStyle: 3, Vest: true})
	for _, want := range []string{"pawn/body_f_heavy_south", "pawn/pants_basic_f_heavy_south", "pawn/shirt_basic_f_heavy_south", "pawn/vest_utility_f_heavy_south", "pawn/hair_long_south"} {
		if !f[want] {
			t.Errorf("female heavy lacks %q: %v", want, f)
		}
	}
	b := keys(look.Look{Sex: look.Male, Build: look.Thin, Bare: true, Beard: 2})
	if b["pawn/shirt_basic_m_thin_south"] || b["pawn/pants_basic_m_thin_south"] || !b["pawn/beard_full_south"] {
		t.Errorf("bare, bearded, thin: %v", b)
	}
	// Tints come from the catalog's groups; a grey look uses the grey hair.
	grey := portraitLayers(art, look.Look{Grey: true, HairStyle: 0})
	if last := grey[len(grey)-1]; last.tint != art.tint("hair", "grey") {
		t.Errorf("grey hair tint %v", last.tint)
	}
	if pick(nil, 3) != "" || pick([]string{"a", "b"}, 3) != "b" {
		t.Error("palette indices wrap")
	}
}
