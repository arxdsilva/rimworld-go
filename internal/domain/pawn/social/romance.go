package social

// PairKind is a couple's standing (append only, saved). The constants are
// prefixed: Lover, Fiance, Spouse... are relation Kinds.
type PairKind uint8

const (
	PairNone PairKind = iota
	PairLover
	PairFiance
	PairSpouse
	PairExLover
	PairExSpouse
)

// Pair is a couple's romance state (saved).
type Pair struct {
	Kind                                             PairKind
	Since, LastChange, NextRomanceAt, NextProposalAt uint64
}

// Attraction is a's attraction to b, permille; 0 means ineligible.
func Attraction(a, b Person, kin bool, opinion int32) int32 {
	return 0 // TODO(M5)
}
