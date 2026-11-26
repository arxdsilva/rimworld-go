// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package commands_test

import (
	"bytes"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/sim/command"
)

func TestWorkColumnCommandsAndSave(t *testing.T) {
	g := game.New(game.Config{Seed: 37, Width: 24, Height: 16})
	id, err := g.SpawnNamedPawn(8, 8, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []command.Command{
		{Kind: command.SetWorkEnabled, Pawn: uint64(id), Work: command.WorkCraft, Enabled: false},
		{Kind: command.SetWorkPriority, Pawn: uint64(id), Work: command.WorkTailor, Count: 1},
		{Kind: command.SetWorkPriority, Pawn: uint64(id), Work: command.WorkSmith, Count: 4},
		{Kind: command.SetWorkPriority, Pawn: uint64(id), Work: command.WorkGrow, Count: 0},
	} {
		g.Apply(c)
	}
	g.ApplyPending()
	check := func(g *game.Game) {
		t.Helper()
		pawns := g.Snapshot()
		if len(pawns) != 1 {
			t.Fatalf("pawns %d", len(pawns))
		}
		p := pawns[0]
		if p.ID != id || p.WorkCraft || p.WorkGrow || p.Priorities[work.Tailor] != 1 ||
			p.Priorities[work.Smith] != 4 || p.Priorities[work.Grow] != 0 {
			t.Fatalf("work priority command changed unrelated columns: %+v", p.Priorities)
		}
	}
	check(g)
	var b bytes.Buffer
	if err := g.Save(&b, "labor"); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := game.Load(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	check(loaded)
}
