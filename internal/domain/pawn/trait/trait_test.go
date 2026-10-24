package trait

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

func TestCatalogue(t *testing.T) {
	all := All()
	if len(all) != 40 {
		t.Fatalf("%d traits, want 40", len(all))
	}
	keys, labels := map[string]bool{}, map[string]bool{}
	for i, d := range all {
		if d.ID != ID(i+1) || d.Key == "" || d.Label == "" || d.Desc == "" || d.Effects() == "" {
			t.Fatalf("trait %d incomplete: %+v (effects %q)", i+1, d, d.Effects())
		}
		if keys[d.Key] || labels[d.Label] {
			t.Fatalf("duplicate %q", d.Key)
		}
		keys[d.Key], labels[d.Label] = true, true
		if got, ok := Parse(d.Key); !ok || got != d.ID {
			t.Fatalf("Parse(%q)", d.Key)
		}
		if DefOf(d.ID).Key != d.Key {
			t.Fatal("DefOf")
		}
	}
	if _, ok := Parse("nope"); ok {
		t.Fatal("parsed unknown key")
	}
	if DefOf(NumTraits).Key != "" || DefOf(None).Key != "" {
		t.Fatal("DefOf out of range")
	}
}

func TestConflicts(t *testing.T) {
	cases := []struct {
		a, b ID
		want bool
	}{
		{Tireless, Unhurried, true}, {Sunny, Brooding, true}, {Steady, Brittle, true},
		{FleetFooted, Plodding, true}, {HeartyEater, LightEater, true}, {Roughneck, Fussy, true},
		{DeliberateAim, QuickDraw, true}, {Gentle, DeliberateAim, true}, {QuickDraw, Gentle, true},
		{Sunny, Sunny, true}, {Sturdy, Sturdy, true},
		{Tireless, Sunny, false}, {GreenThumb, Bookish, false}, {Gentle, Sturdy, false},
		{None, Sunny, false}, {Sunny, NumTraits, false},
	}
	for _, c := range cases {
		if Conflicts(c.a, c.b) != c.want || Conflicts(c.b, c.a) != c.want {
			t.Fatalf("Conflicts(%d,%d) want %v", c.a, c.b, c.want)
		}
	}
}

func TestClashes(t *testing.T) {
	soldier := backstory.Def{Skills: map[skill.ID]int8{skill.Shooting: 4}}
	hurt := backstory.Def{Skills: map[skill.ID]int8{skill.Shooting: -2}}
	noPlants := backstory.Def{Incapable: backstory.Plants}
	pacifist := backstory.Def{Incapable: backstory.Violence}
	switch {
	case !Clashes(Gentle, soldier):
		t.Fatal("gentle soldier")
	case Clashes(Gentle, hurt):
		t.Fatal("a negative skill delta is no clash")
	case !Clashes(GreenThumb, noPlants):
		t.Fatal("green thumb without plants")
	case !Clashes(QuickDraw, pacifist):
		t.Fatal("quick draw pacifist")
	case Clashes(Gentle, pacifist), Clashes(Sunny, soldier), Clashes(None, soldier), Clashes(NumTraits, soldier):
		t.Fatal("false clash")
	}
}

func TestEffectsText(t *testing.T) {
	cases := map[ID][]string{
		Tireless:    {"Work speed +20%"},
		Unhurried:   {"Work speed -20%", "Recreation drain -15%"},
		Roughneck:   {"\"slept outside\" x0.25", "\"ate raw food\" x0.25"},
		Fussy:       {"x2", "\"slept in a nice room\" +3"},
		HeartyEater: {"Hunger rate +25%", "\"ate a cooked meal\" +2"},
		Bookish:     {"Intellectual speed +25%", "Intellectual learning x1.5"},
		Gentle:      {"Social +3 levels", "Incapable: Violence"},
		ColdHardy:   {"Cold comfort -6 °C"},
		Sturdy:      {"Pain tolerance +15%"},
		Steady:      {"Break threshold -6"},
	}
	for id, wants := range cases {
		e := DefOf(id).Effects()
		for _, w := range wants {
			if !strings.Contains(e, w) {
				t.Fatalf("%s effects %q lack %q", DefOf(id).Label, e, w)
			}
		}
	}
	if pct(125) != "1.25" || pct(5) != "0.05" || pct(300) != "3" {
		t.Fatal("pct")
	}
}

func TestTraitStatMods(t *testing.T) {
	s := stat.Build(stat.Base(), DefOf(Tireless).Mods, DefOf(FleetFooted).Mods, DefOf(Sunny).Mods)
	if s[stat.WorkSpeed] != 1200 || s[stat.MoveSpeed] != 1150 || s[stat.MoodOffset] != 8 {
		t.Fatalf("%v", s)
	}
}

// TestGenerated: legacy rules roll only the original 19 traits (old seeds
// keep their pawns); otherwise every trait, in ID order.
func TestGenerated(t *testing.T) {
	legacy, all := Generated(true), Generated(false)
	if len(legacy) != 19 || legacy[0] != Tireless || legacy[18] != Sturdy {
		t.Fatalf("legacy = %v", legacy)
	}
	if len(all) != int(NumTraits)-1 || all[len(all)-1] != NaturalPurist {
		t.Fatalf("all = %v", all)
	}
	for i, id := range all {
		if id != ID(i+1) {
			t.Fatalf("all[%d] = %d", i, id)
		}
	}
}

// TestSocialTraitGroups: the ENG-003 groups exclude each other within, and
// orientation traits are rare.
func TestSocialTraitGroups(t *testing.T) {
	for _, c := range [][2]ID{{Striking, Grotesque}, {Fair, Homely}, {Gay, Bisexual}, {Asexual, Gay},
		{Abstainer, Hedonist}, {Indulgent, Abstainer}, {AugmentFan, NaturalPurist}, {DistrustsMen, DistrustsWomen},
		{Idle, Tireless}, {Bloodthirsty, Gentle}} {
		if !Conflicts(c[0], c[1]) {
			t.Errorf("%s and %s should conflict", DefOf(c[0]).Label, DefOf(c[1]).Label)
		}
	}
	if Conflicts(Striking, Gay) || Conflicts(Gracious, ColdHearted) {
		t.Error("false conflict")
	}
	if DefOf(Gay).Rarity != 10 || DefOf(Bisexual).Rarity != 10 || DefOf(Asexual).Rarity != 20 || DefOf(Striking).Rarity != 0 {
		t.Error("orientation rarity")
	}
	if DefOf(Striking).Beauty != 2 || DefOf(Grotesque).Beauty != -2 {
		t.Error("beauty")
	}
	if e := DefOf(Idle).Effects(); !strings.Contains(e, "Work speed -40%") {
		t.Errorf("idle effects %q", e)
	}
	for _, d := range All() {
		if strings.Contains(d.Note, "{") {
			t.Errorf("%s: placeholder in note %q", d.Label, d.Note)
		}
	}
}
