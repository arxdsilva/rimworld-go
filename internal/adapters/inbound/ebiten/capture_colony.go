package ebiten

import (
	"slices"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/randx"
)

// Captures of the colony UI of NG-1 (UI-COLONY). They start a colony from a
// Setup (the new-game path, with drop pods, bios, factions and a storyteller)
// built here instead of by the wizard, then open the UI state:
//
//	factions-colony  the Factions window, goodwill history and events
//	bio              a colonist's Bio tab
//	work-incapable   the Work tab with hatched incapable cells (and one tip)
//	arrival          the drop pods falling (use -capture-ticks 60)
//	options-story    Options > Story
//	letter-intro     the intro letter (use -capture-ticks 300)
//	arch-hospital    the Architect's Hospital bed card, tooltip with components
//	arch-cabinet     the Architect's Tool cabinet card, tooltip with components
//	arch-locked      a scenario-forbidden building shown disabled with its reason
//	chips            the top strip's condition chip (Ashfall sky)
//
// Captures are developer tooling: they use the same Enqueue / Step calls
// as play and a fixed seed, so a shot is repeatable.

// colonyCaptures lists the states above.
var colonyCaptures = []string{
	"factions-colony", "bio", "work-incapable", "arrival", "options-story",
	"letter-intro", "arch-hospital", "arch-cabinet", "arch-locked", "chips",
}

// colonyCapture reports ui is one of them.
func colonyCapture(ui string) bool { return slices.Contains(colonyCaptures, ui) }

// captureStart starts the capture's colony: a setup colony for the colony
// UI states, else the classic start.
func (a *App) captureStart(ui string) {
	if !colonyCapture(ui) {
		a.startGame()
		return
	}
	a.cfg.Capture.Bare = true // no scripted blueprints over the arrival
	s := captureSetup(ui)
	b, err := a.simBuilder(NewGame{Seed: s.Seed, Biome: s.Site.Biome, Setup: &s})
	if err != nil {
		a.sessionErr = err.Error()
		return
	}
	for {
		if _, _, done := b.Step(loadUnits); done {
			break
		}
	}
	sim, err := b.Sim()
	if err != nil {
		a.sessionErr = err.Error()
		return
	}
	a.finishStart(sim)
}

// captureSetup is the capture colony: Broken Hull on a 100 x 100 temperate
// site, the Settled storyteller defaults, generated factions with a
// goodwill history and a team with something to show (an incapable
// colonist), with the scenario part the state needs.
func captureSetup(ui string) game.Setup {
	def, _ := scenario.Builtin("broken_hull")
	def.Parts = slices.Clone(def.Parts)
	switch ui {
	case "arch-locked":
		def.Parts = append(def.Parts, scenario.Part{Kind: scenario.PartDisallowBuilding, Building: construction.Campfire})
	case "chips":
		def.Parts = append(def.Parts, scenario.Part{Kind: scenario.PartCondition, Cond: scenario.AshfallSky})
	}
	s := game.Setup{
		Scenario: def, Storyteller: "steady", Preset: difficulty.Settled, Difficulty: difficulty.Of(difficulty.Settled),
		Site: site.Site{Biome: biome.TemperateForest, Climate: biome.DefOf(biome.TemperateForest).Climate,
			Stones: []geology.Stone{geology.Granite, geology.Marble}},
		Factions: faction.Generate(faction.DefaultSlots(), randx.New(7)),
		MapSize:  100, Season: game.SeasonSpring, Seed: 4242,
	}
	captureHistory(s.Factions)
	// The first seed whose three have someone who can't do some work.
	fx := def.Apply()
	for i := uint64(0); i < 64; i++ {
		pool := game.NewCandidatePool(s.Seed+i, fx)
		s.Colonists = pool.Selected()
		if slices.ContainsFunc(s.Colonists, func(p pawngen.Pawn) bool { return p.Incapable() != 0 }) {
			break
		}
	}
	return s
}

// captureHistory gives every moving faction forty days of goodwill and two
// events, so the Factions window has something to draw.
func captureHistory(fs []faction.Faction) {
	for i := range fs {
		f := &fs[i]
		hist := make([]int16, 0, 40)
		for d := 0; d < 40; d++ {
			v := f.Goodwill - 18 + d/2
			if d >= 14 && d < 20 { // a raid repelled
				v -= 5 * (d - 13)
			}
			hist = append(hist, int16(max(-100, min(100, v))))
		}
		f.History = hist
		f.Events = []string{"Day 14: a raid was repelled (goodwill -5)", "Day 27: a trader was welcomed (goodwill +3)"}
	}
}

// captureColonyUI opens the state's UI once the colony has run.
func (a *App) captureColonyUI(ui string) {
	sw, sh := a.size()
	a.uiSt().LastScreen = a.nav.screen() // a window opened now must survive the kit's screen-change reset
	pawns := a.renderer.snapshot()
	cardAt := func(id string) { // the tool's command button in the grid
		if c, ok := a.archToolCell(id); ok {
			x, y := c.Center()
			a.cursor = &[2]int{int(x), int(y)}
		}
	}
	if ui != "letter-intro" { // the intro letter would sit over every other shot
		for _, m := range a.renderer.world().Messages {
			if m.Letter {
				if a.msgSeen == nil {
					a.msgSeen = map[string]uint64{}
				}
				a.msgSeen[a.letterKey(m)] = 0
			}
		}
	}
	switch ui {
	case "factions-colony":
		a.cursor = &[2]int{-100, -100}
		a.openColonyFactions()
	case "bio":
		for _, p := range pawns {
			if hasBio(p) {
				a.renderer.Select(p.ID)
				a.openPaneTab("bio")
				a.renderer.centerOn(float64(p.X)+4, float64(p.Y), sw, sh)
				break
			}
		}
	case "work-incapable":
		a.win.Open, a.win.Mode = true, "work"
		for row, p := range pawns {
			for col := range workColumns {
				if _, live := workLive[col]; live && p.Incapable.Enabled(work.Type(col)) && a.cursor == nil {
					c := workCellRect(a.windowRect(), row, col)
					x, y := c.Center()
					a.cursor = &[2]int{int(x), int(y)}
				}
			}
		}
	case "options-story":
		a.opt.Tab = "story"
		a.cursor = &[2]int{-100, -100}
		a.apply(intent{kind: intentPush, screen: screenOptions})
	case "arch-hospital":
		a.openArchitect()
		a.selectArchCategory("furniture")
		a.selectArchTool("hospital_bed")
		cardAt("hospital_bed")
	case "arch-cabinet":
		a.openArchitect()
		a.selectArchCategory("production")
		a.selectArchTool("tool_cabinet")
		cardAt("tool_cabinet")
	case "arch-locked":
		a.openArchitect()
		a.selectArchCategory("furniture")
		cardAt("campfire")
	}
}
