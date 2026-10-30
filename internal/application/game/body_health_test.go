package game

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/entity"
)

// ENG-003 M1: humanlike bodies behind health.Record.Injury.

func TestHurtAddsWoundAndInjuryIsBodyDamage(t *testing.T) {
	g := New(Config{Seed: 3, Width: 12, Height: 8})
	p, _ := g.SpawnNamedPawn(4, 4, "Ada")
	g.hurt(p, 12, 0)
	b := g.bodies[p]
	if b == nil || len(b.Wounds) == 0 {
		t.Fatalf("a hit leaves a wound on the body: %+v", b)
	}
	if v := g.VitalOf(p); v.Injury != 12 || v.Injury != b.Damage() {
		t.Fatalf("Injury %d, body damage %d, want both 12", v.Injury, b.Damage())
	}
	for _, w := range b.Wounds {
		if w.Kind != body.Bruise {
			t.Fatalf("an attackerless hit bruises: %+v", w)
		}
	}
	g.hurt(p, 7, 0)
	if v := g.VitalOf(p); v.Injury != 19 || v.Injury != g.bodies[p].Damage() {
		t.Fatalf("a second hit adds up: Injury %d, damage %d", v.Injury, g.bodies[p].Damage())
	}
}

func TestHitKindsFollowTheWeapon(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	bo, _ := g.SpawnNamedPawn(3, 2, "Bo")
	knife := weapon.HitOf(weapon.Knife, weapon.DefOf(weapon.Knife))
	knife.Amount = 5
	g.equip[bo] = item.Rifle // CBT-1: the blow's own weapon decides, not what bo holds now
	hits(g).HurtHit(ada, hits(g).Absorb(ada, knife, g.rng.IntN), bo)
	if w := g.bodies[ada].Wounds; len(w) == 0 || w[0].Kind != body.Cut {
		t.Fatalf("a knife hit leaves a cut: %+v", w)
	}
	if fists := weapon.HitOf(weapon.Fists, weapon.DefOf(weapon.Fists)); body.WoundOf(fists.Form) != body.Bruise {
		t.Fatal("fists bruise")
	}
}

func TestDownedAndDeathThresholdsUnchanged(t *testing.T) {
	for seed := uint64(1); seed <= 25; seed++ {
		g := New(Config{Seed: seed, Width: 12, Height: 8})
		p, _ := g.SpawnNamedPawn(4, 4, "Ada")
		g.hurt(p, health.DownedInjury, 0)
		if v := g.VitalOf(p); v.Status != health.Downed || v.Injury != health.DownedInjury {
			t.Fatalf("seed %d: %d damage downs, not kills: %+v", seed, health.DownedInjury, v)
		}
		g.hurt(p, health.MaxHP-health.DownedInjury-1, 0)
		if v := g.VitalOf(p); v.Status == health.Dead {
			t.Fatalf("seed %d: %d damage is not yet death: %+v", seed, health.MaxHP-1, v)
		}
		g.hurt(p, 1, 0)
		if v := g.VitalOf(p); v.Status != health.Dead || v.Injury != health.MaxHP {
			t.Fatalf("seed %d: %d damage kills: %+v", seed, health.MaxHP, v)
		}
	}
}

// TestPlainWoundHealsAtWikiRate (MED-02; replaces the old 300-tick
// cadence): an untended bruise on a standing colonist heals 8 HP a day,
// a point every 3 hours, and Injury follows the body.
func TestPlainWoundHealsAtWikiRate(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	p, _ := g.SpawnNamedPawn(4, 4, "Ada")
	addWound(t, g, p, body.LegL, 6, body.Bruise) // bruises never scar
	for h := 1; h <= 18; h++ {
		medHour(g)
		v := g.VitalOf(p)
		if want := 6 - h/3; v.Injury != want {
			t.Fatalf("after %d h: Injury %d, want %d", h, v.Injury, want)
		}
		if v.Injury != g.bodies[p].Damage() {
			t.Fatalf("Injury %d drifted from the body %d", v.Injury, g.bodies[p].Damage())
		}
	}
	if g.indexedPatient(p) {
		t.Fatal("healed: the sweep drops the patient")
	}
}

