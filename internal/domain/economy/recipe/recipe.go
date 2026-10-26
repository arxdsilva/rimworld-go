package recipe

import (
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
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
	ID     ID
	Name   string
	Label  string
	Bench  construction.Kind
	Input  Input
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
}

var defs = [numRecipes]Def{
	SimpleMeal: {ID: SimpleMeal, Name: "simple_meal", Label: "Cook simple meal", Bench: construction.Stove,
		Input: Input{AnyRaw: true, Qty: 2}, Output: item.Meal, OutQty: 1, Work: 120, Skill: work.Cook, Learn: skill.Cooking},
	Club: {ID: Club, Name: "club", Label: "Make wooden club", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 4}, Output: item.Club, OutQty: 1, Work: 240, Skill: work.Craft, Learn: skill.Crafting},
	ButcherDustbuck: {ID: ButcherDustbuck, Name: "butcher_dustbuck", Label: "Butcher dustbuck", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseDustbuck, Qty: 1}, Output: item.Meat, OutQty: 12, Work: 200, Skill: work.Cook, Learn: skill.Cooking,
		Extra: item.Leather, ExtraQty: 5, Legacy: true},
	ButcherRidgewolf: {ID: ButcherRidgewolf, Name: "butcher_ridgewolf", Label: "Butcher ridgewolf", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseRidgewolf, Qty: 1}, Output: item.Meat, OutQty: 6, Work: 200, Skill: work.Cook, Learn: skill.Cooking,
		Extra: item.Leather, ExtraQty: 4, Legacy: true},
	ButcherAnimal: {ID: ButcherAnimal, Name: "butcher_animal", Label: "Butcher animal", Bench: construction.ButcherTable,
		Input: Input{Kind: item.CorpseAnimal, Qty: 1}, Output: item.Meat, Work: 200, Skill: work.Cook, Learn: skill.Cooking,
		Extra: item.Leather, BySpecies: true},
	// Cooking over an open fire is half again as slow as at a stove.
	CampfireMeal: {ID: CampfireMeal, Name: "simple_meal_campfire", Label: "Cook simple meal over the fire",
		Bench: construction.Campfire, Input: Input{AnyRaw: true, Qty: 2}, Output: item.Meal, OutQty: 1,
		Work: 180, Skill: work.Cook, Learn: skill.Cooking},
	// Tailoring: a parka takes the hides of about three deer.
	MakeParka: {ID: MakeParka, Name: "make_parka", Label: "Sew leather parka", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 14}, Output: item.Parka, OutQty: 1, Work: 900, Skill: work.Craft, Learn: skill.Crafting},
	MakeTuque: {ID: MakeTuque, Name: "make_tuque", Label: "Sew leather tuque", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 4}, Output: item.Tuque, OutQty: 1, Work: 300, Skill: work.Craft, Learn: skill.Crafting},
	MakeDuster: {ID: MakeDuster, Name: "make_duster", Label: "Sew leather duster", Bench: construction.TailoringBench,
		Input: Input{Kind: item.Leather, Qty: 10}, Output: item.Duster, OutQty: 1, Work: 700, Skill: work.Craft, Learn: skill.Crafting},
	// A crude rifle: a long, careful job (needs Gunsmithing research).
	MakeRifle: {ID: MakeRifle, Name: "make_rifle", Label: "Make crude rifle", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 12}, Output: item.Rifle, OutQty: 1, Work: 1500, Skill: work.Craft, Learn: skill.Crafting},
	Knife: {ID: Knife, Name: "knife", Label: "Make stone knife", Bench: construction.Workbench,
		Input: Input{AnyStoneBlocks: true, Qty: 3}, Output: item.Knife, OutQty: 1, Work: 200, Skill: work.Craft, Learn: skill.Crafting},
	Spear: {ID: Spear, Name: "spear", Label: "Make wooden spear", Bench: construction.Workbench,
		Input: Input{Kind: item.Wood, Qty: 5}, Output: item.Spear, OutQty: 1, Work: 320, Skill: work.Craft, Learn: skill.Crafting},
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
		Work: 1500, Skill: work.Craft, Learn: skill.Crafting, MinSkill: 6},
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
		Work: 1400, Skill: work.Craft, Learn: skill.Crafting, MinSkill: 5},
	MakeProstheticArm: {ID: MakeProstheticArm, Name: "make_prosthetic_arm", Label: "Make prosthetic arm",
		Bench: construction.Workbench, Input: Input{Kind: item.Steel, Qty: 20}, Output: item.ProstheticArm, OutQty: 1,
		Work: 1400, Skill: work.Craft, Learn: skill.Crafting, MinSkill: 5},
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
		Output: out, OutQty: 1, Work: servoWork, Skill: work.Craft, Learn: skill.Crafting, MinSkill: servoSkill}
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

// For lists recipes a bench kind can run (legacy ones excluded).
func For(bench construction.Kind) []Def {
	var out []Def
	for _, d := range defs {
		if d.Bench == bench && !d.Legacy {
			out = append(out, d)
		}
	}
	return out
}

// Accepts reports whether stack kind k can feed input in.
func (in Input) Accepts(k item.Kind) bool {
	switch {
	case in.AnyRaw:
		return k.Raw()
	case in.AnyDryable:
		return k.Dryable()
	case in.AnyStoneBlocks:
		return item.IsStoneBlocks(k)
	}
	return k == in.Kind
}

// AcceptsStack is Accepts plus per-stack state: a dry recipe skips stacks
// already dried or spoiled.
func (in Input) AcceptsStack(s item.Stack) bool {
	if in.AnyDryable {
		return s.Dryable()
	}
	return in.Accepts(s.Kind)
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
}

// ResumeAt is the stock an "until you have" bill waits for after reaching
// its target, so one meal eaten doesn't send a cook back for one meal.
func (b Bill) ResumeAt() int {
	return b.Target - max(1, b.Target/5)
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
