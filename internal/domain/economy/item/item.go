// Package item is ground (and carried) resource stacks.
package item

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
)

// Kind is a stack category.
type Kind uint8

const (
	Food Kind = iota // packaged survival ration
	Wood
	Stone    // stone blocks
	Berries  // raw, foraged
	Potatoes // raw, farmed
	Meat     // raw, hunted
	Meal     // cooked
	Club     // melee weapon
	Rifle    // ranged weapon
	// Corpses are butchered into meat at a butcher table.
	CorpseDustbuck
	CorpseRidgewolf
	// More farmed produce (raw).
	Corn
	Wheat
	Beans
	Lettuce
	Radishes
	Pumpkins
	// Leather comes from butchering; tailors make clothes from it.
	Leather
	// Clothes (apparel) are worn: see Slot and Insulation.
	Parka
	Tuque
	Duster
	// HerbalMedicine is dried healroot: doctors tend wounds with it.
	HerbalMedicine
	// Knife (stone) and Spear (wood) are melee weapons (SIM-015).
	Knife
	Spear
	// CorpseHuman is a dead colonist or raider (Stack.Of names who);
	// Bones are what a corpse rots down to (SIM-017).
	CorpseHuman
	Bones
	// CorpseAnimal is any animal's carcass (Stack.Animal names the
	// species, SIM-019); CorpseDustbuck and CorpseRidgewolf are legacy.
	CorpseAnimal
	// Cloth is picked from cotton; Straw comes with the wheat harvest (SIM-021).
	Cloth
	Straw
	// Carpets are crafted, then installed at an entrance: stepping on one
	// wipes off part of the dirt a colonist tracks in (filth.Catch).
	CarpetStraw
	CarpetCloth
	CarpetLeather
	CarpetWood
	// Animal products (SIM-026): milk and eggs are food, wool is a
	// textile, ember oil burns in a campfire. A FertileEgg hatches
	// (Stack.Animal names the species, Stack.Hatch its progress).
	Milk
	Wool
	Egg
	FertileEgg
	EmberOil
	WoolParka
	CarpetWool
	// Stone (SIM-029): mining drops a chunk of the rock's stone; a
	// stonecutter cuts chunks into blocks, the building material. The
	// legacy Stone kind above is migrated to GraniteBlocks on load.
	SandstoneChunk
	GraniteChunk
	LimestoneChunk
	SlateChunk
	MarbleChunk
	SandstoneBlocks
	GraniteBlocks
	LimestoneBlocks
	SlateBlocks
	MarbleBlocks
	// Metals are mined from ore veins; steel is also a building material.
	Steel
	Silver
	Gold
	// Components are salvaged or fabricated parts for advanced builds;
	// industrial medicine is refined from herbal (NG-1 §4.8).
	Components
	IndustrialMedicine
	// Social and host systems (ENG-003, append only, saved): drugs, their
	// crops' harvests and the simple prosthetics. Brewgrain is a raw food;
	// ashleaf is not food.
	Beer
	AshleafTea
	SparkTablet
	Brewgrain
	AshleafLeaf
	PegLeg
	ProstheticLeg
	ProstheticArm
	WoodenHand
	Denture
	HearingTrumpet
	// SIM-049: a foot for a lost foot, and two cosmetic face pieces.
	ProstheticFoot
	GlassEye
	WoodenNose
	// MED-05 (append only, saved): the nanomed kit (the best medicine;
	// never crafted, only found or traded), the servo tier of implants,
	// the valve heart, and natural organs (harvested in MED-06).
	NanomedKit
	ServoArm
	ServoLeg
	OpticImplant
	CochlearImplant
	SpinalBrace
	PumpHeart
	SynthStomach
	ValveHeart
	Kidney
	Lung
	Heart
	Liver
	// MED-07 (append only, saved): the medical drugs, a painkiller and a
	// prophylactic against plague, malaria and sleeping sickness.
	NumbrootTincture
	ProphylTablet
	// DEF-2 (append only, saved): the vanilla weapon catalogue (wiki:
	// Weapons) — medieval blades, bows and the pila, firearms, grenades
	// (one stack is the weapon) and launchers — and the armor (wiki: Armor).
	Mace
	Gladius
	Longsword
	ShortBow
	RecurveBow
	Greatbow
	Pila
	Revolver
	Autopistol
	MachinePistol
	AssaultRifle
	SniperRifle
	PumpShotgun
	ChainShotgun
	HeavySMG
	LMG
	Minigun
	FragGrenades
	MolotovCocktails
	EMPGrenades
	IncendiaryLauncher
	EMPLauncher
	FlakVest
	FlakPants
	FlakJacket
	PlateArmor
	MarineArmor
	MarineHelmet
	SimpleHelmet
	AdvancedHelmet
	// DEF-3 (append only, saved): mortar shells, loaded into a mortar and
	// buried in charges. "mortar shells" are the high-explosive ones.
	// wiki: High-explosive shell, Incendiary shell, EMP shell, Smoke shell
	MortarShells
	IncendiaryShells
	EMPShells
	SmokeShells
	// Wave E contract (append only, saved): PRD-4's recipe catalogue.
	// ENG-003: wiki numbers; the wiki's invented drug and reagent names are
	// replaced by our own (ash flake for flake, silverdust for yayo,
	// battle tonic for go-juice, reagent for neutroamine). Fuel and brewing
	// (chemfuel, wort), the meal tiers and kibble (animal feed), the hard
	// drugs, the high-tech materials and the mortar's reinforced barrel.
	Chemfuel
	Wort
	FineMeal
	LavishMeal
	Pemmican
	SurvivalMeal
	Kibble
	AshFlake
	Silverdust
	BattleTonic
	Reagent
	AdvancedComponents
	Uranium
	Plasteel
	ReinforcedBarrel
	numKinds
)

