package ship

import (
	"strings"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// EndingShip is WorldView.AX.Ending once the ship has left; LaunchTitle
// the launch letter's title (at the ship's centre and the launch tick:
// the renderer's launch flare finds it).
const (
	EndingShip  = "ship"
	LaunchTitle = "The ship has launched"
)

// Launch launches the ship part id belongs to: refused with its launch
// report while it lacks anything, else the ship lifts off the map with
// everyone asleep in its caskets, and the colony's story ends.
func (s *State) Launch(h Host, id entity.EntityID) []string {
	sh, ok := s.ShipOf(h, id)
	if !ok {
		return []string{"Not part of a ship"}
	}
	if issues := s.Issues(sh, h.Now()); len(issues) > 0 {
		b, _ := h.Building(id)
		h.Message(b.X, b.Y, "The ship can't launch yet: "+strings.Join(issues, "; ")+".")
		return issues
	}
	cx, cy := centre(h, sh)
	var names []string
	for _, c := range sh.Caskets {
		if p := s.Sleeper(c); p != 0 {
			names = append(names, h.Name(p))
			s.setSleeper(c, 0) // they go with the ship: stashed for good
		}
	}
	for _, pid := range sh.Parts {
		h.Lift(pid)
	}
	s.reactors = s.reactors[:0]
	s.nextRaid = 0
	s.launched, s.launchAt = true, h.Now()
	s.escaped = append(s.escaped, names...)
	h.Letter(LaunchTitle,
		"Fire blooms under the engines and the ship climbs out of the sky, its sleepers dreaming of a world far from here: "+
			strings.Join(names, ", ")+". This colony's story is complete.", cx, cy, false)
	h.Changed()
	return nil
}

// centre is the middle of ship sh's parts' footprints.
func centre(h Sites, sh Ship) (int, int) {
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	for _, id := range sh.Parts {
		b, _ := h.Building(id)
		cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		for _, c := range cells {
			minX, minY, maxX, maxY = min(minX, c[0]), min(minY, c[1]), max(maxX, c[0]), max(maxY, c[1])
		}
	}
	return (minX + maxX) / 2, (minY + maxY) / 2
}

// Escaped is who left aboard, in the order they boarded the ship's
// caskets (the slice is the state's, read only).
func (s *State) Escaped() []string { return s.escaped }

// LaunchAt is the launch's tick (0: no launch).
func (s *State) LaunchAt() uint64 { return s.launchAt }
