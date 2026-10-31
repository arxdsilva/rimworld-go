package recipe

import (
	"slices"
	"strconv"

	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
)

// ID identifies a recipe.
type ID uint8

const (
	SimpleMeal ID = iota
	Club
	// ButcherDustbuck and ButcherRidgewolf are legacy (older saves' bills):
	// ButcherAnimal butchers every species.
	ButcherDustbuck
	ButcherRidgewolf
	DryFood
	MakeParka
	MakeTuque
	MakeDuster
	CampfireMeal
	MakeRifle
	// Knife (stone) and Spear (wood): hand-made melee weapons.
	Knife
	Spear
	// Cremate burns a human corpse on a funeral pyre.
	Cremate
	// ButcherAnimal butchers any animal corpse; its yields come from the
	// species (SIM-019).
	ButcherAnimal
	// Carpets (SIM-021): woven straw and wood slats at the workbench, cloth
	// and leather at the tailoring bench.
	MakeStrawCarpet
	MakeWoodCarpet
	SewClothCarpet
	SewLeatherCarpet
	// Wool (SIM-026): a knitted parka and a woven carpet at the
	// tailoring bench.
	KnitWoolParka
	WeaveWoolCarpet
	// Stonecutting (SIM-029): one chunk becomes a pile of blocks of its stone.
	CutSandstone
	CutGranite
	CutLimestone
	CutSlate
	CutMarble
	// FabricateComponent and RefineMedicine (NG-1 §4.8): components from
	// steel, industrial medicine from herbal medicine, both at the
	// workbench behind research and a minimum skill.
	FabricateComponent
	RefineMedicine
	// Social and host systems (ENG-003, append only, saved): beer at the
	// brewing bench, ashleaf tea and spark tablets at the drug table, and
	// the simple prosthetics at the workbench.
	BrewBeer
	BrewTea
	PressTablet
	MakePegLeg
	MakeProstheticLeg
	MakeProstheticArm
	MakeWoodenHand
	MakeDenture
	MakeHearingTrumpet
	// SIM-049 implants.
	MakeProstheticFoot
	MakeGlassEye
	MakeWoodenNose
	// MED-05: the servo tier (behind ServoReplacements) and the valve
	// heart (behind Prosthetics).
	MakeServoArm
	MakeServoLeg
	MakeOpticImplant
	MakeCochlearImplant
	MakeSpinalBrace
	MakePumpHeart
	MakeSynthStomach
	MakeValveHeart
	// MED-07: the medical drugs at the drug table, behind Pharmacology.
	MakeNumbroot
	PressProphyl
	// PRD-3 (append only, saved): the smithy's blades and the pila, the
	// crafting spot's bows, the machining table's guns, smelting at the
	// electric smelter and the sculpting bench's sculptures (benches.go).
	MakeMace
	MakeGladius
	MakeLongsword
	MakePila
	MakeShortBow
	MakeRecurveBow
	MakeGreatbow
	MakeRevolver
	MakeAutopistol
	MakeMachinePistol
	MakePumpShotgun
	MakeChainShotgun
	MakeHeavySMG
	MakeLMG
	MakeAssaultRifle
	MakeSniperRifle
	MakeMinigun
	MakeIncendiaryLauncher
	MakeEMPLauncher
	MakeEMPGrenades
	SmeltWeapon
	SculptSmall
	SculptLarge
	SculptGrand
	// PRD-4 (append only, saved): the wave C-E catalogue (wave_e.go):
	// the better meals, pemmican and survival meals at the stove, kibble
	// at the butcher table and spot, wort at the brewing bench and the
	// barrel's fill, the hard drugs and reagent at the drug table,
	// chemfuel at the refinery, advanced components at the fabrication
	// bench, armour and apparel, grenades and mortar shells, and smelting
	// metal apparel.
	CookFineMeal
	CookLavishMeal
	CookPemmican
	CookSurvivalMeal
	MakeKibble
	MakeKibbleSpot
	BrewWort
	FillBarrel
	MakeAshFlake
	MakeSilverdust
	MakeBattleTonic
	MakeReagent
	RefineChemfuelWood
	RefineChemfuelFood
	FabricateAdvanced
	SmeltApparel
	MakeFlakVest
	MakeFlakPants
	MakeFlakJacket
	MakePlateArmor
	MakeSimpleHelmet
	MakeAdvancedHelmet
	MakeMarineArmor
	MakeMarineHelmet
	MakeFragGrenades
	MakeMolotovs
	MakeHEShells
	MakeIncendiaryShells
	MakeEMPShells
	MakeSmokeShells
	// ShredMech (DEF-4 wrecks) shreds a mechanoid's wreck for salvage.
	ShredMech
	// ButcherHuman (FOOD-3, append only, saved) butchers a human corpse
	// into human meat; never a default bill (wiki: Butcher table).
	ButcherHuman
	// FOOD-4 (append only, saved): the vegetarian and carnivore fine and
	// lavish meals and the x4 batch bills (wiki: Meals).
	CookFineMealVeg
	CookFineMealMeat
	CookLavishMealVeg
	CookLavishMealMeat
	CookSimpleMealBulk
	CookFineMealBulk
	CookLavishMealBulk
	CookSurvivalMealBulk
	CookPemmicanBulk
	numRecipes
)

