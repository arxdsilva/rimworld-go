package ebiten

import (
	"fmt"
	"math"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// --- fixtures ---------------------------------------------------------------

// envSim is a motion sim (scripted walkers) that also reports a fixture
// world: plants, season and weather.
type envSim struct {
	*motionSim
	world game.WorldView
}

func (s *envSim) World() game.WorldView {
	w := s.world
	w.Animals = s.motionSim.World().Animals
	return w
}

func newEnvSim(w game.WorldView, movers ...*scriptMover) *envSim {
	return &envSim{motionSim: newMotionSim(movers...), world: w}
}

// envRenderer is a renderer whose environment has seen w once.
func envRenderer(w game.WorldView) *Renderer {
	r := New(newEnvSim(w))
	r.stepEnvironment(w, nil)
	return r
}

// leanEast is how far a pose tips its top east, in radians. GeoM.Skew
// shears x by tan(skew)*y about the pivot; above the pivot y < 0, so a
// negative skew moves the top east.
func leanEast(ps pose) float64 { return -ps.skew }

// envArt is a catalog holding just these sprite keys.
func envArt(keys ...string) *artCatalog {
	c := &artCatalog{sprites: map[string]*artSprite{}, tinted: map[tintKey]*ebiten.Image{}}
	for _, k := range keys {
		c.sprites[k] = fakeSprite([2]int{128, 128}, pt{0, 0}, nil)
	}
	return c
}

func windy(wx, wy float64) game.WorldView {
	return game.WorldView{Weather: "cloudy", WeatherIntensity: 0.2, WindX: wx, WindY: wy, Season: "Summer"}
}

func stepFrames(r *Renderer, w game.WorldView, n int) {
	for i := 0; i < n; i++ {
		r.present++
		r.stepEnvironment(w, nil)
	}
}

// --- wind ---------------------------------------------------------------------

func TestWindFieldFollowsWorldWind(t *testing.T) {
	t.Parallel()
	for _, wind := range [][2]float64{{2, 0}, {-1, 1.5}, {0, -3}} {
		r := envRenderer(windy(wind[0], wind[1]))
		s := math.Hypot(wind[0], wind[1])
		var sx, sy, sxx float64
		n := 0.0
		for f := 0; f < 600; f += 7 {
			r.present = uint64(f)
			for y := 0; y < 40; y++ {
				for x := 0; x < 40; x++ {
					vx, vy := r.windAt(float64(x)+0.3, float64(y)+0.7)
					if vx*wind[0]+vy*wind[1] <= 0 {
						t.Fatalf("wind %v: local wind %v,%v blows against the world wind", wind, vx, vy)
					}
					sx, sy, sxx = sx+vx, sy+vy, sxx+(vx-wind[0])*(vx-wind[0])+(vy-wind[1])*(vy-wind[1])
					n++
				}
			}
		}
		mx, my := sx/n, sy/n
		if math.Hypot(mx-wind[0], my-wind[1]) > 0.08*s {
			t.Fatalf("wind %v: field mean %.3f,%.3f strays from the world wind", wind, mx, my)
		}
		if sd := math.Sqrt(sxx / n); sd < 0.15*s {
			t.Fatalf("wind %v: gusts too weak (sd %.3f)", wind, sd)
		}
	}
	r := envRenderer(windy(0, 0))
	r.present = 333
	if vx, vy := r.windAt(5, 5); vx != 0 || vy != 0 {
		t.Fatalf("still air has wind %v,%v", vx, vy)
	}
}

func TestGustFrontsTravelDownwind(t *testing.T) {
	t.Parallel()
	wx, wy := 2.0, 1.0
	r := envRenderer(windy(wx, wy))
	s := math.Hypot(wx, wy)
	const frames = 45
	dist := 1.2 * s * frames / 60.0
	dx, dy := wx/s*dist, wy/s*dist
	moved := 0
	for i := 0; i < 50; i++ {
		px, py := float64(i%10)*1.7, float64(i/10)*2.3
		r.present = 120
		ax, ay := r.windAt(px, py)
		r.present = 120 + frames
		bx, by := r.windAt(px+dx, py+dy) // the same gust, carried downwind
		if math.Hypot(ax-bx, ay-by) > 1e-9 {
			t.Fatalf("gust at %.1f,%.1f did not travel downwind at 1.2x the wind: %v,%v vs %v,%v", px, py, ax, ay, bx, by)
		}
		cx, cy := r.windAt(px, py)
		ux, uy := r.windAt(px-dx, py-dy)
		if math.Hypot(ax-cx, ay-cy) > 1e-3 {
			moved++
		}
		if math.Hypot(ax-ux, ay-uy) < 1e-9 {
			t.Fatalf("gust at %.1f,%.1f travelled upwind", px, py)
		}
	}
	if moved < 40 {
		t.Fatalf("gust fronts do not move (%d/50 points changed)", moved)
	}
}

// --- plant lean ---------------------------------------------------------------

var leanLimit = map[plantKind]float64{kindGrass: 0.35, kindCrop: 0.22, kindBush: 0.12, kindTree: 0.06 + 0.02}

func TestPlantLeanBoundedPerKind(t *testing.T) {
	t.Parallel()
	amounts := map[plantKind]float64{kindGrass: 0.05, kindCrop: 0.07, kindBush: 0.035, kindTree: 0.022}
	peak := map[plantKind]float64{}
	for _, wind := range [][2]float64{{4.5, 0}, {-3.2, 3.2}} {
		r := envRenderer(game.WorldView{Weather: "storm", WeatherIntensity: 1, WindX: wind[0], WindY: wind[1]})
		for f := 0; f < 400; f += 5 {
			r.present = uint64(f)
			for x := 0; x < 30; x++ {
				for k, lim := range leanLimit {
					ps := r.plantPose(x, 3, k, amounts[k], 1)
					if math.Abs(ps.skew) > lim+1e-9 {
						t.Fatalf("kind %d leans %.3f > limit %.2f", k, ps.skew, lim)
					}
					peak[k] = math.Max(peak[k], math.Abs(ps.skew))
				}
			}
		}
		// a creature shoving hard still cannot break the limit
		r.env.springs[envTileKey(4, 3)] = &envSpring{cx: 50, cy: -50, vx: 90}
		for k, lim := range leanLimit {
			if ps := r.plantPose(4, 3, k, amounts[k], 1); math.Abs(ps.skew) > lim+1e-9 || ps.sy <= 0 {
				t.Fatalf("kind %d shoved: skew %.3f sy %.3f", k, ps.skew, ps.sy)
			}
		}
	}
	for k, lim := range leanLimit {
		if peak[k] < 0.4*lim {
			t.Fatalf("kind %d peaks at %.3f, under 40%% of its %.2f limit in a storm", k, peak[k], lim)
		}
	}
	if !(peak[kindGrass] > peak[kindCrop] && peak[kindCrop] > peak[kindBush] && peak[kindBush] > peak[kindTree]) {
		t.Fatalf("stiffer plants should lean less: %v", peak)
	}
}

func TestGrassSways(t *testing.T) {
	t.Parallel()
	r := envRenderer(windy(1.6, 0.4))
	lo, hi := math.Inf(1), math.Inf(-1)
	for f := 0; f < 600; f++ {
		r.present = uint64(f)
		l := leanEast(r.plantPose(10, 10, kindGrass, 0.05, 1))
		lo, hi = math.Min(lo, l), math.Max(hi, l)
	}
	if hi-lo < 0.03 {
		t.Fatalf("grass barely sways: %.4f..%.4f", lo, hi)
	}
	if lo <= 0 {
		t.Fatalf("grass in an east wind should lean east, min %.4f", lo)
	}
	// a field ripples: tufts along the wind are out of phase
	r.present = 200
	var xs []float64
	for x := 0; x < 24; x++ {
		xs = append(xs, leanEast(r.plantPose(x, 10, kindGrass, 0.05, 1)))
	}
	if _, cv := meanCV(xs); cv < 0.05 {
		t.Fatalf("tufts across the field lean in lockstep (cv %.4f)", cv)
	}
	// still air: grass only trembles a little
	calm := envRenderer(windy(0, 0))
	for f := 0; f < 300; f++ {
		calm.present = uint64(f)
		if l := math.Abs(leanEast(calm.plantPose(10, 10, kindGrass, 0.05, 1))); l > 0.03 {
			t.Fatalf("grass leans %.3f in still air", l)
		}
	}
}

// --- contact ------------------------------------------------------------------

// walkPast runs a walker along path and the same world without it, frame by
// frame, handing each frame's walker motion and the pose difference at
// (px, py).
func walkPast(t *testing.T, w game.WorldView, path []cell, px, py int, kind plantKind, frames int,
	fn func(f int, m moveState, d pose, with, ref *Renderer)) {
	t.Helper()
	with := New(newEnvSim(w, &scriptMover{id: 1, path: path, start: 10}))
	ref := New(newEnvSim(w))
	for f := 0; f < frames; f++ {
		with.advance()
		ref.advance()
		a, b := with.plantPose(px, py, kind, 0.055, 1), ref.plantPose(px, py, kind, 0.055, 1)
		fn(f, with.pawnMotion(1), pose{skew: a.skew - b.skew, sy: a.sy - b.sy}, with, ref)
	}
}

func TestContactBendsAwayFromMover(t *testing.T) {
	t.Parallel()
	w := windy(0, 0)
	east, west, south := 0.0, 0.0, 0.0
	walkPast(t, w, line(2, 5, 1, 0, 10), 7, 5, kindCrop, 200, func(_ int, m moveState, d pose, _, _ *Renderer) {
		if m.X > 6.4 && m.X < 7 && math.Abs(m.Y-5) < 0.01 {
			east = math.Max(east, leanEast(d))
		}
	})
	walkPast(t, w, line(12, 5, -1, 0, 10), 7, 5, kindCrop, 200, func(_ int, m moveState, d pose, _, _ *Renderer) {
		if m.X > 7 && m.X < 7.6 {
			west = math.Max(west, -leanEast(d))
		}
	})
	walkPast(t, w, line(7, 1, 0, 1, 10), 7, 5, kindCrop, 200, func(_ int, m moveState, d pose, _, _ *Renderer) {
		if m.Y > 4.4 && m.Y < 5 {
			south = math.Max(south, -d.sy) // leaning south shortens it on screen
		}
	})
	t.Logf("bend: east %.3f west %.3f south %.3f", east, west, south)
	if east < 0.02 || west < 0.02 || south < 0.01 {
		t.Fatalf("plants should bend away from an approaching walker: east %.3f west %.3f south %.3f", east, west, south)
	}
}

func TestContactSpringsBack(t *testing.T) {
	t.Parallel()
	w := windy(0, 0)
	var lean []float64
	last := -1
	var with *Renderer
	walkPast(t, w, line(2, 5, 1, 0, 10), 7, 5, kindCrop, 420, func(f int, m moveState, d pose, r, _ *Renderer) {
		lean = append(lean, math.Abs(d.skew))
		if math.Hypot(m.X-7, m.Y-5) < contactRadius {
			last = f
		}
		with = r
	})
	if last < 0 {
		t.Fatal("walker never touched the plant")
	}
	peak := 0.0
	for _, l := range lean {
		peak = math.Max(peak, l)
	}
	settled := -1
	for f := len(lean) - 1; f >= 0; f-- {
		if lean[f] > 0.1*peak {
			settled = f + 1
			break
		}
	}
	t.Logf("peak %.3f rad, settled %.2fs after the last touch", peak, float64(settled-last)/60)
	if secs := float64(settled-last) / 60; secs >= 1.2 {
		t.Fatalf("plant took %.2fs to spring back (peak %.3f)", secs, peak)
	}
	if n := len(with.env.springs); n != 0 {
		t.Fatalf("%d contact springs kept after everything settled", n)
	}
}

func TestContactIgnoresFarAndStillCreatures(t *testing.T) {
	t.Parallel()
	w := windy(0.8, 0)
	still := &scriptMover{id: 2, path: []cell{{7, 5}}}
	far := &scriptMover{id: 1, path: line(2, 9, 1, 0, 10), start: 10}
	with := New(newEnvSim(w, far, still))
	ref := New(newEnvSim(w))
	for f := 0; f < 240; f++ {
		with.advance()
		ref.advance()
		for k := range with.env.springs {
			if y := int(k >> 16); y != 9 {
				t.Fatalf("frame %d: contact spring on row %d, away from the only walker", f, y)
			}
		}
		if a, b := with.plantPose(7, 5, kindCrop, 0.055, 1), ref.plantPose(7, 5, kindCrop, 0.055, 1); a != b {
			t.Fatalf("frame %d: a standing creature moved the plant under it: %+v vs %+v", f, a, b)
		}
	}
}

func TestTreeShakesWhenBrushed(t *testing.T) {
	t.Parallel()
	for _, season := range []string{"Fall", "Summer"} {
		w := windy(0.5, 0)
		w.Season = season
		w.Plants = []game.PlantView{{ID: 50, X: 7, Y: 5, Species: "tree", Growth: 100}}
		shake := 0.0
		walkPast(t, w, line(2, 5, 1, 0, 10), 7, 5, kindTree, 200, func(_ int, _ moveState, d pose, _, _ *Renderer) {
			shake = math.Max(shake, math.Abs(d.skew))
		})
		t.Logf("%s: shake %.4f", season, shake)
		if shake < 0.01 || shake > leanLimit[kindTree] {
			t.Fatalf("%s: brushed tree shake %.4f", season, shake)
		}
	}
	// leaves fall only in Fall
	for _, season := range []string{"Fall", "Summer"} {
		w := windy(0.5, 0)
		w.Season = season
		w.Plants = []game.PlantView{{ID: 50, X: 7, Y: 5, Species: "tree", Growth: 100}}
		r := New(newEnvSim(w, &scriptMover{id: 1, path: line(2, 5, 1, 0, 10), start: 10}))
		dropped, prev := 0, 0
		for f := 0; f < 200; f++ {
			r.advance()
			n := 0
			for _, p := range r.parts {
				if p.kind == partLeaf {
					n++
				}
			}
			dropped += max(n-prev, 0)
			prev = n
		}
		if season == "Fall" && (dropped < 1 || dropped > 2) {
			t.Fatalf("Fall: a brushed tree should drop 1-2 leaves, dropped %d", dropped)
		}
		if season == "Summer" && dropped != 0 {
			t.Fatalf("Summer: dropped %d leaves", dropped)
		}
	}
}

// --- sprite keys ----------------------------------------------------------------

// treeAt finds a tile whose tree is the given species.
func treeAt(t *testing.T, species string) (game.PlantView, string) {
	t.Helper()
	for i := 0; i < 400; i++ {
		p := game.PlantView{X: i % 20, Y: i / 20, Species: "tree", Growth: 100}
		if k, _ := plantSprite(p); strings.HasPrefix(k, "plant/tree_"+species+"_") {
			return p, k
		}
	}
	t.Fatalf("no %s tree on 400 tiles", species)
	return game.PlantView{}, ""
}

func TestSeasonalTreeKey(t *testing.T) {
	t.Parallel()
	var all []string
	for _, sp := range []string{"oak", "birch", "pine"} {
		for v := 0; v < 3; v++ {
			k := artKey("plant", "tree_"+sp, v)
			all = append(all, k)
			if sp != "pine" {
				all = append(all, k+"_autumn", k+"_bare")
			}
		}
	}
	r := New(&stubSim{w: 64, h: 64})
	r.art = envArt(all...)
	season := func(s string) game.WorldView { return game.WorldView{Season: s} }
	for _, sp := range []string{"oak", "birch"} {
		_, k := treeAt(t, sp)
		if got := r.seasonKey(k, season("Fall")); got != k+"_autumn" {
			t.Fatalf("%s in Fall: %s", sp, got)
		}
		if got := r.seasonKey(k, season("Winter")); got != k+"_bare" {
			t.Fatalf("%s in Winter: %s", sp, got)
		}
		for _, s := range []string{"Spring", "Summer", ""} {
			if got := r.seasonKey(k, season(s)); got != k {
				t.Fatalf("%s in %q: %s", sp, s, got)
			}
		}
	}
	_, pine := treeAt(t, "pine")
	for _, s := range []string{"Spring", "Summer", "Fall", "Winter"} {
		if got := r.seasonKey(pine, season(s)); got != pine {
			t.Fatalf("pine in %s: %s", s, got)
		}
	}
	// missing art falls back: no autumn look -> the species
	p, oak := treeAt(t, "oak")
	r.art = envArt(oak)
	if got := r.seasonKey(oak, season("Fall")); got != oak {
		t.Fatalf("no autumn art: %s", got)
	}
	if again := r.seasonKey(oak, season("Fall")); again != oak {
		t.Fatalf("cached key changed: %s then %s", oak, again)
	}
	if k, _ := plantSprite(p); k != oak {
		t.Fatalf("tree key not stable per tile: %s then %s", oak, k)
	}
}

func TestCropStageAndVariantKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		stage, growth int
		want, legacy  string
	}{
		{0, 3, "sown", "plant/potato_sown_v0"},
		{0, 8, "sprout", "plant/potato_sown_v0"},
		{1, 30, "young", "plant/potato_young_v0"},
		{2, 60, "mature", "plant/potato_mature_v0"},
		{3, 100, "harvestable", "plant/potato_harvestable_v0"},
	}
	// PLAY-016: a missing variant falls back to the stage's v0 (a sprout to
	// the sown v0); the old alias sprites are gone
	legacy := envArt("plant/potato_sown_v0", "plant/potato_young_v0", "plant/potato_mature_v0", "plant/potato_harvestable_v0")
	r := New(&stubSim{w: 64, h: 64})
	for _, c := range cases {
		seen := map[string]bool{}
		for x := 0; x < 30; x++ {
			p := game.PlantView{X: x, Y: 4, Species: "potato", Stage: c.stage, Growth: c.growth}
			k, scale := plantSprite(p)
			v := int(tileHash(x, 4) % cropVariants)
			want := "plant/potato_" + c.want + "_v" + string(rune('0'+v))
			if k != want || scale != 1 {
				t.Fatalf("stage %d growth %d at x=%d: %s (scale %v), want %s", c.stage, c.growth, x, k, scale, want)
			}
			seen[k] = true
			r.art = legacy
			if got := r.seasonKey(k, game.WorldView{Season: "Fall"}); got != c.legacy {
				t.Fatalf("%s falls back to %s, want %s", k, got, c.legacy)
			}
			r.art = envArt(k)
			if got := r.seasonKey(k, game.WorldView{Season: "Winter"}); got != k {
				t.Fatalf("crops have no seasonal look: %s -> %s", k, got)
			}
			v0 := "plant/potato_" + c.want + "_v0"
			r.art = envArt(v0, c.legacy)
			if got := r.seasonKey(k, game.WorldView{}); got != v0 {
				t.Fatalf("%s without its variant: %s, want %s", k, got, v0)
			}
		}
		if len(seen) != cropVariants {
			t.Fatalf("stage %s uses %d variants, want %d", c.want, len(seen), cropVariants)
		}
	}
}

