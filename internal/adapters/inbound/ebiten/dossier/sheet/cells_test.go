package sheet

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
)

func melee(m ...func(*game.SkillView)) game.SkillView {
	s := game.SkillView{Key: "melee", Label: "Melee", Level: 12, XPPct: 45, SpeedPct: 130, Affects: "fighting"}
	for _, f := range m {
		f(&s)
	}
	return s
}

func cardWith(m ...func(*dossierview.Card)) dossierview.Card {
	c := dossierview.Card{ActiveSkill: -1, WorkType: -1}
	for _, f := range m {
		f(&c)
	}
	return c
}

// Each state, then every pair: the higher priority always wins. Priority is
// off, saturated, active, major passion, minor passion, plain.
func TestSkillStatePriority(t *testing.T) {
	t.Parallel()
	melee := int(skill.Melee)
	states := []struct {
		name  string
		want  SkillState
		apply func(*game.SkillView, *dossierview.Card)
	}{
		{"off", SkillOff, func(s *game.SkillView, _ *dossierview.Card) { s.Incapable = true }},
		{"full", SkillFull, func(_ *game.SkillView, c *dossierview.Card) { c.Saturated[melee] = true }},
		{"active", SkillActive, func(_ *game.SkillView, c *dossierview.Card) { c.ActiveSkill = melee }},
		{"love", SkillLove, func(s *game.SkillView, _ *dossierview.Card) { s.Passion = 2 }},
		{"like", SkillLike, func(s *game.SkillView, _ *dossierview.Card) { s.Passion = 1 }},
	}
	build := func(idx ...int) (game.SkillView, dossierview.Card) {
		s, c := melee_(), cardWith()
		for _, i := range idx {
			states[i].apply(&s, &c)
		}
		return s, c
	}
	if s, c := build(); SkillStateOf(s, c) != SkillPlain {
		t.Fatal("plain")
	}
	for i, st := range states {
		if s, c := build(i); SkillStateOf(s, c) != st.want {
			t.Errorf("%s alone: %v", st.name, SkillStateOf(s, c))
		}
	}
	for hi := range states {
		for lo := hi + 1; lo < len(states); lo++ {
			if states[hi].name == "love" && states[lo].name == "like" {
				continue // a pawn has one passion
			}
			s, c := build(hi, lo)
			if got := SkillStateOf(s, c); got != states[hi].want {
				t.Errorf("%s over %s: got %v", states[hi].name, states[lo].name, got)
			}
		}
	}
	s, c := build(0, 1, 2, 3, 4)
	if SkillStateOf(s, c) != SkillOff {
		t.Error("all at once is off")
	}
	// the other skills' flags do not leak
	s, c = build()
	c.Saturated[int(skill.Shooting)] = true
	c.ActiveSkill = int(skill.Shooting)
	if SkillStateOf(s, c) != SkillPlain {
		t.Error("another skill's flags")
	}
	// colour keys are distinct
	seen := map[string]SkillState{}
	for st := SkillPlain; st <= SkillOff; st++ {
		if prev, dup := seen[st.Key()]; dup {
			t.Errorf("states %d and %d share %s", prev, st, st.Key())
		}
		seen[st.Key()] = st
	}
}

func melee_() game.SkillView { return melee() }

func TestSkillCell(t *testing.T) {
	t.Parallel()
	c := SkillCell(en, melee(), cardWith(), false)
	if c.Label != "Melee" || c.Value != "12" || c.Pct != 60 || c.Tone != ToneText || c.Active {
		t.Errorf("%+v", c)
	}
	if v := SkillCell(en, melee(), cardWith(), true).Value; v != "12.45" {
		t.Errorf("decimals %q", v)
	}
	if v := SkillCell(en, melee(func(s *game.SkillView) { s.XPPct = 7 }), cardWith(), true).Value; v != "12.07" {
		t.Errorf("padded %q", v)
	}
	if v := SkillCell(en, melee(func(s *game.SkillView) { s.XPPct = 100 }), cardWith(), true).Value; v != "12.99" {
		t.Errorf("capped at 99: %q", v)
	}
	if v := SkillCell(en, melee(func(s *game.SkillView) { s.Level = 20 }), cardWith(), true).Value; v != "20" {
		t.Errorf("a maxed skill has no progress: %q", v)
	}
	off := SkillCell(en, melee(func(s *game.SkillView) { s.Incapable = true; s.Level = 0 }), cardWith(), true)
	if off.Value != "-" || off.Tone != ToneOff || off.Pct != 0 {
		t.Errorf("%+v", off)
	}
	if !strings.Contains(off.Tip, "cannot do") || strings.Contains(off.Tip, "speed") {
		t.Errorf("a disabled skill lists no stats: %q", off.Tip)
	}
	act := SkillCell(en, melee(), cardWith(func(c *dossierview.Card) { c.ActiveSkill = int(skill.Melee) }), false)
	if !act.Active || !strings.Contains(act.Tip, "In use right now") {
		t.Errorf("%+v", act)
	}
	full := SkillCell(en, melee(), cardWith(func(c *dossierview.Card) { c.Saturated[skill.Melee] = true }), false)
	if !strings.Contains(full.Tip, "capped for today") {
		t.Errorf("the tip says why it is full: %q", full.Tip)
	}
}

