package ebiten

// journey_ax4_test.go (AX-4): power and industry through the real UI
// loop: every AX-4 tool laid from the Architect, a blueprint's secondary
// material on its pane, the deep drill's and scanner's panes and the deep
// ore overlay they force on, the moisture pump's pane, and on a river
// colony the watermill (its wheel in the river) and the waterproof
// conduit over shallow water.

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/randx"
)

// ax4Lay arms tool in cat from the Architect, turns it to facing (quarter
// turns with E), clicks (x, y) and checks the blueprint of kind it lays.
func ax4Lay(t *testing.T, j *journey, cat, tool, kind string, facing, x, y int) {
	t.Helper()
	j.lookAt(x, y)
	j.update(1)
	j.openArch(cat, tool)
	for range facing {
		j.key("E", 0)
	}
	j.clickTile(x, y, inputframe.Left, 0)
	if c, ok := j.lastCmd(command.DesignateBuild); !ok || c.X != x || c.Y != y {
		t.Fatalf("%s: DesignateBuild %+v ok=%v, want (%d, %d)", tool, c, ok, x, y)
	}
	j.update(1)
	for _, f := range j.world().Frames {
		if f.X == x && f.Y == y && f.Kind == kind {
			j.leaveArch()
			return
		}
	}
	t.Fatalf("%s: no %s blueprint at (%d, %d): %+v", tool, kind, x, y, j.world().Frames)
}

// ax4Research finishes the AX-4 projects.
func ax4Research(j *journey) {
	for _, p := range []research.Project{research.Electricity, research.DeepDrilling, research.GroundScanning,
		research.Drainage, research.WaterPower} {
		rsFinish(j, int(p))
	}
}

// TestJourneyAX4Industry: the chemfuel generator, deep drill, electric
// crematorium, scanner and moisture pump are laid from their categories;
// the crematorium's blueprint lists its steel; a built drill's and
// scanner's panes show their cycle and selecting one forces the deep ore
// overlay; the pump's pane shows its dried radius.
func TestJourneyAX4Industry(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(40, 32), jPawns([2]int{8, 4}))
	ax4Research(j)
	ax4Lay(t, j, "power", "chemfuel_generator", "chemfuel generator", 0, 14, 8)
	ax4Lay(t, j, "production", "deep_drill", "deep drill", 0, 18, 8)
	ax4Lay(t, j, "production", "crematorium", "electric crematorium", 0, 12, 14)
	ax4Lay(t, j, "misc", "deep_scanner", "ground-penetrating scanner", 0, 20, 14)
	ax4Lay(t, j, "misc", "moisture_pump", "moisture pump", 0, 16, 18)
	ax4Lay(t, j, "power", "waterproof_conduit", "waterproof conduit", 0, 10, 18)

	// the crematorium's blueprint: 2 steel as its secondary material
	var crem game.FrameView
	for _, f := range j.world().Frames {
		if f.Kind == "electric crematorium" {
			crem = f
		}
	}
	j.lookAt(crem.X, crem.Y)
	j.update(1)
	j.settle()
	j.clickTile(crem.X, crem.Y, inputframe.Left, 0)
	if !j.a.sel.HasFrame || j.a.sel.SelFrame != crem.ID {
		t.Fatal("the click did not select the crematorium blueprint")
	}
	if lines := strings.Join(j.a.insp.Thing().FramePane(crem.ID).Lines, "\n"); !strings.Contains(lines, "Steel  0 / 2") {
		t.Fatalf("crematorium blueprint pane %q, want its steel", lines)
	}

	wdBuild(j, "deep drill", 30, 4)
	id := wdSelect(j, 30, 4)
	if lines := wbPaneLines(j, id); !strings.Contains(lines, "drilling  0%") || !strings.Contains(lines, "stone chunks") {
		t.Fatalf("drill pane %q", lines)
	}
	j.update(1)
	if !j.a.renderer.overlays().DeepOre.Forced {
		t.Fatal("a selected deep drill does not show the deep ore overlay")
	}
	wdBuild(j, "ground-penetrating scanner", 30, 10)
	id = wdSelect(j, 30, 10)
	if lines := wbPaneLines(j, id); !strings.Contains(lines, "scanning") {
		t.Fatalf("scanner pane %q", lines)
	}
	wdBuild(j, "moisture pump", 26, 18)
	id = wdSelect(j, 26, 18)
	j.update(1)
	if j.a.renderer.overlays().DeepOre.Forced {
		t.Fatal("the overlay stays forced off the drill and scanner")
	}
	if lines := wbPaneLines(j, id); !strings.Contains(lines, "dried radius  0 / 7") {
		t.Fatalf("pump pane %q", lines)
	}
}

