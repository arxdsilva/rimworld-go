package quest

import (
	"slices"

	qd "rimworld-go/internal/domain/quest"
)

// Op is a player command on quests (QST-016, QST-150).
type Op uint8

const (
	OpNone Op = iota
	// OpAccept accepts offer Quest with reward option Choice (-1: none).
	OpAccept
	// OpDismiss and OpRestore hide and show Quest (QST-006).
	OpDismiss
	OpRestore
	// OpFulfil hands Quest's requested goods over from Caravan (2g);
	// OpNegotiate opens Quest's talks with Caravan (2h). Both reach the
	// quest's kind as SigFulfil / SigNegotiate.
	OpFulfil
	OpNegotiate
	// OpSetPref sets whether goodwill is accepted as a reward from Faction
	// (On; QST-083).
	OpSetPref
	// OpOffer offers a quest of Kind now, if it can be (developer mode).
	OpOffer
)

// Cmd is one player command (built with Accept, Dismiss, ...).
type Cmd struct {
	Op      Op
	Quest   qd.ID
	Choice  int
	Caravan uint64
	// Faction is 1 + index, as qd.Quest.Giver.
	Faction int
	On      bool
	Kind    qd.KindID
}

// Accept accepts offer id with reward option choice.
func Accept(id qd.ID, choice int) Cmd { return Cmd{Op: OpAccept, Quest: id, Choice: choice} }

// Dismiss hides quest id under Historical; Restore shows it again.
func Dismiss(id qd.ID) Cmd { return Cmd{Op: OpDismiss, Quest: id} }
func Restore(id qd.ID) Cmd { return Cmd{Op: OpRestore, Quest: id} }

// Fulfil hands quest id's goods over from caravan.
func Fulfil(id qd.ID, caravan uint64) Cmd { return Cmd{Op: OpFulfil, Quest: id, Caravan: caravan} }

// Negotiate opens quest id's talks with caravan.
func Negotiate(id qd.ID, caravan uint64) Cmd {
	return Cmd{Op: OpNegotiate, Quest: id, Caravan: caravan}
}

// SetPref sets whether goodwill from faction ref (1 + index) is accepted.
func SetPref(ref int, acceptGoodwill bool) Cmd {
	return Cmd{Op: OpSetPref, Faction: ref, On: acceptGoodwill}
}

// OfferKind offers a quest of kind k now (the developer's offer).
func OfferKind(k qd.KindID) Cmd { return Cmd{Op: OpOffer, Kind: k} }

// Apply runs player command c at the clock's tick. Errors: ErrNoQuest,
// ErrNoKind, ErrOp, ErrCannotOffer, or the domain's (qd.ErrMove, qd.ErrChoice).
func (q *Quests) Apply(c Cmd) error {
	if c.Op == OpSetPref {
		q.setPref(c.Faction-1, c.On)
		return nil
	}
	if c.Op == OpOffer {
		if _, ok := q.Offer(c.Kind); !ok {
			return ErrCannotOffer
		}
		return nil
	}
	if c.Op == OpNone || c.Op > OpOffer {
		return ErrOp
	}
	i, ok := q.find(c.Quest)
	if !ok {
		return ErrNoQuest
	}
	now := q.r.Clock.Now()
	var (
		n   qd.Quest
		err error
	)
	switch c.Op {
	case OpAccept:
		return q.accept(i, now, c.Choice)
	case OpDismiss:
		n, err = qd.Dismiss(q.list[i])
	case OpRestore:
		n, err = qd.Restore(q.list[i])
	default: // OpFulfil, OpNegotiate
		return q.handOver(i, c, now)
	}
	if err != nil {
		return err
	}
	q.list[i] = n
	q.touched(i)
	return nil
}

// handOver sends a Fulfil or Negotiate command to active quest i's kind.
func (q *Quests) handOver(i int, c Cmd, now uint64) error {
	if q.list[i].State != qd.Active {
		return qd.ErrMove
	}
	if q.kindOf(q.list[i].Kind) == nil {
		return ErrNoKind
	}
	s := Signal{Kind: SigFulfil, Quest: c.Quest, Caravan: c.Caravan}
	if c.Op == OpNegotiate {
		s.Kind = SigNegotiate
	}
	q.signalOne(c.Quest, s, now)
	return nil
}

// setPref turns goodwill rewards from faction index f on or off.
func (q *Quests) setPref(f int, on bool) {
	if f < 0 {
		return
	}
	i, found := slices.BinarySearch(q.noGoodwill, f)
	switch {
	case on && found:
		q.noGoodwill = slices.Delete(q.noGoodwill, i, i+1)
	case !on && !found:
		q.noGoodwill = slices.Insert(q.noGoodwill, i, f)
	default:
		return
	}
	q.ver++
}
