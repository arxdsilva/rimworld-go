package game

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/scenario"
)

const poolSeed = 0x5EED_CAFE

func builtinEffects(t *testing.T) scenario.Effects {
	t.Helper()
	bs := scenario.Builtins()
	if len(bs) == 0 {
		t.Fatal("no built-in scenarios")
	}
	return bs[0].Apply()
}

func storyByTitle(t *testing.T, title string) backstory.ID {
	t.Helper()
	for _, d := range backstory.All() {
		if d.Title == title {
			return d.ID
		}
	}
	t.Fatalf("no backstory %q", title)
	return 0
}

func nicks(p *CandidatePool) []string {
	var out []string
	for _, v := range p.Views() {
		out = append(out, v.Nick)
	}
	return out
}

func TestCandidatePoolOps(t *testing.T) {
	fx := builtinEffects(t)
	p := NewCandidatePool(poolSeed, fx)

	// Counts: ChooseFrom candidates, the first People Selected.
	vs := p.Views()
	if len(vs) != fx.ChooseFrom {
		t.Fatalf("pool size %d, want %d", len(vs), fx.ChooseFrom)
	}
	for i, v := range vs {
		if v.Index != i || v.Selected != (i < fx.People) {
			t.Fatalf("view %d: index %d selected %v", i, v.Index, v.Selected)
		}
	}
	if got := len(p.Selected()); got != fx.People {
		t.Fatalf("selected %d, want %d", got, fx.People)
	}
	// "Choose from" unset means People+5; never fewer than People.
	if n := len(NewCandidatePool(1, scenario.Effects{People: 2}).Views()); n != 7 {
		t.Fatalf("default pool size %d, want 7", n)
	}
	if n := len(NewCandidatePool(1, scenario.Effects{People: 4, ChooseFrom: 2}).Views()); n != 4 {
		t.Fatalf("pool below People: size %d, want 4", n)
	}
	// Nicknames are unique across the pool.
	seen := map[string]bool{}
	for _, n := range nicks(p) {
		if seen[n] {
			t.Fatalf("duplicate nickname %q in %v", n, nicks(p))
		}
		seen[n] = true
	}

	// Reroll: next roll of the slot, identical across pools.
	q := NewCandidatePool(poolSeed, fx)
	want := pawngen.Generate(poolSeed, 2, 1, p.rules(2))
	p.Reroll(2, false)
	q.Reroll(2, false)
	if !reflect.DeepEqual(p.pawns[2], want) || !reflect.DeepEqual(p.Views(), q.Views()) {
		t.Fatal("reroll is not the slot's next roll or not deterministic")
	}

	// keepName keeps an edited name; Shift (keepName=false) drops it.
	if err := p.Rename(1, " Ada ", "Quillon", "Stone"); err != nil {
		t.Fatal(err)
	}
	p.Reroll(1, true)
	if c := p.pawns[1]; c.First != "Ada" || c.Nick != "Quillon" || c.Last != "Stone" {
		t.Fatalf("edited name lost on reroll: %q %q %q", c.First, c.Nick, c.Last)
	}
	if c := p.pawns[1]; c.Temperament != temperamentOf(c) {
		t.Fatal("kept name with stale temperament")
	}
	p.Reroll(1, false)
	if p.pawns[1].Nick == "Quillon" || p.edited[1] {
		t.Fatal("reroll without keepName kept the edited name")
	}
	// An unedited name is not kept even with keepName.
	want = pawngen.Generate(poolSeed, 1, p.rolls[1]+1, p.rules(1))
	p.Reroll(1, true)
	if p.pawns[1].Nick != want.Nick {
		t.Fatalf("unedited reroll nick %q, want %q", p.pawns[1].Nick, want.Nick)
	}

	// Rename validation and uniqueness.
	other := p.pawns[0].Nick
	cases := []struct {
		first, nick, last string
		want              error
	}{
		{"Ada", "", "Stone", ErrNickRequired},
		{"Ada", "   ", "Stone", ErrNickRequired},
		{"Ada", "Abcdefghijklmnopq", "Stone", ErrNameTooLong},
		{"Abcdefghijklmnopq", "Ada", "Stone", ErrNameTooLong},
		{"Ada", "R2", "Stone", ErrNameChars},
		{"Ada", "Ada", "St_one", ErrNameChars},
		{"Ada", strings.ToUpper(other), "Stone", ErrNickTaken},
		{"Zoë", "O'Neil-Ray", "Van Dorn", nil},
		{"", "Abcdefghijklmnop", "", nil}, // 16 runes; first and last optional
	}
	for _, c := range cases {
		if err := p.Rename(3, c.first, c.nick, c.last); !errors.Is(err, c.want) {
			t.Fatalf("Rename(%q, %q, %q) = %v, want %v", c.first, c.nick, c.last, err, c.want)
		}
	}
	if err := p.Rename(3, "", "Abcdefghijklmnop", "Stone"); err != nil {
		t.Fatalf("renaming to one's own nickname: %v", err)
	}
	if err := p.Rename(99, "", "X", ""); err == nil {
		t.Fatal("rename out of range accepted")
	}
	colony := fx
	colony.Rules.Taken = map[string]bool{"Bram": true}
	if err := NewCandidatePool(poolSeed, colony).Rename(0, "", "bram", ""); !errors.Is(err, ErrNickTaken) {
		t.Fatalf("colony nickname reused: %v", err)
	}

	// Move: Selected are always the first People; reroll keeps the slot.
	before := nicks(p)
	p.Move(0, 5)
	after := nicks(p)
	wantOrder := append(append(append([]string{}, before[1:6]...), before[0]), before[6:]...)
	if !slices.Equal(after, wantOrder) {
		t.Fatalf("move 0→5: %v, want %v", after, wantOrder)
	}
	sel := p.Selected()
	if len(sel) != fx.People || sel[fx.People-1].Nick != before[fx.People] {
		t.Fatal("moving a Selected candidate out did not pull the top left-behind one in")
	}
	wantSlot0 := pawngen.Generate(poolSeed, 0, 1, p.rules(5))
	p.Reroll(5, false)
	if !reflect.DeepEqual(p.pawns[5], wantSlot0) {
		t.Fatal("reroll after a move did not use the candidate's own slot")
	}
	p.Move(5, 0)
	if p.slots[0] != 0 || p.slots[5] != 5 {
		t.Fatalf("move back: slots %v", p.slots)
	}

	// Swap.
	before = nicks(p)
	last := len(before) - 1
	p.Swap(0, last)
	after = nicks(p)
	if after[0] != before[last] || after[last] != before[0] || !p.Views()[0].Selected || p.Views()[last].Selected {
		t.Fatalf("swap: %v → %v", before, after)
	}
	// Out-of-range operations are ignored.
	p.Swap(-1, 0)
	p.Move(0, 99)
	p.Reroll(99, true)
	if !slices.Equal(nicks(p), after) {
		t.Fatal("out-of-range operation changed the roster")
	}

	// RandomizeAll rerolls everyone and keeps edited names.
	if err := p.Rename(4, "", "Quillon", ""); err != nil {
		t.Fatal(err)
	}
	rolls := slices.Clone(p.rolls)
	p.RandomizeAll()
	for i := range rolls {
		if p.rolls[i] != rolls[i]+1 {
			t.Fatalf("randomize all: roll %d is %d, want %d", i, p.rolls[i], rolls[i]+1)
		}
	}
	if p.pawns[4].Nick != "Quillon" {
		t.Fatal("randomize all dropped an edited name")
	}

	// Selected returns independent copies.
	sel = p.Selected()
	sel[0].Traits = append(sel[0].Traits[:0], trait.Gentle, trait.Sunny, trait.Bookish)
	sel[0].Nick = "Changed"
	if p.pawns[0].Nick == "Changed" || reflect.DeepEqual(p.pawns[0].Traits, sel[0].Traits) {
		t.Fatal("Selected aliases the pool")
	}
}

