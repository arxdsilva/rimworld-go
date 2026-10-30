package game

import (
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
