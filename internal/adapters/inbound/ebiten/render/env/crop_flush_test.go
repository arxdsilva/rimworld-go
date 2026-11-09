package env

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// A draw that misses every queued crop quad leaves the batch open (so the
// quads keep merging into one command); one that touches a quad flushes it
// first (PERF-009).
func TestCropFlushOnlyOnOverlap(t *testing.T) {
	t.Parallel()
	w := soilWorld(16, 12)
	w.Plants = []game.PlantView{
		{X: 4, Y: 3, Species: "corn", Zone: "corn", Growth: 60, Stage: 2},
		{X: 5, Y: 3, Species: "wheat", Zone: "wheat", Growth: 100, Stage: 3},
	}
	rig := newLayerRig(t, w, 16, 12)
	e, v := rig.e, rig.v
	e.Crops.Begin()
	e.FlushCropsOver(rig.dst, 0, 0, 1e6, 1e6) // nothing queued: a no-op
	for i := range w.Plants {
		e.DrawCropObj(v, rig.dst, &w, &w.Plants[i], rig.tile)
	}
	cb := e.Crops
	if cb.Quads == 0 {
		t.Fatal("no crop units queued")
	}
	x0, y0, x1, y1 := cb.bx0, cb.by0, cb.bx1, cb.by1
	if x1 <= x0 || y1 <= y0 {
		t.Fatalf("empty box %v %v %v %v", x0, y0, x1, y1)
	}
	queued, base := len(cb.Idx), cb.Flushes // a new species flushed once while queueing
	for _, far := range [][4]float32{
		{x1 + 3, y0, x1 + 50, y1},            // east
		{x0 - 50, y0, x0 - 3, y1},            // west
		{x0, y1 + 3, x1, y1 + 50},            // south
		{x0, y0 - 50, x1, y0 - 3},            // north
		{x1 + 1.5, y1 + 1.5, x1 + 9, y1 + 9}, // diagonal, past the 1 px widening
	} {
		e.FlushCropsOver(rig.dst, far[0], far[1], far[2], far[3])
		if cb.Flushes != base || len(cb.Idx) != queued {
			t.Fatalf("a draw at %v missed the units but flushed (%d flushes, %d of %d indexes)", far, cb.Flushes, len(cb.Idx), queued)
		}
	}
	// more units grow the box: a draw that missed the first plant but touches the second must flush
	e.FlushCropsOver(rig.dst, x1-1, y1-1, x1+4, y1+4) // touches a corner
	if cb.Flushes != base+1 || len(cb.Idx) != 0 {
		t.Fatalf("an overlapping draw must flush first: %d flushes, %d indexes", cb.Flushes, len(cb.Idx))
	}
	// the box starts afresh after a flush: the corn lies west of the wheat
	// that made the first one
	e.DrawCropObj(v, rig.dst, &w, &w.Plants[0], rig.tile)
	if cb.bx1 > x1 || cb.bx1 <= cb.bx0 {
		t.Fatalf("stale box: %v..%v after %v..%v", cb.bx0, cb.bx1, x0, x1)
	}
}
