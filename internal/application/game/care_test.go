package game

import (
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// MED-03: care levels, the doctor's work order, emergency tends.

// careScene is a doctor at (4, 6) and a downed patient at (14, 6) with no
// bed (tended where it lies), herbal and industrial medicine near the
// doctor, everyone on call around the clock.
func careScene(t *testing.T) (g *Game, doc, patient entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 2, Width: 20, Height: 12})
	patient, _ = g.SpawnNamedPawn(14, 6, "Patient")
	doc, _ = g.SpawnNamedPawn(4, 6, "Doc")
	scheduleAll(g, schedule.Work)
	stack(t, g, item.HerbalMedicine, 5, 6, 9)
	stack(t, g, item.IndustrialMedicine, 5, 7, 9)
	g.hurt(patient, health.DownedInjury+10, 0)
	if v := g.VitalOf(patient); v.Status != health.Downed {
		t.Fatalf("fixture: the patient is downed: %+v", v)
	}
	return g, doc, patient
}

func setCare(g *Game, id entity.EntityID, p medcare.Policy) {
	g.Enqueue(command.Command{Kind: command.SetMedicalCare, Pawn: uint64(id), Mode: uint8(p)})
	g.Step(1)
}

// runCare steps until cond holds (or n steps of 10 ticks), keeping the
// colonists fed and rested.
func runCare(g *Game, n int, cond func() bool, ids ...entity.EntityID) bool {
	for i := 0; i < n; i++ {
		if cond() {
			return true
		}
		g.Step(10)
		for _, id := range ids {
			g.SetNeeds(id, needs.Full, needs.Full)
		}
	}
	return cond()
}

func TestCarePolicyCapsMedicine(t *testing.T) {
	g, doc, patient := careScene(t) // injury ≥ severeInjury: industrial first
	want := map[medcare.Policy]item.Kind{
		medcare.Best: item.IndustrialMedicine, medcare.IndustrialMax: item.IndustrialMedicine,
		medcare.HerbalMax: item.HerbalMedicine, medcare.NoMeds: 0, medcare.NoCare: 0,
	}
	for p := medcare.Policy(0); p < medcare.NumPolicies; p++ {
		setCare(g, patient, p)
		rec := g.care.Doctor.WithMedicine(doc, job.Record{Kind: job.Tend, Target: patient, TX: 13, TY: 6})
		got := item.Kind(0)
		if rec.Dest != 0 {
			st, _ := g.stacks.Get(rec.Dest)
			got = st.Kind
		}
		if got != want[p] || rec.Dest == 0 && (rec.TX != 13 || rec.TY != 6) {
			t.Errorf("%s: fetched %v (dest %d), want %v", p.Label(), got, rec.Dest, want[p])
		}
	}
	// in play: herbal at most with only industrial in stock tends bare
	g, doc, patient = careScene(t)
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.HerbalMedicine {
			g.deleteItem(id)
		}
	}
	setCare(g, patient, medcare.HerbalMax)
	if !runCare(g, 400, func() bool { return g.tended(patient) }, doc) {
		t.Fatal("the patient is tended")
	}
	if n := g.countItem(item.IndustrialMedicine); n != 5 {
		t.Fatalf("herbal or less never takes industrial: %d left", n)
	}
}

func TestNoCareSkipsTend(t *testing.T) {
	g, doc, patient := careScene(t)
	placeBuilt(t, g, construction.SleepingSpot, construction.South, 2, 2)
	setCare(g, patient, medcare.NoCare)
	rescued := runCare(g, 300, func() bool { return g.inBed(patient) }, doc)
	if !rescued {
		t.Fatal("no care still rescues a downed colonist to bed")
	}
	for i := 0; i < 150; i++ {
		g.Step(10)
		g.SetNeeds(doc, needs.Full, needs.Full)
		if j, _ := g.jobs.Get(doc); j.Kind == job.Tend && j.Target == patient {
			t.Fatal("a doctor tends a patient set to no care")
		}
	}
	if g.tended(patient) || !g.needsTending(patient) {
		t.Fatal("no care: the patient stays untended")
	}
	setCare(g, patient, medcare.Best)
	if !runCare(g, 400, func() bool { return g.tended(patient) }, doc) {
		t.Fatal("care restored: the doctor tends")
	}
}

func TestNoMedsTendsWithoutMedicine(t *testing.T) {
	g, doc, patient := careScene(t)
	setCare(g, patient, medcare.NoMeds)
	if !runCare(g, 400, func() bool { return g.tended(patient) }, doc) {
		t.Fatal("care, no medicine: still tended")
	}
	if h, i := g.countItem(item.HerbalMedicine), g.countItem(item.IndustrialMedicine); h != 5 || i != 5 {
		t.Fatalf("no medicine used: herbal %d, industrial %d left", h, i)
	}
}

