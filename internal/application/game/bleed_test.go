package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// downDamage is the bruise damage that puts id into pain shock (it never
// tears a part off: see woundBody, a stump hurts as much as its HP).
func downDamage(g *Game, id entity.EntityID) int {
	return min(int((g.painShockAt(id)*10+124)/125), health.MaxHP-1)
}

// addWound puts a wound straight on id's body (no part roll), as a hit
// would: the body is re-derived and the downing rule applied.
func addWound(t *testing.T, g *Game, id entity.EntityID, p body.Part, dmg int, kind body.WoundKind) *body.Body {
	t.Helper()
	v, ok := g.vital.Get(id)
	if !ok {
		t.Fatalf("no vital record for %d", id)
	}
	b := g.ensureBody(id, &v)
	b.Wounds = append(b.Wounds, body.Wound{Part: p, Dmg: dmg, Kind: kind, At: uint64(g.clock.Now())})
	g.bodyChanged(id, b, &v)
	g.updateDowned(id, b, &v)
	g.woundOpened(id)
	return b
}

// medHour moves the clock to the next hour and runs only the medical
// phase (no doctors, no healing: the bleeding on its own).
func medHour(g *Game) {
	r := uint64(g.clock.Now()) % clock.TicksPerHour
	g.clock.Advance(int(clock.TicksPerHour - r))
	g.phaseMedical()
}

func TestUntendedCutBleedsOut(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	b := addWound(t, g, id, body.Torso, 30, body.Cut) // 1800 ‰/day: 75 an hour
	if v, _ := g.vital.Get(id); v.Status != health.Alive {
		t.Fatal("a 30 cut hurts (pain 375) but does not down")
	}
	want := b.BleedDeathHours()
	if want != 14 || g.bleedDeathHours(id) != 14 || !g.bleedingNow(id) {
		t.Fatalf("death in %d h", want)
	}
	downedAt := -1
	for h := 1; h <= 20; h++ {
		medHour(g)
		v, _ := g.vital.Get(id)
		if v.Status == health.Downed && downedAt < 0 {
			downedAt = h
		}
		if v.Status == health.Dead {
			// MED-02: the cut heals a point every 3 hours meanwhile, so it
			// bleeds a little slower than the first projection
			if h < want || h > want+1 || v.Cause != health.CauseBloodLoss {
				t.Fatalf("died at hour %d of %v, want hour %d-%d of blood loss", h, v.Cause, want, want+1)
			}
			if downedAt < 8 || downedAt > 9 { // 600 ‰ (extreme: consciousness capped at 10 %) after 8 h, 9 with the healing
				t.Fatalf("downed at hour %d, want 8-9", downedAt)
			}
			if _, found := g.patientIndexed(id); found {
				t.Fatal("the dead leave the patients index")
			}
			return
		}
	}
	t.Fatal("an untended 30 cut did not bleed out in 20 h")
}

// patientIndexed reports id in the patients index.
func (g *Game) patientIndexed(id entity.EntityID) (int, bool) {
	for i, p := range g.med.patients {
		if p == id {
			return i, true
		}
	}
	return -1, false
}

func TestBruisesAndBurnsDoNotBleed(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	addWound(t, g, id, body.Torso, 30, body.Bruise)
	addWound(t, g, id, body.ArmL, 10, body.Burn)
	if g.bleedingNow(id) || g.bleedDeathHours(id) != -1 || g.bleeding(id) {
		t.Fatal("bruises and burns do not bleed")
	}
	for range 30 {
		medHour(g)
	}
	if b := g.bodies[id]; b.BloodLoss != 0 {
		t.Fatalf("blood loss %d", b.BloodLoss)
	}
}

