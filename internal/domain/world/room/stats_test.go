package room

import "testing"

func TestSpaceBreakpoints(t *testing.T) {
	t.Parallel()
	if Space(0, 0) != 0 || Space(10, 0) != 140 || Space(10, 2) != 122 || Space(4, 9) != 20 {
		t.Fatal("space tenths")
	}
	for v, want := range map[int]string{0: "cramped", 124: "cramped", 125: "rather tight", 289: "rather tight",
		290: "average-sized", 549: "average-sized", 550: "somewhat spacious", 699: "somewhat spacious",
		700: "quite spacious", 1299: "quite spacious", 1300: "very spacious", 3494: "very spacious",
		3495: "extremely spacious"} {
		if SpaceLabel(v) != want {
			t.Fatalf("space %d = %s", v, SpaceLabel(v))
		}
	}
}

func TestBeautyAndLabels(t *testing.T) {
	t.Parallel()
	// 10 cells: divisor 25; 41 cells: divisor 41.
	if Beauty(100, 0) != 0 || Beauty(250, 10) != 10 || Beauty(410, 41) != 10 || Beauty(400, 40) != 10 || Beauty(400, 41) != 9 {
		t.Fatal("beauty divisor", Beauty(250, 10), Beauty(410, 41), Beauty(400, 40))
	}
	for v, want := range map[int]string{-36: "hideous", -35: "ugly", -1: "ugly", 0: "neutral", 23: "neutral",
		24: "pretty", 49: "pretty", 50: "beautiful", 149: "beautiful", 150: "very beautiful",
		499: "very beautiful", 500: "extremely beautiful", 1000: "unbelievably beautiful"} {
		if BeautyLabel(v) != want {
			t.Fatalf("beauty %d = %s", v, BeautyLabel(v))
		}
	}
}

func TestWealthAndCleanlinessLabels(t *testing.T) {
	t.Parallel()
	for v, want := range map[int]string{499: "impoverished", 500: "poor", 700: "modest", 2000: "comfortable",
		4000: "well-off", 10000: "rich", 40000: "very rich", 100000: "extremely rich", 1000000: "unbelievably luxurious"} {
		if WealthLabel(v) != want {
			t.Fatalf("wealth %d = %s", v, WealthLabel(v))
		}
	}
	for v, want := range map[int]string{-111: "very dirty", -110: "dirty", -41: "dirty", -40: "slightly dirty",
		-6: "slightly dirty", -5: "clean", 39: "clean", 40: "sterile"} {
		if CleanlinessLabel(v) != want {
			t.Fatalf("clean %d = %s", v, CleanlinessLabel(v))
		}
	}
}

func TestImpressivenessFormulaAndLabels(t *testing.T) {
	t.Parallel()
	// all bases 1 (curve 1): 65*1 + 35*1 = 100 points = 1000 tenths.
	if got := Impressiveness4(1500, 30, 1250, 0); got != 1000 {
		t.Fatalf("unit bases = %d", got)
	}
	// ln(e) ~ 1: wealth base e (4077 silver) with the rest at 1 -> mean (1+2+1+1... )
	if got := lnMicro(2_718_282); got < 999_900 || got > 1_000_100 {
		t.Fatalf("ln e = %d", got)
	}
	if lnMicro(micro) != 0 {
		t.Fatal("ln 1")
	}
	// zero room: terms 0,0,0,1 (cleanliness 0 -> base 1): sum 1, min 0 -> 65/4 = 16.25 pts
	if got := Impressiveness4(0, 0, 0, 0); got != 162 {
		t.Fatalf("empty room = %d", got)
	}
	// a filthy ugly empty room goes negative and reads awful
	if got := Impressiveness4(0, -300, 0, -250); got >= 0 || ImpressivenessLabel(got) != "awful" {
		t.Fatalf("squalor = %d", got)
	}
	for v, want := range map[int]string{0: "awful", 199: "awful", 200: "dull", 300: "mediocre", 400: "decent",
		500: "slightly impressive", 650: "somewhat impressive", 850: "very impressive",
		1200: "extremely impressive", 1700: "unbelievably impressive", 2400: "wondrously impressive"} {
		if ImpressivenessLabel(v) != want {
			t.Fatalf("imp %d = %s", v, ImpressivenessLabel(v))
		}
	}
	st := StatsOf(Inputs{Cells: 10, Impassable: 2, BeautySumTenths: 250, Wealth: 300, CleanlinessHundredths: 40})
	if st.SpaceTenths != 122 || st.BeautyTenths != 10 || st.Wealth != 300 || st.ImpressivenessTenths != Impressiveness4(300, 10, 122, 40) {
		t.Fatalf("stats %+v", st)
	}
}

func TestRoleOfFull(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		c    FullContents
		want Role
	}{
		"hospital":         {FullContents{MedicalBeds: 1}, Hospital},
		"hospital>bed":     {FullContents{Contents: Contents{Beds: 1}, MedicalBeds: 1}, Hospital},
		"prison cell":      {FullContents{PrisonerBeds: 1}, PrisonCell},
		"prison barrack":   {FullContents{PrisonerBeds: 3}, PrisonBarracks},
		"prison>hosp":      {FullContents{PrisonerBeds: 1, MedicalBeds: 1}, PrisonCell},
		"kitchen":          {FullContents{Stoves: 1}, Kitchen},
		"rec":              {FullContents{RecBuildings: 1}, RecRoom},
		"dining beats rec": {FullContents{Contents: Contents{Tables: 1}, RecBuildings: 1}, Dining},
		"rec beats dining": {FullContents{Contents: Contents{Tables: 1}, RecBuildings: 2}, RecRoom},
		"rec+bed":          {FullContents{Contents: Contents{Beds: 1}, RecBuildings: 1}, Bedroom},
		"rec+bench":        {FullContents{Contents: Contents{Benches: 1}, RecBuildings: 1}, Workshop},
		"stove+bed":        {FullContents{Contents: Contents{Beds: 1}, Stoves: 1}, Bedroom},
		"plain dining":     {FullContents{Contents: Contents{Tables: 1}}, Dining},
		"empty":            {FullContents{}, General},
	} {
		if got := RoleOfFull(tc.c); got != tc.want {
			t.Errorf("%s: %v want %v", name, got, tc.want)
		}
	}
	if Hospital.String() != "hospital" || Kitchen.String() != "kitchen" || Barracks.String() != "barracks" {
		t.Fatal("role names")
	}
}
