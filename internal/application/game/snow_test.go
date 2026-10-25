package game

import (
	"bytes"
	"slices"
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
)

// snowGame is a 40x20 map at a steady air temperature of tempC tenths
// (no seasons, no day swing) under held clear weather, aligned to a
// climate step.
func snowGame(t testing.TB, tempC int) *Game {
	t.Helper()
	g := New(Config{Seed: 1, Width: 40, Height: 20})
	g.climate = temperature.Climate{YearMean: tempC}
	g.HoldWeather()
	g.SetWeather(weather.Clear, 0)
	jumpTo(g, 10, 1)
	return g
}

// climateSteps runs n whole climate intervals.
func climateSteps(g *Game, n int) {
	for range n {
		g.Step(climateInterval)
	}
}

func snowIdx(g *Game, x, y int) int { return y*g.grid.Width() + x }

func TestSnowBuildsOnOpenCellsOnly(t *testing.T) {
	t.Parallel()
	g := snowGame(t, -150)
	roofOver(g, 2, 2, 4, 4)                    // a roofed patch
	g.setTerrain(30, 10, terrain.ShallowWater) // open water
	g.SetWeather(weather.Snow, 1)
	climateSteps(g, 12)
	if d := g.snowAt(10, 10); d < 20 {
		t.Fatalf("open ground holds %d after 12 snowy steps, want a build-up", d)
	}
	if d := g.snowAt(3, 3); d != 0 {
		t.Fatalf("roofed cell holds %d, want 0", d)
	}
	if d := g.snowAt(30, 10); d != 0 {
		t.Fatalf("water holds %d, want 0", d)
	}
	if w := g.Weather().SnowDepth; w <= 0 || w > 1 {
		t.Fatalf("global depth %v, want the map average in (0, 1]", w)
	}
}

func TestSnowRoofedLaterClears(t *testing.T) {
	t.Parallel()
	g := snowGame(t, -150)
	g.setSnowCell(snowIdx(g, 5, 5), 200)
	roofOver(g, 5, 5, 5, 5)
	climateSteps(g, 1)
	if d := g.snowAt(5, 5); d != 0 {
		t.Fatalf("a newly roofed cell keeps %d, want 0", d)
	}
}

func TestSnowMeltsWithWarmth(t *testing.T) {
	t.Parallel()
	g := snowGame(t, 100) // 10 C: about 10 units a step
	g.setSnowCell(snowIdx(g, 8, 8), 200)
	climateSteps(g, 5)
	d := int(g.snowAt(8, 8))
	if d >= 200 || d < 140 || d > 160 {
		t.Fatalf("after 5 steps at 10 C depth %d, want about 149", d)
	}
	climateSteps(g, 30)
	if d := g.snowAt(8, 8); d != 0 {
		t.Fatalf("snow left at %d after a long thaw", d)
	}
	climateSteps(g, 1)
	if g.snow.depth != nil || g.Weather().SnowDepth != 0 {
		t.Fatal("a fully melted layer is dropped and the global depth is 0")
	}
}

func TestSnowStaysInFrost(t *testing.T) {
	t.Parallel()
	g := snowGame(t, -50)
	g.setSnowCell(snowIdx(g, 8, 8), 120)
	climateSteps(g, 10)
	if d := g.snowAt(8, 8); d != 120 {
		t.Fatalf("frost, no snowfall: depth %d, want 120", d)
	}
}

func TestSnowOpRainBlockHalves(t *testing.T) {
	t.Parallel()
	g := snowGame(t, -50)
	g.weatherFix = false
	block := uint64(weatherBlockTicks) * 7
	g.weather.Kind, g.weather.Target = weather.Rain, weather.Rain
	if halve, _ := g.snowOp(block, 0); !halve {
		t.Fatal("a rain block's first step halves the snow")
	}
	if halve, _ := g.snowOp(block+climateInterval, 0); halve {
		t.Fatal("only the block's first step halves")
	}
	g.weather.Target = weather.Storm
	if halve, _ := g.snowOp(block, 0); !halve {
		t.Fatal("a storm block halves too")
	}
	g.weather.Kind, g.weather.Target = weather.Snow, weather.Snow
	if halve, _ := g.snowOp(block, 0); halve {
		t.Fatal("a snow block does not halve")
	}
	g.weather.Target = weather.Rain
	g.weatherFix = true
	if halve, _ := g.snowOp(block, 0); halve {
		t.Fatal("held weather starts no block")
	}
}