func TestTendStopsBleeding(t *testing.T) {
	g := New(Config{Seed: 3, Width: 20, Height: 12})
	ada, _ := g.SpawnNamedPawn(4, 5, "Ada")
	bo, _ := g.SpawnNamedPawn(8, 5, "Bo")
	for _, id := range []entity.EntityID{ada, bo} {
		g.SetNeeds(id, needs.Full, needs.Full)
	}
	addWound(t, g, bo, body.Torso, 70, body.Cut) // pain 875: down, bleeding 175 ‰ an hour
	if v, _ := g.vital.Get(bo); v.Status != health.Downed {
		t.Fatalf("pain shock downs: %+v", v)
	}
	for i := 0; i < 5*int(clock.TicksPerHour)/250 && g.bleedingNow(bo); i++ {
		g.Step(250)
	}
	v, _ := g.vital.Get(bo)
	if g.bleedingNow(bo) || v.Status == health.Dead {
		t.Fatalf("Ada should have tended Bo before he bled out: %+v, loss %d", v, g.bodies[bo].BloodLoss)
	}
	loss := g.bodies[bo].BloodLoss
	if loss <= 0 || loss >= body.BloodLossDeath {
		t.Fatalf("loss %d", loss)
	}
	g.Step(int(3 * clock.TicksPerHour))
	if b := g.bodies[bo]; b.BleedPerDay() != 0 || b.BloodLoss >= loss {
		t.Fatalf("tended: bleed %d, loss %d (was %d): blood comes back", b.BleedPerDay(), b.BloodLoss, loss)
	}
}

func TestRaiderBleedsOutLikeColonist(t *testing.T) {
	g, ada := raidGame(t)
	g.Raid(1)
	rid := g.raiderIDs[0]
	weapon := g.equip[rid]
	col := uint64ToID(ada)
	addWound(t, g, rid, body.Torso, 70, body.Cut) // 4200 ‰/day
	addWound(t, g, col, body.Torso, 70, body.Cut)
	for _, id := range []entity.EntityID{rid, col} {
		if v, _ := g.vital.Get(id); v.Status != health.Downed {
			t.Fatalf("%d: a 70 cut downs (pain shock)", id)
		}
	}
	died := map[entity.EntityID]int{}
	for h := 1; h <= 12 && len(died) < 2; h++ {
		medHour(g)
		for _, id := range []entity.EntityID{rid, col} {
			if v, _ := g.vital.Get(id); v.Status == health.Dead && died[id] == 0 {
				died[id] = h
			}
		}
	}
	if died[rid] != 6 || died[col] != 6 {
		t.Fatalf("raider bled out after %d h, colonist after %d h; want 6 each (175 ‰ an hour)", died[rid], died[col])
	}
	if v, _ := g.vital.Get(rid); v.Cause != health.CauseBloodLoss {
		t.Fatalf("cause %v", v.Cause)
	}
	// the raid AI drops what it carried and leaves the body
	g.Step(2)
	if st, ok := stackOf(g, item.CorpseHuman); !ok || st.Of != rid && st.Of != col {
		t.Fatal("its body stays as a corpse until buried (SIM-017)")
	}
	if _, ok := stackOf(g, weapon); !ok {
		t.Fatalf("its %s should be left behind", weapon)
	}
}

func TestRaiderBleedsOutInTheRealGame(t *testing.T) {
	g, _ := raidGame(t)
	g.Raid(1)
	rid := g.raiderIDs[0]
	g.hurtAs(rid, 70, 0, body.Cut, health.CauseInjury) // cuts on rolled parts (a stump may bleed faster)
	if v, _ := g.vital.Get(rid); v.Status != health.Downed {
		t.Fatalf("downed: %+v", v)
	}
	start := uint64(g.clock.Now())
	for i := 0; i < 10*int(clock.TicksPerHour)/250; i++ {
		g.Step(250)
		if v, _ := g.vital.Get(rid); v.Status == health.Dead {
			break
		}
	}
	v, _ := g.vital.Get(rid)
	hours := (uint64(g.clock.Now()) - start) / clock.TicksPerHour
	if v.Status != health.Dead || v.Cause != health.CauseBloodLoss || hours < 1 || hours > 7 {
		t.Fatalf("raider: %+v after %d h", v, hours)
	}
}