func TestBushVariantKey(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 64, h: 64})
	var arts []string
	for v := 0; v < groundVariants; v++ {
		arts = append(arts, fmt.Sprintf("plant/berry_bush_mature_v%d", v), fmt.Sprintf("plant/berry_bush_harvestable_v%d", v), fmt.Sprintf("plant/berry_bush_dormant_v%d", v), fmt.Sprintf("plant/bramble_mature_v%d", v))
	}
	r.art = envArt(arts...)
	berry := map[string]bool{}
	for x := 0; x < 30; x++ {
		p := game.PlantView{X: x, Y: 2, Species: "berry_bush", Growth: 100, Harvestable: x%2 == 0}
		k, scale := plantSprite(p)
		if !strings.HasPrefix(k, "plant/berry_bush_") || strings.Contains(k, "_harvestable_v") != p.Harvestable || scale != 1 {
			t.Fatalf("berry bush at x=%d: %s scale %v", x, k, scale)
		}
		berry[strings.Replace(k, "_harvestable", "_x", 1)] = true
		got := r.seasonKey(k, game.WorldView{Season: "Fall"})
		if !r.art.has(got) || strings.Contains(got, "_harvestable_v") != p.Harvestable {
			t.Fatalf("berry bush %s falls back to %s", k, got)
		}
	}
	if len(berry) != 4 {
		t.Fatalf("berry bushes use %d variants, want 4", len(berry))
	}
	wild := map[string]bool{}
	for x := 0; x < 40; x++ {
		k, _ := plantSprite(game.PlantView{X: x, Y: 2, Species: "bramble", Growth: 50})
		wild[k] = true
		if got := r.seasonKey(k, game.WorldView{}); !r.art.has(got) {
			t.Fatalf("bush %s falls back to missing %s", k, got)
		}
	}
	if len(wild) != 4 {
		t.Fatalf("bushes use %d variants, want 4", len(wild))
	}
}

