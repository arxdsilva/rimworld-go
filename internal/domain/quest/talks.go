package quest

// Peace talks (QST-077, QST-096). Signatures are fixed; D-dom writes the
// bodies and tests.

// TalkResult is a talk's outcome, best first. Saved: append-only.
type TalkResult uint8

const (
	TalkTriumph TalkResult = iota
	TalkSuccess
	TalkFlounder
	TalkBackfire
	TalkDisaster
)

// Outcome is what a talk gives.
type Outcome struct {
	Result TalkResult
	// Goodwill is the change with the host faction.
	Goodwill int
	// Gift is the gift's market value (Triumph only; 0 otherwise).
	Gift int
	// Ambush: the caravan is attacked at once (Disaster).
	Ambush bool
}

// TalkXP is the Social experience the negotiator gains, whatever happens.
const TalkXP = 6000

// Negotiate is a talk's outcome for a negotiator with Social level social
// (0..20). The odds follow the 6 published anchors (Social 0, 3, 6, 9, 12,
// 15+), linear in between; roll in [0, 1000) picks the result, and goodwill
// and gift are spread over their ranges by roll2 in [0, 1000).
func Negotiate(social, roll, roll2 int) Outcome { return Outcome{} }
