package clock

// Period is a named part of the 24-hour day. It is derived from Hour
// and never stored as independent authoritative state.
type Period int

const (
	Night Period = iota
	Morning
	Afternoon
	Evening
)

// PeriodAt maps a 0-based hour (0–23) onto a day period.
func PeriodAt(hour int) Period {
	switch {
	case hour >= 6 && hour < 12:
		return Morning
	case hour >= 12 && hour < 17:
		return Afternoon
	case hour >= 17 && hour < 20:
		return Evening
	default:
		return Night
	}
}

func (p Period) String() string {
	switch p {
	case Morning:
		return "morning"
	case Afternoon:
		return "afternoon"
	case Evening:
		return "evening"
	default:
		return "night"
	}
}
