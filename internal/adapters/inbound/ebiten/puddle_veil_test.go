package ebiten

import (
	"fmt"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// PLAY-022 puddles: one smooth, terrain-aware veil baked per chunk with the
// wet darkening (puddle_veil.go, snow_cache.go).

// puddleWorld is a side × side open soil map in fall rain at wetness wet.
func puddleWorld(side int, wet float64) game.WorldView {
	w := game.WorldView{Weather: "rain", WeatherIntensity: 1, Wetness: wet, Season: "Fall", TerrainW: side,
		Terrain: make([]uint8, side*side), RoomGrid: make([]int, side*side), StructVer: 1}
	for i := range w.RoomGrid {
		w.RoomGrid[i] = -1
	}
	return w
}

// puddleSim serves w at its size.
func puddleSim(w game.WorldView) *envSim {
	s := newEnvSim(w)
	s.w, s.h = w.TerrainW, len(w.Terrain)/w.TerrainW
	return s
}

// bakeChunk bakes chunk (cx, cy) of w on r and returns it.
func bakeChunk(r *Renderer, w game.WorldView, cx, cy int) *snowChunk {
	r.env.SetWeather(w) // the room grid, roofs and snow the bake reads
	c := &snowChunk{}
	side := w.TerrainW
	r.bakeSnowChunk(c, w, side, len(w.Terrain)/side, cx, cy)
	return c
}

// pudAt is chunk c's puddle cover (0..255) at texel (sx, sy) of its tile
// (x, y), chunk-local, at the chunk's wet texels a tile.
func pudAt(c *snowChunk, x, y, sx, sy int) uint8 {
	if c.Pud == nil {
		return 0
	}
	return c.Pud[(snowPad+y*c.WetPx+sy)*wetImgW(c.WetPx)+snowPad+x*c.WetPx+sx]
}

// wetTexel is the wet veil's alpha at texel (sx, sy) of chunk-local tile
// (x, y) of the last bake in sc.pix.
func wetTexel(sc *snowCache, x, y, sx, sy int) uint8 { return snowTexel(sc, 1, x, y, sx, sy) }

// pudShare is the share of chunk c's own texels holding any water.
func pudShare(c *snowChunk) float64 {
	n := 0
	for ty := 0; ty < snowChunkTiles*snowPx; ty++ {
		for tx := 0; tx < snowChunkTiles*snowPx; tx++ {
			if pudAt(c, 0, 0, tx, ty) > 0 {
				n++
			}
		}
	}
	return float64(n) / float64(snowChunkTiles*snowPx*snowChunkTiles*snowPx)
}

func TestPuddleCoverGrowsWithWetness(t *testing.T) {
	t.Parallel()
	w := puddleWorld(64, 0)
	r := New(puddleSim(w))
	var shares []float64
	var prev []uint8
	for i := 0; i <= 10; i++ {
		w.Wetness = float64(i) / 10
		c := bakeChunk(r, w, 0, 0)
		s := pudShare(c)
		if len(shares) > 0 && s < shares[len(shares)-1] {
			t.Fatalf("wetness %.1f: puddles shrink %.3f -> %.3f", w.Wetness, shares[len(shares)-1], s)
		}
		// puddles grow from the same hollows: water stays where it lay
		for k, v := range prev {
			if v > 0 && (c.Pud == nil || c.Pud[k] < v) {
				t.Fatalf("wetness %.1f: texel %d drops from %d to %v", w.Wetness, k, v, c.Pud)
			}
		}
		prev = append(prev[:0], c.Pud...)
		shares = append(shares, s)
	}
	if shares[0] != 0 || shares[1] != 0 {
		t.Fatalf("dry and barely damp ground holds water: %.3f %.3f", shares[0], shares[1])
	}
	if shares[5] < 0.02 || shares[10] < 1.5*shares[5] || shares[10] > 0.4 {
		t.Fatalf("cover by wetness %v: want some at 0.5, half again more soaked, at most 40%%", shares)
	}
}

func TestPuddlesAreCoherentPatches(t *testing.T) {
	t.Parallel()
	const side = 128
	w := puddleWorld(side, 1)
	r := New(puddleSim(w))
	wet := make([]bool, side*side) // tile centre under water
	for cy := 0; cy < side/snowChunkTiles; cy++ {
		for cx := 0; cx < side/snowChunkTiles; cx++ {
			c := bakeChunk(r, w, cx, cy)
			for y := 0; y < snowChunkTiles; y++ {
				for x := 0; x < snowChunkTiles; x++ {
					wet[(cy*snowChunkTiles+y)*side+cx*snowChunkTiles+x] = pudAt(c, x, y, snowPx/2, snowPx/2) > 128
				}
			}
		}
	}
	// a wet tile's neighbours are mostly wet too: patches, not speckle
	// (speckle at this cover would wet a neighbour as often as any tile)
	pairs, same, cover := 0, 0, 0
	for _, v := range wet {
		if v {
			cover++
		}
	}
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			if !wet[y*side+x] {
				continue
			}
			for _, n := range [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
				nx, ny := x+n[0], y+n[1]
				if nx < 0 || ny < 0 || nx >= side || ny >= side {
					continue
				}
				pairs++
				if wet[ny*side+nx] {
					same++
				}
			}
		}
	}
	share := float64(cover) / float64(len(wet))
	if pairs == 0 || float64(same)/float64(pairs) < 0.5 || float64(same)/float64(pairs) < 3*share {
		t.Fatalf("wet tiles' neighbours wet %d of %d, %.0f%% of tiles wet", same, pairs, 100*share)
	}
	// patches span several tiles and vary in size
	seen := make([]bool, side*side)
	var sizes []int
	sizesSeen := map[int]bool{}
	for i := range wet {
		if !wet[i] || seen[i] {
			continue
		}
		n, stack := 0, []int{i}
		seen[i] = true
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			n++
			x, y := j%side, j/side
			for _, d := range [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if k := ny*side + nx; nx >= 0 && ny >= 0 && nx < side && ny < side && wet[k] && !seen[k] {
					seen[k] = true
					stack = append(stack, k)
				}
			}
		}
		sizes = append(sizes, n)
		sizesSeen[n] = true
	}
	sum, big := 0, 0
	for _, n := range sizes {
		sum += n
		big = max(big, n)
	}
	if len(sizes) < 5 || float64(sum)/float64(len(sizes)) < 3 || big < 8 || len(sizesSeen) < 4 {
		t.Fatalf("%d patches, mean %.1f tiles, largest %d, %d sizes", len(sizes), float64(sum)/float64(max(len(sizes), 1)), big, len(sizesSeen))
	}
}

