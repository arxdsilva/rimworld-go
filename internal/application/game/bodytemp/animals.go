package bodytemp

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/infection"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// AnimalHost is what animal exposure needs from the game. The animal
// itself never crosses it (an animal handed to an interface escapes to
// the heap on every tick): Step reports what the game must follow up.
type AnimalHost interface {
	// AirAt is the air temperature where animal id stands (tenths).
	AirAt(id entity.EntityID) int
	// Roll is in [0, n) (the body stream: frostbite).
	Roll(n int) int
}

// Outcome is what one Step left for the game to follow up.
type Outcome uint8

const (
	// Same: nothing the game must look at.
	Same Outcome = iota
	// Changed: the body's stages or wounds moved: keep or drop the
	// body, apply the downing rule, index the patient.
	Changed
	// Died: the animal died of exposure (or of its frostbite).
	Died
)

// FrostbiteFrom is the hypothermia frostbite needs (wiki: 37 %).
const FrostbiteFrom = exposure.Full / 1000 * body.FrostbiteMinHypothermia

// Animals steps one animal's exposure at a time (game: tickNeeds, on the
// animal's own decision tick).
type Animals struct {
	H AnimalHost
	L Ledger
}

// Step is DecideEvery ticks of exposure for animal id (state a) in its
// species' comfortable range (animal.Comfort, the wiki's): hypothermia
// and heatstroke build and recover by the pawns' formulas and their
// stages land on its body, a hive insect taking hypothermic slowdown
// instead (it never dies of it). It reports what the game must follow
// up. A machine, or an animal in bearable air with nothing to recover
// from, costs a lookup.
func (c Animals) Step(id entity.EntityID, a *animal.State, now uint64) Outcome {
	lo, hi, ok := animal.Comfort(a.Species)
	if !ok || a.Dead() {
		return Same
	}
	b, t, s := exposure.Bands{Min: lo, Max: hi}, c.H.AirAt(id), c.L[id]
	if s == (Sev{}) && exposure.HypoGain(t, b) == 0 && exposure.HeatGain(t, b) == 0 {
		return Same
	}
	s.Hypo, s.Heat = exposure.Step(s.Hypo, s.Heat, t, animal.DecideEvery, b)
	slows := animal.Slows(a.Species)
	if !slows && (s.Hypo >= exposure.Full || s.Heat >= exposure.Full) {
		delete(c.L, id)
		return Died
	}
	c.L.Set(id, s)
	hypo, heat := exposure.Permille(s.Hypo), exposure.Permille(s.Heat)
	if a.Body == nil {
		if hypo == 0 && heat == 0 {
			return Same
		}
		a.Body = animal.NewBody(a.Species)
	}
	cold := body.CondHypothermia
	if slows {
		cold = body.CondHypothermicSlowdown
	}
	changed := body.SetExposure(a.Body, cold, hypo)
	changed = body.SetExposure(a.Body, body.CondHeatstroke, heat) || changed
	if !slows && t <= body.FrostbiteMaxTemp && c.frostbite(a, s.Hypo, now) {
		if a.Dead() {
			delete(c.L, id)
			return Died
		}
		changed = true
	}
	if changed {
		return Changed
	}
	return Same
}

// frostbite rolls this decision's frostbite chance at hypothermia hypo
// (exposure.FrostbiteChance, per 60 ticks: the cadence) and strikes an
// extremity of a's body (paws, ears, nose, tail, feet: body.frostParts),
// adding its damage to the animal's hit points and scheduling the
// wound's infection roll 6–18 hours on, as a bite's (woundAnimal).
func (c Animals) frostbite(a *animal.State, hypo int32, now uint64) bool {
	p := exposure.FrostbiteChance(hypo, FrostbiteFrom)
	if p == 0 || int32(c.H.Roll(1_000_000)) >= p {
		return false
	}
	_, dmg, ok := body.StrikeFrostbite(a.Body, c.H.Roll, now)
	if !ok {
		return false
	}
	a.Injury += dmg
	at := now + uint64(infection.DelayMin+c.H.Roll(infection.DelaySpan+1))*clock.TicksPerHour
	for i := range a.Body.Wounds {
		if w := &a.Body.Wounds[i]; w.Kind == body.Frostbite && w.At == now && w.InfectAt == 0 {
			w.InfectAt = at
		}
	}
	return true
}
