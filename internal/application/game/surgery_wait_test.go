package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// SIM-049: what a queued operation waits for, the patient's bed, healthy
// part replacement and the picker's options.

func queueOp(g *Game, patient entity.EntityID, kind body.OpKind, part body.Part, k item.Kind) {
	g.Enqueue(command.Command{Kind: command.AddOperation, Pawn: uint64(patient),
		Index: int(kind), Build: uint8(part), Stuff: uint8(k)})
	g.Step(1)
}

// firstStack is the first stack of kind k (0: none).
func firstStack(g *Game, k item.Kind) entity.EntityID {
	for _, iid := range g.itemIDs {
		if st, ok := g.stacks.Get(iid); ok && st.Kind == k {
			return iid
		}
	}
	return 0
}

func studyAll(g *Game) {
	for p := research.Project(0); p < research.NumProjects; p++ {
		g.research[p] = research.DefOf(p).Cost
	}
}

func studyProsthetics(g *Game) {
	g.research[research.Prosthetics] = research.DefOf(research.Prosthetics).Cost
}

// waitOf is the status of patient's first queued operation, read twice
// (once straight, once through the Health view).
func waitOf(t *testing.T, g *Game, patient entity.EntityID) (string, bool) {
	t.Helper()
	ops := g.operationsOf(patient)
	if len(ops) == 0 {
		t.Fatal("nothing queued")
	}
	why, ok := g.surgeryWait(patient, ops[0])
	hv, _ := g.Health(patient)
	if len(hv.Operations) == 0 || hv.Operations[0].Status != why || hv.Operations[0].Blocked == ok {
		t.Fatalf("view status %+v, surgeryWait %q", hv.Operations, why)
	}
	return why, ok
}

func TestSurgeryWaitNoStockAndResearch(t *testing.T) {
	g, patient, _ := legless(t, 10)
	g.research = research.Progress{} // a new colony knows nothing yet
	queueOp(g, patient, body.InstallImplant, body.LegL, item.ProstheticLeg)
	if why, ok := waitOf(t, g, patient); ok || why != "needs research: Prosthetics" {
		t.Fatalf("locked prosthetic: %q %v", why, ok)
	}
	if g.awaitingSurgery(patient) {
		t.Fatal("a patient doesn't lie down for an operation nobody can start")
	}
	studyProsthetics(g)
	g.viewGen++
	if why, ok := waitOf(t, g, patient); ok || why != "no prosthetic leg in stock" {
		t.Fatalf("researched, none made: %q %v", why, ok)
	}
	sid, _ := g.SpawnStack(item.ProstheticLeg, 1, 3, 9)
	st, _ := g.stacks.Get(sid)
	st.Forbidden = true
	g.stacks.Set(sid, st)
	if why, _ := waitOf(t, g, patient); why != "no prosthetic leg in stock" {
		t.Fatalf("a forbidden one is not in stock: %q", why)
	}
	st.Forbidden = false
	g.stacks.Set(sid, st)
	if why, ok := waitOf(t, g, patient); !ok || why != "waiting for the patient to lie down" {
		t.Fatalf("in stock: %q %v", why, ok)
	}
	if !g.awaitingSurgery(patient) {
		t.Fatal("now it lies down")
	}
}

func TestSurgeryWaitResearchIgnoredWithStock(t *testing.T) {
	g, patient, _ := legless(t, 10)
	g.research = research.Progress{}
	g.SpawnStack(item.ProstheticLeg, 1, 3, 9) // traded, or from the dev menu
	queueOp(g, patient, body.InstallImplant, body.LegL, item.ProstheticLeg)
	if why, ok := waitOf(t, g, patient); !ok || why != "waiting for the patient to lie down" {
		t.Fatalf("one in stock needs no research to fit: %q %v", why, ok)
	}
}