// TestClosedCutsMayScarDeterministically: deep cuts close and some scar
// (20 per mille for a 14+ HP cut), the same ones for the same seed.
func TestClosedCutsMayScarDeterministically(t *testing.T) {
	scars := func() (int, []entry[savedBody]) {
		g := New(Config{Seed: 9, Width: 12, Height: 8})
		p, _ := g.SpawnNamedPawn(4, 4, "Ada")
		v := g.VitalOf(p)
		b := g.ensureBody(p, &v)
		for i := 0; i < 600; i++ {
			b.Wounds = append(b.Wounds, body.Wound{Part: body.Torso, Dmg: 1, Kind: body.Cut, Tended: true, Size: 20})
		}
		g.bodyChanged(p, b, &v)
		for i := 0; i < 3; i++ {
			medHour(g)
		}
		n := 0
		for _, w := range g.bodies[p].Wounds {
			if !w.Permanent {
				t.Fatalf("a 1 HP tended cut closes in 2 hours: %+v", w)
			}
			n++
		}
		v = g.VitalOf(p)
		if v.Injury != n {
			t.Fatalf("scars keep a point each: Injury %d for %d scars", v.Injury, n)
		}
		return n, g.exportBodies()
	}
	n, a := scars()
	if n == 0 || n > 30 {
		t.Fatalf("about 2%% of closed deep cuts scar: %d of 600", n)
	}
	if m, b := scars(); m != n || !reflect.DeepEqual(a, b) {
		t.Fatalf("scars follow the seed: %d vs %d", n, m)
	}
}

func TestMissingJawSilencesTalkingInTheView(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	p, _ := g.SpawnNamedPawn(4, 4, "Ada")
	v := g.VitalOf(p)
	b := g.ensureBody(p, &v)
	b.Missing |= 1 << body.Jaw
	g.bodyChanged(p, b, &v)
	hv, ok := g.Health(p)
	if !ok {
		t.Fatal("a colonist has a Health tab")
	}
	if len(hv.Parts) != int(body.NumParts) || hv.Parts[body.Jaw].Status != PartMissing {
		t.Fatalf("parts list every part in order, jaw missing: %+v", hv.Parts)
	}
	got := map[string]int{}
	for _, c := range hv.Capacities {
		got[c.Label] = c.Pct
	}
	if got["Talking"] != 0 || got["Moving"] != 100 || got["Sight"] != 100 {
		t.Fatalf("capacities = %v", got)
	}
	found := false
	for _, th := range g.thoughts(p) {
		found = found || th.Kind == mood.MissingPart
	}
	if !found {
		t.Fatal("a missing part weighs on the mood")
	}
}

func TestCapacitiesFeedStatsOnlyOnChange(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	p, _ := g.SpawnNamedPawn(4, 4, "Ada")
	base := g.statOf(p, stat.MoveSpeed)
	v := g.VitalOf(p)
	b := g.ensureBody(p, &v)
	b.Missing |= 1<<body.LegL | 1<<body.FootL
	g.bodyChanged(p, b, &v)
	if ms := g.statOf(p, stat.MoveSpeed); ms >= base || ms != base*500/1000 {
		t.Fatalf("one leg halves move speed: %d -> %d", base, ms)
	}
	b.Missing |= 1<<body.EyeL | 1<<body.EyeR
	g.bodyChanged(p, b, &v)
	if acc := g.statOf(p, stat.ShootAcc); acc != -sightAccPoints {
		t.Fatalf("blind shooting accuracy %d", acc)
	}
	fx := g.sk.stats[p]
	g.bodyChanged(p, b, &v)
	if g.sk.stats[p] != fx {
		t.Fatal("an unchanged body keeps its stat sheet")
	}
}

func TestUnconsciousGoesDownWithHysteresis(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	p, _ := g.SpawnNamedPawn(4, 4, "Ada")
	v := g.VitalOf(p)
	b := g.ensureBody(p, &v)
	b.Wounds = append(b.Wounds, body.Wound{Part: body.Brain, Dmg: 24, Kind: body.Bruise})
	g.bodyChanged(p, b, &v)
	g.hurt(p, 1, 0) // any further hit finds it out cold
	if v := g.VitalOf(p); v.Status != health.Downed || v.Injury >= health.DownedInjury {
		t.Fatalf("no consciousness downs a lightly hurt pawn: %+v", v)
	}
	// It stays down while consciousness is under consciousRecover, even
	// though its Injury is under RecoverInjury.
	changes, prev := 0, health.Downed
	for i := 0; i < 40; i++ {
		g.SetNeeds(p, 1<<20, 1<<20)
		g.Step(health.HealTicks)
		s := g.VitalOf(p).Status
		if s != prev {
			changes++
			prev = s
		}
		if s == health.Alive && !g.consciousEnough(p) {
			t.Fatalf("stood up with consciousness %d", g.bodies[p].Capacity(body.Consciousness))
		}
	}
	if changes > 1 {
		t.Fatalf("downed state flapped %d times", changes)
	}
}