// Input is what a recipe consumes. AnyRaw accepts any raw food kind;
// AnyDryable any perishable food that isn't dried yet. Qty is the least a
// stack must hold; Max, when set, takes up to that many from one stack.
type Input struct {
	Kind       item.Kind
	AnyRaw     bool
	AnyDryable bool
	// AnyStoneBlocks accepts blocks of any stone (SIM-029).
	AnyStoneBlocks bool
	Qty            int
	Max            int
	// Stuff (PRD-2), when set, takes any material of these families
	// instead (Kind is then the default material, AnyStoneBlocks ignored):
	// the stack's kind becomes the product's Stack.Stuff.
	Stuff item.StuffCategory
	// Smelt (PRD-3) takes any item made with steel (SmeltYield > 0).
	Smelt bool
	// PRD-4: AnyVeg takes raw plant food, AnyMeat meat and animal
	// products (milk, eggs); SmeltWear takes apparel made with steel.
	AnyVeg, AnyMeat bool
	SmeltWear       bool
	// Wreck takes a mechanoid's wreck (a CorpseAnimal of a mechanoid);
	// any other corpse input never does (butchering skips wrecks).
	Wreck bool
	// Hops (FOOD-2) takes hops or brewgrain, the wort crops.
	Hops bool
	// Hay (FOOD-4) lets an AnyVeg input take hay too (wiki: Kibble).
	Hay bool
}

// Take is how many items the input takes from a stack of qty.
func (in Input) Take(qty int) int {
	if in.Max > 0 {
		return min(qty, in.Max)
	}
	return in.Qty
}

// Def is one recipe.
type Def struct {
	ID    ID
	Name  string
	Label string
	Bench construction.Kind
	Input Input
	// Inputs (PRD-1), when set, is every ingredient the recipe takes, each
	// gathered on its own (Input is then Inputs[0] for the older callers).
	// Unset, the recipe takes Input alone.
	Inputs []Input
	Output item.Kind
	OutQty int
	Work   int
	Skill  work.Type // work type that does it
	// Learn is the skill the work trains and is sped up by; MinSkill is
	// the level a pawn needs before taking the bill (0: anyone).
	Learn    skill.ID
	MinSkill uint8
	// Extra is a by-product made alongside Output (leather from butchering).
	Extra    item.Kind
	ExtraQty int
	// BySpecies: the yields (Output meat, Extra leather) come from the
	// carcass's species, not OutQty/ExtraQty. Legacy recipes are kept for
	// older saves only and offered nowhere (Upgrade).
	BySpecies bool
	Legacy    bool
	// Dry recipes don't produce on the spot: the work lays the batch out on
	// the rack, which dries it over time into a dried stack of the same kind.
	Dry bool
	// Also (PRD-3) lists the other benches that run the recipe besides
	// Bench (At; electric twins follow their base bench).
	Also []construction.Kind
	// Builds (PRD-3): the product is a building of this kind (a
	// sculpture) set down beside the bench, not an item; Output is unused.
	Builds construction.Kind
	// Ferment (PRD-4): the work pours the input into the barrel, which
	// ferments it into Output over days (craft.Fermenting); nothing is
	// made on the spot.
	Ferment bool
	// Salvage: the yields (Output steel, Extra plasteel) come from the
	// wreck's mechanoid (SalvageOf), not OutQty/ExtraQty.
	Salvage bool
}

// butcherWork is butchering's work, ticks (wiki: Butcher table, 450).
const butcherWork = 450

