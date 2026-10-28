package creature

import (
	"math"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestWorkSwingAndStrike(t *testing.T) {
	t.Parallel()
	_, st, ok := ActionOf(game.PawnView{Acting: "cut"}) // old Acting strings still map
	if !ok || st.Parts[0].Kind != PartChip || st.Motion != MotionSwing {
		t.Fatal("cutting swings and throws chips")
	}
	// the body winds back, drives in hard on the strike at strikeAt, then
	// eases out
	if k := WorkSwing(float64(tickAt(7, st.Period, 0.30)), 7, st.Period); k >= 0 {
		t.Fatalf("wound back before the strike: %v", k)
	}
	if k := WorkSwing(float64(tickAt(7, st.Period, StrikeAt+0.01)), 7, st.Period); k < 0.8 {
		t.Fatalf("driven in on the strike: %v", k)
	}
	strikes, peak := 0, 0.0
	for f := 0.0; f < float64(st.Period)*3; f++ {
		if WorkStrikeAt(f, f+1, 7, st.Period) {
			strikes++
		}
		peak = math.Max(peak, WorkSwing(f, 7, st.Period))
	}
	if strikes != 3 || peak < 0.99 {
		t.Fatalf("one strike per swing (%d), swing reaches 1 (%v)", strikes, peak)
	}
	if _, _, ok := ActionOf(game.PawnView{}); ok {
		t.Fatal("no style when idle")
	}
}

func TestCremateUsesBench(t *testing.T) {
	t.Parallel()
	d, ok := WorkTable[game.ActionCremate]
	if !ok || d.Motion != MotionBench {
		t.Fatalf("cremate row: %+v", d)
	}
	hasFx := false
	for _, p := range d.Parts {
		if p.Kind == PartEmber || p.Kind == PartSmoke {
			hasFx = true
		}
	}
	if !hasFx {
		t.Fatalf("embers or smoke: %+v", d.Parts)
	}
	for _, a := range []string{game.ActionDig, game.ActionBury, game.ActionCremate} {
		if d := WorkTable[a]; d.Motion == MotionNone || d.Period <= 0 {
			t.Fatalf("%s timing: %+v", a, d)
		}
	}
}

// tickAt is a tick at which pawn id's swing of period ticks is at phase
// ph (work_anim_test.go's twin).
func tickAt(id uint64, period int, ph float64) int {
	want := int(ph*float64(period)) + 1
	return ((want-int(id*13))%period+period)%period + 10*period
}
