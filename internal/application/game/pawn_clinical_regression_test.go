// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/stat"
	"testing"
)

func TestCaptiveRestCollapse(t *testing.T) {
	g := New(Config{Seed: 5, Width: 24, Height: 14})
	ada, _ := g.SpawnNamedPawn(8, 6, "Ada")
	rid := spawnRaiderAt(t, g, 12, 6, 0)
	g.makePrisoner(rid, ada)
	g.SetNeeds(rid, needs.Full, 0)
	g.phaseNeeds()
	v, _ := g.vital.Get(rid)
	if v.Status != health.Downed || v.Cause != health.CauseNoBed {
		t.Fatalf("setup did not collapse: %+v", v)
	}
	for range 600 {
		g.clock.Advance(1)
		g.phaseNeeds()
		g.phaseJobs()
		g.prison.Live(rid, uint64(g.clock.Now()))
	}
	n, _ := g.need.Get(rid)
	v, _ = g.vital.Get(rid)
	j, _ := g.jobs.Get(rid)
	if n.Rest == 0 {
		t.Fatalf("collapsed captive did not recover rest after 600 ticks: needs=%+v vital=%+v job=%+v", n, v, j)
	}
}
func TestCaptivityThoughtOnce(t *testing.T) {
	g := New(Config{Seed: 5, Width: 24, Height: 14})
	ada, _ := g.SpawnNamedPawn(8, 6, "Ada")
	rid := spawnRaiderAt(t, g, 12, 6, 0)
	g.makePrisoner(rid, ada)
	count, total := 0, 0
	for _, th := range g.prisonerThoughtList(rid) {
		if th.Kind == mood.Imprisoned {
			count++
			total += th.Value
		}
	}
	if count != 1 {
		t.Fatalf("imprisoned count=%d total=%d expected one thought=%d", count, total, mood.DefOf(mood.Imprisoned).Value)
	}
}
func TestPrisonerMoodConsumer(t *testing.T) {
	g := New(Config{Seed: 5, Width: 24, Height: 14})
	ada, _ := g.SpawnNamedPawn(8, 6, "Ada")
	rid := spawnRaiderAt(t, g, 12, 6, 0)
	g.makePrisoner(rid, ada)
	target := g.prisonerMood(rid)
	displayed := 100 - target
	if displayed == target {
		displayed = 0
	}
	g.socState().moodM[rid] = int32(displayed * 1000)
	pv := g.prisonerPawnView(rid)
	if pv.Mood != g.prisonerMood(rid) {
		t.Fatalf("view mood=%d target=%d resistance mood=%d", pv.Mood, pv.MoodTarget, g.prisonerMood(rid))
	}
}
func TestDossierEffectiveStats(t *testing.T) {
	g := New(Config{Seed: 4, Width: 14, Height: 10})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	for _, tc := range []struct {
		k mind.InspirationKind
		s stat.ID
	}{{mind.GoFrenzy, stat.MoveSpeed}, {mind.WorkFrenzy, stat.WorkSpeed}} {
		g.mind.Inspire.Start(id, tc.k)
		d, ok := g.Dossier(id)
		if !ok {
			t.Fatal("missing card")
		}
		if got, want := d.Stats[tc.s], g.statOf(id, tc.s); got != want {
			t.Errorf("%v dossier=%d gameplay=%d", tc.s, got, want)
		}
	}
}

func TestClinicalWorkControls(t *testing.T) {
	for _, open := range []bool{true, false} {
		g := New(Config{Seed: 1, Width: 20, Height: 10})
		pat, _ := g.SpawnNamedPawn(4, 4, "Pat")
		bed := buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 2)
		setInjury(g, pat, 45, open)
		g.care.SetSelfTend(pat, false)
		for k := work.Type(0); k < work.NumTypes; k++ {
			g.setPriority(pat, k, 0)
		}
		role := work.BedRest
		other := work.Patient
		if open {
			role, other = other, role
		}
		g.setPriority(pat, other, 1)
		if got := g.care.Rest.WorkType(pat); got != role {
			t.Fatalf("open=%v work=%v want=%v", open, got, role)
		}
		restFed(g, 20)
		if g.care.Rest.On(pat) {
			t.Fatalf("disabled %v admitted clinical rest", role)
		}
		g.setPriority(pat, role, 1)
		if !restFedUntil(g, 600, func() bool { return restingIn(g, pat, bed) }) {
			t.Fatalf("enabled %v did not admit patient", role)
		}
		g.setPriority(pat, role, 0)
		restFed(g, 20)
		if g.care.Rest.On(pat) {
			t.Fatalf("disabled active %v remained admitted", role)
		}
	}
}

func TestClinicalPriorityDefersToAvailableWork(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	pat, _ := g.SpawnNamedPawn(4, 4, "Pat")
	buildNow(t, g, construction.Bed, item.Wood, construction.South, 6, 2)
	setInjury(g, pat, 45, false)
	for k := work.Type(0); k < work.NumTypes; k++ {
		g.setPriority(pat, k, 0)
	}
	g.setPriority(pat, work.Construct, 1)
	g.setPriority(pat, work.BedRest, 4)
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 7, 4)
	fid := g.layerFrameAt(7, 4, construction.LayerObject)
	fr, ok := g.frames.Get(fid)
	if !ok {
		t.Fatal("construction fixture")
	}
	fr.Wood = fr.Need
	g.frames.Set(fid, fr)
	g.updateRestWork(pat)
	if g.care.Rest.On(pat) {
		t.Fatal("low-priority rest preempted work")
	}
	if !g.assignWorkByPriority(pat) {
		t.Fatal("ready construction not assigned")
	}
	j, _ := g.jobs.Get(pat)
	if j.Kind != job.Construct || j.Target != fid {
		t.Fatalf("priority selected %+v", j)
	}
}
