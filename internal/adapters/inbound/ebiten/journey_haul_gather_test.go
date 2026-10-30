package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/domain/economy/item"
)

// TestJourneyHaulerGathersNearbyPiles (HAUL-002): a grid of 12-wood piles
// from chopped trees and a stockpile painted with the Architect tool; the
// colonist sweeps several piles into one load before delivering, and the
// read model shows it carrying wood in a load of at least four piles.
func TestJourneyHaulerGathersNearbyPiles(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	for y := 12; y <= 14; y++ {
		for x := 12; x <= 15; x++ {
			if _, err := j.g.SpawnStack(item.Wood, 12, x, y); err != nil {
				t.Fatal(err)
			}
		}
	}
	j.openArch("zones", "stockpile")
	j.dragTiles(inputframe.Left, 8, 1, 9, 2, 0)
	j.key("Escape", 0)
	j.key("Digit3", 0)
	most, carrier := 0, ""
	for i := 0; i < 8000 && most < 48; i++ {
		careFeedHostileFree(j)
		j.update(1)
		for _, it := range j.world().Items {
			if it.Carried && it.Kind == item.Wood.String() && it.Qty > most {
				most = it.Qty
				for _, p := range j.g.Snapshot() {
					if p.Carrying != "" {
						carrier = p.Carrying
					}
				}
			}
		}
	}
	j.key("Space", 0)
	if most < 48 || carrier != "wood" {
		t.Fatalf("the biggest load carried was %d wood (pawn shows %q), want at least 48 wood", most, carrier)
	}
}
