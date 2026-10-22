package pawngen

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
)

const testSeed = 0xC0FFEE

// many generates n candidates over a spread of slots and rolls.
func many(n int, r Rules) []Pawn {
	out := make([]Pawn, n)
	for i := range out {
		out[i] = Generate(testSeed, i%10, i/10, r)
	}
	return out
}

func TestGenerateDeterministic(t *testing.T) {
	r := Rules{Taken: map[string]bool{"Ace": true}}
	for slot := range 8 {
		for roll := range 4 {
			a := Generate(42, slot, roll, r)
			b := Generate(42, slot, roll, r)
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("slot %d roll %d differs:\n%+v\n%+v", slot, roll, a, b)
			}
		}
	}
	if reflect.DeepEqual(Generate(42, 0, 0, r), Generate(42, 0, 1, r)) ||
		reflect.DeepEqual(Generate(42, 0, 0, r), Generate(42, 1, 0, r)) ||
		reflect.DeepEqual(Generate(42, 0, 0, r), Generate(43, 0, 0, r)) {
		t.Fatal("seed, slot and roll must all matter")
	}
	// Taken only moves the name, never the rest of the colonist.
	p := Generate(7, 3, 2, Rules{})
	q := Generate(7, 3, 2, Rules{Taken: map[string]bool{p.Nick: true}})
	if q.Nick == p.Nick {
		t.Fatal("taken nickname reused")
	}
	p.First, p.Nick, p.Last, q.First, q.Nick, q.Last = "", "", "", "", "", ""
	p.Temperament, q.Temperament = 0, 0
	if !reflect.DeepEqual(p, q) {
		t.Fatal("Taken changed more than the name")
	}
}

func TestDistributions(t *testing.T) {
	const n = 2000
	ps := many(n, Rules{})
	var female, thin, avg, heavy, teen, adultBad, cond, grey50 int
	var traitN [4]int
	ages := map[int]int{}
	for _, p := range ps {
		y := p.AgeYears()
		ages[y]++
		if y < 14 || y > 68 {
			t.Fatalf("age %d out of range", y)
		}
		if p.Look.Sex == look.Female {
			female++
		}
		switch p.Look.Build {
		case look.Thin:
			thin++
		case look.Average:
			avg++
		case look.Heavy:
			heavy++
		}
		if y < 18 {
			teen++
			if p.Adulthood != 0 {
				adultBad++
			}
		} else if p.Adulthood == 0 {
			adultBad++
		}
		if p.Childhood == 0 {
			t.Fatal("no childhood")
		}
		if len(p.Traits) < 1 || len(p.Traits) > 3 {
			t.Fatalf("%d traits", len(p.Traits))
		}
		traitN[len(p.Traits)]++
		if len(p.Conditions) > 0 {
			cond++
		}
		if p.Look.Grey && y < 50 {
			t.Fatalf("grey at %d", y)
		}
		if y >= 65 && p.Look.Grey {
			grey50++
		}
		if p.First == "" || p.Last == "" || p.Nick == "" {
			t.Fatalf("unnamed %+v", p)
		}
		if p.Look.Shirt == p.Look.Pants || p.Look.Bare {
			t.Fatalf("clothes %+v", p.Look)
		}
		if p.Look.Sex == look.Female && p.Look.Beard != 0 {
			t.Fatal("bearded woman generated")
		}
	}
	within := func(name string, got, wantPct, tolPct int) {
		t.Helper()
		pct := got * 100 / n
		if pct < wantPct-tolPct || pct > wantPct+tolPct {
			t.Fatalf("%s: %d%% want %d±%d%%", name, pct, wantPct, tolPct)
		}
	}
	within("female", female, 50, 3)
	within("thin", thin, 25, 5)
	within("average", avg, 50, 5)
	within("heavy", heavy, 25, 5)
	within("teens", teen, 8, 3)
	within("conditions", cond, 12, 3)
	within("one trait", traitN[1], 30, 5)
	within("two traits", traitN[2], 50, 5)
	within("three traits", traitN[3], 20, 5)
	if adultBad != 0 {
		t.Fatalf("%d pawns with the wrong backstory slots", adultBad)
	}
	// Triangular with mode 30: the late twenties/early thirties are the
	// most common decade slice, and the old tail is thin.
	band := func(lo, hi int) (c int) {
		for a := lo; a <= hi; a++ {
			c += ages[a]
		}
		return c
	}
	if b25, b45, b60 := band(25, 34), band(45, 54), band(60, 68); !(b25 > b45 && b45 > b60) {
		t.Fatalf("age shape 25-34:%d 45-54:%d 60-68:%d", b25, b45, b60)
	}
	if mid := band(28, 32); mid < band(18, 22) || mid < band(38, 42) {
		t.Fatal("mode is not near 30")
	}
}

