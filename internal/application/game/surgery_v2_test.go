package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// MED-06: surgery v2 in the game: who can be operated on, medicine,
// organs, euthanasia, the anaesthetic, the vitals monitor and mood.

// surgeryColony is a patient and a Medicine 20 doctor beside a bed, with
// industrial medicine in stock (the best chance an outdoor operation
// gets: capped at 98 % by day).
func surgeryColony(t *testing.T, seed uint64) (*Game, entity.EntityID, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: seed, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(10, 6, "Patient")
	doc, _ := g.SpawnNamedPawn(4, 6, "Doc")
	placeBuilt(t, g, construction.Bed, construction.South, 12, 2)
	g.SpawnStack(item.IndustrialMedicine, 10, 3, 9)
	(*g.sk.skills[doc])[skill.Medicine].Level = 20
	return g, patient, doc
}

// operateAll steps until every listed patient's queue is empty (fed).
func operateAll(t *testing.T, g *Game, patient entity.EntityID, feed ...entity.EntityID) {
	t.Helper()
	for i := 0; i < 1500 && len(g.operationsOf(patient)) > 0; i++ {
		for _, id := range append(feed, patient) {
			g.SetNeeds(id, needs.Full, needs.Full)
		}
		g.Step(20)
	}
	if len(g.operationsOf(patient)) > 0 {
		t.Fatalf("the operation never happened: %+v", g.operationsOf(patient))
	}
}

// surgeryLine is patient's last surgery log line ("" when none).
func surgeryLine(g *Game, patient entity.EntityID) string {
	line := ""
	for _, e := range g.logs[patient] {
		if e.Kind == logSurgery {
			line = e.Name
		}
	}
	return line
}

func TestHarvestYieldsOrgan(t *testing.T) {
	g, patient, doc := surgeryColony(t, 4)
	queueOp(g, patient, body.HarvestOrgan, body.KidneyL, 0)
	operateAll(t, g, patient, doc)
	if l := surgeryLine(g, patient); !strings.Contains(l, "harvested the left kidney") {
		t.Fatalf("the harvest succeeds: %q", l)
	}
	b := g.bodies[patient]
	if b.Missing&(1<<body.KidneyL) == 0 || g.countItem(item.Kidney) != 1 {
		t.Fatalf("the kidney comes out whole: missing %b, kidneys %d", b.Missing, g.countItem(item.Kidney))
	}
	if v := g.VitalOf(patient); v.Status == health.Dead || b.ImmunityGain() != 750 {
		t.Fatalf("one kidney is enough to live: %+v", v)
	}
	if g.countItem(item.IndustrialMedicine) != 8 {
		t.Fatalf("a harvest uses 2 medicine: %d left", g.countItem(item.IndustrialMedicine))
	}
}

func TestHarvestLifeOrganKills(t *testing.T) {
	g, patient, doc := surgeryColony(t, 4)
	queueOp(g, patient, body.HarvestOrgan, body.Heart, 0)
	operateAll(t, g, patient, doc)
	v := g.VitalOf(patient)
	if v.Status != health.Dead || v.Cause != health.CauseOrganFailure || g.countItem(item.Heart) != 1 {
		t.Fatalf("taking the heart kills (and yields it): %+v, hearts %d", v, g.countItem(item.Heart))
	}
}

func TestInstallOrgan(t *testing.T) {
	g, patient, doc := surgeryColony(t, 4)
	v := g.VitalOf(patient)
	b := g.ensureBody(patient, &v)
	b.Missing |= 1 << body.KidneyR
	g.bodyChanged(patient, b, &v)
	queueOp(g, patient, body.InstallOrgan, body.KidneyR, item.Kidney)
	if hv, _ := g.Health(patient); len(hv.Operations) != 1 || hv.Operations[0].Status != "no kidney in stock" {
		t.Fatalf("waits for a kidney: %+v", hv.Operations)
	}
	g.SpawnStack(item.Kidney, 1, 2, 9)
	operateAll(t, g, patient, doc)
	if b := g.bodies[patient]; b.Missing&(1<<body.KidneyR) != 0 || b.ImmunityGain() != 1000 || g.countItem(item.Kidney) != 0 {
		t.Fatalf("the transplant restores the kidney: missing %b (%q)", b.Missing, surgeryLine(g, patient))
	}
}

