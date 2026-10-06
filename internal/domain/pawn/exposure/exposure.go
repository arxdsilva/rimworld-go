// Package exposure is how air temperature affects a colonist in everyday
// clothes: a comfortable band, mood thoughts outside it, and hypothermia or
// heatstroke that build up in dangerous cold or heat and fade when the
// colonist is somewhere bearable again. Temperatures are tenths of a
// degree C; severities are permille.
package exposure

// Comfort and danger bands for a colonist in ordinary clothes.
const (
	ComfortMin = 80  // below 8 C: cold
	ComfortMax = 280 // above 28 C: hot
	ColdDanger = 0   // at or below 0 C hypothermia builds
	HeatDanger = 380 // above 38 C heatstroke builds
	// Freezing/Sweltering are the harsher mood thresholds.
	Freezing   = -50
	Sweltering = 340
)

// Severity thresholds, permille.
const (
	Mild    = 200  // noticeable; the colonist goes looking for warmth/shade
	Serious = 600  // collapses (downed)
	Fatal   = 1000 // dies
	// Recover is the severity a downed colonist stands up again below.
	Recover = 400
)

// Step is the change in hypothermia and heatstroke over interval ticks at
// air temperature t. Cold builds faster the colder it is: at -10 C a
// colonist collapses in about three hours and dies in about five; above
// freezing it fades, quickly once comfortable.
func Step(hypo, heat, t, interval int) (int, int) {
	// rates are per 250 ticks, scaled to the interval
	per := func(v int) int { return v * interval / 250 }
	switch {
	case t <= ColdDanger:
		hypo += per(5 + (ColdDanger-t)/5)
	case t >= ComfortMin:
		hypo -= per(30)
	default:
		hypo -= per(10)
	}
	switch {
	case t >= HeatDanger:
		heat += per(5 + (t-HeatDanger)/5)
	case t <= ComfortMax:
		heat -= per(30)
	default:
		heat -= per(10)
	}
	return clamp(hypo), clamp(heat)
}

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > Fatal {
		return Fatal
	}
	return v
}
