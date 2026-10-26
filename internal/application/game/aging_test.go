package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// MED-08: aging and chronic conditions in the game.

// agingColony is n colonists of the given age.
func agingColony(t *testing.T, seed uint64, n, years int) (*Game, []entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: seed, Width: 24, Height: 12})
	var ids []entity.EntityID
	for i := 0; i < n; i++ {
		id, err := g.SpawnPawn(2+2*i, 6)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := g.sk.bios[id]; !ok {
			t.Fatal("a colonist has a bio")
		}
		setAge(g, id, years) // romance_test.go
		ids = append(ids, id)
	}
	return g, ids
}

// agingConds lists id's aging conditions.
func agingConds(g *Game, id entity.EntityID) []body.Condition {
	var out []body.Condition
	if b := g.bodies[id]; b != nil {
		for _, c := range b.Conds {
			if body.IsAging(c.Kind) {
				out = append(out, c)
			}
		}
	}
	return out
}

// rollDays runs the daily aging roll n times (one a day), with the
// hourly step at each day's end.
func rollDays(g *Game, n int) {
	for d := 1; d <= n; d++ {
		now := uint64(d) * clock.TicksPerDay
		g.rollAging(now)
		g.stepAgingConds(now)
	}
}

func TestNoChronicBeforeOnsetAge(t *testing.T) {
	g, ids := agingColony(t, 3, 6, 20) // before every onset age (artery blockage: 21)
	rollDays(g, 3000)
	for _, id := range ids {
		if c := agingConds(g, id); len(c) != 0 {
			t.Fatalf("a 20-year-old got %+v", c)
		}
	}
	old, oids := agingColony(t, 3, 6, 85)
	rollDays(old, 600)
	n := 0
	for _, id := range oids {
		n += len(agingConds(old, id))
	}
	if n < 6 {
		t.Fatalf("six 85-year-olds over ten years got only %d conditions", n)
	}
}

// TestOnsetChanceRisesWithAge: the older colonists collect more
// conditions over the same days.
func TestOnsetChanceRisesWithAge(t *testing.T) {
	count := func(years int) int {
		g, ids := agingColony(t, 11, 8, years)
		rollDays(g, 240)
		n := 0
		for _, id := range ids {
			n += len(agingConds(g, id))
		}
		return n
	}
	mid, old := count(45), count(75)
	if old <= mid {
		t.Fatalf("75-year-olds %d conditions, 45-year-olds %d", old, mid)
	}
}

func TestAgingRollSkipsTheDead(t *testing.T) {
	g, ids := agingColony(t, 5, 2, 90)
	v := g.VitalOf(ids[1])
	v.Status = health.Dead
	g.vital.Set(ids[1], v)
	rollDays(g, 600)
	if len(agingConds(g, ids[1])) != 0 {
		t.Fatal("the dead do not age")
	}
	if len(agingConds(g, ids[0])) == 0 {
		t.Fatal("a 90-year-old over ten years gets something")
	}
}

// heartAttackRun gives a colonist a heart attack and steps the game
// hourly until it ends; it returns the hours and whether it killed.
func heartAttackRun(t *testing.T, seed uint64) (int, health.Record, []body.Condition) {
	t.Helper()
	g, ids := agingColony(t, seed, 1, 60)
	id := ids[0]
	g.developAging(id, body.CondHeartAttack, int(body.Heart), uint64(g.clock.Now()))
	if _, ok := g.bodies[id].Cond(body.CondHeartAttack); !ok {
		t.Fatal("no heart attack")
	}
	for h := 1; h < 200; h++ {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(int(clock.TicksPerHour))
		if v := g.VitalOf(id); v.Status == health.Dead {
			return h, v, nil
		}
		if _, ok := g.bodies[id].Cond(body.CondHeartAttack); !ok {
			return h, g.VitalOf(id), append([]body.Condition(nil), g.bodies[id].Conds...)
		}
	}
	t.Fatal("a heart attack ends")
	return 0, health.Record{}, nil
}

func TestHeartAttackDeterministic(t *testing.T) {
	deaths := 0
	for s := uint64(1); s <= 12; s++ {
		h1, v1, c1 := heartAttackRun(t, s)
		h2, v2, c2 := heartAttackRun(t, s)
		if h1 != h2 || v1.Status != v2.Status || len(c1) != len(c2) {
			t.Fatalf("seed %d: %d h %+v vs %d h %+v", s, h1, v1, h2, v2)
		}
		if v1.Status == health.Dead {
			deaths++
			if v1.Cause != health.CauseHeartAttack || v1.Cause.String() != "heart attack" {
				t.Fatalf("cause %+v", v1)
			}
		}
	}
	if deaths == 0 {
		t.Fatal("an untended heart attack kills some of the time")
	}
}

