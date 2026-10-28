package ebiten

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// --- floor stains (C1) ------------------------------------------------------

func TestStainShapeByAmount(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		amount   float64
		shape    stainShape
		min, max float64
	}{
		{0.05, stainDrip, 0.5, 0.5},
		{0.19, stainDrip, 0.5, 0.5},
		{0.2, stainSplat, 0.55, 0.55},
		{0.4, stainSplat, 0.55, 0.9},
		{0.599, stainSplat, 0.89, 0.9},
		{0.6, stainPool, 0.8, 0.8},
		{0.8, stainPool, 0.8, 1.1},
		{1, stainPool, 1.1, 1.1},
	} {
		shape, size := stainShapeOf(c.amount)
		if shape != c.shape || size < c.min-1e-9 || size > c.max+1e-9 {
			t.Errorf("amount %v: shape %v size %v, want %v in [%v, %v]", c.amount, shape, size, c.shape, c.min, c.max)
		}
	}
	_, a := stainShapeOf(0.3)
	_, b := stainShapeOf(0.5)
	if a >= b {
		t.Fatalf("splats grow with amount: %v then %v", a, b)
	}
}

func TestStainAlphaIsAmount(t *testing.T) {
	t.Parallel()
	for _, a := range []float64{0, 0.1, 0.5, 1} {
		l := stainLookOf(game.StainView{X: 3, Y: 4, Kind: "blood", Amount: a})
		if math.Abs(float64(l.cs[3])-a*0.85) > 1e-6 {
			t.Fatalf("amount %v: alpha %v", a, l.cs[3])
		}
	}
	if l := stainLookOf(game.StainView{Amount: 3}); l.cs[3] > 0.85+1e-6 {
		t.Fatalf("alpha capped at 0.85: %v", l.cs[3])
	}
}

func rgb8(c [3]float32) [3]int {
	return [3]int{int(math.Round(float64(c[0]) * 255)), int(math.Round(float64(c[1]) * 255)), int(math.Round(float64(c[2]) * 255))}
}

func TestStainDriesDarker(t *testing.T) {
	t.Parallel()
	if c := rgb8(stainColour("blood", 0)); c != [3]int{0x7a, 0x14, 0x14} {
		t.Fatalf("fresh blood %x", c)
	}
	if c := rgb8(stainColour("blood", stainDryTicks)); c != [3]int{0x4a, 0x1d, 0x1a} {
		t.Fatalf("dried blood %x", c)
	}
	if stainColour("blood", 10*stainDryTicks) != stainColour("blood", stainDryTicks) {
		t.Fatal("dry after a day")
	}
	prev := float32(2)
	for age := uint64(0); age <= stainDryTicks; age += stainDryTicks / 8 {
		l := lum3(stainColour("blood", age))
		if l > prev+1e-6 {
			t.Fatalf("darkens as it dries: %v after %v at %d", l, prev, age)
		}
		prev = l
	}
	// a stain's look carries the colour (the tint), premultiplied later
	l := stainLookOf(game.StainView{Kind: "blood", Amount: 0.5, AgeTicks: stainDryTicks})
	if rgb8([3]float32{l.cs[0], l.cs[1], l.cs[2]}) != [3]int{0x4a, 0x1d, 0x1a} {
		t.Fatalf("look colour %v", l.cs)
	}
}

func TestIchorColour(t *testing.T) {
	t.Parallel()
	if c := rgb8(stainColour("ichor", 0)); c != [3]int{0xa8, 0xa0, 0x30} {
		t.Fatalf("fresh ichor %x", c)
	}
	if c := rgb8(stainColour("ichor", stainDryTicks)); c != [3]int{0x5c, 0x5a, 0x1c} {
		t.Fatalf("dried ichor %x", c)
	}
}

