package room

import (
	"testing"

	"rimworld-go/internal/domain/world/filth"
)

func TestTileFloorAndFilthBeauty(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		f       Floor
		indoors bool
		want    int
	}{
		{Bare, true, -10}, {Bare, false, 0}, {Smooth, false, 10}, {WoodFloor, true, 0},
		{StoneTile, true, 10}, {CarpetFloor, false, 20}, {Floor(99), true, 0},
	} {
		if got := FloorBeautyTenths(c.f, c.indoors); got != c.want {
			t.Errorf("FloorBeautyTenths(%d, %v) = %d, want %d", c.f, c.indoors, got, c.want)
		}
	}
	if FilthBeautyTenths(filth.Full, true) != -150 || FilthBeautyTenths(filth.Full, false) != -45 || FilthBeautyTenths(0, true) != 0 {
		t.Error("filth beauty: -15 a unit indoors, 30 % outside")
	}
}
