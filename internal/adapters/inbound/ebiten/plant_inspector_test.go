package ebiten

import (
	"fmt"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func btnByID(bs []button, id string) (button, bool) {
	for _, b := range bs {
		if b.ID == id {
			return b, true
		}
	}
	return button{}, false
}

func plantByID(a *App, id entity.EntityID) game.PlantView {
	for _, p := range a.renderer.world().Plants {
		if p.ID == id {
			return p
		}
	}
	return game.PlantView{}
}

func selectPlantTile(t *testing.T, a *App, id entity.EntityID, x, y int) {
	t.Helper()
	clickTile(a, x, y)
	if p, ok := a.selectedPlant(); !ok || p.ID != id {
		t.Fatalf("plant %d not selected", id)
	}
}

func TestPlantPanelHarvestStates(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	ripe, _ := g.SpawnCrop("potato", 8, 8, 100)
	young, _ := g.SpawnCrop("corn", 10, 10, 30)

	selectPlantTile(t, a, ripe, 8, 8)
	b, ok := thingFind(a.plantCommands(ripe), "plant.harvest")
	if !ok || b.Disabled || b.Label != "Harvest" {
		t.Fatalf("ripe crop harvest button = %+v", b)
	}
	click(t, a, "plant.harvest")
	g.Step(1)
	if !plantByID(a, ripe).Designated {
		t.Fatal("Harvest marks the plant")
	}
	if b, _ := thingFind(a.plantCommands(ripe), "plant.harvest"); b.Label != "Cancel harvest" || !b.On {
		t.Fatalf("marked plant should offer Cancel harvest: %+v", b)
	}
	click(t, a, "plant.harvest")
	g.Step(1)
	if plantByID(a, ripe).Designated {
		t.Fatal("Cancel harvest clears the mark")
	}

	selectPlantTile(t, a, young, 10, 10)
	b, _ = thingFind(a.plantCommands(young), "plant.harvest")
	if !b.Disabled || b.Reason != "not ready" {
		t.Fatalf("unripe crop harvest must be disabled 'not ready': %+v", b)
	}
	lines := strings.Join(plantLines(plantByID(a, young)), "|")
	if !strings.Contains(lines, "Not ready (30%)") {
		t.Fatalf("lines = %q", lines)
	}
	if got := plantLines(plantByID(a, ripe)); got[0] != "Ready to harvest" {
		t.Fatalf("ripe lines = %v", got)
	}
}

func TestPlantCutUsesUndesignate(t *testing.T) {
	t.Parallel()
	p := game.PlantView{X: 4, Y: 5, Species: "corn", Cut: true}
	c := plantMarkCmd(p, false, true)
	if c.Kind != command.Undesignate || c.Order != command.OrderCut || c.X != 4 || c.X2 != 4 || c.Y2 != 5 {
		t.Fatalf("cancel cut = %+v, want 1-tile Undesignate OrderCut", c)
	}
	if c := plantMarkCmd(p, true, true); c.Kind != command.Undesignate || c.Order != command.OrderHarvest {
		t.Fatalf("cancel harvest = %+v", c)
	}
	if c := plantMarkCmd(p, false, false); c.Kind != command.DesignateCut {
		t.Fatalf("cut = %+v", c)
	}

	a, g, _ := liveColony(t, 0)
	id, _ := g.SpawnCrop("corn", 8, 8, 30)
	selectPlantTile(t, a, id, 8, 8)
	click(t, a, "plant.cut")
	g.Step(1)
	if !plantByID(a, id).Cut {
		t.Fatal("Cut marks the plant")
	}
	if b, _ := thingFind(a.plantCommands(id), "plant.cut"); b.Label != "Cancel cut" {
		t.Fatalf("label = %q", b.Label)
	}
	click(t, a, "plant.cut")
	g.Step(1)
	if plantByID(a, id).Cut {
		t.Fatal("Cancel cut clears the mark")
	}
}

func TestTreeSaysChop(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	id, _ := g.SpawnCrop("tree", 8, 8, 100)
	selectPlantTile(t, a, id, 8, 8)
	cs := a.plantCommands(id)
	if b, ok := thingFind(cs, "plant.cut"); !ok || b.Label != "Chop" {
		t.Fatalf("tree cut button = %+v", b)
	}
	if _, ok := thingFind(cs, "plant.harvest"); ok {
		t.Fatal("trees have no Harvest button")
	}
	p := plantByID(a, id)
	if title := plantTitle(p); !strings.HasSuffix(title, " tree") || title == "Tree" {
		t.Fatalf("title = %q", title)
	}
	if plantCategory(p) != "Tree" {
		t.Fatalf("category = %q", plantCategory(p))
	}
}

func TestPlantZoneLine(t *testing.T) {
	t.Parallel()
	got := plantLines(game.PlantView{Species: "potato", Growth: 10, Zone: "potato", Wilted: true, Cut: true})
	want := []string{"Not ready (10%)", "Wilting: too cold", "In a growing zone (Potato)", "Marked to cut"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	tree := plantLines(game.PlantView{Species: "tree", Zone: "tree"})
	if len(tree) != 1 || tree[0] != "In a growing zone (Tree)" {
		t.Fatalf("tree lines = %v", tree)
	}
	if plantCategory(game.PlantView{Species: "berry_bush"}) != "Wild plant" ||
		plantTitle(game.PlantView{Species: "berry_bush"}) != "Berry bush" ||
		plantCategory(game.PlantView{Species: "potato"}) != "Crop" {
		t.Fatal("categories/titles wrong")
	}
	if plantStageWord(game.PlantView{Stage: 1}) != "sprouting" ||
		plantStageWord(game.PlantView{Species: "corn", Harvestable: true, Stage: 3}) != "ready to harvest" {
		t.Fatal("stage words wrong")
	}
	if got := plantLines(game.PlantView{Species: "corn", Harvestable: true, Designated: true}); got[1] != "Marked to harvest" {
		t.Fatalf("lines = %v", got)
	}
}

func TestPlantNowChips(t *testing.T) {
	t.Parallel()
	a, g, pawn := liveColony(t, 0)
	id, _ := g.SpawnCrop("potato", 5, 5, 100)
	selectPlantTile(t, a, id, 5, 5)
	now, ok := thingFind(a.plantCommands(id), "cmd.plant.now")
	if !ok || now.Disabled {
		t.Fatalf("expected an enabled now command, got %+v", now)
	}
	opts := now.Options(a)
	if len(opts) == 0 || opts[0].Disabled || opts[0].Cmd == nil {
		t.Fatalf("expected an enabled option, got %+v", opts)
	}
	if opts[0].Cmd.Pawn != uint64(pawn) || opts[0].Cmd.Order != command.OrderHarvest {
		t.Fatalf("option %+v should send pawn %d to harvest", opts[0].Cmd, pawn)
	}
	thingPress(a, now) // the nearest colonist
	click(t, a, "cmd.plant.now")
	a.applyPlantClick("plant.now.notanumber")
	a.applyPlantClick("plant.close")
	if a.hasPlant {
		t.Fatal("close clears the selection")
	}
	a.applyPlantClick("plant.close") // nothing selected: no-op
}

func TestPlantPanelButtonsDisabledChip(t *testing.T) {
	t.Parallel()
	opts := colonistOptions([]colonistAction{{Pawn: 7, Name: "Ada", Disabled: true, Reason: "cannot reach"}})
	if len(opts) != 1 || !opts[0].Disabled || opts[0].Reason != "cannot reach" || opts[0].Label != "Ada" {
		t.Fatalf("option = %+v", opts)
	}
}

func TestPlantPanelClosesWhenPlantGone(t *testing.T) {
	t.Parallel()
	a, _, _ := liveColony(t, 0)
	a.selPlant, a.hasPlant = 9999, true
	if _, ok := a.selectedPlant(); ok {
		t.Fatal("a vanished plant is not selected")
	}
	if len(a.commandGridButtons()) != 0 || len(a.plantCommands(9999)) != 0 || a.plantPane(9999).Title != "" || a.hasPlant {
		t.Fatal("panel must close with no buttons")
	}
	a.applyPlantClick("plant.harvest") // no-op, no panic
}

// ---- growth state (PLAY-017) ----

func TestPlantGrowCaption(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		p    game.PlantView
		gv   game.PlantGrowthView
		want string
	}{
		{"no state", game.PlantView{Species: "corn", Growth: 42}, game.PlantGrowthView{}, ""},
		{"growing", game.PlantView{Species: "corn", Growth: 42}, game.PlantGrowthView{State: "growing"}, "42% · growing"},
		{"resting at night", game.PlantView{Species: "corn", Growth: 42}, game.PlantGrowthView{State: "resting"}, "42% · resting (night)"},
		{"dark", game.PlantView{Species: "corn", Growth: 42}, game.PlantGrowthView{State: "dark"}, "too dark: needs sun or a lamp nearby"},
		{"cold", game.PlantView{Species: "corn", Growth: 7}, game.PlantGrowthView{State: "cold"}, "7% · too cold"},
		{"hot", game.PlantView{Species: "corn", Growth: 7}, game.PlantGrowthView{State: "hot"}, "7% · too hot"},
		{"ripe crop", game.PlantView{Species: "corn", Growth: 100, Harvestable: true}, game.PlantGrowthView{State: "ripe"}, "ripe"},
		{"full-grown tree", game.PlantView{Species: "tree", Growth: 100}, game.PlantGrowthView{State: "ripe"}, "fully grown"},
		{"unknown state", game.PlantView{Species: "corn"}, game.PlantGrowthView{State: "weird"}, ""},
	} {
		if got := plantGrowCaption(c.p, c.gv); got != c.want {
			t.Errorf("%s: caption %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPlantRipeLine(t *testing.T) {
	t.Parallel()
	d := clock.TicksPerDay
	for _, c := range []struct {
		name   string
		p      game.PlantView
		ripeIn int
		want   string
	}{
		{"unknown", game.PlantView{Species: "corn"}, -1, ""},
		{"unset", game.PlantView{Species: "corn"}, 0, ""},
		{"under two days: one decimal", game.PlantView{Species: "corn"}, d * 14 / 10, "Ripe in about 1.4 days"},
		{"just under two reads as two", game.PlantView{Species: "corn"}, d*2 - 1, "Ripe in about 2 days"},
		{"1.9 stays decimal", game.PlantView{Species: "corn"}, d * 19 / 10, "Ripe in about 1.9 days"},
		{"two days and more: whole", game.PlantView{Species: "corn"}, d*3 + d/3, "Ripe in about 3 days"},
		{"rounds", game.PlantView{Species: "corn"}, d*7 + d*6/10, "Ripe in about 8 days"},
		{"an hour left", game.PlantView{Species: "corn"}, clock.TicksPerHour, "Ripe in about 0.1 days"},
		{"a tree is fully grown, not ripe", game.PlantView{Species: "tree"}, d * 30, "Fully grown in about 30 days"},
		{"moss is fully grown", game.PlantView{Species: "moss"}, d * 2, "Fully grown in about 2 days"},
		{"a berry bush ripens", game.PlantView{Species: "berry_bush"}, d, "Ripe in about 1.0 days"},
		{"nothing once harvestable", game.PlantView{Species: "corn", Harvestable: true}, d, ""},
	} {
		if got := plantRipeLine(c.p, game.PlantGrowthView{RipeIn: c.ripeIn}); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// TestPlantLinesCarryRipeIn: the ripening estimate sits right under the
// progress line, and the old lines stay as they were.
func TestPlantLinesCarryRipeIn(t *testing.T) {
	t.Parallel()
	got := plantBodyLines(game.PlantView{Species: "potato", Growth: 42, Zone: "potato"},
		game.PlantGrowthView{State: "growing", RipeIn: clock.TicksPerDay * 3})
	want := []string{"Not ready (42%)", "Ripe in about 3 days", "In a growing zone (Potato)"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	if strings.Join(plantLines(game.PlantView{Species: "potato", Growth: 42, Zone: "potato"}), "|") != "Not ready (42%)|In a growing zone (Potato)" {
		t.Fatal("the view-only lines carry an estimate")
	}
}

// TestPlantPanelShowsLiveGrowthState: a crop sown in the colony shows the
// state the game gives it (Simulation.PlantGrowth) and a ripening
// estimate when it is growing, in the pane.
func TestPlantPanelShowsLiveGrowthState(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	id, _ := g.SpawnCrop("potato", 8, 8, 30)
	selectPlantTile(t, a, id, 8, 8)
	p := plantByID(a, id)
	gv, ok := g.PlantGrowth(id)
	if !ok || gv.State == "" {
		t.Fatalf("the game gave the plant no growth state: %+v (%v)", gv, ok)
	}
	c := plantGrowCaption(p, gv)
	if c == "" {
		t.Fatalf("no caption for state %q", gv.State)
	}
	line := plantRipeLine(p, gv)
	if growing := gv.RipeIn > 0; growing != (line != "") {
		t.Fatalf("state %q, RipeIn %d, line %q", gv.State, gv.RipeIn, line)
	}
	if sp := a.plantPane(id); len(sp.Lines) == 0 || sp.Lines[0] != c {
		t.Fatalf("the pane does not lead with the live caption %q: %q", c, sp.Lines)
	}
}

// TestPlantPanelTextFitsAtScale1_5: at UI scale 1.5 the caption and the
// body lines, wrapped to the pane's text width, stay inside it, even for a
// crop with the most lines a growing one carries; the caption leads.
func TestPlantPanelTextFitsAtScale1_5(t *testing.T) {
	withUIScale(t, 1.5)
	width := float64(paneW - 2*paneInset - scrollBarW - 4) // the pane body's wrap width
	for _, c := range []struct {
		p  game.PlantView
		gv game.PlantGrowthView
	}{
		{game.PlantView{Species: "potato", Growth: 42, Zone: "potato"}, game.PlantGrowthView{State: "resting", RipeIn: clock.TicksPerDay * 3}},
		{game.PlantView{Species: "potato", Growth: 42, Zone: "potato", Designated: true}, game.PlantGrowthView{State: "dark", RipeIn: -1}},
		{game.PlantView{Species: "potato", Growth: 42, Zone: "potato", Wilted: true, Cut: true}, game.PlantGrowthView{State: "cold", RipeIn: -1}},
		{game.PlantView{Species: "moss", Growth: 42, NeedsLight: true}, game.PlantGrowthView{State: "dark", RipeIn: -1}},
		{game.PlantView{Species: "tree", Growth: 90}, game.PlantGrowthView{State: "growing", RipeIn: clock.TicksPerDay * 12}},
	} {
		p, gv := c.p, c.gv
		lines := plantPaneLines(p, gv)
		var texts []string
		for _, l := range lines {
			texts = append(texts, wrapPx(l, tsBody, width)...)
		}
		if w := maxTextW(texts, tsBody); float64(w) > width {
			t.Errorf("%s/%s: widest line %.0f px, pane text width %.0f", p.Species, gv.State, w, width)
		}
		if c := plantGrowCaption(p, gv); c != "" && (len(lines) == 0 || lines[0] != c) {
			t.Errorf("%s/%s: caption %q is not the first line (%v)", p.Species, gv.State, c, lines)
		}
		if !strings.HasSuffix(strings.Join(lines, "|"), strings.Join(plantBodyLines(p, gv), "|")) {
			t.Errorf("%s/%s: the body lines follow the caption: %v", p.Species, gv.State, lines)
		}
	}
}

// TestPlantInspectorDrawsAtScale1_5: at UI scale 1.5 the legacy draw call
// paints no panel frame any more (the pane replaced it) and does not panic.
func TestPlantInspectorDrawsAtScale1_5(t *testing.T) {
	a, g, _ := liveColony(t, 0) // starting a colony resets the scale: set it after
	id, _ := g.SpawnCrop("corn", 8, 8, 30)
	selectPlantTile(t, a, id, 8, 8)
	withUIScale(t, 1.5)
	got := traceCanvas(t)
	a.drawPlantMark(ebiten.NewImage(900, 700))
	r := rect{X: 8, Y: 40, W: 340, H: 226} // the old panel, top-left
	frame := fmt.Sprintf("%g,%g %gx%g", float64(r.X)*1.5, float64(r.Y)*1.5, float64(r.W)*1.5, float64(r.H)*1.5)
	for _, c := range *got {
		if strings.HasSuffix(c, frame) {
			t.Fatalf("the old panel frame %q is still drawn: %q", frame, *got)
		}
	}
}
