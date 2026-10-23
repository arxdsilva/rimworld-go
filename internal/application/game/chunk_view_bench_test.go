package game_test

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// BenchmarkChunkLayer500 is the renderer's per-tick read-model cost at
// 500² (NG-1 §6.7 E5): after each tick, the first ChunkLayer call
// buckets the new World() by chunk and hashes the changed chunks; the
// rest of the frame's calls are lookups. World() itself is not timed.
func BenchmarkChunkLayer500(b *testing.B) {
	g := loadFixture(b, 500, false)
	g.SetViewCache(true)
	g.ChunkLayer(0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		g.Step(1)
		g.World()
		b.StartTimer()
		for cy := 14; cy < 18; cy++ { // a 16 px view: ~6×4 chunks
			for cx := 13; cx < 19; cx++ {
				g.ChunkLayer(cx, cy)
			}
		}
	}
}

// BenchmarkMinimapLayer500 is the minimap poll after a tick.
func BenchmarkMinimapLayer500(b *testing.B) {
	g := loadFixture(b, 500, false)
	g.SetViewCache(true)
	v, _ := g.MinimapLayer(0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		g.Step(1)
		g.World()
		g.ChunkLayer(0, 0)
		b.StartTimer()
		if n, ok := g.MinimapLayer(v.Ver); ok {
			v = n
		}
	}
	_ = game.MinimapView{}
}
