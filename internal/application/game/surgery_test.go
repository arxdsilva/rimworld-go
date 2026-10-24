package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// ENG-003 M1: queued operations, the surgeon's job and its outcome.

// legless is a colonist missing its left leg, with a bed, a skilled
// doctor and a peg leg in stock.
func legless(t *testing.T, medicine uint8) (*Game, entity.EntityID, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 4, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(10, 6, "Patient")
	doc, _ := g.SpawnNamedPawn(4, 6, "Doc")
	placeBuilt(t, g, construction.Bed, construction.South, 12, 2)
	g.SpawnStack(item.PegLeg, 1, 2, 9)
	(*g.sk.skills[doc])[skill.Medicine].Level = medicine
	v := g.VitalOf(patient)
	b := g.ensureBody(patient, &v)
	b.Missing |= 1<<body.LegL | 1<<body.FootL
	g.bodyChanged(patient, b, &v)
	return g, patient, doc
}

func addPegLeg(g *Game, patient entity.EntityID) {
	g.Enqueue(command.Command{Kind: command.AddOperation, Pawn: uint64(patient),
		Index: int(body.InstallImplant), Build: uint8(body.LegL), Stuff: uint8(item.PegLeg)})
	g.Step(1)
}

// operateUntil steps until patient's queue empties.
func operateUntil(t *testing.T, g *Game, patient, doc entity.EntityID) {
	t.Helper()
	for i := 0; i < 600 && len(g.operationsOf(patient)) > 0; i++ {
		g.SetNeeds(patient, needs.Full, needs.Full)
		g.SetNeeds(doc, needs.Full, needs.Full)
		g.Step(20)
	}
	if len(g.operationsOf(patient)) > 0 {
		j, _ := g.jobs.Get(doc)
		t.Fatalf("the operation never happened (doctor job %+v)", j)
	}
}

func TestOperationQueueValidatesAndRemoves(t *testing.T) {
	g, patient, _ := legless(t, 10)
	addPegLeg(g, patient)
	addPegLeg(g, patient) // a duplicate is ignored
	g.Enqueue(command.Command{Kind: command.AddOperation, Pawn: uint64(patient),
		Index: int(body.InstallImplant), Build: uint8(body.ArmL), Stuff: uint8(item.PegLeg)}) // does not fit
	g.Enqueue(command.Command{Kind: command.AddOperation, Pawn: uint64(patient),
		Index: int(body.Amputate), Build: uint8(body.Heart)}) // vital
	g.Enqueue(command.Command{Kind: command.AddOperation, Pawn: uint64(patient),
		Index: int(body.Amputate), Build: uint8(body.HandR)})
	g.Step(1)
	hv, _ := g.Health(patient)
	if len(hv.Operations) != 2 || hv.Operations[0].Label != "Install peg leg (left leg)" || hv.Operations[1].Label != "Amputate right hand" {
		t.Fatalf("operations = %+v", hv.Operations)
	}
	g.Enqueue(command.Command{Kind: command.RemoveOperation, Pawn: uint64(patient), Index: 1})
	g.Enqueue(command.Command{Kind: command.RemoveOperation, Pawn: uint64(patient), Index: 5}) // out of range
	g.Step(1)
	if ops := g.operationsOf(patient); len(ops) != 1 || ops[0].Kind != body.InstallImplant {
		t.Fatalf("after remove = %+v", ops)
	}
}

func TestSurgeryInstallsAPegLeg(t *testing.T) {
	g, patient, doc := legless(t, 20)
	before := g.bodies[patient].Capacity(body.Moving)
	addPegLeg(g, patient)
	operateUntil(t, g, patient, doc)
	b := g.bodies[patient]
	if b.Implants[body.LegL] != item.PegLeg {
		t.Fatalf("the peg leg is fitted: %+v", b)
	}
	if after := b.Capacity(body.Moving); after <= before {
		t.Fatalf("moving %d -> %d", before, after)
	}
	if g.stockOf(item.PegLeg) != 0 {
		t.Fatal("the peg leg is used up")
	}
	if !g.UnderAnaesthetic(patient) || g.VitalOf(patient).Status != health.Downed {
		t.Fatalf("the patient is under an anaesthetic: %+v", g.VitalOf(patient))
	}
	found := false
	for _, th := range g.thoughts(patient) {
		found = found || th.Kind == mood.Surgery
	}
	if !found {
		t.Fatal("the patient remembers the surgery")
	}
	for i := 0; i < 20 && g.UnderAnaesthetic(patient); i++ {
		g.Step(anaesthTicks / 10)
	}
	if g.UnderAnaesthetic(patient) || g.VitalOf(patient).Status != health.Alive {
		t.Fatalf("the patient wakes after the anaesthetic: %+v", g.VitalOf(patient))
	}
	if missingPart(b) {
		t.Fatal("a fitted leg is not a missing part")
	}
}

func TestBotchedSurgeryWoundsAndReports(t *testing.T) {
	g, patient, doc := legless(t, 0)
	v := g.VitalOf(doc)
	db := g.ensureBody(doc, &v)
	db.Missing |= 1<<body.EyeL | 1<<body.EyeR // a blind surgeon cannot succeed
	g.bodyChanged(doc, db, &v)
	if c := g.surgeryChanceOf(doc, patient, item.HerbalMedicine); c != 0 {
		t.Fatalf("blind surgeon chance %d", c)
	}
	addPegLeg(g, patient)
	before := g.VitalOf(patient).Injury
	operateUntil(t, g, patient, doc)
	b := g.bodies[patient]
	if b.Implants[body.LegL].IsImplant() {
		t.Fatal("a botched operation fits nothing")
	}
	surgical := false
	for _, w := range b.Wounds {
		surgical = surgical || w.Kind == body.Surgical
	}
	if !surgical && g.VitalOf(patient).Status != health.Dead {
		t.Fatalf("a botched operation leaves a surgical wound: %+v", b.Wounds)
	}
	if g.VitalOf(patient).Injury <= before {
		t.Fatal("a botched operation hurts")
	}
	botched := false
	for _, m := range g.messages {
		botched = botched || m.Text == "Doc botched an operation on Patient."
	}
	if !botched {
		t.Fatalf("the botch is reported (and onSurgeryBotched called): %+v", g.messages)
	}
}

func TestSurgeryWaitDoesNotFlap(t *testing.T) {
	g, patient, doc := legless(t, 10)
	addPegLeg(g, patient)
	if !g.awaitingSurgery(patient) {
		t.Fatal("a patient with a doable operation and a doctor waits for it")
	}
	g.surgeryGaveUp(patient)
	changes, prev := 0, g.awaitingSurgery(patient)
	for i := 0; i < 60; i++ {
		g.clock.Advance(surgeryRestRetry / 50)
		if a := g.awaitingSurgery(patient); a != prev {
			changes++
			prev = a
		}
	}
	if changes != 1 || !prev {
		t.Fatalf("waiting resumes once after the cooldown (changes %d, now %v)", changes, prev)
	}
	_ = doc
}

func TestCrafterMakesAPegLegFromABill(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Bo")
	placeBuilt(t, g, construction.Workbench, construction.South, 5, 2)
	g.SpawnStack(item.Wood, 20, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.MakePegLeg), Mode: uint8(recipe.DoCount), Count: 1})
	g.Step(1)
	for i := 0; i < 40 && g.stockOf(item.PegLeg) == 0; i++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	if g.stockOf(item.PegLeg) != 1 {
		t.Fatalf("peg legs = %d", g.stockOf(item.PegLeg))
	}
}