func TestSkillTipLines(t *testing.T) {
	t.Parallel()
	c := cardWith(func(c *dossierview.Card) {
		c.Stats[stat.WorkSpeed] = 1100
		c.Stats[stat.ShootAcc] = 4
		c.Stats[stat.AimCooldown] = 900
		c.ExtraLines[skill.Melee] = []dossierview.StatLine{{Label: "Hit chance", Pct: 71}}
		c.LearnedWeek[skill.Melee] = 350
	})
	has := func(tip []TipLine, want string) {
		t.Helper()
		if !strings.Contains(JoinTip(tip), want) {
			t.Errorf("missing %q in\n%s", want, JoinTip(tip))
		}
	}
	m := SkillTip(en, melee(), c, "Damage 9 per second")
	has(m, "Drives: fighting")
	has(m, "Global work speed: 110%")
	has(m, "Damage 9 per second")
	has(m, "Hit chance: 71%")
	has(m, "Learned this week: 350")
	if strings.Contains(JoinTip(SkillTip(en, melee(), c, "")), "Damage") {
		t.Error("no weapon line when none is given")
	}
	for _, id := range []skill.ID{skill.Animals, skill.Medicine, skill.Shooting, skill.Social, skill.Construction, skill.Cooking, skill.Crafting, skill.Mining, skill.Plants, skill.Intellectual} {
		sv := game.SkillView{Key: id.Key(), Label: id.Label(), Level: 10, SpeedPct: 120}
		if tip := SkillTip(en, sv, c, ""); len(tip) < 3 {
			t.Errorf("%s: only %d lines", id.Key(), len(tip))
		}
	}
	an := SkillTip(en, game.SkillView{Key: "animals", Label: "Animals", Level: 10, SpeedPct: 120}, c, "")
	has(an, "Gather yield: 102%")
	has(an, "Chance to tame at typical wildness (50%)")
	has(an, "Handling speed: 120%")
	md := SkillTip(en, game.SkillView{Key: "medicine", Label: "Medicine", Level: 8}, c, "")
	has(md, "Tend quality without medicine")
	has(md, "Doctoring speed: 106%") // no SpeedPct: from the level (neutral is 7: 1060)
}

func TestTrainCell(t *testing.T) {
	t.Parallel()
	if c := TrainCell(en, game.AnimalTrainingView{Skill: int(training.Obedience), Label: "Obedience", Allowed: false}); c != (Cell{}) {
		t.Errorf("a species that cannot learn it hides the row: %+v", c)
	}
	c := TrainCell(en, game.AnimalTrainingView{Skill: int(training.Obedience), Label: "Obedience", Allowed: true, On: true, Steps: 2, Needed: 3})
	if c.Value != "2 of 3" || c.Pct != 66 || c.Tone != ToneText || c.Active {
		t.Errorf("in progress: %+v", c)
	}
	c = TrainCell(en, game.AnimalTrainingView{Label: "Guard", Allowed: true, On: false, Steps: 0, Needed: 3})
	if c.Tone != ToneOff {
		t.Errorf("not wanted reads off: %+v", c)
	}
	c = TrainCell(en, game.AnimalTrainingView{Label: "Guard", Allowed: true, On: true, Learned: true, Steps: 3, Needed: 3})
	if c.Tone != ToneGood || !c.Active || c.Value != "Learned" || c.Pct != 100 {
		t.Errorf("learned: %+v", c)
	}
	c = TrainCell(en, game.AnimalTrainingView{Label: "Guard", Allowed: true, On: false, Learned: true, Steps: 3, Needed: 3})
	if c.Tone != ToneGood {
		t.Errorf("learned beats not wanted: %+v", c)
	}
	if !strings.Contains(c.Tip, "Fades one step after 10d") {
		t.Errorf("tip %q", c.Tip)
	}
	if c := TrainCell(en, game.AnimalTrainingView{Allowed: true, Steps: 5, Needed: 0}); c.Pct != 0 {
		t.Errorf("no needed steps: %+v", c)
	}
	if DecayDays() != 10 {
		t.Errorf("decay days %d", DecayDays())
	}
}

