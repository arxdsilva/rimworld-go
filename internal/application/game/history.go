package game

import (
	"slices"

	"rimworld-go/internal/domain/sim/entity"
)

// historyCap is how many History lines the colony keeps (oldest dropped).
const historyCap = 500

// historyEntry is one line of the colony's history (ENG-003, M4): the
// weighty social events (marriages, deaths, fights, recruits). Saved as
// History.
type historyEntry struct {
	Tick  uint64
	Text  string
	Kind  string
	Pawns []entity.EntityID
}

// addHistory records a History line of kind ("death", "wedding", ...)
// about pawns, stamped now.
func (g *Game) addHistory(kind, text string, pawns ...entity.EntityID) {
	g.history = append(g.history, historyEntry{Tick: uint64(g.clock.Now()), Text: text, Kind: kind, Pawns: slices.Clone(pawns)})
	if n := len(g.history); n > historyCap {
		g.history = append(g.history[:0:0], g.history[n-historyCap:]...)
	}
}

// historyViews lists the colony's history, oldest first.
func (g *Game) historyViews() []HistoryEntryView {
	if len(g.history) == 0 {
		return nil
	}
	out := make([]HistoryEntryView, len(g.history))
	for i, h := range g.history {
		ps := make([]uint64, len(h.Pawns))
		for j, p := range h.Pawns {
			ps[j] = uint64(p)
		}
		out[i] = HistoryEntryView{Tick: h.Tick, Text: h.Text, Kind: h.Kind, Pawns: ps}
	}
	return out
}
