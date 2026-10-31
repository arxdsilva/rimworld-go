package needs

// Hunger rate and immunity by saturation (FOOD-6, wiki: Saturation). A
// pawn's hunger rate is its base rate x (1 + the summed factor offsets) x
// the multipliers. The multipliers (gourmand x1.5) live in the HungerRate
// stat; the offsets come from conditions (gut worms +100 %, malnutrition
// +50 / +60 %: body.HungerOffset) and from being stoned on the smokeleaf
// stand-in (ashleaf tea, StonedHunger). The wiki's hypothermia figures
// (-10 / -40 / -95 %) belong to the insectoids' hypothermic slowdown, not
// to a colonist's hypothermia (wiki: Hypothermia, Hypothermic slowdown),
// and nothing here models insect exposure, so no offset stands for them.

// StonedHunger is the hunger rate factor offset (permille) while stoned
// on smokeleaf. wiki: Saturation (+30 %).
const StonedHunger = 300

// HungerRate is hunger rate hr (permille, its multipliers already in it)
// with the summed factor offsets off (permille): hr x (1000 + off) / 1000,
// never below 0.
func HungerRate(hr, off int32) int32 { return max(hr*(1000+off)/1000, 0) }

// ImmunityGain is the immunity gain factor (permille) at saturation band
// b. wiki: Saturation: Fed and Hungry x100 %, Ravenously hungry x90 %,
// Malnourished x70 %. The band carries Settle's hysteresis (BandGap), so
// the factor never flaps at a band edge.
func ImmunityGain(b Band) int32 {
	switch b {
	case Ravenous:
		return 900 // wiki: Saturation
	case Malnourished:
		return 700 // wiki: Saturation
	}
	return 1000
}