func TestDownedHysteresis(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	shock := g.painShockAt(id)
	if shock != health.PainShock {
		t.Fatalf("plain pawn shock %d", shock)
	}
	b := addWound(t, g, id, body.Torso, 64, body.Bruise) // pain 800
	v, _ := g.vital.Get(id)
	if v.Status != health.Downed || v.Cause != health.CauseInjury {
		t.Fatalf("pain shock downs: %+v", v)
	}
	set := func(dmg int) health.Record {
		b.Wounds[len(b.Wounds)-1].Dmg = dmg
		v, _ := g.vital.Get(id)
		g.bodyChanged(id, b, &v)
		g.updateDowned(id, b, &v)
		return v
	}
	for _, dmg := range []int{63, 62, 61} { // pain 787..762: inside the band
		if v := set(dmg); v.Status != health.Downed {
			t.Fatalf("stood at %d HP (pain %d)", dmg, b.Pain())
		}
	}
	if v := set(60); v.Status != health.Alive { // pain 750
		t.Fatalf("still down at pain %d", b.Pain())
	}
	if v := set(63); v.Status != health.Alive {
		t.Fatal("back inside the band from below: stays up")
	}
	if v := set(64); v.Status != health.Downed {
		t.Fatal("pain shock again")
	}
	// moving: both legs gone and nothing fitted keeps a pawn down
	g2 := New(Config{Seed: 1, Width: 16, Height: 12})
	p, _ := g2.SpawnNamedPawn(5, 5, "Bo")
	for _, part := range []string{"left leg", "right leg"} {
		if !g2.DevLosePart(p, part) {
			t.Fatalf("lose %s", part)
		}
	}
	if v, _ := g2.vital.Get(p); v.Status != health.Downed || g2.standsUp(p, v) {
		t.Fatalf("no legs: %+v", v)
	}
}

func TestBloodLossDoesNotFlap(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	b := addWound(t, g, id, body.ArmL, 1, body.Bruise) // a patient, nothing bleeding
	changes, last := 0, health.Alive
	for h := 0; h < 48; h++ {
		// 590, 610, 590, ... after the hour's regeneration
		b.BloodLoss = 600 + int32(10*(h%2*2-1)) + body.BloodRegenPerDay/24 + 1
		medHour(g)
		v, _ := g.vital.Get(id)
		if v.Status == health.Dead {
			t.Fatal("blood loss under 1000 does not kill")
		}
		if v.Status != last {
			changes++
			last = v.Status
		}
	}
	if changes > 1 {
		t.Fatalf("downed state changed %d times in 48 h at 600 ± 10", changes)
	}
	if last != health.Downed {
		t.Fatal("extreme blood loss downs")
	}
	// without the stage memory the same loss would flip the stage each hour
	if body.BloodStage(590, 0) == body.BloodStage(610, 0) {
		t.Fatal("the fixture does not straddle the threshold")
	}
}

