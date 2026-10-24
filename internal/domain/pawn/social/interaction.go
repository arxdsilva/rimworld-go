package social

import (
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/trait"
)

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

var interactionNames = [NumInteractions]string{
	"chitchat", "deep talk", "slight", "insult", "kind words", "romance attempt", "breakup", "proposal",
}

// String is the interaction's name.
func (i Interaction) String() string {
	if i >= NumInteractions {
		return ""
	}
	return interactionNames[i]
}

// InteractCtx is what choosing an interaction depends on.
type InteractCtx struct {
	Initiator, Recipient Person
	OpinionIR, OpinionRI int32 // centi
	Compat               int32 // permille
	Pair                 PairKind
	Partnered            bool
	Attraction           int32 // permille
}

// Weights is each interaction's selection weight for c.
func Weights(c InteractCtx) [NumInteractions]int64 {
	var w [NumInteractions]int64
	ini := c.Initiator
	w[Chitchat] = WeightChitchat
	w[DeepTalk] = int64(WeightDeepTalk) * int64(DeepTalkCompatCurve.Eval(c.Compat)) / 1000
	if !ini.Has(trait.Gracious) {
		neg := int64(NegativeCompatCurve.Eval(c.Compat)) * int64(OpinionNegCurve.Eval(c.OpinionIR)) / 1000
		w[Slight] = WeightSlight * neg / 1000
		w[Insult] = WeightInsult * neg / 1000
	} else if !ini.Has(trait.ColdHearted) {
		w[KindWords] = WeightKindWords
	}
	switch c.Pair {
	case PairNone, PairExLover, PairExSpouse:
		if c.Attraction > 0 {
			w[RomanceAttempt] = int64(WeightRomance) * int64(c.Attraction) / 1000
			if c.Partnered {
				w[RomanceAttempt] = w[RomanceAttempt] * AffairFactor / 1000
			}
		}
	case PairLover, PairFiance, PairSpouse:
		w[Breakup] = int64(WeightBreakup) * int64(BreakupOpinionCurve.Eval(c.OpinionIR)) / 1000
		if c.Pair == PairLover {
			w[Proposal] = int64(WeightProposal) * int64(ProposalOpinionCurve.Eval(c.OpinionIR)) / 1000
		}
	}
	return w
}

// Choose picks the initiator's interaction with roll (roll(n) is in [0,n)).
func Choose(roll func(int) int, c InteractCtx) Interaction {
	w := Weights(c)
	var total int64
	for _, x := range w {
		total += x
	}
	if total <= 0 {
		return Chitchat
	}
	r := int64(roll(int(total)))
	for i, x := range w {
		if r < x {
			return Interaction(i)
		}
		r -= x
	}
	return Chitchat
}

// Effect is what an interaction leaves on its recipient.
type Effect struct {
	Memory    MemKind
	HasMemory bool
	Mood      mood.Kind
	HasMood   bool
}

// Effects is the opinion memory and mood the recipient gets from i. The
// romance outcomes are resolved by the romance rules, so they have none here.
func Effects(i Interaction) Effect {
	switch i {
	case Chitchat:
		return Effect{Memory: MemChitchat, HasMemory: true}
	case DeepTalk:
		return Effect{Memory: MemDeepTalk, HasMemory: true}
	case Slight:
		return Effect{Memory: MemSlighted, HasMemory: true}
	case Insult:
		return Effect{Memory: MemInsulted, HasMemory: true, Mood: mood.Insulted, HasMood: true}
	case KindWords:
		return Effect{Memory: MemKindWords, HasMemory: true, Mood: mood.KindWords, HasMood: true}
	}
	return Effect{}
}

// SocialImpact is the Social impact stat in permille: 820 + 27.5 per
// Social skill level (floor 200), scaled by the Talking capacity (permille).
func SocialImpact(socialSkill int, talking int32) int32 {
	base := int32((2*ImpactBase + 55*socialSkill) / 2)
	if base < ImpactFloor {
		base = ImpactFloor
	}
	return int32(int64(base) * int64(talking) / 1000)
}