func TestEuthanasiaCause(t *testing.T) {
	g, patient, doc := surgeryColony(t, 4)
	queueOp(g, patient, body.Euthanize, body.HandL, 0) // whole body: queued on the torso
	if ops := g.operationsOf(patient); len(ops) != 1 || ops[0].Part != body.Torso {
		t.Fatalf("queued %+v", ops)
	}
	if hv, _ := g.Health(patient); hv.Operations[0].ChancePct != 100 {
		t.Fatalf("euthanasia cannot fail: %+v", hv.Operations[0])
	}
	operateAll(t, g, patient, doc)
	if v := g.VitalOf(patient); v.Status != health.Dead || v.Cause != health.CauseEuthanasia || v.Cause.String() != "euthanized" {
		t.Fatalf("euthanized: %+v", v)
	}
	if g.countItem(item.IndustrialMedicine) != 9 {
		t.Fatalf("euthanasia uses 1 medicine: %d left", g.countItem(item.IndustrialMedicine))
	}
}

// TestAnestheticWearsOffInGame: an operation lays the patient down under
// the anaesthetic; woozy, it stands again; after 20 hours it is gone.
func TestAnestheticWearsOffInGame(t *testing.T) {
	g, patient, doc := surgeryColony(t, 4)
	queueOp(g, patient, body.Anesthetize, body.Torso, 0)
	operateAll(t, g, patient, doc)
	since := g.bodies[patient].Conds[0].Since
	if v := g.VitalOf(patient); v.Status != health.Downed || v.Cause != health.CauseAnesthetic || !g.UnderAnaesthetic(patient) {
		t.Fatalf("sedated: %+v", v)
	}
	if v := g.VitalOf(patient); v.Cause.String() != "under anesthetic" {
		t.Fatal("the cause reads plainly")
	}
	step := func(until uint64) {
		for uint64(g.clock.Now()) < until {
			g.SetNeeds(patient, needs.Full, needs.Full)
			g.SetNeeds(doc, needs.Full, needs.Full)
			g.Step(100)
		}
	}
	step(since + 5*clock.TicksPerHour)
	if g.VitalOf(patient).Status != health.Downed {
		t.Fatal("still sedated at 5 hours")
	}
	step(since + 7*clock.TicksPerHour + 1)
	if g.VitalOf(patient).Status != health.Alive || !g.UnderAnaesthetic(patient) || g.bodies[patient].Sedated() {
		t.Fatalf("woozy and up at 7 hours: %+v", g.VitalOf(patient))
	}
	step(since + 21*clock.TicksPerHour + 1)
	if g.UnderAnaesthetic(patient) {
		t.Fatal("gone after 20 hours")
	}
	hv, _ := g.Health(patient)
	if len(hv.Options) == 0 {
		t.Fatal("options still offered")
	}
}