func TestStainRotationAndOffset(t *testing.T) {
	t.Parallel()
	for v := uint8(0); v < 6; v++ {
		l := stainLookOf(game.StainView{X: 7, Y: 9, Amount: 0.4, Variant: v})
		if math.Abs(l.rot-float64(v)*math.Pi/3) > 1e-9 {
			t.Fatalf("variant %d: rot %v", v, l.rot)
		}
		if math.Abs(l.cx-7.5) > 0.15+1e-9 || math.Abs(l.cy-9.5) > 0.15+1e-9 {
			t.Fatalf("offset within ±0.15 tile: (%v, %v)", l.cx, l.cy)
		}
	}
	a := stainLookOf(game.StainView{X: 7, Y: 9, Amount: 0.4})
	b := stainLookOf(game.StainView{X: 8, Y: 9, Amount: 0.4})
	if a.cx-7 == b.cx-8 && a.cy == b.cy {
		t.Fatal("offset varies by tile")
	}
}

func TestStainTexturesDeterministic(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, c := range []struct {
		shape stainShape
		n, px int
	}{{stainSplat, 6, 128}, {stainPool, 3, 160}, {stainDrip, 4, 64}} {
		for v := 0; v < c.n; v++ {
			a, b := stainPixels(c.shape, v), stainPixels(c.shape, v)
			if a.Bounds() != image.Rect(0, 0, c.px, c.px) {
				t.Fatalf("shape %v: %v", c.shape, a.Bounds())
			}
			if !bytes.Equal(a.Pix, b.Pix) {
				t.Fatalf("shape %v variant %d differs between builds", c.shape, v)
			}
			if seen[string(a.Pix)] {
				t.Fatalf("shape %v variant %d repeats another texture", c.shape, v)
			}
			seen[string(a.Pix)] = true
			var cover, edge int
			for y := 0; y < c.px; y++ {
				for x := 0; x < c.px; x++ {
					al := a.NRGBAAt(x, y).A
					if al > 0 {
						cover++
						if x == 0 || y == 0 || x == c.px-1 || y == c.px-1 {
							edge++
						}
					}
				}
			}
			if cover == 0 || edge > 0 {
				t.Fatalf("shape %v variant %d: %d covered, %d on the border", c.shape, v, cover, edge)
			}
		}
	}
	// pools are smooth blobs with a darker rim
	p := stainPixels(stainPool, 0)
	mid := p.NRGBAAt(80, 80)
	if mid.A == 0 || mid.R < 240 {
		t.Fatalf("pool centre %v", mid)
	}
	darkest := uint8(255)
	for x := 80; x < 160; x++ {
		if c := p.NRGBAAt(x, 80); c.A > 128 && c.R < darkest {
			darkest = c.R
		}
	}
	if darkest > 200 {
		t.Fatalf("pool rim not darker: %d", darkest)
	}
}

// stainRenderer is a renderer over a 64×64 map with the camera at the
// origin: 32 px tiles on a 320×320 screen show tiles 0..9.
func stainRenderer() *Renderer {
	r := New(&stubSim{w: 64, h: 64, tick: 100})
	r.cam.Zoom = 32.0 / float64(TileSize)
	return r
}

func TestOffscreenStainsSkipped(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	tile := r.tilePx()
	w := game.WorldView{Stains: []game.StainView{
		{X: 2, Y: 2, Kind: "blood", Amount: 0.5},
		{X: 9, Y: 5, Kind: "ichor", Amount: 0.1},
		{X: 50, Y: 50, Kind: "blood", Amount: 1},
		{X: 3, Y: 40, Kind: "blood", Amount: 1},
		{X: 4, Y: 4, Kind: "blood", Amount: 0}, // faded away
	}}
	r.view.W = w
	var b stainBatch
	if n := r.buildStains(&b, w, 320, 320, tile); n != 2 || len(b.verts) != 8 || len(b.idx) != 12 {
		t.Fatalf("on screen: %d quads, %d verts, %d indices", n, len(b.verts), len(b.idx))
	}
	for _, v := range b.verts {
		if v.Custom3 != 0 || v.Custom2 != 0 {
			t.Fatalf("stains are lit flat and carry no figure blood: %+v", v)
		}
	}
	screen := ebiten.NewImage(320, 320)
	r.drawStains(screen, w, 320, 320, tile) // one batch, no panic
	r.drawStains(screen, game.WorldView{}, 320, 320, tile)
}