func TestOldInjuryMigratesToATorsoBruise(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	p, _ := g.SpawnNamedPawn(4, 4, "Ada")
	v := g.VitalOf(p)
	v.Injury = 30 // an older save: injury and no body
	g.vital.Set(p, v)
	if hv, _ := g.Health(p); hv.Parts[body.Torso].Status == PartHealthy || g.bodies != nil {
		t.Fatalf("the view shows the migrated wound without making a body: %+v", hv.Parts[body.Torso])
	}
	h := saveLoad(t, g)
	h.hurt(p, 1, 0)
	b := h.bodies[p]
	if b == nil || h.VitalOf(p).Injury != 31 || b.Damage() != 31 {
		t.Fatalf("migrated body %+v, Injury %d", b, h.VitalOf(p).Injury)
	}
	if b.Wounds[0].Part != body.Torso || b.Wounds[0].Kind != body.Bruise || b.Wounds[0].Dmg != 30 {
		t.Fatalf("the old injury is a torso bruise: %+v", b.Wounds[0])
	}
}

func TestFixtureSavesLoadWithBodies(t *testing.T) {
	for _, path := range []string{"testdata/premotion_v1.sav", ng1V2Fixture} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		g, _, err := Load(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if len(g.pawnIDs) == 0 {
			t.Fatalf("%s: no pawns", path)
		}
		p := g.pawnIDs[0]
		v := g.VitalOf(p)
		v.Injury += 20
		g.vital.Set(p, v)
		g.Step(health.HealTicks * 2)
		if b := g.bodies[p]; b == nil || b.Damage() != g.VitalOf(p).Injury {
			t.Fatalf("%s: the old injury moved into a body (%+v vs %d)", path, b, g.VitalOf(p).Injury)
		}
	}
}

func TestBodiesSurviveSaveLoad(t *testing.T) {
	g := New(Config{Seed: 5, Width: 12, Height: 8})
	p, _ := g.SpawnNamedPawn(4, 4, "Ada")
	g.hurt(p, 25, 0)
	v := g.VitalOf(p)
	b := g.bodies[p]
	b.Missing |= 1<<body.LegL | 1<<body.FootL
	g.bodyChanged(p, b, &v)
	g.cmdAddOperation(p, int(body.InstallImplant), uint8(body.LegL), uint8(item.PegLeg))
	h := saveLoad(t, g)
	if !reflect.DeepEqual(g.exportBodies(), h.exportBodies()) || !reflect.DeepEqual(g.exportSurgery(), h.exportSurgery()) {
		t.Fatalf("bodies %+v / %+v, surgery %+v / %+v", g.exportBodies(), h.exportBodies(), g.exportSurgery(), h.exportSurgery())
	}
	if h.statOf(p, stat.MoveSpeed) != g.statOf(p, stat.MoveSpeed) {
		t.Fatal("a loaded body rebuilds its stat layer")
	}
	g.Step(health.HealTicks)
	h.Step(health.HealTicks)
	if !reflect.DeepEqual(g.exportBodies(), h.exportBodies()) || g.VitalOf(p) != h.VitalOf(p) {
		t.Fatal("a loaded body heals on as the unbroken game's does")
	}
}

func TestBodiesAreDeterministic(t *testing.T) {
	run := func() ([]entry[savedBody], []entity.EntityID) {
		g := New(Config{Seed: 77, Width: 16, Height: 10})
		a, _ := g.SpawnNamedPawn(4, 4, "Ada")
		b, _ := g.SpawnNamedPawn(8, 4, "Bo")
		for i := 0; i < 12; i++ {
			g.hurt(a, 3+i%5, b)
			g.hurt(b, 2+i%3, a)
			g.Step(97)
		}
		g.Step(5000)
		return g.exportBodies(), []entity.EntityID{a, b}
	}
	x, _ := run()
	y, _ := run()
	if !reflect.DeepEqual(x, y) {
		t.Fatal("same seed, same wounds")
	}
}

func TestInjurySetFromOutsideReconciles(t *testing.T) {
	g := New(Config{Seed: 2, Width: 12, Height: 8})
	p, _ := g.SpawnNamedPawn(4, 4, "Ada")
	g.hurt(p, 20, 0)
	v := g.VitalOf(p)
	v.Injury = 8 // a fixture lowers it: open wounds shrink to match
	b := g.ensureBody(p, &v)
	if b.Damage() != 8 {
		t.Fatalf("trimmed body damage %d, want 8", b.Damage())
	}
	v.Injury = 15 // and raises it: the difference is a torso bruise
	if b = g.ensureBody(p, &v); b.Damage() != 15 {
		t.Fatalf("raised body damage %d, want 15", b.Damage())
	}
}