func TestSurgeryConsumesMedicine(t *testing.T) {
	g, patient, doc := legless(t, 20) // 6 herbal medicine in stock
	addPegLeg(g, patient)
	operateUntil(t, g, patient, doc)
	if n := g.countItem(item.HerbalMedicine); n != 4 {
		t.Fatalf("fitting an implant uses 2 medicine: %d left", n)
	}
	// "No medicine" care: the operation waits for a care level that allows some
	g.Enqueue(command.Command{Kind: command.SetMedicalCare, Pawn: uint64(patient), Mode: uint8(medcare.NoMeds)})
	queueOp(g, patient, body.Amputate, body.HandR, 0)
	if why, ok := g.surgeryWait(patient, g.operationsOf(patient)[0]); ok || why != "the care level allows no medicine" {
		t.Fatalf("no medicine allowed: %q %v", why, ok)
	}
	// herbal only, and only industrial in stock
	g.Enqueue(command.Command{Kind: command.SetMedicalCare, Pawn: uint64(patient), Mode: uint8(medcare.HerbalMax)})
	g.Step(1)
	for _, iid := range append([]entity.EntityID(nil), g.itemIDs...) {
		if st, ok := g.stacks.Get(iid); ok && st.Kind == item.HerbalMedicine {
			g.deleteItem(iid)
		}
	}
	g.SpawnStack(item.IndustrialMedicine, 5, 3, 9)
	if why, _ := g.surgeryWait(patient, g.operationsOf(patient)[0]); why != "needs 2 herbal medicine or better" {
		t.Fatalf("care caps the medicine: %q", why)
	}
	g.SpawnStack(item.HerbalMedicine, 1, 4, 9)
	if why, _ := g.surgeryWait(patient, g.operationsOf(patient)[0]); why != "needs 2 herbal medicine or better" {
		t.Fatalf("one dose is not enough: %q", why)
	}
}

func TestPrisonerGetsOperated(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	capture(t, g, ada, bo, rid)
	(*g.sk.skills[ada])[skill.Medicine].Level = 20
	(*g.sk.skills[bo])[skill.Medicine].Level = 20
	g.SpawnStack(item.HerbalMedicine, 10, 6, 6)
	queueOp(g, rid, body.Anesthetize, body.Torso, 0)
	if len(g.operationsOf(rid)) != 1 {
		t.Fatal("a prisoner's operation queues")
	}
	operateAll(t, g, rid, ada, bo)
	if !g.UnderAnaesthetic(rid) || !strings.Contains(surgeryLine(g, rid), "anesthetized") {
		t.Fatalf("the prisoner was operated on: %q", surgeryLine(g, rid))
	}
}

// spawnGuest puts a downed guest (a visitor of no faction) on (x, y).
func spawnGuest(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	id := g.entities.Next()
	g.peopleReg().add(&person{ID: id, Bio: pawngen.Pawn{First: "Gus"}, HasBio: true, Where: whereVisitor})
	g.placeVisitor(id, 0, x, y, uint64(g.clock.Now()))
	if !g.visitor(id) {
		t.Fatal("not a guest")
	}
	return id
}

func TestGuestGetsOperated(t *testing.T) {
	g, _, doc := surgeryColony(t, 4)
	guest := spawnGuest(t, g, 14, 8)
	g.hurt(guest, health.DownedInjury+10, 0)
	g.Step(1)
	if v := g.VitalOf(guest); v.Status != health.Downed {
		t.Fatalf("the guest is down: %+v", v)
	}
	queueOp(g, guest, body.Anesthetize, body.Torso, 0)
	if why, _ := g.surgeryWait(guest, g.operationsOf(guest)[0]); why != "needs 1 herbal medicine or better" {
		t.Fatalf("a guest's care level is herbal at most: %q", why)
	}
	g.SpawnStack(item.HerbalMedicine, 3, 5, 9)
	if len(g.operationsOf(guest)) != 1 {
		t.Fatal("a guest's operation queues")
	}
	operateAll(t, g, guest, doc)
	if !g.UnderAnaesthetic(guest) {
		t.Fatalf("the guest was operated on: %q", surgeryLine(g, guest))
	}
}

