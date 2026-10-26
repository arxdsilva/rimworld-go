package game

// PLAY-021 electricity (POWERSIM owns this file): the colony's power
// state, the balance phase and the save round trip. The pure rules (nets,
// balance, shedding, the kinds' specs) are domain/building/power.
//
// Contract (PLAY021_PLAN.md Appendix): phasePower runs before
// phaseClimate every 50 ticks, or at once when the grid is dirty,
// integrating the exact elapsed time; in a colony without power devices
// it returns without writing any state. Nets are rebuilt only when the
// structure or a switch changes (cache key: structVer, topology version,
// store identity). Saved: battery charge, wanted and actual on/off, the
// unpowered set, shed timers, thermostats, the flare's end and the last
// balance tick (powerSave, save.go); nets, outputs and the flick index
// are derived.

// powerState is the colony's electricity; Game.pw is nil until the
// colony has a power device, so colonies without one carry nothing.
type powerState struct{} // TODO(POWERSIM): charge, on/off, shed, thermostats, flare, last, dirty, topoVer, net cache, flicks

// phasePower balances every power net (before phaseClimate).
func (g *Game) phasePower() {
	if g.pw == nil {
		return // no power devices: nothing to balance, nothing written
	}
	// TODO(POWERSIM): rebuild nets when stale, integrate since the last
	// balance, store charge, powered and shed state.
}

// exportPower is the Power save field (nil: nothing to keep).
func (g *Game) exportPower() *powerSave {
	return nil // TODO(POWERSIM)
}

// importPower restores exportPower's state (nil: an older save, or no
// power).
func (g *Game) importPower(s *powerSave) {
	if s == nil {
		g.pw = nil
		return
	}
	g.pw = &powerState{} // TODO(POWERSIM): restore the saved fields, rebuild the flick index
}
