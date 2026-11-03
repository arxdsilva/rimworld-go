package aqua

import rules "rimworld-go/internal/domain/building/aqua"

// diedText is the message for stock s lost to cause.
func diedText(s rules.Species, cause rules.Cause) string {
	head := "The " + s.String() + " in a fish tank "
	switch cause {
	case rules.NoPower:
		return head + "died without power"
	case rules.TooCold:
		return head + "died of cold"
	case rules.TooHot:
		return head + "died of heat"
	}
	return head + "starved"
}

// dyingText is the warning that stock s is failing, once per fall (the
// domain's WarnBelow and WarnClear).
func dyingText(s rules.Species, cause rules.Cause) string {
	head := "The " + s.String() + " in a fish tank are dying: "
	switch cause {
	case rules.NoPower:
		return head + "no power"
	case rules.TooCold:
		return head + "too cold"
	case rules.TooHot:
		return head + "too hot"
	}
	return head + "no feed"
}
