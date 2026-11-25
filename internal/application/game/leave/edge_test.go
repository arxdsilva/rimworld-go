// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package leave

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

type routeHost struct {
	moves     []job.Record
	breach    [][2]int
	northRock bool
}

func (*routeHost) Size() (int, int)     { return 40, 30 }
func (f *routeHost) Free(x, y int) bool { return y != 0 || !f.northRock }
func (f *routeHost) StartJob(_ entity.EntityID, j job.Record) bool {
	f.moves = append(f.moves, j)
	return j.TX == 0
}
func (f *routeHost) StartBreach(_ entity.EntityID, x, y int) bool {
	f.breach = append(f.breach, [2]int{x, y})
	return false
}
func (*routeHost) BreachRoute(entity.EntityID, int, int) bool { return false }

func TestRetrySkipsUnreachableEdgeAndChoosesAnother(t *testing.T) {
	h := &routeHost{}
	failed := job.Record{Kind: job.Move, State: job.Fail, Fail: job.Unreachable, TX: 20, TY: 0}
	if !Try(h, 1, 20, 6, 0, failed) {
		t.Fatal("no alternate exit found")
	}
	if len(h.moves) == 0 || h.moves[len(h.moves)-1].TX != 0 {
		t.Fatalf("retried an unreachable north exit: %+v", h.moves)
	}
}

func TestEdgeRockFallsBackToOtherSides(t *testing.T) {
	h := &routeHost{northRock: true}
	if !Try(h, 1, 20, 6, 0, job.Record{}) {
		t.Fatal("rock edge stopped planning")
	}
	if len(h.moves) == 0 || h.moves[len(h.moves)-1].TX != 0 {
		t.Fatalf("wrong exit %v", h.moves)
	}
}