func TestPumpHeartPreventsHeartAttack(t *testing.T) {
	g, ids := agingColony(t, 9, 2, 90)
	for _, id := range ids {
		v := g.VitalOf(id)
		b := g.ensureBody(id, &v)
		if !b.Apply(body.Operation{Kind: body.InstallImplant, Part: body.Heart, Item: item.PumpHeart}, body.Success, nil, 0) {
			t.Fatal("fit the pump heart")
		}
		g.bodyChanged(id, b, &v)
	}
	rollDays(g, 3000)
	for _, id := range ids {
		for _, c := range agingConds(g, id) {
			if c.Kind == body.CondArteryBlockage || c.Kind == body.CondHeartAttack {
				t.Fatalf("a pump heart never blocks or attacks: %+v", c)
			}
		}
	}
	// a natural heart with a blockage at its worst stage does, in time
	h, hids := agingColony(t, 9, 1, 30)
	v := h.VitalOf(hids[0])
	b := h.ensureBody(hids[0], &v)
	b.AddCond(body.Condition{Kind: body.CondArteryBlockage, Part: body.Heart, Sev: 950, Stage: 4})
	h.bodyChanged(hids[0], b, &v)
	attacked := false
	for d := 1; d <= 400 && !attacked; d++ {
		h.rollAging(uint64(d) * clock.TicksPerDay)
		_, attacked = h.bodies[hids[0]].Cond(body.CondHeartAttack)
	}
	if !attacked {
		t.Fatal("an extreme blockage (MTB 30 days) brings a heart attack within 400 days")
	}
}

func TestArteryBlockageProgresses(t *testing.T) {
	g, ids := agingColony(t, 2, 1, 30)
	id := ids[0]
	g.developAging(id, body.CondArteryBlockage, int(body.Heart), 0)
	if !g.indexedPatient(id) {
		t.Fatal("a chronic patient is indexed")
	}
	pump := g.bodies[id].Capacity(body.BloodPumping) // 950: minor, heart −5 %
	prev := int32(0)
	for y := 1; y <= 60; y++ {
		g.stepAgingConds(uint64(y) * clock.DaysPerYear * clock.TicksPerDay)
		if g.VitalOf(id).Status == health.Dead {
			if g.VitalOf(id).Cause != health.CauseHeartAttack {
				t.Fatalf("cause %+v", g.VitalOf(id))
			}
			if y < 7 {
				t.Fatalf("dead after %d years (wiki: 7.9 at the fastest)", y)
			}
			return
		}
		c, ok := g.bodies[id].Cond(body.CondArteryBlockage)
		if !ok || c.Sev < prev {
			t.Fatalf("year %d: %+v", y, c)
		}
		prev = c.Sev
		if y == 30 && g.bodies[id].Capacity(body.BloodPumping) >= pump {
			t.Fatal("the heart pumps less as it blocks")
		}
	}
	t.Fatal("artery blockage kills within 48 years (wiki: 47.6 at the slowest)")
}

func TestExciseCarcinoma(t *testing.T) {
	g, patient, doc := surgeryColony(t, 4)
	v := g.VitalOf(patient)
	b := g.ensureBody(patient, &v)
	if !b.Develop(body.CondCarcinoma, body.Stomach, 0) {
		t.Fatal("develop")
	}
	g.bodyChanged(patient, b, &v)
	hv, _ := g.Health(patient)
	found := false
	for _, o := range hv.Options {
		if o.Kind == uint8(body.ExciseCarcinoma) {
			found = o.Part == uint8(body.Stomach) && !o.Disabled && strings.Contains(o.Label, "Excise carcinoma (stomach)")
		}
	}
	if !found {
		t.Fatalf("the picker offers the excision: %+v", hv.Options)
	}
	queueOp(g, patient, body.ExciseCarcinoma, body.Stomach, 0)
	operateAll(t, g, patient, doc)
	if l := surgeryLine(g, patient); !strings.Contains(l, "excised the carcinoma from the stomach") {
		t.Fatalf("the excision succeeds: %q", l)
	}
	if _, ok := g.bodies[patient].Cond(body.CondCarcinoma); ok {
		t.Fatal("the carcinoma is gone")
	}
	if n := g.countItem(item.IndustrialMedicine); n != 6 {
		t.Fatalf("an excision uses 4 industrial medicine: %d left", n)
	}
}

