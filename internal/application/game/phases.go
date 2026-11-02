package game

import "time"

// Phase names, in Step order (PhaseObserver).
const (
	PhaseCommands  = "commands"
	PhasePlants    = "plants"
	PhasePower     = "power" // PLAY-021: before climate
	PhaseClimate   = "climate"
	PhaseMedical   = "medical" // MED-01: bleeding, blood loss, infection (hourly)
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
	return []string{PhaseCommands, PhasePlants, PhasePower, PhaseClimate, PhaseMedical, PhaseNeeds, PhaseAI,
		PhaseJobs, PhaseSocial, PhaseAnimals, PhaseThreats, PhaseIncidents, PhaseWorld, PhaseWeather, PhaseMovement}
}

// PhaseObserver receives the wall-clock duration of every Step phase
// (NG-1 §6.2). It is observation only: wall-clock time never feeds
// gameplay, and a game steps identically with or without one. Set it only
// from cmd/* (the benchmark runner) and benchmarks.
type PhaseObserver func(phase string, elapsed time.Duration)

// SetPhaseObserver installs o (nil removes it).
func (g *Game) SetPhaseObserver(o PhaseObserver) { g.phaseObs = o }

// runPhases runs the current map's 15 phases in Step order, timing each
// for the observer when one is set.
func (g *Game) runPhases() {
	phases := [...]struct {
		name string
		run  func(*Game)
	}{
		{PhaseCommands, (*Game).phaseCommands}, {PhasePlants, (*Game).phasePlants}, {PhasePower, (*Game).phasePower},
		{PhaseClimate, (*Game).phaseClimate},
		{PhaseMedical, (*Game).phaseMedical}, {PhaseNeeds, (*Game).phaseNeeds}, {PhaseAI, (*Game).phaseAI},
		{PhaseJobs, (*Game).phaseJobs}, {PhaseSocial, (*Game).phaseSocial}, {PhaseAnimals, (*Game).phaseAnimals},
		{PhaseThreats, (*Game).phaseThreats}, {PhaseIncidents, (*Game).phaseIncidents}, {PhaseWorld, (*Game).phaseWorld},
		{PhaseWeather, (*Game).phaseWeather}, {PhaseMovement, (*Game).phaseMovement},
	}
	for _, p := range phases {
		if g.phaseObs == nil {
			p.run(g)
			continue
		}
		t0 := time.Now()
		p.run(g)
		g.phaseObs(p.name, time.Since(t0))
	}
}