func TestTips(t *testing.T) {
	t.Parallel()
	h := game.HealthView{
		Parts: []game.PartView{
			{Label: "Left arm", Status: "missing"},
			{Label: "Head", Status: "hurt", HP: 4, TendTip: "cut, 6 HP: tended 74%"},
			{Label: "Torso", Status: "hurt", HP: 9, Bleed: 12},
		},
		Conditions: []game.ConditionView{
			cond("Flu", 30, func(c *game.ConditionView) { c.ImmPct = 100 }),
			cond("Gut worms", 30, func(c *game.ConditionView) { c.TendPct = 60 }),
			cond("Plague", 50, losing),
		},
		Drugs: []game.DrugLineView{{Label: "Smokeleaf", Pct: 40}},
		Pain:  15,
	}
	tip := HealthTip(en, h)
	want := map[string]Tone{
		"Left arm: missing, permanent":                    ToneNeutral,
		"Head: hurt, tended":                              ToneWarning,
		"Torso: hurt, 9 HP, untended, bleeding 12% a day": ToneCritical,
		"Flu 30%, immune":                                 ToneNeutral,
		"Gut worms 30%, tended 60%":                       ToneWarning,
		"Plague 50%, life-threatening":                    ToneCritical,
		"Smokeleaf dependence 40%":                        ToneWarning,
	}
	got := map[string]Tone{}
	for _, l := range tip {
		got[l.Text] = l.Tone
	}
	for text, tone := range want {
		if g, ok := got[text]; !ok || g != tone {
			t.Errorf("%q: %v (present %v), want %v", text, g, ok, tone)
		}
	}
	if tip[0].Text != "Left arm: missing, permanent" {
		t.Errorf("parts keep their order: %q", tip[0].Text)
	}
	if none := HealthTip(en, game.HealthView{}); len(none) != 1 || none[0].Text != "Nothing wrong" || none[0].Tone != ToneOff {
		t.Errorf("%+v", none)
	}

	p := game.PawnView{Mood: 55, Thoughts: []game.ThoughtView{{Label: "Ate well", Value: 6}, {Label: "Slept outside", Value: -12}, {Label: "Ate well", Value: 6}, {Label: "Room is ugly", Value: -3}, {Label: "Nice view", Value: 0}}}
	mt := MindTip(en, p, dossierview.Card{BreakAt: dossierview.BreakLevels{Minor: 35, Major: 20, Extreme: 5}})
	text := JoinTip(mt)
	for _, w := range []string{"Mood: 55%", "Breaks start below 35 (minor), 20 (serious), 5 (extreme)", "+12 Ate well (x2)", "-12 Slept outside", "-3 Room is ugly", "0 Nice view"} {
		if !strings.Contains(text, w) {
			t.Errorf("mind tip lacks %q:\n%s", w, text)
		}
	}
	tones := map[string]Tone{}
	for _, l := range mt {
		tones[l.Text] = l.Tone
	}
	if tones["+12 Ate well (x2)"] != ToneGood || tones["-12 Slept outside"] != ToneCritical || tones["-3 Room is ugly"] != ToneWarning || tones["0 Nice view"] != ToneNeutral {
		t.Errorf("tones %v", tones)
	}
	if JoinTip(MindTip(en, game.PawnView{}, dossierview.Card{})) != "Mood: 0%\nNo thoughts" {
		t.Errorf("%q", JoinTip(MindTip(en, game.PawnView{}, dossierview.Card{})))
	}

	var c dossierview.Card
	c.EatSpeedPct = 80
	c.Stats[stat.HungerRate] = 1100
	c.Stats[stat.RestRate] = 950
	if got := JoinTip(FoodTip(en, "omnivore", "Fine meals", c)); got != "Diet: omnivore\nFood policy: Fine meals\nEating speed: 80%\nHunger rate: 110%" {
		t.Errorf("%q", got)
	}
	if got := JoinTip(RestTip(en, c)); got != "Rest rate: 95%" {
		t.Errorf("%q", got)
	}
	if len(FunTip(en, game.PawnView{})) != 4 {
		t.Error("fun tip")
	}
	b := game.BioView{Childhood: "Street kid", Adulthood: "Medic", Traits: []game.TraitView{{Label: "Brave"}}, Incapable: []string{"Violence (backstory: Medic)"}}
	bt := BioTip(en, game.PawnView{Name: "Ana", Title: "Medic"}, b, dossierview.Card{Faction: "Reach"})
	if bt[0].Text != "Ana, medic of Reach" || bt[len(bt)-1].Tone != ToneOff {
		t.Errorf("%+v", bt)
	}
	at := JoinTip(AnimalTip(en, game.AnimalView{Label: "deer", Stage: "adult", MinHandling: 4, Wildness: 30, Revenge: 5, Diet: "plants", Master: "Ana", Bond: "Ana", Product: "milk", Tame: true}))
	for _, w := range []string{"Deer (adult)", "fades one step after 10d", "Eats: plants", "Gives milk", "Master: Ana", "Bonded with Ana"} {
		if !strings.Contains(at, w) {
			t.Errorf("animal tip lacks %q:\n%s", w, at)
		}
	}
	if wt := JoinTip(AnimalTip(en, game.AnimalView{Label: "deer", MinHandling: 4, Wildness: 30, Revenge: 5})); !strings.Contains(wt, "Needs Animals skill 4 to tame") || strings.Contains(wt, "fades") {
		t.Errorf("wild tip:\n%s", wt)
	}
}

func TestWeaponLine(t *testing.T) {
	t.Parallel()
	var p game.PawnView
	if WeaponLine(en, p) != "" {
		t.Error("no stats, no line")
	}
	p.WeaponStats.Name, p.WeaponStats.Damage, p.WeaponStats.Warmup, p.WeaponStats.Cooldown = "fists", 4, 30, 90
	if got := WeaponLine(en, p); got != "Fists: 4 damage a hit, 2.0 a second" {
		t.Errorf("%q", got)
	}
}