var defs = [numRecipes]Def{
	// wiki: Meals: a simple meal is 0.5 nutrition of raw food, 10 units.
	SimpleMeal: {ID: SimpleMeal, Name: "simple_meal", Label: "Cook simple meal", Bench: construction.Stove,
		Input: Input{AnyRaw: true, Qty: 10}, Output: item.Meal, OutQty: 1, Work: 120, Skill: work.Cook, Learn: skill.Cooking},
	Club: {ID: Club, Name: "club", Label: "Make wooden club", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 4, Stuff: item.MadeFrom(item.Club)}, Output: item.Club, OutQty: 1, Work: 240, Skill: work.Craft, Learn: skill.Crafting,
		Also: spotAndSmithy},
	ButcherDustbuck: {ID: ButcherDustbuck, Name: "butcher_dustbuck", Label: "Butcher dustbuck", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseDustbuck, Qty: 1}, Output: item.Meat, OutQty: 12, Work: butcherWork, Skill: work.Cook, Learn: skill.Cooking,
		Extra: item.Leather, ExtraQty: 5, Legacy: true},
	ButcherRidgewolf: {ID: ButcherRidgewolf, Name: "butcher_ridgewolf", Label: "Butcher ridgewolf", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseRidgewolf, Qty: 1}, Output: item.Meat, OutQty: 6, Work: butcherWork, Skill: work.Cook, Learn: skill.Cooking,
		Extra: item.Leather, ExtraQty: 4, Legacy: true},
	ButcherAnimal: {ID: ButcherAnimal, Name: "butcher_animal", Label: "Butcher animal", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseAnimal, Qty: 1}, Output: item.Meat, Work: butcherWork, Skill: work.Cook, Learn: skill.Cooking,
		Extra: item.Leather, BySpecies: true, Also: []construction.Kind{construction.ButcherSpot}},
	// wiki: Simple meal: cooking over a campfire doubles the work (600).
	CampfireMeal: {ID: CampfireMeal, Name: "simple_meal_campfire", Label: "Cook simple meal over the fire",
		Bench: construction.Campfire, Input: Input{AnyRaw: true, Qty: 10}, Output: item.Meal, OutQty: 1,
		Work: 240, Skill: work.Cook, Learn: skill.Cooking},
	// Tailoring: a parka takes the hides of about three deer.
	MakeParka: {ID: MakeParka, Name: "make_parka", Label: "Sew leather parka", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 14, Stuff: item.MadeFrom(item.Parka)}, Output: item.Parka, OutQty: 1, Work: 900, Skill: work.Craft, Learn: skill.Crafting},
	MakeTuque: {ID: MakeTuque, Name: "make_tuque", Label: "Sew leather tuque", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 4, Stuff: item.MadeFrom(item.Tuque)}, Output: item.Tuque, OutQty: 1, Work: 300, Skill: work.Craft, Learn: skill.Crafting},
	MakeDuster: {ID: MakeDuster, Name: "make_duster", Label: "Sew leather duster", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 10, Stuff: item.MadeFrom(item.Duster)}, Output: item.Duster, OutQty: 1, Work: 700, Skill: work.Craft, Learn: skill.Crafting},
	// A crude rifle: a long, careful job (needs Gunsmithing research).
	MakeRifle: {ID: MakeRifle, Name: "make_rifle", Label: "Make crude rifle", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 12}, Output: item.Rifle, OutQty: 1, Work: 1500, Skill: work.Craft, Learn: skill.Crafting,
		Also: machining},
	Knife: {ID: Knife, Name: "knife", Label: "Make stone knife", Bench: construction.Workbench,
		Input: Input{Kind: item.GraniteBlocks, Qty: 3, Stuff: item.MadeFrom(item.Knife)}, Output: item.Knife, OutQty: 1, Work: 200, Skill: work.Craft, Learn: skill.Crafting,
		Also: spotAndSmithy},
	Spear: {ID: Spear, Name: "spear", Label: "Make wooden spear", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 5, Stuff: item.MadeFrom(item.Spear)}, Output: item.Spear, OutQty: 1, Work: 320, Skill: work.Craft, Learn: skill.Crafting,
		Also: smithy},
	Cremate: {ID: Cremate, Name: "cremate", Label: "Cremate corpse", Bench: construction.Pyre,
		Input: Input{Kind: item.CorpseHuman, Qty: 1}, Work: 300, Skill: work.Craft, Learn: skill.Crafting,
		Extra: item.Wood, ExtraQty: 2},
	DryFood: {ID: DryFood, Name: "dry_food", Label: "Dry food", Bench: construction.DryingRack,
		Input: Input{AnyDryable: true, Qty: 1, Max: RackCapacity}, Work: 90, Skill: work.Cook, Learn: skill.Cooking, Dry: true},
	// Entrance carpets: the better the material, the more dirt it catches.
	MakeStrawCarpet: {ID: MakeStrawCarpet, Name: "make_straw_carpet", Label: "Weave straw carpet", Bench: construction.Workbench,
		Input: Input{Kind: item.Straw, Qty: 10}, Output: item.CarpetStraw, OutQty: 1, Work: 300, Skill: work.Craft, Learn: skill.Crafting},
	MakeWoodCarpet: {ID: MakeWoodCarpet, Name: "make_wood_carpet", Label: "Make wood slat carpet", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 6}, Output: item.CarpetWood, OutQty: 1, Work: 250, Skill: work.Craft, Learn: skill.Crafting},
	SewClothCarpet: {ID: SewClothCarpet, Name: "sew_cloth_carpet", Label: "Sew cloth carpet", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Cloth, Qty: 8}, Output: item.CarpetCloth, OutQty: 1, Work: 400, Skill: work.Craft, Learn: skill.Crafting},
	SewLeatherCarpet: {ID: SewLeatherCarpet, Name: "sew_leather_carpet", Label: "Sew leather carpet", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 6}, Output: item.CarpetLeather, OutQty: 1, Work: 400, Skill: work.Craft, Learn: skill.Crafting},
	KnitWoolParka: {ID: KnitWoolParka, Name: "knit_wool_parka", Label: "Knit wool parka", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Wool, Qty: 16}, Output: item.WoolParka, OutQty: 1, Work: 1000, Skill: work.Craft, Learn: skill.Crafting},
	WeaveWoolCarpet: {ID: WeaveWoolCarpet, Name: "weave_wool_carpet", Label: "Weave wool carpet", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Wool, Qty: 8}, Output: item.CarpetWool, OutQty: 1, Work: 450, Skill: work.Craft, Learn: skill.Crafting},
	CutSandstone: cut(CutSandstone, geology.Sandstone),
	CutGranite:   cut(CutGranite, geology.Granite),
	CutLimestone: cut(CutLimestone, geology.Limestone),
	CutSlate:     cut(CutSlate, geology.Slate),
	CutMarble:    cut(CutMarble, geology.Marble),
	FabricateComponent: {ID: FabricateComponent, Name: "fabricate_component", Label: "Fabricate component",
		Bench: construction.Workbench, Input: Input{Kind: item.Steel, Qty: 12}, Output: item.Components, OutQty: 1,
		Work: 1500, Skill: work.Craft, Learn: skill.Crafting, MinSkill: 6, Also: fabrication},
	// Refining medicine is a doctor's job, done at the workbench.
	RefineMedicine: {ID: RefineMedicine, Name: "refine_medicine", Label: "Refine industrial medicine",
		Bench: construction.Workbench, Input: Input{Kind: item.HerbalMedicine, Qty: 2}, Output: item.IndustrialMedicine,
		OutQty: 1, Work: 800, Skill: work.Doctor, Learn: skill.Medicine, MinSkill: 4},
	// ENG-003: drugs (brewing is a cook's job) and prosthetics.
	BrewBeer: {ID: BrewBeer, Name: "brew_beer", Label: "Brew beer", Bench: construction.BrewingBench,
		Input: Input{Kind: item.Brewgrain, Qty: 6}, Output: item.Beer, OutQty: 3, Work: 600, Skill: work.Cook, Learn: skill.Cooking},
	BrewTea: {ID: BrewTea, Name: "brew_tea", Label: "Brew ashleaf tea", Bench: construction.DrugTable,
		Input: Input{Kind: item.AshleafLeaf, Qty: 4}, Output: item.AshleafTea, OutQty: 2, Work: 500, Skill: work.Craft, Learn: skill.Crafting},
	PressTablet: {ID: PressTablet, Name: "press_tablet", Label: "Press spark tablet", Bench: construction.DrugTable,
		Input: Input{Kind: item.AshleafLeaf, Qty: 6}, Output: item.SparkTablet, OutQty: 1, Work: 900, Skill: work.Craft,
		Learn: skill.Crafting, MinSkill: 4},
	MakePegLeg: {ID: MakePegLeg, Name: "make_peg_leg", Label: "Carve peg leg", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 10}, Output: item.PegLeg, OutQty: 1, Work: 600, Skill: work.Craft, Learn: skill.Crafting},
	MakeProstheticLeg: {ID: MakeProstheticLeg, Name: "make_prosthetic_leg", Label: "Make prosthetic leg",
		Bench: construction.Workbench, Input: Input{Kind: item.Steel, Qty: 20}, Output: item.ProstheticLeg, OutQty: 1,
		Work: 1400, Skill: work.Craft, Learn: skill.Crafting, MinSkill: 5, Also: machining},
	MakeProstheticArm: {ID: MakeProstheticArm, Name: "make_prosthetic_arm", Label: "Make prosthetic arm",
		Bench: construction.Workbench, Input: Input{Kind: item.Steel, Qty: 20}, Output: item.ProstheticArm, OutQty: 1,
		Work: 1400, Skill: work.Craft, Learn: skill.Crafting, MinSkill: 5, Also: machining},
	MakeWoodenHand: {ID: MakeWoodenHand, Name: "make_wooden_hand", Label: "Carve wooden hand", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 8}, Output: item.WoodenHand, OutQty: 1, Work: 500, Skill: work.Craft, Learn: skill.Crafting},
	MakeDenture: {ID: MakeDenture, Name: "make_denture", Label: "Carve denture", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 4}, Output: item.Denture, OutQty: 1, Work: 400, Skill: work.Craft, Learn: skill.Crafting},
	MakeHearingTrumpet: {ID: MakeHearingTrumpet, Name: "make_hearing_trumpet", Label: "Make hearing trumpet",
		Bench: construction.Workbench, Input: Input{Kind: item.Steel, Qty: 6}, Output: item.HearingTrumpet, OutQty: 1,
		Work: 500, Skill: work.Craft, Learn: skill.Crafting},
	MakeProstheticFoot: {ID: MakeProstheticFoot, Name: "make_prosthetic_foot", Label: "Carve prosthetic foot", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 6}, Output: item.ProstheticFoot, OutQty: 1, Work: 450, Skill: work.Craft, Learn: skill.Crafting},
	MakeGlassEye: {ID: MakeGlassEye, Name: "make_glass_eye", Label: "Make glass eye", Bench: construction.Workbench,
		Input: Input{Kind: item.Steel, Qty: 2}, Output: item.GlassEye, OutQty: 1, Work: 400, Skill: work.Craft, Learn: skill.Crafting,
		MinSkill: 4},
	MakeWoodenNose: {ID: MakeWoodenNose, Name: "make_wooden_nose", Label: "Carve wooden nose", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 3}, Output: item.WoodenNose, OutQty: 1, Work: 300, Skill: work.Craft, Learn: skill.Crafting},
	// MED-05 (original tuning: a recipe takes one input, so components,
	// our top-tier material, stand in for plasteel and advanced
	// components). Crafting 8 for the servo tier, 5 for the valve heart.
	MakeServoArm:        servo(MakeServoArm, "make_servo_arm", "Make servo arm", item.ServoArm, 8),
	MakeServoLeg:        servo(MakeServoLeg, "make_servo_leg", "Make servo leg", item.ServoLeg, 8),
	MakeOpticImplant:    servo(MakeOpticImplant, "make_optic_implant", "Make optic implant", item.OpticImplant, 8),
	MakeCochlearImplant: servo(MakeCochlearImplant, "make_cochlear_implant", "Make cochlear implant", item.CochlearImplant, 6),
	MakeSpinalBrace:     servo(MakeSpinalBrace, "make_spinal_brace", "Make spinal brace", item.SpinalBrace, 8),
	MakePumpHeart:       servo(MakePumpHeart, "make_pump_heart", "Make pump heart", item.PumpHeart, 8),
	MakeSynthStomach:    servo(MakeSynthStomach, "make_synth_stomach", "Make synth stomach", item.SynthStomach, 6),
	MakeValveHeart: {ID: MakeValveHeart, Name: "make_valve_heart", Label: "Make valve heart", Bench: construction.Workbench,
		Input: Input{Kind: item.Steel, Qty: 25}, Output: item.ValveHeart, OutQty: 1, Work: 1400, Skill: work.Craft,
		Learn: skill.Crafting, MinSkill: 5},
	// MED-07 (original tuning; a recipe takes one input): numbroot
	// tincture steeped from herbal medicine, prophyl tablets pressed from
	// industrial medicine (two per unit), both at the drug table.
	MakeNumbroot: {ID: MakeNumbroot, Name: "make_numbroot", Label: "Steep numbroot tincture", Bench: construction.DrugTable,
		Input: Input{Kind: item.HerbalMedicine, Qty: 1}, Output: item.NumbrootTincture, OutQty: 1, Work: 450, Skill: work.Craft,
		Learn: skill.Crafting},
	PressProphyl: {ID: PressProphyl, Name: "press_prophyl", Label: "Press prophyl tablets", Bench: construction.DrugTable,
		Input: Input{Kind: item.IndustrialMedicine, Qty: 1}, Output: item.ProphylTablet, OutQty: 2, Work: 700, Skill: work.Craft,
		Learn: skill.Crafting, MinSkill: 4},
	MakeMace:               smith(MakeMace, "make_mace", "Forge mace", item.Mace, 5, 600),
	MakeGladius:            smith(MakeGladius, "make_gladius", "Forge gladius", item.Gladius, 5, 1200),
	MakeLongsword:          smith(MakeLongsword, "make_longsword", "Forge longsword", item.Longsword, 10, 1800),
	MakePila:               wooden(MakePila, "make_pila", "Make pila", item.Pila, construction.FueledSmithy, 7, 700, 3),
	MakeShortBow:           wooden(MakeShortBow, "make_short_bow", "Make short bow", item.ShortBow, construction.CraftingSpot, 3, 240, 2),
	MakeRecurveBow:         wooden(MakeRecurveBow, "make_recurve_bow", "Make recurve bow", item.RecurveBow, construction.CraftingSpot, 4, 500, 5),
	MakeGreatbow:           wooden(MakeGreatbow, "make_greatbow", "Make greatbow", item.Greatbow, construction.CraftingSpot, 6, 900, 6),
	MakeRevolver:           gun(MakeRevolver, "make_revolver", "Machine revolver", item.Revolver, 3, 2, 400, 0),
	MakeAutopistol:         gun(MakeAutopistol, "make_autopistol", "Machine autopistol", item.Autopistol, 3, 2, 500, 4),
	MakeMachinePistol:      gun(MakeMachinePistol, "make_machine_pistol", "Machine machine pistol", item.MachinePistol, 5, 3, 1100, 4),
	MakePumpShotgun:        gun(MakePumpShotgun, "make_pump_shotgun", "Machine pump shotgun", item.PumpShotgun, 6, 3, 1200, 5),
	MakeChainShotgun:       gun(MakeChainShotgun, "make_chain_shotgun", "Machine chain shotgun", item.ChainShotgun, 7, 5, 3100, 6),
	MakeHeavySMG:           gun(MakeHeavySMG, "make_heavy_smg", "Machine heavy SMG", item.HeavySMG, 8, 4, 2400, 0),
	MakeLMG:                gun(MakeLMG, "make_lmg", "Machine LMG", item.LMG, 8, 5, 3400, 6),
	MakeAssaultRifle:       gun(MakeAssaultRifle, "make_assault_rifle", "Machine assault rifle", item.AssaultRifle, 6, 7, 4000, 6),
	MakeSniperRifle:        gun(MakeSniperRifle, "make_sniper_rifle", "Machine sniper rifle", item.SniperRifle, 6, 8, 4500, 7),
	MakeMinigun:            gun(MakeMinigun, "make_minigun", "Machine minigun", item.Minigun, 16, 20, 6000, 7),
	MakeIncendiaryLauncher: gun(MakeIncendiaryLauncher, "make_incendiary_launcher", "Machine incendiary launcher", item.IncendiaryLauncher, 8, 4, 2000, 0),
	MakeEMPLauncher:        gun(MakeEMPLauncher, "make_emp_launcher", "Machine EMP launcher", item.EMPLauncher, 8, 8, 3000, 0),
	MakeEMPGrenades:        gun(MakeEMPGrenades, "make_emp_grenades", "Machine EMP grenades", item.EMPGrenades, 2, 6, 2400, 0),
	// wiki: Electric smelter (1600 ticks, a quarter of the materials back).
	SmeltWeapon: {ID: SmeltWeapon, Name: "smelt_weapon", Label: "Smelt metal weapons", Bench: construction.ElectricSmelter,
		Input: Input{Smelt: true, Qty: 1}, Output: item.Steel, Work: 160, Skill: work.Craft, Learn: skill.Crafting},
	SculptSmall: sculpt(SculptSmall, "sculpt_small", "Sculpt small sculpture", construction.SmallSculpture),
	SculptLarge: sculpt(SculptLarge, "sculpt_large", "Sculpt large sculpture", construction.LargeSculpture),
	SculptGrand: sculpt(SculptGrand, "sculpt_grand", "Sculpt grand sculpture", construction.GrandSculpture),
	// PRD-4 (wave_e.go).
	CookFineMeal:         meal(CookFineMeal, "cook_fine_meal", "Cook fine meal", item.FineMeal, 5, 5, 1, 450, 6),
	CookLavishMeal:       meal(CookLavishMeal, "cook_lavish_meal", "Cook lavish meal", item.LavishMeal, 10, 10, 1, 800, 8),
	CookPemmican:         meal(CookPemmican, "cook_pemmican", "Make pemmican", item.Pemmican, 5, 5, 16, 700, 0),
	CookSurvivalMeal:     meal(CookSurvivalMeal, "cook_survival_meal", "Cook packaged survival meal", item.SurvivalMeal, 6, 6, 1, 450, 8),
	MakeKibble:           kibble(MakeKibble, "make_kibble", "Make kibble", construction.ButcherTable, 50),
	MakeKibbleSpot:       kibble(MakeKibbleSpot, "make_kibble_spot", "Make kibble at the spot", construction.ButcherSpot, 35),
	BrewWort:             brewWort(),
	FillBarrel:           fillBarrel(),
	MakeAshFlake:         drug(MakeAshFlake, "make_ash_flake", "Make ash flake", item.AshFlake, 250, 0, Input{Kind: item.AshleafLeaf, Qty: 4}),
	MakeSilverdust:       drug(MakeSilverdust, "make_silverdust", "Make silverdust", item.Silverdust, 350, 0, Input{Kind: item.AshleafLeaf, Qty: 8}),
	MakeBattleTonic:      drug(MakeBattleTonic, "make_battle_tonic", "Make battle tonic", item.BattleTonic, 600, 0, Input{Kind: item.Reagent, Qty: 2}, Input{Kind: item.Silverdust, Qty: 1}),
	MakeReagent:          drug(MakeReagent, "make_reagent", "Distil reagent", item.Reagent, 700, 4, Input{Kind: item.Chemfuel, Qty: 4}, Input{Kind: item.AshleafLeaf, Qty: 2}),
	RefineChemfuelWood:   refine(RefineChemfuelWood, "refine_chemfuel_wood", "Make chemfuel from wood", Input{Kind: item.Wood, Qty: 70}, 200),
	RefineChemfuelFood:   refine(RefineChemfuelFood, "refine_chemfuel_food", "Make chemfuel from raw food", Input{AnyRaw: true, Qty: 70}, 250),
	FabricateAdvanced:    made(FabricateAdvanced, "fabricate_advanced", "Fabricate advanced component", construction.FabricationBench, item.AdvancedComponents, 1000, 8, in(item.Components, 1), in(item.Steel, 2), in(item.Plasteel, 1), in(item.Gold, 1)),
	SmeltApparel:         smeltWear(),
	MakeFlakVest:         made(MakeFlakVest, "make_flak_vest", "Make flak vest", construction.MachiningTable, item.FlakVest, 900, 0, in(item.Steel, 6), in(item.Cloth, 3), in(item.Components, 1)),
	MakeFlakPants:        made(MakeFlakPants, "make_flak_pants", "Make flak pants", construction.MachiningTable, item.FlakPants, 900, 0, in(item.Steel, 6), in(item.Cloth, 3), in(item.Components, 1)),
	MakeFlakJacket:       made(MakeFlakJacket, "make_flak_jacket", "Make flak jacket", construction.MachiningTable, item.FlakJacket, 1400, 0, in(item.Steel, 7), in(item.Cloth, 5), in(item.Components, 1)),
	MakePlateArmor:       smith(MakePlateArmor, "make_plate_armor", "Forge plate armor", item.PlateArmor, 17, 3800),
	MakeSimpleHelmet:     smith(MakeSimpleHelmet, "make_simple_helmet", "Forge simple helmet", item.SimpleHelmet, 4, 320),
	MakeAdvancedHelmet:   made(MakeAdvancedHelmet, "make_advanced_helmet", "Make advanced helmet", construction.MachiningTable, item.AdvancedHelmet, 800, 0, in(item.Steel, 4), in(item.Components, 2), in(item.Plasteel, 1)),
	MakeMarineArmor:      made(MakeMarineArmor, "make_marine_armor", "Fabricate marine armor", construction.FabricationBench, item.MarineArmor, 6000, 8, in(item.Plasteel, 10), in(item.Uranium, 2), in(item.AdvancedComponents, 4)),
	MakeMarineHelmet:     made(MakeMarineHelmet, "make_marine_helmet", "Fabricate marine helmet", construction.FabricationBench, item.MarineHelmet, 2100, 8, in(item.Plasteel, 4), in(item.AdvancedComponents, 1)),
	MakeFragGrenades:     made(MakeFragGrenades, "make_frag_grenades", "Make frag grenades", construction.MachiningTable, item.FragGrenades, 1200, 0, in(item.Steel, 2), in(item.Chemfuel, 8)),
	MakeMolotovs:         made(MakeMolotovs, "make_molotovs", "Make molotov cocktails", construction.MachiningTable, item.MolotovCocktails, 600, 0, in(item.Cloth, 3), in(item.Chemfuel, 8)),
	MakeHEShells:         made(MakeHEShells, "make_he_shells", "Make high-explosive shell", construction.MachiningTable, item.MortarShells, 80, 0, in(item.Steel, 2), in(item.Chemfuel, 2)),
	MakeIncendiaryShells: made(MakeIncendiaryShells, "make_incendiary_shells", "Make incendiary shell", construction.MachiningTable, item.IncendiaryShells, 80, 0, in(item.Steel, 1), in(item.Chemfuel, 2)),
	MakeEMPShells:        made(MakeEMPShells, "make_emp_shells", "Make EMP shell", construction.MachiningTable, item.EMPShells, 80, 0, in(item.Steel, 2), in(item.Components, 2)),
	ShredMech:            shredMech(),
	MakeSmokeShells:      made(MakeSmokeShells, "make_smoke_shells", "Make smoke shell", construction.MachiningTable, item.SmokeShells, 80, 0, in(item.Steel, 3)),
	// FOOD-3: a human corpse into human meat (wiki: Butcher table; the
	// yield is the body's, like an animal's).
	ButcherHuman: {ID: ButcherHuman, Name: "butcher_human", Label: "Butcher human", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseHuman, Qty: 1}, Output: item.HumanMeat, Work: butcherWork, Skill: work.Cook, Learn: skill.Cooking,
		BySpecies: true, Also: []construction.Kind{construction.ButcherSpot}},
	// FOOD-4 (food4.go).
	CookFineMealVeg:      oneSided(CookFineMealVeg, "cook_fine_meal_veg", "Cook fine meal (vegetarian)", item.FineMeal, Input{AnyVeg: true, Qty: 15}, 450, 6),
	CookFineMealMeat:     oneSided(CookFineMealMeat, "cook_fine_meal_meat", "Cook fine meal (carnivore)", item.FineMeal, Input{AnyMeat: true, Qty: 15}, 450, 6),
	CookLavishMealVeg:    oneSided(CookLavishMealVeg, "cook_lavish_meal_veg", "Cook lavish meal (vegetarian)", item.LavishMeal, Input{AnyVeg: true, Qty: 25}, 800, 8),
	CookLavishMealMeat:   oneSided(CookLavishMealMeat, "cook_lavish_meal_meat", "Cook lavish meal (carnivore)", item.LavishMeal, Input{AnyMeat: true, Qty: 25}, 800, 8),
	CookSimpleMealBulk:   bulkSimple(),
	CookFineMealBulk:     bulk(CookFineMealBulk, "cook_fine_meal_bulk", "Cook 4 fine meals", item.FineMeal, 5, bulkFineWork, 6),
	CookLavishMealBulk:   bulk(CookLavishMealBulk, "cook_lavish_meal_bulk", "Cook 4 lavish meals", item.LavishMeal, 10, bulkLavishWork, 8),
	CookSurvivalMealBulk: bulk(CookSurvivalMealBulk, "cook_survival_meal_bulk", "Cook 4 packaged survival meals", item.SurvivalMeal, 6, bulkSurvivalWork, 8),
	CookPemmicanBulk:     meal(CookPemmicanBulk, "cook_pemmican_bulk", "Make pemmican x4", item.Pemmican, 20, 20, 64, 2800, 0), // wiki: Pemmican
}

