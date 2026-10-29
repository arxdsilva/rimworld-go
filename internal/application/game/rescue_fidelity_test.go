package game

import (
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// SIM-063 rescue fidelity, whole game: every way a carry ends early
// drops the patient on the carrier's tile with the carry index clean;
// every downed cause but a collapse is rescued; the safe bed keeps the
// patient's own claim and skips hazards; the warden takes a downed
// prisoner back to a prison bed.

// pickedUp orders doc to rescue bo (rescueScene) and steps until doc
// carries it a few tiles on its way.
func pickedUp(t *testing.T, g *Game, doc, bo entity.EntityID) {
	t.Helper()
	g.Enqueue(prioritize(command.OrderRescue, doc, bo, 12, 6, false))
	if !runCare(g, 300, func() bool { return g.care.Carry.Carrier(bo) == doc }, doc) {
		t.Fatal("fixture: Doc picks Bo up")
	}
	g.Step(3) // under way
	if g.care.Carry.Carrier(bo) != doc {
		t.Fatal("fixture: Doc still carries Bo")
	}
}

// droppedHere: bo lies on doc's tile, out of bed, and no one carries
// anyone.
func droppedHere(t *testing.T, g *Game, doc, bo entity.EntityID, how string) {
	t.Helper()
	if g.care.Carry.Len() != 0 || g.care.Carry.Carrier(bo) != 0 || g.care.Carry.Patient(doc) != 0 {
		t.Fatalf("%s: the carry index is clean (carrier %d, len %d)", how, g.care.Carry.Carrier(bo), g.care.Carry.Len())
	}
	if g.positionOf(bo) != g.positionOf(doc) || g.inBed(bo) {
		t.Fatalf("%s: dropped on the carrier's tile: bo %v doc %v", how, g.positionOf(bo), g.positionOf(doc))
	}
	if j, _ := g.jobs.Get(doc); j.Kind == job.Rescue && j.Active() {
		t.Fatalf("%s: the rescue ended: %+v", how, j)
	}
}

func TestRescueFidelityCarrierDowned(t *testing.T) {
	g, doc, bo, _, _ := rescueScene(t)
	pickedUp(t, g, doc, bo)
	g.hurt(doc, health.DownedInjury+10, 0)
	droppedHere(t, g, doc, bo, "carrier downed")
	g.Step(5)
	if g.care.Carry.Len() != 0 || g.positionOf(bo) != g.positionOf(doc) {
		t.Fatal("carrier downed: the patient stays where it fell")
	}
}

func TestRescueFidelityCarrierDrafted(t *testing.T) {
	g, doc, bo, _, _ := rescueScene(t)
	pickedUp(t, g, doc, bo)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(doc), Enabled: true})
	g.Step(1)
	droppedHere(t, g, doc, bo, "carrier drafted")
}

func TestRescueFidelityCarrierOrderedElsewhere(t *testing.T) {
	for _, drafted := range []bool{false, true} {
		g, doc, bo, _, _ := rescueScene(t)
		if drafted {
			g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(doc), Enabled: true})
			g.Step(1)
		}
		pickedUp(t, g, doc, bo) // a forced rescue survives the draft
		g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(doc), X: 2, Y: 10})
		g.Step(1)
		droppedHere(t, g, doc, bo, "ordered elsewhere")
		if j, _ := g.jobs.Get(doc); j.Kind != job.Move {
			t.Fatalf("drafted %v: the new order runs: %+v", drafted, j)
		}
	}
}

func TestRescueFidelityCarrierFlees(t *testing.T) {
	g, doc, bo, _, _ := rescueScene(t)
	pickedUp(t, g, doc, bo)
	p := g.positionOf(doc)
	wolf, _ := g.SpawnAnimal("ridgewolf", p.X+8, p.Y)
	holdAttacker(g, wolf, doc, p.X+8, p.Y) // flee_test.go: charging the unarmed carrier
	g.Step(1)
	if !fleeing(g, doc).Fleeing {
		t.Fatal("fixture: the unarmed carrier flees")
	}
	droppedHere(t, g, doc, bo, "carrier fled")
}

func TestRescueFidelityPatientDies(t *testing.T) {
	g, doc, bo, _, _ := rescueScene(t)
	pickedUp(t, g, doc, bo)
	g.die(bo, health.CauseBloodLoss)
	droppedHere(t, g, doc, bo, "patient died")
	at := g.positionOf(doc)
	g.Step(10)
	if g.positionOf(bo) != at || g.care.Carry.Len() != 0 {
		t.Fatal("the body stays where the carrier let go")
	}
}