func TestStainsLitByTheirTile(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	w := roomWorld()
	w.Stains = []game.StainView{{X: 0, Y: 0, Amount: 0.5}}
	r.view.W = w
	var b stainBatch
	r.buildStains(&b, w, 320, 320, r.tilePx())
	sky, lamp := r.lightTile(0, 0)
	if len(b.verts) != 4 || b.verts[0].Custom0 != sky || b.verts[0].Custom1 != lamp {
		t.Fatalf("stain lit as its tile (%v, %v): %+v", sky, lamp, b.verts)
	}
}

// --- blood on creatures (C2) --------------------------------------------------

func TestCustom3Encoding(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		cov   float64
		wet   int
		ichor bool
	}{{0.1, 0, false}, {0.75, 15, false}, {0.5, 7, true}, {0.95, 15, true}, {0.01, 1, false}} {
		w := encodeBlood(c.cov, c.wet, c.ichor)
		if want := float64(32*b2i(c.ichor)+2*c.wet) + c.cov; math.Abs(float64(w)-want) > 1e-5 {
			t.Fatalf("%+v: w %v, want %v", c, w, want)
		}
		cov, wet, ichor := decodeBlood(w)
		if math.Abs(cov-c.cov) > 1e-5 || wet != c.wet || ichor != c.ichor {
			t.Fatalf("%+v round trip: %v %v %v", c, cov, wet, ichor)
		}
	}
	if encodeBlood(0, 15, true) != 0 || encodeBlood(-1, 3, false) != 0 {
		t.Fatal("no coverage: no blood")
	}
	if cov, wet, _ := decodeBlood(encodeBlood(2, 99, false)); cov >= 1 || wet != 15 {
		t.Fatalf("clamped: %v %v", cov, wet)
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestCoverageFromHealthAndWound(t *testing.T) {
	t.Parallel()
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	if c := pawnBloodCoverage(game.PawnView{Health: 100}); c != 0 {
		t.Fatalf("healthy: %v", c)
	}
	if c := pawnBloodCoverage(game.PawnView{Health: 50}); !near(c, 0.4) {
		t.Fatalf("half health: %v", c)
	}
	if c := pawnBloodCoverage(game.PawnView{Health: 50, Wounded: true}); !near(c, 0.6) {
		t.Fatalf("open wound: %v", c)
	}
	if c := pawnBloodCoverage(game.PawnView{Health: 0, Wounded: true}); !near(c, 0.75) {
		t.Fatalf("capped: %v", c)
	}
	if c := animalBloodCoverage(game.AnimalView{Health: 70}); !near(c, 0.3) {
		t.Fatalf("animal: %v", c)
	}
	if c := animalBloodCoverage(game.AnimalView{Health: 5}); !near(c, 0.75) {
		t.Fatalf("animal capped: %v", c)
	}
	if !isHiveSpecies("hive_beetle") || !isHiveSpecies("hive_brute") || !isHiveSpecies("hive_crawler") || isHiveSpecies("muffalo") {
		t.Fatal("hive species")
	}
}

func TestFreshHitWetFades(t *testing.T) {
	t.Parallel()
	if w, add := bloodWet(0, true); w != 15 || add != 0.2 {
		t.Fatalf("just hit: %v %v", w, add)
	}
	if w, _ := bloodWet(bloodWetTicks/2, true); w != 8 {
		t.Fatalf("half way: %v", w)
	}
	if w, add := bloodWet(bloodWetTicks, true); w != 0 || add != 0 {
		t.Fatalf("dry after 2 s: %v %v", w, add)
	}
	if w, add := bloodWet(0, false); w != 0 || add != 0 {
		t.Fatal("never hit")
	}
	prev := 16
	for a := 0.0; a < bloodWetTicks; a += 10 {
		w, _ := bloodWet(a, true)
		if w > prev {
			t.Fatalf("wet fades: %d after %d", w, prev)
		}
		prev = w
	}
	// through the renderer: a pawn hit 30 ticks ago is wet, one hit long ago is not
	r := New(&stubSim{w: 16, h: 16, tick: 1000})
	_, wet, _ := decodeBlood(r.pawnBlood(game.PawnView{ID: 1, Health: 60, HitTick: 970}))
	if wet != 11 {
		t.Fatalf("hit 30 ticks ago: wet %d", wet)
	}
	cov, wet, _ := decodeBlood(r.pawnBlood(game.PawnView{ID: 1, Health: 60, HitTick: 100}))
	if wet != 0 || math.Abs(cov-0.32) > 1e-5 {
		t.Fatalf("old hit: %v %v", cov, wet)
	}
	if _, _, ichor := decodeBlood(r.animalBlood(game.AnimalView{ID: 2, Species: "hive_brute", Health: 40})); !ichor {
		t.Fatal("hive animals bleed ichor")
	}
	if r.animalBlood(game.AnimalView{ID: 2, Species: "muffalo", Health: 100}) != 0 {
		t.Fatal("healthy animal clean")
	}
	if r.pawnBlood(game.PawnView{ID: 3, Health: 10, CarriedBy: 1}) != 0 {
		t.Fatal("a carried patient stays clean")
	}
}

// bloodArt is figureArt plus an animal and a ground sprite.
func bloodArt() *artCatalog {
	c := figureArt()
	for f := faceSouth; f <= faceWest; f++ {
		c.Sprites["animal/muffalo_"+f.String()] = fakeSprite([2]int{160, 160}, pt{16, 16}, nil)
	}
	return c
}

// litLog records the Custom3 value of every lit draw by sprite key.
func litLog(r *Renderer) map[string][]float32 {
	got := map[string][]float32{}
	r.lit.OnDraw = func(img *ebiten.Image, blood float32) {
		k := keyOf(r.art, img)
		got[k] = append(got[k], blood)
	}
	return got
}

func TestHairAndHeldStayClean(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 64, h: 64, tick: 100})
	r.art = bloodArt()
	got := litLog(r)
	p := game.PawnView{ID: 1, X: 2, Y: 2, Health: 30, Wounded: true, Weapon: "spear", Drafted: true, Status: "alive"}
	patient := game.PawnView{ID: 2, X: 2, Y: 2, Health: 20, CarriedBy: 1, Status: "downed"}
	r.view.Pawns = []game.PawnView{p, patient}
	r.drawPawnFigure(ebiten.NewImage(512, 512), p, 2, 2, 64, r.pawnFigurePoseOf(p))
	want := encodeBlood(0.75, 0, false)
	var body, cloth, hair, held int
	for k, vs := range got {
		for _, v := range vs {
			switch {
			case strings.HasPrefix(k, "pawn/body_"):
				// the carried patient's body is drawn clean
				if v == want {
					body++
				} else if v != 0 {
					t.Fatalf("%s: %v", k, v)
				}
			case strings.HasPrefix(k, "pawn/shirt_"), strings.HasPrefix(k, "pawn/pants_"):
				if v != want && v != 0 {
					t.Fatalf("%s: %v", k, v)
				}
				if v == want {
					cloth++
				}
			case strings.HasPrefix(k, "pawn/hair_"):
				if v != 0 {
					t.Fatalf("hair bloodied: %v", v)
				}
				hair++
			case strings.HasPrefix(k, "weapon/"):
				if v != 0 {
					t.Fatalf("held item bloodied: %v", v)
				}
				held++
			}
		}
	}
	if body != 1 || cloth != 2 || hair < 1 || held < 1 {
		t.Fatalf("draws: body %d cloth %d hair %d held %d (%v)", body, cloth, hair, held, got)
	}
	if r.figureBlood != 0 {
		t.Fatal("figureBlood reset after the figure")
	}
	// lying (downed) pawns are bloodied too, hair still clean
	for k := range got {
		delete(got, k)
	}
	down := game.PawnView{ID: 3, X: 2, Y: 2, Health: 30, Lying: true, Status: "downed"}
	r.drawPawnFigure(ebiten.NewImage(512, 512), down, 2, 2, 64, r.pawnFigurePoseOf(down))
	bloody := 0
	for k, vs := range got {
		for _, v := range vs {
			if strings.HasPrefix(k, "pawn/hair_") && v != 0 {
				t.Fatalf("lying hair bloodied: %v", v)
			}
			if strings.HasPrefix(k, "pawn/body_") && v != 0 {
				bloody++
			}
		}
	}
	if bloody == 0 || r.figureBlood != 0 {
		t.Fatalf("lying body bloodied (%d), reset after (%v): %v", bloody, r.figureBlood, got)
	}
}

