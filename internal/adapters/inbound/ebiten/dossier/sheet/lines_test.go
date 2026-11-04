package sheet

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/pawn/mentalbreak"
)

func cond(label string, sev int, mods ...func(*game.ConditionView)) game.ConditionView {
	c := game.ConditionView{Label: label, SevPct: sev}
	for _, m := range mods {
		m(&c)
	}
	return c
}

func due(c *game.ConditionView)    { c.Due = true }
func losing(c *game.ConditionView) { c.Losing = true }

func healthy() game.PawnView { return game.PawnView{Status: "alive", Health: 100} }

func TestBodyLineRungs(t *testing.T) {
	t.Parallel()
	with := func(m func(*game.PawnView)) game.PawnView { p := healthy(); m(&p); return p }
	hv := func(cs ...game.ConditionView) game.HealthView { return game.HealthView{Conditions: cs} }
	cases := []struct {
		name string
		p    game.PawnView
		h    game.HealthView
		text string
		tone Tone
	}{
		{"dead", with(func(p *game.PawnView) { p.Status = "dead" }), hv(), "Dead", ToneCritical},
		{"embodied", with(func(p *game.PawnView) { p.Embodied = true }), hv(), "Dead", ToneCritical},
		{"bleeding out", with(func(p *game.PawnView) { p.Bleeding, p.BleedDeathHours = true, 3 }), hv(), "Bleeding (death in 3h)", ToneCritical},
		{"bleeding out within the hour", with(func(p *game.PawnView) { p.Bleeding = true }), hv(), "Bleeding (death within the hour)", ToneCritical},
		{"bleeding slowly", with(func(p *game.PawnView) { p.Bleeding, p.BleedDeathHours = true, 30 }), hv(), "Healthy", ToneGood},
		{"one untended", healthy(), hv(cond("Cut", 20, due)), "Cut needs tending", ToneWarning},
		{"one untended and losing", healthy(), hv(cond("Infection", 40, due, losing)), "Infection needs tending, life-threatening", ToneCritical},
		{"many untended", healthy(), hv(cond("Cut", 10, due), cond("Bruise", 10, due), cond("Gash", 10, due)), "3 wounds need tending", ToneWarning},
		{"many untended one losing", healthy(), hv(cond("Cut", 10, due), cond("Bruise", 10, due, losing)), "2 wounds need tending, one life-threatening", ToneCritical},
		{"many untended all losing", healthy(), hv(cond("Cut", 10, due, losing), cond("Bruise", 10, due, losing)), "2 wounds need tending, 2 life-threatening", ToneCritical},
		{"wounded without rows", with(func(p *game.PawnView) { p.Wounded = true }), hv(), "A wound needs tending", ToneWarning},
		{"wounded without rows and losing", with(func(p *game.PawnView) { p.Wounded, p.InfectionLosing = true, true }), hv(), "A wound needs tending, life-threatening", ToneCritical},
		{"one losing, tended", healthy(), hv(cond("Plague", 50, losing)), "Plague (life-threatening)", ToneCritical},
		{"two losing", healthy(), hv(cond("Plague", 50, losing), cond("Gut worms", 20, losing)), "2 life-threatening conditions", ToneCritical},
		{"infection flag only", with(func(p *game.PawnView) { p.InfectionLosing = true }), hv(), "Infection (life-threatening)", ToneCritical},
		{"affected one", healthy(), hv(cond("Bruise", 10)), "Bruise", ToneNeutral},
		{"affected two, worst first", healthy(), hv(cond("Bruise", 10), cond("Cut", 30)), "Cut, bruise", ToneNeutral},
		{"affected many", healthy(), hv(cond("A", 1), cond("B", 2), cond("C", 3), cond("D", 4)), "4 conditions", ToneNeutral},
		{"disease only", with(func(p *game.PawnView) { p.Disease = "flu" }), hv(), "Flu", ToneNeutral},
		{"downed", with(func(p *game.PawnView) { p.Status = "downed" }), hv(), "Downed", ToneWarning},
		{"gravely hurt", with(func(p *game.PawnView) { p.Health = 29 }), hv(), "Gravely hurt", ToneCritical},
		{"poor", with(func(p *game.PawnView) { p.Health = 59 }), hv(), "Poor health", ToneWarning},
		{"stable at 60", with(func(p *game.PawnView) { p.Health = 60 }), hv(), "Stable", ToneNeutral},
		{"stable at 90", with(func(p *game.PawnView) { p.Health = 90 }), hv(), "Stable", ToneNeutral},
		{"healthy at 91", with(func(p *game.PawnView) { p.Health = 91 }), hv(), "Healthy", ToneGood},
		{"critical edge 30", with(func(p *game.PawnView) { p.Health = 30 }), hv(), "Poor health", ToneWarning},
	}
	for _, c := range cases {
		l := BodyLine(c.p, c.h)
		if l.Text != c.text || l.Tone != c.tone {
			t.Errorf("%s: %q/%v, want %q/%v", c.name, l.Text, l.Tone, c.text, c.tone)
		}
		if l.Tab != TabHealth {
			t.Errorf("%s: click opens %q", c.name, l.Tab)
		}
		if !WarnsInWords(l) {
			t.Errorf("%s: tone %v but the words do not say why: %q", c.name, l.Tone, l.Text)
		}
	}
}

