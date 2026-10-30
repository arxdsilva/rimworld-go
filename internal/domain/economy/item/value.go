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
	// MED-07: numbroot tincture original tuning; wiki: Penoxycyline (18).
	NumbrootTincture: 16, ProphylTablet: 18,
	// DEF-2, Normal quality (stuffed ones in steel). wiki: Weapons, Mace,
	// Gladius, Longsword, Incendiary launcher, EMP launcher, Armor pages.
	Mace: 117, Gladius: 138, Longsword: 255, ShortBow: 45, RecurveBow: 66, Greatbow: 104, Pila: 109,
	Revolver: 135, Autopistol: 139, MachinePistol: 220, AssaultRifle: 480, SniperRifle: 530,
	PumpShotgun: 255, ChainShotgun: 405, HeavySMG: 355, LMG: 425, Minigun: 1160,
	FragGrenades: 265, MolotovCocktails: 245, EMPGrenades: 315, IncendiaryLauncher: 340, EMPLauncher: 505,
	FlakVest: 225, FlakPants: 225, FlakJacket: 290, PlateArmor: 460, MarineArmor: 2035,
	MarineHelmet: 635, SimpleHelmet: 88, AdvancedHelmet: 260,
	// DEF-3 shells: original tuning (the research notes carry no shell
	// market values), about their steel, chemfuel and components.
	MortarShells: 50, IncendiaryShells: 50, EMPShells: 60, SmokeShells: 40,
	// Wave E, wiki market values rounded to whole silver (the wiki
	// stack limits are stackLimit below). wiki: Chemfuel 2.3, Wort 2.5, Fine meal,
	// Lavish meal, Pemmican 1.4, Packaged survival meal, Kibble 1.1, Flake,
	// Yayo, Go-juice, Neutroamine, Advanced component, Uranium, Plasteel,
	// Reinforced barrel.
	Chemfuel: 2, Wort: 3, FineMeal: 20, LavishMeal: 40, Pemmican: 1, SurvivalMeal: 24, Kibble: 1,
	AshFlake: 14, Silverdust: 21, BattleTonic: 53, Reagent: 6,
	AdvancedComponents: 200, Uranium: 6, Plasteel: 9, ReinforcedBarrel: 600,
	// STO/CBT contract, wiki market values (Normal quality; the ikwa in
	// steel): Nutrient paste meal, Smokepop pack, Shield belt, Doomsday and
	// Triple rocket launcher, Smoke launcher, Charge rifle, Charge lance,
	// Ikwa.
	NutrientPasteMeal: 10, SmokepopPack: 166, ShieldBelt: 390, RocketLauncher: 1000, TripleRocketLauncher: 1000,
	SmokeLauncher: 380, ChargeRifle: 1010, ChargeLance: 1355, Ikwa: 113,
}

// Value is the market value of one unit of k.
func (k Kind) Value() int {
	if k >= numKinds {
		return 0
	}
	return value[k]
}

// stackLimit is the wiki's per-kind stack limit (PRD-4 for the wave E
// kinds, STO-2 for the older ones; wiki: each item's page): silver and
// gold 500; chemfuel, the hard drugs, reagent, ashleaf tea (psychite tea)
// and the prophyl tablet (penoxycyline) 150; wood, stone blocks, steel,
// plasteel, uranium, raw food, milk, eggs, textiles, kibble and pemmican
// 75; components 50; medicine, beer, wort, shells and the reinforced
// barrel 25; meals and fertilized eggs 10. A thing with no stack limit in
// the game (weapons, apparel, body parts, corpses, chunks) is one per
// stack. Original items take the limit of the vanilla item they stand
// in for (original tuning): the ration and the paste meal a meal's,
// ember oil chemfuel's, the spark tablet a hard drug's, the numbroot
// tincture medicine's, bones a corpse's, ashleaf and brewgrain raw
// plants', straw hay's (75). Carpets keep StackLimit (our items, no
// vanilla counterpart).
var stackLimit = func() (t [numKinds]int) {
	for k := Kind(0); k < numKinds; k++ {
		switch {
		case k.IsWeapon(), k.IsContractWeapon(), k.IsApparel(), k.IsBelt(), k.IsCorpse(), IsChunk(k), k.IsImplant(), k.IsOrgan():
			t[k] = 1
		}
	}
	for k, n := range map[Kind]int{
		Silver: 500, Gold: 500,
		Chemfuel: 150, AshFlake: 150, Silverdust: 150, BattleTonic: 150, Reagent: 150, AshleafTea: 150, ProphylTablet: 150,
		SparkTablet: 150, EmberOil: 150,
		Wood: 75, Stone: 75, SandstoneBlocks: 75, GraniteBlocks: 75, LimestoneBlocks: 75, SlateBlocks: 75, MarbleBlocks: 75,
		Steel: 75, Plasteel: 75, Uranium: 75, Berries: 75, Potatoes: 75, Corn: 75, Wheat: 75, Beans: 75, Lettuce: 75,
		Radishes: 75, Pumpkins: 75, Brewgrain: 75, AshleafLeaf: 75, Meat: 75, Milk: 75, Egg: 75, Leather: 75, Cloth: 75,
		Wool: 75, Straw: 75, Kibble: 75, Pemmican: 75,
		Components: 50, AdvancedComponents: 50,
		HerbalMedicine: 25, IndustrialMedicine: 25, NanomedKit: 25, NumbrootTincture: 25, Beer: 25, Wort: 25,
		MortarShells: 25, IncendiaryShells: 25, EMPShells: 25, SmokeShells: 25, ReinforcedBarrel: 25,
		Food: 10, Meal: 10, FineMeal: 10, LavishMeal: 10, SurvivalMeal: 10, NutrientPasteMeal: 10, FertileEgg: 10,
		Bones: 1,
	} {
		t[k] = n
	}
	return t
}()

// StackLimit is how many items of kind k share a tile (StackLimit for a
// kind without a wiki limit here).
func (k Kind) StackLimit() int {
	if k < numKinds && stackLimit[k] > 0 {
		return stackLimit[k]
	}
	return StackLimit
}