// TestRescueFidelityStarvedToOwnBed: a colonist down from malnutrition is
// rescued (SIM-063) to its own bed, not the medical one it doesn't need.
func TestRescueFidelityStarvedToOwnBed(t *testing.T) {
	g := New(Config{Seed: 3, Width: 40, Height: 14})
	doc, _ := g.SpawnNamedPawn(6, 6, "Doc")
	bo, _ := g.SpawnNamedPawn(12, 6, "Bo")
	buildNow(t, g, construction.HospitalBed, item.Wood, construction.South, 14, 3)
	own := buildNow(t, g, construction.Bed, item.Wood, construction.South, 24, 4)
	b, _ := g.bldgs.Get(own)
	g.bldgs.Set(own, b.Claim(uint64(bo)))
	g.SetNeeds(bo, 0, needs.Full) // starving: it stays down until fed
	g.down(bo, health.CauseNoFood)
	if !runCare(g, 600, func() bool { return g.bedUnder(bo) == own }, doc) {
		t.Fatalf("starved: rescued to its own bed (under %d)", g.bedUnder(bo))
	}
}

// TestRescueFidelityKeepsOwnClaim: an injured colonist goes to the
// medical bed and keeps its own bed's claim.
func TestRescueFidelityKeepsOwnClaim(t *testing.T) {
	g, doc, bo, hosp, plain := rescueScene(t)
	b, _ := g.bldgs.Get(plain)
	g.bldgs.Set(plain, b.Claim(uint64(bo)))
	if !runCare(g, 600, func() bool { return g.bedUnder(bo) == hosp }, doc) {
		t.Fatal("injured: rescued to the hospital bed")
	}
	if b, _ := g.bldgs.Get(plain); b.SlotOf(uint64(bo)) < 0 {
		t.Fatal("its own bed stays its own")
	}
	if g.care.Rest.HeldBed(bo) != hosp {
		t.Fatalf("the medical bed is held for the stay: %d", g.care.Rest.HeldBed(bo))
	}
}

func TestRescueFidelityBedHazard(t *testing.T) {
	g, _, bo, hosp, plain := rescueScene(t)
	h := careReach{g}
	if h.BedHazard(hosp) || h.BedHazard(plain) {
		t.Fatal("fixture: both beds safe")
	}
	hb, _ := g.bldgs.Get(hosp)
	g.devStartFire(hb.X, hb.Y)
	if !h.BedHazard(hosp) || g.care.SafeBedFor(bo, 12, 6, care.RescueBed) != plain {
		t.Fatal("a burning bed is no safe bed")
	}
	pb, _ := g.bldgs.Get(plain)
	if g.residual == nil {
		g.residual = map[int]temperature.Temp{}
	}
	g.residual[pb.Y*g.grid.Width()+pb.X] = 1300 // 130 °C: past comfort by more than 90 °C
	if !h.BedHazard(plain) || g.care.SafeBedFor(bo, 12, 6, care.RescueBed) != 0 {
		t.Fatal("deadly heat: no safe bed")
	}
	g.residual[pb.Y*g.grid.Width()+pb.X] = -900 // -90 °C
	if !h.BedHazard(plain) {
		t.Fatal("deadly cold: no safe bed")
	}
	if !h.BedHazard(1 << 30) {
		t.Fatal("no such bed: never safe")
	}
}

func TestRescueFidelityWardenTakesPrisonerToBed(t *testing.T) {
	g, ada, bo, rid, bed := prisonGame(t)
	g.makePrisoner(rid, ada) // downed where it was taken, out of bed
	for _, id := range []entity.EntityID{ada, bo} {
		g.setPriority(id, work.Doctor, 0)
	}
	g.setPriority(bo, work.Warden, 1)
	carried := false
	ok := runCare(g, 600, func() bool {
		if c := g.care.Carry.Carrier(rid); c != 0 {
			carried = carried || c == bo
		}
		return g.bedUnder(rid) == bed
	}, ada, bo)
	if !ok || !carried || !g.prison.Is(rid) || g.care.Carry.Len() != 0 {
		j, _ := g.jobs.Get(bo)
		t.Fatalf("the warden carries the prisoner to its bed (ok %v carried %v, warden job %+v)", ok, carried, j)
	}
}