// replaceWith runs an install of implant k on part p and reports the
// patient's conditions after.
func replaceWith(t *testing.T, cond body.CondKind, p body.Part, k item.Kind) []body.Condition {
	t.Helper()
	g, patient, doc := surgeryColony(t, 4)
	v := g.VitalOf(patient)
	b := g.ensureBody(patient, &v)
	b.Develop(cond, p, 0)
	g.bodyChanged(patient, b, &v)
	g.SpawnStack(k, 1, 2, 9)
	queueOp(g, patient, body.InstallImplant, p, k)
	operateAll(t, g, patient, doc)
	if g.bodies[patient].Implant(p) != k {
		t.Fatalf("the %s is fitted (%q)", k, surgeryLine(g, patient))
	}
	return agingConds(g, patient)
}

func TestCataractCuredByEyeReplacement(t *testing.T) {
	if c := replaceWith(t, body.CondCataract, body.EyeL, item.OpticImplant); len(c) != 0 {
		t.Fatalf("an optic implant ends the cataract: %+v", c)
	}
	if c := replaceWith(t, body.CondHearingLoss, body.EarR, item.CochlearImplant); len(c) != 0 {
		t.Fatalf("a cochlear implant ends the hearing loss: %+v", c)
	}
}

func TestBadBackCuredBySpinalBrace(t *testing.T) {
	if c := replaceWith(t, body.CondBadBack, body.Spine, item.SpinalBrace); len(c) != 0 {
		t.Fatalf("a spinal brace ends the bad back: %+v", c)
	}
}

func TestAgingHealthRows(t *testing.T) {
	g, ids := agingColony(t, 6, 1, 70)
	id := ids[0]
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	b.Develop(body.CondBadBack, body.Spine, 0)
	b.Develop(body.CondCataract, body.EyeL, 0)
	g.bodyChanged(id, b, &v)
	hv, _ := g.Health(id)
	if hv.State != "Healthy" || len(hv.Conditions) != 2 || hv.Conditions[0].Label != "Bad back" ||
		hv.Conditions[1].Label != "Cataract (left eye)" || hv.Conditions[0].SevPct != 0 {
		t.Fatalf("permanent rows: %q %+v", hv.State, hv.Conditions)
	}
	if !strings.Contains(hv.Conditions[0].Tip, "spinal brace") || !strings.Contains(hv.Conditions[1].Tip, "optic implant") {
		t.Fatalf("the cards say how they end: %+v", hv.Conditions)
	}
	b.Develop(body.CondCarcinoma, body.Stomach, 0)
	hv, _ = g.Health(id)
	c := hv.Conditions[2]
	growth := body.CarcinomaGrowth(b.Conds[2], g.agingSalt(id)).String()
	if c.Label != "Carcinoma (stomach), "+growth || hv.State != "Carcinoma" || !strings.Contains(c.Tip, "Medicine 10") {
		t.Fatalf("carcinoma row: %q %+v", hv.State, c)
	}
	b.Develop(body.CondHeartAttack, body.Heart, 0)
	hv, _ = g.Health(id)
	if hv.State != "Heart attack: needs tending" || hv.Conditions[3].Label != "Heart attack (painful)" ||
		!hv.Conditions[3].Losing || !hv.Conditions[3].Due {
		t.Fatalf("heart attack row: %q %+v", hv.State, hv.Conditions[3])
	}
}

func TestDementiaWanders(t *testing.T) {
	g, ids := agingColony(t, 8, 1, 75)
	id := ids[0]
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	b.Develop(body.CondDementia, body.Brain, 0)
	g.bodyChanged(id, b, &v)
	wanders := 0
	for d := 1; d <= 100; d++ {
		g.rollAging(uint64(d) * clock.TicksPerDay)
		if br, ok := g.breaks[id]; ok {
			if br.kind != mentalbreak.SadWander {
				t.Fatalf("break %v", br.kind)
			}
			wanders++
			delete(g.breaks, id)
		}
	}
	if wanders < 8 || wanders > 35 { // MTB 5 days: about 20 in 100
		t.Fatalf("%d confused wanders in 100 days", wanders)
	}
}

// TestHeartAttackDownedDoesNotFlap: a heart attack hovering around its
// debilitating stage downs the patient once; the downing band keeps it
// down while the stage moves back and forth.
func TestHeartAttackDownedDoesNotFlap(t *testing.T) {
	g, ids := agingColony(t, 4, 1, 60)
	id := ids[0]
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	b.Develop(body.CondHeartAttack, body.Heart, 0)
	changes, last := 0, g.VitalOf(id).Status
	for i := 0; i < 40; i++ {
		c, _ := b.Cond(body.CondHeartAttack)
		c.Sev = 580 + int32(i%2)*40
		c.Stage = body.DefOfCond(body.CondHeartAttack).StageAt(c.Sev)
		v := g.VitalOf(id)
		g.bodyChanged(id, b, &v)
		g.updateDowned(id, b, &v)
		if s := g.VitalOf(id).Status; s != last {
			changes++
			last = s
		}
	}
	if changes > 1 || last != health.Downed {
		t.Fatalf("status changed %d times, ends %v", changes, last)
	}
}

