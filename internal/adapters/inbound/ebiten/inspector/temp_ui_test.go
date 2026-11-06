package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestVentButtonEmitsToggleVent(t *testing.T) {
	t.Parallel()
	if ventCommands(game.BuildingView{Kind: "wall"}) != nil {
		t.Fatal("only a vent has the button")
	}
	shut := ventCommands(game.BuildingView{ID: 7, Kind: "vent"})
	open := ventCommands(game.BuildingView{ID: 7, Kind: "vent", Open: true})
	if len(shut) != 1 || len(open) != 1 {
		t.Fatalf("buttons %d %d", len(shut), len(open))
	}
	if c := shut[0].Cmds[0]; c.Kind != command.ToggleVent || c.Target != 7 || !c.Enabled || shut[0].On {
		t.Fatalf("shut vent: %+v on=%v", c, shut[0].On)
	}
	if c := open[0].Cmds[0]; c.Enabled || !open[0].On {
		t.Fatalf("open vent: %+v on=%v", c, open[0].On)
	}
}

func TestTempLines(t *testing.T) {
	t.Parallel()
	join := func(b game.BuildingView) string { return strings.Join(TempLines(b), "|") }
	for _, c := range []struct {
		b    game.BuildingView
		want string
	}{
		{game.BuildingView{Kind: "vent", Open: true}, "Open: the rooms either side share their air"},
		{game.BuildingView{Kind: "vent"}, "Shut: no air passes"},
		{game.BuildingView{Kind: "passive cooler", Fuel: true}, "Out of wood: no cooling"},
		{game.BuildingView{Kind: "passive cooler", Fuel: true, FuelPct: 80, HeatRate: -110}, "Burns wood to cool the room|Cooling -11.0 heat/s"},
		{game.BuildingView{Kind: "heater", HeatRate: 210}, "Heat output +21.0 heat/s"},
	} {
		if got := join(c.b); got != c.want {
			t.Errorf("%s: %q, want %q", c.b.Kind, got, c.want)
		}
	}
	if TempLines(game.BuildingView{Kind: "wall"}) != nil {
		t.Fatal("a wall has no temperature lines")
	}
}

func TestExposureLinesNameStagesAndFrostbite(t *testing.T) {
	t.Parallel()
	got := strings.Join(ExposureLines(game.PawnView{Hypothermia: 360, Heatstroke: 30, Frostbite: 2}), "|")
	for _, w := range []string{"Hypothermia 36% (serious)", "Heatstroke 3% (shivering)", "Frostbite on 2 parts"} {
		if !strings.Contains(got, w) {
			t.Errorf("%q lacks %q", got, w)
		}
	}
	if ExposureLines(game.PawnView{}) != nil {
		t.Fatal("a warm colonist has no exposure lines")
	}
	if !strings.Contains(strings.Join(ExposureLines(game.PawnView{Frostbite: 1}), ""), "1 part") {
		t.Fatal("one part, singular")
	}
	if ExposureTip("Hypothermia (minor)") == "" || ExposureTip("Flu") != "" {
		t.Fatal("tips for exposure rows only")
	}
}

func TestAnimalTempLines(t *testing.T) {
	t.Parallel()
	if AnimalTempLines(game.AnimalView{}) != nil {
		t.Fatal("no range, no lines")
	}
	got := strings.Join(AnimalTempLines(game.AnimalView{ComfortMin: -200, ComfortMax: 450, Hypothermia: 45}), "|")
	if got != "Comfortable -20 °C to 45 °C|Hypothermia 45% (serious)" {
		t.Fatalf("lines %q", got)
	}
}

func TestInsulationLine(t *testing.T) {
	t.Parallel()
	if InsulationLine("steel", "", "") != "" || InsulationLine("nothing", "", "") != "" {
		t.Fatal("only apparel insulates")
	}
	parka := InsulationLine("parka", "", "")
	if !strings.HasPrefix(parka, "Insulation: cold ") {
		t.Fatalf("parka line %q", parka)
	}
	if !strings.Contains(strings.Join(ItemLines(game.ItemView{Kind: "parka", FreshDays: -1}, false), "|"), "Insulation: cold") {
		t.Fatal("the item pane lacks the insulation line")
	}
}
