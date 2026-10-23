package game

import (
	"fmt"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

// nearFromCells recounts occupancy.near (blocker cells) from scratch.
func nearFromCells(o *occupancy) []uint8 { return nearFrom(o, o.cells) }

// pnearFromCells recounts occupancy.pnear (partner cells) from scratch.
func pnearFromCells(o *occupancy) []uint8 { return nearFrom(o, o.pfirst) }

func nearFrom(o *occupancy, held []entity.EntityID) []uint8 {
	near := make([]uint8, len(held))
	for i, id := range held {
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

// checkPartnerLayer: every partner cell's count is its first partner plus
// the extras recorded on it, and a cell has a first partner iff it counts
// any.
func checkPartnerLayer(t *testing.T, o *occupancy, when string) {
	t.Helper()
	extra := map[int]int{}
	for _, s := range o.pextra {
		extra[s.idx]++
		if s.id == o.pfirst[s.idx] {
			t.Fatalf("%s: %d both first and extra on cell %d", when, s.id, s.idx)
		}
	}
	for i, n := range o.pcount {
		want := extra[i]
		if o.pfirst[i] != 0 {
			want++
		}
		if int(n) != want || (n == 0) != (o.pfirst[i] == 0) {
			t.Fatalf("%s cell %d: count %d, first %d, extras %d", when, i, n, o.pfirst[i], extra[i])
		}
	}
}

func checkNear(t *testing.T, o *occupancy, when string) {
	t.Helper()
	want, pwant := nearFromCells(o), pnearFromCells(o)
	for i := range want {
		if o.near[i] != want[i] {
			t.Fatalf("%s cell %d: near %d, recount %d", when, i, o.near[i], want[i])
		}
		if o.pnear[i] != pwant[i] {
			t.Fatalf("%s cell %d: pnear %d, recount %d", when, i, o.pnear[i], pwant[i])
		}
	}
}

// TestOccupancyNearMatchesCells: the held-cells-around counts crowdCost
// skips empty ground with stay equal to a recount through rebuilds, moves
// and stack hand-overs of a busy colony, for blockers and partners alike.
func TestOccupancyNearMatchesCells(t *testing.T) {
	g := NewColony(Config{Seed: 7, Width: 48, Height: 40})
	g.Raid(2)
	for tick := 0; tick < 3000; tick++ {
		g.Step(1)
		o := &g.occ
		if o.cells == nil || o.dirty {
			continue
		}
		when := fmt.Sprintf("tick %d", tick)
		checkNear(t, o, when)
		checkPartnerLayer(t, o, when)
	}
}

// TestOccupancyPutStacked: a stacked blocker taking a cell over keeps
// near unchanged; the last one leaving clears it.
func TestOccupancyPutStacked(t *testing.T) {
	g := occTestGame()
	a := wildAt(t, g, 5, 5)
	b := wildAt(t, g, 5, 5) // stacked on a
	rebuild(g)
	o := &g.occ
	i := 5*o.w + 5
	if o.near[i] != 1 {
		t.Fatalf("near = %d with two creatures on one cell, want 1", o.near[i])
	}
	o.move(a, false, Position{X: 5, Y: 5}, Position{X: 6, Y: 5})
	if o.cells[i] != b || o.near[i] != 2 {
		t.Fatalf("after a left: cell holds %d, near %d; want %d, 2", o.cells[i], o.near[i], b)
	}
	o.move(b, false, Position{X: 5, Y: 5}, Position{X: 4, Y: 5})
	if o.cells[i] != entity.EntityID(0) {
		t.Fatalf("cell still held by %d", o.cells[i])
	}
	checkNear(t, o, "after both left")
}

// TestOccupancyPartnerLayer: partners on one cell are counted, the lowest
// ID is first after a rebuild, the next one takes over when the first
// leaves, and they never show as blockers.
func TestOccupancyPartnerLayer(t *testing.T) {
	g := occTestGame()
	a := mustPawn(t, g, 5, 5)
	b := mustPawn(t, g, 5, 5)
	dog := animalAt(t, g, "husky", 5, 5, true)
	rebuild(g)
	o := &g.occ
	i := 5*o.w + 5
	if o.cells[i] != 0 || o.near[i] != 0 {
		t.Fatalf("partners indexed as blockers: cell %d near %d", o.cells[i], o.near[i])
	}
	if o.pcount[i] != 3 || o.pfirst[i] != a || o.pnear[i] != 1 {
		t.Fatalf("partner cell: count %d first %d pnear %d; want 3, %d, 1", o.pcount[i], o.pfirst[i], o.pnear[i], a)
	}
	if id, ok := g.occupantAt(5, 5); !ok || id != a {
		t.Fatalf("occupantAt = %d, want the first partner %d", id, a)
	}
	o.move(a, true, Position{X: 5, Y: 5}, Position{X: 6, Y: 5})
	if o.pcount[i] != 2 || o.pfirst[i] != b {
		t.Fatalf("after a left: count %d first %d; want 2, %d", o.pcount[i], o.pfirst[i], b)
	}
	o.move(dog, true, Position{X: 5, Y: 5}, Position{X: 6, Y: 5}) // joins a
	o.move(b, true, Position{X: 5, Y: 5}, Position{X: 4, Y: 5})
	if o.pcount[i] != 0 || o.pfirst[i] != 0 {
		t.Fatalf("emptied cell: count %d first %d", o.pcount[i], o.pfirst[i])
	}
	if j := 5*o.w + 6; o.pcount[j] != 2 || o.pfirst[j] != a {
		t.Fatalf("joined cell: count %d first %d", o.pcount[j], o.pfirst[j])
	}
	checkNear(t, o, "after moves")
	checkPartnerLayer(t, o, "after moves")
	// a rebuild starts over from the positions (unchanged here)
	rebuild(g)
	if o.pcount[i] != 3 || o.pfirst[i] != a {
		t.Fatalf("rebuilt: count %d first %d", o.pcount[i], o.pfirst[i])
	}
}