func TestPuddlesOnlyOnOpenSoil(t *testing.T) {
	t.Parallel()
	w := puddleWorld(64, 1)
	// the left half is soil; the right half holds 8x8 blocks of ground
	// puddles never lie on
	type block struct {
		what   string
		x0, y0 int
	}
	blocks := []block{{"shallow_water", 32, 0}, {"deep_water", 40, 0}, {"sand", 48, 0}, {"mud", 56, 0},
		{"marsh", 32, 8}, {"ice", 40, 8}, {"track", 48, 8}, {"stone", 56, 8},
		{"stone_smooth", 32, 16}, {"floor", 40, 16}, {"crop", 48, 16}, {"roof", 56, 16}, {"wall", 32, 24}}
	crop := game.Crops()[0].Name
	for _, b := range blocks {
		for y := b.y0; y < b.y0+8; y++ {
			for x := b.x0; x < b.x0+8; x++ {
				i := y*64 + x
				switch b.what {
				case "floor":
					w.Buildings = append(w.Buildings, game.BuildingView{Kind: "floor", Stuff: "wood", X: x, Y: y, W: 1, H: 1, Floor: true})
				case "crop":
					w.Plants = append(w.Plants, game.PlantView{X: x, Y: y, Species: crop, Stage: 2, Growth: 50, Zone: crop})
				case "roof":
					if w.RoofCells == nil {
						w.RoofCells = make([]uint8, 64*64)
					}
					w.RoofCells[i] = roofBuilt
				case "wall":
					w.RoomGrid[i] = -2
				default:
					w.Terrain[i] = terrainCodeOf(b.what)
				}
			}
		}
	}
	// the rest of the right half is soil as well, so the noise has room
	r := New(puddleSim(w))
	soilWater := 0
	for cy := 0; cy < 2; cy++ {
		for cx := 0; cx < 2; cx++ {
			c := bakeChunk(r, w, cx, cy)
			for y := 0; y < snowChunkTiles; y++ {
				for x := 0; x < snowChunkTiles; x++ {
					gx, gy := cx*snowChunkTiles+x, cy*snowChunkTiles+y
					var in string
					for _, b := range blocks {
						if gx >= b.x0 && gx < b.x0+8 && gy >= b.y0 && gy < b.y0+8 {
							in = b.what
						}
					}
					for sy := 0; sy < snowPx; sy++ {
						for sx := 0; sx < snowPx; sx++ {
							p := pudAt(c, x, y, sx, sy)
							if in != "" && p != 0 {
								t.Fatalf("puddle on %s at tile (%d,%d) texel (%d,%d): %d", in, gx, gy, sx, sy, p)
							}
							if in == "" && p > 0 {
								soilWater++
							}
							if (in == "roof" || in == "wall") && wetTexel(&r.snowc, x, y, sx, sy) != 0 {
								t.Fatalf("wet veil on %s at (%d,%d)", in, gx, gy)
							}
						}
					}
				}
			}
		}
	}
	if soilWater == 0 {
		t.Fatal("soaked soil holds no water")
	}
	for _, n := range []string{"soil", "rich_soil", "gravel"} {
		if !puddleGroundCode[terrainCodeOf(n)] {
			t.Errorf("%s takes no puddles", n)
		}
	}
	for _, n := range []string{"sand", "mud", "marsh", "ice", "track", "stone", "stone_smooth", "rock", "rock_smooth", "shallow_water", "deep_water"} {
		if puddleGroundCode[terrainCodeOf(n)] {
			t.Errorf("%s takes puddles", n)
		}
	}
}