func TestHealthViewBleeding(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	hv, ok := g.healthView(id)
	if !ok || hv.State != "Healthy" || hv.DeathInHours != -1 || hv.BleedPerDay != 0 {
		t.Fatalf("healthy view %+v", hv)
	}
	addWound(t, g, id, body.ArmL, 10, body.Cut) // 60 %/day
	addWound(t, g, id, body.LegL, 10, body.Bruise)
	medHour(g)
	hv, _ = g.healthView(id)
	if hv.BleedPerDay != 60 || hv.BloodLossPct != 2 || hv.DeathInHours != 39 ||
		!strings.HasPrefix(hv.State, "Bleeding 60%/day") || !strings.Contains(hv.State, "death in 39h") {
		t.Fatalf("bleeding view %+v", hv)
	}
	if hv.Parts[body.ArmL].Bleed != 60 || hv.Parts[body.LegL].Bleed != 0 {
		t.Fatalf("part bleed %d %d", hv.Parts[body.ArmL].Bleed, hv.Parts[body.LegL].Bleed)
	}
	var pv PawnView
	for _, p := range g.Snapshot() {
		if p.ID == id {
			pv = p
		}
	}
	if !pv.Bleeding || pv.BleedDeathHours != 39 {
		t.Fatalf("pawn view bleeding %v %d", pv.Bleeding, pv.BleedDeathHours)
	}
	v, _ := g.vital.Get(id)
	b := g.bodies[id]
	b.TendAll(0, uint64(g.clock.Now())) // tended (MED-02)
	g.bodyChanged(id, b, &v)
	hv, _ = g.healthView(id)
	if hv.State != "Injured" || hv.DeathInHours != -1 || hv.BleedPerDay != 0 {
		t.Fatalf("tended view %+v", hv)
	}
	if healthState(health.Record{Status: health.Downed}, &body.Body{}) != "Downed" ||
		healthState(health.Record{Status: health.Dead}, &body.Body{}) != "Dead" {
		t.Fatal("states")
	}
}

func TestDoctorTendsTheFastestBleederFirst(t *testing.T) {
	g := New(Config{Seed: 3, Width: 24, Height: 12})
	doc, _ := g.SpawnNamedPawn(4, 5, "Ada")
	near, _ := g.SpawnNamedPawn(6, 5, "Bo")
	far, _ := g.SpawnNamedPawn(16, 5, "Cy")
	for _, id := range []entity.EntityID{doc, near, far} {
		g.SetNeeds(id, needs.Full, needs.Full)
	}
	addWound(t, g, near, body.Torso, 70, body.Bruise) // worse hurt, but not bleeding
	addWound(t, g, far, body.Torso, 66, body.Cut)
	if !g.assignDoctor(doc) {
		t.Fatal("no doctor job")
	}
	if j, _ := g.jobs.Get(doc); j.Target != far {
		t.Fatalf("doctor went for %d, want the bleeding %d", j.Target, far)
	}
}

func TestBloodStateSurvivesSave(t *testing.T) {
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(5, 5, "Ada")
	b := addWound(t, g, id, body.Torso, 10, body.Cut)
	b.Fresh = 1 << body.HandL
	b.Missing |= 1 << body.HandL
	for range 6 { // 3000 ‰/day with the stump: 750 after 6 h, less the cut healing a point every 3 h (MED-02)
		medHour(g)
	}
	want := *g.bodies[id]
	if v, _ := g.vital.Get(id); want.BloodLoss != 744 || want.BloodStage != body.BloodStageExtreme || v.Status != health.Downed {
		t.Fatalf("fixture: %+v", want)
	}
	got := saveLoad(t, g).bodies[id]
	if got.BloodLoss != want.BloodLoss || got.BloodStage != want.BloodStage || got.Fresh != want.Fresh {
		t.Fatalf("loaded %+v, want %+v", *got, want)
	}
}

// A downed raider that does not bleed heals slowly, gets up once past the
// band and flees (MED-01: no bleed-out timer keeps it lying forever).
func TestBruisedRaiderGetsUpAndFlees(t *testing.T) {
	g, _ := raidGame(t)
	g.Raid(1)
	rid := g.raiderIDs[0]
	addWound(t, g, rid, body.Torso, 66, body.Bruise)
	if v, _ := g.vital.Get(rid); v.Status != health.Downed {
		t.Fatalf("pain shock downs: %+v", v)
	}
	for i := 0; i < 100; i++ { // MED-02: 6 HP at 12 a day lying on the ground: about 12 h
		g.Step(clock.TicksPerHour / 4)
		if r, ok := g.raiders[rid]; !ok || r.Mode == raidFlee {
			break
		}
	}
	v, _ := g.vital.Get(rid)
	if r, ok := g.raiders[rid]; ok && (v.Status != health.Alive || r.Mode != raidFlee) {
		t.Fatalf("raider %+v mode %v", v, r.Mode)
	}
}
