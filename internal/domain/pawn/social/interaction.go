package social

// Interaction is a social interaction (append only, saved).
type Interaction uint8

const (
	Chitchat Interaction = iota
	DeepTalk
	Slight
	Insult
	KindWords
	RomanceAttempt
	Breakup
	Proposal
	NumInteractions
)

// InteractCtx is what choosing an interaction depends on.
type InteractCtx struct {
	Initiator, Recipient Person
	OpinionIR, OpinionRI int32 // centi
	Compat               int32 // permille
	Pair                 PairKind
	Partnered            bool
	Attraction           int32 // permille
}

// Choose picks the initiator's interaction with roll.
func Choose(roll func(int) int, c InteractCtx) Interaction {
	return Chitchat // TODO(M3)
}
