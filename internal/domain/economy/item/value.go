package item

// value is a rough market value per unit, used to weigh a colony's wealth.
var value = [numKinds]int{
	Food: 2, Wood: 1, Stone: 1, Berries: 1, Potatoes: 1, Meat: 2, Meal: 5,
	Club: 15, Knife: 10, Spear: 20, Rifle: 120, CorpseDustbuck: 20, CorpseRidgewolf: 25, CorpseAnimal: 20,
	Corn: 1, Wheat: 1, Beans: 1, Lettuce: 1, Radishes: 1, Pumpkins: 1,
	Leather: 3, Parka: 80, Tuque: 20, Duster: 60, HerbalMedicine: 15,
	Cloth: 2, Straw: 1, CarpetStraw: 15, CarpetWood: 12, CarpetLeather: 25, CarpetCloth: 30,
	Milk: 2, Wool: 3, Egg: 2, FertileEgg: 4, EmberOil: 4, WoolParka: 95, CarpetWool: 30,
	SandstoneBlocks: 1, GraniteBlocks: 1, LimestoneBlocks: 1, SlateBlocks: 1, MarbleBlocks: 1,
	Steel: 2, Silver: 1, Gold: 10,
	Components: 28, IndustrialMedicine: 24,
	Beer: 12, AshleafTea: 10, SparkTablet: 18, Brewgrain: 1, AshleafLeaf: 2,
	PegLeg: 15, ProstheticLeg: 120, ProstheticArm: 120, WoodenHand: 12, Denture: 10, HearingTrumpet: 20,
	// MED-00: original tuning for these original items.
	ProstheticFoot: 90, GlassEye: 40, WoodenNose: 5,
	// MED-05. wiki: Glitterworld medicine (50), Bionic arm/leg/eye/spine/
	// heart (1030), Bionic ear/stomach (785), Prosthetic heart (230),
	// Kidney (900), Lung (1000), Heart (1200), Liver (1200).
	NanomedKit: 50,
	ServoArm:   1030, ServoLeg: 1030, OpticImplant: 1030, SpinalBrace: 1030, PumpHeart: 1030,
	CochlearImplant: 785, SynthStomach: 785, ValveHeart: 230,
	Kidney: 900, Lung: 1000, Heart: 1200, Liver: 1200,
}

// Value is the market value of one unit of k.
func (k Kind) Value() int {
	if k >= numKinds {
		return 0
	}
	return value[k]
}