func TestWiltedKey(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 64, h: 64})
	r.art = envArt("plant/corn_wilted_v0")
	for stage := 0; stage <= 3; stage++ {
		p := game.PlantView{X: 3, Y: 3, Species: "corn", Stage: stage, Growth: 25 * stage, Damage: cropWiltDamage, DamageKind: "cold"}
		k, _ := plantSprite(p)
		want := "plant/corn_wilted_v" + string(rune('0'+int(tileHash(3, 3)%cropVariants)))
		if k != want {
			t.Fatalf("wilted corn at stage %d: %s, want %s", stage, k, want)
		}
		if got := r.seasonKey(k, game.WorldView{}); got != "plant/corn_wilted_v0" {
			t.Fatalf("wilted corn falls back to %s", got)
		}
	}
	// the plain Wilted flag (near frost, no damage yet) keeps the normal look
	if k, _ := plantSprite(game.PlantView{X: 3, Y: 3, Species: "corn", Stage: 2, Growth: 60, Wilted: true}); strings.Contains(k, "wilted") {
		t.Fatalf("a flagged but undamaged crop wilted: %s", k)
	}
	// only crops wilt
	tree, _ := plantSprite(game.PlantView{X: 3, Y: 3, Species: "tree", Growth: 100, Wilted: true})
	if strings.Contains(tree, "wilted") {
		t.Fatalf("a tree wilted: %s", tree)
	}
}

