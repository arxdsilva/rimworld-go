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

// The contract below is QST-1 step 1. Signatures are fixed; D-dom writes
// the bodies and tests. Every function is pure.

// Offer starts quest q at tick now under def d: an AutoAccept kind is
// Active at once (Deadline from DurationTicks), any other is Offered until
// now + AcceptTicks. Chosen starts at -1.
func Offer(q Quest, now uint64, d Def) Quest { return q }

// Accept accepts offer q at now with reward option choice (QST-018: the
// choice is made at accept). A dismissed offer can be accepted. Errors on a
// non-Offered quest or a choice out of range (no options: choice -1).
func Accept(q Quest, now uint64, choice int, d Def) (Quest, error) { return q, nil }

// Expire ends offer q (its accept window ran out): Expired, Timeout.
func Expire(q Quest, now uint64) (Quest, error) { return q, nil }

// Succeed ends active q as a success.
func Succeed(q Quest, now uint64) (Quest, error) { return q, nil }

// Fail ends q as a failure for reason r. An Offered quest that fails (its
// giver turned hostile) is Expired with r instead.
func Fail(q Quest, now uint64, r Reason) (Quest, error) { return q, nil }

// Dismiss hides q under Historical; Restore shows it again. Neither
// changes State or a timer.
func Dismiss(q Quest) (Quest, error) { return q, nil }
func Restore(q Quest) (Quest, error) { return q, nil }

// Due is q's next timer: the earliest of AcceptBy (Offered), Deadline
// (Active) and an unfinished part's At (Active). ok false: none.
func Due(q Quest) (tick uint64, ev Event, part int, ok bool) { return 0, EvNone, -1, false }

// FailOnRelation is the failure a relation change causes (QST-010/011):
// GiverHostile when d.FailOnGiverHostile and the giver is Hostile;
// TargetNotHostile when d.HasTarget and the target is not Hostile.
func FailOnRelation(q Quest, d Def, giver, target faction.Relation) (Reason, bool) {
	return ReasonNone, false
}