// teamPawn is an adult whose every skill is at level.
func teamPawn(nick string, level uint8, years int) pawngen.Pawn {
	p := pawngen.Pawn{First: nick, Nick: nick, AgeDays: int32(years * pawngen.DaysPerYear)}
	for i := range p.Skills {
		p.Skills[i].Level = level
	}
	return p
}

func TestTeamWarnings(t *testing.T) {
	gentle := func(p pawngen.Pawn) pawngen.Pawn { p.Traits = []trait.ID{trait.Gentle}; return p }
	withSkill := func(p pawngen.Pawn, id skill.ID, l uint8) pawngen.Pawn { p.Skills[id].Level = l; return p }
	collector := storyByTitle(t, "Debt collector") // incapable of caring
	carer := func(p pawngen.Pawn) pawngen.Pawn { p.Adulthood = collector; return p }

	all := []string{WarnNoFighter, WarnNoDoctor, WarnNoCook, WarnNoGrower, WarnNoBuilder, WarnNoResearch, WarnYoungOrOld}
	cases := []struct {
		name string
		team []pawngen.Pawn
		want []string
	}{
		{"capable team", []pawngen.Pawn{teamPawn("A", 8, 30), teamPawn("B", 5, 40)}, nil},
		{"all gentle", []pawngen.Pawn{gentle(teamPawn("A", 8, 30)), gentle(teamPawn("B", 8, 30))}, []string{WarnNoFighter}},
		{"one gentle", []pawngen.Pawn{gentle(teamPawn("A", 8, 30)), teamPawn("B", 8, 30)}, nil},
		{"low medicine", []pawngen.Pawn{withSkill(teamPawn("A", 8, 30), skill.Medicine, 2), withSkill(teamPawn("B", 8, 30), skill.Medicine, 1)}, []string{WarnNoDoctor}},
		{"all incapable of caring", []pawngen.Pawn{carer(teamPawn("A", 9, 30)), carer(teamPawn("B", 9, 30))}, []string{WarnNoDoctor}},
		{"one can care", []pawngen.Pawn{carer(teamPawn("A", 9, 30)), withSkill(teamPawn("B", 9, 30), skill.Medicine, 3)}, nil},
		{"low cooking", []pawngen.Pawn{withSkill(teamPawn("A", 8, 30), skill.Cooking, 2), withSkill(teamPawn("B", 8, 30), skill.Cooking, 0)}, []string{WarnNoCook}},
		{"low plants", []pawngen.Pawn{withSkill(teamPawn("A", 8, 30), skill.Plants, 2), withSkill(teamPawn("B", 8, 30), skill.Plants, 2)}, []string{WarnNoGrower}},
		{"low construction", []pawngen.Pawn{withSkill(teamPawn("A", 8, 30), skill.Construction, 2), withSkill(teamPawn("B", 8, 30), skill.Construction, 1)}, []string{WarnNoBuilder}},
		{"low intellectual", []pawngen.Pawn{withSkill(teamPawn("A", 8, 30), skill.Intellectual, 1), withSkill(teamPawn("B", 8, 30), skill.Intellectual, 2)}, []string{WarnNoResearch}},
		{"young and old", []pawngen.Pawn{teamPawn("A", 8, 15), teamPawn("B", 8, 70)}, []string{WarnYoungOrOld}},
		{"young and adult", []pawngen.Pawn{teamPawn("A", 8, 17), teamPawn("B", 8, 64)}, nil},
		{"everything wrong", []pawngen.Pawn{gentle(carer(teamPawn("A", 2, 14))), gentle(teamPawn("B", 0, 65))}, all},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := NewCandidatePool(poolSeed, scenario.Effects{People: 2, ChooseFrom: 3})
			copy(p.pawns, c.team)
			// A brilliant left-behind candidate must not count.
			p.pawns[2] = teamPawn("Left", 20, 30)
			if got := p.Team().Warnings; !slices.Equal(got, c.want) {
				t.Fatalf("warnings %q, want %q", got, c.want)
			}
		})
	}

	// Best level per skill and who has it (first in order on ties).
	p := NewCandidatePool(poolSeed, scenario.Effects{People: 2, ChooseFrom: 2})
	p.pawns[0] = withSkill(gentle(teamPawn("A", 6, 30)), skill.Cooking, 4)
	p.pawns[1] = withSkill(teamPawn("B", 6, 30), skill.Cooking, 12)
	r := p.Team()
	if len(r.Best) != int(skill.NumSkills) {
		t.Fatalf("best has %d skills", len(r.Best))
	}
	if b := r.Best[skill.Cooking]; b.Level != 12 || b.Who != "B" || b.Key != "cooking" || b.Label != "Cooking" {
		t.Fatalf("best cooking %+v", b)
	}
	if b := r.Best[skill.Mining]; b.Level != 6 || b.Who != "A" {
		t.Fatalf("tie should go to the first: %+v", b)
	}
	if b := r.Best[skill.Shooting]; b.Who != "B" {
		t.Fatalf("incapable candidate counted for shooting: %+v", b)
	}
	p.pawns[1] = gentle(p.pawns[1])
	if b := p.Team().Best[skill.Shooting]; b.Level != 0 || b.Who != "" {
		t.Fatalf("all incapable: %+v", b)
	}
}

