package game

import "rimworld-go/internal/domain/sim/entity"

// historyEntry is one line of the colony's history (ENG-003, M4): the
// weighty social events (marriages, deaths, fights, recruits). Saved as
// History.
type historyEntry struct {
	Tick  uint64
	Text  string
	Kind  string
	Pawns []entity.EntityID
}
