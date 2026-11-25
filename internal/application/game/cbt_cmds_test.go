// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/sim/command"
)

// TestThreatResponseAndSmokepopCommands: SetThreatResponse sets a
// colonist's undrafted threat response (shown on the view), out-of-range
// values are ignored, and PopSmokepop without a pack does nothing.
func TestThreatResponseAndSmokepopCommands(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 20, Height: 20})
	id, err := g.SpawnPawn(5, 5)
	if err != nil {
		t.Fatal(err)
	}
	view := func() PawnView {
		for _, p := range g.Snapshot() {
			if p.ID == id {
				return p
			}
		}
		t.Fatal("no colonist")
		return PawnView{}
	}
	if r := view().ThreatResponse; r != "default" {
		t.Fatalf("fresh threat response %q, want default", r)
	}
	g.Enqueue(command.Command{Kind: command.SetThreatResponse, Pawn: uint64(id), X: 1})
	g.ApplyPending()
	if r := view().ThreatResponse; r != "flee" {
		t.Fatalf("after SetThreatResponse 1: %q, want flee", r)
	}
	g.Enqueue(command.Command{Kind: command.SetThreatResponse, Pawn: uint64(id), X: 9})
	g.ApplyPending()
	if r := view().ThreatResponse; r != "flee" {
		t.Fatalf("an out-of-range response changed it to %q", r)
	}
	before := view().Smokepops
	g.Enqueue(command.Command{Kind: command.PopSmokepop, Pawn: uint64(id)})
	g.ApplyPending()
	if after := view().Smokepops; after != before {
		t.Fatalf("PopSmokepop without a pack changed uses %d -> %d", before, after)
	}
}

func TestPlayerOrdersRejectRaiderAndPreserveNativeJob(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	id, _ := g.SpawnNamedPawn(15, 10, "Ada")
	g.Raid(1)
	rid := g.raiderIDs[0]
	before, _ := g.jobs.Get(rid)
	for _, k := range []command.Kind{command.MovePawn, command.DraftPawn, command.Prioritize} {
		g.Enqueue(command.Command{Kind: k, Pawn: uint64(rid), X: 2, Y: 2, Enabled: true, Queue: true})
		g.Enqueue(command.Command{Kind: k, Pawn: uint64(rid), X: 2, Y: 2, Enabled: true})
	}
	g.ApplyPending()
	after, _ := g.jobs.Get(rid)
	if !reflect.DeepEqual(before, after) || g.draft[rid] || len(g.orderQueue[rid]) != 0 {
		t.Fatal("player orders mutated raider")
	}
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(id), Enabled: true})
	g.ApplyPending()
	if !g.draft[id] {
		t.Fatal("own resident draft not admitted")
	}
}