var kindNames = [numKinds]string{"food", "wood", "stone", "berries", "potatoes", "meat", "meal", "club", "rifle", "dustbuck corpse", "ridgewolf corpse",
	"corn", "wheat", "beans", "lettuce", "radishes", "pumpkins", "leather", "parka", "tuque", "duster", "herbal medicine",
	"knife", "spear", "human corpse", "bones", "animal corpse",
	"cloth", "straw", "straw carpet", "cloth carpet", "leather carpet", "wood carpet",
	"milk", "wool", "egg", "fertile egg", "ember oil", "wool parka", "wool carpet",
	"sandstone chunk", "granite chunk", "limestone chunk", "slate chunk", "marble chunk",
	"sandstone blocks", "granite blocks", "limestone blocks", "slate blocks", "marble blocks",
	"steel", "silver", "gold",
	"components", "industrial medicine",
	"beer", "ashleaf tea", "spark tablet", "brewgrain", "ashleaf",
	"peg leg", "prosthetic leg", "prosthetic arm", "wooden hand", "denture", "hearing trumpet",
	"prosthetic foot", "glass eye", "wooden nose",
	"nanomed kit", "servo arm", "servo leg", "optic implant", "cochlear implant", "spinal brace", "pump heart",
	"synth stomach", "valve heart", "kidney", "lung", "heart", "liver",
	"numbroot tincture", "prophyl tablet",
	"mace", "gladius", "longsword", "short bow", "recurve bow", "greatbow", "pila",
	"revolver", "autopistol", "machine pistol", "assault rifle", "sniper rifle", "pump shotgun", "chain shotgun",
	"heavy smg", "lmg", "minigun", "frag grenades", "molotov cocktails", "EMP grenades",
	"incendiary launcher", "emp launcher",
	"flak vest", "flak pants", "flak jacket", "plate armor", "marine armor", "marine helmet", "simple helmet", "advanced helmet",
	"mortar shells", "incendiary shells", "EMP shells", "smoke shells",
	"chemfuel", "wort", "fine meal", "lavish meal", "pemmican", "packaged survival meal", "kibble",
	"ash flake", "silverdust", "battle tonic", "reagent", "advanced components", "uranium", "plasteel", "reinforced barrel"}

// IsWeapon reports whether k can be equipped.
func (k Kind) IsWeapon() bool {
	return k == Club || k == Rifle || k == Knife || k == Spear || k >= Mace && k <= EMPLauncher
}

// IsArmor reports apparel whose job is protection (DEF-2: flak, plate,
// marine armor and helmets; armor.Apparel has the ratings).
func (k Kind) IsArmor() bool { return k >= FlakVest && k <= AdvancedHelmet }

// IsShell reports a mortar shell (DEF-3).
func (k Kind) IsShell() bool { return k >= MortarShells && k <= SmokeShells }

// IsCorpse reports an animal carcass.
func (k Kind) IsCorpse() bool {
	return k == CorpseAnimal || k == CorpseDustbuck || k == CorpseRidgewolf || k == CorpseHuman
}