func TestAgingConditionsSurviveSave(t *testing.T) {
	g, ids := agingColony(t, 6, 1, 70)
	id := ids[0]
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	b.Develop(body.CondCarcinoma, body.Liver, 0)
	b.Develop(body.CondHearingLoss, body.EarL, 0)
	g.bodyChanged(id, b, &v)
	h := loadAsVersion(t, g, SaveVersion)
	if c := agingConds(h, id); len(c) != 2 || !h.indexedPatient(id) {
		t.Fatalf("aging conditions round-trip: %+v", c)
	}
	if body.AgingRate(agingConds(h, id)[0], h.agingSalt(id)) != body.AgingRate(agingConds(g, id)[0], g.agingSalt(id)) {
		t.Fatal("the growth rate is the same after a load")
	}
}

// TestLegacySaveHasNoAgingConds: an older save loads with no aging
// conditions and starts rolling them.
func TestLegacySaveHasNoAgingConds(t *testing.T) {
	g, ids := agingColony(t, 6, 2, 70)
	h := loadAsVersion(t, g, 6)
	for _, id := range ids {
		if c := agingConds(h, id); len(c) != 0 {
			t.Fatalf("legacy save: %+v", c)
		}
	}
	rollDays(h, 600)
	if len(agingConds(h, ids[0]))+len(agingConds(h, ids[1])) == 0 {
		t.Fatal("the loaded colonists age")
	}
}

// TestAgingIsDeterministic: two same-seed colonies of old colonists,
// stepped across many midnights with conditions growing and a heart
// attack under way, end in the same state.
func TestAgingIsDeterministic(t *testing.T) {
	run := func() (uint64, int) {
		g, ids := agingColony(t, 21, 4, 85)
		g.clock = clock.NewAt(clock.Tick(30*clock.TicksPerDay - clock.TicksPerHour))
		v := g.VitalOf(ids[0])
		b := g.ensureBody(ids[0], &v)
		b.AddCond(body.Condition{Kind: body.CondArteryBlockage, Part: body.Heart, Sev: 950, Stage: 4})
		b.Develop(body.CondCarcinoma, body.Stomach, uint64(g.clock.Now()))
		g.bodyChanged(ids[0], b, &v)
		g.developAging(ids[1], body.CondHeartAttack, int(body.Heart), uint64(g.clock.Now()))
		for d := 0; d < 12; d++ {
			for _, id := range ids {
				g.SetNeeds(id, needs.Full, needs.Full)
			}
			g.Step(int(3 * clock.TicksPerHour))
			g.clock = clock.NewAt(clock.Tick(uint64(g.clock.Now()) + clock.TicksPerDay - 3*clock.TicksPerHour))
		}
		n := 0
		for _, id := range ids {
			n += len(agingConds(g, id))
		}
		return g.StateHash(), n
	}
	h1, n1 := run()
	h2, n2 := run()
	if h1 != h2 || n1 != n2 {
		t.Fatalf("same seed: %x/%d vs %x/%d", h1, n1, h2, n2)
	}
	if n1 < 2 {
		t.Fatalf("the colonists age: %d conditions", n1)
	}
}

// TestDoctorTendsHeartAttack: a doctor tends a heart attack (it takes
// tending every hour) and uses medicine on it.
func TestDoctorTendsHeartAttack(t *testing.T) {
	g, patient, doc := surgeryColony(t, 5)
	g.developAging(patient, body.CondHeartAttack, int(body.Heart), uint64(g.clock.Now()))
	tended := false
	for h := 0; h < 48; h++ {
		g.SetNeeds(patient, needs.Full, needs.Full)
		g.SetNeeds(doc, needs.Full, needs.Full)
		g.Step(int(clock.TicksPerHour / 5))
		c, on := g.bodies[patient].Cond(body.CondHeartAttack)
		if on && c.TendQ > 0 {
			tended = true
		}
		if !on || g.VitalOf(patient).Status == health.Dead {
			break
		}
	}
	if !tended || g.countItem(item.IndustrialMedicine) >= 10 {
		t.Fatalf("the doctor tends the heart attack: tended %v, medicine %d", tended, g.countItem(item.IndustrialMedicine))
	}
}
