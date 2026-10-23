package animal

import (
	"testing"

	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
)

func TestRosterHas61(t *testing.T) {
	t.Parallel()
	all := All()
	if len(all) != 61 {
		t.Fatalf("roster has %d species, want 61", len(all))
	}
	seen := map[string]bool{}
	for _, s := range all {
		k := s.String()
		if k == "" || seen[k] {
			t.Fatalf("species %d key %q empty or duplicate", s, k)
		}
		seen[k] = true
		if got, ok := ParseSpecies(k); !ok || got != s {
			t.Fatalf("ParseSpecies(%q) = %v %v", k, got, ok)
		}
	}
	if Deer != 0 || TimberWolf != 1 {
		t.Fatal("deer and timber wolf keep the saved slots 0 and 1")
	}
}

func TestLegacyNamesAlias(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]Species{"dustbuck": Deer, "ridgewolf": TimberWolf, "deer": Deer, "timber_wolf": TimberWolf} {
		if got, ok := ParseSpecies(name); !ok || got != want {
			t.Errorf("ParseSpecies(%q) = %v %v, want %v", name, got, ok, want)
		}
	}
	if _, ok := ParseSpecies("muffalo"); ok {
		t.Error("unknown names do not parse")
	}
	if Dustbuck != Deer || Ridgewolf != TimberWolf {
		t.Error("legacy constants alias the new species")
	}
	if Species(200).String() != "animal" || RosterOf(Species(200)).Key != "deer" || DefOf(Species(200)).MaxHP != DefOf(Deer).MaxHP {
		t.Error("out-of-range species fall back safely")
	}
	if TimberWolf.Label() != "timber wolf" {
		t.Errorf("label %q", TimberWolf.Label())
	}
}

func TestEveryDefComplete(t *testing.T) {
	t.Parallel()
	for _, s := range All() {
		d, r := DefOf(s), RosterOf(s)
		if d.MaxHP <= 0 || d.StepTicks <= 0 || d.FleeStep <= 0 || d.FleeStep >= d.StepTicks {
			t.Errorf("%s: hp %d step %d flee %d", s, d.MaxHP, d.StepTicks, d.FleeStep)
		}
		if d.Damage <= 0 || d.Cooldown < 100 || d.Cooldown > 140 || d.Accuracy < 70 || d.Accuracy > 80 {
			t.Errorf("%s: damage %d cooldown %d accuracy %d", s, d.Damage, d.Cooldown, d.Accuracy)
		}
		if d.Meat <= 0 {
			t.Errorf("%s: no meat", s)
		}
		if r.Size > SizeXS && d.Leather <= 0 {
			t.Errorf("%s: size %d gives no leather", s, r.Size)
		}
		if d.Corpse != item.CorpseAnimal {
			t.Errorf("%s: corpse %v", s, d.Corpse)
		}
		if d.WanderRadius <= 0 {
			t.Errorf("%s: no wander radius", s)
		}
		if d.Predator && (d.HuntRadius <= 0 || d.HungerTicks == 0 || d.AggroRadius <= 0) {
			t.Errorf("%s: predator without hunt numbers %+v", s, d)
		}
		if r.Behaviour != Hive && r.Behaviour != Legendary && r.HerdMin <= 0 {
			t.Errorf("%s: no herd size", s)
		}
		if r.HerdMax < r.HerdMin {
			t.Errorf("%s: herd %d..%d", s, r.HerdMin, r.HerdMax)
		}
		if meat, leather := Yield(s); meat != d.Meat || leather != d.Leather {
			t.Errorf("%s: yield %d/%d", s, meat, leather)
		}
	}
}