// A rung higher in the ladder always beats one below it.
func TestBodyLineFirstMatchWins(t *testing.T) {
	t.Parallel()
	ladder := []struct {
		name string
		set  func(*game.PawnView, *game.HealthView)
		text string
	}{
		{"dead", func(p *game.PawnView, _ *game.HealthView) { p.Status = "dead" }, "Dead"},
		{"bleeding", func(p *game.PawnView, _ *game.HealthView) { p.Bleeding, p.BleedDeathHours = true, 2 }, "Bleeding (death in 2h)"},
		{"untended", func(_ *game.PawnView, h *game.HealthView) { h.Conditions = append(h.Conditions, cond("Cut", 10, due)) }, "Cut needs tending"},
		{"losing", func(_ *game.PawnView, h *game.HealthView) {
			h.Conditions = append(h.Conditions, cond("Plague", 10, losing))
		}, "Plague (life-threatening)"},
		{"affected", func(_ *game.PawnView, h *game.HealthView) { h.Conditions = append(h.Conditions, cond("Bruise", 5)) }, "Bruise"},
		{"downed", func(p *game.PawnView, _ *game.HealthView) { p.Status = "downed" }, "Downed"},
		{"hurt", func(p *game.PawnView, _ *game.HealthView) { p.Health = 10 }, "Gravely hurt"},
	}
	for hi := range ladder {
		for lo := hi + 1; lo < len(ladder); lo++ {
			p, h := healthy(), game.HealthView{}
			// set the lower one first so the higher cannot overwrite a status it relies on
			ladder[lo].set(&p, &h)
			ladder[hi].set(&p, &h)
			if got := BodyLine(p, h).Text; got != ladder[hi].text {
				t.Errorf("%s over %s: %q, want %q", ladder[hi].name, ladder[lo].name, got, ladder[hi].text)
			}
		}
	}
}

