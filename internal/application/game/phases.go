package game

import (
	"time"

	"rimworld-go/internal/application/game/cadence"
	"rimworld-go/internal/application/game/sched"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/temperature"
)

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

// SetPhaseObserver installs o (nil removes it): the scheduler times its systems for it.
func (g *Game) SetPhaseObserver(o PhaseObserver) { g.systems.Observe(sched.Observer(o)) }

// runPhases runs the current map's pass of the phase systems (game/sched).
func (g *Game) runPhases() { g.systems.Pass(primary(g), uint64(g.clock.Now())) }

// phaseSystems are the 15 phases in Step order, wrapped, and the colony's
// work (primary) and whole-pass cadence gates lifted out of them in place.
func phaseSystems(g *Game) *sched.Schedule {
	w, now, hour := sched.Wrap, func() uint64 { return uint64(g.clock.Now()) }, uint64(clock.TicksPerHour)
	talk := func() bool { return len(g.pawnIDs) > 0 || len(g.maps) > 1 } // the colony's social upkeep: anyone home, or a second map
	world := func() bool { return g.worldSt != nil }
	return sched.Must(Phases(), []sched.System{
		w(PhaseCommands, PhaseCommands, g.phaseCommands), w(PhasePlants, PhasePlants, g.phasePlants), w(PhasePower, PhasePower, g.phasePower),
		w("heat", PhaseClimate, g.heatRooms).At(temperature.EqualiseInterval, 0).Then("rot"), // game/thermo (TEMP-1): the wiki's 120-tick room equalisation
		w("rot", PhaseClimate, func() { g.craft.Condition.Rot(climateInterval) }).At(climateInterval, cadence.OffRot).Then("wear"),
		w("wear", PhaseClimate, func() { g.craft.Condition.Weather(now() + cadence.OffWear) }).At(hour, cadence.OffWear).Then(PhaseClimate), // PRD-2, the hour it closes
		w(PhaseClimate, PhaseClimate, g.phaseClimate).At(climateInterval, 0),
		w(PhaseMedical, PhaseMedical, g.phaseMedical).At(hour, cadence.OffMedical),
		w(PhaseNeeds, PhaseNeeds, g.phaseNeeds), w(PhaseAI, PhaseAI, g.phaseAI), w(PhaseJobs, PhaseJobs, g.phaseJobs),
		w("social-day", PhaseSocial, func() { g.socialDay(now()) }).ForColony().At(clock.TicksPerDay, cadence.OffSocialDay).When(talk).Then("gather"),
		w("gather", PhaseSocial, func() { g.gatherTick(now()) }).ForColony().When(talk).Then(PhaseSocial), // gathering.go: weddings and parties (M5)
		w(PhaseSocial, PhaseSocial, g.phaseSocial), w(PhaseAnimals, PhaseAnimals, g.phaseAnimals),
		w("arrival", PhaseThreats, func() { g.arrivalTick(now()) }).ForColony().When(g.hasSetup).Then("losses"), // arrival.go (NG-1)
		w("losses", PhaseThreats, func() { g.checkLosses(now()) }).At(hour, cadence.OffStory).When(g.hasSetup).Then("raids-over"),
		w("raids-over", PhaseThreats, func() { g.checkRaidsOver(now()) }).At(hour, cadence.OffStory).When(g.hasSetup).Then("story"), // factions.go
		w("story", PhaseThreats, func() { storyHour(g, now()) }).ForColony().At(hour, cadence.OffStory).When(g.hasSetup).Then("storyteller"),
		w("storyteller", PhaseThreats, func() { fireStoryteller(g, now()) }).ForColony().When(func() bool { return g.threatsOn && now() >= g.nextThreat }).Then(PhaseThreats),
		w(PhaseThreats, PhaseThreats, g.phaseThreats), w(PhaseIncidents, PhaseIncidents, g.phaseIncidents),
		w("visitors", PhaseWorld, func() { visitsOf(g).Tick(aidOf(g), now()) }).ForColony().When(func() bool { return world() && len(g.worldSt.visitors) > 0 }).Then(PhaseWorld), // game/visit
		w(PhaseWorld, PhaseWorld, g.phaseWorld).When(world).Then("world-upkeep"),
		w("world-upkeep", PhaseWorld, func() { worldUpkeep(g, now()) }).ForColony().At(worldStep, 0).When(world),
		w(PhaseWeather, PhaseWeather, g.phaseWeather), w(PhaseMovement, PhaseMovement, g.phaseMovement),
	})
}
