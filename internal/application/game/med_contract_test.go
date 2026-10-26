package game

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"slices"
	"testing"
	"time"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// MED-00: the medical contract — body and drug saves as slices, the
// patients index, the care command stubs and operation reordering.

// loadLegacy saves g, lets edit rewrite the decoded save, then loads it
// as version v: edit can put the state back in an older save's shape.
func loadLegacy(t *testing.T, g *Game, v int, edit func(*saveState)) *Game {
	t.Helper()
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	var s saveState
	if err := dec.Decode(&info); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	info.Version = v
	edit(&s)
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	enc := gob.NewEncoder(zw)
	if err := enc.Encode(info); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(&s); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&out)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// pegLegged is a colonist with a peg leg fitted and some ashleaf
// tolerance.
func pegLegged(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 4, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(10, 6, "Patient")
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	b.Missing |= 1<<body.LegL | 1<<body.FootL
	if !b.Install(body.LegL, item.PegLeg) {
		t.Fatal("install")
	}
	g.bodyChanged(id, b, &v)
	g.drugState(id).Ingest(item.AshleafTea, nil, 50)
	g.warmIndexes(true) // roofs and rooms, as a load warms them, so hashes compare
	return g, id
}

// TestLegacyBodySaveKeepsImplants: a version 5 save wrote the implants as
// a fixed array of 20 parts and drug tolerance and last doses as arrays
// of 3; it loads into the slices with nothing lost.
func TestLegacyBodySaveKeepsImplants(t *testing.T) {
	g, id := pegLegged(t)
	moving := g.bodies[id].Capacity(body.Moving)
	check := func(name string, h *Game) {
		t.Helper()
		b := h.bodies[id]
		if b == nil || b.Implant(body.LegL) != item.PegLeg || b.Capacity(body.Moving) != moving {
			t.Fatalf("%s: body %+v", name, b)
		}
		s := h.drugs[id]
		if s == nil || s.ToleranceOf(item.AshleafTea) != 150 || s.LastDoseOf(item.AshleafTea) != 50 || s.Policy != drug.SocialDrinking {
			t.Fatalf("%s: drugs %+v", name, s)
		}
		if h.StateHash() != g.StateHash() {
			t.Fatalf("%s: state differs after the load", name)
		}
	}
	check("v5 relabelled", loadAsVersion(t, g, 5))
	legacy := loadLegacy(t, g, 5, func(s *saveState) {
		for i := range s.Bodies {
			b := &s.Bodies[i].V
			copy(b.Implants[:], b.Fitted)
			b.Fitted = nil
		}
		for i := range s.Drugs {
			d := &s.Drugs[i].V
			copy(d.Tolerance[:], d.Tolerances)
			copy(d.LastDose[:], d.LastDoses)
			d.Tolerances, d.LastDoses = nil, nil
		}
	})
	check("v5 legacy arrays", legacy)
}

// TestSavedBodyRoundTrip: the new body fields survive a save.
func TestSavedBodyRoundTrip(t *testing.T) {
	g, id := pegLegged(t)
	b := g.bodies[id]
	b.BloodLoss = 120
	b.Conds = []body.Condition{{Kind: 7, Part: body.Torso, Sev: 300}}
	b.Wounds = append(b.Wounds, body.Wound{Part: body.ArmL, Dmg: 3, Kind: body.Cut, TendQ: 400, TendAt: 9, InfectAt: 30, Bleed: 80})
	h := loadAsVersion(t, g, SaveVersion)
	got := h.bodies[id]
	if got.BloodLoss != 120 || !slices.Equal(got.Conds, b.Conds) || !slices.Equal(got.Wounds, b.Wounds) || !slices.Equal(got.Fitted, b.Fitted) {
		t.Fatalf("%+v", got)
	}
}

