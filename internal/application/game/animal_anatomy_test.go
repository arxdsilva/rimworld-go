package game

import (
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// MED-13: animals have their species' anatomy.

// A body saved before MED-13 (the one-part adapter: humanlike, no Scale,
// every wound on the torso slot) loads as the species' anatomy with its
// wounds and infection on the root part; it then saves and loads as is.
func TestLegacyAnimalBodyMigrates(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 51, Width: 20, Height: 12})
	cow, _ := g.SpawnAnimal("cow", 10, 6)
	snake, _ := g.SpawnAnimal("cobra", 4, 4)
	legacy := func(dmg int) *body.Body {
		return &body.Body{
			Wounds:    []body.Wound{{Part: body.Torso, Dmg: dmg, Kind: body.Bite, At: 5, Size: int32(dmg)}},
			Conds:     []body.Condition{{Kind: body.CondInfection, Part: body.Torso, Sev: 200}},
			BloodLoss: 120, BloodStage: 1,
		}
	}
	setAnimal(g, cow, func(a *animal.State) { a.Injury, a.Body = 30, legacy(30) })
	setAnimal(g, snake, func(a *animal.State) { a.Injury, a.Body = 4, legacy(4) })
	l := saveLoad(t, g)
	for _, c := range []struct {
		id  entity.EntityID
		sp  animal.Species
		an  body.Anatomy
		dmg int
	}{{cow, animal.Cow, body.Quadruped, 30}, {snake, animal.Cobra, body.Snake, 4}} {
		b := animalBody(l, c.id)
		root := body.RootOf(c.an)
		if b == nil || b.Anatomy != c.an || b.Scale != animalHealthScale(c.sp) {
			t.Fatalf("%v: migrated body %+v", c.sp, b)
		}
		if len(b.Wounds) != 1 || b.Wounds[0].Part != root || b.Wounds[0].Dmg != c.dmg || b.Wounds[0].Kind != body.Bite {
			t.Fatalf("%v: wounds %+v, want one on the %s", c.sp, b.Wounds, b.PartLabel(root))
		}
		if len(b.Conds) != 1 || b.Conds[0].Part != root || b.BloodLoss != 120 || b.BloodStage != 1 {
			t.Fatalf("%v: conditions %+v, blood %d/%d", c.sp, b.Conds, b.BloodLoss, b.BloodStage)
		}
		if b.Missing != 0 || b.Fresh != 0 || b.VitalLost() {
			t.Fatalf("%v: nothing is lost by the migration: %+v", c.sp, b)
		}
	}
	if !slices.Contains(l.animalPatientIDs(), cow) || !slices.Contains(l.animalPatientIDs(), snake) {
		t.Fatal("the migrated animals stay patients")
	}
	again := saveLoad(t, l)
	if !reflect.DeepEqual(animalBody(again, cow), animalBody(l, cow)) || again.StateHash() != l.StateHash() {
		t.Fatal("a migrated body loads as it saved")
	}
}

// An animal with a wound on a leg and its tail lost loads back exactly:
// same parts, same moving, same hash, and both games step alike.
func TestAnimalPartWoundSurvivesSave(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 52, Width: 20, Height: 12})
	wolf, _ := g.SpawnAnimal("timber_wolf", 10, 6)
	setAnimal(g, wolf, func(a *animal.State) {
		b := newAnimalBody(a.Species)
		b.Wounds = []body.Wound{{Part: body.QLegFL, Dmg: 6, Kind: body.Bite, At: 1, Size: 6}}
		b.Missing = 1 << body.QTail
		a.Injury, a.Body = 6+b.PartDef(body.QTail).MaxHP, b
	})
	g.markAnimalPatient(wolf)
	toNextHour(g)
	before := *animalBody(g, wolf)
	l := saveLoad(t, g)
	got := animalBody(l, wolf)
	if got == nil || !reflect.DeepEqual(*got, before) || got.Anatomy != body.Quadruped {
		t.Fatalf("loaded %+v, want %+v", got, before)
	}
	if got.Capacity(body.Moving) != before.Capacity(body.Moving) || got.Capacity(body.Moving) >= 1000 {
		t.Fatalf("moving %d, want %d (below full)", got.Capacity(body.Moving), before.Capacity(body.Moving))
	}
	if l.StateHash() != g.StateHash() {
		t.Fatal("same state, same hash after a load")
	}
	toNextHour(g)
	toNextHour(l)
	if !reflect.DeepEqual(animalBody(l, wolf), animalBody(g, wolf)) || l.StateHash() != g.StateHash() {
		t.Fatal("the loaded game steps the wolf as the original")
	}
}

