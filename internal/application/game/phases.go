package game

import "time"

// Phase names, in Step order (PhaseObserver).
const (
	PhaseCommands  = "commands"
	PhasePlants    = "plants"
	PhasePower     = "power" // PLAY-021: before climate
	PhaseClimate   = "climate"
	PhaseInfection = "infection"
	PhaseNeeds     = "needs"
	PhaseAI        = "ai"
	PhaseJobs      = "jobs"
	PhaseSocial    = "social" // ENG-003: after jobs
	PhaseAnimals   = "animals"
	PhaseThreats   = "threats"
	PhaseIncidents = "incidents"
	PhaseWorld     = "world" // ENG-003: after incidents
	PhaseWeather   = "weather"
	PhaseMovement  = "movement"
)

// Phases lists the phase names in Step order.
func Phases() []string {
	return []string{PhaseCommands, PhasePlants, PhasePower, PhaseClimate, PhaseInfection, PhaseNeeds, PhaseAI,
		PhaseJobs, PhaseSocial, PhaseAnimals, PhaseThreats, PhaseIncidents, PhaseWorld, PhaseWeather, PhaseMovement}
}

// PhaseObserver receives the wall-clock duration of every Step phase
// (NG-1 §6.2). It is observation only: wall-clock time never feeds
// gameplay, and a game steps identically with or without one. Set it only
// from cmd/* (the benchmark runner) and benchmarks.
type PhaseObserver func(phase string, elapsed time.Duration)

// SetPhaseObserver installs o (nil removes it).
func (g *Game) SetPhaseObserver(o PhaseObserver) { g.phaseObs = o }

// stepObserved is Step with each phase timed for the observer.
func (g *Game) stepObserved(n int) {
	phases := [...]struct {
		name string
		run  func()
	}{
		{PhaseCommands, g.phaseCommands}, {PhasePlants, g.phasePlants}, {PhasePower, g.phasePower},
		{PhaseClimate, g.phaseClimate},
		{PhaseInfection, g.phaseInfection}, {PhaseNeeds, g.phaseNeeds}, {PhaseAI, g.phaseAI},
		{PhaseJobs, g.phaseJobs}, {PhaseSocial, g.phaseSocial}, {PhaseAnimals, g.phaseAnimals},
		{PhaseThreats, g.phaseThreats}, {PhaseIncidents, g.phaseIncidents}, {PhaseWorld, g.phaseWorld},
		{PhaseWeather, g.phaseWeather}, {PhaseMovement, g.phaseMovement},
	}
	for i := 0; i < n; i++ {
		for _, p := range phases {
			t0 := time.Now()
			p.run()
			g.phaseObs(p.name, time.Since(t0))
		}
		g.clock.Advance(1)
	}
}
