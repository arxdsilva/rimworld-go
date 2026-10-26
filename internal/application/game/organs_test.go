package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/sim/entity"
)

// MED-05: organs, the life capacities and the servo tier in the game.

// organPatient is a colonist whose body has lost parts (Missing bits) and
// carries a small tended bruise, so it is on the patients index.
func organPatient(t *testing.T, lost ...body.Part) (*Game, entity.EntityID, *body.Body) {
	t.Helper()
	g := New(Config{Seed: 3, Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	for _, p := range lost {
		b.Missing |= 1 << p
	}
	b.Wounds = append(b.Wounds, body.Wound{Part: body.ArmL, Dmg: 2, Kind: body.Bruise, Tended: true})
	g.bodyChanged(id, b, &v)
	return g, id, b
}

func TestKidneyLossKillsWhenBothInGame(t *testing.T) {
	t.Parallel()
	g, id, b := organPatient(t, body.KidneyL)
	medHour(g)
	if v := g.VitalOf(id); v.Status == health.Dead || g.immunityGain(id) != 750 {
		t.Fatalf("one kidney: %v, immunity gain %d", v.Status, g.immunityGain(id))
	}
	b.Missing |= 1 << body.KidneyR // lost outside a hit (a condition, an older save)
	medHour(g)
	if v := g.VitalOf(id); v.Status != health.Dead || v.Cause != health.CauseOrganFailure {
		t.Fatalf("no kidneys: %v of %v, want dead of organ failure", v.Status, v.Cause)
	}
	if g.indexedPatient(id) {
		t.Fatal("the dead leave the patients index")
	}
}

func TestOneKidneyLowersImmunity(t *testing.T) {
	t.Parallel()
	g, id, _ := organPatient(t, body.KidneyR)
	h, hid, _ := organPatient(t)
	if g.immunityGain(id) != 750 || h.immunityGain(hid) != 1000 || g.immunityGain(entity.EntityID(9999)) != 1000 {
		t.Fatalf("immunity gain %d / %d", g.immunityGain(id), h.immunityGain(hid))
	}
	d := body.DefOfCond(body.CondInfection)
	one := body.Condition{Kind: body.CondInfection, Sev: 100}
	two := one
	infection.StepRace(&one, d, false, g.immunityGain(id), 24)
	infection.StepRace(&two, d, false, h.immunityGain(hid), 24)
	if one.Immunity >= two.Immunity || one.Immunity*1000/two.Immunity > 760 {
		t.Fatalf("a day's immunity: one kidney %d, two %d", one.Immunity, two.Immunity)
	}
}

func TestSpineLossDownsInGame(t *testing.T) {
	t.Parallel()
	g, id, b := organPatient(t, body.Spine)
	v := g.VitalOf(id)
	g.updateDowned(id, b, &v)
	if v := g.VitalOf(id); v.Status != health.Downed || b.Capacity(body.Moving) != 0 {
		t.Fatalf("no spine: %v, moving %d", v.Status, b.Capacity(body.Moving))
	}
	medHour(g)
	if v := g.VitalOf(id); v.Status != health.Downed {
		t.Fatalf("a lost spine is not fatal, only crippling: %v", v.Status)
	}
}

func TestHealthViewShowsOrgansAndNewCapacities(t *testing.T) {
	t.Parallel()
	g, id, b := organPatient(t, body.ArmL, body.HandL)
	b.Install(body.ArmL, item.ServoArm)
	b.Wounds = append(b.Wounds, body.Wound{Part: body.Liver, Dmg: 10, Kind: body.Bruise, Tended: true})
	hv, ok := g.Health(id)
	if !ok {
		t.Fatal("no health view")
	}
	caps := map[string]int{}
	for _, c := range hv.Capacities {
		caps[c.Label] = c.Pct
	}
	if len(hv.Capacities) != int(body.NumCaps) || caps["Blood pumping"] != 100 || caps["Blood filtration"] != 80 ||
		caps["Digestion"] != 80 || caps["Manipulation"] <= 100 {
		t.Fatalf("capacities %v", caps)
	}
	if len(hv.Parts) != int(body.NumParts) || hv.Parts[body.Liver].Label != "liver" ||
		!strings.HasPrefix(hv.Parts[body.Liver].Status, PartWounded) || hv.Parts[body.KidneyL].Status != PartHealthy {
		t.Fatalf("organ rows %+v / %+v", hv.Parts[body.Liver], hv.Parts[body.KidneyL])
	}
	for _, o := range hv.Options {
		if o.Label == "Amputate liver" || o.Label == "Amputate left kidney" || o.Label == "Amputate spine" ||
			o.Label == "Amputate left lung" || strings.HasPrefix(o.Label, "Remove implant (heart)") {
			t.Errorf("the picker offers %q", o.Label)
		}
	}
	found := false
	for _, o := range hv.Options {
		found = found || o.Label == "Replace heart with pump heart"
	}
	if !found {
		t.Fatal("the picker offers no artificial heart")
	}
}

func TestLegacyBodyHasNewOrgans(t *testing.T) {
	t.Parallel()
	g, id := pegLegged(t)
	v := g.VitalOf(id)
	b := g.bodies[id]
	b.Wounds = append(b.Wounds, body.Wound{Part: body.Torso, Dmg: 6, Kind: body.Cut, Tended: true, TendQ: 500})
	g.bodyChanged(id, b, &v)
	check := func(name string, h *Game) {
		t.Helper()
		hb := h.bodies[id]
		if hb == nil || hb.Implant(body.LegL) != item.PegLeg || hb.Damage() != b.Damage() {
			t.Fatalf("%s: body %+v", name, hb)
		}
		ps := hb.Parts()
		for _, p := range []body.Part{body.KidneyL, body.KidneyR, body.Liver, body.Spine} {
			if s := ps[p]; s.Missing || s.HP != body.DefOf(p).MaxHP || s.Wounds != 0 {
				t.Fatalf("%s: %v loaded as %+v", name, p, s)
			}
		}
		for _, c := range []body.Capacity{body.BloodPumping, body.BloodFiltration, body.Digestion} {
			if hb.Capacity(c) != 1000 {
				t.Fatalf("%s: capacity %d = %d", name, c, hb.Capacity(c))
			}
		}
		if hb.OrganFailure() || h.VitalOf(id).Status != health.Alive || h.immunityGain(id) != 1000 {
			t.Fatalf("%s: organs failing after the load", name)
		}
	}
	check("v5", loadAsVersion(t, g, 5))
	check("v6", loadAsVersion(t, g, 6))
	check("v5 legacy arrays", loadLegacy(t, g, 5, func(s *saveState) {
		for i := range s.Bodies {
			sb := &s.Bodies[i].V
			copy(sb.Implants[:], sb.Fitted)
			sb.Fitted = nil
		}
	}))
}

func TestNanomedIsMedicine(t *testing.T) {
	t.Parallel()
	for _, k := range []item.Kind{item.HerbalMedicine, item.IndustrialMedicine, item.NanomedKit} {
		if !isMedicine(k) {
			t.Errorf("%v is medicine", k)
		}
	}
	if isMedicine(item.Components) || isMedicine(item.ServoArm) {
		t.Fatal("not medicine")
	}
}
