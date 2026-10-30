package prison

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Prison breaks (CBT-4). wiki: Prison break (Events): each prisoner may
// start one, so more prisoners break out more often; the prisoner's own
// room always joins it and every other prison room within 20 tiles joins
// half the time; the escapees open the doors, may fight, and try to get
// away. The wiki publishes no timing: EscapeMTB (per prisoner, checked
// hourly), EscapeGrace (after a capture) and EscapeCalm (for the
// prisoners left behind after a break) are original tuning, so a prison
// never breaks out twice in a row (TestPrisonBreakCalm).

const (
	// EscapeMTB is the mean time between one prisoner's breaks.
	EscapeMTB = 60 * clock.TicksPerDay
	// EscapeGrace: a new prisoner tries nothing for this long.
	EscapeGrace = 3 * clock.TicksPerDay
	// EscapeCalm: after a break the prisoners left try nothing for this
	// long.
	EscapeCalm = 10 * clock.TicksPerDay
	// escapeReach is how far a break spreads to other rooms (wiki: 20
	// tiles); escapeStagger spreads the hourly checks.
	escapeReach   = 20
	escapeStagger = 613
)

// breakOut is prisoner id's hourly roll for a prison break at now; it
// reports whether one started (id joins it).
func (p *Prison) breakOut(id entity.EntityID, now uint64) bool {
	st := p.st[id]
	if st == nil || (now+uint64(id)*escapeStagger)%clock.TicksPerHour != 0 || now < st.Calm {
		return false
	}
	h := p.h
	if uint64(h.Roll(int(EscapeMTB))) >= clock.TicksPerHour {
		return false
	}
	x, y, _ := h.Pos(id)
	room := h.RoomOf(id)
	var out []entity.EntityID
	joins := map[int]bool{room: true} // wiki: the starter's room always joins
	for _, o := range h.RaiderIDs() {
		ost := p.st[o]
		if ost == nil {
			continue
		}
		ox, oy, ok := h.Pos(o)
		if !ok || max(abs(ox-x), abs(oy-y)) > escapeReach {
			continue
		}
		r := h.RoomOf(o)
		in, rolled := joins[r]
		if !rolled {
			in = h.Roll(2) == 0 // wiki: other rooms 50 %
			joins[r] = in
		}
		if in && (r != 0 || o == id) {
			out = append(out, o)
		}
	}
	for _, o := range out {
		delete(p.st, o)
		h.Interrupt(o, job.Cancelled)
		h.ReleaseBed(o)
		h.Escape(o)
	}
	for _, o := range h.RaiderIDs() {
		if ost := p.st[o]; ost != nil {
			ost.Calm = max(ost.Calm, now+EscapeCalm)
		}
	}
	h.Breakout(id, len(out))
	return true
}