func TestSurgeryWaitLoneColonist(t *testing.T) {
	g := New(Config{Seed: 4, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(10, 6, "Solo")
	g.SpawnStack(item.PegLeg, 1, 2, 9)
	v := g.VitalOf(patient)
	b := g.ensureBody(patient, &v)
	b.Missing |= 1<<body.LegL | 1<<body.FootL
	g.bodyChanged(patient, b, &v)
	addPegLeg(g, patient)
	if why, ok := waitOf(t, g, patient); ok || why != "needs another colonist who can doctor" {
		t.Fatalf("lone colonist: %q %v", why, ok)
	}
	hv, _ := g.Health(patient)
	if len(hv.Options) == 0 {
		t.Fatal("the picker still offers operations")
	}
	for _, o := range hv.Options {
		if !o.Disabled || o.Reason == "" {
			t.Fatalf("with nobody to operate every option is greyed with a reason: %+v", o)
		}
	}
	if g.awaitingSurgery(patient) {
		t.Fatal("a lone colonist never lies down for surgery")
	}
}

func TestSurgeryWaitUnreachableImplantNeverLiesDown(t *testing.T) {
	g := New(Config{Seed: 4, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(10, 6, "Patient")
	doc, _ := g.SpawnNamedPawn(4, 6, "Doc")
	placeBuilt(t, g, construction.Bed, construction.South, 12, 2)
	g.SpawnStack(item.PegLeg, 1, 2, 9)
	walledIn(t, g, 2, 9, 2, 9)
	v := g.VitalOf(patient)
	b := g.ensureBody(patient, &v)
	b.Missing |= 1<<body.LegL | 1<<body.FootL
	g.bodyChanged(patient, b, &v)
	addPegLeg(g, patient)
	if why, ok := waitOf(t, g, patient); ok || why != "the peg leg can't be reached" {
		t.Fatalf("walled-in implant: %q %v", why, ok)
	}
	// A day of play: the patient never lies down for it, the doctor never
	// sets out for it (the old 6h loop).
	for i := 0; i < 24*6; i++ {
		g.SetNeeds(patient, needs.Full, needs.Full)
		g.SetNeeds(doc, needs.Full, needs.Full)
		g.Step(clock.TicksPerHour / 6)
		if g.awaitingSurgery(patient) {
			t.Fatal("waits for an implant nobody can reach")
		}
		if j, _ := g.jobs.Get(doc); j.Kind == job.Surgery {
			t.Fatalf("the doctor sets out for an unreachable implant: %+v", j)
		}
	}
	if len(g.operationsOf(patient)) != 1 {
		t.Fatal("the operation stays queued")
	}
}

func TestSurgeryWaitDoctorOnTheWay(t *testing.T) {
	g, patient, doc := legless(t, 10)
	addPegLeg(g, patient)
	saw := map[string]bool{}
	for i := 0; i < 600 && len(g.operationsOf(patient)) > 0; i++ {
		g.SetNeeds(patient, needs.Full, needs.Full)
		g.SetNeeds(doc, needs.Full, needs.Full)
		g.Step(5)
		if ops := g.operationsOf(patient); len(ops) > 0 {
			why, ok := g.surgeryWait(patient, ops[0])
			if !ok {
				t.Fatalf("a doable operation is never blocked: %q", why)
			}
			saw[why] = true
		}
	}
	for _, want := range []string{"waiting for the patient to lie down", "a doctor is on the way"} {
		if !saw[want] {
			t.Fatalf("status %q never shown (saw %v)", want, saw)
		}
	}
}

// TestSurgeryWaitLyingDoesNotFlap: a patient lies down on the strict test
// (a free implant) and stays down on the looser one, so a hauler briefly
// holding the implant neither gets it up nor puts it down again.
func TestSurgeryWaitLyingDoesNotFlap(t *testing.T) {
	g, patient, doc := legless(t, 10)
	addPegLeg(g, patient)
	sid := firstStack(g, item.PegLeg)
	hold := func(on bool) {
		st, _ := g.stacks.Get(sid)
		st.Carrier = 0
		if on {
			st.Carrier = doc // a hauler moving it
		}
		g.stacks.Set(sid, st)
	}
	hold(true)
	if g.awaitingSurgery(patient) {
		t.Fatal("standing, a held implant doesn't put the patient down")
	}
	hold(false)
	if !g.awaitingSurgery(patient) {
		t.Fatal("a free implant does")
	}
	g.lieDownForCare(patient)
	j, _ := g.jobs.Get(patient)
	if j.Kind != job.Sleep {
		t.Fatalf("the patient lies down: %+v", j)
	}
	j.State = job.Do // in bed (no stepping: the doctor stays out of it)
	g.jobs.Set(patient, j)
	changes, prev := 0, g.awaitingSurgery(patient)
	for i := 0; i < 40; i++ {
		hold(i%2 == 0)
		if a := g.awaitingSurgery(patient); a != prev {
			changes++
			prev = a
		}
	}
	if changes != 0 || !prev {
		t.Fatalf("a lying patient keeps waiting while the implant comes and goes (changes %d, now %v)", changes, prev)
	}
	if why, ok := g.surgeryWait(patient, g.operationsOf(patient)[0]); !ok || why != "" {
		t.Fatalf("lying, with an implant about: ready (%q %v)", why, ok)
	}
}

func TestHealthyLegReplacedWithProsthetic(t *testing.T) {
	g := New(Config{Seed: 4, Width: 20, Height: 12})
	patient, _ := g.SpawnNamedPawn(10, 6, "Patient")
	doc, _ := g.SpawnNamedPawn(4, 6, "Doc")
	placeBuilt(t, g, construction.Bed, construction.South, 12, 2)
	studyProsthetics(g)
	g.SpawnStack(item.ProstheticLeg, 1, 2, 9)
	(*g.sk.skills[doc])[skill.Medicine].Level = 20
	hv, _ := g.Health(patient)
	i := slices.IndexFunc(hv.Options, func(o OperationOptionView) bool {
		return o.Label == "Replace left leg with prosthetic leg"
	})
	if i < 0 || hv.Options[i].Disabled {
		t.Fatalf("a healthy leg can be replaced: %+v", hv.Options)
	}
	o := hv.Options[i]
	g.Enqueue(command.Command{Kind: command.AddOperation, Pawn: uint64(patient), Index: int(o.Kind), Build: o.Part, Stuff: o.Item})
	g.Step(1)
	if ops := g.operationsOf(patient); len(ops) != 1 {
		t.Fatalf("the replacement is queued: %+v", ops)
	}
	if hv, _ := g.Health(patient); hv.Operations[0].Label != "Replace left leg with prosthetic leg" {
		t.Fatalf("queued label %+v", hv.Operations)
	}
	operateUntil(t, g, patient, doc)
	b := g.bodies[patient]
	if b.Implant(body.LegL) != item.ProstheticLeg || b.Missing&(1<<body.LegL) == 0 || b.Missing&(1<<body.FootL) == 0 {
		t.Fatalf("the leg (and foot) are replaced: missing %b implants %v", b.Missing, b.Fitted)
	}
	if m := b.Capacity(body.Moving); m == 1000 {
		t.Fatal("a prosthetic leg is not as good as a healthy one")
	}
	if fx := g.sk.stats[patient]; fx == nil || fx.body != keyOf(b) {
		t.Fatal("the stat sheet is rebuilt from the new body")
	}
	if g.stockOf(item.ProstheticLeg) != 0 {
		t.Fatal("the prosthetic is used up")
	}
	hv, _ = g.Health(patient)
	if pv := hv.Parts[body.LegL]; pv.Part != int(body.LegL) || pv.Implant != "prosthetic leg" || pv.Status != PartArtificial+": prosthetic leg" {
		t.Fatalf("part row %+v", pv)
	}
	if pv := hv.Parts[body.FootL]; pv.Status != PartReplaced || pv.Implant != "" {
		t.Fatalf("foot row %+v", pv)
	}
	for _, th := range g.bodyThoughts(patient) {
		if th.Kind == mood.MissingPart {
			t.Fatal("a replaced leg is no missing part")
		}
	}
}

func TestSurgeryPatientPrefersHospitalBed(t *testing.T) {
	g, patient, doc := legless(t, 20)
	studyAll(g)
	placeBuilt(t, g, construction.HospitalBed, construction.South, 15, 2)
	var own, hosp entity.EntityID
	for _, bid := range g.bldgIDs {
		b, _ := g.bldgs.Get(bid)
		switch b.Kind {
		case construction.Bed:
			own = bid
			g.bldgs.Set(bid, b.Claim(uint64(patient)))
		case construction.HospitalBed:
			hosp = bid
		}
	}
	if own == 0 || hosp == 0 {
		t.Fatal("beds placed")
	}
	addPegLeg(g, patient)
	for i := 0; i < 200 && !g.lyingForCare(patient); i++ {
		g.SetNeeds(patient, needs.Full, needs.Full)
		g.SetNeeds(doc, needs.Full, needs.Full)
		g.Step(5)
	}
	if g.bedUnder(patient) != hosp {
		t.Fatalf("the patient lies in the hospital bed (under %d, hospital %d)", g.bedUnder(patient), hosp)
	}
	if b, _ := g.bldgs.Get(own); b.SlotOf(uint64(patient)) < 0 {
		t.Fatal("its own bed stays its own")
	}
	operateUntil(t, g, patient, doc)
	if !g.inHospitalBed(patient) {
		t.Fatal("operated in the hospital bed")
	}
	for i := 0; i < 40 && (g.UnderAnaesthetic(patient) || g.surg.hosp[patient] != 0); i++ {
		g.SetNeeds(patient, needs.Full, needs.Full)
		g.Step(anaesthTicks / 10)
	}
	if b, _ := g.bldgs.Get(hosp); b.SlotOf(uint64(patient)) >= 0 {
		t.Fatal("up again, it gives the hospital bed back")
	}
	if b, _ := g.bldgs.Get(own); b.SlotOf(uint64(patient)) < 0 {
		t.Fatal("and keeps its own")
	}
}

func TestHealthOptions(t *testing.T) {
	g, patient, _ := legless(t, 10) // left leg and foot gone, a peg leg in stock
	g.research = research.Progress{}
	hv, _ := g.Health(patient)
	opt := func(label string) (OperationOptionView, bool) {
		i := slices.IndexFunc(hv.Options, func(o OperationOptionView) bool { return o.Label == label })
		if i < 0 {
			return OperationOptionView{}, false
		}
		return hv.Options[i], true
	}
	for label, want := range map[string]string{
		"Install peg leg (left leg)":              "",
		"Install prosthetic leg (left leg)":       "needs research: Prosthetics",
		"Replace right leg with peg leg":          "",
		"Replace right leg with prosthetic leg":   "needs research: Prosthetics",
		"Replace left eye with glass eye":         "needs research: Prosthetics",
		"Replace nose with wooden nose":           "no wooden nose in stock",
		"Replace right foot with prosthetic foot": "no prosthetic foot in stock",
		"Amputate right leg":                      "",
		"Amputate left eye":                       "",
	} {
		o, ok := opt(label)
		if !ok {
			t.Fatalf("option %q missing from %+v", label, hv.Options)
		}
		if o.Disabled != (want != "") || o.Reason != want {
			t.Fatalf("%q: disabled %v reason %q, want %q", label, o.Disabled, o.Reason, want)
		}
	}
	for _, label := range []string{"Install prosthetic foot (left foot)", "Amputate heart", "Amputate left leg", "Remove implant (left leg)"} {
		if _, ok := opt(label); ok {
			t.Fatalf("%q is not possible on this body", label)
		}
	}
	last := body.OpKind(0)
	for _, o := range hv.Options {
		if body.OpKind(o.Kind) < last {
			t.Fatalf("installs, then removals, then amputations: %+v", hv.Options)
		}
		last = body.OpKind(o.Kind)
	}
	again, _ := g.Health(patient)
	if !slices.Equal(again.Options, hv.Options) {
		t.Fatal("options are deterministic")
	}
	addPegLeg(g, patient)
	hv, _ = g.Health(patient)
	if _, ok := opt("Install peg leg (left leg)"); ok {
		t.Fatal("a queued operation's part is not offered again")
	}
	if _, ok := opt("Replace right leg with peg leg"); !ok {
		t.Fatal("other parts still are")
	}
}

func TestGlassEyeCoversTheMissingPartThought(t *testing.T) {
	g, patient, _ := legless(t, 10)
	b := g.bodies[patient]
	b.Missing = 1 << body.EyeL
	b.SetImplant(body.EyeL, item.GlassEye)
	if missingPart(b) || len(g.bodyThoughts(patient)) != 0 {
		t.Fatal("a glass eye covers the empty socket")
	}
	b.Missing |= 1 << body.EyeR
	if !missingPart(b) {
		t.Fatal("the other, empty socket still weighs")
	}
}
