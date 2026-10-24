package social

import "rimworld-go/internal/domain/pawn/trait"

// Person is what opinion and interaction rules read about someone.
type Person struct {
	ID              ID
	Male            bool
	AgeYears        int
	Traits          []trait.ID
	Beauty          int8
	Disfigured      bool
	Blind, Deaf     bool
	ArtificialParts int
	Addicted        bool
	Dead            bool
}

// Term is one line of an opinion breakdown, centi-points.
type Term struct {
	Label string
	Value int32
}

// Inputs is everything Observer's opinion of Target depends on.
type Inputs struct {
	Observer, Target Person
	Relations        []Kind
	Memories         []OpinionMemory
	Now              uint64
}

// Opinion is Observer's opinion of Target in centi-points (clamped to
// +-100 points) and the terms that make it up.
func Opinion(in Inputs) (centi int32, terms []Term) {
	return 0, nil // TODO(M3)
}
