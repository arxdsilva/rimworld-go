package sense

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/entity"
)

// wanderWorld is a flat map for strolls: a pawn at (px, py), a roof over
// x >= roofX, a counted random stream, and private cells.
type wanderWorld struct {
	px, py, roofX int
	hide          bool
	rude          map[[2]int]bool
	draws         int
	seq           uint64
	started       []job.Record
}

func (w *wanderWorld) Pos(entity.EntityID) (int, int)                             { return w.px, w.py }
func (w *wanderWorld) SetJob(entity.EntityID, job.Record)                         {}
func (w *wanderWorld) Reachable(entity.EntityID, int, int) bool                   { return true }
func (w *wanderWorld) ReachableSpot(_ entity.EntityID, x, y int) (int, int, bool) { return x, y, true }
func (w *wanderWorld) PostponeTile(entity.EntityID, job.Kind, int, int)           {}
func (w *wanderWorld) ClaimTile(entity.EntityID, int, int)                        {}
func (w *wanderWorld) TileTaken(int, int) bool                                    { return false }
func (w *wanderWorld) StartJob(_ entity.EntityID, r job.Record) bool {
	w.started = append(w.started, r)
	return true
}
func (w *wanderWorld) Roams(entity.EntityID) bool { return false }
func (w *wanderWorld) Roll(n int) int {
	w.draws++
	w.seq = w.seq*6364136223846793005 + 1442695040888963407
	return int(w.seq>>33) % n
}
func (w *wanderWorld) OpenTile(_ entity.EntityID, x, y int) bool { return x >= 0 && y >= 0 }
func (w *wanderWorld) ShouldHide(entity.EntityID) bool           { return w.hide }
func (w *wanderWorld) Roofed(x, y int) bool                      { return x >= w.roofX }
func (w *wanderWorld) Home(x, y int) bool                        { return true }
func (w *wanderWorld) Rude(_ entity.EntityID, x, y int) bool     { return w.rude[[2]int{x, y}] }

func (w *wanderWorld) wander(rules gameplay.Settings) Wander {
	return NewWander(w, w, rules, w, w)
}

var wanderOn = gameplay.Settings{}.With(gameplay.ShelterWander, true).With(gameplay.PoliteWander, true)

// TestWanderOffDrawsSameNumbers: with both rules off the stream and the
// pick are exactly the old stroll's, whatever the roles say.
func TestWanderOffDrawsSameNumbers(t *testing.T) {
	off := gameplay.Settings{}.With(gameplay.ShelterWander, false).With(gameplay.PoliteWander, false)
	a := &wanderWorld{px: 10, py: 10, roofX: 100, hide: true, rude: map[[2]int]bool{{11, 10}: true}}
	b := &wanderWorld{px: 10, py: 10, roofX: 0}
	for range 30 {
		ra, rb := a.wander(off).Pick(1, 50), b.wander(off).Pick(1, 50)
		if ra != rb {
			t.Fatal("picks differ")
		}
	}
	if a.draws != b.draws || len(a.started) != len(b.started) {
		t.Fatalf("draws %d/%d starts %d/%d", a.draws, b.draws, len(a.started), len(b.started))
	}
	for i := range a.started {
		if x, y := a.started[i], b.started[i]; x.TX != y.TX || x.TY != y.TY || x.WorkLeft != y.WorkLeft {
			t.Fatalf("pick %d: %+v vs %+v", i, a.started[i], b.started[i])
		}
	}
}

// TestWanderStaysRoofedInRain: roofed in the rain, every stroll ends roofed.
func TestWanderStaysRoofedInRain(t *testing.T) {
	w := &wanderWorld{px: 10, py: 10, roofX: 7, hide: true}
	for range 60 {
		w.wander(wanderOn).Pick(1, 50)
	}
	if len(w.started) == 0 {
		t.Fatal("never strolled")
	}
	for _, r := range w.started {
		if r.TX < 7 {
			t.Fatalf("stroll to open sky %+v", r)
		}
	}
}

// TestWanderSeeksRoofInFallout: out under the sky, the stroll is to the
// nearest roofed cell; with no roof within reach, it stays put.
func TestWanderSeeksRoofInFallout(t *testing.T) {
	w := &wanderWorld{px: 4, py: 5, roofX: 8, hide: true}
	if !w.wander(wanderOn).Pick(1, 50) {
		t.Fatal("no walk to shelter")
	}
	if r := w.started[0]; r.TX != 8 {
		t.Fatalf("went to %+v, want the nearest roofed column", r)
	}
	far := &wanderWorld{px: 4, py: 5, roofX: 100, hide: true}
	if far.wander(wanderOn).Pick(1, 50) || len(far.started) != 0 {
		t.Fatal("no roof near: stay put")
	}
}

// TestShelterDoesNotFlap: once under a roof in bad weather the pawn never
// strolls back out, and clear weather frees it: no ping-pong.
func TestShelterDoesNotFlap(t *testing.T) {
	w := &wanderWorld{px: 4, py: 5, roofX: 8, hide: true}
	w.wander(wanderOn).Pick(1, 50)
	w.px = w.started[0].TX
	w.started = nil
	for range 100 {
		w.wander(wanderOn).Pick(1, 50)
	}
	for _, r := range w.started {
		if r.TX < 8 {
			t.Fatalf("left the roof: %+v", r)
		}
	}
	w.hide, w.started = false, nil
	out := false
	for range 100 {
		w.wander(wanderOn).Pick(1, 50)
	}
	for _, r := range w.started {
		out = out || r.TX < 8
	}
	if !out {
		t.Fatal("clear sky should free the pawn again")
	}
}

// TestWanderAvoidsCell: a rude cell is never picked, and the others are.
func TestWanderAvoidsCell(t *testing.T) {
	w := &wanderWorld{px: 10, py: 10, rude: map[[2]int]bool{}}
	for x := 5; x <= 15; x++ {
		for y := 5; y <= 15; y++ {
			if x < 10 {
				w.rude[[2]int{x, y}] = true
			}
		}
	}
	for range 80 {
		w.wander(wanderOn).Pick(1, 50)
	}
	if len(w.started) == 0 {
		t.Fatal("never strolled")
	}
	for _, r := range w.started {
		if r.TX < 10 {
			t.Fatalf("entered a private cell %+v", r)
		}
	}
}

func BenchmarkWanderPick(b *testing.B) {
	for _, tc := range []struct {
		name  string
		rules gameplay.Settings
		hide  bool
	}{{"off", gameplay.Settings{}, false}, {"on-fair", wanderOn, false}, {"on-rain", wanderOn, true}} {
		b.Run(tc.name, func(b *testing.B) {
			w := &wanderWorld{px: 10, py: 10, roofX: 7, hide: tc.hide}
			c := w.wander(tc.rules)
			b.ReportAllocs()
			for b.Loop() {
				w.started = w.started[:0]
				c.Pick(1, 50)
			}
		})
	}
}