// nutrition is hunger restored per unit eaten (needs.Full = 10000).
// Produce values follow real energy density per portion, rounded for
// play: grains and dry beans are dense, leafy greens are light.
var nutrition = [numKinds]int{Food: 4000, Berries: 1500, Meat: 2200, Meal: 6000,
	Potatoes: 1600, Corn: 1700, Wheat: 2400, Beans: 2400, Lettuce: 450, Radishes: 500, Pumpkins: 800,
	Milk: 900, Egg: 700, Brewgrain: 1200,
	// Wave E, wiki nutrition on the simple meal's scale (0.9 = 6000):
	// fine and survival meals 0.9, lavish 1.0, pemmican and kibble 0.05.
	FineMeal: 6000, LavishMeal: 6667, SurvivalMeal: 6000, Pemmican: 333, Kibble: 333}

// preference ranks food choice: higher is eaten first.
// Raw grain, dry beans and raw potatoes are edible but unappealing.
var preference = [numKinds]int{Meal: 4, Food: 3, Berries: 2, Corn: 2, Lettuce: 2, Radishes: 2, Pumpkins: 2,
	Potatoes: 1, Beans: 1, Wheat: 1, Meat: 1, Milk: 2, Egg: 1, Brewgrain: 1,
	// Wave E: the better the meal the sooner it is eaten; kibble (animal
	// feed) is left to PRD-4.
	LavishMeal: 6, FineMeal: 5, SurvivalMeal: 4, Pemmican: 3}

// Nutrition is the hunger restored by eating one unit of k (0 if inedible).
func (k Kind) Nutrition() int {
	if k >= numKinds {
		return 0
	}
	return nutrition[k]
}

// Preference ranks edible kinds; higher is chosen first.
func (k Kind) Preference() int {
	if k >= numKinds {
		return 0
	}
	return preference[k]
}

// Raw reports uncooked food (eating it is unpleasant): not a meal,
// pemmican or kibble.
func (k Kind) Raw() bool {
	return k.IsFood() && k != Food && k != Meal && (k < FineMeal || k > Kibble)
}

// Foods lists every edible kind.
func Foods() []Kind {
	return []Kind{Food, Berries, Potatoes, Corn, Wheat, Beans, Lettuce, Radishes, Pumpkins, Meat, Meal, Milk, Egg, Brewgrain,
		FineMeal, LavishMeal, Pemmican, SurvivalMeal, Kibble}
}

// IsDrug reports a drug: taken for its effect, not its nutrition (ENG-003).
// MED-07 adds the medical drugs (numbroot tincture, prophyl tablet);
// wave E the hard drugs (ash flake, silverdust, battle tonic).
func (k Kind) IsDrug() bool {
	return k == Beer || k == AshleafTea || k == SparkTablet || k == NumbrootTincture || k == ProphylTablet ||
		k >= AshFlake && k <= BattleTonic
}

// IsImplant reports a prosthetic installed by surgery (ENG-003; the
// servo tier and the valve heart from MED-05). Natural organs are not
// implants: see IsOrgan.
func (k Kind) IsImplant() bool {
	return k >= PegLeg && k <= HearingTrumpet || k >= ProstheticFoot && k <= WoodenNose || k >= ServoArm && k <= ValveHeart
}

// IsOrgan reports a natural organ, harvested from one body and installed
// in another (MED-05; the operations come with MED-06).
func (k Kind) IsOrgan() bool { return k >= Kidney && k <= Liver }

// IsMedicine reports a medicine doctors tend with: herbal, industrial
// or the nanomed kit (MED-05).
func (k Kind) IsMedicine() bool {
	return k == HerbalMedicine || k == IndustrialMedicine || k == NanomedKit
}

func (k Kind) String() string {
	if k >= numKinds {
		return "unknown"
	}
	return kindNames[k]
}

// ParseKind returns the kind for a name.
func ParseKind(name string) (Kind, bool) {
	for i, n := range kindNames {
		if n == name {
			return Kind(i), true
		}
	}
	return 0, false
}

// IsFood reports whether k can be eaten.
func (k Kind) IsFood() bool { return k.Nutrition() > 0 }

// Slot is where a piece of apparel is worn.
type Slot uint8

const (
	NoSlot Slot = iota
	Body        // outer layer over the shirt: parka, duster
	Head
	Legs // DEF-2: flak pants
)