// patientScan is the patients index by brute force: every body that
// needs care (an open wound, a condition, blood loss) on a living pawn.
func patientScan(g *Game) []entity.EntityID {
	var out []entity.EntityID
	for id, b := range g.bodies {
		if g.isPatient(id, b) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// TestPatientsIndexMatchesScan: over a 20 000-tick colony run with two
// raids, every pawn who needs care is indexed at the end of every tick,
// the index equals a brute-force scan of the bodies right after every
// hourly sweep, and it is rebuilt the same on load.
func TestPatientsIndexMatchesScan(t *testing.T) {
	ticks := 20000
	if testing.Short() {
		ticks = 6000
	}
	g := NewColony(Config{Seed: 7, Width: 48, Height: 40})
	g.Raid(3)
	most, checks := 0, 0
	g.SetPhaseObserver(func(p string, _ time.Duration) {
		if p == PhaseMovement { // every tick: nobody who needs care is missing
			for _, id := range patientScan(g) {
				if _, found := slices.BinarySearch(g.med.patients, id); !found {
					t.Fatalf("tick %d: patient %d not indexed (%v)", g.clock.Now(), id, g.med.patients)
				}
			}
			return
		}
		if p != PhaseMedical || uint64(g.clock.Now())%clock.TicksPerHour != 0 {
			return
		}
		checks++
		if want := patientScan(g); !slices.Equal(g.med.patients, want) {
			t.Fatalf("tick %d: index %v, scan %v", g.clock.Now(), g.med.patients, want)
		}
		most = max(most, len(g.med.patients))
	})
	for i := 0; i < ticks; i += 500 {
		g.Step(500)
		if i == 2000 {
			g.Raid(3)
		}
	}
	g.SetPhaseObserver(nil)
	if most == 0 || checks < ticks/clock.TicksPerHour-1 {
		t.Fatalf("nobody was ever a patient (%d checks): the test proves nothing", checks)
	}
	g.sweepPatients()
	h := loadAsVersion(t, g, SaveVersion)
	if !slices.Equal(h.med.patients, g.med.patients) || !slices.Equal(h.med.patients, patientScan(h)) {
		t.Fatalf("after load %v, before %v", h.med.patients, g.med.patients)
	}
}

func TestPatientsIndexSortedUnique(t *testing.T) {
	var g Game
	for _, id := range []entity.EntityID{9, 3, 7, 3, 12, 9} {
		g.markPatient(id)
	}
	if want := []entity.EntityID{3, 7, 9, 12}; !slices.Equal(g.med.patients, want) {
		t.Fatalf("%v", g.med.patients)
	}
	g.clearPatient(7)
	g.clearPatient(100)
	if want := []entity.EntityID{3, 9, 12}; !slices.Equal(g.med.patients, want) {
		t.Fatalf("%v", g.med.patients)
	}
}

// TestPatientMarkedByWoundAndClearedWhenWell: a hit marks the pawn; once
// its wounds are healed or scarred the sweep clears it, and a dead pawn
// is no patient.
func TestPatientMarkedByWoundAndClearedWhenWell(t *testing.T) {
	g := New(Config{Seed: 4, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(10, 6, "Patient")
	other, _ := g.SpawnNamedPawn(4, 6, "Other")
	g.hurt(id, 8, 0)
	g.hurt(other, 8, 0)
	if !slices.Contains(g.med.patients, id) || !slices.Contains(g.med.patients, other) {
		t.Fatalf("hit pawns are patients: %v", g.med.patients)
	}
	b := g.bodies[id]
	for i := range b.Wounds {
		b.Wounds[i].Permanent = true // all scars: nothing left to treat
	}
	g.dieBy(other, health.CauseInjury, 0)
	g.sweepPatients()
	if len(g.med.patients) != 0 {
		t.Fatalf("well and dead pawns are cleared: %v", g.med.patients)
	}
}

func TestHealthViewMedicalDefaults(t *testing.T) {
	g, id := pegLegged(t)
	hv, ok := g.Health(id)
	if !ok || hv.DeathInHours != -1 || hv.Conditions != nil || hv.BloodLossPct != 0 {
		t.Fatalf("%+v", hv)
	}
}

// TestReorderOperation: ReorderOperation swaps a queued operation with
// its neighbour above (-1) or below (+1); anything out of range is a
// no-op.
func TestReorderOperation(t *testing.T) {
	g, patient, _ := legless(t, 10)
	addPegLeg(g, patient)
	queueOp(g, patient, body.Amputate, body.HandL, 0)
	queueOp(g, patient, body.Amputate, body.HandR, 0)
	parts := func() []body.Part {
		var out []body.Part
		for _, op := range g.operationsOf(patient) {
			out = append(out, op.Part)
		}
		return out
	}
	reorder := func(slot, by int) {
		g.Enqueue(command.Command{Kind: command.ReorderOperation, Pawn: uint64(patient), Index: slot, Count: by})
		g.Step(1)
	}
	if want := []body.Part{body.LegL, body.HandL, body.HandR}; !slices.Equal(parts(), want) {
		t.Fatalf("queued %v", parts())
	}
	reorder(0, 1)
	if want := []body.Part{body.HandL, body.LegL, body.HandR}; !slices.Equal(parts(), want) {
		t.Fatalf("down: %v", parts())
	}
	reorder(2, -1)
	if want := []body.Part{body.HandL, body.HandR, body.LegL}; !slices.Equal(parts(), want) {
		t.Fatalf("up: %v", parts())
	}
	for _, c := range [][2]int{{0, -1}, {2, 1}, {-1, 1}, {3, -1}, {0, 2}, {1, 0}} {
		reorder(c[0], c[1])
		if want := []body.Part{body.HandL, body.HandR, body.LegL}; !slices.Equal(parts(), want) {
			t.Fatalf("slot %d by %d must be a no-op: %v", c[0], c[1], parts())
		}
	}
	g.cmdReorderOperation(9999, 0, 1) // no such patient
	(&Game{}).cmdReorderOperation(patient, 0, 1)
}

// TestCareStubsChangeNothing: the MED-03 commands are routed but change
// nothing yet.
func TestCareStubsChangeNothing(t *testing.T) {
	a, id := pegLegged(t)
	b, _ := pegLegged(t)
	for _, c := range []command.Command{
		{Kind: command.SetMedicalCare, Pawn: uint64(id), Mode: 2},
		{Kind: command.SetDefaultCare, Index: 1, Mode: 3},
		{Kind: command.SetSelfTend, Pawn: uint64(id), Enabled: true},
		{Kind: command.EmergencyTend, Pawn: uint64(id), Target: uint64(id)},
	} {
		a.Enqueue(c)
	}
	a.Step(5)
	b.Step(5)
	if a.StateHash() != b.StateHash() {
		t.Fatal("a stub command changed the game")
	}
}

// TestAddOperationRefusesUnimplementedKinds: the surgery v2 operations
// are in the enum but cannot be queued yet.
func TestAddOperationRefusesUnimplementedKinds(t *testing.T) {
	g, patient, _ := legless(t, 10)
	for k := body.HarvestOrgan; k <= body.NumOps; k++ {
		queueOp(g, patient, k, body.HandL, 0)
	}
	if n := len(g.operationsOf(patient)); n != 0 {
		t.Fatalf("queued %d unimplemented operations", n)
	}
	queueOp(g, patient, body.Amputate, body.HandL, 0)
	if n := len(g.operationsOf(patient)); n != 1 {
		t.Fatal("amputation still queues")
	}
	if l := operationLabel(g.bodies[patient], body.Operation{Kind: body.Euthanize, Part: body.Torso}); l == "" || l == "Operation (torso)" {
		t.Fatalf("label %q", l)
	}
}
