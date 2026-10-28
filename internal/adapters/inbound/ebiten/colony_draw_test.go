package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// The colony UI's drawing: every panel and window of UI-COLONY painted
// over a stub colony, so a drawing slip (a nil art catalog, a rectangle
// off the screen, a missing glyph) fails here and not in a capture. The
// shots themselves are reviewed by eye (capture_colony.go).

// drawFrame runs a frame and paints the whole app into an off-screen image.
func drawFrame(t *testing.T, a *App, f *fakeInput) *ebiten.Image {
	t.Helper()
	f.Frame(t, a)
	img := ebiten.NewImage(a.sw, a.sh)
	a.Draw(img)
	return img
}

// drawSim is a colony with a bio, a hostile and a friendly faction, a
// scenario-locked building, a heat wave and a condition.
func drawSim() *stubSim {
	s := bioSim()
	s.pawns[0].Incapable = work.Mask(0).Set(work.Haul, true).Set(work.Clean, true)
	s.pawns[0].SkillLv[skill.Plants], s.pawns[0].Passion[skill.Plants] = 12, 2
	s.pawns[0].SkillLv[skill.Mining], s.pawns[0].Passion[skill.Mining] = 5, 1
	s.pawns[0].Priorities = []int{0, 3, 3, 3, 3, 3, 0, 0, 3, 3, 3, 3}
	s.pawns = append(s.pawns, game.PawnView{ID: 2, Name: "Bo", X: 6, Y: 5, Status: "alive"}) // no bio
	s.incapable = map[entity.EntityID]map[work.Type]string{1: {work.Haul: "Incapable: Hauling (backstory: Convoy brat)"}}
	s.factions = factionViews()
	s.factions[2].History = []int16{10, 12, 15, 20, 30}
	s.buildables = lockedCatalog("campfire")
	s.story = storySim().story
	s.world.Extreme = "heat_wave"
	s.world.Conditions = []string{"ashfall_sky", "endless_chill"}
	s.world.Messages = []game.MessageView{{Tick: 1, Text: "A heat wave has settled over the land.", Letter: true, Title: "Heat wave", Danger: true}}
	return s
}

func TestColonyHUDDrawsEveryState(t *testing.T) {
	t.Parallel()
	a, f := colonyApp(t, drawSim())
	a.renderer.Select(1)
	s := a.renderer.game.(*stubSim)
	s.social = map[entity.EntityID]game.SocialView{1: socialFixture()}
	s.history = []game.HistoryEntryView{{Tick: 100, Text: "Colony founded.", Kind: "colony"}, {Tick: 5000, Text: "Ada and Bo married.", Pawns: []uint64{1}}}
	for _, tab := range []string{"character", "needs", "bio", "gear", "log", "health", "social"} {
		a.inspectTab = tab
		drawFrame(t, a, f)
	}
	a.openHistory()
	drawFrame(t, a, f)
	a.uiCloseTop()
	a.renderer.Select(2) // a colonist with no biography still shows the Bio tab's disabled state
	a.inspectTab = "bio"
	drawFrame(t, a, f)
	a.renderer.ClearSelect()

	// The Work tab, with the cursor on an incapable cell and on a header.
	a.workOpen, a.panelMode = true, "work"
	r := a.windowRect()
	for _, c := range []rect{workCellRect(r, 0, 6), workCellRect(r, 0, 2), {X: workCellRect(r, 0, 4).X, Y: r.y + 24, W: 30, H: 16}} {
		x, y := c.Center()
		f.Cur.Mx, f.Cur.My = int(x), int(y)
		for i := 0; i < tipDelay+2; i++ {
			f.Frame(t, a)
		}
		drawFrame(t, a, f)
	}
	a.workOpen = false

	// The Architect in every category with a card hovered.
	f.Cur.Mx, f.Cur.My = -50, -50
	a.openArchitect()
	for _, c := range archCategories {
		a.selectArchCategory(c.id)
		if cells := a.hud().GridCells; len(cells) > 0 { // hover the category's first tool
			f.Cur.Mx, f.Cur.My = int(cells[0].X+cells[0].W/2), int(cells[0].Y+cells[0].H/2)
		}
		drawFrame(t, a, f)
	}
	a.selectArchCategory("furniture")
	a.selectArchTool("hospital_bed")
	drawFrame(t, a, f)
	a.closeArchitect()
}