// laidDown lays id in a sleep job where it stands (a patient a doctor can
// work on), without stepping the game.
func laidDown(g *Game, id entity.EntityID) {
	p, _ := g.positions.Get(id)
	g.jobs.Set(id, job.Record{Kind: job.Sleep, State: job.Do, TX: p.X, TY: p.Y, WorkLeft: 1 << 20})
}

func TestDoctorPriorityOrder(t *testing.T) {
	g, patient, doc := legless(t, 10) // a bed at (12, 2), a peg leg in stock
	addPegLeg(g, patient)
	downed, _ := g.SpawnNamedPawn(4, 9, "Downed")
	g.hurt(downed, health.DownedInjury+10, 0)
	placeBuilt(t, g, construction.SleepingSpot, construction.South, 16, 9)
	if g.care.SafeBedFor(downed, 4, 9, care.RescueBed) == 0 {
		t.Fatal("fixture: a free bed for the downed colonist")
	}
	bleeder, _ := g.SpawnNamedPawn(8, 9, "Bleeder")
	addWound(t, g, bleeder, body.ArmL, 6, body.Cut)
	laidDown(g, bleeder)
	bruised, _ := g.SpawnNamedPawn(2, 6, "Bruised")
	addWound(t, g, bruised, body.ArmL, 6, body.Bruise)
	laidDown(g, bruised)
	laidDown(g, patient) // lying: ready for surgery
	dog, _ := g.SpawnAnimal("labrador", 6, 3)
	g.TameAnimal(dog, "Rex")
	setAnimal(g, dog, func(a *animal.State) { a.Injury = 20 })
	held := entity.EntityID(1 << 30) // a stand-in holding each finished patient
	steps := []struct {
		kind   job.Kind
		target entity.EntityID
	}{
		{job.Rescue, downed}, {job.Tend, bleeder}, {job.Tend, bruised}, {job.Surgery, patient}, {job.TendAnimal, dog},
	}
	for i, s := range steps {
		if !g.assignWorkType(doc, work.Doctor) {
			t.Fatalf("step %d: no doctor job (want %v)", i, s.kind)
		}
		j, _ := g.jobs.Get(doc)
		if j.Kind != s.kind || j.Target != s.target {
			t.Fatalf("step %d: %v on %d, want %v on %d", i, j.Kind, j.Target, s.kind, s.target)
		}
		g.interrupt(doc, job.Cancelled)
		g.reserve.TryEntity(held, s.target)
	}
}

// TestDoctorDoesNotFlapBetweenPatients (no-flap): a doctor tending one
// patient sticks with it even when another starts bleeding faster
// halfway; the patient stays reserved until the tend ends.
func TestDoctorDoesNotFlapBetweenPatients(t *testing.T) {
	g := New(Config{Seed: 2, Width: 24, Height: 12})
	a, _ := g.SpawnNamedPawn(12, 4, "A")
	b, _ := g.SpawnNamedPawn(12, 9, "B")
	doc, _ := g.SpawnNamedPawn(2, 6, "Doc")
	scheduleAll(g, schedule.Work)
	addWound(t, g, a, body.ArmL, 6, body.Cut)
	addWound(t, g, b, body.ArmL, 6, body.Bruise)
	for _, id := range []entity.EntityID{a, b} {
		g.hurt(id, health.DownedInjury+10, 0) // down where they stand
	}
	g.bodies[b].TendAll(500, uint64(g.clock.Now())) // B: nothing to tend yet
	var targets []entity.EntityID
	track := func() {
		if j, _ := g.jobs.Get(doc); j.Kind == job.Tend && j.Active() &&
			(len(targets) == 0 || targets[len(targets)-1] != j.Target) {
			targets = append(targets, j.Target)
		}
	}
	if !runCare(g, 200, func() bool { track(); return len(targets) > 0 }, doc) || targets[0] != a {
		t.Fatalf("the doctor goes to A first: %v", targets)
	}
	for _, p := range []body.Part{body.Torso, body.LegL, body.LegR, body.ArmR} { // B now bleeds faster than A
		if g.bodies[b].BleedPerDay() <= g.bodies[a].BleedPerDay() {
			addWound(t, g, b, p, 15, body.Cut)
		}
	}
	if g.bodies[b].BleedPerDay() <= g.bodies[a].BleedPerDay() {
		t.Fatalf("fixture: B bleeds faster: %d vs %d", g.bodies[b].BleedPerDay(), g.bodies[a].BleedPerDay())
	}
	if owner, _ := g.reserve.EntityOwner(a); owner != doc {
		t.Fatal("A is held by its doctor")
	}
	runCare(g, 400, func() bool { track(); return g.tended(a) && g.tended(b) }, doc)
	if len(targets) != 2 || targets[0] != a || targets[1] != b {
		t.Fatalf("one tend each, A then B, no switching back and forth: %v", targets)
	}
}

