package hud

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

func tempWorld(season string) game.WorldView {
	return game.WorldView{BiomeKey: "temperate_forest", Season: season}
}

func TestNeedWarmClothesAlert(t *testing.T) {
	t.Parallel()
	cold := game.PawnView{Name: "Ada", ComfortMin: 160, ComfortMax: 260, X: 1, Y: 2}
	warm := game.PawnView{Name: "Bo", ComfortMin: -400, ComfortMax: 300}
	if _, ok := NeedWarmClothesAlert([]game.PawnView{cold}, tempWorld("Summer")); ok {
		t.Fatal("no alert in summer")
	}
	al, ok := NeedWarmClothesAlert([]game.PawnView{cold, warm}, tempWorld("Fall"))
	if !ok || al.Key != "warmclothes" || al.Label != "Ada needs warm clothes" || len(al.Targets) != 1 {
		t.Fatalf("fall alert %+v ok=%v", al, ok)
	}
	if _, ok := NeedWarmClothesAlert([]game.PawnView{warm}, tempWorld("Winter")); ok {
		t.Fatal("a warmly dressed colonist raises nothing")
	}
	two := []game.PawnView{cold, {Name: "Cy", ComfortMin: 100, ComfortMax: 200}}
	if al, _ := NeedWarmClothesAlert(two, tempWorld("Winter")); !strings.HasPrefix(al.Label, "2 colonists") {
		t.Fatalf("label %q", al.Label)
	}
	// the season alone decides, so the alert cannot flap with the day's swing
	a, b := tempWorld("Winter"), tempWorld("Winter")
	a.OutdoorTemp, b.OutdoorTemp = -300, 150
	l1, _ := ComingLow(a)
	l2, _ := ComingLow(b)
	if l1 != l2 {
		t.Fatalf("coming low moved with the outdoor temperature: %d vs %d", l1, l2)
	}
	if strings.Contains(strings.Join(alertKeys(BuildAlerts([]game.PawnView{cold}, tempWorld("Winter"))), ","), "warmclothes") == false {
		t.Fatal("BuildAlerts lacks the warm clothes alert")
	}
	if AlertExplain("warmclothes") == AlertExplain("zzz") || AlertExplain("animalcold") == AlertExplain("zzz") {
		t.Fatal("the new alerts need explanations")
	}
}

func TestColdAnimalAlert(t *testing.T) {
	t.Parallel()
	w := game.WorldView{Animals: []game.AnimalView{
		{Tame: true, Hypothermia: 30, X: 3, Y: 4},
		{Tame: false, Hypothermia: 90},
		{Tame: true, Hypothermia: 5},
	}}
	al, ok := ColdAnimalAlert(w)
	if !ok || al.Label != "Hypothermic animal" || len(al.Targets) != 1 || al.Targets[0] != [2]int{3, 4} {
		t.Fatalf("alert %+v ok=%v", al, ok)
	}
	w.Animals = append(w.Animals, game.AnimalView{Tame: true, Hypothermia: 60})
	if al, _ := ColdAnimalAlert(w); al.Label != "Hypothermic animals (2)" {
		t.Fatalf("label %q", al.Label)
	}
	if _, ok := ColdAnimalAlert(game.WorldView{}); ok {
		t.Fatal("no animals, no alert")
	}
}

func TestVolcanicWinterChipAndRoomHeatLines(t *testing.T) {
	t.Parallel()
	chips := StatusChips(game.WorldView{Extreme: "volcanic_winter"})
	if len(chips) != 1 || chips[0].Label != "Volcanic winter" || !strings.Contains(chips[0].Tip, "30% less sunlight") {
		t.Fatalf("chips %+v", chips)
	}
	got := strings.Join(RoomHeatLines(game.RoomView{ExposedWalls: 8, DoubleWalls: 2, RoofKind: "thin", HeatRate: -105}), "|")
	if got != "Exposed walls 8 (2 double), thin roof|Heat -10.5/s" {
		t.Fatalf("heat lines %q", got)
	}
	if RoomHeatLines(game.RoomView{}) != nil {
		t.Fatal("a plain room has no thermal lines")
	}
	if !strings.Contains(strings.Join(RoomCardLines(game.RoomView{Temp: 210, ExposedWalls: 4}), "|"), "Exposed walls 4") {
		t.Fatal("the card shows the thermal lines")
	}
}

func TestRoomCardOutdoorsFlag(t *testing.T) {
	t.Parallel()
	if got := strings.Join(RoomHeatLines(game.RoomView{Cells: 20, Roofed: 0.5}), "|"); !strings.Contains(got, "Counts as outdoors") {
		t.Fatalf("lines %q", got)
	}
	if RoomHeatLines(game.RoomView{Cells: 20, Roofed: 1}) != nil {
		t.Fatal("a roofed room is indoors")
	}
}
