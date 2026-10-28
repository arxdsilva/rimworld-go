package light

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

func TestSkyOverlayNightDarkerThanNoon(t *testing.T) {
	t.Parallel()
	night := SkyOverlay(2 * clock.TicksPerHour)
	noon := SkyOverlay(12 * clock.TicksPerHour)
	if night.A <= noon.A {
		t.Fatalf("night alpha %d should exceed noon %d", night.A, noon.A)
	}
	if noon.A != 0 {
		t.Fatalf("noon overlay should be clear, alpha=%d", noon.A)
	}
	if night.A > SkyOverlayMaxAlpha {
		t.Fatalf("night alpha %d exceeds soft cap %d", night.A, SkyOverlayMaxAlpha)
	}
}

func TestSkyOverlayDayIsClear(t *testing.T) {
	t.Parallel()
	morning := SkyOverlay(clock.StartingMorning())
	if morning.A != 0 {
		t.Fatalf("morning overlay should be clear, got %+v", morning)
	}
	noon := SkyOverlay(12 * clock.TicksPerHour)
	if noon.A != 0 {
		t.Fatalf("noon overlay should be clear, got %+v", noon)
	}
	afternoon := SkyOverlay(16 * clock.TicksPerHour)
	if afternoon.A != 0 {
		t.Fatalf("afternoon overlay should be clear, got %+v", afternoon)
	}
}

func TestSkyOverlayEveningHasAlpha(t *testing.T) {
	t.Parallel()
	evening := SkyOverlay(17 * clock.TicksPerHour)
	if evening.A == 0 {
		t.Fatalf("evening overlay should veil the map, got %+v", evening)
	}
}
