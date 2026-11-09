package gfx

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func withScale(t *testing.T, s Scale) {
	t.Helper()
	old := CurrentScale()
	SetScale(s)
	t.Cleanup(func() { SetScale(old) })
}

func TestRunsKeepEachNumberAndItsMetrics(t *testing.T) {
	var c Runs
	a, b := c.Int(7, Small), c.Int(1234, Small)
	if a == b || c.Int(7, Small) != a {
		t.Fatal("a repeated number is the same run, another number another run")
	}
	if a.s != "7" || b.s != "1234" || !(b.W > a.W) {
		t.Fatalf("runs %q %q widths %v %v", a.s, b.s, a.W, b.W)
	}
	if a.W != TextW("7", Small) || a.Step != LineStep(Small) {
		t.Fatal("a run's metrics are TextW and LineStep")
	}
}

func TestRunDrawsTheNewNumberAfterACountChange(t *testing.T) {
	var c Runs
	var got []string
	SetDrawTrace(func(e TraceEvent) { got = append(got, e.Text) })
	t.Cleanup(func() { SetDrawTrace(nil) })
	dst := ebiten.NewImage(64, 32)
	for _, n := range []int{12, 12, 13} {
		c.Int(n, Small).Draw(dst, 2, 2, Small)
	}
	if len(got) != 3 || got[0] != "12" || got[2] != "13" {
		t.Fatalf("traced text %v", got)
	}
}

func TestRunsInvalidateOnAScaleChange(t *testing.T) {
	var c Runs
	one := c.Int(42, Small)
	withScale(t, Scale{Dev: 2, UI: 2, S: 2, World: 1})
	two := c.Int(42, Small)
	if one == two || two.W == 0 {
		t.Fatal("a UI scale change renders the number again")
	}
	if two.img == nil || one.img == nil || two.img.Bounds().Dx() <= one.img.Bounds().Dx() {
		t.Fatal("the run at 2x is a larger image")
	}
	SetScale(UnitScale)
	if c.Int(42, Small) != one {
		t.Fatal("the unscaled run is still cached")
	}
	SetLargerSmall(true)
	t.Cleanup(func() { SetLargerSmall(false) })
	if c.Int(42, Small) == one {
		t.Fatal("larger small text renders the number again")
	}
	SetLargerSmall(false)
	ForgetFonts()
	if c.Int(42, Small) == one {
		t.Fatal("a font reset renders the number again")
	}
}

func TestRunsStayWithinTheirCap(t *testing.T) {
	var c Runs
	for n := 0; n < 3*RunCap; n++ {
		c.Int(n, Small)
		if c.Len() > RunCap {
			t.Fatalf("%d runs after %d numbers", c.Len(), n+1)
		}
	}
	last := c.Int(3*RunCap-1, Small)
	if c.Int(3*RunCap-1, Small) != last {
		t.Fatal("the newest number is kept")
	}
	// a number drawn every frame survives the churn of the others
	hot := c.Int(5, Small)
	for n := 10_000; n < 10_000+2*RunCap; n++ {
		c.Int(n, Small)
		if c.Int(5, Small) != hot {
			t.Fatal("the least recently drawn run goes first")
		}
	}
}

func TestRunsSteadyStateAllocatesNothing(t *testing.T) {
	var c Runs
	for n := 2; n < 80; n++ {
		c.Int(n, Small)
	}
	if a := testing.AllocsPerRun(50, func() {
		for n := 2; n < 80; n++ {
			c.Int(n, Small)
		}
	}); a != 0 {
		t.Fatalf("%v allocations looking up cached runs", a)
	}
}