func TestPuddleEdgesAreSoft(t *testing.T) {
	t.Parallel()
	w := puddleWorld(64, 0.8)
	r := New(puddleSim(w))
	r.snowc.WantPx = wetPxFor(lodLiveMin) // up close, where shores are crisp
	c := bakeChunk(r, w, 0, 0)
	if c.WetPx != 8 {
		t.Fatalf("live detail %d texels a tile, want 8", c.WetPx)
	}
	n := snowChunkTiles * c.WetPx
	at := func(tx, ty int) int { return int(pudAt(c, 0, 0, tx, ty)) }
	wet, partial, hard := 0, 0, 0
	for ty := 0; ty < n; ty++ {
		for tx := 0; tx < n; tx++ {
			v := at(tx, ty)
			if v > 0 {
				wet++
				if v < 250 {
					partial++
				}
			}
			for _, d := range [][2]int{{1, 0}, {0, 1}} {
				if tx+d[0] < n && ty+d[1] < n {
					u := at(tx+d[0], ty+d[1])
					if (v == 0 && u >= 250) || (u == 0 && v >= 250) {
						hard++
						t.Logf("hard step at texel (%d,%d)+%v: %d -> %d", tx, ty, d, v, u)
					}
				}
			}
		}
	}
	// the cover ramps over a fraction of a tile at the water's edge
	// (puddleEdge): no texel next to a dry one is already full
	if wet == 0 || partial*10 < wet || hard != 0 {
		t.Fatalf("%d wet texels, %d partial, %d hard steps", wet, partial, hard)
	}
	// no rim: the veil deepens steadily from the edge into the middle,
	// darker and more reflective toward the centre, never a dark ring
	pa, pr, _ := puddleShade(0.1, -1, 1)
	for d := -0.5; d <= 3; d += 0.02 {
		a, rf, cov := puddleShade(0.1, d, 1)
		if a < pa-1e-12 || rf < pr-1e-12 || cov < 0 || cov > 1 {
			t.Fatalf("shade at %.2f tiles in: alpha %.3f refl %.3f after %.3f %.3f", d, a, rf, pa, pr)
		}
		if rf > a {
			t.Fatalf("reflection %.3f over alpha %.3f is not premultiplied", rf, a)
		}
		pa, pr = a, rf
	}
	edge, _, _ := puddleShade(0.1, 0.4, 1)
	mid, _, _ := puddleShade(0.1, 3, 1)
	if !(mid > edge && mid <= 0.6) {
		t.Fatalf("alpha edge %.2f centre %.2f: want low (the old stickers were 0.9), a little stronger in the middle", edge, mid)
	}
}