func TestIncapableSkillsZeroNoPassion(t *testing.T) {
	ps := many(2000, Rules{Forced: []ForcedTrait{{Trait: trait.Gentle, Chance: 30}}})
	seen := 0
	for _, p := range ps {
		inc := p.Incapable()
		passions := 0
		for id := skill.ID(0); id < skill.NumSkills; id++ {
			s := p.Skills[id]
			if s.Level > skill.MaxLevel {
				t.Fatalf("level %d", s.Level)
			}
			if s.Passion != skill.NoPassion {
				passions++
			}
			if inc.DisablesSkill(id) {
				seen++
				if s.Level != 0 || s.XP != 0 || s.Passion != skill.NoPassion {
					t.Fatalf("incapable %s: %+v", id.Label(), s)
				}
			}
		}
		if passions == 0 || passions > 4 {
			t.Fatalf("%d passions", passions)
		}
	}
	if seen == 0 {
		t.Fatal("no incapable skills generated")
	}
}

func TestTraitConflictsNever(t *testing.T) {
	r := Rules{Forced: []ForcedTrait{{Trait: trait.Gentle, Chance: 50}, {Trait: trait.QuickDraw}, {Trait: trait.Tireless, Chance: 100}}}
	for _, rules := range []Rules{{}, r} {
		for _, p := range many(2000, rules) {
			for i, a := range p.Traits {
				for _, b := range p.Traits[i+1:] {
					if trait.Conflicts(a, b) {
						t.Fatalf("conflicting traits %v", p.Traits)
					}
				}
				for _, b := range []backstory.ID{p.Childhood, p.Adulthood} {
					if b != 0 && trait.Clashes(a, backstory.DefOf(b)) {
						t.Fatalf("%s clashes with %q", trait.DefOf(a).Label, backstory.DefOf(b).Title)
					}
				}
			}
		}
	}
}

func TestForcedRules(t *testing.T) {
	r := Rules{
		MinAge: 40, MaxAge: 45,
		Forced:     []ForcedTrait{{Trait: trait.Sunny}, {Trait: trait.Brooding}, {Trait: trait.Gentle, Chance: 100}},
		ForcedCond: Exhausted,
		NoClothes:  true,
	}
	for _, p := range many(300, r) {
		if y := p.AgeYears(); y < 40 || y > 45 {
			t.Fatalf("age %d", y)
		}
		if !p.Has(trait.Sunny) || p.Has(trait.Brooding) || !p.Has(trait.Gentle) {
			t.Fatalf("forced traits %v", p.Traits)
		}
		if p.Temperament != SocialType {
			t.Fatal("sunny pawns are social")
		}
		if len(p.Conditions) != 1 || p.Conditions[0] != Exhausted {
			t.Fatalf("conditions %v", p.Conditions)
		}
		if !p.Look.Bare || p.Look.Vest {
			t.Fatal("no-clothes rule ignored")
		}
		if p.Incapable()&backstory.Violence == 0 || p.Skills[skill.Shooting].Level != 0 {
			t.Fatal("gentle pawns can't fight")
		}
	}
	// Teens only; and a chance-limited forced condition.
	teens := many(300, Rules{MaxAge: 16, ForcedCond: Nearsighted, CondChance: 50})
	near := 0
	for _, p := range teens {
		if y := p.AgeYears(); y < 14 || y > 16 || p.Adulthood != 0 {
			t.Fatalf("teen rule: age %d adulthood %d", y, p.Adulthood)
		}
		if len(p.Conditions) == 1 && p.Conditions[0] == Nearsighted {
			near++
		}
	}
	if near < 100 || near > 220 {
		t.Fatalf("nearsighted %d/300 at 50%%", near)
	}
	for _, p := range many(200, Rules{Forced: []ForcedTrait{{Trait: trait.Brooding}}}) {
		if p.Temperament != Homebody {
			t.Fatal("brooding pawns are homebodies")
		}
	}
}