func (s Slot) String() string {
	switch s {
	case Body:
		return "body"
	case Head:
		return "head"
	case Legs:
		return "legs"
	}
	return ""
}

// apparel: slot, cold insulation and heat insulation in tenths of a
// degree C. Cold insulation lowers the bottom of the wearer's comfortable
// range; heat insulation raises the top (negative: the garment is hot to
// wear). A leather parka is serious winter gear but sweltering in summer;
// a knitted tuque keeps the head warm; a long leather duster shades the
// wearer in heat and cuts the wind a little.
var apparel = [numKinds]struct {
	slot       Slot
	cold, heat int
}{
	Parka:  {Body, 200, -30},
	Tuque:  {Head, 60, 0},
	Duster: {Body, 40, 80},
	// knitted wool traps more warmth than leather and breathes a little
	WoolParka: {Body, 240, -20},
	// DEF-2 armor, wiki insulation (Normal quality; helmets in steel).
	FlakVest: {Body, 10, 0}, FlakPants: {Legs, 35, 10}, FlakJacket: {Body, 144, 30},
	PlateArmor: {Body, 0, 0}, MarineArmor: {Body, 680, 100},
	MarineHelmet: {Head, 40, 20}, SimpleHelmet: {Head, 4, 0}, AdvancedHelmet: {Head, 4, 0},
}

// IsApparel reports clothing that can be worn.
func (k Kind) IsApparel() bool { return k < numKinds && apparel[k].slot != NoSlot }

// Slot is where apparel k is worn (NoSlot for non-apparel).
func (k Kind) Slot() Slot {
	if k >= numKinds {
		return NoSlot
	}
	return apparel[k].slot
}

// Insulation is apparel k's cold and heat insulation (tenths of a degree).
func (k Kind) Insulation() (cold, heat int) {
	if k >= numKinds {
		return 0, 0
	}
	return apparel[k].cold, apparel[k].heat
}

// Apparel lists every wearable kind.
func Apparel() []Kind {
	return []Kind{Parka, Tuque, Duster, WoolParka,
		FlakVest, FlakPants, FlakJacket, PlateArmor, MarineArmor, MarineHelmet, SimpleHelmet, AdvancedHelmet}
}

// IsStuff reports whether k is a building material.
func (k Kind) IsStuff() bool { return k == Wood || IsStoneBlocks(k) || k == Steel }

// IsCarpet reports a crafted carpet (installed as a carpet building).
func (k Kind) IsCarpet() bool { return k >= CarpetStraw && k <= CarpetWood || k == CarpetWool }

// Carpets lists every carpet kind, cheapest material first.
func Carpets() []Kind { return []Kind{CarpetStraw, CarpetWood, CarpetLeather, CarpetCloth, CarpetWool} }

// StackLimit is how many items of one kind share a tile.
const StackLimit = 1000

// CarryLimit is how many items a colonist hauls in one trip.
const CarryLimit = 100

// Stack is a positive quantity of one kind at one location model.
type Stack struct {
	Kind    Kind
	Qty     int
	X, Y    int
	Carrier entity.EntityID // 0 = on ground or in a frame
	// Rot is spoilage accumulated so far, in ticks at full rot speed. The
	// stack spoils when Rot reaches Shelf().
	Rot int
	// Dried food (from a drying rack) keeps indefinitely.
	Dried bool
	// Of is the dead pawn a human corpse (or its bones) once was; DiedAt
	// the tick it died.
	Of     entity.EntityID
	DiedAt uint64
	// Animal is the species (animal.Species value) of a CorpseAnimal, a
	// FertileEgg or a Wool stack.
	Animal uint8
	// Hatch is a fertile egg's incubation so far (ticks); Tame: laid by a
	// tame mother, so the hatchling joins the colony.
	Hatch int
	Tame  bool
	// Forbidden stacks are left alone by colonists' autonomous work
	// (hauling, eating, building, crafting) until allowed (UI-016).
	Forbidden bool
	// Unclaimed stacks were found on the map, not made or brought by the
	// colony (ruin loot, stone chunks, a predator's kill): haulers leave
	// them where they lie unless HaulMarked. Picking one up or storing it
	// claims it (PLAY-017).
	Unclaimed  bool
	HaulMarked bool
	// Poison is a cooked meal's chance to give food poisoning, permille
	// per meal eaten (set by the cook from skill and kitchen, WS-M); 0
	// for everything else and for stacks from older saves.
	Poison int16
	// Quality (PRD-2) is the made item's quality stored as quality.Level+1
	// (economy/quality: Stored/FromStored); 0 = none (raw materials, food).
	Quality uint8
	// Wear (PRD-2) is the hit points the stack has lost to use and weather
	// (deterioration); 0 = pristine, so older saves load as new items.
	Wear int
	// Stuff (PRD-2) is the material a stuffed item is made of (a steel
	// longsword, a cloth parka); 0 = none or its kind's default.
	Stuff Kind
}