func TestMindLineRungs(t *testing.T) {
	t.Parallel()
	br := dossierview.BreakLevels{Minor: 35, Major: 20, Extreme: 5}
	pawn := func(mood int, m ...func(*game.PawnView)) game.PawnView {
		p := game.PawnView{Mood: mood}
		for _, f := range m {
			f(&p)
		}
		return p
	}
	card := func(m ...func(*dossierview.Card)) dossierview.Card {
		c := dossierview.Card{BreakAt: br}
		for _, f := range m {
			f(&c)
		}
		return c
	}
	cases := []struct {
		name string
		p    game.PawnView
		c    dossierview.Card
		text string
		tone Tone
	}{
		{"stunned", pawn(80), card(func(c *dossierview.Card) { c.StunTicks = 61 }), "Stunned for 2 s", ToneCritical},
		{"stunned one tick", pawn(80), card(func(c *dossierview.Card) { c.StunTicks = 1 }), "Stunned for 1 s", ToneCritical},
		{"staggered", pawn(80), card(func(c *dossierview.Card) { c.StaggerTicks = 120 }), "Staggered for 2 s", ToneWarning},
		{"staggered flag", pawn(80, func(p *game.PawnView) { p.Staggered = true }), card(), "Staggered", ToneWarning},
		{"break extreme", pawn(50, func(p *game.PawnView) { p.MentalBreak = "berserk" }), card(func(c *dossierview.Card) { c.BreakTier = mentalbreak.Extreme }), "Mental break: berserk", ToneCritical},
		{"break minor", pawn(50, func(p *game.PawnView) { p.MentalBreak = "sad wander" }), card(func(c *dossierview.Card) { c.BreakTier = mentalbreak.Minor }), "Mental break: sad wander", ToneWarning},
		{"near extreme", pawn(4), card(), "Near collapse", ToneCritical},
		{"at extreme edge", pawn(5), card(), "Near a serious break", ToneWarning},
		{"near major", pawn(19), card(), "Near a serious break", ToneWarning},
		{"at major edge", pawn(20), card(), "Close to a minor break", ToneWarning},
		{"near minor", pawn(34), card(), "Close to a minor break", ToneWarning},
		{"at minor edge", pawn(35), card(), "Content", ToneNeutral},
		{"inspired", pawn(80, func(p *game.PawnView) { p.Inspiration, p.InspirationTicks = "frenzy", 2500*5 }), card(), "Inspired: frenzy, 5h left", ToneTop},
		{"inspired short", pawn(80, func(p *game.PawnView) { p.Inspiration, p.InspirationTicks = "frenzy", 100 }), card(), "Inspired: frenzy, under 1h left", ToneTop},
		{"break beats inspired", pawn(10, func(p *game.PawnView) { p.Inspiration = "x" }), card(), "Near a serious break", ToneWarning},
		{"top", pawn(91), card(), "Joyful", ToneTop},
		{"good edge 90", pawn(90), card(), "Joyful", ToneGood},
		{"good", pawn(66), card(), "Happy", ToneGood},
		{"neutral edge 65", pawn(65), card(), "Happy", ToneNeutral},
		{"neutral", pawn(50), card(), "Content", ToneNeutral},
		{"unhappy but no card", pawn(25), dossierview.Card{}, "Unhappy", ToneWarning},
		{"miserable", pawn(10), dossierview.Card{}, "Miserable", ToneWarning},
	}
	for _, c := range cases {
		l := MindLine(c.p, c.c)
		if l.Text != c.text || l.Tone != c.tone {
			t.Errorf("%s: %q/%v, want %q/%v", c.name, l.Text, l.Tone, c.text, c.tone)
		}
		if l.Tab != TabNeeds || !WarnsInWords(l) {
			t.Errorf("%s: tab %q words %v", c.name, l.Tab, WarnsInWords(l))
		}
	}
}

func TestMindLineFirstMatchWins(t *testing.T) {
	t.Parallel()
	p := game.PawnView{Mood: 1, MentalBreak: "x", Inspiration: "y", Staggered: true}
	c := dossierview.Card{StunTicks: 60, StaggerTicks: 60, BreakAt: dossierview.BreakLevels{Minor: 35, Major: 20, Extreme: 5}}
	steps := []struct {
		strip func()
		want  string
	}{
		{func() { c.StunTicks = 0 }, "Staggered for 1 s"},
		{func() { c.StaggerTicks = 0 }, "Staggered"},
		{func() { p.Staggered = false }, "Mental break: x"},
		{func() { p.MentalBreak = "" }, "Near collapse"},
		{func() { p.Mood = 80 }, "Inspired: y, under 1h left"},
		{func() { p.Inspiration = "" }, "Joyful"},
	}
	if got := MindLine(p, c).Text; got != "Stunned for 1 s" {
		t.Fatalf("%q", got)
	}
	for _, s := range steps {
		s.strip()
		if got := MindLine(p, c).Text; got != s.want {
			t.Fatalf("got %q want %q", got, s.want)
		}
	}
}

func TestToneAndWordsAgree(t *testing.T) {
	t.Parallel()
	// every critical or warning line from the ladders above names its state
	for _, p := range []game.PawnView{{Status: "dead"}, {Status: "downed"}, {Health: 10}, {Health: 50}, {Mood: 3}, {Wounded: true}, {MentalBreak: "x"}} {
		for _, l := range []Line{BodyLine(p, game.HealthView{}), MindLine(p, dossierview.Card{})} {
			if !WarnsInWords(l) {
				t.Errorf("%+v: %q has tone %v and no matching words", p, l.Text, l.Tone)
			}
		}
	}
	for _, rel := range []dossierview.Relation{dossierview.RelationFoe, dossierview.RelationHeld} {
		l := StandingLine(game.PawnView{Title: "x", Hostile: rel == dossierview.RelationFoe, Prisoner: rel == dossierview.RelationHeld}, game.BioView{}, dossierview.Card{})
		if !WarnsInWords(l) {
			t.Errorf("relation %d: %q", rel, l.Text)
		}
	}
	if !WarnsInWords(Line{Text: "anything", Tone: ToneGood}) {
		t.Error("only warning and critical tones need words")
	}
	if WarnsInWords(Line{Text: "all fine", Tone: ToneWarning}) {
		t.Error("the guard itself must catch a mismatch")
	}
}

