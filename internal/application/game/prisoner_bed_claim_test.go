// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
)

func TestPrisonBedSurvivesFloorSleep(t *testing.T) {
	g, ada, bo, rid, bed := prisonGame(t)
	capture(t, g, ada, bo, rid)
	g.sleptAt[rid] = 0
	if got, ok := g.prisonerBed(rid); !ok || got != bed {
		t.Fatalf("floor sleep discarded assigned prison bed: got %d, %v; want %d", got, ok, bed)
	}
	if v, ok := g.Prisoner(rid); !ok || !v.ForPrisonersBed {
		t.Fatalf("prisoner view lost permanent bed claim: %+v", v)
	}
	v, _ := g.vital.Get(rid)
	v.Status, v.Cause = health.Alive, health.CauseNone
	g.vital.Set(rid, v)
	g.SetNeeds(rid, needs.Full, needs.Full)
	g.jobs.Set(rid, job.Record{Kind: job.Sleep, State: job.Do, Target: bed, WorkLeft: 1})
	g.phaseJobs()
	j, _ := g.jobs.Get(rid)
	if j.State != job.Complete || g.sleptAt[rid] != bed {
		t.Fatalf("native sleep did not complete at owned prison bed: %+v, sleptAt %d", j, g.sleptAt[rid])
	}
	g.prison.Live(rid, uint64(g.clock.Now()))
	j, _ = g.jobs.Get(rid)
	if j.Kind == job.Sleep && j.Active() {
		t.Fatal("completed captive sleep survived lifecycle tick")
	}
}
