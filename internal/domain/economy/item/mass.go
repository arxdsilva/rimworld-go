package item

// Mass (AX-9, ENG-003): one unit's mass in grams, for what a transport
// pod can lift (150 kg a pod). The figures are the wiki's per-unit Mass
// (wiki: each item's infobox; Steel, Wood, Silver, Chemfuel, Components,
// Meals, Medicine, Weapons, Apparel); a kind the wiki does not weigh, or
// one without a vanilla counterpart, takes its category's mass (original
// tuning, massByCategory). Units are unscaled, as the deep-ore lumps and
// the mineral sites are.

// massOf are the kinds weighed one by one (grams; wiki unless noted).
var massOf = map[Kind]int{
	Wood: 400, Stone: 1000, Steel: 500, Silver: 8, Gold: 8, Plasteel: 250, Uranium: 1000, Jade: 500,
	Components: 600, AdvancedComponents: 600, Chemfuel: 50, Cloth: 26, Leather: 30, Wool: 28, Straw: 14, Hay: 14,
	Berries: 30, Potatoes: 30, Meat: 30, Corn: 30, Wheat: 30, Beans: 30, Lettuce: 30, Radishes: 30, Pumpkins: 30,
	Rice: 30, Strawberries: 30, Hops: 30, HumanMeat: 30, InsectMeat: 30, Fish: 30, AgaveFruit: 30, Brewgrain: 30,
	Milk: 45, Egg: 150, FertileEgg: 150, Meal: 440, FineMeal: 440, LavishMeal: 440, NutrientPasteMeal: 440,
	SurvivalMeal: 300, Pemmican: 18, Kibble: 15, Beer: 300, Wort: 20,
	HerbalMedicine: 350, IndustrialMedicine: 500, NanomedKit: 500,
	Club: 2000, Knife: 500, Spear: 3000, Mace: 2000, Gladius: 1500, Longsword: 2000, ShortBow: 1000,
	RecurveBow: 1000, Greatbow: 1200, Pila: 3000, Rifle: 4200, Revolver: 1400, Autopistol: 1200,
	MachinePistol: 1600, AssaultRifle: 3500, SniperRifle: 7000, PumpShotgun: 3400, ChainShotgun: 4400,
	HeavySMG: 2600, LMG: 8500, Minigun: 20000, Parka: 3500, Tuque: 150, Duster: 1800, WoolParka: 3500,
	TShirt: 250, ButtonDownShirt: 250, Pants: 500, Jacket: 1500, CowboyHat: 200, BowlerHat: 200, Tribalwear: 1000,
	FlakVest: 6000, FlakPants: 3000, FlakJacket: 7000, PlateArmor: 18000, MarineArmor: 18000,
	MarineHelmet: 2000, SimpleHelmet: 2000, AdvancedHelmet: 1500,
	Novel: 500, Textbook: 500, Schematic: 500, MindCore: 2000, AntimatterWarhead: 5000,
	// original: a body (the human figure is the wiki's 60 kg), bones,
	// a crated building of average size.
	CorpseHuman: 60000, CorpseAnimal: 30000, CorpseDustbuck: 30000, CorpseRidgewolf: 30000, Bones: 1000,
	MinifiedBuilding: 10000,
}

// massByCategory is the original fallback per resource category (grams).
var massByCategory = map[string]int{
	"Food": 300, "Materials": 500, "Medicine": 300, "Apparel": 1500, "Weapons": 3000,
	"Manufactured": 1000, "Corpses": 30000, "Other": 1000,
}

// MassGrams is one unit of k's mass in grams (always at least 1).
func (k Kind) MassGrams() int {
	if g, ok := massOf[k]; ok {
		return g
	}
	switch {
	case IsChunk(k):
		return 25000 // wiki: a stone chunk
	case IsStoneBlocks(k):
		return 1000 // wiki: Stone blocks
	}
	return massByCategory[k.Category()]
}