func TestNonFigureDrawsHaveNoBlood(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 64, h: 64, tick: 100})
	r.art = bloodArt()
	got := litLog(r)
	dst := ebiten.NewImage(512, 512)
	an := game.AnimalView{ID: 9, X: 3, Y: 3, Species: "muffalo", Health: 50}
	r.drawAnimalFigure(dst, an, 3, 3, 64)
	if r.figureBlood != 0 {
		t.Fatal("reset after the animal")
	}
	r.drawSpritePose(dst, "item/wood_pile", color.NRGBA{}, 1, 1, 64, pose{})
	var animal []float32
	for k, vs := range got {
		if strings.HasPrefix(k, "animal/") {
			animal = append(animal, vs...)
		}
	}
	if len(animal) != 1 || animal[0] != encodeBlood(0.5, 0, false) {
		t.Fatalf("animal figure blood %v (%v)", animal, got)
	}
	if v := got["item/wood_pile"]; len(v) != 1 || v[0] != 0 {
		t.Fatalf("items carry no blood: %v", v)
	}
	// the shadow pass draws silhouettes: no lit draws, no blood
	for k := range got {
		delete(got, k)
	}
	r.cast = &shadowCast{}
	r.drawAnimalFigure(dst, an, 3, 3, 64)
	r.cast = nil
	if len(got) != 0 || r.figureBlood != 0 {
		t.Fatalf("cast mode: %v %v", got, r.figureBlood)
	}
}