func TestBackstoryFitsTech(t *testing.T) {
	for _, p := range many(1000, Rules{Tech: backstory.TagNeolithic}) {
		for _, b := range []backstory.ID{p.Childhood, p.Adulthood} {
			if b != 0 && !backstory.DefOf(b).Fits(backstory.TagNeolithic) {
				t.Fatalf("clan colonist with %q", backstory.DefOf(b).Title)
			}
		}
	}
	for _, tech := range []backstory.Tag{0, backstory.TagIndustrial, backstory.TagSpacer} {
		modern := tech
		if modern == 0 {
			modern = backstory.TagIndustrial | backstory.TagSpacer
		}
		picks, neoOnly := 0, 0
		for _, p := range many(2000, Rules{Tech: tech}) {
			for _, b := range []backstory.ID{p.Childhood, p.Adulthood} {
				if b == 0 {
					continue
				}
				d := backstory.DefOf(b)
				picks++
				if !d.Fits(modern) {
					if !d.Fits(backstory.TagNeolithic) {
						t.Fatalf("tech %b: %q fits nothing allowed", tech, d.Title)
					}
					neoOnly++
				}
			}
		}
		if pct := neoOnly * 100 / picks; neoOnly == 0 || pct > 12 {
			t.Fatalf("tech %b: %d%% clan-only backstories, want a few (<=10%%)", tech, pct)
		}
	}
}

func TestLegacyTemperamentMatchesGame(t *testing.T) {
	// Golden values from the game's temperamentOf (idle.go) before NG-1:
	// 0 wanderer, 1 social, 2 homebody.
	golden := map[string]Temperament{
		"Ada": 2, "Bo": 0, "Cy": 1, "Dee": 1, "Eli": 0, "Fay": 1,
		"Gus": 2, "Hal": 1, "Ivy": 2, "Jo": 0, "": 1, "Marta Quell": 2,
	}
	for name, want := range golden {
		if got := Legacy(1, name, 5, 5, 6).Temperament; got != want {
			t.Fatalf("%q: temperament %d want %d", name, got, want)
		}
	}
	if Wanderer != 0 || SocialType != 1 || Homebody != 2 {
		t.Fatal("temperament order must match the game's")
	}
}

func TestLegacyPawnIsNeutral(t *testing.T) {
	p := Legacy(17, "Rook", 5, 5, 6)
	if p.Skills != skill.NeutralSet() || len(p.Traits) != 0 || p.Incapable() != 0 ||
		p.AgeYears() != 30 || p.Look != look.Legacy(17, 5, 5, 6) || p.Childhood != 0 || p.Adulthood != 0 {
		t.Fatalf("legacy pawn %+v", p)
	}
	if Stats(p) != stat.Base() {
		t.Fatalf("legacy stats %v", Stats(p))
	}
	if p.FullName() != "Rook" || p.Title() != "" {
		t.Fatalf("legacy names %q %q", p.FullName(), p.Title())
	}
	for id := skill.ID(0); id < skill.NumSkills; id++ {
		if SkillWorkPct(p, id) != 100 || LearnPct(p, id) != 100 {
			t.Fatal("legacy multipliers")
		}
	}
}

func TestStatsLayers(t *testing.T) {
	p := Pawn{
		Look:       look.Look{Build: look.Thin},
		AgeDays:    70 * DaysPerYear,
		Conditions: []Condition{OldFracture, HandTremor},
		Traits:     []trait.ID{trait.Tireless, trait.Bookish},
	}
	s := Stats(p)
	// body x1.03 -> age x0.9 -> fracture x0.92
	if s[stat.MoveSpeed] != 1000*1030/1000*900/1000*920/1000 {
		t.Fatalf("move %d", s[stat.MoveSpeed])
	}
	// age x0.85 -> tireless x1.2
	if s[stat.WorkSpeed] != 1020 || s[stat.HungerRate] != 900 || s[stat.ColdComfort] != 20 || s[stat.MeleeAcc] != -5 {
		t.Fatalf("sheet %v", s)
	}
	if SkillWorkPct(p, skill.Crafting) != 90 || SkillWorkPct(p, skill.Intellectual) != 125 || LearnPct(p, skill.Intellectual) != 150 {
		t.Fatal("skill multipliers")
	}
	young := Stats(Pawn{AgeDays: 15 * DaysPerYear, Look: look.Look{Build: look.Heavy}})
	if young[stat.LearnRate] != 1250 || young[stat.WorkSpeed] != 850 || young[stat.HungerRate] != 1150 {
		t.Fatalf("young %v", young)
	}
	mid := Stats(Pawn{AgeDays: 55 * DaysPerYear})
	if mid[stat.WorkSpeed] != 950 || mid[stat.MoveSpeed] != 950 {
		t.Fatalf("mid %v", mid)
	}
	for c := NoCondition + 1; c < numConditions; c++ {
		if c.Label() == "" || c.Desc() == "" {
			t.Fatalf("condition %d unnamed", c)
		}
	}
	if numConditions.Label() != "" || numConditions.Desc() != "" || numConditions.Mods() != nil {
		t.Fatal("out of range condition")
	}
}

