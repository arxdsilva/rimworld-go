package game

import (
	"testing"

	"rimworld-go/internal/application/game/pathstage"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/pool"
)

// repathHerd is a 500 x 500 map with a 1-wide corridor (row 250, walled
// on rows 249 and 251 from x 100 to 400) holding 64 wild animals nose to
// tail, each walking east to (450, 250), and a wild animal standing still
// in front of them: every one is due a search around the creature ahead.
type repathHerd struct {
	g     *Game
	herd  []entity.EntityID
	paths [][]pathing.Point
	still entity.EntityID
}

func newRepathHerd(b *testing.B, p pathstage.Pool) *repathHerd {
	b.Helper()
	SetIndexSelfCheck(false) // time the searches, not the self-check scans
	b.Cleanup(func() { SetIndexSelfCheck(true) })
	g := New(Config{Seed: 1, Width: 500, Height: 500})
	bareGround(g)
	for x := 100; x <= 400; x++ {
		g.grid.SetWalkable(x, 249, false)
		g.grid.SetWalkable(x, 251, false)
	}
	h := &repathHerd{g: g}
	for i := range 64 {
		id, err := g.SpawnAnimal("dustbuck", 200+i, 250)
		if err != nil {
			b.Fatal(err)
		}
		a, _ := g.animals.Get(id)
		g.animalGoTo(id, &a, 450, 250)
		if len(a.Path) == 0 {
			b.Fatal("no walk east")
		}
		g.animals.Set(id, a)
		h.herd, h.paths = append(h.herd, id), append(h.paths, a.Path)
	}
	id, err := g.SpawnAnimal("dustbuck", 264, 250)
	if err != nil {
		b.Fatal(err)
	}
	h.still = id
	g.regs.paths.SetPool(p)
	return h
}

// reset puts the herd back in the jam, every walker on a search tick.
func (h *repathHerd) reset() {
	g := h.g
	for i, id := range h.herd {
		a, _ := g.animals.Get(id)
		a.Path, a.PathI, a.RetryAt = h.paths[i], 0, 0
		g.animals.Set(id, a)
		g.positions.Set(id, Position{X: 200 + i, Y: 250})
		g.motion.Set(id, motionRec{BlockedTicks: repathAfter, BlockedBy: id + 1})
	}
	a, _ := g.animals.Get(h.still)
	a.Path, a.PathI = nil, 0
	g.animals.Set(h.still, a)
	g.positions.Set(h.still, Position{X: 264, Y: 250})
	g.motion.Set(h.still, motionRec{})
}

// BenchmarkAnimalRepath500 times one movement phase of the jammed herd
// (64 searches queued on the path stage), serially and on the pool.
func BenchmarkAnimalRepath500(b *testing.B) {
	for _, c := range []struct {
		name  string
		lanes int
	}{{"serial", 0}, {"pool8", pool.MaxLanes}} {
		b.Run(c.name, func(b *testing.B) {
			var p pathstage.Pool
			if c.lanes > 0 {
				pl := pool.New(pool.Options{Lanes: c.lanes})
				b.Cleanup(pl.Close)
				p = pl
			}
			h := newRepathHerd(b, p)
			_, n0 := h.g.regs.paths.Stats()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				h.reset()
				b.StartTimer()
				h.g.phaseMovement()
			}
			b.StopTimer()
			_, n1 := h.g.regs.paths.Stats()
			if per := float64(n1-n0) / float64(b.N); per < 30 {
				b.Fatalf("%.1f searches a phase, want at least 30", per)
			} else {
				b.ReportMetric(per, "searches/op")
			}
		})
	}
}