func TestDrawLitWritesCustom3(t *testing.T) {
	t.Parallel()
	r := &Renderer{}
	r.view.W = roomWorld()
	img := ebiten.NewImage(4, 4)
	dst := ebiten.NewImage(16, 16)
	r.figureBlood = encodeBlood(0.4, 3, true)
	r.drawLit(dst, img, ebiten.GeoM{}, noCS, 0, 0)
	for _, v := range r.lit.Verts {
		if v.Custom3 != r.figureBlood {
			t.Fatalf("Custom3 %v, want %v", v.Custom3, r.figureBlood)
		}
	}
	r.figureBlood = 0
	r.drawLit(dst, img, ebiten.GeoM{}, noCS, 0, 0)
	for _, v := range r.lit.Verts {
		if v.Custom3 != 0 {
			t.Fatalf("Custom3 %v after reset", v.Custom3)
		}
	}
}

func TestBloodShaderCompiles(t *testing.T) {
	t.Parallel()
	s, err := ebiten.NewShader(litKage)
	if err != nil {
		t.Fatalf("lit.kage: %v", err)
	}
	s.Deallocate()
	src := string(litKage)
	for _, banned := range []string{"imageSrc1", "imageSrc0Origin", "imageSrc0Size", "imageSrcRegion"} {
		if strings.Contains(src, banned) {
			t.Fatalf("lit.kage uses %s: it would break batching", banned)
		}
	}
	if !strings.Contains(src, "bloodNoise(") || !strings.Contains(src, "custom.w > 0") {
		t.Fatal("lit.kage decodes custom.w into blood blotches")
	}
	// draw through it with blood set: compiles and runs on this backend
	r := &Renderer{}
	r.view.W = roomWorld()
	r.lit.Light = lightAt(12, summerDay, "clear", 0)
	r.figureBlood = encodeBlood(0.6, 10, false)
	img := ebiten.NewImage(8, 8)
	img.Fill(color.White)
	r.drawLit(ebiten.NewImage(16, 16), img, ebiten.GeoM{}, noCS, 0, 0)
	if r.lit.Err != nil {
		t.Fatal(r.lit.Err)
	}
}

