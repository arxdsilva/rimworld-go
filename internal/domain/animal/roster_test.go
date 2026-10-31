package animal

import (
	"testing"

	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
)

func TestRosterHas65(t *testing.T) {
	t.Parallel()
	all := All()
	if len(all) != 65 { // wave E: +4 mechanoids
		t.Fatalf("roster has %d species, want 65", len(all))
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
		// DEF-4: a machine's attack and yields are its wiki row (TestMechDefs)
		if !d.Mech && (d.Damage <= 0 || d.Cooldown < 100 || d.Cooldown > 140 || d.Accuracy < 70 || d.Accuracy > 80) {
			t.Errorf("%s: damage %d cooldown %d accuracy %d", s, d.Damage, d.Cooldown, d.Accuracy)
		}
		if !d.Mech && d.Meat <= 0 {
			t.Errorf("%s: no meat", s)
		}
		if !d.Mech && r.Size > SizeXS && d.Leather <= 0 && r.Behaviour != Hive {
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
		if r.Behaviour != Hive && r.Behaviour != Legendary && r.Behaviour != Mechanoid && r.HerdMin <= 0 {
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
		// M, normal pace: the size row as is; flee = round(16 * 0.6).
		// FOOD-3: meat and leather are the wiki's (140 and 40 per unit of
		// body size, small bodies through the curve): deer 1.2.
		{Deer, 55, 16, 10, 168, 48, 7, weapon.Kick},
		// S+ (+15%), fast: round(28*1.15)=32, round(14*1.15)-4=12, flee round(7.2)
		{Gazelle, 32, 12, 7, 98, 31, 5, weapon.Kick},
		// S, slow, with its shell: 28*1.5 = 42 hp, 14+12 = 26 step, flee round(15.6)
		// (PLAY-022: the slow class widened from +6 to +12 so a tortoise is clearly slow)
		{Tortoise, 42, 26, 16, 70, 25, 4, weapon.Bite},
		// wiki: a chicken gives no leather
		{Chicken, 12, 16, 10, 42, 0, 2, weapon.Peck},
		// XL legendary: hp x2.5
		{Glimmerstag, 425, 22, 13, 560, 160, 16, weapon.Gore},
		// L slow: 18+12 = 30 step, flee 18 (PLAY-022: the slow class widened from +6 to +12)
		{Cow, 100, 30, 18, 336, 96, 11, weapon.Headbutt},
		// L- (-15%): round(100*0.85) = 85
		{DireWolf, 85, 15, 9, 196, 56, 9, weapon.Bite},
		{GrizzlyBear, 100, 18, 11, 301, 86, 11, weapon.Claw},
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
		// FOOD-3: revenge is the species' wiki hunting chance
		{Cow, 75, 0, 0, true, false, false, false, false},
		{Labrador, 70, 0, 0, true, false, false, false, false},
		{Caribou, 40, 10, 3, true, false, false, false, false},
		{Hare, 30, 0, 5, true, false, false, false, false},
		{Tortoise, 50, 0, 0, true, false, false, false, false},
		{Bison, 15, 10, 0, false, false, false, false, false},
		{TimberWolf, 15, 100, 0, false, true, false, false, false},
		{Cougar, 15, 50, 0, false, true, false, false, false},
		{HiveBrute, 0, 50, 0, false, false, true, false, false},
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
	// PLAY-022 (sizes are areas now): M 1.8, S+ 1.65*1.15, XL 3.0 tiles
	// (they were 0.95, 0.6*1.15 and 2.0 when sizes were lengths)
	if BodyLength(Deer) != 1.8 || BodyLength(Gazelle) != 1.65*1.15 || BodyLength(Elephant) != 3.0 {
		t.Errorf("body lengths %v %v %v", BodyLength(Deer), BodyLength(Gazelle), BodyLength(Elephant))
	}
	if x, y := FleeGoal(5, 5, 5, 5, 4); x != 9 || y != 5 {
		t.Errorf("a threat on the same tile: flee east, got %d,%d", x, y)
	}
}

// TestSizeClassBasesIncrease: the class base lengths strictly increase
// (PLAY-022: they are length targets that reach the AREA targets XS 1,
// S 1.5, M 2, L 4, XL 6 tiles^2; was the old 0.44/0.6/0.95/1.4/2.0), and
// every species' trim stays within +/-15% of its class.
func TestSizeClassBasesIncrease(t *testing.T) {
	t.Parallel()
	for c := SizeS; c <= SizeXL; c++ {
		if sizeLength[c] <= sizeLength[c-1] {
			t.Errorf("class %d base %.2f must exceed class %d base %.2f", c, sizeLength[c], c-1, sizeLength[c-1])
		}
	}
	if sizeLength[SizeXS] <= 0.35 {
		t.Errorf("XS base %.2f: a rat is drawn about a tile long, far above the old 0.35", sizeLength[SizeXS])
	}
	for _, s := range All() {
		if tr := RosterOf(s).SizeTrim; tr < -15 || tr > 15 {
			t.Errorf("%s: size trim %d%% outside +/-15%%", s, tr)
		}
	}
}

// TestAreasNotLengthsOrderClasses: the classes are ordered by drawn AREA
// (a rat is long and thin, a hare tall), so the longest XS (XS+, a hive
// beetle) may be longer than the shortest S (S-, a hare). It was the
// reverse while sizes were lengths (TestXSBetweenOldAndS, PLAY-010).
func TestAreasNotLengthsOrderClasses(t *testing.T) {
	t.Parallel()
	xsPlus, sMinus := sizeLength[SizeXS]*1.15, sizeLength[SizeS]*0.85
	if xsPlus <= sMinus {
		t.Fatalf("XS+ %.3f should exceed S- %.3f: the class areas, not lengths, order them", xsPlus, sMinus)
	}
	maxXS, minS := 0.0, 99.0
	for _, s := range All() {
		switch l := BodyLength(s); RosterOf(s).Size {
		case SizeXS:
			maxXS = max(maxXS, l)
		case SizeS:
			minS = min(minS, l)
		}
	}
	if maxXS <= minS {
		t.Errorf("the roster has an XS+ (%.3f) longer than an S- (%.3f), as the table allows", maxXS, minS)
	}
}
