package needs

import "testing"

func TestLevelPercent(t *testing.T) {
	for _, c := range []struct {
		level int32
		want  int
	}{{-5, 0}, {0, 0}, {LevelPct - 1, 0}, {LevelPct, 1}, {LevelFull / 2, 50}, {LevelFull, 100}, {LevelFull + 9, 100}} {
		if got := LevelPercent(c.level); got != c.want {
			t.Errorf("LevelPercent(%d) = %d, want %d", c.level, got, c.want)
		}
	}
}

func TestLadderStagesFollowTheWiki(t *testing.T) {
	cases := []struct {
		name  string
		l     *Ladder
		pct   []int32 // a level in each stage, percent
		value []int
	}{
		{"comfort", &ComfortLadder, []int32{5, 30, 65, 75, 85, 95}, []int{-3, 0, 4, 6, 8, 10}},
		{"beauty", &BeautyLadder, []int32{10, 20, 35, 50, 60, 80, 90}, []int{-15, -10, -5, 0, 5, 10, 15}},
		{"outdoors", &OutdoorsLadder, []int32{0, 10, 30, 50, 70, 90}, []int{-9, -7, -5, -3, -1, 0}},
	}
	for _, c := range cases {
		if c.l.Stages() != len(c.value) || len(c.l.Value) != len(c.value) || len(c.l.Label) != len(c.value) {
			t.Fatalf("%s: %d stages, %d values, %d labels", c.name, c.l.Stages(), len(c.l.Value), len(c.l.Label))
		}
		for i, p := range c.pct {
			s := c.l.Stage(-1, p*LevelPct)
			if s != i || c.l.Value[s] != c.value[i] {
				t.Errorf("%s at %d%%: stage %d value %d, want %d %d", c.name, p, s, c.l.Value[s], i, c.value[i])
			}
			if (c.l.Value[s] == 0) != (c.l.Label[s] == "") {
				t.Errorf("%s stage %d: value %d with label %q", c.name, s, c.l.Value[s], c.l.Label[s])
			}
		}
	}
	// outdoors: at or below 0.05 % is entombed, just above is the next rung
	if OutdoorsLadder.Stage(-1, LevelPct/20-1) != 0 || OutdoorsLadder.Stage(-1, LevelPct/20) != 1 {
		t.Error("outdoors bottom boundary")
	}
}

// TestLadderDoesNotFlap is the no-flap test for every need thought: a
// level wobbling on a boundary keeps its stage; it climbs only past the
// margin and falls as soon as it drops below.
func TestLadderDoesNotFlap(t *testing.T) {
	for _, l := range []*Ladder{&ComfortLadder, &BeautyLadder, &OutdoorsLadder} {
		for b, up := range l.Up {
			stage := b // just below boundary b
			changes := 0
			for i := range 200 {
				level := up - LevelPct/2
				if i%2 == 1 {
					level = up + LevelPct // wobbles within the margin
				}
				if s := l.Stage(stage, level); s != stage {
					stage, changes = s, changes+1
				}
			}
			if changes > 1 {
				t.Errorf("boundary %d (%d): %d stage changes while wobbling", b, up, changes)
			}
			// clearing the margin climbs; any drop below falls
			if s := l.Stage(b, min(up+StageMargin, LevelFull)); s != b+1 {
				t.Errorf("boundary %d: past the margin stage %d, want %d", b, s, b+1)
			}
			if s := l.Stage(b+1, up-1); s != b {
				t.Errorf("boundary %d: below it stage %d, want %d", b, s, b)
			}
		}
	}
	// out-of-range stages take the raw stage; a big rise climbs several
	if ComfortLadder.Stage(99, 95*LevelPct) != 5 || ComfortLadder.Stage(0, 95*LevelPct) != 5 {
		t.Error("stage from out of range or far below")
	}
}

func TestComfortStep(t *testing.T) {
	half := LevelFull / 2
	if got := ComfortStep(half, 0, false); got != half-ComfortFall {
		t.Errorf("standing: %d", got)
	}
	if got := ComfortStep(0, 0, false); got != 0 {
		t.Errorf("floor: %d", got)
	}
	// a stool (0.5) fills to 50 % and no further; a better seat climbs
	// 3.6 % a step; sitting above the seat's comfort falls back to it
	if got := ComfortStep(48*LevelPct, 500, true); got != 50*LevelPct {
		t.Errorf("stool cap: %d", got)
	}
	if got := ComfortStep(10*LevelPct, 800, true); got != 10*LevelPct+ComfortGain {
		t.Errorf("armchair gain: %d", got)
	}
	if got := ComfortStep(50*LevelPct+LevelPct/10, 500, true); got != 50*LevelPct {
		t.Errorf("above the seat: %d", got)
	}
	if got := ComfortStep(80*LevelPct, 500, true); got != 80*LevelPct-ComfortFall {
		t.Errorf("well above the seat: %d", got)
	}
	// wiki: 60 % an hour up, 4 % an hour down
	if ComfortGain*2500/StepTicks/LevelPct != 60 || ComfortFall*2500/StepTicks/LevelPct != 4 {
		t.Error("comfort rates")
	}
}

func TestOutdoorsStep(t *testing.T) {
	cases := []struct {
		indoors, roofed, thick, bed bool
		want                        int32
	}{
		{false, false, false, false, 20_000},
		{false, true, false, false, 2_500},
		{false, true, true, false, -1_000},
		{false, true, true, true, -200},
		{true, false, false, false, 12_500},
		{true, false, false, true, 12_500}, // gains are not scaled in bed
		{true, true, false, false, -800},
		{true, true, false, true, -160},
		{true, true, true, false, -1_125},
		{true, true, true, true, -225},
	}
	for _, c := range cases {
		p := PlaceOf(c.indoors, c.roofed, c.thick)
		if got := OutdoorsStep(LevelFull/2, p, c.bed) - LevelFull/2; got != c.want {
			t.Errorf("%+v (place %d): %d, want %d", c, p, got, c.want)
		}
	}
	if OutdoorsStep(LevelFull, OutsideOpen, false) != LevelFull || OutdoorsStep(0, IndoorMountain, false) != 0 {
		t.Error("clamp")
	}
}

func TestBeautyTargetAndStep(t *testing.T) {
	for _, c := range []struct {
		tenths int
		want   int32
	}{{0, 50 * LevelPct}, {10, 60 * LevelPct}, {-15, 35 * LevelPct}, {200, LevelFull}, {-200, 0}} {
		if got := BeautyTarget(c.tenths); got != c.want {
			t.Errorf("BeautyTarget(%d) = %d, want %d", c.tenths, got, c.want)
		}
	}
	if BeautyStep(0, LevelFull) != BeautyDrift || BeautyStep(LevelFull, 0) != LevelFull-BeautyDrift ||
		BeautyStep(50*LevelPct, 55*LevelPct) != 55*LevelPct || BeautyStep(50*LevelPct, 45*LevelPct) != 45*LevelPct {
		t.Error("beauty drift")
	}
}