func TestStatsFromClasses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s                                  Species
		hp, step, flee, meat, leather, dmg int
		attack                             weapon.Kind
	}{
		// M, normal pace: the size row as is; flee = round(16 * 0.6)
		{Deer, 55, 16, 10, 14, 5, 7, weapon.Kick},
		// S+ (+15%), fast: round(28*1.15)=32, round(14*1.15)-4=12, flee round(7.2)
		{Gazelle, 32, 12, 7, 7, 2, 5, weapon.Kick},
		// S, slow, with its shell: 28*1.5 = 42 hp, 14+6 = 20 step
		{Tortoise, 42, 20, 12, 6, 2, 4, weapon.Bite},
		// XS: no leather
		{Chicken, 12, 16, 10, 2, 0, 2, weapon.Peck},
		// XL legendary: hp x2.5
		{Glimmerstag, 425, 22, 13, 55, 16, 16, weapon.Gore},
		// L slow
		{Cow, 100, 24, 14, 30, 10, 11, weapon.Headbutt},
		// L- (-15%): round(100*0.85) = 85
		{DireWolf, 85, 15, 9, 26, 9, 9, weapon.Bite},
		{GrizzlyBear, 100, 18, 11, 30, 10, 11, weapon.Claw},
	}
	for _, c := range cases {
		d := DefOf(c.s)
		if d.MaxHP != c.hp || d.StepTicks != c.step || d.FleeStep != c.flee || d.Meat != c.meat ||
			d.Leather != c.leather || d.Damage != c.dmg || d.Attack != c.attack {
			t.Errorf("%s: got hp %d step %d flee %d meat %d leather %d dmg %d attack %v; want %+v",
				c.s, d.MaxHP, d.StepTicks, d.FleeStep, d.Meat, d.Leather, d.Damage, d.Attack, c)
		}
	}
	behaviours := []struct {
		s                     Species
		tame, revenge, fleeR  int
		skittish, predator    bool
		hive, legend, explode bool
	}{
		{Cow, 75, 5, 0, true, false, false, false, false},
		{Labrador, 70, 3, 0, true, false, false, false, false},
		{Caribou, 40, 10, 3, true, false, false, false, false},
		{Hare, 30, 0, 5, true, false, false, false, false},
		{Tortoise, 50, 5, 0, true, false, false, false, false},
		{Bison, 15, 50, 0, false, false, false, false, false},
		{TimberWolf, 15, 45, 0, false, true, false, false, false},
		{Cougar, 15, 45, 0, false, true, false, false, false},
		{HiveBrute, 0, 0, 0, false, false, true, false, false},
		{Glimmerstag, 5, 100, 0, false, false, false, true, false},
		{Emberbuck, 30, 10, 3, true, false, false, false, true},
		{CinderRat, 30, 0, 5, true, false, false, false, true},
	}
	for _, c := range behaviours {
		d := DefOf(c.s)
		if d.TameChance != c.tame || d.Revenge != c.revenge || d.FleeRadius != c.fleeR || d.Skittish != c.skittish ||
			d.Predator != c.predator || d.Hive != c.hive || d.Legendary != c.legend || d.Explosive != c.explode {
			t.Errorf("%s: got %+v; want %+v", c.s, d, c)
		}
	}
	w := DefOf(TimberWolf)
	if w.AggroRadius != 5 || w.HungerTicks != 90000 || w.HuntRadius != 22 || !w.Pack {
		t.Errorf("predator numbers %+v", w)
	}
	if DefOf(ArcticFox).Pack || !DefOf(ArcticFox).SmallPrey {
		t.Error("an arctic fox hunts small prey alone")
	}
	if DefOf(Cow).TameRevenge >= DefOf(TimberWolf).TameRevenge {
		t.Error("a botched taming angers a predator more than a cow")
	}
}

func TestSizeScaleRounds(t *testing.T) {
	t.Parallel()
	if scale(28, 15) != 32 || scale(14, 15) != 16 || scale(100, -15) != 85 || scale(0, 15) != 0 || scale(7, -15) != 6 {
		t.Fatal("scale rounds to nearest")
	}
}

func TestAttackModeAndBodyNames(t *testing.T) {
	t.Parallel()
	for k, want := range map[AttackKind]string{AttackBite: "bite", AttackClaw: "claw", AttackKick: "kick", AttackGore: "gore",
		AttackHeadbutt: "headbutt", AttackPeck: "peck", AttackKind(99): "bite"} {
		if k.String() != want {
			t.Errorf("attack %d = %q, want %q", k, k.String(), want)
		}
	}
	for m, want := range map[Mode]string{Wander: "wandering", Flee: "fleeing", Attack: "attacking"} {
		if m.String() != want {
			t.Errorf("mode %d = %q", m, m.String())
		}
	}
	if s := (State{Manhunter: 100}); !s.Enraged(99) || s.Enraged(100) || (State{}).Enraged(5) {
		t.Error("enraged until the Manhunter tick")
	}
	if BodyLength(Deer) != 0.95 || BodyLength(Gazelle) != 0.6*1.15 || BodyLength(Elephant) != 2.0 {
		t.Errorf("body lengths %v %v %v", BodyLength(Deer), BodyLength(Gazelle), BodyLength(Elephant))
	}
	if x, y := FleeGoal(5, 5, 5, 5, 4); x != 9 || y != 5 {
		t.Errorf("a threat on the same tile: flee east, got %d,%d", x, y)
	}
}

// TestXSBetweenOldAndS: XS animals read bigger than the old 0.35-tile
// class, yet the biggest XS (with its trim) stays below the smallest S
// (with its trim), so the size classes never overlap (PLAY-010).
func TestXSBetweenOldAndS(t *testing.T) {
	t.Parallel()
	maxXS, minS := 0.0, 99.0
	for _, s := range All() {
		l := BodyLength(s)
		switch RosterOf(s).Size {
		case SizeXS:
			if l <= 0.35 {
				t.Errorf("%s: XS body length %.3f, want above the old 0.35", s, l)
			}
			maxXS = max(maxXS, l)
		case SizeS:
			minS = min(minS, l)
		}
	}
	if maxXS >= minS {
		t.Fatalf("largest XS %.3f must stay below smallest S %.3f", maxXS, minS)
	}
}