// Shelf is how many full-speed rot ticks the stack keeps (0 = forever).
func (s Stack) Shelf() int {
	if s.Dried {
		return 0
	}
	return s.Kind.ShelfTicks()
}

// Dryable reports whether s can go on a drying rack: perishable food that
// isn't dried or spoiled yet.
func (s Stack) Dryable() bool { return s.Kind.Dryable() && !s.Dried && !s.Rotten() }

// Dryable reports perishable food kinds (corpses are butchered, not dried;
// milk and eggs don't dry; pemmican is dried already).
func (k Kind) Dryable() bool {
	return k.IsFood() && k.Perishable() && k != Milk && k != Egg && k != Pemmican
}

// dryDays is how long a rack batch takes to dry in warm air: thin meat
// strips and leaves dry in about a day, juicy fruit, sliced roots and
// squash take longer, and cooked meals are dried into hard travel cakes.
var dryDays = [numKinds]float64{Meat: 1, Meal: 1, Lettuce: 0.75, Berries: 2, Radishes: 1.5,
	Potatoes: 2, Corn: 1.5, Pumpkins: 2.5, FineMeal: 1, LavishMeal: 1}

// DryTicks is how long a batch of k takes to dry at full speed.
func (k Kind) DryTicks() int {
	if k >= numKinds {
		return 0
	}
	return int(dryDays[k] * clock.TicksPerDay)
}

// DryRate is drying speed in permille at air temperature tenthsC: full
// speed from 20 C, slower in cool air, and a slow freeze-dry below 0 C.
func DryRate(tenthsC int) int {
	switch {
	case tenthsC >= 200:
		return 1000
	case tenthsC <= 0:
		return 250
	default:
		return 250 + 750*tenthsC/200
	}
}

// CanMerge reports whether a and b may combine on the same tile.
// Rotten corpses never merge with fresh ones, one person's body (or
// bones) never merges with another's, and animal corpses stack by species;
// fertile eggs stack by species and by whose they are (Tame), and the
// merged stack keeps the incubation of the one it joins. Made items
// stack only with their own quality, material and wear (PRD-2).
func CanMerge(a, b Stack) bool {
	return a.Kind == b.Kind && a.Carrier == 0 && b.Carrier == 0 && a.X == b.X && a.Y == b.Y &&
		a.Rotten() == b.Rotten() && a.Dried == b.Dried && a.Of == b.Of && a.Animal == b.Animal && a.Tame == b.Tame &&
		a.Forbidden == b.Forbidden && a.Unclaimed == b.Unclaimed && a.HaulMarked == b.HaulMarked &&
		a.Quality == b.Quality && a.Stuff == b.Stuff && a.Wear == b.Wear
}

// ConvertLegacy turns a save's former dustbuck or ridgewolf corpse into a
// CorpseAnimal of the species that took its slot (deer 0, timber wolf 1);
// ok is false (and s unchanged) for any other stack.
func ConvertLegacy(s Stack) (Stack, bool) {
	switch s.Kind {
	case CorpseDustbuck:
		s.Kind, s.Animal = CorpseAnimal, 0
	case CorpseRidgewolf:
		s.Kind, s.Animal = CorpseAnimal, 1
	default:
		return s, false
	}
	return s, true
}

// MergeRot is the spoilage of a stack of qa items with rot ra combined with
// qb items with rot rb: the quantity-weighted average.
func MergeRot(qa, ra, qb, rb int) int {
	if qa+qb <= 0 {
		return 0
	}
	return (qa*ra + qb*rb) / (qa + qb)
}

// MergePoison is the food-poisoning chance of qa items with chance pa
// combined with qb items with chance pb: the quantity-weighted average,
// rounded to the nearest permille.
func MergePoison(qa int, pa int16, qb int, pb int16) int16 {
	if qa+qb <= 0 {
		return 0
	}
	return int16((qa*int(pa) + qb*int(pb) + (qa+qb)/2) / (qa + qb))
}