func TestSnowRainBlockHalvesCells(t *testing.T) {
	t.Parallel()
	g := snowGame(t, -50)
	g.setSnowCell(snowIdx(g, 8, 8), 200)
	cg := g.snowChunkGrid()
	c := cg.Of(8, 8)
	g.snowChunk(cg, c, true, 0)
	if d := g.snowAt(8, 8); d != 100 {
		t.Fatalf("halved depth %d, want 100", d)
	}
}

func TestSnowOpFallAndMeltRates(t *testing.T) {
	t.Parallel()
	g := snowGame(t, -50)
	g.SetWeather(weather.Snow, 1)
	sum := 0
	for c := range 100 {
		_, d := g.snowOp(uint64(climateInterval*(c+40)), c)
		sum += d
	}
	if sum < 290 || sum > 322 { // 3.06 a step at full intensity, dithered
		t.Fatalf("100 full-intensity steps add %d, want about 306", sum)
	}
	g.climate = temperature.Climate{YearMean: 100}
	g.SetWeather(weather.Clear, 0)
	if _, d := g.snowOp(uint64(climateInterval*40), 0); d > -10 || d < -11 {
		t.Fatalf("10 C melts %d a step, want -10 or -11", d)
	}
}

func TestSnowSlowsWalkingPerCell(t *testing.T) {
	t.Parallel()
	g := snowGame(t, -50)
	g.setSnowCell(snowIdx(g, 8, 8), 200)
	g.setSnowCell(snowIdx(g, 9, 8), 60)
	if got := g.weatherSlow(8, 8, 10); got != 12 {
		t.Fatalf("deep snow step %d, want 12", got)
	}
	if got := g.weatherSlow(9, 8, 10); got != 10 {
		t.Fatalf("shallow snow step %d, want 10", got)
	}
	if got := g.weatherSlow(20, 8, 10); got != 10 {
		t.Fatalf("bare ground step %d, want 10", got)
	}
}

func TestGrassRegrowthPausesPerSnowyCell(t *testing.T) {
	t.Parallel()
	g := mildGame()
	x, y := grassTile(t, g, 2, 2, 20, 12)
	x2, y2 := grassTile(t, g, x+3, 2, 30, 12)
	regrowNow(t, g, x, y)
	regrowNow(t, g, x2, y2)
	for range clock.TicksPerDay / climateInterval {
		g.setSnowCell(snowIdx(g, x, y), 200) // 15 C melts it: keep it lying
		g.Step(climateInterval)
	}
	if got := g.grassGrowthAt(x, y); got != plant.GrassShoots {
		t.Fatalf("snowed grass growth %d, want paused at %d", got, plant.GrassShoots)
	}
	if got := g.grassGrowthAt(x2, y2); got <= plant.GrassShoots {
		t.Fatalf("bare grass growth %d, want growing", got)
	}
}