func TestStandingLine(t *testing.T) {
	t.Parallel()
	own := dossierview.Card{Relation: dossierview.RelationOwn}
	cases := []struct {
		name string
		p    game.PawnView
		c    dossierview.Card
		text string
		tone Tone
	}{
		{"colonist", game.PawnView{Title: "Convoy mechanic"}, own, "Convoy mechanic", ToneGood},
		{"colonist untitled", game.PawnView{}, own, "Colonist", ToneGood},
		{"prisoner", game.PawnView{Title: "Cook", Prisoner: true, GuestFaction: "Ashen Court"}, dossierview.Card{Relation: dossierview.RelationHeld}, "Prisoner, Cook of Ashen Court", ToneWarning},
		{"raider", game.PawnView{Title: "Raider", Hostile: true}, dossierview.Card{Relation: dossierview.RelationFoe, Faction: "Ashen Court"}, "Hostile, Raider of Ashen Court", ToneCritical},
		{"ally", game.PawnView{Title: "Soldier", Ally: true}, dossierview.Card{Relation: dossierview.RelationAlly, Faction: "Reach"}, "Allied, Soldier of Reach", ToneTop},
		{"trader role wins", game.PawnView{Title: "Cook", Visitor: true}, dossierview.Card{Relation: dossierview.RelationNone, Role: "Bulk trader", Faction: "Reach"}, "Visitor, Bulk trader of Reach", ToneNeutral},
		{"drifter", game.PawnView{Title: "Hermit", Visitor: true}, dossierview.Card{Relation: dossierview.RelationNone}, "Drifter, Hermit", ToneNeutral},
		{"visitor flagged but card says own", game.PawnView{Visitor: true, Title: "Guest"}, own, "Drifter, Guest", ToneNeutral},
	}
	for _, c := range cases {
		l := StandingLine(c.p, game.BioView{}, c.c)
		if l.Text != c.text || l.Tone != c.tone || l.Tab != TabBio {
			t.Errorf("%s: %q/%v/%s, want %q/%v", c.name, l.Text, l.Tone, l.Tab, c.text, c.tone)
		}
	}
}

func TestAnimalKinds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a    game.AnimalView
		want string
	}{
		{"wild deer", game.AnimalView{Species: "deer"}, "Wild creature"},
		{"wild wolf", game.AnimalView{Species: "timber_wolf"}, "Wild predator"},
		{"wild beetle", game.AnimalView{Species: "hive_beetle"}, "Wild insect"},
		{"tame beetle", game.AnimalView{Species: "hive_beetle", Tame: true}, "Tame insect"},
		{"mech unit", game.AnimalView{Species: "harrow", Mech: true, Tame: true}, "Mech unit"},
		{"manhunter", game.AnimalView{Species: "deer", Manhunter: true}, "Manhunter"},
		{"mech", game.AnimalView{Species: "deer", Mech: true}, "Hostile mech unit"},
		{"tame", game.AnimalView{Species: "deer", Tame: true}, "Tame animal"},
		{"tame hunter", game.AnimalView{Species: "timber_wolf", Tame: true, Hunts: true}, "Tame hunter"},
		{"farm", game.AnimalView{Species: "deer", Tame: true, Product: "milk"}, "Farm animal"},
		{"herd", game.AnimalView{Species: "deer", Tame: true, Herd: 4}, "Herd animal"},
	}
	for _, c := range cases {
		if got := AnimalKind(c.a); got != c.want {
			t.Errorf("%s: %q want %q", c.name, got, c.want)
		}
	}
	if got := AnimalKind(game.AnimalView{Species: "not_a_species"}); got != "Wild creature" {
		t.Errorf("unknown species: %q", got)
	}
	if got := AnimalKind(game.AnimalView{Species: "not_a_species", Tame: true}); got != "Tame animal" {
		t.Errorf("unknown tame species: %q", got)
	}
	l := AnimalStandingLine(game.AnimalView{Species: "timber_wolf"}, dossierview.Card{})
	if l.Tone != ToneText {
		t.Errorf("wild is plain text: %v", l.Tone)
	}
	if AnimalStandingLine(game.AnimalView{Tame: true}, dossierview.Card{}).Tone != ToneGood {
		t.Error("tame is the colony's")
	}
	if AnimalStandingLine(game.AnimalView{Hostile: true}, dossierview.Card{}).Tone != ToneCritical {
		t.Error("hostile")
	}
}