// shelfDays is how long food keeps at room temperature (10 C or warmer)
// before it spoils; 0 never spoils. Calibrated to real produce, squeezed for
// play: fresh meat and cooked meals go off in days, greens in a week, roots
// and squash keep a season, and dried grain and beans keep indefinitely.
var shelfDays = [numKinds]float64{Meat: 2, Meal: 4, CorpseDustbuck: 2.5, CorpseRidgewolf: 2.5, CorpseAnimal: 2.5, CorpseHuman: 3,
	Berries: 10, Lettuce: 6, Radishes: 14, Potatoes: 30, Corn: 40, Pumpkins: 50,
	Milk: 3, Egg: 10,
	// Wave E, wiki days to rot: fine and lavish meals 4, pemmican 70, wort
	// 5.6 (the survival meal and kibble never rot).
	FineMeal: 4, LavishMeal: 4, Pemmican: 70, Wort: 5.6}

// ShelfTicks is how many full-speed rot ticks k survives (0 = never rots).
func (k Kind) ShelfTicks() int {
	if k >= numKinds {
		return 0
	}
	return int(shelfDays[k] * clock.TicksPerDay)
}

// Perishable reports whether k rots.
func (k Kind) Perishable() bool { return k.ShelfTicks() > 0 }

// Rotten reports a stack that has spoiled. Rotten food is destroyed; a
// rotten corpse lingers but can no longer be butchered.
func (s Stack) Rotten() bool {
	sh := s.Shelf()
	return sh > 0 && s.Rot >= sh
}

// FreshTicks is how many full-speed rot ticks remain before s spoils.
func (s Stack) FreshTicks() int {
	sh := s.Shelf()
	if sh == 0 || s.Rot >= sh {
		return 0
	}
	return sh - s.Rot
}

// RotRate is the rot speed in permille at air temperature tenthsC (tenths
// of a degree C): frozen food (0 C or below) doesn't rot, cool food rots
// proportionally slower, and from 10 C up it rots at full speed.
func RotRate(tenthsC int) int {
	switch {
	case tenthsC <= 0:
		return 0
	case tenthsC >= 100:
		return 1000
	default:
		return tenthsC * 10
	}
}

// Add returns qty after adding n, capped, and whether the result is still positive.
func Add(qty, n int) int {
	v := qty + n
	if v < 0 {
		return 0
	}
	if v > StackLimit {
		return StackLimit
	}
	return v
}

// Take removes n from qty. ok is false if qty would go negative.
func Take(qty, n int) (int, bool) {
	if n <= 0 || qty < n {
		return qty, false
	}
	return qty - n, true
}

// ChunkOf is the chunk mined from stone s (Food when s is None).
func ChunkOf(s geology.Stone) Kind {
	if s < geology.Sandstone || s > geology.Marble {
		return Food
	}
	return SandstoneChunk + Kind(s-geology.Sandstone)
}

// BlocksOf is the blocks cut from stone s (Food when s is None).
func BlocksOf(s geology.Stone) Kind {
	if s < geology.Sandstone || s > geology.Marble {
		return Food
	}
	return SandstoneBlocks + Kind(s-geology.Sandstone)
}

// StoneOf is the stone a chunk or blocks kind is made of; the legacy
// Stone kind reads as granite; anything else is None.
func StoneOf(k Kind) geology.Stone {
	switch {
	case IsChunk(k):
		return geology.Sandstone + geology.Stone(k-SandstoneChunk)
	case IsStoneBlocks(k):
		return geology.Sandstone + geology.Stone(k-SandstoneBlocks)
	case k == Stone:
		return geology.Granite
	}
	return geology.None
}

// IsChunk reports a rough stone chunk.
func IsChunk(k Kind) bool { return k >= SandstoneChunk && k <= MarbleChunk }

// IsStoneBlocks reports cut stone blocks.
func IsStoneBlocks(k Kind) bool { return k >= SandstoneBlocks && k <= MarbleBlocks }

// IsMetal reports a mined metal.
func IsMetal(k Kind) bool { return k == Steel || k == Silver || k == Gold }

// StoneBlocks lists the five block kinds in enum order.
func StoneBlocks() []Kind {
	return []Kind{SandstoneBlocks, GraniteBlocks, LimestoneBlocks, SlateBlocks, MarbleBlocks}
}