func TestColonyWindowsDraw(t *testing.T) {
	t.Parallel()
	s := drawSim()
	s.world.Messages = nil
	a, f := colonyApp(t, s)
	f.Cur.Mx, f.Cur.My = -50, -50

	// The Factions window on each faction row, and with no factions at all.
	a.openColonyFactions()
	drawFrame(t, a, f)
	a.uiCloseTop()
	s.factions = nil
	a.openColonyFactions()
	drawFrame(t, a, f)
	a.uiCloseTop()

	// A letter in the stack with its message line, then opened from the
	// stack by a click (PLAY-018: only the intro opens by itself).
	s.world.Messages = []game.MessageView{{Tick: 0, Text: "A heat wave has settled over the land.", Letter: true, Title: "Heat wave"}}
	s.world.Letters = []game.LetterView{{ID: 3, Tick: 0, Kind: "caution", Title: "Heat wave", Text: "A heat wave has settled over the land.", X: -1, Y: -1}}
	drawFrame(t, a, f)
	if len(a.letterRows(a.hud())) != 1 {
		t.Fatal("the letter should be in the stack")
	}
	pressButton(t, a, f, "letter.3")
	drawFrame(t, a, f)
	if !a.uiModalOpen() {
		t.Fatal("the letter should be open")
	}
	a.uiCloseTop()
	drawFrame(t, a, f)

	// Options > Story, with the custom dialog over it.
	a.optionsTab = "story"
	a.apply(intent{kind: intentPush, screen: screenOptions})
	drawFrame(t, a, f)
	a.openStoryCustom(s.story)
	drawFrame(t, a, f)
	f.Cur.Mx, f.Cur.My = 640, 300
	drawFrame(t, a, f)
}

func TestCaptureSetupHasWhatTheShotsShow(t *testing.T) {
	t.Parallel()
	for _, ui := range colonyCaptures {
		if !colonyCapture(ui) {
			t.Fatalf("%s is not a colony capture", ui)
		}
	}
	if colonyCapture("architect") {
		t.Fatal("the classic architect shot starts the classic colony")
	}
	s := captureSetup("arch-locked")
	if len(s.Colonists) != 3 || len(s.Factions) == 0 || s.Scenario.Key != "broken_hull" {
		t.Fatalf("setup = %d colonists, %d factions, scenario %q", len(s.Colonists), len(s.Factions), s.Scenario.Key)
	}
	last := len(s.Scenario.Parts) - 1
	if s.Scenario.Parts[last].Building == 0 || s.Scenario.Parts[last].Kind == 0 {
		t.Fatalf("arch-locked adds no disallowed building: %+v", s.Scenario.Parts[last])
	}
	for _, f := range s.Factions {
		if len(f.History) != 40 || len(f.Events) != 2 {
			t.Fatalf("faction %s history %d events %d", f.Name, len(f.History), len(f.Events))
		}
	}
	if n := len(captureSetup("chips").Scenario.Parts); n != len(s.Scenario.Parts) {
		t.Fatalf("chips adds one part like arch-locked: %d vs %d", n, len(s.Scenario.Parts))
	}
}

func TestOptionsTabsDrawInColony(t *testing.T) {
	t.Parallel()
	a, f := colonyApp(t, drawSim())
	a.apply(intent{kind: intentPush, screen: screenOptions})
	tabs := a.optionTabList()
	if tabs[3] != "story" {
		t.Fatalf("tabs = %v, want Story after Gameplay", tabs)
	}
	for _, tab := range tabs {
		a.optionsTab = tab
		drawFrame(t, a, f)
		if a.currentOptionsTab() != tab {
			t.Fatalf("tab %q did not open", tab)
		}
	}
}

// TestColonyToolGhostsAndHoverDraw paints the Architect's ghosts (a wall
// line, a growing zone, a roof area) and the hover card over real ground.
func TestColonyToolGhostsAndHoverDraw(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	if _, err := g.SpawnCrop("potato", 8, 8, 50); err != nil {
		t.Fatal(err)
	}
	f := fakeOn(a)
	f.Frame(t, a)
	a.renderer.speed = speedPaused
	sx, sy := a.renderer.worldToScreenF(8.5, 8.5)
	a.cursor = &[2]int{int(sx), int(sy)}
	drawFrame(t, a, f) // the hover card over a plant

	a.openArchitect()
	for _, c := range []struct{ cat, tool string }{{"structure", "wall"}, {"zones", "grow"}, {"zones", "stockpile"}, {"zones", "roof"}, {"orders", "mine"}, {"furniture", "hospital_bed"}} {
		a.selectArchCategory(c.cat)
		a.selectArchTool(c.tool)
		a.archDrag = true
		a.archX0, a.archY0, a.archX1, a.archY1 = 7, 7, 10, 9
		drawFrame(t, a, f)
	}
}

func TestInspectorHelpers(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"parka", "wool parka", "tuque", "duster", "sweater"} {
		if gearLine(k) == "" {
			t.Errorf("gearLine(%q) is empty", k)
		}
	}
	if titleCase("") != "" || titleCase("rifle") != "Rifle" {
		t.Error("titleCase")
	}
	if moodColor(10) == moodColor(90) || moodColor(40) == moodColor(90) {
		t.Error("moodColor should tell low, middling and good moods apart")
	}
	a, _ := colonyApp(t, bioSim())
	if got := a.creatureName(1); got != "Ada" {
		t.Errorf("creatureName = %q", got)
	}
	if got := a.creatureName(99); got != "" {
		t.Errorf("unknown creature = %q", got)
	}
}
