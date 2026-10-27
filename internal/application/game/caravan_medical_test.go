package game

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// roadHours runs the world and medical phases for hours whole hours (the
// map stands still; the caravan's members are off it).
func roadHours(g *Game, hours int) {
	for h := 0; h < hours; h++ {
		now := uint64(g.clock.Now())
		g.clock.Advance(int(clock.TicksPerHour - now%clock.TicksPerHour))
		g.phaseMedical()
		g.phaseWorld()
	}
}

// setMedicine sets id's Medicine level.
func setMedicine(g *Game, id entity.EntityID, lvl int) {
	(*g.sk.skills[id])[skill.Medicine].Level = uint8(lvl)
}

func TestCaravanMemberCatchesFlu(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	g.ng.setup.Difficulty.DiseaseFreq = 100
	away := g.pawnIDs[0]
	formCaravan(t, g, nearDest(t, g), away)
	if !slices.Contains(g.diseaseCandidates(body.CondFlu), away) {
		t.Fatal("a caravan member is no flu candidate")
	}
	for _, id := range g.homeColonists() { // everyone at home already has it
		g.GiveDisease(id, uint8(body.CondFlu))
	}
	if cands := g.diseaseCandidates(body.CondFlu); !slices.Equal(cands, []entity.EntityID{away}) {
		t.Fatalf("candidates %v, want only the caravan member %d", cands, away)
	}
	if !g.diseaseOutbreak(incident.Flu, uint64(g.clock.Now())) {
		t.Fatal("no outbreak")
	}
	flu, ok := g.bodies[away].Cond(body.CondFlu)
	if !ok || !g.indexedPatient(away) {
		t.Fatalf("the caravan member did not catch it (patient %v)", g.indexedPatient(away))
	}
	letter := ""
	for _, l := range g.letters.list {
		if l.Title == "Flu" {
			letter = l.Text
		}
	}
	if !strings.Contains(letter, "away with the caravan to") {
		t.Fatalf("the letter does not name the caravan: %q", letter)
	}
	sev := flu.Sev
	roadHours(g, 6)
	if flu, _ = g.bodies[away].Cond(body.CondFlu); flu.Sev <= sev {
		t.Fatalf("the flu did not step on the road: %d → %d", sev, flu.Sev)
	}
	if v := g.WorldMap(); len(v.Caravans) != 1 || len(v.Caravans[0].Health) != 1 || !strings.Contains(v.Caravans[0].Health[0], "flu") {
		t.Fatalf("caravan view %+v", v.Caravans)
	}
}

func TestCaravanMemberDiesAwayCleanly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		cause health.Cause
		hurt  func(g *Game, id entity.EntityID)
	}{
		{"flu", health.CauseDisease, func(g *Game, id entity.EntityID) {
			g.GiveDisease(id, uint8(body.CondFlu))
			c, _ := g.bodies[id].Cond(body.CondFlu)
			c.Sev, c.Stage = 999, 2
		}},
		{"blood loss", health.CauseBloodLoss, func(g *Game, id entity.EntityID) {
			v, _ := g.vital.Get(id)
			b := g.ensureBody(id, &v)
			b.Wounds = append(b.Wounds, body.Wound{Part: body.Torso, Kind: body.Cut, Dmg: 10})
			b.BloodLoss = body.BloodLossDeath - 1
			g.bodyChanged(id, b, &v)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := worldGame(t)
			a, b := g.pawnIDs[0], g.pawnIDs[1]
			c := formCaravan(t, g, nearDest(t, g), a, b)
			cid := entity.EntityID(c.C.ID)
			for _, id := range []entity.EntityID{a, b} {
				setMedicine(g, id, 0) // no medic: nothing tends it
			}
			tc.hurt(g, a)
			roadHours(g, 1)
			v, _ := g.vital.Get(a)
			if v.Status != health.Dead || v.Cause != tc.cause {
				t.Fatalf("member is %v (%v), want dead of %v", v.Status, v.Cause, tc.cause)
			}
			if g.isStashed(a) || g.indexedPatient(a) {
				t.Fatalf("dead member still stashed %v / a patient %v", g.isStashed(a), g.indexedPatient(a))
			}
			if cv := g.caravanAt(cid); cv == nil || slices.Contains(cv.C.Members, uint64(a)) || !slices.Contains(cv.C.Members, uint64(b)) {
				t.Fatalf("caravan members after the death: %+v", cv)
			}
			if p := g.personOf(a); p.Where != whereDead {
				t.Fatalf("registry where %d", p.Where)
			}
			if _, st, ok := g.bodyOf(a); ok {
				t.Fatalf("a corpse appeared on the map: %+v", st)
			}
			road := false
			for _, l := range g.letters.list {
				road = road || l.Title == "Death on the road"
			}
			if !road {
				t.Fatal("no letter for the death on the road")
			}
		})
	}
}

func TestCaravanMedicTends(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	medic, patient := g.pawnIDs[0], g.pawnIDs[1]
	formCaravan(t, g, nearDest(t, g), medic, patient)
	setMedicine(g, medic, 10)
	setMedicine(g, patient, 0)
	g.GiveDisease(patient, uint8(body.CondFlu))
	v, _ := g.vital.Get(patient)
	pb := g.ensureBody(patient, &v)
	pb.Wounds = append(pb.Wounds, body.Wound{Part: body.ArmL, Kind: body.Cut, Dmg: 3})
	g.bodyChanged(patient, pb, &v)
	if pb.BleedPerDay() == 0 {
		t.Fatal("fixture: the cut does not bleed")
	}
	if g.caravanMedic(g.caravanOf(medic)) != medic {
		t.Fatalf("medic %d, want %d", g.caravanMedic(g.caravanOf(medic)), medic)
	}
	roadHours(g, 1)
	flu, _ := pb.Cond(body.CondFlu)
	now := uint64(g.clock.Now())
	if flu.TendQ <= 0 || flu.TendUntil <= now || pb.BleedPerDay() != 0 {
		t.Fatalf("not tended on the road: flu %+v, bleeding %d", *flu, pb.BleedPerDay())
	}
	sum := flu.TendSum
	roadHours(g, 2) // inside the 12-hour window: no second tend
	if flu, _ = pb.Cond(body.CondFlu); flu.TendSum != sum {
		t.Fatalf("tended again inside its window: %d → %d", sum, flu.TendSum)
	}
	roadHours(g, 12)
	if flu, _ = pb.Cond(body.CondFlu); flu.TendSum <= sum {
		t.Fatal("no new tend once the window ran out")
	}

	// a caravan with no one who knows medicine tends nothing
	g2 := worldGame(t)
	a, b := g2.pawnIDs[0], g2.pawnIDs[1]
	formCaravan(t, g2, nearDest(t, g2), a, b)
	setMedicine(g2, a, 0)
	setMedicine(g2, b, 0)
	g2.GiveDisease(b, uint8(body.CondFlu))
	roadHours(g2, 2)
	if f, _ := g2.bodies[b].Cond(body.CondFlu); f.TendQ != 0 {
		t.Fatalf("tended with no medic: %+v", *f)
	}
}
