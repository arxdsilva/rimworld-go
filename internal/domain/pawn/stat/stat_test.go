package stat

import "testing"

func TestBase(t *testing.T) {
	b := Base()
	for id := ID(0); id < NumStats; id++ {
		if id.Label() == "" {
			t.Fatalf("stat %d has no label", id)
		}
	}
	if b[WorkSpeed] != 1000 || b[RecreationRate] != 1000 || b[MoodOffset] != 0 || b[PainTolerance] != 0 {
		t.Fatalf("base %v", b)
	}
	if NumStats.Label() != "" {
		t.Fatal("out of range label")
	}
}

// TestStatSheetOrder: within a mod the multiplication comes first; mods
// and layers apply in the order given, which changes the result.
func TestStatSheetOrder(t *testing.T) {
	s := Base().Apply(Mod{Stat: WorkSpeed, Mul: 1200, Add: 100})
	if s[WorkSpeed] != 1300 { // 1000*1.2 + 100, not (1000+100)*1.2
		t.Fatalf("mul then add: %d", s[WorkSpeed])
	}
	mulFirst := Build(Base(), []Mod{{Stat: MoveSpeed, Mul: 500}}, []Mod{{Stat: MoveSpeed, Add: 200}})
	addFirst := Build(Base(), []Mod{{Stat: MoveSpeed, Add: 200}}, []Mod{{Stat: MoveSpeed, Mul: 500}})
	if mulFirst[MoveSpeed] != 700 || addFirst[MoveSpeed] != 600 {
		t.Fatalf("layer order: %d %d", mulFirst[MoveSpeed], addFirst[MoveSpeed])
	}
	// body -> age -> condition -> traits -> difficulty, chained exactly.
	body := []Mod{{Stat: HungerRate, Mul: 900}}
	age := []Mod{{Stat: WorkSpeed, Mul: 850}}
	cond := []Mod{{Stat: MoveSpeed, Mul: 920}}
	traits := []Mod{{Stat: WorkSpeed, Mul: 1200}, {Stat: MoodOffset, Add: 8}}
	diff := []Mod{{Stat: MoodOffset, Add: -5}, {Stat: PainTolerance, Add: 10}}
	got := Build(Base(), body, age, cond, traits, diff)
	if got[HungerRate] != 900 || got[WorkSpeed] != 1020 || got[MoveSpeed] != 920 ||
		got[MoodOffset] != 3 || got[PainTolerance] != 10 {
		t.Fatalf("chain %v", got)
	}
	if Base().Apply(Mod{Stat: NumStats, Add: 5}) != Base() {
		t.Fatal("out of range stat applied")
	}
	if Build(Base()) != Base() {
		t.Fatal("no layers changed the sheet")
	}
}

func TestFormat(t *testing.T) {
	cases := map[Mod]string{
		{Stat: WorkSpeed, Mul: 1200}:     "Work speed +20%",
		{Stat: MoveSpeed, Mul: 880}:      "Move speed -12%",
		{Stat: MoveSpeed, Mul: 1025}:     "Move speed +2.5%",
		{Stat: MoodOffset, Add: -8}:      "Mood -8",
		{Stat: ColdComfort, Add: -60}:    "Cold comfort -6 °C",
		{Stat: ColdComfort, Add: 25}:     "Cold comfort +2.5 °C",
		{Stat: PainTolerance, Add: 15}:   "Pain tolerance +15%",
		{Stat: ShootAcc, Add: 12}:        "Shooting accuracy +12",
		{Stat: HungerRate, Add: 50}:      "Hunger rate +5%",
		{Stat: NumStats, Add: 1}:         "",
		{Stat: AimCooldown, Mul: 750}:    "Aim time -25%",
		{Stat: RecreationRate, Mul: 850}: "Recreation drain -15%",
	}
	for m, want := range cases {
		if got := Format(m); got != want {
			t.Fatalf("Format(%+v) = %q want %q", m, got, want)
		}
	}
}

func TestFurnitureStats(t *testing.T) {
	t.Parallel()
	b := Base()
	if b[Comfort] != 0 || b[Beauty] != 0 || b[RestEffectiveness] != 1000 {
		t.Fatalf("furniture stat defaults %v %v %v", b[Comfort], b[Beauty], b[RestEffectiveness])
	}
	for id, want := range map[ID]string{Comfort: "Comfort", RestEffectiveness: "Rest effectiveness", Beauty: "Beauty"} {
		if id.Label() != want {
			t.Fatalf("label %d = %q", id, id.Label())
		}
	}
	if ArrestSuccess+1 != Comfort || Comfort+1 != RestEffectiveness || RestEffectiveness+1 != Beauty || Beauty+1 != NumStats {
		t.Fatal("append-only order")
	}
}
