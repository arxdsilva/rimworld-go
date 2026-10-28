package ebiten

import (
	"hash/fnv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
)

// Start and loading (NG-1 §2.10): Start turns the draft into a game.Setup
// and builds the colony in steps behind a progress screen, so a big map
// shows progress instead of freezing.

// SimBuilder builds a simulation in steps (the consumer-owned view of
// game.ColonyBuilder, wrapped by the composition root).
type SimBuilder interface {
	// Step does up to units of work and names the phase now running.
	Step(units int) (phase string, frac float32, done bool)
	// Sim is the built simulation (valid once Step reported done).
	Sim() (Simulation, error)
}

// oneShotBuilder adapts a SimFactory: it builds everything in one step.
type oneShotBuilder struct {
	make func(NewGame) (Simulation, error)
	ng   NewGame
	sim  Simulation
	err  error
	done bool
}

func (b *oneShotBuilder) Step(int) (string, float32, bool) {
	if !b.done {
		b.sim, b.err = b.make(b.ng)
		b.done = true
	}
	return loadPhases[0], 1, true
}

func (b *oneShotBuilder) Sim() (Simulation, error) { return b.sim, b.err }

// loadPhases are the loading screen's phases.
var loadPhases = []string{"Preparing the site", "Shaping the land", "Placing rock and ore", "Seeding plants and animals", "Arrival"}

// loadUnits is the work units stepped per frame.
const loadUnits = 4000

// loadState is a colony being built.
type loadState struct {
	b     SimBuilder
	Phase string
	frac  float32
}

// fnv64a hashes text.
func fnv64a(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// setupSeedOf is the setup seed (NG-1 §4.7):
// fnv64a(world seed text) ^ tile * 0x9E3779B97F4A7C15 ^ scenario seed.
func setupSeedOf(ng *newGameState) uint64 {
	text := ng.World.SeedText
	if ng.Planet != nil {
		text = ng.Planet.Params.SeedText
	}
	return fnv64a(text) ^ uint64(int64(ng.Tile))*0x9E3779B97F4A7C15 ^ ng.Scenario.Seed
}

// cloneFactions copies the generated factions so the colony owns them.
func cloneFactions(fs []faction.Faction) []faction.Faction {
	out := make([]faction.Faction, len(fs))
	for i, f := range fs {
		f.Settlements = append([]int(nil), f.Settlements...)
		f.SettlementNames = append([]string(nil), f.SettlementNames...)
		f.History = append([]int16(nil), f.History...)
		f.Events = append([]string(nil), f.Events...)
		out[i] = f
	}
	return out
}

// BuildSetup turns the draft into a game.Setup. It needs a generated
// planet, a landing tile and the candidate pool.
func (ng *newGameState) BuildSetup() (game.Setup, bool) {
	if ng.Planet == nil || ng.Tile < 0 || ng.Pool == nil {
		return game.Setup{}, false
	}
	diff := difficulty.Of(ng.Preset)
	if ng.Preset == difficulty.Custom {
		diff = ng.Custom
	}
	params := ng.Planet.Params
	params.Factions = append([]faction.Slot(nil), params.Factions...)
	st := ng.Planet.Site(ng.Tile)
	return game.Setup{
		Scenario:    ng.Scenario,
		Storyteller: ng.Storyteller,
		Preset:      ng.Preset,
		Difficulty:  diff,
		SaveMode:    ng.SaveMode,
		World:       params,
		Tile:        ng.Tile,
		Site:        st,
		Factions:    cloneFactions(ng.Planet.Factions),
		MapSize:     ng.MapSize,
		Season:      ng.ResolvedSeason(),
		Colonists:   ng.Pool.Selected(),
		Seed:        setupSeedOf(ng),
	}, true
}

// newGameOf is what the factory receives for a setup: the seed and the
// site's biome (the legacy start reads these) and the full setup.
func newGameOf(s *game.Setup) NewGame {
	return NewGame{Seed: s.Seed, Biome: s.Site.Biome, Setup: s}
}

// simBuilder makes the stepper for ng. The composition root's factory
// builds in one step; a stepping builder hooks in here.
func (a *App) simBuilder(ng NewGame) (SimBuilder, error) {
	if a.ng != nil && a.ng.BuildHook != nil && ng.Setup != nil {
		return a.ng.BuildHook(ng) // tests, and later the stepping factory
	}
	if a.cfg.NewSimBuilder != nil && ng.Setup != nil {
		return a.cfg.NewSimBuilder(ng)
	}
	if a.cfg.NewSim == nil {
		return nil, errNoFactory
	}
	return &oneShotBuilder{make: a.cfg.NewSim, ng: ng}, nil
}

type loadErr string

func (e loadErr) Error() string { return string(e) }

const errNoFactory = loadErr("no simulation factory")

// beginLoading is the Start button: build the setup, start the builder and
// run its first step (a one-step builder is done at once).
func (a *App) beginLoading() {
	ng := a.draft()
	setup, ok := ng.BuildSetup()
	if !ok {
		a.sessionErr = "the setup is incomplete"
		return
	}
	b, err := a.simBuilder(newGameOf(&setup))
	if err != nil {
		a.sessionErr = err.Error()
		return
	}
	a.sessionErr = ""
	ng.Load = &loadState{b: b, Phase: loadPhases[0]}
	a.loadingStep()
}

// loadingStep advances the colony builder one frame.
func (a *App) loadingStep() {
	ng := a.ng
	if ng == nil || ng.Load == nil {
		return
	}
	ld := ng.Load
	phase, frac, done := ld.b.Step(loadUnits)
	if phase == "" {
		phase = loadPhases[min(int(frac*float32(len(loadPhases))), len(loadPhases)-1)]
	}
	ld.Phase, ld.frac = phase, frac
	if !done {
		return
	}
	sim, err := ld.b.Sim()
	ng.Load = nil
	if err != nil || sim == nil {
		if err == nil {
			err = loadErr("the colony could not be built")
		}
		a.sessionErr = err.Error()
		return
	}
	a.finishStart(sim)
}

// finishStart installs the built colony and ends the wizard.
func (a *App) finishStart(sim Simulation) {
	a.startSession(sim)
	a.saveSlot = newSlotName()
	a.autosaveDay = 0
	a.ng = nil // the setup is done: the wizard's draft goes with it
}

// drawLoading paints the loading screen over the setup backdrop.
func (a *App) drawLoading(screen *ebiten.Image, sw, sh int) {
	a.drawSetupBackdrop(screen, sw, sh, a.ng.Planet)
	ld := a.ng.Load
	box := winRect(sw, sh).Centered(560, 150)
	drawPanel(screen, box.X, box.Y, box.W, box.H)
	st := tsLarge
	st.Color = uiHeading
	drawText(screen, "Preparing your colony", float64(box.X+20), float64(box.Y+16), st)
	drawProgress(screen, rect{X: box.X + 20, Y: box.Y + 56, W: box.W - 40, H: 60}, ld.Phase, float64(ld.frac), "")
}
