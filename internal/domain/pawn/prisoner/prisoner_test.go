package prisoner

import "testing"

func TestResistanceFactor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		op   int32
		mood int
		want int32
	}{
		{0, 50, 1000}, {-10000, 50, 500}, {10000, 50, 1500},
		{0, 0, 200}, {0, 100, 1500}, {0, 25, 600}, {0, 75, 1250},
		{99999, 500, 2250}, {-99999, -5, 100},
	}
	for _, c := range cases {
		if got := ResistanceFactor(c.op, c.mood); got != c.want {
			t.Errorf("(%d,%d)=%d want %d", c.op, c.mood, got, c.want)
		}
	}
}

func TestReductionAndInitial(t *testing.T) {
	t.Parallel()
	if got := Reduction(1000, 1000); got != 1000 {
		t.Fatalf("%d", got)
	}
	if got := Reduction(1500, 500); got != 750 {
		t.Fatalf("%d", got)
	}
	if InitialResistance(0) != 10000 || InitialResistance(1000) != 30000 || InitialResistance(500) != 20000 {
		t.Fatal("initial")
	}
	if InitialResistance(-5) != 10000 || InitialResistance(5000) != 30000 {
		t.Fatal("clamp")
	}
}

func TestRecruitChance(t *testing.T) {
	t.Parallel()
	if RecruitChance(0) != 150 || RecruitChance(10) != 600 || RecruitChance(20) != 1000 {
		t.Fatal("curve")
	}
	if RecruitChance(-3) != 150 || RecruitChance(99) != 1000 {
		t.Fatal("clamp")
	}
	if VisitInterval != 15000 {
		t.Fatal("interval")
	}
}
