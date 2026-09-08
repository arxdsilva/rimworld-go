package ebiten

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

func TestSkyOverlayNightDarkerThanNoon(t *testing.T) {
	t.Parallel()
	night := skyOverlay(2 * clock.TicksPerHour)
	noon := skyOverlay(12 * clock.TicksPerHour)
	if night.A <= noon.A {
		t.Fatalf("night alpha %d should exceed noon %d", night.A, noon.A)
	}
	if noon.A != 0 {
		t.Fatalf("noon overlay should be clear, alpha=%d", noon.A)
	}
	if night.A > skyOverlayMaxAlpha {
		t.Fatalf("night alpha %d exceeds soft cap %d", night.A, skyOverlayMaxAlpha)
	}
}

func TestSkyOverlayMorningIsWarm(t *testing.T) {
	t.Parallel()
	morning := skyOverlay(clock.StartingMorning())
	if morning.R <= morning.B {
		t.Fatalf("morning tint should be warmer than it is blue: %+v", morning)
	}
}
