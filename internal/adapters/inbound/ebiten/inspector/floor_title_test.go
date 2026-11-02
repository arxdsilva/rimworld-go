package inspector

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/axview"
)

// TestFloorTitlesAndLines (AX-3): floor titles name the stone or the
// colour, and a floor's pane lines carry its room stats and colour.
func TestFloorTitlesAndLines(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		b    game.BuildingView
		want string
	}{
		{game.BuildingView{Kind: "stone tile", Stuff: "granite blocks"}, "Granite tile"},
		{game.BuildingView{Kind: "flagstone", Stuff: "slate blocks"}, "Slate flagstone"},
		{game.BuildingView{Kind: "carpet floor", Stuff: "cloth", AX: axview.Building{Colour: "powder blue"}}, "Powder blue carpet floor"},
		{game.BuildingView{Kind: "sterile tile", Stuff: "steel"}, "Sterile tile"},
		{game.BuildingView{Kind: "stone tile", Stuff: "marble blocks", Ruin: true}, "Ruined marble tile"},
		{game.BuildingView{Kind: "floor", Stuff: "wood"}, "Floor"},
	} {
		if got := BuildingTitle(c.b); got != c.want {
			t.Errorf("%s of %s: %q, want %q", c.b.Kind, c.b.Stuff, got, c.want)
		}
	}
	carpet := game.BuildingView{Kind: "carpet floor", Stuff: "cloth", AX: axview.Building{Colour: "mint"}}
	if l := FloorLines(carpet); len(l) != 2 || l[0] != "Beauty +2   Cleanliness 0" || l[1] != "Colour: Mint" {
		t.Fatalf("carpet floor lines %q", l)
	}
	if FloorLines(game.BuildingView{Kind: "wall", Stuff: "wood"}) != nil {
		t.Fatal("a wall has no floor lines")
	}
}
