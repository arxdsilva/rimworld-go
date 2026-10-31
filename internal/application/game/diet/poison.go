package diet

// Food poisoning chances (wiki: Ailments, Food poisoning), in basis points
// (hundredths of a percent). A cooked meal is poisoned when either of two
// rolls hits: the kitchen's (by room cleanliness, or a flat chance at an
// outdoor station) and the cook's (by Cooking skill). Raw food and
// carcasses poison by a flat chance; nutrient paste never does.
const (
	// OutdoorChance is the kitchen roll at a station under the sky.
	OutdoorChance = 200
	// kitchenMax caps the kitchen roll (a room at cleanliness -5).
	kitchenMax = 500
	// RawChance and CorpseChance are eating raw food and a carcass.
	RawChance    = 200
	CorpseChance = 500
)

// cookChance is the cook roll by Cooking level 0..8; 9 and above 0.1 %.
var cookChance = [...]int{500, 400, 300, 200, 150, 100, 50, 25, 15}

// CookChance is the cook roll of a cook at Cooking level lvl.
func CookChance(lvl int) int {
	if lvl < 0 {
		lvl = 0
	}
	if lvl < len(cookChance) {
		return cookChance[lvl]
	}
	return 10
}

// KitchenChance is the kitchen roll of a room of cleanliness clean
// (RimWorld cleanliness x100): -(C + 2) x 5 % / 3, none at -2 or
// cleaner, 5 % at -5; OutdoorChance outdoors.
func KitchenChance(clean int, indoors bool) int {
	if !indoors {
		return OutdoorChance
	}
	return min(max(-(clean+200)*5/3, 0), kitchenMax)
}

// MealChance is a cooked meal's chance from both rolls.
func MealChance(cook, kitchen int) int { return cook + kitchen - cook*kitchen/10000 }

// Permille is chance bp as item.Stack.Poison stores it (permille,
// rounded, at least 1 when there is any chance).
func Permille(bp int) int16 {
	if bp <= 0 {
		return 0
	}
	return int16(max((bp+5)/10, 1))
}
