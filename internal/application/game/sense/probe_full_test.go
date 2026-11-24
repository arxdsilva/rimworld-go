// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package sense

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

const probeTiles = 40

func probeQueue() []int32 {
	q := make([]int32, probeTiles)
	for i := range q {
		q[i] = int32(i)
	}
	return q
}

type fullFilth struct {
	Filth
	q []int32
}

func (f *fullFilth) Queue() []int32                           { return f.q }
func (f *fullFilth) SetQueue(q []int32)                       { f.q = q }
func (f *fullFilth) CleanFree(entity.EntityID, int, int) bool { return true }
func (f *fullFilth) Dirt(int) int                             { return 1 }
func (f *fullFilth) Stain(int) int                            { return 0 }

type fullGround struct{ Ground }

func (fullGround) Width() int { return probeTiles }
func (fullGround) HomeCells() []uint8 {
	h := make([]uint8, probeTiles)
	for i := range h {
		h[i] = 1
	}
	return h
}

type fullWalks struct {
	Walks
	started int
}

func (fullWalks) Pos(entity.EntityID) (int, int) { return 0, 0 }
func (fullWalks) ReachableSpot(_ entity.EntityID, x, y int) (int, int, bool) {
	return x, y, true
}
func (w *fullWalks) StartJob(entity.EntityID, job.Record) bool { w.started++; return true }
func (fullWalks) ClaimTile(entity.EntityID, int, int)          {}

func TestAssignFullProbeBufferSkipsFartherTiles(t *testing.T) {
	t.Parallel()
	f := &fullFilth{q: probeQueue()}
	w := &fullWalks{}
	c := NewCleaning(f, fullGround{}, w, nil)
	if !c.Assign(1) || w.started != 1 {
		t.Fatalf("assigned %d jobs", w.started)
	}
	if len(f.q) != probeTiles {
		t.Fatalf("queue kept %d, want all %d", len(f.q), probeTiles)
	}
}

func TestTidyListStopsAtItsLimit(t *testing.T) {
	t.Parallel()
	const limit = 3
	r := &tidyProbe{q: probeQueue()}
	td := Tidy{r: r}
	var buf [maxTidyCells]tidyCell
	got := td.list(1, 0, probeTiles, 0, 0, limit, buf[:])
	if len(got) != limit {
		t.Fatalf("listed %d, want %d", len(got), limit)
	}
	for i, c := range got {
		if int(c.i) != i {
			t.Errorf("slot %d holds tile %d, want the nearest", i, c.i)
		}
	}
}

type tidyProbe struct {
	TidyRoles
	q []int32
}

func (r *tidyProbe) Queue() []int32                           { return r.q }
func (r *tidyProbe) RoomGrid() []int                          { return make([]int, probeTiles) }
func (r *tidyProbe) Width() int                               { return probeTiles }
func (r *tidyProbe) HomeCells() []uint8                       { return fullGround{}.HomeCells() }
func (r *tidyProbe) CleanFree(entity.EntityID, int, int) bool { return true }
