package social

// Point is one vertex of a piecewise-linear curve.
type Point struct{ X, Y int32 }

// Curve is piecewise linear over ascending X; outside the ends it is flat.
type Curve []Point

// Eval is the curve's value at x, interpolating in integers.
func (c Curve) Eval(x int32) int32 {
	if len(c) == 0 {
		return 0
	}
	if x <= c[0].X {
		return c[0].Y
	}
	for i := 1; i < len(c); i++ {
		if x <= c[i].X {
			a, b := c[i-1], c[i]
			span := int64(b.X - a.X)
			if span <= 0 {
				return b.Y
			}
			return a.Y + int32(int64(b.Y-a.Y)*int64(x-a.X)/span)
		}
	}
	return c[len(c)-1].Y
}

// Curves (x in permille or centi-points as noted, y in permille).
var (
	// DeepTalkCompatCurve: compatibility (permille) to deep-talk multiplier.
	DeepTalkCompatCurve = Curve{{-2000, 0}, {0, 500}, {1000, 1500}, {3000, 3000}}
	// NegativeCompatCurve: compatibility to slight/insult multiplier.
	NegativeCompatCurve = Curve{{-2000, 3000}, {-1000, 2000}, {0, 1000}, {1000, 300}, {3000, 50}}
	// OpinionNegCurve: opinion (centi) to slight/insult multiplier.
	OpinionNegCurve = Curve{{-10000, 6000}, {0, 1000}, {5000, 100}, {10000, 0}}
	// BreakupOpinionCurve: partner opinion (centi) to breakup base multiplier.
	BreakupOpinionCurve = Curve{{-10000, 4000}, {-2500, 1000}, {0, 100}, {2000, 0}}
	// ProposalOpinionCurve: lover opinion (centi) to proposal multiplier.
	ProposalOpinionCurve = Curve{{0, 0}, {2500, 500}, {5000, 1500}, {10000, 3000}}
	// ProposalAcceptCurve: opinion (centi) to acceptance chance.
	ProposalAcceptCurve = Curve{{-10000, 0}, {0, 0}, {2000, 500}, {5000, 900}, {10000, 1000}}
	// ProposalTogetherCurve: days a couple has been together to proposal
	// multiplier (few propose in the first days, more after weeks).
	ProposalTogetherCurve = Curve{{0, 100}, {3, 400}, {10, 1000}, {30, 1500}, {60, 2000}}
	// ProposalAcceptTogetherCurve: days together to acceptance multiplier.
	ProposalAcceptTogetherCurve = Curve{{0, 500}, {5, 800}, {15, 1000}, {40, 1300}}
	// RomanceOpinionCurve: opinion (centi) to attraction factor.
	RomanceOpinionCurve = Curve{{-10000, 0}, {-500, 0}, {2000, 1000}, {10000, 1500}}
	// RomanceAgeGapCurve: years apart to attraction factor.
	RomanceAgeGapCurve = Curve{{0, 1000}, {5, 1000}, {15, 500}, {30, 100}, {40, 0}}
)
