package biome

import (
	"testing"

	"rimworld-go/internal/domain/world/plant"
)

// TestGroundPlantsPlaced: the PLAY-010 wild ground plants grow where the
// plan puts them, at its permille.
func TestGroundPlantsPlaced(t *testing.T) {
	t.Parallel()
	want := map[ID][]PlantDensity{
		TemperateForest:    {{plant.Bramble, 12}, {plant.Dandelion, 15}},
		TemperateSwamp:     {{plant.Bramble, 15}, {plant.Moss, 20}},
		BorealForest:       {{plant.Moss, 30}, {plant.Bramble, 8}},
		Tundra:             {{plant.Moss, 25}},
		ColdBog:            {{plant.Moss, 35}},
		TropicalRainforest: {{plant.ElephantEar, 25}, {plant.FireLily, 10}, {plant.Bramble, 10}},
		TropicalSwamp:      {{plant.ElephantEar, 20}, {plant.Moss, 10}},
		AridShrubland:      {{plant.Agave, 15}, {plant.BallCactus, 10}},
		Desert:             {{plant.Agave, 10}, {plant.BallCactus, 12}},
		ExtremeDesert:      {{plant.BallCactus, 5}},
	}
	for _, d := range All() {
		have := map[plant.Species]int{}
		for _, pd := range d.Plants {
			have[pd.Species] += pd.Permil
		}
		for _, pd := range want[d.ID] {
			if have[pd.Species] != pd.Permil {
				t.Errorf("%s: %s %d permille, want %d", d.Key, pd.Species, have[pd.Species], pd.Permil)
			}
		}
		for s := range have {
			if plant.DefOf(s).Ground && !wanted(want[d.ID], s) {
				t.Errorf("%s: unexpected ground plant %s", d.Key, s)
			}
		}
		// new plants go after the old ones: their old rolls keep their odds
		seenGround := false
		for _, pd := range d.Plants {
			g := plant.DefOf(pd.Species).Ground
			if seenGround && !g {
				t.Errorf("%s: %s listed after a ground plant", d.Key, pd.Species)
			}
			seenGround = seenGround || g
		}
	}
}

func wanted(l []PlantDensity, s plant.Species) bool {
	for _, pd := range l {
		if pd.Species == s {
			return true
		}
	}
	return false
}