func TestHarvestMood(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	capture(t, g, ada, bo, rid)
	g.ng.factions = []faction.Faction{{Type: faction.Settlers, Name: "Valley", Goodwill: 0}}
	g.personOf(rid).Faction = 1
	value := func(id entity.EntityID, k mood.Kind) (sum, n int) {
		for _, th := range g.thoughts(id) {
			if th.Kind == k {
				sum, n = sum+th.Value, n+1
			}
		}
		return sum, n
	}
	g.organHarvested(ada, rid, body.KidneyL)
	if v, _ := value(rid, mood.OrganHarvested); v != -30 {
		t.Fatalf("the donor: %d", v)
	}
	if v, _ := value(ada, mood.KnowOrganHarvested); v != -5 {
		t.Fatalf("the colony: %d", v)
	}
	if f := g.factionOf(1); f.Goodwill != -80 {
		t.Fatalf("a kidney costs 80 goodwill: %d", f.Goodwill)
	}
	g.organHarvested(ada, rid, body.KidneyR)
	if v, n := value(bo, mood.KnowOrganHarvested); n != 2 || v != -5-3 {
		t.Fatalf("stacking at 75 %%: %d over %d", v, n)
	}
	for i := 0; i < 6; i++ {
		g.organHarvested(ada, rid, body.LungL)
	}
	if _, n := value(bo, mood.KnowOrganHarvested); n != 5 {
		t.Fatalf("at most 5 stack: %d", n)
	}
	g.ng.factions[0].Goodwill = 0
	g.organHarvested(ada, rid, body.Heart)
	if f := g.factionOf(1); f.Goodwill != -100 {
		t.Fatalf("a heart costs at most 100 goodwill: %d", f.Goodwill)
	}
	g.organHarvested(bo, ada, body.KidneyL)
	if v, _ := value(bo, mood.KnowColonistOrganHarvested); v != -6 {
		t.Fatalf("a colonist's organ weighs more: %d", v)
	}
}

func TestMonitorNeedsPowerAndAdjacency(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(5, 2, "Patient")
	bed := buildNow(t, g, construction.HospitalBed, item.Steel, construction.South, 5, 2)
	b, _ := g.bldgs.Get(bed)
	g.bldgs.Set(bed, b.Claim(uint64(patient)))
	sx, sy := g.sleepCellFor(patient, b)
	g.positions.Set(patient, Position{sx, sy})
	if g.bedUnder(patient) != bed {
		t.Fatal("the patient lies in the hospital bed")
	}
	before := g.immunityGain(patient)
	far := buildNow(t, g, construction.VitalsMonitor, item.Steel, construction.South, 12, 2)
	onPower(t, g, 12, 2, 14, 2)
	if !g.powered(far) || g.monitorBy(patient) {
		t.Fatal("a monitor two tiles off does nothing")
	}
	mon := buildNow(t, g, construction.VitalsMonitor, item.Steel, construction.South, 6, 3)
	if g.bedMonitor(bed) != mon || g.monitorBy(patient) {
		t.Fatal("an unpowered monitor beside the bed does nothing")
	}
	for x := 6; x <= 12; x++ {
		buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, x, 3)
	}
	g.balancePower(uint64(g.clock.Now()))
	if !g.powered(mon) || !g.monitorBy(patient) {
		t.Fatalf("powered and beside the bed: powered %v", g.powered(mon))
	}
	if g.monitorTend(patient) != 70 || g.immunityGain(patient) != before+20 {
		t.Fatalf("tend +70, immunity +20: %d, %d -> %d", g.monitorTend(patient), before, g.immunityGain(patient))
	}
	in := g.surgeryInput(0, patient, body.Amputate, item.HerbalMedicine)
	if !in.Monitor || !in.HospitalBed {
		t.Fatalf("surgery sees the monitor: %+v", in)
	}
	// a plain bed gets no monitor
	plain := buildNow(t, g, construction.Bed, item.Wood, construction.South, 2, 6)
	buildNow(t, g, construction.VitalsMonitor, item.Steel, construction.South, 3, 6)
	if g.bedMonitor(plain) != 0 {
		t.Fatal("only hospital beds take a monitor")
	}
	// deconstructed: the cache moves with the buildings
	g.removeBuilding(mon)
	if g.bedMonitor(bed) != 0 || g.monitorBy(patient) {
		t.Fatal("the monitor is gone")
	}
}

