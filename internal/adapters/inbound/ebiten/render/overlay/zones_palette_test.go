package overlay

import "testing"

func TestZoneColourIsDeterministicPerID(t *testing.T) {
	t.Parallel()
	for id := uint16(0); id < 40; id++ {
		first, second := ZoneColour(id), ZoneColour(id)
		if first != second || ZoneColour(id) != ZonePalette[int(id)%len(ZonePalette)] {
			t.Fatalf("zone %d colour not stable", id)
		}
	}
	if ZoneColour(1) == ZoneColour(2) {
		t.Error("neighbouring zones share a colour")
	}
}
