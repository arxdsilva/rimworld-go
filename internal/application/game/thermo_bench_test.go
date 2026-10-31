package game

import (
	"slices"
	"testing"
	"time"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

// climateBenchGame is a 250x250 map tiled with 5x5 roofed rooms (961 of
// them), a fuelled campfire in every fourth, 60 colonists about.
func climateBenchGame(b *testing.B) *Game {
	b.Helper()
	SetIndexSelfCheck(false) // the self-check's full room detects would dominate the setup
	b.Cleanup(func() { SetIndexSelfCheck(true) })
	g := New(Config{Seed: 1, Width: 250, Height: 250})
	g.warmIndexes(false)
	n := 0
	for y0 := 2; y0+6 < 250; y0 += 8 {
		for x0 := 2; x0+6 < 250; x0 += 8 {
			walledRoom(g, x0, y0, x0+4, y0+4)
			if n%4 == 0 {
				id := g.placeGenerated(construction.Building{Kind: construction.Campfire, Stuff: item.Wood, X: x0 + 2, Y: y0 + 2})
				g.setFuel(id, g.fuelCap(id))
			}
			n++
		}
	}
	g.RoofRooms()
	for i := range 60 {
		if _, err := g.SpawnPawn(1+i%248, 0); err != nil {
			b.Fatal(err)
		}
	}
	g.ensureRooms()
	if len(g.rooms) < 900 {
		b.Fatalf("%d rooms", len(g.rooms))
	}
	return g
}

// BenchmarkClimateStep is one room equalisation step (heatRooms) on the
// many-room map; p99-us is the 99th percentile step, recount-us the step
// after a roof change (exposure recounted from the map).
func BenchmarkClimateStep(b *testing.B) {
	g := climateBenchGame(b)
	g.heatRooms()
	times := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for range b.N {
		t0 := time.Now()
		g.heatRooms()
		times = append(times, time.Since(t0))
	}
	b.StopTimer()
	slices.Sort(times)
	b.ReportMetric(float64(times[len(times)*99/100].Microseconds()), "p99-us")
	g.roofx.ver++ // a roof changed: the next step recounts
	t0 := time.Now()
	g.heatRooms()
	b.ReportMetric(float64(time.Since(t0).Microseconds()), "recount-us")
}