// servoSkill and servoWork are the servo tier's crafting level and work
// (original tuning).
const (
	servoSkill = 8
	servoWork  = 2600
)

// servo is a servo-tier recipe at the workbench: comps components make
// one out.
func servo(id ID, name, label string, out item.Kind, comps int) Def {
	return Def{ID: id, Name: name, Label: label, Bench: construction.Workbench, Input: Input{Kind: item.Components, Qty: comps},
		Output: out, OutQty: 1, Work: servoWork, Skill: work.Craft, Learn: skill.Crafting, MinSkill: servoSkill, Also: fabrication}
}

// CutYield is how many blocks one chunk makes at the stonecutter table.
const CutYield = 10

func cut(id ID, s geology.Stone) Def {
	return Def{ID: id, Name: "cut_" + s.String(), Label: "Cut " + s.String() + " blocks",
		Bench: construction.StonecutterTable, Input: Input{Kind: item.ChunkOf(s), Qty: 1},
		Output: item.BlocksOf(s), OutQty: CutYield, Work: 300, Skill: work.Craft, Learn: skill.Crafting}
}

// RackCapacity is how many items one drying rack holds.
const RackCapacity = 25

// DefOf returns the recipe definition.
func DefOf(id ID) Def {
	if id >= numRecipes {
		return Def{}
	}
	return defs[id]
}

