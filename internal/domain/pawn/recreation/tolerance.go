package recreation

// Recreation tolerance and the variety rule (AX-6; wiki: Recreation).
// Every joy kind a colonist enjoys builds a tolerance for that kind, two
// thirds of the recreation it gave, and the tolerance cuts what the kind
// gives next time by the same share. It wears off by a share a day set by
// the colony's wealth. A colonist above BoredAbove of one kind is bored of
// it and won't seek it until it falls below BoredBelow (hysteresis), so it
// spreads its free time across kinds instead of repeating one.
//
// Tolerance is kept in milli-points of Full (0..Full*1000) so the small
// per-step gains add up exactly.

// HourGain is the recreation one hour of an activity of power 1.0 gives
// (wiki: Recreation, 36 % an hour x the activity's recreation power).
const HourGain = Full * 36 / 100

// Tolerance share of the gain and boredom thresholds (wiki: Recreation:
// tolerance rises by 2/3 of the recreation gained; bored above 50 %,
// no longer bored below 30 %).
const (
	TolNum, TolDen = 2, 3
	BoredAbove     = Full * 50 / 100 * 1000
	BoredBelow     = Full * 30 / 100 * 1000
)

// Tolerance is one colonist's tolerance per joy kind.
type Tolerance struct {
	Milli [NumJoyKinds]int32 // milli-points of Full
	Bored uint16             // bit k: bored of kind k
	At    uint64             // tick the tolerances last wore off
}

// DecayPct is the tolerance share (percent) worn off a day at colony
// wealth w (wiki: Recreation, by expectations: extremely low 18, very
// low 13, low 11, moderate 10, high 8, sky-high 7).
func DecayPct(w int) int {
	switch {
	case w < 15000:
		return 18
	case w < 31000:
		return 13
	case w < 81000:
		return 11
	case w < 182000:
		return 10
	case w < 308000:
		return 8
	}
	return 7
}

// Decay wears the tolerances off from At to now at pct percent of Full a
// day of ticksPerDay ticks, and clears boredom that fell below BoredBelow.
func (t *Tolerance) Decay(now uint64, pct, ticksPerDay int) {
	if now <= t.At {
		return
	}
	drop := int64(now-t.At) * int64(pct) * Full * 10 / int64(ticksPerDay) // milli: Full*1000*pct/100
	t.At = now
	for k := range t.Milli {
		t.Milli[k] = int32(max(int64(t.Milli[k])-drop, 0))
		t.mark(JoyKind(k))
	}
}

// Take is what a step of gain milli-points of kind k gives after the
// tolerance, which rises by TolNum/TolDen of that.
func (t *Tolerance) Take(k JoyKind, gain int) int {
	if k == JoyNone || k >= NumJoyKinds || gain <= 0 {
		return max(gain, 0)
	}
	got := int(int64(gain) * int64(Full*1000-t.Milli[k]) / (Full * 1000))
	t.Milli[k] = int32(min(int64(t.Milli[k])+int64(got)*TolNum/TolDen, Full*1000))
	t.mark(k)
	return got
}

// mark sets or clears k's boredom at its thresholds (between them it
// stays as it was).
func (t *Tolerance) mark(k JoyKind) {
	switch {
	case t.Milli[k] > BoredAbove:
		t.Bored |= 1 << k
	case t.Milli[k] < BoredBelow:
		t.Bored &^= 1 << k
	}
}

// IsBored reports the colonist won't seek kind k now.
func (t Tolerance) IsBored(k JoyKind) bool { return k < NumJoyKinds && t.Bored&(1<<k) != 0 }

// Pct is kind k's tolerance in whole percent.
func (t Tolerance) Pct(k JoyKind) int {
	if k >= NumJoyKinds {
		return 0
	}
	return int(t.Milli[k]) / (Full * 10)
}