// --- weather particles ------------------------------------------------------------

// weatherRun steps a renderer viewing a 24x14-tile window through n frames
// of weather w.
func weatherRun(w game.WorldView, n int) *Renderer {
	r := New(&stubSim{w: 64, h: 64})
	r.env.view = envRect{0, 0, 24, 14}
	stepFrames(r, w, n)
	return r
}

func countDrops(r *Renderer, kind dropKind) int {
	n := 0
	for _, d := range r.env.drops {
		if d.kind == kind {
			n++
		}
	}
	return n
}

func TestRainDensityFollowsIntensity(t *testing.T) {
	t.Parallel()
	rain := func(i float64) int {
		return countDrops(weatherRun(game.WorldView{Weather: "rain", WeatherIntensity: i, WindX: 1}, 240), dropRain)
	}
	none, light, mid, heavy := rain(0), rain(0.2), rain(0.5), rain(1)
	t.Logf("drops: %d %d %d %d", none, light, mid, heavy)
	if none != 0 || !(light > 0 && light < mid && mid < heavy) {
		t.Fatalf("drops by intensity: 0->%d 0.2->%d 0.5->%d 1->%d", none, light, mid, heavy)
	}
	if ratio := float64(heavy) / float64(mid); ratio < 1.5 || ratio > 2.5 {
		t.Fatalf("density not proportional to intensity: 0.5->%d 1->%d", mid, heavy)
	}
	if n := len(weatherRun(game.WorldView{Weather: "clear", WindX: 1}, 120).env.drops); n != 0 {
		t.Fatalf("clear sky has %d drops", n)
	}
	if n := countDrops(weatherRun(game.WorldView{Weather: "rain", WeatherIntensity: 0.8}, 240), dropSplash); n == 0 {
		t.Fatal("rain makes no splashes where it lands")
	}
}

