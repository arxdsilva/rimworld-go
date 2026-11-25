// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/sim/statehash"
)

// A healed body may retain zero-length non-nil slices. Gob restores
// those slices as nil. Native StateHash must describe bytes that Save
// actually persists, not the runtime backing-array allocation pattern.
func TestNativeBodyHealedZeroLengthSlicesRoundTripHash(t *testing.T) {
	g := New(Config{Seed: 18022, Width: 20, Height: 20})
	p, err := g.SpawnNamedPawn(8, 8, "Healed")
	if err != nil {
		t.Fatal(err)
	}
	g.bodies = make(map[entity.EntityID]*body.Body)
	g.bodies[p] = &body.Body{Wounds: make([]body.Wound, 0, 3), Conds: make([]body.Condition, 0, 2),
		Fitted: make([]item.Kind, 0, 1)}
	b := g.exportBodies()
	if len(b) != 1 || g.bodies[p].Wounds == nil || g.bodies[p].Conds == nil {
		t.Fatalf("fixture lost its live allocated empty body: %+v", g.bodies[p])
	}
	if b[0].V.Wounds != nil || b[0].V.Conds != nil ||
		b[0].V.Fitted != nil {
		t.Fatalf("save did not canonicalize allocated empty slices: %+v", b[0].V)
	}
	beforeBody := statehash.OfFields(map[string]any{"Bodies": g.exportBodies()})
	var buf bytes.Buffer
	if err = g.Save(&buf, "body-empty"); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	afterBody := statehash.OfFields(map[string]any{"Bodies": loaded.exportBodies()})
	t.Logf("saved Bodies canonical hash before=%016x after=%016x", beforeBody, afterBody)
	if beforeBody != afterBody {
		t.Fatalf("saved empty body field changed native state hash %016x -> %016x", beforeBody, afterBody)
	}
	if g.bodies[p].Wounds == nil || g.bodies[p].Conds == nil || g.bodies[p].Fitted == nil {
		t.Fatal("saving mutated the live body's backing allocations")
	}
}

// The same healed-body allocation pattern must be invisible to a full
// continuously stepping Game after Save -> fresh native Game -> more ticks.
func TestNativeHealedBodySaveContinueEqualsContinuous(t *testing.T) {
	g := New(Config{Seed: 18023, Width: 32, Height: 32})
	if err := g.LoadStartKit(); err != nil {
		t.Fatal(err)
	}
	g.Step(60)
	if len(g.pawnIDs) == 0 {
		t.Fatal("no starting resident")
	}
	p := g.pawnIDs[0]
	if g.bodies == nil {
		g.bodies = make(map[entity.EntityID]*body.Body)
	}
	g.bodies[p] = &body.Body{Wounds: make([]body.Wound, 0, 1), Conds: make([]body.Condition, 0, 1)}
	before := g.StateHash()
	var out bytes.Buffer
	if err := g.Save(&out, "healed-body"); err != nil {
		t.Fatal(err)
	}
	restored, _, err := Load(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got := restored.StateHash(); got != before {
		t.Fatalf("native saved Game hash mismatch: %016x/%016x", before, got)
	}
	for tick := 0; tick < 100; tick++ {
		g.Step(1)
		restored.Step(1)
		a, b := g.StateHash(), restored.StateHash()
		if a != b {
			t.Fatalf("healed body continuous/reloaded mismatch at +%d: %016x/%016x", tick, a, b)
		}
	}
}