func TestIdentityLine(t *testing.T) {
	t.Parallel()
	own := dossierview.Card{}
	cases := []struct {
		doy  int
		want string
	}{
		{0, "Female, age 31, birthday today"},
		{1, "Female, age 31 and 1 day"},
		{12, "Female, age 31 and 12 days"},
	}
	for _, c := range cases {
		l := IdentityLine(game.PawnView{Sex: "female", AgeYears: 31}, game.BioView{}, dossierview.Card{AgeDayOfYear: c.doy})
		if l.Text != c.want || l.Tab != TabBio {
			t.Errorf("%d: %q", c.doy, l.Text)
		}
	}
	if got := IdentityLine(game.PawnView{AgeYears: 20}, game.BioView{}, own).Text; got != "Age 20, birthday today" {
		t.Errorf("no sex: %q", got)
	}
	a := game.AnimalView{Label: "arctic fox", Female: true, AgeDays: 60*2 + 12}
	if got := AnimalIdentityLine(a, own).Text; got != "Female arctic fox, age 2 and 12 days" {
		t.Errorf("%q", got)
	}
	a = game.AnimalView{Label: "deer", AgeDays: 1}
	if got := AnimalIdentityLine(a, own).Text; got != "Male deer, age 1 day" {
		t.Errorf("%q", got)
	}
	a.AgeDays = 7
	if got := AnimalIdentityLine(a, own).Text; got != "Male deer, age 7 days" {
		t.Errorf("%q", got)
	}
}

func TestAnimalLines(t *testing.T) {
	t.Parallel()
	if l := AnimalBodyLine(game.AnimalView{Health: 100}); l.Text != "Healthy" || l.Tone != ToneGood {
		t.Errorf("%+v", l)
	}
	if l := AnimalBodyLine(game.AnimalView{Health: 100, Downed: true}); l.Text != "Downed" {
		t.Errorf("%+v", l)
	}
	if l := AnimalBodyLine(game.AnimalView{Health: 10}); l.Tone != ToneCritical || !WarnsInWords(l) {
		t.Errorf("%+v", l)
	}
	if l := AnimalMindLine(game.AnimalView{Stunned: true}, dossierview.Card{}); l.Text != "Stunned" || l.Tone != ToneCritical {
		t.Errorf("%+v", l)
	}
	if l := AnimalMindLine(game.AnimalView{}, dossierview.Card{StunTicks: 90}); l.Text != "Stunned for 2 s" {
		t.Errorf("%+v", l)
	}
	if l := AnimalMindLine(game.AnimalView{}, dossierview.Card{}); l.Text != "" {
		t.Errorf("a calm animal has nothing to say: %+v", l)
	}
}

func TestActivityLine(t *testing.T) {
	t.Parallel()
	card := dossierview.Card{WorkType: -1, ActiveSkill: -1}
	cases := []struct {
		name string
		p    game.PawnView
		c    dossierview.Card
		want string
		tone Tone
	}{
		{"fleeing first", game.PawnView{Fleeing: true, RaidPlan: "assault", Job: "x"}, card, "Fleeing", ToneWarning},
		{"raid", game.PawnView{RaidPlan: "siege", Job: "x"}, card, "Raiding (siege)", ToneText},
		{"attack", game.PawnView{Job: "x"}, dossierview.Card{AttackTarget: "Bo"}, "Attacking Bo", ToneText},
		{"job", game.PawnView{Job: "Build wall", Acting: "construct"}, card, "Build wall", ToneText},
		{"acting", game.PawnView{Acting: "cut"}, card, "Cut", ToneText},
		{"idle", game.PawnView{}, card, "Idle", ToneOff},
	}
	for _, c := range cases {
		l := ActivityLine(c.p, c.c)
		if l.Text != c.want || l.Tone != c.tone || l.Tab != TabWork {
			t.Errorf("%s: %+v", c.name, l)
		}
	}
	tip := ActivityTip(dossierview.Card{WorkType: 4, ActiveSkill: 2})
	if JoinTip(tip) != "Work: Construction\nDraws on Construction\nPractising Construction" {
		t.Errorf("tip %q", JoinTip(tip))
	}
	if len(ActivityTip(card)) != 0 {
		t.Error("nothing to say without a work type or skill")
	}
	if l := AnimalActivityLine(game.AnimalView{State: "grazing"}, dossierview.Card{}); l.Text != "Grazing" {
		t.Errorf("%+v", l)
	}
}

