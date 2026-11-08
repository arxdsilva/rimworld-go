package devscene

import (
	"rimworld-go/internal/application/game"
	appscenario "rimworld-go/internal/application/scenario"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

// Config is a dev run's map: the -scenario set piece, the map size and the
// -hour/-day jump.
type Config struct {
	Scenario      string
	Width, Height int
	Hour          int  // jump to this hour of the scene's day; < 0 stays
	Day           int  // with Hour: land on this day or later (autosave skips day 0)
	Capturing     bool // a media capture: the grown colony gets its roofs
	// FixtureDir caches the grown colony (RIMWORLD_FIXTURE_DIR); "" builds it.
	FixtureDir string
}

// Build makes the colony a new game starts on, with the wizard's seed and
// biome: the grown or social colony, or a fresh colony with the scenario's
// set piece (none for "" or an unknown name), the -hour jump and its rooms
// roofed.
func Build(c Config, seed uint64, b biome.ID) (*game.Game, error) {
	g, err := build(c, seed, b)
	if err == nil {
		jump(g, c)
	}
	return g, err
}

func build(c Config, seed uint64, b biome.ID) (*game.Game, error) {
	switch c.Scenario {
	case "grown": // the benchmark's grown colony at Width x Height
		appscenario.CacheDir = c.FixtureDir
		g, err := appscenario.Load(appscenario.GrownColony(1, c.Width, c.Height, false))
		if err == nil && c.Capturing {
			// media captures show the colony as its builders would leave
			// it: closed rooms roofed. The benchmark fixture itself (and
			// -fpsprobe) stays as recorded in ENG-001.
			g.RoofRooms()
		}
		return g, err
	case "social": // the social colony (internal/application/scenario/social.go)
		g, _, err := appscenario.Social(seed, c.Width, c.Height)
		return g, err
	}
	g := game.NewColony(game.Config{Seed: seed, Width: c.Width, Height: c.Height, Biome: b})
	if scene, ok := scenes[c.Scenario]; ok {
		scene(g, c.Width, c.Height)
	}
	g.RoofRooms() // scenario rooms start roofed (SIM-018)
	return g, nil
}

// jump moves the clock forward to c.Hour of the scene's day (the next day's
// if that hour has passed), and to day c.Day if that is later. Every scene
// takes it, the grown and social colonies too: the probe matrix's day-cross
// and autosave cells start from it.
func jump(g *game.Game, c Config) {
	if c.Hour < 0 {
		return
	}
	now := int(g.Tick())
	day := now / clock.TicksPerDay
	if now%clock.TicksPerDay > c.Hour*clock.TicksPerHour {
		day++
	}
	g.SkipTo(max(day, c.Day), c.Hour)
}

// Bare reports the set pieces a capture shows without the scripted
// starter orders.
func Bare(scenario string) bool {
	return scenario == "walls" || scenario == "grown" || scenario == "social"
}

// Known reports whether Build knows the scenario name.
func Known(scenario string) bool {
	_, ok := scenes[scenario]
	return ok || scenario == "grown" || scenario == "social"
}