func TestPuddlesFadeUnderSnow(t *testing.T) {
	t.Parallel()
	w := puddleWorld(64, 1)
	r := New(puddleSim(w))
	total := func(depth uint8) int {
		w.Snow = make([]uint8, 64*64)
		for i := range w.Snow {
			w.Snow[i] = depth
		}
		w.SnowVer++
		r.env.SetWeather(w)
		c := bakeChunk(r, w, 0, 0)
		s := 0
		for _, v := range c.Pud {
			s += int(v)
		}
		return s
	}
	var got []int
	for d := 0; d <= 128; d += 8 {
		got = append(got, total(uint8(d)))
	}
	partial := 0
	for i, v := range got {
		if i > 0 && v > got[i-1] {
			t.Fatalf("puddles grow as the snow deepens: %v", got)
		}
		if v > 0 && v < got[0] {
			partial++
		}
	}
	if got[0] == 0 || got[len(got)-1] != 0 || partial < 3 {
		t.Fatalf("puddle cover by snow depth %v: want a smooth fade to none", got)
	}
}

func TestRipplesOnlyInPuddles(t *testing.T) {
	t.Parallel()
	w := puddleWorld(64, 0.9)
	w.WindX = 0.4
	r := New(puddleSim(w))
	screen := ebiten.NewImage(640, 400)
	ripples, splashes := 0, 0
	for f := 0; f < 240; f++ {
		r.present++
		r.stepEnvironment(w, nil)
		r.drawPuddleVeil(screen, w, 640, 400, r.tilePx())
		for _, d := range r.env.Drops {
			switch {
			case d.Kind == dropRipple && d.Age == 0:
				ripples++
				if p := r.puddleAt(d.Gx, d.Gy); p < puddleRippleMin {
					t.Fatalf("a ripple at (%.2f, %.2f) outside the water: cover %.2f", d.Gx, d.Gy, p)
				}
			case d.Kind == dropSplash && d.Age == 0:
				splashes++
				if p := r.puddleAt(d.Gx, d.Gy); p >= puddleRippleMin {
					t.Fatalf("a splash at (%.2f, %.2f) inside a puddle: cover %.2f", d.Gx, d.Gy, p)
				}
			}
		}
	}
	if ripples == 0 || splashes == 0 {
		t.Fatalf("%d ripples, %d splashes", ripples, splashes)
	}
	// dry ground: splashes only
	w.Wetness = 0
	r = New(puddleSim(w))
	for f := 0; f < 120; f++ {
		r.present++
		r.stepEnvironment(w, nil)
		r.drawPuddleVeil(screen, w, 640, 400, r.tilePx())
	}
	if countDrops(r, dropRipple) != 0 {
		t.Fatal("ripples on dry ground")
	}
}