func TestBioOfFields(t *testing.T) {
	p := pawngen.Pawn{
		First: "Ada", Nick: "Wren", Last: "Stone",
		Look:       look.Look{Sex: look.Female, Build: look.Thin},
		AgeDays:    34*pawngen.DaysPerYear + 7,
		Childhood:  storyByTitle(t, "Lighthouse kid"),
		Adulthood:  storyByTitle(t, "Seed archivist"), // incapable of violence
		Traits:     []trait.ID{trait.Gentle},
		Conditions: []pawngen.Condition{pawngen.OldFracture},
	}
	p.Skills = skill.NeutralSet()
	p.Skills[skill.Shooting] = skill.Skill{} // incapable skills stay 0
	p.Skills[skill.Melee] = skill.Skill{}
	p.Skills[skill.Plants] = skill.Skill{Level: 12, XP: skill.Cost(12) / 4, Passion: skill.Major}
	v := bioOf(p)

	if v.First != "Ada" || v.Nick != "Wren" || v.Last != "Stone" || v.Sex != "Female" || v.Build != "Thin" || v.AgeYears != 34 {
		t.Fatalf("identity %+v", v)
	}
	if v.Childhood != "Lighthouse kid" || v.Adulthood != "Seed archivist" {
		t.Fatalf("titles %q / %q", v.Childhood, v.Adulthood)
	}
	if !strings.Contains(v.ChildhoodDesc, "Wren") || !strings.Contains(v.ChildhoodDesc, "taught her patience") {
		t.Fatalf("childhood desc not filled: %q", v.ChildhoodDesc)
	}
	for _, s := range []string{v.ChildhoodDesc, v.AdulthoodDesc, v.Traits[0].Desc} {
		if s == "" || strings.ContainsAny(s, "{}") {
			t.Fatalf("unfilled description %q", s)
		}
	}
	if len(v.Traits) != 1 || v.Traits[0].Key != "gentle" || v.Traits[0].Label != "Gentle" ||
		!strings.Contains(v.Traits[0].Effects, "Incapable: Violence") {
		t.Fatalf("traits %+v", v.Traits)
	}
	if want := []string{"Violence (backstory: Seed archivist, trait: Gentle)"}; !slices.Equal(v.Incapable, want) {
		t.Fatalf("incapable %q, want %q", v.Incapable, want)
	}
	if want := []string{"Old leg fracture: " + pawngen.OldFracture.Desc()}; !slices.Equal(v.Conditions, want) {
		t.Fatalf("conditions %q", v.Conditions)
	}
	if len(v.Skills) != int(skill.NumSkills) {
		t.Fatalf("%d skills", len(v.Skills))
	}
	for _, id := range []skill.ID{skill.Shooting, skill.Melee} {
		s := v.Skills[id]
		if !s.Incapable || s.LevelLabel != "—" || s.Level != 0 || s.Passion != 0 || s.SpeedPct != 0 || s.Key != id.Key() {
			t.Fatalf("incapable skill shown as %+v", s)
		}
	}
	if s := v.Skills[skill.Plants]; s.Incapable || s.Level != 12 || s.LevelLabel != "Expert" || s.Passion != 2 ||
		s.XPPct != 25 || s.Affects == "" || s.SpeedPct != skill.SpeedPermil(12)/10 {
		t.Fatalf("plants %+v", s)
	}
	if s := v.Skills[skill.Social]; s.SpeedPct != 0 || s.Level != 7 {
		t.Fatalf("social drives no work: %+v", s)
	}

	// A teenager has no adulthood; a Tireless pawn works faster.
	teen := pawngen.Pawn{Nick: "Kit", AgeDays: 15 * pawngen.DaysPerYear, Childhood: p.Childhood, Skills: skill.NeutralSet()}
	if tv := bioOf(teen); tv.Adulthood != "—" || tv.AdulthoodDesc != "" || tv.Incapable != nil {
		t.Fatalf("teen %+v", tv)
	}
	quick := teamPawn("Q", 7, 30)
	quick.Traits = []trait.ID{trait.Tireless}
	if s := bioOf(quick).Skills[skill.Construction]; s.SpeedPct != 120 {
		t.Fatalf("tireless construction speed %d%%, want 120", s.SpeedPct)
	}

	// A legacy pawn: no backstories, all skills neutral at 100%.
	lv := bioOf(pawngen.Legacy(7, "Mira", 5, 5, 6))
	if lv.Childhood != "—" || lv.Adulthood != "—" || lv.Nick != "Mira" || lv.Sex != "Male" || lv.Incapable != nil || lv.Traits != nil {
		t.Fatalf("legacy %+v", lv)
	}
	for _, s := range lv.Skills {
		if s.Level != skill.Neutral || (s.Key != "social" && s.SpeedPct != 100) {
			t.Fatalf("legacy skill %+v", s)
		}
	}

	// Generated pools: every description filled, reasons for every incapability.
	for _, d := range scenario.Builtins() {
		for _, cv := range NewCandidatePool(poolSeed, d.Apply()).Views() {
			if strings.ContainsAny(cv.ChildhoodDesc+cv.AdulthoodDesc, "{}") {
				t.Fatalf("%s: unfilled backstory text for %s", d.Name, cv.Nick)
			}
			for _, s := range cv.Incapable {
				if !strings.Contains(s, "(backstory: ") && !strings.Contains(s, "(trait: ") {
					t.Fatalf("incapability without reason: %q", s)
				}
			}
		}
	}
}

func TestPoolDeterministic(t *testing.T) {
	for _, d := range scenario.Builtins() {
		fx := d.Apply()
		a, b := NewCandidatePool(poolSeed, fx), NewCandidatePool(poolSeed, fx)
		if !reflect.DeepEqual(a.Views(), b.Views()) || !reflect.DeepEqual(a.Team(), b.Team()) {
			t.Fatalf("%s: same seed, different pools", d.Name)
		}
		for _, p := range []*CandidatePool{a, b} {
			p.Reroll(0, false)
			p.Move(1, len(p.pawns)-1)
			p.Swap(0, 1)
			p.RandomizeAll()
		}
		if !reflect.DeepEqual(a.Views(), b.Views()) || !reflect.DeepEqual(a.Selected(), b.Selected()) {
			t.Fatalf("%s: same operations, different pools", d.Name)
		}
		if reflect.DeepEqual(NewCandidatePool(poolSeed+1, fx).Views(), NewCandidatePool(poolSeed, fx).Views()) {
			t.Fatalf("%s: different seeds gave the same pool", d.Name)
		}
	}
}
