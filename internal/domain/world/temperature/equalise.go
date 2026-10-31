package temperature

// Room equalisation (TEMP-1, ENG-003): how a room's air moves toward the
// outside through its walls and roof, per the wiki's measurements. Pure
// integer math on Milli so a 120-tick step can move a large room by less
// than a tenth of a degree and still be exact.

// Milli is air temperature in thousandths of a degree C: the room model's
// working precision (Temp is what the rest of the game reads).
type Milli int

// Milli is t in thousandths of a degree.
func (t Temp) Milli() Milli { return Milli(t) * 100 }

// Temp is m rounded to tenths of a degree (half away from zero).
func (m Milli) Temp() Temp {
	if m < 0 {
		return Temp((m - 50) / 100)
	}
	return Temp((m + 50) / 100)
}

// EqualiseInterval is how often (ticks) rooms equalise with the outside.
// wiki: Temperature (every 120 ticks).
const EqualiseInterval = 120

// Exposure is what of a room meets the outside air, counted when the room
// is built and cached until a wall, roof or door changes.
type Exposure struct {
	// Cells is the room's floor cells.
	Cells int
	// Single counts wall faces with outdoor air right behind the wall,
	// Double faces with a second wall and then outdoor air (half the loss;
	// a third layer adds nothing). Rock behind rock counts as neither.
	Single, Double int
	// Thin counts cells under a constructed or thin-rock roof, Thick under
	// overhead mountain, Open under the sky.
	Thin, Thick, Open int
}

// The equalisation rates, per 120 ticks, in parts per 10^7 of the
// difference from the outside air.
const (
	// wallRate: one exposed wall face per room cell. wiki: Temperature,
	// the loss table at a -9555 C difference (1x1 -161.47, 2x2 -81.24,
	// 3x3 -54.16, 5x5 -32.49, 7x7 -23.21): 40.62 per unit of
	// perimeter / area, 40.62 / 9555 = 0.0042512.
	wallRate = 42512
	// thinRate: a thin roof over every cell. wiki: Temperature (-57.331
	// at -9555 whatever the room size: 0.0060001).
	thinRate = 60001
	// thickRate: overhead mountain exchanges nothing with the outside; it
	// only cools a room warmer than thickFloor, "extremely" slowly
	// (wiki: Temperature). original tuning: a twentieth of a thin roof.
	thickRate  = 3000
	thickFloor = Milli(15000)
	// openRate: cells under the sky in a room below OutdoorShare.
	// original tuning: four times a thin roof.
	openRate = 240000
	// OutdoorShare (permille): a room this unroofed or more simply takes
	// the outdoor temperature. wiki: Rooms (25 %).
	OutdoorShare = 250
	ratePer      = 10_000_000
)

// Outdoors reports a room unroofed enough to take the outdoor air.
func (e Exposure) Outdoors() bool { return e.Cells > 0 && e.Open*1000 >= e.Cells*OutdoorShare }

// Equalise is room air t after one EqualiseInterval with the outside air
// at out.
func (e Exposure) Equalise(t, out Milli) Milli {
	if e.Cells <= 0 {
		return t
	}
	if e.Outdoors() {
		return out
	}
	t += Milli(int64(out-t) * e.conductance() / (2 * int64(e.Cells) * ratePer))
	if e.Thick > 0 && t > thickFloor {
		t -= Milli(int64(t-thickFloor) * int64(thickRate) * int64(e.Thick) / (int64(e.Cells) * ratePer))
	}
	return t
}

// conductance is the room's exchange with the outside, twice over (a
// double wall face counts half): parts per RatePer of a degree-cell per
// degree of difference per interval, times 2.
func (e Exposure) conductance() int64 {
	return int64(wallRate)*int64(2*e.Single+e.Double) + 2*(int64(thinRate)*int64(e.Thin)+int64(openRate)*int64(e.Open))
}

// Rise is how far above the outside air (thousandths of a degree) a
// steady push of q degree-milli-cells per interval holds the room, once
// the walls and roof lose as much as it gains (0: no exchange at all).
func (e Exposure) Rise(q int64) Milli {
	c := e.conductance()
	if c == 0 {
		return 0
	}
	return Milli(q * 2 * ratePer / c)
}

// Loss is how far (thousandths of a degree) one interval moves the room
// at a diff difference from the outside, with no roof term: the wiki's
// wall table reads it at -9555 C.
func (e Exposure) Loss(diff Milli) Milli {
	w := Exposure{Cells: e.Cells, Single: e.Single, Double: e.Double}
	return w.Equalise(0, diff)
}
