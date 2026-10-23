package game

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

// nearFromCells recounts occupancy.near from scratch.
func nearFromCells(o *occupancy) []uint8 {
	near := make([]uint8, len(o.cells))
	for i, id := range o.cells {
		if id == 0 {
			continue
		}
		x, y := i%o.w, i/o.w
		for ny := max(y-1, 0); ny <= min(y+1, o.h-1); ny++ {
			for nx := max(x-1, 0); nx <= min(x+1, o.w-1); nx++ {
				near[ny*o.w+nx]++
			}
		}
	}
	return near
}

// TestOccupancyNearMatchesCells: the held-cells-around counts crowdCost
// skips empty ground with stay equal to a recount through rebuilds, moves
// and stack hand-overs of a busy colony.
func TestOccupancyNearMatchesCells(t *testing.T) {
	g := NewColony(Config{Seed: 7, Width: 48, Height: 40})
	for tick := 0; tick < 3000; tick++ {
		g.Step(1)
		o := &g.occ
		if o.cells == nil || o.dirty {
			continue
		}
		want := nearFromCells(o)
		for i := range want {
			if o.near[i] != want[i] {
				t.Fatalf("tick %d cell %d: near %d, recount %d", tick, i, o.near[i], want[i])
			}
		}
	}
}

// TestOccupancyPutStacked: a stacked creature taking a cell over keeps
// near unchanged; the last one leaving clears it.
func TestOccupancyPutStacked(t *testing.T) {
	g := occTestGame()
	a := mustPawn(t, g, 5, 5)
	b := mustPawn(t, g, 5, 5) // stacked on a
	rebuild(g)
	o := &g.occ
	i := 5*o.w + 5
	if o.near[i] != 1 {
		t.Fatalf("near = %d with two creatures on one cell, want 1", o.near[i])
	}
	o.move(a, Position{X: 5, Y: 5}, Position{X: 6, Y: 5})
	if o.cells[i] != b || o.near[i] != 2 {
		t.Fatalf("after a left: cell holds %d, near %d; want %d, 2", o.cells[i], o.near[i], b)
	}
	o.move(b, Position{X: 5, Y: 5}, Position{X: 4, Y: 5})
	if o.cells[i] != entity.EntityID(0) {
		t.Fatalf("cell still held by %d", o.cells[i])
	}
	want := nearFromCells(o)
	for k := range want {
		if o.near[k] != want[k] {
			t.Fatalf("cell %d: near %d, recount %d", k, o.near[k], want[k])
		}
	}
}