func TestSnowViewPerCell(t *testing.T) {
	t.Parallel()
	g := snowGame(t, -50)
	var w WorldView
	g.fillSnowView(&w)
	if w.Snow != nil || w.SnowClear != nil {
		t.Fatal("no snow, no area: empty layers")
	}
	g.setSnowCell(snowIdx(g, 8, 8), 90)
	g.paintSnowClear(1, 1, 2, 3, true)
	var v WorldView
	g.fillSnowView(&v)
	if len(v.Snow) != 40*20 || v.Snow[snowIdx(g, 8, 8)] != 90 || v.Snow[snowIdx(g, 9, 8)] != 0 {
		t.Fatal("view snow per cell")
	}
	if len(v.SnowClear) != 40*20 || v.SnowClear[snowIdx(g, 2, 3)] != 1 || v.SnowClear[snowIdx(g, 3, 3)] != 0 {
		t.Fatal("view snow clearing area per cell")
	}
	if v.SnowVer == w.SnowVer || v.SnowClearVer == w.SnowClearVer {
		t.Fatal("versions move with their layers")
	}
	var u WorldView
	g.fillSnowView(&u)
	if &u.Snow[0] != &v.Snow[0] || u.SnowVer != v.SnowVer {
		t.Fatal("an unchanged layer is shared, not copied")
	}
}

func TestSnowSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := snowGame(t, -150)
	g.SetWeather(weather.Snow, 0.8)
	climateSteps(g, 6)
	g.paintSnowClear(3, 3, 6, 5, true)
	g.Step(37) // mid-interval
	var buf bytes.Buffer
	if err := g.Save(&buf, "snow"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(h.snow.depth, g.snow.depth) || !slices.Equal(h.snow.clear, g.snow.clear) {
		t.Fatal("snow layer or clearing area changed by a save")
	}
	if h.Weather().SnowDepth != g.Weather().SnowDepth {
		t.Fatalf("global depth %v after load, want %v", h.Weather().SnowDepth, g.Weather().SnowDepth)
	}
	h.climate = g.climate
	h.HoldWeather()
	g.Step(500)
	h.Step(500)
	if !slices.Equal(h.snow.depth, g.snow.depth) {
		t.Fatal("a loaded game snows differently from the one it was saved from")
	}
}

func TestOldSaveSeedsSnowFromGlobal(t *testing.T) {
	t.Parallel()
	g := snowGame(t, -150)
	roofOver(g, 2, 2, 4, 4)
	g.snow = snowState{} // an older save: one map-wide depth
	g.weather.SnowDepth = 0.4
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if d := h.snowAt(10, 10); d != 102 {
		t.Fatalf("before the first step open ground reads %d, want 102", d)
	}
	h.climate = g.climate
	h.HoldWeather()
	h.Step(climateInterval)
	if h.snow.depth == nil {
		t.Fatal("the global depth seeds the layer")
	}
	if d := h.snowAt(10, 10); d != 102 {
		t.Fatalf("open cell seeded to %d, want 102", d)
	}
	if d := h.snowAt(3, 3); d != 0 {
		t.Fatalf("roofed cell seeded to %d, want 0", d)
	}
}

func TestSnowDeterministic(t *testing.T) {
	t.Parallel()
	run := func() []uint8 {
		g := snowGame(t, -20)
		g.SetWeather(weather.Blizzard, 0.9)
		climateSteps(g, 40)
		g.SetWeather(weather.Clear, 0)
		g.climate = temperature.Climate{YearMean: 30}
		climateSteps(g, 10)
		return slices.Clone(g.snow.depth)
	}
	a, b := run(), run()
	if a == nil || !slices.Equal(a, b) {
		t.Fatal("two identical runs differ")
	}
}

// BenchmarkSnowClimateStep500 is one climate step's worth of snow chunk
// passes (climateInterval ticks) on a fully snowed 500x500 map while it
// snows: every chunk is visited once. Budget: 1 ms.
func BenchmarkSnowClimateStep500(b *testing.B) {
	g := New(Config{Seed: 1, Width: 500, Height: 500})
	g.climate = temperature.Climate{YearMean: -100}
	g.HoldWeather()
	g.SetWeather(weather.Snow, 1)
	g.weather.SnowDepth = 0.3
	g.seedSnow()
	g.ensureRooms()
	base := uint64(g.clock.Now()) - uint64(g.clock.Now())%climateInterval
	b.ResetTimer()
	for range b.N {
		for ph := range uint64(climateInterval) {
			g.snowChunksAt(base + ph)
		}
	}
}