func TestBloodMaskGrowsWithCoverage(t *testing.T) {
	t.Parallel()
	share := func(cov float32) float64 {
		var sum float64
		n := 0
		for y := 0; y < 96; y++ {
			for x := 0; x < 96; x++ {
				sum += float64(bloodMask([2]float32{float32(x) + 0.5, float32(y) + 0.5}, cov))
				n++
			}
		}
		return sum / float64(n)
	}
	if s := share(0); s != 0 {
		t.Fatalf("no coverage: %v", s)
	}
	prev := 0.0
	for _, c := range []float32{0.1, 0.2, 0.35, 0.5, 0.75, 0.95} {
		s := share(c)
		if s < prev {
			t.Fatalf("coverage %v: share %v below %v", c, s, prev)
		}
		prev = s
	}
	if s := share(0.2); s <= 0 || s > 0.3 {
		t.Fatalf("light wounds: a few blotches, got %v", s)
	}
	if s := share(0.75); s < 0.5 {
		t.Fatalf("near death: mostly covered, got %v", s)
	}
	for x := float32(0); x < 64; x += 0.7 {
		if m := bloodMask([2]float32{x, x * 1.3}, 0.5); m < 0 || m > 1 {
			t.Fatalf("mask %v", m)
		}
	}
}

// --- sprays (C3) -------------------------------------------------------------

// sprayParts runs one hit trace and returns the particles of kind.
func sprayParts(t *testing.T, tr game.TraceView, w game.WorldView, kind partKind) []particle {
	t.Helper()
	w.Traces = []game.TraceView{tr}
	r := combatRenderer(100, nil, w)
	r.animateCombat(w, nil)
	var out []particle
	for _, p := range r.parts {
		if p.Kind == kind {
			out = append(out, p)
		}
	}
	return out
}

func TestSprayScalesWithDamage(t *testing.T) {
	t.Parallel()
	shot := func(dmg int, melee bool) game.TraceView {
		return game.TraceView{FromX: 2, FromY: 5, ToX: 6, ToY: 5, Age: 1, Hit: true, Melee: melee, Damage: dmg, Attacker: 1, Target: 2}
	}
	for _, c := range []struct{ dmg, n int }{{0, 4}, {1, 4}, {9, 8}, {10, 9}, {24, 16}, {60, 16}} {
		if got := len(sprayParts(t, shot(c.dmg, false), game.WorldView{}, partBlood)); got != c.n {
			t.Fatalf("damage %d: %d drops, want %d", c.dmg, got, c.n)
		}
	}
	light, heavy := sprayParts(t, shot(9, false), game.WorldView{}, partBlood), sprayParts(t, shot(10, false), game.WorldView{}, partBlood)
	for i := range light {
		if d := heavy[i].Vx - light[i].Vx; math.Abs(d-0.25*bloodSpeed) > 1e-9 {
			t.Fatalf("drop %d: heavy hits spray 1.25× faster (Δvx %v)", i, d)
		}
	}
	// a melee hit flicks 2 drops back toward the attacker (west)
	m := sprayParts(t, shot(9, true), game.WorldView{}, partBlood)
	if len(m) != 10 {
		t.Fatalf("melee: %d drops", len(m))
	}
	back := 0
	for _, p := range m[8:] {
		if p.Vx < 0 {
			back++
		}
	}
	if back != 2 {
		t.Fatalf("flicked back: %d of %v", back, m[8:])
	}
}

func TestInsectSprayIsIchor(t *testing.T) {
	t.Parallel()
	tr := game.TraceView{FromX: 2, FromY: 5, ToX: 6, ToY: 5, Age: 1, Hit: true, Melee: true, Damage: 6, Attacker: 1, Target: 2}
	w := game.WorldView{Animals: []game.AnimalView{{ID: 2, X: 6, Y: 5, Species: "hive_crawler"}}}
	if n := len(sprayParts(t, tr, w, partIchor)); n != 9 {
		t.Fatalf("ichor drops %d", n)
	}
	if n := len(sprayParts(t, tr, w, partBlood)); n != 0 {
		t.Fatalf("insects do not bleed red: %d", n)
	}
	w.Animals[0].Species = "muffalo"
	if n := len(sprayParts(t, tr, w, partBlood)); n != 9 {
		t.Fatalf("muffalo bleeds blood: %d", n)
	}
}
