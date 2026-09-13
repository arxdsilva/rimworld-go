package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

var scenarios = []struct {
	name, desc, summary string
}{
	{
		name:    "Emergency landing",
		desc:    "A small crew survives a hard landing with leftover gear and a working fabricator.",
		summary: "3 colonists  |  mid-tech  |  modest supplies",
	},
	{
		name:    "Frontier camp",
		desc:    "A low-tech group arrives with tools, seeds, and little else.",
		summary: "5 colonists  |  low-tech  |  scarce supplies",
	},
	{
		name:    "Sole survivor",
		desc:    "One colonist, a wreck, and a long walk to safety.",
		summary: "1 colonist  |  mixed tech  |  hard start",
	},
}

var storytellers = []struct {
	name, desc string
}{
	{"Measured tide", "Incidents ramp as the colony grows. Predictable pressure."},
	{"Quiet years", "Fewer threats, more room to build. Forgiving pacing."},
	{"Wild dice", "Quiet stretches and sudden spikes. High variance."},
}

var difficulties = []string{"Forgiving", "Standard", "Harsh"}

var stubPawns = []string{
	"Mira   grower / cook     no violence incapacity",
	"Hal    constructor       modest shooting",
	"Rin    researcher        frail, good intellectual",
}

func (a *App) scenarioButtons(sw, sh int) []button {
	specs := make([]buttonSpec, 0, 5)
	for i, s := range scenarios {
		specs = append(specs, buttonSpec{
			id:       "scenario." + itoa(i),
			label:    s.name,
			selected: a.scenario == i,
			w:        260,
			h:        40,
		})
	}
	left := stackButtons(56, 120, 260, 40, 8, specs)
	left = append(left, button{
		id:       "scenario.edit",
		label:    "Create / edit custom",
		x:        56,
		y:        300,
		w:        260,
		h:        32,
		disabled: true,
		reason:   "not implemented",
	})
	return append(left, a.footer(sw, sh, "Next", "wizard.next")...)
}

func (a *App) drawScenario(screen *ebiten.Image, sw, sh int) {
	s := scenarios[a.scenario]
	a.paintMenu(screen, sw, sh, "New Game — Scenario", []string{
		"Left: choose a starting situation. Center: description.",
		"Right: starting conditions. Plugins may register scenarios later.",
		"",
		"Selected: " + s.name,
		s.desc,
		s.summary,
	}, a.scenarioButtons(sw, sh))
}

func (a *App) storytellerButtons(sw, sh int) []button {
	specs := make([]buttonSpec, 0, 6)
	for i, s := range storytellers {
		specs = append(specs, buttonSpec{
			id:       "storyteller." + itoa(i),
			label:    s.name,
			selected: a.storyteller == i,
			w:        280,
			h:        40,
		})
	}
	for i, name := range difficulties {
		specs = append(specs, buttonSpec{
			id:       "difficulty." + itoa(i),
			label:    "Difficulty: " + name,
			selected: a.difficulty == i,
			w:        280,
			h:        32,
		})
	}
	left := stackButtons(56, 120, 280, 40, 8, specs)
	left = append(left, button{
		id:       "storyteller.custom",
		label:    "Customize",
		x:        56,
		y:        420,
		w:        280,
		h:        32,
		disabled: true,
		reason:   "not implemented",
	})
	return append(left, a.footer(sw, sh, "Next", "wizard.next")...)
}

func (a *App) drawStoryteller(screen *ebiten.Image, sw, sh int) {
	s := storytellers[a.storyteller]
	a.paintMenu(screen, sw, sh, "Storyteller & Difficulty", []string{
		"Selected: " + s.name,
		s.desc,
		"",
		"Preset: " + difficulties[a.difficulty],
		"Custom threat/economy/mood sliders are not wired yet.",
	}, a.storytellerButtons(sw, sh))
}

func (a *App) worldGenButtons(sw, sh int) []button {
	btns := []button{{
		id:       "world.generate",
		label:    "Generate",
		x:        56,
		y:        280,
		w:        180,
		h:        36,
		disabled: true,
		reason:   "world generation not implemented",
	}}
	return append(btns, a.footer(sw, sh, "Next", "wizard.next")...)
}

func (a *App) drawWorldGen(screen *ebiten.Image, sw, sh int) {
	a.paintMenu(screen, sw, sh, "World Generation", []string{
		"Seed: 1  (placeholder)",
		"Coverage: small  |  Temperature: temperate  |  Rainfall: average",
		"Faction density: default",
		"",
		"Generate will build a campaign world later. Next keeps this stub",
		"and continues to landing-site selection.",
	}, a.worldGenButtons(sw, sh))
}

func (a *App) landingButtons(sw, sh int) []button {
	return a.footer(sw, sh, "Next", "wizard.next")
}

func (a *App) drawLanding(screen *ebiten.Image, sw, sh int) {
	a.paintMenu(screen, sw, sh, "Landing Site", []string{
		"World map is a placeholder. Selected tile (12, 8):",
		"",
		"Biome: temperate grassland",
		"Terrain: flat  |  Elevation: low  |  Growing period: long",
		"Temperature: mild  |  Rainfall: average  |  Forage: fair",
		"Nearby settlements: unknown",
	}, a.landingButtons(sw, sh))
}

func (a *App) pawnButtons(sw, sh int) []button {
	btns := []button{
		{
			id:       "pawns.reroll",
			label:    "Reroll",
			x:        56,
			y:        300,
			w:        140,
			h:        32,
			disabled: true,
			reason:   "not implemented",
		},
		{
			id:       "pawns.rename",
			label:    "Rename",
			x:        208,
			y:        300,
			w:        140,
			h:        32,
			disabled: true,
			reason:   "not implemented",
		},
	}
	return append(btns, a.footer(sw, sh, "Start", "wizard.start")...)
}

func (a *App) drawPawns(screen *ebiten.Image, sw, sh int) {
	body := []string{
		"Candidate roster (placeholder; not the live simulation):",
		"",
	}
	body = append(body, stubPawns...)
	body = append(body, "", "Team warnings will flag missing doctor/cook/grower later.")
	if a.sessionErr != "" {
		body = append(body, "", "start failed: "+a.sessionErr)
	}
	a.paintMenu(screen, sw, sh, "Starting Pawns", body, a.pawnButtons(sw, sh))
}

func itoa(i int) string {
	return string(rune('0' + i))
}