func TestRainAngleFollowsWind(t *testing.T) {
	t.Parallel()
	angle := func(wx float64) float64 {
		r := weatherRun(game.WorldView{Weather: "rain", WeatherIntensity: 0.7, WindX: wx}, 120)
		sum, n := 0.0, 0
		for _, d := range r.env.drops {
			if d.kind == dropRain {
				sx, sy := d.screenVel()
				if sy <= 0 {
					t.Fatalf("rain falls upward on screen: %v,%v", sx, sy)
				}
				sum += math.Atan2(sx, sy)
				n++
			}
		}
		return sum / float64(n)
	}
	calm, breeze, gale, west := angle(0), angle(1.5), angle(4), angle(-2)
	t.Logf("slant: %.3f %.3f %.3f %.3f", calm, breeze, gale, west)
	if math.Abs(calm) > 0.02 {
		t.Fatalf("rain in still air slants %.3f", calm)
	}
	if !(breeze > 0.04 && gale > breeze && west < -0.04) {
		t.Fatalf("rain slant by wind: 0->%.3f 1.5->%.3f 4->%.3f -2->%.3f", calm, breeze, gale, west)
	}
}

func TestSnowFlakesDriftAndFlutter(t *testing.T) {
	t.Parallel()
	w := game.WorldView{Weather: "snow", WeatherIntensity: 0.6, WindX: 1.2, WindY: 0}
	r := weatherRun(w, 200)
	sizes := map[float32]bool{}
	var drift float64
	n := 0
	for _, d := range r.env.drops {
		if d.kind != dropSnow {
			continue
		}
		sizes[d.size] = true
		drift += d.vx
		n++
	}
	if n < 50 || len(sizes) < 3 {
		t.Fatalf("snow: %d flakes in %d sizes", n, len(sizes))
	}
	if m := drift / float64(n); math.Abs(m-1.2) > 0.25 {
		t.Fatalf("flakes drift %.2f tiles/s in a 1.2 wind", m)
	}
	// one flake's path wanders around its drift line
	var d *weatherDrop
	for i := range r.env.drops {
		if r.env.drops[i].kind == dropSnow && r.env.drops[i].t > 2 {
			d = &r.env.drops[i]
			break
		}
	}
	if d == nil {
		t.Fatal("no long-falling flake")
	}
	flips, prev := 0, 0.0
	for k := 0; k < 120; k++ {
		tt := d.t - float64(k)/60
		if tt < 0 {
			break
		}
		x, _ := d.posAt(tt)
		off := x - (d.gx - d.vx*tt)
		if k > 0 && (off > 0) != (prev > 0) {
			flips++
		}
		prev = off
	}
	if flips < 1 {
		t.Fatal("snowflake falls without fluttering")
	}
	// snow drifts more slowly than rain falls
	rain := weatherRun(game.WorldView{Weather: "rain", WeatherIntensity: 0.6}, 60)
	for _, rd := range rain.env.drops {
		if rd.kind == dropRain && rd.vz < 3*d.vz {
			t.Fatalf("snow falls at %.2f, rain at %.2f", d.vz, rd.vz)
		}
	}
}

