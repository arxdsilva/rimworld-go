package social

import "rimworld-go/internal/domain/pawn/trait"

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

// Relation is the opinion relation kind of p (false for PairNone).
func (p PairKind) Relation() (Kind, bool) {
	switch p {
	case PairLover:
		return Lover, true
	case PairFiance:
		return Fiance, true
	case PairSpouse:
		return Spouse, true
	case PairExLover:
		return ExLover, true
	case PairExSpouse:
		return ExSpouse, true
	}
	return 0, false
}

// Active reports a current partnership (lover, fiancé, spouse).
func (p PairKind) Active() bool { return p == PairLover || p == PairFiance || p == PairSpouse }

// validNext is the transition table: from -> allowed to.
var validNext = map[PairKind][]PairKind{
	PairNone:     {PairLover},
	PairLover:    {PairFiance, PairExLover},
	PairFiance:   {PairSpouse, PairExLover},
	PairSpouse:   {PairExSpouse},
	PairExLover:  {PairLover},
	PairExSpouse: {PairLover},
}

// CanTransition reports whether a pair may go from -> to.
func CanTransition(from, to PairKind) bool {
	for _, t := range validNext[from] {
		if t == to {
			return true
		}
	}
	return false
}

// Pair is a couple's romance state (saved).
type Pair struct {
	Kind                                             PairKind
	Since, LastChange, NextRomanceAt, NextProposalAt uint64
}

// Transition moves the pair to kind to at now when the table allows it.
func (p *Pair) Transition(to PairKind, now uint64) bool {
	if !CanTransition(p.Kind, to) {
		return false
	}
	p.Kind = to
	p.LastChange = now
	if p.Kind.Active() {
		p.Since = now
	}
	return true
}

// CanBreakUp is false within BreakupGrace of the last change.
func (p Pair) CanBreakUp(now uint64) bool {
	return p.Kind.Active() && now >= p.LastChange+BreakupGrace
}

// CanPropose: only lovers, and not during the proposal cooldown.
func (p Pair) CanPropose(now uint64) bool {
	return p.Kind == PairLover && now >= p.NextProposalAt
}

// RejectProposal starts the proposal cooldown.
func (p *Pair) RejectProposal(now uint64) { p.NextProposalAt = now + ProposalCooldown }

// CanRomance is false while a rebuff cooldown runs.
func (p Pair) CanRomance(now uint64) bool { return now >= p.NextRomanceAt }

// Rebuff starts the romance cooldown after a failed attempt.
func (p *Pair) Rebuff(now uint64) { p.NextRomanceAt = now + RebuffCooldown }

func orientationAllows(a, b Person) bool {
	switch {
	case a.Has(trait.Asexual):
		return false
	case a.Has(trait.Bisexual):
		return true
	case a.Has(trait.Gay):
		return a.Male == b.Male
	}
	return a.Male != b.Male
}

// Attraction is a's attraction to b, permille; 0 means ineligible.
func Attraction(a, b Person, kin bool, opinion int32) int32 {
	if kin || a.Dead || b.Dead || a.ID == b.ID {
		return 0
	}
	if a.AgeYears < MinRomanceAge || b.AgeYears < MinRomanceAge || !orientationAllows(a, b) {
		return 0
	}
	gap := a.AgeYears - b.AgeYears
	if gap < 0 {
		gap = -gap
	}
	age := int64(RomanceAgeGapCurve.Eval(int32(gap)))
	beauty := int64(1000)
	if !a.Blind {
		beauty += 150 * int64(b.Beauty)
		if b.Disfigured {
			beauty -= 150
		}
		if beauty < 100 {
			beauty = 100
		}
	}
	op := int64(RomanceOpinionCurve.Eval(opinion))
	return int32(age * beauty / 1000 * op / 1000)
}

// RomanceSuccess is the permille chance recipient accepts an attempt,
// from the recipient's attraction to the initiator.
func RomanceSuccess(attraction int32, partnered bool) int32 {
	if attraction <= 0 {
		return 0
	}
	c := int64(attraction)
	if partnered {
		c = c * PartneredRomanceCut / 1000
	}
	if c > RomanceSuccessMax {
		c = RomanceSuccessMax
	}
	return int32(c)
}

// BreakupWeight is the extra breakup pressure (permille weight) from the
// partner opinion (centi) and a known affair.
func BreakupWeight(opinion int32, affair bool) int32 {
	w := BreakupOpinionCurve.Eval(opinion)
	if affair {
		w += AffairBreakupBonus
	}
	return w
}

// ProposalAccept is the permille chance a proposal is accepted.
func ProposalAccept(opinion int32) int32 { return ProposalAcceptCurve.Eval(opinion) }
