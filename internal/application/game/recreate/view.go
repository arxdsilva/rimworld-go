package recreate

import (
	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
)

// View is the recreation read side (the game's views): never makes or
// changes state, so a view taken or not leaves the simulation the same.
type View struct {
	h Host
	s *State // nil: no colonist has a tolerance yet
}

// NewView is the read side over host h and state s (may be nil).
func NewView(h Host, s *State) View { return View{h, s} }

// Acting is what a colonist on session j reads as and the cell it faces
// (PawnView.Acting, ActX, ActY): "play chess" toward the table.
func (v View) Acting(j job.Record) (string, int, int) {
	b, ok := v.h.Building(j.Dest)
	sp, rec := SpecOf(b.Kind)
	if !ok || !rec {
		return j.Kind.String(), j.DestX, j.DestY
	}
	return sp.Verb, j.DestX, j.DestY
}

// Joy is the joy kind colonist id is enjoying now ("" none).
func (v View) Joy(id entity.EntityID) string {
	j := v.h.Job(id)
	if !j.Active() {
		return ""
	}
	if Uses(j.Kind) {
		b, _ := v.h.Building(j.Dest)
		sp, _ := SpecOf(b.Kind)
		return sp.Joy.String()
	}
	return joyOfIdle(j.Kind).String()
}

// Tolerances are colonist id's joy kinds with a tolerance, in kind
// order (PawnView.AX.Tolerance).
func (v View) Tolerances(id entity.EntityID) []axview.JoyTol {
	if v.s == nil {
		return nil
	}
	t := v.s.Tolerance(id)
	var out []axview.JoyTol
	for k := recreation.JoySolitary; k < recreation.NumJoyKinds; k++ {
		if pct := t.Pct(k); pct > 0 || t.IsBored(k) {
			out = append(out, axview.JoyTol{Kind: k.String(), Pct: pct, Bored: t.IsBored(k)})
		}
	}
	return out
}

// PieceView is a recreation piece's pane lines' data (BuildingView.AX.Joy
// and Users): its joy kind and the colonists using it.
func (v View) PieceView(bid entity.EntityID) (joy string, users int) {
	b, ok := v.h.Building(bid)
	sp, rec := SpecOf(b.Kind)
	if !ok || !rec {
		return "", 0
	}
	return sp.Joy.String(), usersOf(v.h, bid)
}