func TestWeatherParticlePoolCapped(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		kind string
		cap  int
	}{{"storm", 1400}, {"blizzard", 1100}, {"rain", 1400}, {"snow", 1100}} {
		r := New(&stubSim{w: 400, h: 400})
		r.env.view = envRect{0, 0, 300, 200} // zoomed far out
		w := game.WorldView{Weather: c.kind, WeatherIntensity: 1, WindX: 3}
		peak := 0
		for f := 0; f < 500; f++ {
			r.present++
			r.stepEnvironment(w, nil)
			peak = max(peak, len(r.env.drops))
			if len(r.parts) != 0 {
				t.Fatalf("%s spilled into the effect particles", c.kind)
			}
		}
		t.Logf("%s: pool peak %d", c.kind, peak)
		if peak > c.cap || peak < c.cap*3/4 {
			t.Fatalf("%s pool peaked at %d, cap %d", c.kind, peak, c.cap)
		}
	}
}

// --- ground cover and sky -----------------------------------------------------------

func TestSnowCoverFollowsDepthWithVariants(t *testing.T) {
	t.Parallel()
	cover := func(depth float64) (float64, map[string]bool) {
		sum, keys := 0.0, map[string]bool{}
		for i := 0; i < 400; i++ {
			k, a := snowCover(i%20, i/20, depth)
			if a < 0 || a > 1 {
				t.Fatalf("cover alpha %v", a)
			}
			if a > 0 {
				keys[k] = true
			}
			sum += a
		}
		return sum / 400, keys
	}
	none, _ := cover(0)
	thin, _ := cover(0.15)
	deep, keys := cover(0.8)
	if none != 0 || !(thin > 0 && thin < deep) || deep < 0.8 {
		t.Fatalf("snow cover by depth: 0->%.2f 0.15->%.2f 0.8->%.2f", none, thin, deep)
	}
	// open ground takes the tileable covers (04, 05); 00..03 are the
	// north/east/south/west edge masks that bank snow against walls
	if len(keys) != 2 || !keys["weather/snow_cover_04"] || !keys["weather/snow_cover_05"] {
		t.Fatalf("snow cover variants %v", keys)
	}
	for d, want := range []string{"weather/snow_cover_00", "weather/snow_cover_01", "weather/snow_cover_02", "weather/snow_cover_03"} {
		if snowBankKeys[d] != want {
			t.Fatalf("bank %d uses %s", d, snowBankKeys[d])
		}
	}
	if k1, _ := snowCover(4, 9, 0.5); func() bool { k2, _ := snowCover(4, 9, 0.9); return k1 != k2 }() {
		t.Fatal("a tile's snow variant changes with depth")
	}
	drifts := func(depth float64) int {
		n := 0
		for i := 0; i < 400; i++ {
			if k, a := snowDrift(i%20, i/20, depth); a > 0 {
				if !strings.HasPrefix(k, "weather/snow_drift_0") {
					t.Fatalf("drift key %s", k)
				}
				n++
			}
		}
		return n
	}
	if drifts(0.2) != 0 || drifts(0.9) == 0 {
		t.Fatalf("drifts: shallow %d deep %d", drifts(0.2), drifts(0.9))
	}
	// snow buries the grass tufts progressively
	visible := func(depth float64) float64 {
		w := windy(0.5, 0)
		w.SnowDepth = depth
		r := envRenderer(w)
		sum := 0.0
		for x := 0; x < 40; x++ {
			ps := r.plantPose(x, 7, kindGrass, 0.05, 1)
			a := float64(ps.cs[3])
			if ps.cs == (colorScale{}) {
				a = 1
			}
			sum += a * ps.sy
		}
		return sum / 40
	}
	if g0, g1, g2 := visible(0), visible(0.4), visible(0.9); !(g0 > 0.95 && g1 < g0 && g2 < g1 && g2 < 0.15) {
		t.Fatalf("grass under snow: 0->%.2f 0.4->%.2f 0.9->%.2f", g0, g1, g2)
	}
}

func TestFrostOnFreezingMornings(t *testing.T) {
	t.Parallel()
	at := func(h int) clock.Date { return clock.Date{Hour: h} }
	if frostStrength(-40, 0, at(6)) <= frostStrength(-10, 0, at(6)) || frostStrength(-40, 0, at(6)) <= 0 {
		t.Fatal("colder mornings should frost more")
	}
	if frostStrength(-40, 0, at(9)) >= frostStrength(-40, 0, at(6)) {
		t.Fatal("frost should fade through the morning")
	}
	for _, c := range []struct {
		temp int
		snow float64
		hour int
	}{{10, 0, 6}, {-40, 0, 14}, {-40, 0, 1}, {-40, 0.5, 6}} {
		if f := frostStrength(c.temp, c.snow, at(c.hour)); f != 0 {
			t.Fatalf("frost %.2f at %+v", f, c)
		}
	}
}

func TestPuddlesFollowWetness(t *testing.T) {
	t.Parallel()
	count := func(wet float64) (int, map[string]bool) {
		n, keys := 0, map[string]bool{}
		for i := 0; i < 900; i++ {
			k, a := puddle(i%30, i/30, wet)
			if a > 0 {
				n++
				keys[k] = true
				if !strings.HasPrefix(k, "weather/puddle_0") {
					t.Fatalf("puddle key %s", k)
				}
			}
		}
		return n, keys
	}
	dry, _ := count(0)
	damp, _ := count(0.3)
	soaked, keys := count(1)
	if dry != 0 || !(damp > 0 && damp < soaked) || soaked > 450 {
		t.Fatalf("puddles by wetness: 0->%d 0.3->%d 1->%d of 900", dry, damp, soaked)
	}
	if len(keys) < 5 {
		t.Fatalf("puddles use %d variants", len(keys))
	}
	_, a1 := puddle(5, 5, 0.6)
	_, a2 := puddle(5, 5, 1)
	if a2 < a1 {
		t.Fatalf("a puddle shrinks as it gets wetter: %.2f -> %.2f", a1, a2)
	}
}

