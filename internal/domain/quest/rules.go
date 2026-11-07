package quest

import "rimworld-go/internal/domain/faction"

// Error is a quest error (a constant, so the package holds no state).
type Error string

func (e Error) Error() string { return string(e) }

// ErrMove is returned when a transition is not allowed from q's state;
// ErrChoice when a reward choice is out of range.
const (
	ErrMove   Error = "quest: move not allowed in this state"
	ErrChoice Error = "quest: no such reward option"
)

// The contract below is QST-1 step 1. Signatures are fixed. Every
// function is pure: it takes a Quest by value and returns the new one.

// Offer starts quest q at tick now under def d: an AutoAccept kind is
// Active at once (Deadline from DurationTicks), any other is Offered until
// now + AcceptTicks. Chosen starts at -1.
func Offer(q Quest, now uint64, d Def) Quest {
	if q.Kind == KindNone {
		q.Kind = d.Kind
	}
	q.Reason, q.Chosen, q.Paid = ReasonNone, -1, false
	q.OfferedAt, q.AcceptedAt, q.EndedAt = now, 0, 0
	q.AcceptBy, q.Deadline = 0, 0
	if d.AutoAccept {
		q.State, q.AcceptedAt = Active, now
		q.Deadline = after(now, d.DurationTicks)
		return q
	}
	q.State = Offered
	q.AcceptBy = after(now, d.AcceptTicks)
	return q
}

// after is now + ticks, or 0 (no timer) when ticks is 0.
func after(now, ticks uint64) uint64 {
	if ticks == 0 {
		return 0
	}
	return now + ticks
}

// Accept accepts offer q at now with reward option choice (QST-018: the
// choice is made at accept). A dismissed offer can be accepted. Errors on a
// non-Offered quest or a choice out of range (no options: choice -1).
func Accept(q Quest, now uint64, choice int, d Def) (Quest, error) {
	if q.State != Offered {
		return q, ErrMove
	}
	if len(q.Rewards) == 0 {
		if choice != -1 {
			return q, ErrChoice
		}
	} else if choice < 0 || choice >= len(q.Rewards) {
		return q, ErrChoice
	}
	q.State, q.AcceptedAt, q.Chosen = Active, now, int8(choice)
	q.Deadline = after(now, d.DurationTicks)
	return q, nil
}

// Expire ends offer q (its accept window ran out): Expired, Timeout.
func Expire(q Quest, now uint64) (Quest, error) {
	if q.State != Offered {
		return q, ErrMove
	}
	return end(q, now, Expired, Timeout), nil
}

// Succeed ends active q as a success.
func Succeed(q Quest, now uint64) (Quest, error) {
	if q.State != Active {
		return q, ErrMove
	}
	return end(q, now, Succeeded, ReasonNone), nil
}

// Fail ends q as a failure for reason r. An Offered quest that fails (its
// giver turned hostile) is Expired with r instead. A paid reward stays
// paid (QST-008).
func Fail(q Quest, now uint64, r Reason) (Quest, error) {
	switch q.State {
	case Offered:
		return end(q, now, Expired, r), nil
	case Active:
		return end(q, now, Failed, r), nil
	}
	return q, ErrMove
}

// end moves q to terminal state s for reason r at now.
func end(q Quest, now uint64, s State, r Reason) Quest {
	q.State, q.Reason, q.EndedAt = s, r, now
	return q
}

// Dismiss hides q under Historical; Restore shows it again. Neither
// changes State or a timer. Any state may be dismissed; dismissing a
// dismissed quest, or restoring one that is not, is ErrMove.
func Dismiss(q Quest) (Quest, error) {
	if q.Dismissed {
		return q, ErrMove
	}
	q.Dismissed = true
	return q, nil
}

func Restore(q Quest) (Quest, error) {
	if !q.Dismissed {
		return q, ErrMove
	}
	q.Dismissed = false
	return q, nil
}

// Due is q's next timer: the earliest of AcceptBy (Offered), Deadline
// (Active) and an unfinished part's At (Active). ok false: none. On a tie
// the deadline comes first, then the lowest part index.
func Due(q Quest) (tick uint64, ev Event, part int, ok bool) {
	switch q.State {
	case Offered:
		if q.AcceptBy == 0 {
			return 0, EvNone, -1, false
		}
		return q.AcceptBy, EvAcceptEnd, -1, true
	case Active:
	default:
		return 0, EvNone, -1, false
	}
	tick, ev, part = 0, EvNone, -1
	if q.Deadline != 0 {
		tick, ev, ok = q.Deadline, EvDeadline, true
	}
	for i, p := range q.Parts {
		if p.Done || p.At == 0 || (ok && p.At >= tick) {
			continue
		}
		tick, ev, part, ok = p.At, EvPart, i, true
	}
	return tick, ev, part, ok
}

// FailOnRelation is the failure a relation change causes (QST-010/011):
// GiverHostile when d.FailOnGiverHostile and the giver is Hostile;
// TargetNotHostile when d.HasTarget and the target is not Hostile. The
// giver is checked first. A terminal quest, or a quest with no giver
// (no target) for that check, never fails here.
func FailOnRelation(q Quest, d Def, giver, target faction.Relation) (Reason, bool) {
	if q.State.Terminal() {
		return ReasonNone, false
	}
	if d.FailOnGiverHostile && q.Giver != 0 && giver == faction.Hostile {
		return GiverHostile, true
	}
	if d.HasTarget && q.Target != 0 && target != faction.Hostile {
		return TargetNotHostile, true
	}
	return ReasonNone, false
}