// All lists every recipe.
func All() []Def { return defs[:] }

// Upgrade maps a legacy recipe to the one that replaced it.
func Upgrade(id ID) ID {
	if id == ButcherDustbuck || id == ButcherRidgewolf {
		return ButcherAnimal
	}
	return id
}

// For lists recipes a bench kind can run (legacy ones excluded; At).
func For(bench construction.Kind) []Def {
	var out []Def
	for _, d := range defs {
		if d.At(bench) && !d.Legacy {
			out = append(out, d)
		}
	}
	return out
}

// Accepts reports whether stack kind k can feed input in.
func (in Input) Accepts(k item.Kind) bool {
	switch {
	case in.Stuff != 0:
		return k.StuffCategory()&in.Stuff != 0
	case in.Smelt, in.SmeltWear:
		return smeltable(k, in.SmeltWear)
	case in.AnyVeg:
		return plantFood(k) || in.Hay && k == item.Hay
	case in.AnyMeat:
		return animalFood(k)
	case in.AnyRaw:
		return k.Raw()
	case in.AnyDryable:
		return k.Dryable()
	case in.AnyStoneBlocks:
		return item.IsStoneBlocks(k)
	case in.Hops:
		return k == item.Hops || k == item.Brewgrain
	}
	return k == in.Kind
}