func TestPuddleVeilDrawsFlatLitAtEveryTier(t *testing.T) {
	t.Parallel()
	for _, px := range []float64{64, 40, 24, 8} {
		w := puddleWorld(64, 1)
		r := New(puddleSim(w))
		r.cam.Zoom = px / 64
		screen := ebiten.NewImage(640, 400)
		before := r.lit.Calls
		r.drawPuddleVeil(screen, w, 640, 400, r.tilePx())
		if r.snowc.WetDraws == 0 || r.lit.Calls-before < r.snowc.WetDraws {
			t.Fatalf("tile %v px: %d veil draws, %d lit calls", px, r.snowc.WetDraws, r.lit.Calls-before)
		}
		// lit flat like the ground: no normal map, a sky value per vertex
		for _, v := range r.lod.verts {
			if v.Custom2 != 0 || v.Custom0 <= 0 {
				t.Fatalf("tile %v px: veil vertex %+v is not lit flat under the sky", px, v)
			}
		}
		w.Wetness = 0
		r.drawPuddleVeil(screen, w, 640, 400, r.tilePx())
		if r.snowc.WetDraws != 0 {
			t.Fatalf("tile %v px: dry ground draws %d veil passes", px, r.snowc.WetDraws)
		}
	}
	// the flat ground decals that remain are lit as ground, without normals
	for _, k := range []string{artKey("weather", "snow_cover", 4), artKey("weather", "frost", 0)} {
		if !isGroundKey(k) {
			t.Errorf("%s is not lit flat", k)
		}
	}
	if isGroundKey(artKey("weather", "snow_drift", 0)) {
		t.Error("a snow drift is a heap, not flat ground")
	}
}

func TestPuddleVeilRebakesOnWetBucketWithinBudget(t *testing.T) {
	t.Parallel()
	const side = 256
	w := puddleWorld(side, 38.0/wetBuckets)
	sim := puddleSim(w)
	r := New(sim)
	r.cam.Zoom = 8.0 / 64 // map tier: many chunks in view
	screen := ebiten.NewImage(1280, 800)
	draw := func() int {
		n := r.snowc.Bakes
		r.drawPuddleVeil(screen, w, 1280, 800, r.tilePx())
		return r.snowc.Bakes - n
	}
	first := draw()
	if first <= puddleBakeBudget {
		t.Fatalf("first draw baked %d chunks: want more than the budget in view", first)
	}
	if n := draw(); n != 0 {
		t.Fatalf("same frame rebaked %d", n)
	}
	w.Wetness += 0.2 / wetBuckets // the same bucket
	if wetBucket(w.Wetness) != 38 {
		t.Fatal("test: left the bucket")
	}
	if n := draw(); n != 0 {
		t.Fatalf("a wetness change inside its bucket rebaked %d", n)
	}
	w.Wetness = 0.7
	total := 0
	for i := 0; i < 20; i++ {
		n := draw()
		if n > puddleBakeBudget {
			t.Fatalf("frame %d rebaked %d chunks, budget %d", i, n, puddleBakeBudget)
		}
		total += n
	}
	if total != first {
		t.Fatalf("a new bucket rebaked %d of %d chunks", total, first)
	}
	// a crop sown at (40, 40) rebakes its chunk (1, 1); the key is kept per
	// 16-tile read-model chunk, so the chunks whose ring reaches that one
	// may follow, and nothing else
	c11 := r.snowc.Chunks[1*((side+snowChunkTiles-1)/snowChunkTiles)+1]
	before := c11.Plants
	w.Plants = append(w.Plants, game.PlantView{X: 40, Y: 40, Species: game.Crops()[0].Name, Zone: game.Crops()[0].Name})
	sim.world = w
	if n := draw(); n < 1 || n > 4 || c11.Plants == before {
		t.Fatalf("a crop rebaked %d chunks (its own: %v), want 1..4 with its own", n, c11.Plants != before)
	}
	if n := draw(); n != 0 {
		t.Fatalf("then %d more", n)
	}
}

