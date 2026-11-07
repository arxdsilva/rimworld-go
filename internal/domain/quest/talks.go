package quest

// Peace talks (QST-077, QST-096). Signatures are fixed.

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
//
// Rolls outside [0, 1000) are clamped; social is clamped to 0..15.
func Negotiate(social, roll, roll2 int) Outcome {
	roll = min(max(roll, 0), 999)
	roll2 = min(max(roll2, 0), 999)
	res := talkResult(min(max(social, 0), 15), roll)
	out := Outcome{Result: res}
	switch res {
	case TalkTriumph:
		out.Goodwill = spread(100, 110, roll2)
		out.Gift = spread(500, 1200, roll2)
	case TalkSuccess:
		out.Goodwill = spread(60, 70, roll2)
	case TalkBackfire:
		out.Goodwill = -spread(10, 20, roll2)
	case TalkDisaster:
		out.Goodwill = -spread(40, 50, roll2)
		out.Ambush = true
	}
	return out
}

// talkOdds are the published odds in permille at Social 0, 3, 6, 9, 12
// and 15, best result first (QST-096). The Social 12 row is published as
// 99.9 %; its missing 0.1 % goes to Flounder (152 → 153).
func talkOdds() [6][5]int {
	return [6][5]int{
		{42, 230, 235, 329, 164},
		{57, 316, 242, 257, 128},
		{80, 438, 231, 167, 84},
		{105, 575, 190, 87, 43},
		{119, 655, 153, 49, 24},
		{133, 729, 106, 21, 11},
	}
}

// talkResult picks the result for social in 0..15 and roll in [0, 1000):
// the odds are linear between anchors, kept in thirds of a permille so
// the interpolation is exact.
func talkResult(social, roll int) TalkResult {
	odds := talkOdds()
	i, f := social/3, social%3
	if i == len(odds)-1 {
		i, f = i-1, 3
	}
	cum, r := 0, roll*3
	for k := range odds[i] {
		cum += odds[i][k]*(3-f) + odds[i+1][k]*f
		if r < cum {
			return TalkResult(k)
		}
	}
	return TalkFlounder
}

// spread maps roll in [0, 1000) onto lo..hi inclusive.
func spread(lo, hi, roll int) int {
	return lo + roll*(hi-lo+1)/1000
}
