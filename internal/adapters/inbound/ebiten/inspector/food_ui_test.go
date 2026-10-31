package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestFoodPlantAndGrowLines(t *testing.T) {
	t.Parallel()
	got := strings.Join(FoodPlantLines(game.PlantView{Fertility: 280, Light: 90, Lifespan: 40, Blighted: true}), "|")
	for _, want := range []string{"Fertility 280% (hydroponics)", "Growing light 90%", "Lifespan 40% used", "Blighted"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing from %q", want, got)
		}
	}
	if len(FoodPlantLines(game.PlantView{})) != 0 {
		t.Error("unfilled view, no lines")
	}
	b := game.BuildingView{Kind: BasinKind, X: 4, Y: 4, W: 1, H: 4}
	ps := []game.PlantView{{X: 4, Y: 5, Species: "rice", Growth: 40}, {X: 9, Y: 9, Species: "rice"}}
	if l := strings.Join(GrowLines(b, ps), "|"); !strings.Contains(l, "Plants: 1 of 4 cells") || !strings.Contains(l, "Rice: 40%") {
		t.Errorf("basin lines %q", l)
	}
	lamp := GrowLines(game.BuildingView{Kind: SunLampKind, Glow: true, GlowRadius: 11}, nil)
	if len(lamp) == 0 || !strings.Contains(lamp[0], "shining") || !strings.Contains(lamp[1], "06:00 to 19:12") {
		t.Errorf("lamp lines %q", lamp)
	}
	if GrowLines(game.BuildingView{Kind: "wall"}, nil) != nil {
		t.Error("other buildings get none")
	}
}

func TestCutBlightCmdsMarkOnlyUnmarkedBlight(t *testing.T) {
	t.Parallel()
	cmds := CutBlightCmds([]game.PlantView{{X: 1, Y: 1, Blighted: true}, {X: 2, Y: 2, Blighted: true, Cut: true}, {X: 3, Y: 3}})
	if len(cmds) != 1 || cmds[0].X != 1 || cmds[0].Y != 1 {
		t.Fatalf("cmds %+v", cmds)
	}
}

func TestFoodReadouts(t *testing.T) {
	t.Parallel()
	if l := IngredientsLine(game.ItemView{Ingredients: []string{"rice", "meat", "rice"}}); l != "Made with: rice, meat" {
		t.Errorf("ingredients %q", l)
	}
	if IngredientsLine(game.ItemView{}) != "" {
		t.Error("no ingredients, no line")
	}
	if got := strings.Join(AnimalFoodLines(game.AnimalView{Revenge: 30, Wildness: 60, MinHandling: 7}), "|"); !strings.Contains(got, "Revenge chance 30%") || !strings.Contains(got, "Animals 7") {
		t.Errorf("wild lines %q", got)
	}
	if got := AnimalFoodLines(game.AnimalView{Tame: true, FoodPolicy: "Herbivore"}); len(got) != 1 || got[0] != "Food policy: Herbivore" {
		t.Errorf("tame lines %q", got)
	}
	if got := PawnFoodLines(game.PawnView{FoodPolicy: "Anything", LastMeal: "fine meal"}); len(got) != 2 {
		t.Errorf("pawn lines %q", got)
	}
}
