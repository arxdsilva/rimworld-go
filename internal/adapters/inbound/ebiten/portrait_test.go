package ebiten

import (
	"runtime"
	"sync"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/pawn/look"
)

var (
	realArtOnce sync.Once
	realArtCat  *artCatalog
	realArtErr  error

	atlasArtOnce sync.Once
	atlasArtCat  *artCatalog
	atlasArtErr  error
)

// realArt indexes the generated art catalog once, or skips. It is shared
// by parallel tests: key lookups and the (once-guarded) lazy decode are
// safe, but drawing also writes texture caches, so a test that draws
// takes its own testArt.
func realArt(t *testing.T) *artCatalog {
	t.Helper()
	realArtOnce.Do(func() {
		root, err := LocateArt()
		if err != nil {
			realArtErr = err
			return
		}
		realArtCat, realArtErr = indexArtCatalog(root, true)
	})
	if realArtErr != nil {
		t.Skip(realArtErr)
	}
	return realArtCat
}

// atlasArt is the game's view of the generated art, packed families
// served from their atlas pages, shared by the tests that need the pages
// themselves. Every page layer is cut once up front on all cores
// (cutPageAside is safe per page and layer): one by one under the race
// detector that takes minutes.
func atlasArt(t *testing.T) *artCatalog {
	t.Helper()
	atlasArtOnce.Do(func() {
		root, err := LocateArt()
		if err != nil {
			atlasArtErr = err
			return
		}
		if atlasArtCat, atlasArtErr = indexArtCatalog(root, false); atlasArtErr != nil {
			return
		}
		type job struct {
			a         *spriteAtlas
			pg, layer int
		}
		jobs := make(chan job)
		var wg sync.WaitGroup
		for range runtime.GOMAXPROCS(0) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var z deflater
				for j := range jobs {
					j.a.CutPageAside(j.pg, j.layer, &z)
				}
			}()
		}
		seen := map[*spriteAtlas]bool{}
		for _, sp := range atlasArtCat.Sprites {
			if sp.Src == nil || seen[sp.Src.Atlas] {
				continue
			}
			a := sp.Src.Atlas
			seen[a] = true
			for pg := range a.ByPage {
				for layer := 0; layer < atlasLayers; layer++ {
					jobs <- job{a, pg, layer}
				}
			}
		}
		close(jobs)
		wg.Wait()
	})
	if atlasArtErr != nil {
		t.Skip(atlasArtErr)
	}
	return atlasArtCat
}

// testArt is a fresh lazy index of the generated art for one test that
// draws, or skips: it decodes only the sprites the test hands out, which
// keeps the race-enabled suite fast.
func testArt(t testing.TB) *artCatalog {
	t.Helper()
	root, err := LocateArt()
	if err != nil {
		t.Skip("generated art not present:", err)
	}
	c, err := indexArtCatalog(root, true)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPortraitWithoutRenderer(t *testing.T) {
	t.Parallel()
	art := atlasArt(t) // portrait builds live only in the pawn atlas
	dst := ebiten.NewImage(200, 240)
	for _, sex := range []look.Sex{look.Male, look.Female} {
		for _, build := range []look.Build{look.Average, look.Thin, look.Heavy} {
			l := look.Look{Sex: sex, Build: build, Skin: 2, Hair: 1, HairStyle: 3, Beard: 1, Shirt: 1, Pants: 2, Vest: true, VestCloth: 4}
			layers := portraitLayers(art, l)
			if len(layers) < 5 {
				t.Fatalf("%v: %d layers", l.BuildKey(), len(layers))
			}
			for _, ly := range layers {
				if !art.Has(ly.Key) && !(ly.Key[:11] == "pawn/beard_" && sex == look.Female) {
					t.Errorf("%s: no art for %q", l.BuildKey(), ly.Key)
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
			m[ly.Key] = true
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
	if last := grey[len(grey)-1]; last.Tint != art.Tint("hair", "grey") {
		t.Errorf("grey hair tint %v", last.Tint)
	}
	if pick(nil, 3) != "" || pick([]string{"a", "b"}, 3) != "b" {
		t.Error("palette indices wrap")
	}
}
