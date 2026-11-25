// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"fmt"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/planet"
)

type deferredFormFreight struct{ Freight }

func (deferredFormFreight) Pack(uint64, string, int)                {}
func (deferredFormFreight) Hitch(uint64, entity.EntityID, int) bool { return false }
func (deferredFormFreight) Gather(uint64, []entity.EntityID, uint64, uint64) bool {
	return false
}

type deferredFormHost struct {
	Host
	world   *planet.Planet
	tile    int
	drafted bool
	job     job.Record
	starts  int
}

func (h *deferredFormHost) Planet() *planet.Planet { return h.world }
func (h *deferredFormHost) ColonyTile() int        { return h.tile }
func (*deferredFormHost) Current() uint32          { return 0 }
func (*deferredFormHost) Now() uint64              { return 10 }
func (*deferredFormHost) NewID() uint64            { return 511 }
func (*deferredFormHost) Cargo() Freight           { return deferredFormFreight{} }
func (*deferredFormHost) Candidate(id entity.EntityID) bool {
	return id == 7
}
func (h *deferredFormHost) SetDraft(_ entity.EntityID, on bool) { h.drafted = on }
func (h *deferredFormHost) Drafted(entity.EntityID) bool        { return h.drafted }
func (*deferredFormHost) Standing(entity.EntityID) bool         { return true }
func (*deferredFormHost) Breaking(entity.EntityID) bool         { return false }
func (*deferredFormHost) Note(string, ...any)                   {}
func (h *deferredFormHost) Job(entity.EntityID) (job.Record, bool) {
	return h.job, true
}
func (h *deferredFormHost) StartExit(entity.EntityID) bool {
	h.starts++
	h.job = job.Record{Kind: job.ExitMap}
	if h.starts <= 2 {
		h.job.State, h.job.Fail = job.Fail, job.Reserved
		return false
	}
	h.job.State = job.Goto
	return true
}

func TestFormingCaravanRetainsDeferredExitSearch(t *testing.T) {
	t.Parallel()
	p := planet.Build(planet.Params{SeedText: "deferred-form", Coverage: 30,
		Rain: 2, Temp: 2, Pop: 1, Factions: faction.DefaultSlots()})
	home := p.RandomLanding(0)
	if home < 0 {
		t.Fatal("no landing")
	}
	dest := -1
	for _, next := range p.Neighbours(home) {
		if len(p.Route(home, int(next))) >= 2 {
			dest = int(next)
			break
		}
	}
	if dest < 0 {
		t.Fatal("no adjacent world route")
	}
	h := &deferredFormHost{world: p, tile: home}
	var list []Caravan
	f := Of(&list, &Atlas{}, h)
	f.Form(fmt.Sprint(7), dest)
	if len(list) != 1 || len(list[0].Exiting) != 1 || !h.drafted || h.starts != 1 {
		t.Fatalf("initial deferred search lost caravan: list=%+v draft=%v starts=%d", list, h.drafted, h.starts)
	}
	f.CheckForming(10)
	if len(list[0].Exiting) != 1 || !h.drafted || h.starts != 2 {
		t.Fatal("repeatedly deferred search dropped a forming member")
	}
	f.CheckForming(11)
	if len(list[0].Exiting) != 1 || !h.job.Active() || h.starts != 3 {
		t.Fatal("member did not resume once a path search became available")
	}
}

// Exit selection stays on the exact retry path after blocked and
// budget-deferred attempts.
type fakeExitGround struct {
	nearX, nearY int
	edgeX, edgeY int
	blockedNear  bool
}

func (g fakeExitGround) WalkableNear(_, _, _ int) (int, int, bool) {
	return g.nearX, g.nearY, !g.blockedNear
}
func (g fakeExitGround) Edge() (int, int, bool) {
	return g.edgeX, g.edgeY, true
}

func TestExitFallsBackToReachableBorder(t *testing.T) {
	t.Parallel()
	g := fakeExitGround{nearX: 0, nearY: 5, edgeX: 29, edgeY: 10}
	var attempted [][2]int
	begin := func(_ entity.EntityID, j job.Record) bool {
		attempted = append(attempted, [2]int{j.TX, j.TY})
		return j.Kind == job.ExitMap && j.TX == 29 && j.TY == 10
	}
	if !TryExit(g, 7, 0, 5, func() bool { return false }, begin) {
		t.Fatal("reachable alternate border was refused")
	}
	if len(attempted) != 2 || attempted[0] != [2]int{0, 5} || attempted[1] != [2]int{29, 10} {
		t.Fatalf("exit route attempted out of order: %v", attempted)
	}
	attempted = nil
	if TryExit(g, 7, 0, 5, func() bool { return true }, begin) || len(attempted) != 1 {
		t.Fatalf("deferred search must remain pending: %v", attempted)
	}
}

// A previous pawn can exhaust the path budget before this pawn discovers
// that no nearby exit tile exists. No search was attempted for this pawn.
func TestExitMissingNearTileIgnoresPriorDeferredSearch(t *testing.T) {
	t.Parallel()
	g := fakeExitGround{blockedNear: true, edgeX: 29, edgeY: 10}
	calls := 0
	begin := func(_ entity.EntityID, j job.Record) bool {
		calls++
		return j.Kind == job.ExitMap && j.TX == 29 && j.TY == 10
	}
	if !TryExit(g, 7, 0, 5, func() bool { return true }, begin) || calls != 1 {
		t.Fatalf("unrelated deferred budget suppressed reachable fallback; calls=%d", calls)
	}
}
