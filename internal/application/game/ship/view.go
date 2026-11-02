package ship

import (
	"slices"

	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// FillBuilding sets part id's fields of its view: the ship it belongs
// to, a reactor's countdown, a casket's sleeper.
func (s *State) FillBuilding(h Host, v *axview.Building, id entity.EntityID, b construction.Building) {
	if !IsPart(b.Kind) {
		return
	}
	if sh, ok := s.ShipOf(h, id); ok {
		v.ShipID = sh.ID
	}
	switch b.Kind {
	case construction.ShipReactor:
		v.Countdown = -1
		if r := s.reactor(id); r != nil {
			v.Countdown = hourly(r.left(h.Now()))
		}
	case construction.ShipCasket:
		if p := s.Sleeper(id); p != 0 {
			v.Sleeper, v.SleeperID = h.Name(p), uint64(p)
		}
	}
}

// FillWorld sets the world view's ship fields: the launch report of the
// primary ship and its countdown, and the ending once it has left.
func (s *State) FillWorld(h Host, w *axview.World) {
	w.ShipLaunched = s.launched
	if s.launched {
		w.Ending = EndingShip
		w.Escaped = slices.Clone(s.escaped)
	}
	sh, ok := s.Primary(h)
	if !ok {
		return
	}
	now := h.Now()
	w.ShipID, w.Countdown, w.LaunchIssues = sh.ID, hourly(s.left(sh, now)), s.Issues(sh, now)
}

// hourly is a countdown rounded up to the whole hour, so the views change
// once an hour, not every tick (-1 and 0 stay).
func hourly(left int) int {
	if left <= 0 {
		return left
	}
	return (left + clock.TicksPerHour - 1) / clock.TicksPerHour * clock.TicksPerHour
}