func TestStormDarkensSky(t *testing.T) {
	t.Parallel()
	veil := func(kind string, i float64) float64 {
		c, _ := weatherVeil(game.WorldView{Weather: kind, WeatherIntensity: i})
		lum := (float64(c.R) + float64(c.G) + float64(c.B)) / (3 * 255)
		return float64(c.A) / 255 * (1 - lum) // how much it darkens
	}
	clear, cloudy, rain, storm := veil("clear", 0), veil("cloudy", 0.15), veil("rain", 0.6), veil("storm", 1)
	if clear != 0 || !(cloudy > 0 && rain > cloudy && storm > rain) {
		t.Fatalf("darkening: clear %.3f cloudy %.3f rain %.3f storm %.3f", clear, cloudy, rain, storm)
	}
	if _, fog := weatherVeil(game.WorldView{Weather: "blizzard", WeatherIntensity: 1}); fog.A < 40 {
		t.Fatalf("a blizzard barely cuts visibility: fog alpha %d", fog.A)
	}
	if _, fog := weatherVeil(game.WorldView{Weather: "rain", WeatherIntensity: 1}); fog.A != 0 {
		t.Fatal("rain fogs the screen like a blizzard")
	}
	// lightning: rare, deterministic, storms only
	flashes := 0
	for b := uint64(0); b < 4000; b++ {
		f := lightningAt("storm", 1, b)
		if f != lightningAt("storm", 1, b) {
			t.Fatal("lightning is not deterministic")
		}
		if f {
			flashes++
		}
		if lightningAt("rain", 1, b) || lightningAt("clear", 0, b) {
			t.Fatal("lightning outside a storm")
		}
	}
	if flashes == 0 || flashes > 200 {
		t.Fatalf("%d flashes in 4000 buckets", flashes)
	}
}

func TestWeatherHUDLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		w    game.WorldView
		want string
	}{
		{game.WorldView{Weather: "rain", WindX: 1.5, WindY: -1.5}, "Rain ↗ 2.1"},
		{game.WorldView{Weather: "storm", WindX: -3}, "Storm ← 3.0"},
		{game.WorldView{Weather: "snow", WindY: 1}, "Snow ↓ 1.0"},
		{game.WorldView{Weather: "blizzard", WindX: 3, WindY: 3}, "Blizzard ↘ 4.2"},
		{game.WorldView{Weather: "cloudy", WindX: -0.5, WindY: -0.6}, "Cloudy ↖ 0.8"},
		{game.WorldView{Weather: "clear"}, "Clear, calm"},
		{game.WorldView{}, ""},
	}
	for _, c := range cases {
		if got := weatherLabel(c.w); got != c.want {
			t.Fatalf("label for %+v = %q, want %q", c.w, got, c.want)
		}
	}
}

// --- draw smoke ---------------------------------------------------------------

func TestEnvironmentDrawsWithAndWithoutArt(t *testing.T) {
	t.Parallel()
	weather := []string{"weather/raindrop", "weather/splash_00", "weather/splash_01", "weather/splash_02",
		"weather/snowflake_00", "weather/snowflake_01", "weather/snowflake_02", "weather/snowflake_03"}
	for i := 0; i < 6; i++ {
		weather = append(weather, artKey("weather", "snow_cover", i), artKey("weather", "puddle", i))
	}
	for i := 0; i < 4; i++ {
		weather = append(weather, artKey("weather", "snow_drift", i), artKey("weather", "frost", i))
	}
	for _, art := range []*artCatalog{nil, envArt(), envArt(weather...)} {
		for _, kind := range []string{"rain", "storm", "snow", "blizzard", "cloudy"} {
			w := game.WorldView{Weather: kind, WeatherIntensity: 1, WindX: 2, SnowDepth: 0.7, Wetness: 0.8,
				OutdoorTemp: -30, TerrainW: 30, Terrain: make([]uint8, 30*20), RoomGrid: make([]int, 30*20)}
			for i := range w.RoomGrid {
				w.RoomGrid[i] = -1
			}
			w.RoomGrid[33] = 0 // one indoor tile
			sim := newEnvSim(w)
			sim.w, sim.h = 30, 20
			r := New(sim)
			r.art = art
			screen := ebiten.NewImage(640, 400)
			for f := 0; f < 90; f++ {
				r.advance()
				r.drawGroundCover(screen, w, 640, 400, r.tilePx())
				r.drawWeather(screen, w, 640, 400, r.tilePx())
			}
			if kind != "cloudy" && len(r.env.drops) == 0 {
				t.Fatalf("%s drew no particles", kind)
			}
		}
	}
	// puddles without their art, a lightning flash, the HUD label
	w := game.WorldView{Weather: "storm", WeatherIntensity: 1, WindX: 2, WindY: -1, Wetness: 1, Season: "Fall"}
	sim := &stubSim{w: 30, h: 20}
	b := uint64(7)
	sim.tick = clock.Tick(b * lightningBucket)
	w.Strikes = []game.StrikeView{{X: 3, Y: 4, Tick: uint64(sim.tick)}} // the sim's strike this tick
	r := New(sim)
	screen := ebiten.NewImage(640, 400)
	for f := 0; f < 50; f++ {
		r.present++
		r.stepEnvironment(w, nil)
		r.drawGroundCover(screen, w, 640, 400, r.tilePx())
		r.drawWeather(screen, w, 640, 400, r.tilePx())
	}
	if !r.env.flashSeen || r.env.flashBucket != uint64(sim.tick)/lightningBucket {
		t.Fatal("the storm's lightning never flashed")
	}
	if countDrops(r, dropRipple) == 0 {
		t.Fatal("rain on soaked ground makes no ripples")
	}
	for _, lw := range []game.WorldView{w, {Weather: "clear"}, {}} {
		drawWeatherLabel(screen, 300, 10, lw)
	}
}

