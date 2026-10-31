package plant

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// TestFalloutKillsOutdoorPlants (FOOD-6, wiki: Toxic fallout): a crop
// dies in about an hour and a half (57.5 ±15 % of 85 HP an hour), a tree
// in three to four and a half.
func TestFalloutKillsOutdoorPlants(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		s          Species
		minH, maxH int
	}{{Potato, 1, 2}, {Tree, 3, 5}} {
		for x := 0; x < 20; x++ {
			p, h := Plant{Species: c.s, X: x, Y: 3}, 0
			for ; !p.Dead() && h < 24; h++ {
				p = p.Fallout(uint64(h)*clock.TicksPerHour, uint64(h+1)*clock.TicksPerHour)
			}
			if h < c.minH || h > c.maxH {
				t.Fatalf("%s at %d: dead after %d hours, want %d-%d", c.s, x, h, c.minH, c.maxH)
			}
		}
	}
	if r := FalloutRate(Plant{Species: Potato}); r < 575 || r > 778 {
		t.Fatalf("crop rate %d, want 575-778 (48.9-66.1 of 85 HP)", r)
	}
}