// frozenSim serves a fixed world with its chunk read model built once, as
// the game's view cache does between ticks (the doubles rebuild it on
// every call, which would swamp a benchmark).
type frozenSim struct {
	*envSim
	idx game.ChunkIndex
}

func newFrozenSim(w game.WorldView) *frozenSim {
	s := &frozenSim{envSim: puddleSim(w)}
	s.idx.Update(&w, s.w, s.h)
	return s
}

func (s *frozenSim) ChunkLayer(cx, cy int) game.ChunkView { return s.idx.Layer(cx, cy) }
func (s *frozenSim) PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32 {
	w := s.world
	return s.idx.PlantsIn(&w, x0, y0, x1, y1, buf)
}

// BenchmarkPuddleVeil is the puddle veil of a soaked 500² world at each
// LOD's tile size. bakes/op is 0 once warm.
func BenchmarkPuddleVeil(b *testing.B) {
	const side = 500
	w := puddleWorld(side, 0.8)
	for _, px := range []float64{64, 32, 16, 4} {
		b.Run(fmt.Sprintf("tile%v", px), func(b *testing.B) {
			r := New(newFrozenSim(w))
			r.cam.Zoom = px / 64
			screen := ebiten.NewImage(1280, 800)
			tile := r.tilePx()
			r.cam.OffsetX, r.cam.OffsetY = 100*tile, 100*tile
			r.drawPuddleVeil(screen, w, 1280, 800, tile)
			b0, d0 := r.snowc.Bakes, 0
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.drawPuddleVeil(screen, w, 1280, 800, tile)
				d0 += r.snowc.WetDraws
			}
			b.ReportMetric(float64(r.snowc.Bakes-b0)/float64(b.N), "bakes/op")
			b.ReportMetric(float64(d0)/float64(b.N), "draws/op")
		})
	}
}

// BenchmarkPuddleBake is one chunk bake of the soaked veil at each wet
// detail (texels a tile).
func BenchmarkPuddleBake(b *testing.B) {
	w := puddleWorld(128, 0.8)
	for _, px := range []int{2, 4, 8} {
		b.Run(fmt.Sprintf("px%d", px), func(b *testing.B) {
			r := New(newFrozenSim(w))
			r.env.SetWeather(w)
			r.snowc.WantPx = px
			var c snowChunk
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.bakeSnowChunk(&c, w, 128, 128, i%4, (i/4)%4)
			}
		})
	}
}

// TestSnowCacheEvictsUnseenChunks: chunks the view left long ago are
// dropped with their images; the ones in view stay.
func TestSnowCacheEvictsUnseenChunks(t *testing.T) {
	t.Parallel()
	w := puddleWorld(128, 0.8)
	r := New(puddleSim(w))
	screen := ebiten.NewImage(640, 400)
	r.cam.OffsetX, r.cam.OffsetY = 0, 0
	r.drawPuddleVeil(screen, w, 640, 400, r.tilePx())
	if _, ok := r.snowc.Chunks[0]; !ok {
		t.Fatal("chunk (0, 0) not baked")
	}
	tile := r.tilePx()
	r.cam.OffsetX, r.cam.OffsetY = 96*tile, 96*tile // the far corner: chunk (3, 3)
	for i := 0; i <= snowEvictPasses+256; i++ {
		r.drawPuddleVeil(screen, w, 640, 400, tile)
	}
	if _, ok := r.snowc.Chunks[0]; ok {
		t.Fatal("chunk (0, 0) kept long after the view left it")
	}
	if _, ok := r.snowc.Chunks[3*4+3]; !ok {
		t.Fatal("the chunk in view was dropped")
	}
}
