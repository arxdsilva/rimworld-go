package skill

import (
	"testing"

	"rimworld-go/internal/domain/ai/work"
)

func TestKeysLabelsParse(t *testing.T) {
	seen := map[string]bool{}
	for id := ID(0); id < NumSkills; id++ {
		k := id.Key()
		if k == "" || id.Label() == "" || seen[k] {
			t.Fatalf("skill %d key %q label %q", id, k, id.Label())
		}
		seen[k] = true
		if got, ok := Parse(k); !ok || got != id {
			t.Fatalf("Parse(%q) = %d,%v", k, got, ok)
		}
	}
	if NumSkills.Key() != "" || NumSkills.Label() != "" {
		t.Fatal("out of range skill has a name")
	}
	if _, ok := Parse("juggling"); ok {
		t.Fatal("unknown key parsed")
	}
}

func TestForWork(t *testing.T) {
	want := map[work.Type]ID{
		work.Hunt: Shooting, work.Construct: Construction, work.Mine: Mining,
		work.Cook: Cooking, work.Grow: Plants, work.PlantCut: Plants, work.Handle: Animals,
		work.Craft: Crafting, work.Doctor: Medicine, work.Research: Intellectual, work.Warden: Social,
	}
	for wt := work.Type(0); wt < work.NumTypes; wt++ {
		got, ok := ForWork(wt)
		w, has := want[wt]
		if ok != has || (ok && got != w) {
			t.Fatalf("ForWork(%d) = %d,%v want %d,%v", wt, got, ok, w, has)
		}
	}
}

func TestSpeedAndCost(t *testing.T) {
	if SpeedPermil(Neutral) != 1000 || SpeedPermil(0) != 580 || SpeedPermil(20) != 1780 || SpeedPermil(30) != 1780 {
		t.Fatal("speed curve")
	}
	if Cost(0) != 20000 || Cost(7) != 62000 || Cost(20) != 140000 {
		t.Fatal("cost curve")
	}
}

func TestLevelLabel(t *testing.T) {
	cases := map[uint8]string{0: "Novice", 3: "Novice", 4: "Capable", 7: "Capable", 8: "Skilled",
		11: "Skilled", 12: "Expert", 15: "Expert", 16: "Master", 20: "Master"}
	for l, w := range cases {
		if LevelLabel(l) != w {
			t.Fatalf("LevelLabel(%d) = %q want %q", l, LevelLabel(l), w)
		}
	}
	if NoPassion.Label() != "" || Minor.Label() != "Minor" || Major.Label() != "Major" {
		t.Fatal("passion labels")
	}
}

func TestLearnPassionMultipliers(t *testing.T) {
	for _, c := range []struct {
		p    Passion
		want int32
	}{{NoPassion, 350}, {Minor, 1000}, {Major, 1500}} {
		s := Skill{Level: 5, Passion: c.p}
		Learn(&s, 1000)
		if s.XP != c.want || s.Level != 5 {
			t.Fatalf("passion %d: xp %d want %d", c.p, s.XP, c.want)
		}
	}
	// Single units add up through the carry: 100 x 1 XP at 35% = 35 XP.
	s := Skill{}
	for range 100 {
		Learn(&s, 1)
	}
	if s.XP != 35 || s.Carry != 0 {
		t.Fatalf("carry: xp %d carry %d", s.XP, s.Carry)
	}
	Learn(&s, 0)
	Learn(&s, -50)
	if s.XP != 35 {
		t.Fatal("non-positive xp changed the skill")
	}
}

func TestLearnLevelsUpAndCaps(t *testing.T) {
	s := Skill{Level: 0, Passion: Minor}
	Learn(&s, int(Cost(0)+Cost(1)+10))
	if s.Level != 2 || s.XP != 10 {
		t.Fatalf("multi level-up: L%d xp %d", s.Level, s.XP)
	}
	s = Skill{Level: 19, Passion: Major}
	Learn(&s, 10_000_000)
	if s.Level != MaxLevel || s.XP != Cost(MaxLevel) {
		t.Fatalf("cap: L%d xp %d", s.Level, s.XP)
	}
}

func TestDecay(t *testing.T) {
	s := Skill{Level: 10, XP: 5}
	DecayDay(&s)
	if s != (Skill{Level: 10, XP: 5}) {
		t.Fatal("level 10 decayed")
	}
	s = Skill{Level: 12}
	DecayDay(&s)
	if s.XP != -1500 || s.Level != 12 {
		t.Fatalf("decay at 12: %+v", s)
	}
	// Falls only below -Cost(L-1)/2, then restarts at 75% of Cost(L-1).
	s = Skill{Level: 11, XP: -Cost(10)/2 + 1000}
	DecayDay(&s)
	if s.Level != 11 {
		t.Fatalf("dropped inside the band: %+v", s)
	}
	DecayDay(&s)
	if s.Level != 10 || s.XP != Cost(10)*3/4 {
		t.Fatalf("drop: %+v", s)
	}
}

// TestSkillXPAndDecayNoFlap: a pawn that alternates a day of learning with
// days of decay right at a level boundary must not bounce between levels.
func TestSkillXPAndDecayNoFlap(t *testing.T) {
	cases := []struct {
		name  string
		start Skill
		learn func(day int) int
	}{
		// Just under 11: each learning day crosses the boundary, each
		// decay day would cross it back without the band.
		{"cross every day", Skill{Level: 10, XP: Cost(10) - 300}, func(d int) int {
			if d%2 == 0 {
				return 700
			}
			return 0
		}},
		// Just levelled to 11: learning roughly matches decay on average.
		{"balanced", Skill{Level: 11}, func(d int) int {
			if d%2 == 0 {
				return 2000
			}
			return 0
		}},
		// Learning a little less than decay: drifts down at most once.
		{"slow fade", Skill{Level: 11, XP: 0}, func(d int) int {
			if d%2 == 0 {
				return 1200
			}
			return 0
		}},
		// High level, sparse learning bursts.
		{"bursts", Skill{Level: 16, XP: 100}, func(d int) int {
			if d%3 == 0 {
				return 9000
			}
			return 0
		}},
	}
	for _, c := range cases {
		s := c.start
		s.Passion = Minor
		changes, last := 0, s.Level
		for day := range 60 {
			Learn(&s, c.learn(day))
			DecayDay(&s)
			if s.Level != last {
				changes++
				last = s.Level
			}
		}
		if changes > 1 {
			t.Fatalf("%s: level changed %d times in 60 days (now %+v)", c.name, changes, s)
		}
	}
}

func TestNeutralSet(t *testing.T) {
	for i, s := range NeutralSet() {
		if s.Level != Neutral || s.XP != 0 || s.Passion != NoPassion {
			t.Fatalf("skill %d: %+v", i, s)
		}
		if SpeedPermil(s.Level) != 1000 {
			t.Fatal("neutral speed")
		}
	}
}