// --- roofs (SIM-018) ------------------------------------------------------------

// roofedWorld is a 64x64 outdoor map whose left half (x < 32) is roofed.
func roofedWorld(weather string) game.WorldView {
	w := game.WorldView{Weather: weather, WeatherIntensity: 1, WindX: 0.5, SnowDepth: 0.8, Wetness: 0.9,
		OutdoorTemp: -40, Season: "Winter", TerrainW: 64, Terrain: make([]uint8, 64*64),
		RoomGrid: make([]int, 64*64), RoofCells: make([]uint8, 64*64)}
	for i := range w.RoomGrid {
		w.RoomGrid[i] = -1
		if i%64 < 32 {
			w.RoofCells[i] = roofBuilt
		}
	}
	return w
}

func TestNoGroundCoverUnderRoof(t *testing.T) {
	t.Parallel()
	w := roofedWorld("snow")
	w.RoofCells[5*64+40] = roofNatural
	w.RoofCells[6*64+40] = roofPlanBuild // planned, not built: still open
	r := envRenderer(w)
	for _, c := range []struct {
		x, y int
		open bool
	}{{3, 3, false}, {31, 9, false}, {40, 5, false}, {40, 6, true}, {45, 9, true}} {
		if got := r.groundOpen(w, c.x, c.y); got != c.open {
			t.Errorf("groundOpen(%d,%d) = %v, want %v", c.x, c.y, got, c.open)
		}
	}
	// grass under a roof is not buried in snow
	under := r.plantPose(3, 3, kindGrass, 0.05, 1)
	open := r.plantPose(45, 3, kindGrass, 0.05, 1)
	if under.cs != (colorScale{}) && under.cs[3] != 1 || under.sy < open.sy {
		t.Errorf("roofed grass buried: %+v vs open %+v", under, open)
	}
	if open.sy >= 1 {
		t.Errorf("open grass not pressed by snow: %+v", open)
	}
	// and a whole frame draws
	sim := newEnvSim(w)
	r = New(sim)
	r.art = envArt()
	screen := ebiten.NewImage(640, 400)
	r.drawGroundCover(screen, w, 640, 400, r.tilePx())
}

func TestRainSkipsRoofedTiles(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"rain", "snow"} {
		w := roofedWorld(kind)
		r := New(&stubSim{w: 64, h: 64})
		r.env.view = envRect{20, 0, 44, 14}
		stepFrames(r, w, 240)
		if len(r.env.drops) == 0 {
			t.Fatalf("%s: no drops at all", kind)
		}
		for _, d := range r.env.drops {
			if x := int(math.Floor(d.gx)); x >= 0 && x < 32 && d.gy >= 0 { // off the map's top edge is open sky
				t.Fatalf("%s: drop kind %d lands under the roof at x=%.2f", kind, d.kind, d.gx)
			}
		}
	}
	// a roof built over falling rain swallows it: nothing lands there
	w := roofedWorld("rain")
	for i := range w.RoofCells {
		w.RoofCells[i] = 0
	}
	r := New(&stubSim{w: 64, h: 64})
	r.env.view = envRect{0, 0, 24, 14}
	stepFrames(r, w, 60)
	for i := range w.RoofCells {
		w.RoofCells[i] = roofBuilt
	}
	stepFrames(r, w, 60)
	for _, d := range r.env.drops {
		if d.kind != dropRain && d.gx >= 0 && d.gy >= 0 { // beyond the map edge is open sky
			t.Fatalf("drop kind %d landed under a new roof", d.kind)
		}
	}
}

func TestCollapseDustOncePerStamp(t *testing.T) {
	t.Parallel()
	w := game.WorldView{Weather: "clear", RoofCollapses: []game.RoofCollapseView{{X: 4, Y: 5, Tick: 900}}}
	r := envRenderer(w)
	count := func(k partKind) int {
		n := 0
		for _, p := range r.parts {
			if p.kind == k {
				n++
			}
		}
		return n
	}
	if count(partDust) != 12 || count(partChip) != 4 {
		t.Fatalf("first frame: dust %d chips %d, want 12 and 4", count(partDust), count(partChip))
	}
	for _, p := range r.parts {
		if p.x < 4 || p.x > 5 || p.y < 5 || p.y > 6 {
			t.Fatalf("particle at %.2f,%.2f, want over tile 4,5", p.x, p.y)
		}
	}
	stepFrames(r, w, 5)
	if count(partDust) != 12 || count(partChip) != 4 {
		t.Fatalf("stamp replayed: dust %d chips %d", count(partDust), count(partChip))
	}
	// a second collapse puffs once more; the first stays quiet
	w.RoofCollapses = append(w.RoofCollapses, game.RoofCollapseView{X: 9, Y: 2, Tick: 1200})
	stepFrames(r, w, 3)
	if count(partDust) != 24 || count(partChip) != 8 {
		t.Fatalf("second stamp: dust %d chips %d, want 24 and 8", count(partDust), count(partChip))
	}
	// Draw never emits: drawing a frame leaves the pool as it was
	n := len(r.parts)
	r.drawRoofOverlay(ebiten.NewImage(64, 64), w, 64, 64, r.tilePx())
	if len(r.parts) != n {
		t.Fatal("drawing emitted particles")
	}
}