// AcceptsStack is Accepts plus per-stack state: a dry recipe skips stacks
// already dried or spoiled.
func (in Input) AcceptsStack(s item.Stack) bool {
	if in.AnyDryable {
		return s.Dryable()
	}
	if in.Smelt || in.SmeltWear {
		return in.Accepts(s.Kind) && SmeltYield(s) > 0
	}
	if s.Kind == item.CorpseAnimal && animal.IsMech(animal.Species(s.Animal)) != in.Wreck {
		return false // a wreck is shredded, never butchered (DEF-4)
	}
	return in.Accepts(s.Kind)
}

// AppendInputs appends every ingredient d takes to dst: Inputs when set,
// else Input alone (so a caller's stack buffer serves both).
func (d Def) AppendInputs(dst []Input) []Input {
	if len(d.Inputs) > 0 {
		return append(dst, d.Inputs...)
	}
	return append(dst, d.Input)
}

// Accepts reports whether any of d's ingredients takes kind k.
func (d Def) Accepts(k item.Kind) bool {
	if len(d.Inputs) == 0 {
		return d.Input.Accepts(k)
	}
	for _, in := range d.Inputs {
		if in.Accepts(k) {
			return true
		}
	}
	return false
}

// InputKinds lists every item kind one of d's ingredients takes, in kind
// order: the bill's ingredient allow list.
func (d Def) InputKinds() []item.Kind {
	var out []item.Kind
	for k := item.Kind(0); k.String() != "unknown"; k++ {
		if d.Accepts(k) {
			out = append(out, k)
		}
	}
	return out
}