// ax4RiverColony is a generated colony on a river tile.
func ax4RiverColony(t *testing.T) *game.Game {
	t.Helper()
	d, ok := scenario.Builtin("broken_hull")
	if !ok {
		t.Fatal("no broken_hull scenario")
	}
	s := &game.Setup{
		Scenario: d, Storyteller: "steady", Preset: difficulty.Seasoned, Difficulty: difficulty.Of(difficulty.Seasoned),
		Site: site.Site{Biome: biome.TemperateForest, Climate: biome.DefOf(biome.TemperateForest).Climate, Hills: site.Flat,
			Stones: []geology.Stone{geology.Granite}, River: site.RiverPath{Size: site.River, In: 0, Out: 4}},
		Factions: faction.Generate(faction.DefaultSlots(), randx.New(7)),
		MapSize:  80, Season: game.SeasonSpring, Seed: 4243,
	}
	s.Colonists = game.NewCandidatePool(s.Seed, s.Scenario.Apply()).Selected()
	b := game.NewColonyBuilder(game.Config{Setup: s})
	for i := 0; i < 400; i++ {
		if _, _, done := b.Step(1); done {
			return b.Game()
		}
	}
	t.Fatal("the colony builder never finished")
	return nil
}

// TestJourneyAX4Watermill: on a river colony the watermill is laid from
// Power with its wheel in the river (and refused on dry land), a built one
// makes power and shows its river flow; a waterproof conduit is laid over
// shallow water where a plain one is refused.
func TestJourneyAX4Watermill(t *testing.T) {
	t.Parallel()
	g := ax4RiverColony(t)
	j := newJourney(t, jGame(g))
	ax4Research(j)
	j.g.ClearPlants(0, 0, 79, 79)
	facings := []string{"south", "east", "north", "west"}
	fx, fy, ff := -1, -1, 0
	for y := 2; y < 72 && fx < 0; y++ {
		for x := 2; x < 72 && fx < 0; x++ {
			for f, name := range facings {
				if ok, _ := j.g.CanPlaceAt("watermill generator", "wood", name, x, y); ok {
					fx, fy, ff = x, y, f
					break
				}
			}
		}
	}
	if fx < 0 {
		t.Fatal("no place for a watermill on the river map")
	}
	if ok, r := j.g.CanPlaceAt("watermill generator", "wood", "south", 2, 2); ok || r == "" {
		t.Fatalf("a watermill away from the river: %v %q", ok, r)
	}
	ax4Lay(t, j, "power", "watermill", "watermill generator", ff, fx, fy)
	j.g.Enqueue(command.Command{Kind: command.CancelFrame, Target: uint64(j.world().Frames[len(j.world().Frames)-1].ID)})
	j.g.ApplyPending()
	if !j.g.BuildNow("watermill generator", "wood", facings[ff], fx, fy) {
		t.Fatal("BuildNow refused the watermill")
	}
	id := wdSelect(j, fx, fy)
	if lines := wbPaneLines(j, id); !strings.Contains(lines, "river flow  100%") {
		t.Fatalf("watermill pane %q", lines)
	}
	// a shallow water cell for the conduits
	w := j.world()
	cx, cy := -1, -1
	for i, tt := range w.Terrain {
		x, y := i%w.TerrainW, i/w.TerrainW
		if terrain.Type(tt) == terrain.ShallowWater && x > 2 && y > 2 && x < 70 && y < 70 {
			if ok, _ := j.g.CanPlaceAt("waterproof conduit", "steel", "south", x, y); ok {
				cx, cy = x, y
				break
			}
		}
	}
	if cx < 0 {
		t.Fatal("no shallow water for a waterproof conduit")
	}
	if ok, _ := j.g.CanPlaceAt("power conduit", "steel", "south", cx, cy); ok {
		t.Fatal("a plain conduit on shallow water")
	}
	ax4Lay(t, j, "power", "waterproof_conduit", "waterproof conduit", 0, cx, cy)
}
