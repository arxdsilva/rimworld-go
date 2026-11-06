package ship

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Apply runs one of the ship's commands (StartReactor, BoardShip,
// LeaveShip, LaunchShip); a refusal is a message at the part.
func (s *State) Apply(h Host, c command.Command) {
	id := entity.EntityID(c.Target)
	why, what := "", ""
	switch c.Kind {
	case command.StartReactor:
		why, what = s.Start(h, id), "start the reactor"
	case command.BoardShip:
		why, what = s.Board(h, entity.EntityID(c.Pawn), id), "board the ship"
	case command.LeaveShip:
		why, what = s.Leave(h, id), "wake the sleeper"
	case command.LaunchShip:
		s.Launch(h, id) // refused with the launch report in a message
	}
	if why != "" {
		b, _ := h.Building(id)
		h.Message(b.X, b.Y, "Can't %s: %s.", what, why)
	}
}

// Order is pawn's right-click entry on building id: board an empty ship
// casket (ok false: none).
func (s *State) Order(h Host, pawn, id entity.EntityID) (string, command.Command, bool) {
	if b, ok := h.Building(id); !ok || b.Kind != construction.ShipCasket || s.Sleeper(id) != 0 || s.launched {
		return "", command.Command{}, false
	}
	return "Board the ship stasis casket", command.Command{Kind: command.BoardShip, Pawn: uint64(pawn), Target: uint64(id)}, true
}
