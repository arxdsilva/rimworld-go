package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"rimworld-go/internal/application/game"
)

// Start and loading (NG-1 §2.10): the wizard's Start hands the shell a
// game.Setup (wizard.Host.Load); the shell builds the colony in steps
// behind the wizard's progress screen and installs it.

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
	return wizard.LoadPhases[0], 1, true
}

func (b *oneShotBuilder) Sim() (Simulation, error) { return b.sim, b.err }

// newGameOf is what the factory receives for a setup: the seed and the
// site's biome (the legacy start reads these) and the full setup.
func newGameOf(s *game.Setup) NewGame {
	return NewGame{Seed: s.Seed, Biome: s.Site.Biome, Setup: s}
}

// simBuilder makes the stepper for ng. The composition root's factory
// builds in one step; a stepping builder (NewSimBuilder) hooks in here.
func (a *App) simBuilder(ng NewGame) (SimBuilder, error) {
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

const (
	errNoFactory  = loadErr("no simulation factory")
	errIncomplete = loadErr("the setup is incomplete")
	errNotBuilt   = loadErr("the colony could not be built")
)

// loadColony starts building the wizard's setup s (nil: incomplete),
// keeping why it failed in sessionErr.
func (a *App) loadColony(s *game.Setup) (wizard.Loader, error) {
	if s == nil {
		a.sessionErr = errIncomplete.Error()
		return nil, errIncomplete
	}
	b, err := a.simBuilder(newGameOf(s))
	if err != nil {
		a.sessionErr = err.Error()
		return nil, err
	}
	a.sessionErr = ""
	return colonyLoader{a: a, b: b}, nil
}

// colonyLoader is a SimBuilder as the wizard's loading screen steps it.
type colonyLoader struct {
	a *App
	b SimBuilder
}

func (l colonyLoader) Step(units int) (string, float32, bool) { return l.b.Step(units) }

// Finish installs the built colony, or keeps why it failed in sessionErr.
func (l colonyLoader) Finish() error {
	sim, err := l.b.Sim()
	if err == nil && sim == nil {
		err = errNotBuilt
	}
	if err != nil {
		l.a.sessionErr = err.Error()
		return err
	}
	l.a.finishStart(sim)
	return nil
}

// finishStart installs the built colony and ends the wizard.
func (a *App) finishStart(sim Simulation) {
	a.startSession(sim)
	a.opt.SaveSlot = options.NewSlotName()
	a.opt.AutosaveDay = 0
	a.wiz.Drop() // the setup is done: the wizard's draft goes with it
}