func TestOtherLines(t *testing.T) {
	t.Parallel()
	if QueuedLine(game.PawnView{}).Text != "" {
		t.Error("nothing queued, nothing shown")
	}
	q := []game.QueuedOrderView{{Label: "Chop"}, {Label: "Haul"}, {Label: "Mine"}}
	if got := QueuedLine(game.PawnView{QueuedOrders: q}).Text; got != "Next: Chop (+2)" {
		t.Error(got)
	}
	if got := QueuedLine(game.PawnView{QueuedOrders: q[:1]}).Text; got != "Next: Chop" {
		t.Error(got)
	}
	if got := EquippedLine(game.PawnView{Prisoner: true, Weapon: "rifle"}).Text; got != "Held captive" {
		t.Error(got)
	}
	if got := EquippedLine(game.PawnView{Weapon: "rifle"}).Text; got != "Armed: rifle" {
		t.Error(got)
	}
	if got := EquippedLine(game.PawnView{Weapon: "fists"}).Text; got != "Unarmed" {
		t.Error(got)
	}
	if CarryingLine(game.PawnView{}).Text != "" || CarryingLine(game.PawnView{Carrying: "pawn"}).Text != "Carrying a person" ||
		CarryingLine(game.PawnView{Carrying: "wood"}).Text != "Carrying wood" {
		t.Error("carrying")
	}
	b := game.BioView{Traits: []game.TraitView{{Label: "Brave", Desc: "Does not flinch."}, {Label: "Lazy"}}}
	if l := TraitsLine(b); l.Text != "Brave, Lazy" || !strings.Contains(l.Tip, "Does not flinch.") {
		t.Errorf("%+v", l)
	}
	if TraitsLine(game.BioView{}).Text != "" {
		t.Error("no traits, no line")
	}
	pr := game.PrisonerView{Resistance: 45}
	if got := CaptiveLine(game.PawnView{Prisoner: true, GuestFaction: "Reach"}, pr, dossierview.Card{}).Text; got != "Resistance 45, from Reach" {
		t.Error(got)
	}
	if got := CaptiveLine(game.PawnView{Prisoner: true}, pr, dossierview.Card{}).Text; got != "Resistance 45, no faction" {
		t.Error(got)
	}
	if CaptiveLine(game.PawnView{}, pr, dossierview.Card{}).Text != "" {
		t.Error("only prisoners")
	}
	if l := KeeperLine(game.AnimalView{Master: "Ana", Bond: "Ana"}); l.Text != "Master: Ana, bonded" || l.Tone != ToneGood {
		t.Errorf("%+v", l)
	}
	if l := KeeperLine(game.AnimalView{Master: "Ana"}); l.Text != "Master: Ana" || l.Tone != ToneText {
		t.Errorf("%+v", l)
	}
	if l := KeeperLine(game.AnimalView{}); l.Text != "No master" || l.Tone != ToneOff {
		t.Errorf("%+v", l)
	}
	if TallyLine(game.PawnView{Kills: 3}, "kills").Text != "Kills: 3" || TallyLine(game.PawnView{DamageDealt: 9}, "damage").Text != "Damage dealt: 9" ||
		TallyLine(game.PawnView{}, "nope").Text != "" {
		t.Error("tally")
	}
	if l, v, ok := NeedValue(game.PawnView{Beauty: 70}, "beauty"); l != "Beauty" || v != 70 || !ok {
		t.Error("need value")
	}
	if _, _, ok := NeedValue(game.PawnView{}, "nope"); ok {
		t.Error("unknown need")
	}
}

func TestStatLines(t *testing.T) {
	t.Parallel()
	var c dossierview.Card
	c.Stats[0] = 1200 // work speed
	if got := StatLine(c, "work_speed", false).Text; got != "Work speed: 120%" {
		t.Error(got)
	}
	c.Stats[0] = 1205
	if got := StatLine(c, "work_speed", true).Text; got != "Work speed: 120.5%" {
		t.Error(got)
	}
	if got := StatLine(c, "work_speed", false).Text; got != "Work speed: 121%" {
		t.Error(got)
	}
	if StatLine(c, "nope", false).Text != "" {
		t.Error("unknown")
	}
	c.Stats[0] = -1
	if StatLine(c, "work_speed", false).Text != "" {
		t.Error("-1 hides the stat")
	}
	for _, d := range Defs("stat") {
		id, _ := StatByKey(d.Key)
		if FormatStat(id, 17, true) == "" {
			t.Errorf("%s formats to nothing", d.Key)
		}
	}
	if got := FormatStat(6, -55, false); got != "-5.5 °C" {
		t.Error(got)
	}
}
