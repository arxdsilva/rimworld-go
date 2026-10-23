// Package infection is wound infection: the chance an injury festers, and
// how an infection grows or recedes hour by hour.
//
// A fresh wound left untended for Delay hours may become infected; tending
// it closes the window, with a smaller chance the better the care. An
// infection's severity climbs while untended and falls while it is being
// treated; it kills at Fatal and is beaten at zero.
package infection

// Care is how a wound or infection was last treated.
type Care uint8

const (
	Untreated Care = iota
	SelfTended
	Tended   // by a doctor, without medicine
	Medicine // by a doctor with herbal medicine
	// IndustrialMedicine: by a doctor with industrial medicine (NG-1 §4.8).
	IndustrialMedicine
)

const (
	// DelayHours an untended wound stays open before it may fester.
	DelayHours = 8
	// Onset is the severity a new infection starts at.
	Onset = 15
	// Fatal severity kills.
	Fatal = 100
	// Sick: at or above this severity the patient feels it (mood) and
	// should rest in bed.
	Sick = 20
)

// Chance is the percent chance a wound becomes infected given its care.
func Chance(c Care) int {
	switch c {
	case IndustrialMedicine:
		return 1
	case Medicine:
		return 3
	case Tended:
		return 12
	case SelfTended:
		return 20
	}
	return 40
}

// Step is the severity after one hour: falling while treated (more with
// medicine), climbing otherwise. Results are clamped to [0, Fatal].
func Step(severity int, treated bool, c Care) int {
	switch {
	case treated && c == IndustrialMedicine:
		severity -= 12
	case treated && c == Medicine:
		severity -= 9
	case treated:
		severity -= 5
	default:
		severity += 4
	}
	return max(0, min(Fatal, severity))
}

// HealPct is how much of a plain tend's healing a tend with care c gives,
// in percent: medicine doubles it, industrial medicine gives 2.5×, a
// self-tend half.
func HealPct(c Care) int {
	switch c {
	case IndustrialMedicine:
		return 250
	case Medicine:
		return 200
	case SelfTended:
		return 50
	}
	return 100
}
