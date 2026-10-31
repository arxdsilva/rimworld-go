package incident

import "rimworld-go/internal/domain/sim/clock"

// Climate events (TEMP-2, wiki: Events). A heat wave adds 17 °C and a
// cold snap takes 20 °C off the outdoor air for 1.5 to 3.5 days; a
// volcanic winter takes 7 °C off for 7.5 to 40 days, dims the sky light
// by 30 % and halves the wildlife the land carries. Each ramps in from
// nothing over its ramp (12,000 ticks, 50,000 for the volcanic winter)
// and back out over the same ramp before it ends. Only one runs at a
// time: they block one another (wiki). A heat wave needs the season's
// temperature (the day's mean) at 20 °C or more; a cold snap needs it
// above 0 and below 15 °C (the wiki's "summer temp 0-15 °C", read as the
// season's temperature now, the same measure as the heat wave's); a
// volcanic winter comes no earlier than day 60. Weights are original
// tuning (the wiki's are unconfirmed); the wizard's Extreme weather
// multiplies them by ExtremeClimatePct.
const (
	HeatWaveOffset  = 170 // tenths of a degree, wiki: Events
	ColdSnapOffset  = -200
	VolcanicOffset  = -70
	SnapRamp        = 12000 // ticks in and out, wiki: Events
	VolcanicRamp    = 50000
	SnapMin         = 150 // hundredths of a day, wiki: Events
	SnapMax         = 350
	VolcanicMin     = 750
	VolcanicMax     = 4000
	HeatWaveMinTemp = 200 // tenths: the day's mean a heat wave needs, wiki: Events
	ColdSnapMinTemp = 0   // tenths: a cold snap needs the day's mean above this ...
	ColdSnapMaxTemp = 150 // ... and below this, wiki: Events
	// VolcanicSkyPermil and VolcanicWildPermil are the sky light and the
	// wildlife capacity at the volcanic winter's full strength (wiki:
	// light -30 %, wildlife halved).
	VolcanicSkyPermil  = 700
	VolcanicWildPermil = 500
	// ExtremeClimatePct: the wizard's Extreme weather triples the climate
	// events' weights (original tuning).
	ExtremeClimatePct = 300
)

// IsClimate reports a climate event (heat wave, cold snap, volcanic
// winter).
func IsClimate(id ID) bool { return id == HeatWave || id == ColdSnap || id == VolcanicWinter }

// climateSpec is a climate event's offset (tenths), ramp (ticks) and
// length range (hundredths of a day).
func climateSpec(id ID) (offset, ramp, minLen, maxLen int) {
	switch id {
	case HeatWave:
		return HeatWaveOffset, SnapRamp, SnapMin, SnapMax
	case ColdSnap:
		return ColdSnapOffset, SnapRamp, SnapMin, SnapMax
	case VolcanicWinter:
		return VolcanicOffset, VolcanicRamp, VolcanicMin, VolcanicMax
	}
	return 0, 0, 0, 0
}

// ClimateTicks is how long climate event id lasts for roll r in
// [0, ClimateSpan(id)]: its shortest length plus r ticks (0 for any other
// event).
func ClimateTicks(id ID, r int) uint64 {
	if !IsClimate(id) {
		return 0
	}
	_, _, lo, _ := climateSpec(id)
	return uint64(lo)*clock.TicksPerDay/100 + uint64(max(r, 0))
}

// ClimateSpan is the spread (ticks) of climate event id's length.
func ClimateSpan(id ID) int {
	_, _, lo, hi := climateSpec(id)
	return (hi - lo) * clock.TicksPerDay / 100
}

// ClimateFits reports whether climate event id suits a day whose mean
// outdoor temperature (the season's, tenths) is seasonal.
func ClimateFits(id ID, seasonal int) bool {
	switch id {
	case HeatWave:
		return seasonal >= HeatWaveMinTemp
	case ColdSnap:
		return seasonal > ColdSnapMinTemp && seasonal < ColdSnapMaxTemp
	}
	return id == VolcanicWinter
}

// StartClimate starts climate event id at tick now lasting ticks; false
// for an event that is not a climate event, or while one is under way
// (they exclude one another).
func (st *State) StartClimate(id ID, now, ticks uint64) bool {
	if !IsClimate(id) || st.ClimateOn(now) {
		return false
	}
	st.Climate, st.ClimateFrom, st.ClimateUntil = id, now, now+ticks
	return true
}

// ClimateOn reports a climate event under way at tick now.
func (st State) ClimateOn(now uint64) bool { return st.ClimateUntil > now && IsClimate(st.Climate) }