func TestNamesAndTitles(t *testing.T) {
	p := Pawn{First: "Ada", Nick: "Pip", Last: "Okafor"}
	if p.FullName() != "Ada 'Pip' Okafor" {
		t.Fatal(p.FullName())
	}
	p.Nick = "Ada"
	if p.FullName() != "Ada Okafor" {
		t.Fatal(p.FullName())
	}
	kid := Generate(1, 0, 0, Rules{MaxAge: 15})
	if kid.Title() != backstory.DefOf(kid.Childhood).Title || kid.Title() == "" {
		t.Fatal("teen title")
	}
	// Everything taken: the full name is the fallback, then a number.
	taken := map[string]bool{}
	for _, n := range append(append(append([]string(nil), femaleNames...), maleNames...), nicknames...) {
		taken[n] = true
	}
	a := Generate(5, 0, 0, Rules{Taken: taken})
	if a.Nick != a.First+" "+a.Last {
		t.Fatalf("fallback %q", a.Nick)
	}
	taken[a.Nick] = true
	b := Generate(5, 0, 0, Rules{Taken: taken})
	if b.Nick != a.Nick+" 2" {
		t.Fatalf("numbered fallback %q", b.Nick)
	}
	if itoa(0) != "0" || itoa(123) != "123" {
		t.Fatal("itoa")
	}
}

func TestNamePools(t *testing.T) {
	pools := map[string]struct {
		names []string
		min   int
	}{
		"female": {femaleNames, 90}, "male": {maleNames, 90},
		"surnames": {surnames, 110}, "nicknames": {nicknames, 50},
	}
	for label, p := range pools {
		if len(p.names) < p.min {
			t.Fatalf("%s: %d names, want >= %d", label, len(p.names), p.min)
		}
		seen := map[string]bool{}
		for _, n := range p.names {
			if n == "" || seen[n] || strings.TrimSpace(n) != n {
				t.Fatalf("%s: bad or duplicate %q", label, n)
			}
			seen[n] = true
		}
	}
	// A nickname must not double as somebody's first name.
	first := map[string]bool{}
	for _, n := range append(append([]string(nil), femaleNames...), maleNames...) {
		first[n] = true
	}
	for _, n := range nicknames {
		if first[n] {
			t.Fatalf("nickname %q is also a first name", n)
		}
	}
}

func TestNamesAvoidBlocklist(t *testing.T) {
	block := Blocklist()
	if len(block) == 0 {
		t.Fatal("empty blocklist")
	}
	var words []*regexp.Regexp
	for _, b := range block {
		words = append(words, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(b)+`\b`))
	}
	check := func(where, text string) {
		t.Helper()
		for i, w := range words {
			if w.MatchString(text) {
				t.Fatalf("%s %q uses blocklisted %q", where, text, block[i])
			}
		}
	}
	for _, pool := range [][]string{femaleNames, maleNames, surnames, nicknames} {
		for _, n := range pool {
			check("name", n)
		}
	}
	for _, d := range backstory.All() {
		check("backstory title", d.Title)
		check("backstory short", d.Short)
		check("backstory text", d.Desc)
	}
	for _, d := range trait.All() {
		check("trait", d.Label)
		check("trait text", d.Desc)
	}
	for c := NoCondition + 1; c < numConditions; c++ {
		check("condition", c.Label()+" "+c.Desc())
	}
}