func TestEmergencyTendWhileDrafted(t *testing.T) {
	g := New(Config{Seed: 3, Width: 48, Height: 12})
	doc, _ := g.SpawnNamedPawn(6, 6, "Doc")
	patient, _ := g.SpawnNamedPawn(12, 6, "Bo")
	g.setPriority(doc, work.Doctor, 0) // it would never tend on its own
	stack(t, g, item.HerbalMedicine, 3, 8, 8)
	stack(t, g, item.IndustrialMedicine, 3, 40, 8) // out of reach for an emergency
	g.hurt(patient, health.DownedInjury+10, 0)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(doc), Enabled: true})
	g.Step(1)
	opts := g.OrdersAt(doc, 12, 6)
	var tend *OrderOption
	for i := range opts {
		if opts[i].Label == "Tend Bo" {
			tend = &opts[i]
		}
	}
	if tend == nil || tend.Disabled || tend.Cmd.Kind != command.EmergencyTend || tend.Cmd.Target != uint64(patient) {
		t.Fatalf("drafted orders offer Tend Bo: %+v", opts)
	}
	g.Enqueue(tend.Cmd)
	g.Step(1)
	if j, _ := g.jobs.Get(doc); j.Kind != job.EmergencyTend {
		t.Fatalf("the drafted doctor goes to tend: %+v", j)
	}
	if !runCare(g, 300, func() bool { return g.tended(patient) }, doc) {
		j, _ := g.jobs.Get(doc)
		t.Fatalf("emergency tend done (doctor job %+v)", j)
	}
	if !g.drafted(doc) || g.countItem(item.HerbalMedicine) != 2 || g.countItem(item.IndustrialMedicine) != 3 {
		t.Fatalf("still drafted %v, used the herbal near by: herbal %d, industrial %d",
			g.drafted(doc), g.countItem(item.HerbalMedicine), g.countItem(item.IndustrialMedicine))
	}
	if reason := g.care.Doctor.CanEmergencyTend(doc, patient); reason != care.ReasonNothingToTend {
		t.Fatalf("tended: %q", reason)
	}
	// undrafting ends it
	g.hurt(patient, 5, 0)
	g.Enqueue(command.Command{Kind: command.EmergencyTend, Pawn: uint64(doc), Target: uint64(patient)})
	g.Step(1)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(doc), Enabled: false})
	g.Step(1)
	if j, _ := g.jobs.Get(doc); j.Kind == job.EmergencyTend {
		t.Fatal("undrafted: the emergency tend ends")
	}
	if opts := g.care.PawnOrders(doc, 12, 6); opts != nil {
		t.Fatalf("no Tend order undrafted: %+v", opts)
	}
	// not drafted: the command does nothing
	g.Enqueue(command.Command{Kind: command.EmergencyTend, Pawn: uint64(doc), Target: uint64(patient)})
	g.Step(1)
	if j, _ := g.jobs.Get(doc); j.Kind == job.EmergencyTend {
		t.Fatal("only a drafted colonist tends on command")
	}
}

func TestDefaultCareForPrisoners(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	capture(t, g, ada, bo, rid)
	dog, _ := g.SpawnAnimal("labrador", 6, 3)
	if !g.isPrisoner(rid) {
		t.Fatal("fixture: captured")
	}
	if g.care.Level(ada) != medcare.Best || g.care.Level(rid) != medcare.HerbalMax || g.care.Level(dog) != medcare.HerbalMax {
		t.Fatalf("defaults: colonist %v, prisoner %v, animal %v", g.care.Level(ada), g.care.Level(rid), g.care.Level(dog))
	}
	g.Enqueue(command.Command{Kind: command.SetDefaultCare, Index: int(medcare.Prisoners), Mode: uint8(medcare.NoMeds)})
	g.Step(1)
	if g.care.Level(rid) != medcare.NoMeds || g.care.Level(ada) != medcare.Best || g.DefaultCare()[medcare.Prisoners] != uint8(medcare.NoMeds) {
		t.Fatal("the prisoners' default applies to prisoners only")
	}
	setCare(g, rid, medcare.Best) // set by hand: wins over the default
	if g.care.Level(rid) != medcare.Best {
		t.Fatal("a patient's own level wins")
	}
	for _, c := range []command.Command{ // ignored: bad group, bad level, not a patient
		{Kind: command.SetDefaultCare, Index: 9, Mode: 1},
		{Kind: command.SetDefaultCare, Index: 0, Mode: 9},
		{Kind: command.SetMedicalCare, Pawn: uint64(rid), Mode: 9},
		{Kind: command.SetMedicalCare, Pawn: 1 << 30, Mode: 1},
	} {
		g.Enqueue(c)
	}
	g.Step(1)
	if _, own := g.care.Policy.Own(1 << 30); g.care.Level(rid) != medcare.Best || g.DefaultCare()[medcare.Colonists] != uint8(medcare.Best) || own {
		t.Fatal("bad care commands change nothing")
	}
	g.Enqueue(command.Command{Kind: command.SetDefaultCare, Index: int(medcare.Prisoners), Mode: uint8(medcare.HerbalMax)})
	g.Step(1)
	if s := g.care.Policy.Export(); s != nil && s.Def != nil {
		t.Fatal("back at the defaults: nothing custom left to save")
	}
}