// EndClimate clears the climate event that is over at tick now and
// returns it (false: none ended).
func (st *State) EndClimate(now uint64) (ID, bool) {
	if st.ClimateUntil == 0 || now < st.ClimateUntil {
		return 0, false
	}
	id := st.Climate
	st.Climate, st.ClimateFrom, st.ClimateUntil = 0, 0, 0
	return id, IsClimate(id)
}

// climateStrength is how far in the climate event under way is at tick
// now, permille: rising over its ramp, full, falling over its ramp to its
// end (0: none).
func (st State) climateStrength(now uint64) int {
	if !st.ClimateOn(now) || now < st.ClimateFrom {
		return 0
	}
	_, ramp, _, _ := climateSpec(st.Climate)
	in, out := now-st.ClimateFrom, st.ClimateUntil-now
	return int(min(in, out, uint64(ramp)) * 1000 / uint64(ramp))
}

// ClimateOffset is the outdoor temperature offset (tenths of a degree)
// of the climate event under way at tick now (0: none).
func (st State) ClimateOffset(now uint64) int {
	off, _, _, _ := climateSpec(st.Climate)
	return off * st.climateStrength(now) / 1000
}

// SkyPermil is the sky light at tick now, permille of the plain
// daylight: the volcanic winter dims it.
func (st State) SkyPermil(now uint64) int {
	if st.Climate != VolcanicWinter {
		return 1000
	}
	return 1000 - (1000-VolcanicSkyPermil)*st.climateStrength(now)/1000
}

// WildlifePermil is the wildlife capacity at tick now, permille of the
// biome's: the volcanic winter halves it.
func (st State) WildlifePermil(now uint64) int {
	if st.Climate != VolcanicWinter {
		return 1000
	}
	return 1000 - (1000-VolcanicWildPermil)*st.climateStrength(now)/1000
}

// ClimateView is the climate event under way at tick now for the views:
// its key ("heat_wave", "cold_snap", "volcanic_winter" or "") and its end
// tick (0: none).
func (st State) ClimateView(now uint64) (string, uint64) {
	if !st.ClimateOn(now) {
		return "", 0
	}
	return DefOf(st.Climate).Key, st.ClimateUntil
}

// ClimateLetter is the letter's title and text for climate event id
// lasting ticks.
func ClimateLetter(id ID, ticks uint64) (title, text string) {
	days := dayText(float64(ticks) / clock.TicksPerDay)
	switch id {
	case HeatWave:
		return "Heat wave", "Hot air is rolling in and will hang over the land for about " + days +
			", up to 17 degrees above the season. Keep everyone out of the midday sun and find somewhere cool to sleep."
	case ColdSnap:
		return "Cold snap", "A bitter cold is coming down for about " + days +
			", up to 20 degrees below the season. Keep the fires fed, dress warmly and bring the tender crops in if you can."
	case VolcanicWinter:
		return "Volcanic winter", "A far-off eruption has filled the sky with ash. For about " + days +
			" the air will be up to 7 degrees colder, the daylight a third dimmer, and the land will carry half its wildlife."
	}
	return "", ""
}

// ClimateEndText is the message when climate event id is over.
func ClimateEndText(id ID) string {
	switch id {
	case HeatWave:
		return "The heat wave has broken."
	case ColdSnap:
		return "The cold snap is over."
	case VolcanicWinter:
		return "The ash has cleared from the sky: the volcanic winter is over."
	}
	return ""
}

// ClimateShift is how far the outdoor curve moves at tick now: the
// climate event's offset now less applied, the offset already in it;
// applied becomes the offset now.
func (st State) ClimateShift(now uint64, applied *int) int {
	off := st.ClimateOffset(now)
	d := off - *applied
	*applied = off
	return d
}

// StartTimed starts sky or climate event id at tick now for a length
// rolled with roll (one draw, roll(n) in [0, n)) and returns its letter;
// ok false when id is neither, or one of its kind is under way.
func (st *State) StartTimed(id ID, now uint64, roll func(int) int) (title, text string, ok bool) {
	switch {
	case id == Eclipse || id == ToxicFallout:
		ticks := SkyTicks(id, roll(SkySpan(id)+1))
		if !st.StartSky(id, now, ticks) {
			return "", "", false
		}
		title, text = SkyLetter(id, ticks)
	case IsClimate(id):
		ticks := ClimateTicks(id, roll(ClimateSpan(id)+1))
		if !st.StartClimate(id, now, ticks) {
			return "", "", false
		}
		title, text = ClimateLetter(id, ticks)
	default:
		return "", "", false
	}
	return title, text, true
}
