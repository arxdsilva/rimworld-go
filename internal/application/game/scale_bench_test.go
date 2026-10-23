package game_test

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"slices"
	"strconv"
	"testing"
	"time"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scenario"
)

// Engine-scale benchmarks (NG-1 §6.2, E0). Grown-colony fixtures are
// built once per process (minutes at 500² on today's code); set
// RIMWORLD_FIXTURE_DIR to keep them on disk between runs:
//
//	RIMWORLD_FIXTURE_DIR=/tmp/fx go test ./internal/application/game \
//	    -run '^$' -bench 'TickGrown/500' -benchtime 3000x -cpuprofile cpu.out

const benchSeed = 1

func init() {
	if d := os.Getenv("RIMWORLD_FIXTURE_DIR"); d != "" {
		scenario.CacheDir = d
	}
}

func loadFixture(b *testing.B, side int, raid bool) *game.Game {
	b.Helper()
	noSelfCheck(b)
	data := scenario.GrownColony(benchSeed, side, side, raid)
	g, err := scenario.Load(data)
	if err != nil {
		b.Fatal(err)
	}
	return g
}

// noSelfCheck turns the tests' index self-check off for a benchmark.
func noSelfCheck(b *testing.B) {
	game.SetIndexSelfCheck(false)
	b.Cleanup(func() { game.SetIndexSelfCheck(true) })
}

// benchTicks times each tick and reports ms/tick, p99-ms/tick and the
// worst three consecutive ticks (one 3x-speed frame), plus steady-B/tick:
// bytes allocated per tick over the run's second half (B/op also counts
// the one-off buffers the first ticks after a load build).
func benchTicks(b *testing.B, g *game.Game) {
	durs := make([]time.Duration, 0, b.N)
	var m0, m1 runtime.MemStats
	half := b.N / 2
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i == half {
			runtime.ReadMemStats(&m0)
		}
		t0 := time.Now()
		g.Step(1)
		durs = append(durs, time.Since(t0))
	}
	b.StopTimer()
	runtime.ReadMemStats(&m1)
	reportTicks(b, durs)
	if n := b.N - half; n > 0 && b.N >= 100 {
		b.ReportMetric(float64(m1.TotalAlloc-m0.TotalAlloc)/float64(n), "steady-B/tick")
	}
}

func reportTicks(b *testing.B, durs []time.Duration) {
	if len(durs) == 0 {
		return
	}
	var sum time.Duration
	worst3 := time.Duration(0)
	for i, d := range durs {
		sum += d
		if i >= 2 {
			worst3 = max(worst3, durs[i]+durs[i-1]+durs[i-2])
		}
	}
	sorted := slices.Clone(durs)
	slices.Sort(sorted)
	p99 := sorted[min(len(sorted)-1, len(sorted)*99/100)]
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	b.ReportMetric(ms(sum)/float64(len(durs)), "ms/tick")
	b.ReportMetric(ms(p99), "p99-ms/tick")
	b.ReportMetric(ms(worst3), "max-ms/3ticks")
}

// BenchmarkTickGrown steps a grown colony one tick at a time.
func BenchmarkTickGrown(b *testing.B) {
	for _, side := range []int{120, 250, 500} {
		b.Run(strconv.Itoa(side), func(b *testing.B) {
			benchTicks(b, loadFixture(b, side, false))
		})
	}
}

// BenchmarkTickRaid steps a grown colony under a 12-raider assault.
func BenchmarkTickRaid(b *testing.B) {
	b.Run("500", func(b *testing.B) { benchTicks(b, loadFixture(b, 500, true)) })
}

// BenchmarkReadModel builds both read models once per frame with nothing
// changed since the last frame (the renderer's steady state).
// "500/cached" is the same with the view cache on (SetViewCache: the
// renderer's setting), "500/tick" pays one tick of state change per
// frame (World and Snapshot rebuilt, per-tile layers shared).
func BenchmarkReadModel(b *testing.B) {
	b.Run("500/cached", func(b *testing.B) {
		g := loadFixture(b, 500, false)
		g.SetViewCache(true)
		benchViews(b, g, false)
	})
	b.Run("500/tick", func(b *testing.B) {
		g := loadFixture(b, 500, false)
		g.SetViewCache(true)
		benchViews(b, g, true)
	})
	b.Run("500", func(b *testing.B) {
		g := loadFixture(b, 500, false)
		g.Step(1)
		_, _ = g.World(), g.Snapshot()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := g.World()
			p := g.Snapshot()
			if len(w.Terrain) == 0 || len(p) == 0 {
				b.Fatal("empty read model")
			}
		}
	})
}

