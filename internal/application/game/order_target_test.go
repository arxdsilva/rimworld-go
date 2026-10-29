package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// TestEmergencyTendQueues (SIM-062): a Shift-queued "Tend X" waits behind
// the current order instead of replacing the queue, is named after its
// patient, and runs when the first tend is done.
func TestEmergencyTendQueues(t *testing.T) {
	g := New(Config{Seed: 3, Width: 48, Height: 12})
	doc, _ := g.SpawnNamedPawn(6, 6, "Doc")
	bo, _ := g.SpawnNamedPawn(10, 6, "Bo")
	al, _ := g.SpawnNamedPawn(14, 6, "Al")
	g.setPriority(doc, work.Doctor, 0) // it would never tend on its own
	g.hurt(bo, health.DownedInjury+10, 0)
	g.hurt(al, health.DownedInjury+10, 0)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(doc), Enabled: true})
	g.Step(1)
	g.Enqueue(command.Command{Kind: command.EmergencyTend, Pawn: uint64(doc), Target: uint64(bo), X: 10, Y: 6})
	g.Step(1)
	g.Enqueue(command.Command{Kind: command.EmergencyTend, Pawn: uint64(doc), Target: uint64(al), X: 14, Y: 6, Queue: true})
	g.Step(1)
	if j, _ := g.jobs.Get(doc); j.Kind != job.EmergencyTend || j.Target != bo {
		t.Fatalf("the first tend keeps going: %+v", j)
	}
	q := g.orderQueue[doc]
	if len(q) != 1 || q[0].label != "Tend Al" {
		t.Fatalf("the second tend is queued and named after its patient: %+v", q)
	}
	if !runCare(g, 600, func() bool { return g.tended(bo) && g.tended(al) }, doc) {
		j, _ := g.jobs.Get(doc)
		t.Fatalf("both tended in turn (doctor job %+v, queue %+v)", j, g.orderQueue[doc])
	}
}

// TestOrderLabelMatchesTarget (SIM-062): two pawn-targeted options on one
// tile differ only by their target; the queue label must follow the
// target, not the first option of the same kind.
func TestOrderLabelMatchesTarget(t *testing.T) {
	g := New(Config{Seed: 3, Width: 48, Height: 12})
	doc, _ := g.SpawnNamedPawn(6, 6, "Doc")
	bo, _ := g.SpawnNamedPawn(10, 6, "Bo")
	al, _ := g.SpawnNamedPawn(10, 6, "Al") // the same tile
	g.hurt(bo, health.DownedInjury+10, 0)
	g.hurt(al, health.DownedInjury+10, 0)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(doc), Enabled: true})
	g.Step(1)
	for _, want := range []struct {
		id   entity.EntityID
		name string
	}{{bo, "Tend Bo"}, {al, "Tend Al"}} {
		c := command.Command{Kind: command.EmergencyTend, Pawn: uint64(doc), Target: uint64(want.id), X: 10, Y: 6}
		if got := g.orderLabel(doc, c); got != want.name {
			t.Fatalf("label for target %d: %q, want %q", want.id, got, want.name)
		}
	}
}
