package backstory

import (
	"regexp"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/skill"
)

func TestWorkMaskTable(t *testing.T) {
	cases := map[Bits][]work.Type{
		Violence:     {work.Hunt},
		Caring:       {work.Doctor},
		DumbLabor:    {work.Haul, work.Clean},
		Intellectual: {work.Research},
		Animals:      {work.Handle},
		Cooking:      {work.Cook},
		Plants:       {work.Grow, work.PlantCut},
		Mining:       {work.Mine},
		Crafting:     {work.Craft},
		Construction: {work.Construct},
	}
	for b, types := range cases {
		var want work.Mask
		for _, wt := range types {
			want = want.Set(wt, true)
		}
		if got := b.WorkMask(); got != want {
			t.Fatalf("bit %b: mask %b want %b", b, got, want)
		}
	}
	if Bits(0).WorkMask() != 0 || (Violence|Caring).WorkMask() != Violence.WorkMask()|Caring.WorkMask() {
		t.Fatal("mask union")
	}
	if AllBits.WorkMask().Enabled(work.Firefight) {
		t.Fatal("firefighting is never disabled")
	}
}

func TestBitsSkillsAndLabels(t *testing.T) {
	got := (Violence | Caring | DumbLabor).Skills()
	if len(got) != 3 || got[0] != skill.Shooting || got[1] != skill.Melee || got[2] != skill.Medicine {
		t.Fatalf("skills %v", got)
	}
	if len(DumbLabor.Skills()) != 0 || AllBits.DisablesSkill(skill.Social) {
		t.Fatal("dumb labor and social have no skill bit")
	}
	l := (Violence | Cooking).Labels()
	if len(l) != 2 || l[0] != "Violence" || l[1] != "Cooking" {
		t.Fatalf("labels %v", l)
	}
	if len(AllBits.Labels()) != numBits {
		t.Fatal("every bit needs a label")
	}
}

func TestFill(t *testing.T) {
	d := "{name} said {they} lost {their} hat. {They} asked us to help {them}. {Their} hat. {Them}?"
	cases := map[look.Sex]string{
		look.Male:   "Ivo said he lost his hat. He asked us to help him. His hat. Him?",
		look.Female: "Ivo said she lost her hat. She asked us to help her. Her hat. Her?",
		look.Sex(9): "Ivo said they lost their hat. They asked us to help them. Their hat. Them?",
	}
	for s, want := range cases {
		if got := Fill(d, s, "Ivo"); got != want {
			t.Fatalf("sex %d: %q", s, got)
		}
	}
	if capital("") != "" {
		t.Fatal("capital of empty")
	}
}

// agreement catches "{they} was" style text that only reads right for one
// pronoun.
var agreement = regexp.MustCompile(`\{[Tt]hey\} (is|are|was|were|has|have|does|do|[a-z]+s)\b`)

var placeholder = regexp.MustCompile(`\{[^}]*\}`)

func TestBackstoryTextUniqueNonEmpty(t *testing.T) {
	all := All()
	titles := map[string]bool{}
	descs := map[string]bool{}
	var child, adult, neoChild, neoAdult int
	for i, d := range all {
		if d.ID != ID(i+1) || DefOf(d.ID).Title != d.Title {
			t.Fatalf("%q: id %d", d.Title, d.ID)
		}
		if d.Title == "" || d.Short == "" || strings.TrimSpace(d.Desc) == "" {
			t.Fatalf("entry %d incomplete", d.ID)
		}
		key := strings.ToLower(d.Title)
		if titles[key] || descs[d.Desc] {
			t.Fatalf("duplicate title or text: %q", d.Title)
		}
		titles[key], descs[d.Desc] = true, true
		if !strings.Contains(d.Desc, "{name}") {
			t.Fatalf("%q never names the colonist", d.Title)
		}
		for _, p := range placeholder.FindAllString(d.Desc, -1) {
			switch p {
			case "{name}", "{they}", "{them}", "{their}", "{They}", "{Them}", "{Their}":
			default:
				t.Fatalf("%q: unknown placeholder %s", d.Title, p)
			}
		}
		if m := agreement.FindString(d.Desc); m != "" {
			t.Fatalf("%q: %q reads wrong for he/she/they", d.Title, m)
		}
		if d.Weight <= 0 || d.Tags == 0 || d.Tags&^(TagNeolithic|TagIndustrial|TagSpacer) != 0 {
			t.Fatalf("%q: weight %d tags %b", d.Title, d.Weight, d.Tags)
		}
		if d.Incapable&^AllBits != 0 {
			t.Fatalf("%q: invalid incapable bits %b", d.Title, d.Incapable)
		}
		if len(d.Skills) == 0 {
			t.Fatalf("%q: no skills", d.Title)
		}
		for id, v := range d.Skills {
			if id >= skill.NumSkills || !(v == -2 || (v >= 2 && v <= 6)) {
				t.Fatalf("%q: skill %d delta %d", d.Title, id, v)
			}
			if v > 0 && d.Incapable.DisablesSkill(id) {
				t.Fatalf("%q: raises a skill it disables", d.Title)
			}
		}
		switch d.Slot {
		case Childhood:
			child++
			if d.Fits(TagNeolithic) {
				neoChild++
			}
		case Adulthood:
			adult++
			if d.Fits(TagNeolithic) {
				neoAdult++
			}
		default:
			t.Fatalf("%q: slot %d", d.Title, d.Slot)
		}
	}
	if child < 24 || adult < 30 || neoChild < 8 || neoAdult < 8 {
		t.Fatalf("counts: %d childhood (%d neolithic), %d adulthood (%d neolithic)", child, neoChild, adult, neoAdult)
	}
	if DefOf(0).Title != "" || DefOf(ID(len(all)+1)).Title != "" {
		t.Fatal("DefOf out of range")
	}
}