// Label is the input as the bills UI shows it, e.g. "2 raw food".
func (in Input) Label() string {
	name := in.Kind.String()
	switch {
	case in.AnyRaw:
		name = "raw food"
	case in.Stuff != 0:
		name = in.Stuff.String()
	case in.AnyStoneBlocks:
		name = "any stone blocks"
	case in.Smelt:
		name = "metal weapon"
	case in.SmeltWear:
		name = "metal apparel"
	case in.AnyVeg && in.Hay:
		name = "plant food or hay"
	case in.AnyVeg:
		name = "plant food"
	case in.AnyMeat:
		name = "meat or animal product"
	case in.Wreck:
		name = "mechanoid wreck"
	}
	if in.ByNutrition() { // FOOD-4: food counts nutrition, e.g. "0.5 nutrition of raw food"
		return strconv.FormatFloat(float64(in.Qty*NutritionUnit)/10000, 'f', -1, 64) + " nutrition of " + name
	}
	return strconv.Itoa(in.Qty) + " " + name
}

// InputsLabel is every ingredient's Label joined with " + ".
func (d Def) InputsLabel() string {
	if len(d.Inputs) == 0 {
		return d.Input.Label()
	}
	s := ""
	for i, in := range d.Inputs {
		if i > 0 {
			s += " + "
		}
		s += in.Label()
	}
	return s
}

// Mode is how many times a bill repeats.
type Mode uint8

const (
	DoCount   Mode = iota // do Target times, then stop
	Forever               // repeat forever
	UntilHave             // repeat while the colony has fewer than Target outputs
)

func (m Mode) String() string {
	switch m {
	case Forever:
		return "forever"
	case UntilHave:
		return "until you have"
	default:
		return "do"
	}
}