func TestSelfTendToggle(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	addWound(t, g, id, body.ArmL, 6, body.Cut)
	if !g.care.SelfTendAllowed(id) {
		t.Fatal("self-tend is on by default")
	}
	g.Enqueue(command.Command{Kind: command.SetSelfTend, Pawn: uint64(id), Enabled: false})
	g.Step(1)
	g.interrupt(id, job.Cancelled)
	if g.care.Doctor.Assign(id) {
		t.Fatal("self-tend off: a lone colonist leaves its wounds")
	}
	laidDown(g, id)
	j, _ := g.jobs.Get(id)
	j.WorkLeft = 0
	if g.care.Rest.SelfTendDue(id, j) {
		t.Fatal("self-tend off: no tend in bed either")
	}
	if hv, _ := g.Health(id); hv.SelfTend || hv.Care != uint8(medcare.Best) {
		t.Fatalf("health view: %+v", hv)
	}
	g.Enqueue(command.Command{Kind: command.SetSelfTend, Pawn: uint64(id), Enabled: true})
	g.Step(1)
	g.interrupt(id, job.Cancelled)
	if !g.care.Doctor.Assign(id) {
		t.Fatal("self-tend on: it tends itself")
	}
	setCare(g, id, medcare.NoCare)
	if g.care.SelfTendAllowed(id) || g.Snapshot()[0].CareLevel != uint8(medcare.NoCare) {
		t.Fatal("no care: no self-tend, and the pawn view shows it")
	}
	g.care.SetSelfTend(1<<30, false) // not a colonist
	if g.care.Policy.SelfTendOff(1 << 30) {
		t.Fatal("self-tend is for colonists")
	}
}

func TestCareSurvivesSave(t *testing.T) {
	g, doc, patient := careScene(t)
	if g.care.Policy.Export() != nil {
		t.Fatal("all defaults: no Care save field (older saves hash as before)")
	}
	setCare(g, patient, medcare.NoMeds)
	g.Enqueue(command.Command{Kind: command.SetDefaultCare, Index: int(medcare.Animals), Mode: uint8(medcare.NoCare)})
	g.Enqueue(command.Command{Kind: command.SetSelfTend, Pawn: uint64(doc), Enabled: false})
	g.Step(1)
	l := saveLoad(t, g)
	if l.care.Level(patient) != medcare.NoMeds || l.care.Level(doc) != medcare.Best || l.care.SelfTendAllowed(doc) ||
		l.DefaultCare() != g.DefaultCare() || l.DefaultCare()[medcare.Animals] != uint8(medcare.NoCare) {
		t.Fatalf("loaded care %+v, want %+v", l.care, g.care)
	}
	if l.StateHash() != g.StateHash() {
		t.Fatal("the care settings hash the same after a load")
	}
	l.care.Policy.Import(&care.PolicySave{Per: []care.PolicyEntry{{ID: patient, V: 9}}, Def: []uint8{9, 0, 0, 0}})
	if l.care.Level(patient) != medcare.Best || l.DefaultCare()[medcare.Colonists] != uint8(medcare.Best) {
		t.Fatal("out-of-range saved levels fall back to the defaults")
	}
}

// TestCareReasonsMatchGame: the care package repeats three of the game's
// order reasons (it can't import them); they stay the same text.
func TestCareReasonsMatchGame(t *testing.T) {
	if care.ReasonIncapable != ReasonIncapable || care.ReasonBusyTarget != ReasonBusyTarget ||
		care.ReasonUnreachable != ReasonUnreachable {
		t.Fatal("care's reasons drifted from the game's")
	}
}
