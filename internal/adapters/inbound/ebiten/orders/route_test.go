package orders

import (
	"image/color"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

func TestRouteTintPremultiplies(t *testing.T) {
	t.Parallel()
	for _, c := range []color.RGBA{uikit.Accent, uikit.Btn} {
		got := RouteTint(c, 200)
		if got.A != 200 || got.R > got.A || got.G > got.A || got.B > got.A {
			t.Errorf("RouteTint(%v, 200) = %v, want a premultiplied colour", c, got)
		}
	}
	if got := RouteTint(uikit.Accent, 255); got != uikit.Accent {
		t.Errorf("opaque tint = %v", got)
	}
}

func TestSqrt32(t *testing.T) {
	t.Parallel()
	if got := sqrt32(144); got < 11.99 || got > 12.01 {
		t.Fatalf("sqrt32(144) = %v", got)
	}
}