// Hits land on parts by coverage: across many hits on every plan a part
// may be lost, but never one whose loss kills, and the open wounds never
// outgrow Injury.
func TestAnimalHitsLandOnParts(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 53, Width: 40, Height: 20})
	species := []string{"bison", "chicken", "cobra", "tortoise", "hive_beetle", "monkey"}
	lost, parts := 0, map[body.Anatomy]map[body.Part]bool{}
	for i, key := range species {
		for j := 0; j < 6; j++ {
			id, err := g.SpawnAnimal(key, 2+j*5, 2+i*3)
			if err != nil {
				t.Fatal(err)
			}
			sp := animalOf(g, id).Species
			hp := animal.DefOf(sp).MaxHP
			for k := 0; k < 6; k++ {
				g.hurtAs(id, max(hp/10, 1), 0, body.Cut, health.CauseInjury)
				a, ok := g.animals.Get(id)
				if !ok || a.Body == nil {
					break
				}
				b := a.Body
				if b.Anatomy != animal.AnatomyOf(sp) || b.VitalLost() {
					t.Fatalf("%s: body %+v", key, b)
				}
				if woundDmg(b) > a.Injury {
					t.Fatalf("%s: wounds %d over injury %d", key, woundDmg(b), a.Injury)
				}
				if parts[b.Anatomy] == nil {
					parts[b.Anatomy] = map[body.Part]bool{}
				}
				for _, w := range b.Wounds {
					if int(w.Part) >= b.PartCount() {
						t.Fatalf("%s: wound on part %d of %d", key, w.Part, b.PartCount())
					}
					parts[b.Anatomy][w.Part] = true
				}
				if b.Missing != 0 {
					lost++
				}
			}
		}
	}
	for an, ps := range parts {
		if len(ps) < 3 {
			t.Fatalf("%v: hits landed on only %d parts", an, len(ps))
		}
	}
	if lost == 0 {
		t.Fatal("over that many hits some part is lost")
	}
}

// A lost part keeps the body once the wounds are healed (the leg stays
// lost), but the animal leaves the hourly patients.
func TestAnimalLostPartKeepsBody(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 54, Width: 20, Height: 12})
	deer, _ := g.SpawnAnimal("deer", 10, 6)
	setAnimal(g, deer, func(a *animal.State) {
		b := newAnimalBody(a.Species)
		b.Missing = 1<<body.QLegHL | 1<<body.QPawHL
		b.Wounds = []body.Wound{{Part: body.QTorso, Dmg: 1, Kind: body.Bruise, At: 1, Size: 1}}
		a.Injury, a.Body = 1, b
	})
	g.markAnimalPatient(deer)
	for h := 0; h < 8 && len(animalBody(g, deer).Wounds) > 0; h++ {
		toNextHour(g)
	}
	toNextHour(g)
	b := animalBody(g, deer)
	if b == nil || len(b.Wounds) != 0 || !b.PartMissing(body.QLegHL) {
		t.Fatalf("healed, the leg stays lost: %+v", b)
	}
	if slices.Contains(g.animalPatientIDs(), deer) {
		t.Fatal("nothing to treat: not a patient")
	}
	av := animalViewOf(t, g, deer)
	if av.MovingPct != 75 || len(av.Parts) != 1 || av.Parts[0].Label != "hind left leg" || !av.Parts[0].Missing {
		t.Fatalf("view: moving %d, parts %+v", av.MovingPct, av.Parts)
	}
	if a := animalOf(g, deer); a.Step() <= animal.DefOf(animal.Deer).StepTicks {
		t.Fatalf("a three-legged deer steps slower: %d", a.Step())
	}
}

// The view lists hurt and lost parts (a lost leg's paw goes with it) and
// the bleeding ones; a whole animal lists none.
func TestAnimalViewListsParts(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 55, Width: 20, Height: 12})
	dog, _ := g.SpawnAnimal("labrador", 10, 6)
	if av := animalViewOf(t, g, dog); av.Parts != nil || av.MovingPct != 100 {
		t.Fatalf("whole: %+v %d", av.Parts, av.MovingPct)
	}
	setAnimal(g, dog, func(a *animal.State) {
		b := newAnimalBody(a.Species)
		b.Missing = 1<<body.QLegFR | 1<<body.QPawFR
		b.Fresh = 1 << body.QLegFR
		b.Wounds = []body.Wound{{Part: body.QEarL, Dmg: 2, Kind: body.Bruise, At: 1}, {Part: body.QTorso, Dmg: 3, Kind: body.Cut, At: 1}}
		a.Injury, a.Body = 20, b
	})
	av := animalViewOf(t, g, dog)
	labels := []string{}
	for _, p := range av.Parts {
		labels = append(labels, p.Label)
	}
	if !slices.Equal(labels, []string{"left ear", "torso", "front right leg"}) {
		t.Fatalf("parts %q", labels)
	}
	ear, torso, leg := av.Parts[0], av.Parts[1], av.Parts[2]
	if ear.Bleeding || ear.HP != ear.MaxHP-2 || !torso.Bleeding || !leg.Missing || !leg.Bleeding {
		t.Fatalf("rows %+v", av.Parts)
	}
	if av.MovingPct > 75 || av.MovingPct <= 0 {
		t.Fatalf("three legs and pain: moving %d", av.MovingPct)
	}
}

// Surgery stays humanlike: no operation can be queued on an animal.
func TestAnimalSurgeryRefused(t *testing.T) {
	t.Parallel()
	g, _, dog := bondYard(t, 56)
	setAnimal(g, dog, func(a *animal.State) {
		a.Body = newAnimalBody(a.Species)
		a.Body.Missing = 1<<body.QLegFL | 1<<body.QPawFL
	})
	for k := body.OpKind(0); k < body.NumOps; k++ {
		g.cmdAddOperation(dog, int(k), uint8(body.QLegFL), 0)
	}
	if ops := g.operationsOf(dog); len(ops) != 0 {
		t.Fatalf("operations queued on an animal: %+v", ops)
	}
	if b := animalBody(g, dog); b.CanOperate(body.Operation{Kind: body.InstallImplant, Part: body.QLegFL}) || len(b.InstallOptions(0)) != 0 {
		t.Fatal("the animal body refuses implants")
	}
}
