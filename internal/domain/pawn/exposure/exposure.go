// Package exposure is how air temperature affects a colonist: a
// comfortable band (set by what it wears), mood thoughts outside it, and
// hypothermia or heatstroke that build up in dangerous cold or heat and
// fade when the colonist is somewhere bearable again. Temperatures are
// tenths of a degree C; severities are permille.
package exposure

// Bands is a colonist's comfortable range, tenths of a degree C.
type Bands struct{ Min, Max int }

// Everyday is the range in plain everyday clothes.
var Everyday = Bands{Min: 80, Max: 280}

// Wearing widens the band by apparel insulation: cold lowers Min, heat
// raises Max (negative heat insulation lowers it).
func (b Bands) Wearing(cold, heat int) Bands {
	return Bands{Min: b.Min - cold, Max: b.Max + heat}
}

// Offsets from the comfortable band, tenths of a degree.
const (
	coldDangerBelow = 80  // hypothermia builds 8 C under the comfort floor
	heatDangerAbove = 100 // heatstroke builds 10 C over the ceiling
	freezingBelow   = 130 // "freezing" thought
	swelteringAbove = 60  // "sweltering" thought
)

// ColdDanger/HeatDanger are where exposure starts to build.
func (b Bands) ColdDanger() int { return b.Min - coldDangerBelow }
func (b Bands) HeatDanger() int { return b.Max + heatDangerAbove }

// Freezing/Sweltering are the harsher mood thresholds.
func (b Bands) Freezing() int   { return b.Min - freezingBelow }
func (b Bands) Sweltering() int { return b.Max + swelteringAbove }

// Comfortable reports t inside the band.
func (b Bands) Comfortable(t int) bool { return t >= b.Min && t <= b.Max }

// Everyday-clothes thresholds, kept for callers without a colonist.
const (
	ComfortMin = 80
	ComfortMax = 280
	ColdDanger = 0
	HeatDanger = 380
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
// air temperature t for a colonist with comfort band b. Cold builds faster
// the colder it is: 10 C below the danger line a colonist collapses in
// about three hours and dies in about five; above it the exposure fades,
// quickly once comfortable.
func Step(hypo, heat, t, interval int, b Bands) (int, int) {
	per := func(v int) int { return v * interval / 250 }
	switch cd := b.ColdDanger(); {
	case t <= cd:
		hypo += per(5 + (cd-t)/5)
	case t >= b.Min:
		hypo -= per(30)
	default:
		hypo -= per(10)
	}
	switch hd := b.HeatDanger(); {
	case t >= hd:
		heat += per(5 + (t-hd)/5)
	case t <= b.Max:
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
