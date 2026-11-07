// Package letters is the letter stack (PLAY-018): every message posted with
// Letter set also becomes a saved letter that stays in the right-side stack
// until the player dismisses it (command.DismissLetter). The message itself
// is posted as before (the ticker, pause-on-threat and the intro window read
// it); the stack only adds a longer memory and a stable id.
//
// The stack knows nothing of the game: the game decides what a letter's
// place, join offer or demand means (its glue sits beside post, in
// climate.go).
package letters

import (
	"slices"

	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/msgfmt"
)

// Max is how many letters the stack keeps: the oldest are dropped.
const Max = 40

// Letter kinds colour a letter: threat (raids, break-ins, kidnappings),
// caution (bad news that is not an attack), info (arrivals, the intro) and
// good (joins, weddings, parties).
const (
	Threat  = "threat"
	Caution = "caution"
	Info    = "info"
	Good    = "good"
)

// KindOf is the kind a letter message shows: its own kind when it is one of
// the four, else threat for a danger message and info for the rest.
func KindOf(kind string, danger bool) string {
	switch kind {
	case Threat, Caution, Info, Good:
		return kind
	}
	if danger {
		return Threat
	}
	return Info
}

// Letter is one letter as kept and saved.
type Letter struct {
	ID, Tick uint64
	Kind     string
	Title    string
	Text     string
	Msg      msgfmt.Text // Text as its catalog key and args (OPT-001)
	X, Y     int
	Map      uint32 // the game's map id (MAP-0)
	Focus    entity.EntityID
	Intro    bool
	// Join: the rescued stranger whose offer to join this letter carries
	// (0: none; SIM-063). AcceptLetter takes it in, dismissing declines.
	Join entity.EntityID
	// Demand: a caravan's encounter asks Give or Refuse by this letter
	// (command.EncounterAnswer, MAP-08).
	Demand bool
}

// State is the stack, oldest first, and how many letters were ever issued:
// ids are never reused, so a dismissed letter's id stays dead. Version
// moves on every change, so a read model can tell when to rebuild.
type State struct {
	list []Letter
	next uint64
	ver  uint64
}

// Add puts l on the stack with the next id and returns that id; the
// oldest letters past Max are dropped.
func (s *State) Add(l Letter) uint64 {
	s.next++
	l.ID = s.next
	s.list = append(s.list, l)
	if n := len(s.list); n > Max {
		s.list = append(s.list[:0:0], s.list[n-Max:]...)
	}
	s.ver++
	return l.ID
}

// EditLast changes the newest letter (no-op on an empty stack).
func (s *State) EditLast(f func(*Letter)) {
	if n := len(s.list); n > 0 {
		f(&s.list[n-1])
		s.ver++
	}
}

// EditAll changes every letter in place.
func (s *State) EditAll(f func(*Letter)) {
	for i := range s.list {
		f(&s.list[i])
	}
	s.ver++
}

// Remove takes letter id off the stack; ok false for an unknown id.
func (s *State) Remove(id uint64) (Letter, bool) {
	i := slices.IndexFunc(s.list, func(l Letter) bool { return l.ID == id })
	if i < 0 {
		return Letter{}, false
	}
	l := s.list[i]
	s.list = slices.Delete(s.list, i, i+1)
	s.ver++
	return l, true
}

// Get is letter id; ok false for an unknown id.
func (s *State) Get(id uint64) (Letter, bool) {
	i := slices.IndexFunc(s.list, func(l Letter) bool { return l.ID == id })
	if i < 0 {
		return Letter{}, false
	}
	return s.list[i], true
}

// List is the stack, oldest first. The caller must not change it.
func (s *State) List() []Letter { return s.list }

// Version moves on every change.
func (s *State) Version() uint64 { return s.ver }

// Export is the stack and its next id, for a save.
func (s *State) Export() ([]Letter, uint64) { return s.list, s.next }

// Import restores the stack from a save.
func (s *State) Import(list []Letter, next uint64) {
	s.list, s.next = slices.Clone(list), next
	s.ver++
}
