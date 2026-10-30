package prison

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/entity"
)

// visitStagger spreads first visits over the interval by id.
const visitStagger = 7919

// Prison holds the colony's prisoners: each one's state (mode,
// resistance, next visit), nil until the first capture.
type Prison struct {
	h  Host
	st map[entity.EntityID]*prisoner.State
}

// New builds a Prison over h.
func New(h Host) *Prison { return &Prison{h: h} }

// Is reports whether id is the colony's prisoner.
func (p *Prison) Is(id entity.EntityID) bool { return p.st[id] != nil }

// State is prisoner id's state (nil: no prisoner). The caller may change
// it in place.
func (p *Prison) State(id entity.EntityID) *prisoner.State { return p.st[id] }

// Drop forgets prisoner id (it died, or left the prison another way).
func (p *Prison) Drop(id entity.EntityID) { delete(p.st, id) }

// Export is the live state, for the save to copy (nil: none).
func (p *Prison) Export() map[entity.EntityID]*prisoner.State { return p.st }

// Import restores Export's state.
func (p *Prison) Import(m map[entity.EntityID]*prisoner.State) { p.st = m }

// SetMode sets prisoner id's interaction mode: what its warden does on
// the next visit (Release and Execute do not wait for the interval).
func (p *Prison) SetMode(id entity.EntityID, mode uint8) {
	if st := p.st[id]; st != nil && prisoner.Mode(mode) <= prisoner.Execute {
		st.Mode = prisoner.Mode(mode)
	}
}

// Take makes target a prisoner, taken by captor at now: a fresh
// resistance and the Recruit mode, its first visit staggered by id.
func (p *Prison) Take(target, captor entity.EntityID, now uint64) {
	if p.st == nil {
		p.st = map[entity.EntityID]*prisoner.State{}
	}
	roll := int32(p.h.Roll(1001))
	p.st[target] = &prisoner.State{
		Mode: prisoner.Recruit, Resistance: prisoner.InitialResistance(roll),
		NextVisit: now + uint64(target)*visitStagger%prisoner.VisitInterval, Captor: uint64(captor),
		Calm: now + EscapeGrace, // escape.go: no break straight after the capture
	}
}

// Release frees prisoner id (by: the warden who let it go, 0 for the
// player's order): it gets up from its bed and walks off the map.
func (p *Prison) Release(id, by entity.EntityID) {
	if !p.Is(id) {
		return
	}
	h := p.h
	now := h.Now()
	delete(p.st, id)
	h.Interrupt(id, job.Cancelled)
	h.ReleaseBed(id)
	r := h.Held(id)
	r.Leaving, r.Hungry, r.Idle, r.RetryAt = true, false, now, 0
	if by != 0 {
		r.Warden = by
	}
	h.SetHeld(id, r)
	x, y, _ := h.Pos(id)
	h.Message(x, y, h.PersonName(id)+" has been released.")
}

func abs(v int) int { return max(v, -v) }