func TestSurgeryCleanlinessFromRoom(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	if c, out := g.surgeryCleanliness(15, 9); !out || c != 0 {
		t.Fatalf("outdoors: %d %v", c, out)
	}
	hut(t, g, 1, 1, 7, 7) // a bare-ground room
	if c, out := g.surgeryCleanliness(4, 4); out || c != -100 {
		t.Fatalf("a room on bare ground: %d %v", c, out)
	}
	for x := 2; x <= 6; x++ {
		for y := 2; y <= 6; y++ {
			buildNow(t, g, construction.Floor, item.Wood, construction.South, x, y)
		}
	}
	if c, _ := g.surgeryCleanliness(4, 4); c != 0 {
		t.Fatalf("a clean floored room: %d", c)
	}
	r := g.roomAt(4, 4)
	g.filth.roomSum[r] = 25 * 100 // 100 permille of filth a cell
	if c, _ := g.surgeryCleanliness(4, 4); c != -200 {
		t.Fatalf("a dirty room: %d", c)
	}
	g.filth.roomSum[r] = 25 * 1000
	if c, _ := g.surgeryCleanliness(4, 4); c != -500 {
		t.Fatalf("filthy bottoms out: %d", c)
	}
}

// TestOldSurgerySaveLoads: a version 6 save with queued operations and a
// patient still under the old hour-long anaesthetic loads, wakes the
// patient within the hour, and the queue still runs.
func TestOldSurgerySaveLoads(t *testing.T) {
	g, patient, doc := legless(t, 20)
	addPegLeg(g, patient)
	queueOp(g, patient, body.Amputate, body.HandR, 0)
	now := uint64(g.clock.Now())
	v := g.VitalOf(patient)
	v.Status, v.Cause, v.DownedTick = health.Downed, health.CauseSurgery, now
	g.vital.Set(patient, v)
	g.surg.under[patient] = now + anaesthTicks
	h := loadAsVersion(t, g, 6)
	if ops := h.operationsOf(patient); len(ops) != 2 || ops[0].Kind != body.InstallImplant || ops[1].Kind != body.Amputate {
		t.Fatalf("the queue survives: %+v", ops)
	}
	if !h.UnderAnaesthetic(patient) {
		t.Fatal("the old anaesthetic survives")
	}
	for i := 0; i < 12 && h.UnderAnaesthetic(patient); i++ {
		h.SetNeeds(patient, needs.Full, needs.Full)
		h.SetNeeds(doc, needs.Full, needs.Full)
		h.Step(anaesthTicks / 10)
	}
	if h.surg.under[patient] != 0 || h.VitalOf(patient).Cause == health.CauseSurgery {
		t.Fatalf("woken within the hour: %+v", h.VitalOf(patient))
	}
	h.cmdRemoveOperation(patient, 1)
	operateUntil(t, h, patient, doc)
	if h.bodies[patient].Implant(body.LegL) != item.PegLeg {
		t.Fatal("the old queue still runs")
	}
}

// TestSurgeryChanceView: a queued operation shows its chance with the
// factor breakdown.
func TestSurgeryChanceView(t *testing.T) {
	g, patient, _ := legless(t, 8)
	queueOp(g, patient, body.Amputate, body.HandR, 0)
	hv, _ := g.Health(patient)
	op := hv.Operations[0]
	in := g.surgeryInput(g.bestSurgeon(patient), patient, body.Amputate, item.HerbalMedicine)
	c, _ := body.SurgeryChance(in)
	if op.ChancePct != int(c/10) || op.ChancePct == 0 {
		t.Fatalf("chance %d, want %d", op.ChancePct, c/10)
	}
	for _, want := range []string{"Success chance", "Medicine skill 8: 80%", "Manipulation", "Light", "herbal medicine", "Operation: ×120%"} {
		if !strings.Contains(op.ChanceTip, want) {
			t.Fatalf("tooltip lacks %q:\n%s", want, op.ChanceTip)
		}
	}
	hv, _ = g.Health(patient)
	for _, label := range []string{"Harvest left kidney", "Harvest heart", "Euthanize", "Anesthetize"} {
		found := false
		for _, o := range hv.Options {
			found = found || o.Label == label
		}
		if !found {
			t.Fatalf("picker lacks %q", label)
		}
	}
}