// Bill is a standing work order on a bench.
type Bill struct {
	Recipe    ID
	Mode      Mode
	Target    int
	Done      int
	Suspended bool
	// Waiting: an "until you have" bill that reached its target and
	// rests until the stock falls to ResumeAt (absent in older saves).
	Waiting bool
	// PRD-1 settings (append only; zero values are the defaults, so older
	// saves load unchanged). Store is where the product goes and Zone the
	// stockpile for StockpileZone. Radius limits the ingredient search to
	// that many tiles from the bench (0: anywhere). SkillMin..SkillMax is
	// the worker's allowed level in the recipe's skill (SkillMax 0: no
	// cap). Worker: only this colonist (0: anyone). UnpauseAt: the "until
	// you have" resume level (0, or not below Target: the default).
	// Forbid lists the ingredient kinds the bill refuses, sorted.
	Store              StoreMode
	Zone               uint64
	Radius             int
	SkillMin, SkillMax uint8
	Worker             entity.EntityID
	UnpauseAt          int
	Forbid             []item.Kind
	// SearchAt is the tick before which the bill doesn't look for
	// ingredients again after a search found none (the retry cooldown).
	SearchAt uint64
	// Stuff (PRD-2) pins a stuffed recipe's material: only that kind is
	// taken (0: any material the recipe takes and Forbid allows). The
	// allowed-stuff list is Forbid over the recipe's materials.
	Stuff item.Kind
	// Source (RES-1, "take ingredients from stockpile") is the storage
	// zone or storage group the bill takes its ingredients from, besides
	// the radius (0: anywhere).
	Source uint64
}

// StoreMode is where a finished bill's product goes.
type StoreMode uint8

const (
	BestStockpile StoreMode = iota // the crafter carries it to the best stockpile
	DropOnFloor                    // left at the bench for haulers
	StockpileZone                  // the crafter carries it to stockpile Bill.Zone
	numStoreModes
)

// Valid reports a known store mode.
func (m StoreMode) Valid() bool { return m < numStoreModes }

func (m StoreMode) String() string {
	switch m {
	case DropOnFloor:
		return "drop on floor"
	case StockpileZone:
		return "specific stockpile"
	default:
		return "best stockpile"
	}
}

// ResumeAt is the stock an "until you have" bill waits for after reaching
// its target, so one meal eaten doesn't send a cook back for one meal:
// UnpauseAt when the player set one below the target, else a fifth below.
func (b Bill) ResumeAt() int {
	if b.UnpauseAt > 0 && b.UnpauseAt < b.Target {
		return b.UnpauseAt
	}
	return b.Target - max(1, b.Target/5)
}

// MaxSkill is the bill's effective skill cap (SkillMax, or the top level
// when unset).
func (b Bill) MaxSkill() uint8 {
	if b.SkillMax == 0 {
		return skill.MaxLevel
	}
	return b.SkillMax
}

// SkillOK reports a worker of skill level lvl inside the bill's range.
func (b Bill) SkillOK(lvl uint8) bool { return lvl >= b.SkillMin && lvl <= b.MaxSkill() }

// Allows reports an ingredient kind the bill doesn't forbid.
func (b Bill) Allows(k item.Kind) bool {
	if b.Stuff != 0 && k != b.Stuff && k.StuffCategory() != 0 {
		return false // PRD-2: another material than the pinned one
	}
	_, found := slices.BinarySearch(b.Forbid, k)
	return !found
}

// Stuffed reports a recipe whose product is made of a chosen material
// (PRD-2: its product's Stack.Stuff is the material it took).
func (d Def) Stuffed() bool { return d.Input.Stuff != 0 && len(d.Inputs) == 0 }

// Materials lists the materials a stuffed recipe takes, in kind order
// (nil for any other recipe).
func (d Def) Materials() []item.Kind {
	if !d.Stuffed() {
		return nil
	}
	return item.Stuffs(d.Input.Stuff)
}

// PinStuff is b with its material pinned to k when recipe d takes it,
// else unpinned (k 0, or a kind d does not take).
func (b Bill) PinStuff(d Def, k item.Kind) Bill {
	b.Stuff = 0
	if k != 0 && d.Stuffed() && d.Input.Accepts(k) {
		b.Stuff = k
	}
	return b
}

// SetAllowed allows or forbids ingredient kind k, keeping Forbid sorted.
// It builds a new slice, never writing through one a copy may share.
func (b Bill) SetAllowed(k item.Kind, allow bool) Bill {
	i, found := slices.BinarySearch(b.Forbid, k)
	switch {
	case allow && found:
		out := make([]item.Kind, 0, len(b.Forbid)-1)
		b.Forbid = append(append(out, b.Forbid[:i]...), b.Forbid[i+1:]...)
		if len(b.Forbid) == 0 {
			b.Forbid = nil
		}
	case !allow && !found:
		out := make([]item.Kind, 0, len(b.Forbid)+1)
		b.Forbid = append(append(append(out, b.Forbid[:i]...), k), b.Forbid[i:]...)
	}
	return b
}

// Update latches the "until you have" wait: it starts at the target and
// ends once the stock falls to ResumeAt.
func (b Bill) Update(stock int) Bill {
	if b.Mode != UntilHave {
		b.Waiting = false
		return b
	}
	switch {
	case !b.Waiting && stock >= b.Target:
		b.Waiting = true
	case b.Waiting && stock <= b.ResumeAt():
		b.Waiting = false
	}
	return b
}

// Wants reports whether the bill should run now, given the colony's
// current stock of the output.
func (b Bill) Wants(stock int) bool {
	if b.Suspended {
		return false
	}
	switch b.Mode {
	case Forever:
		return true
	case UntilHave:
		return !b.Waiting && stock < b.Target
	default:
		return b.Done < b.Target
	}
}
