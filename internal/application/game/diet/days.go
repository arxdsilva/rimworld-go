package diet

import "rimworld-go/internal/domain/pawn/needs"

// DailyNeed is what one adult eats a day: 1.6 nutrition (wiki:
// Nutrition; needs.Full is 1.0).
const DailyNeed = needs.Full * 8 / 5

// Days is the colony's days of food in tenths: nutrition (needs units)
// over what eaters people eat a day; 0 with nobody to feed.
func Days(nutrition, eaters int) int {
	if eaters <= 0 || nutrition <= 0 {
		return 0
	}
	return nutrition * 10 / (eaters * DailyNeed)
}
