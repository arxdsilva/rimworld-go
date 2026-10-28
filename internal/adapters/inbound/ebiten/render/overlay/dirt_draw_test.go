package overlay

import (
	"math"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestDirtAlphaByAmount(t *testing.T) {
	t.Parallel()
	for _, a := range []float64{0, 0.1, 0.5, 1} {
		l := DirtLookOf(game.DirtView{X: 3, Y: 4, Amount: a})
		if want := 0.15 + 0.6*a; math.Abs(float64(l.Cs[3])-want) > 1e-6 {
			t.Fatalf("amount %v: alpha %v, want %v", a, l.Cs[3], want)
		}
	}
	if l := DirtLookOf(game.DirtView{Amount: 4}); l.Cs[3] > 0.75+1e-6 {
		t.Fatalf("alpha capped at 0.75: %v", l.Cs[3])
	}
	if l := DirtLookOf(game.DirtView{Amount: 0.2}); DirtLookOf(game.DirtView{Amount: 0.9}).Cs[3] <= l.Cs[3] {
		t.Fatal("dirtier is more opaque")
	}
}

func TestDirtColourLightToHeavy(t *testing.T) {
	t.Parallel()
	if got := rgb8(DirtColour(0)); got != [3]int{0x6b, 0x52, 0x38} {
		t.Fatalf("light dirt %x", got)
	}
	if got := rgb8(DirtColour(1)); got != [3]int{0x4a, 0x38, 0x26} {
		t.Fatalf("heavy dirt %x", got)
	}
	if mid := DirtColour(0.5); !(mid[0] < DirtLight[0] && mid[0] > DirtHeavy[0]) {
		t.Fatalf("in between: %v", mid)
	}
}

func TestDirtOrientedByTileHash(t *testing.T) {
	t.Parallel()
	a := DirtLookOf(game.DirtView{X: 5, Y: 6, Amount: 0.1})
	if b := DirtLookOf(game.DirtView{X: 5, Y: 6, Amount: 0.1}); a != b {
		t.Fatal("the same tile always looks the same")
	}
	rots := map[float64]bool{}
	for x := 0; x < 20; x++ {
		l := DirtLookOf(game.DirtView{X: x, Y: 2, Amount: 0.1})
		rots[l.Rot] = true
		if l.Rot < 0 || l.Rot >= 2*math.Pi {
			t.Fatalf("rotation %v", l.Rot)
		}
		if math.Abs(l.Cx-float64(x)-0.5) > 0.08+1e-9 || math.Abs(l.Cy-2.5) > 0.08+1e-9 {
			t.Fatalf("offset leaves the tile centre: %+v", l)
		}
	}
	if len(rots) < 10 {
		t.Fatalf("tiles should face different ways, got %d headings", len(rots))
	}
}
