package drug

// LineKind classifies a UI line.
type LineKind uint8

const (
	LineAlcohol LineKind = iota
	LineHigh
	LineAddiction
	LineCraving
	LineWithdrawal
	// LineProtected (MED-07): a prophylactic still acting.
	LineProtected
)

// Line is one read-only row for the UI.
type Line struct {
	Kind    LineKind
	Label   string
	Percent int32 // 0..100
}

// Lines lists what the drugs are doing to the colonist, for the UI.
func (s *State) Lines() []Line {
	var out []Line
	if b := s.Band(); b != Sober {
		out = append(out, Line{LineAlcohol, b.Label(), min(s.Alcohol/10, 100)})
	}
	for _, h := range s.Highs {
		d, _ := DefOf(h.Drug)
		if d.Protects {
			out = append(out, Line{LineProtected, "protected by " + d.Short, h.Severity / 10})
			continue
		}
		out = append(out, Line{LineHigh, "high on " + d.Short, h.Severity / 10})
	}
	for _, a := range s.Addictions {
		d, _ := DefOf(a.Drug)
		switch {
		case a.Withdrawal:
			out = append(out, Line{LineWithdrawal, "withdrawal from " + d.Short, 100})
		case a.Seeking:
			out = append(out, Line{LineCraving, "craving " + d.Short, a.Need / 10})
		default:
			out = append(out, Line{LineAddiction, "hooked on " + d.Short, a.Need / 10})
		}
	}
	return out
}