func benchViews(b *testing.B, g *game.Game, step bool) {
	g.Step(1)
	_, _ = g.World(), g.Snapshot()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if step {
			b.StopTimer()
			g.Step(1)
			b.StartTimer()
		}
		w := g.World()
		p := g.Snapshot()
		if len(w.Terrain) == 0 || len(p) == 0 {
			b.Fatal("empty read model")
		}
	}
}

// BenchmarkGenerate500 generates a new 500² colony.
func BenchmarkGenerate500(b *testing.B) {
	noSelfCheck(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = game.NewColony(game.Config{Seed: uint64(i + 1), Width: 500, Height: 500})
	}
}

// BenchmarkSave500 saves the grown 500² colony.
func BenchmarkSave500(b *testing.B) {
	g := loadFixture(b, 500, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := g.Save(io.Discard, "bench"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoad500 loads the grown 500² colony.
func BenchmarkLoad500(b *testing.B) {
	noSelfCheck(b)
	data := scenario.GrownColony(benchSeed, 500, 500, false)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := game.Load(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}

// farCorner is a walkable tile near the given corner of the map.
func farCorner(b *testing.B, g *game.Game, fromRight, fromBottom bool) (int, int) {
	b.Helper()
	w, h := g.Width(), g.Height()
	for r := 2; r < w/2; r++ {
		x, y := r, r
		if fromRight {
			x = w - 1 - r
		}
		if fromBottom {
			y = h - 1 - r
		}
		if g.TerrainAt(x, y) != "rock" && g.BenchReachable(g.BenchColonist(), x, y) {
			return x, y
		}
	}
	b.Fatal("no reachable corner tile")
	return 0, 0
}

// BenchmarkReachable500: can a colonist reach a far corner?
func BenchmarkReachable500(b *testing.B) {
	g := loadFixture(b, 500, false)
	id := g.BenchColonist()
	x, y := farCorner(b, g, false, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !g.BenchReachable(id, x, y) {
			b.Fatal("unreachable")
		}
	}
}

// BenchmarkPath500Long: a colonist's path to the far bottom-right corner.
func BenchmarkPath500Long(b *testing.B) {
	g := loadFixture(b, 500, false)
	id := g.BenchColonist()
	x, y := farCorner(b, g, true, true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if g.BenchPath(id, x, y) < 0 {
			b.Fatal("no path")
		}
	}
}

// BenchmarkPathUnreachable500: a colonist's path to a tile sealed inside
// a wall ring (today: floods the whole reachable component).
func BenchmarkPathUnreachable500(b *testing.B) {
	g := loadFixture(b, 500, false)
	id := g.BenchColonist()
	x, y := farCorner(b, g, true, false)
	x, y = x-3, y+3
	g.ClearPlants(x-2, y-2, x+2, y+2)
	g.SetGround(x-2, y-2, x+2, y+2, "soil", "", "")
	for dx := -2; dx <= 2; dx++ {
		for dy := -2; dy <= 2; dy++ {
			if dx == -2 || dx == 2 || dy == -2 || dy == 2 {
				if !g.BuildNow("wall", "granite blocks", "south", x+dx, y+dy) {
					b.Fatalf("wall at %d,%d", x+dx, y+dy)
				}
			}
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if g.BenchPath(id, x, y) >= 0 {
			b.Fatal("sealed tile reached")
		}
	}
}

// BenchmarkRoomsAfterWall500: the room rebuild a wall change triggers.
func BenchmarkRoomsAfterWall500(b *testing.B) {
	g := loadFixture(b, 500, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if g.BenchRebuildRooms() == 0 {
			b.Fatal("no rooms")
		}
	}
}
