package item

// Stuff (PRD-2): the material a stuffed item is made of, and what that
// material does to the item's stats. A stuffed product keeps its
// material in Stack.Stuff (a steel club, a cloth parka); a recipe names
// the stuff categories it takes. Numbers are the published wiki values
// (ENG-003), scaled to permille so callers stay in integers.

// StuffCategory is a set of material families (a bit each).
type StuffCategory uint8

const (
	Woody StuffCategory = 1 << iota
	Metallic
	Stony
	Fabric
	Leathery
)

// Has reports whether c includes every family in o.
func (c StuffCategory) Has(o StuffCategory) bool { return o != 0 && c&o == o }

// StuffProps is one material's stat factors. Every factor is permille
// (1000 = unchanged); Value is the market value per unit in thousandths
// of a silver; the insulations are tenths of a degree C added by the
// material (apparel only).
type StuffProps struct {
	Value          int
	MaxHP          int
	BeautyFactor   int
	BeautyOffset   int // whole beauty points
	SharpDamage    int // melee sharp damage (0: the material can't hold an edge)
	BluntDamage    int
	ArmorSharp     int
	ArmorBlunt     int
	ArmorHeat      int
	ColdInsulation int
	HeatInsulation int
	WorkToMake     int
	Flammability   int
	// Immune: the material never deteriorates (metals and stone).
	Immune bool
}

// stuffs is the material table. wiki: Materials (market value, max HP,
// beauty, melee sharp/blunt damage, armor sharp/blunt/heat, insulation,
// work to make, flammability); wiki: Deterioration (metals and stone
// are immune). The wiki leaves stone's armor and insulation blank: 0.
var stuffs = [numKinds]StuffProps{
	Wood: {Value: 1200, MaxHP: 650, BeautyFactor: 1000, SharpDamage: 400, BluntDamage: 900,
		ArmorSharp: 540, ArmorBlunt: 540, ArmorHeat: 400, ColdInsulation: 80, HeatInsulation: 40, WorkToMake: 700, Flammability: 1000},
	Steel: {Value: 1900, MaxHP: 1000, BeautyFactor: 1000, SharpDamage: 1000, BluntDamage: 1000,
		ArmorSharp: 900, ArmorBlunt: 450, ArmorHeat: 600, ColdInsulation: 30, WorkToMake: 1000, Flammability: 400, Immune: true},
	Silver: {Value: 1000, MaxHP: 700, BeautyFactor: 2000, BeautyOffset: 6, SharpDamage: 850, BluntDamage: 1000,
		ArmorSharp: 720, ArmorBlunt: 360, ArmorHeat: 360, ColdInsulation: 30, WorkToMake: 1000, Flammability: 400, Immune: true},
	Gold: {Value: 10000, MaxHP: 600, BeautyFactor: 4000, BeautyOffset: 20, SharpDamage: 750, BluntDamage: 1000,
		ArmorSharp: 720, ArmorBlunt: 360, ArmorHeat: 360, ColdInsulation: 30, WorkToMake: 900, Flammability: 400, Immune: true},
	GraniteBlocks:   {Value: 900, MaxHP: 1700, BeautyFactor: 1000, SharpDamage: 650, BluntDamage: 1000, WorkToMake: 1300, Immune: true},
	SandstoneBlocks: {Value: 900, MaxHP: 1400, BeautyFactor: 1100, SharpDamage: 500, BluntDamage: 1000, WorkToMake: 1100, Immune: true},
	LimestoneBlocks: {Value: 900, MaxHP: 1550, BeautyFactor: 1000, SharpDamage: 600, BluntDamage: 1000, WorkToMake: 1300, Immune: true},
	SlateBlocks:     {Value: 900, MaxHP: 1300, BeautyFactor: 1100, SharpDamage: 600, BluntDamage: 1000, WorkToMake: 1300, Immune: true},
	MarbleBlocks: {Value: 900, MaxHP: 1200, BeautyFactor: 1350, BeautyOffset: 1, SharpDamage: 600, BluntDamage: 1000,
		WorkToMake: 1150, Immune: true},
	Cloth: {Value: 1500, MaxHP: 1000, BeautyFactor: 1000, ArmorSharp: 360, ArmorHeat: 180,
		ColdInsulation: 180, HeatInsulation: 180, WorkToMake: 1000, Flammability: 1200},
	// Plainleather stands for every leather (one Leather kind).
	Leather: {Value: 2100, MaxHP: 1300, BeautyFactor: 1000, ArmorSharp: 810, ArmorBlunt: 240, ArmorHeat: 1500,
		ColdInsulation: 160, HeatInsulation: 160, WorkToMake: 1000, Flammability: 1000},
	// Sheep wool stands for every wool.
	Wool: {Value: 2700, MaxHP: 1000, BeautyFactor: 1500, ArmorSharp: 360, ArmorHeat: 1100,
		ColdInsulation: 260, HeatInsulation: 100, WorkToMake: 1000, Flammability: 1700},
}

// stuffCats is each material's family.
var stuffCats = [numKinds]StuffCategory{
	Wood: Woody, Steel: Metallic, Silver: Metallic, Gold: Metallic,
	GraniteBlocks: Stony, SandstoneBlocks: Stony, LimestoneBlocks: Stony, SlateBlocks: Stony, MarbleBlocks: Stony,
	Cloth: Fabric, Wool: Fabric, Leather: Leathery,
}

// StuffCategory is material k's family (0: k is no material).
func (k Kind) StuffCategory() StuffCategory {
	if k >= numKinds {
		return 0 // the legacy stone kind is no material any more
	}
	return stuffCats[k]
}

// StuffOf is material k's factors (ok false when k is no material; the
// legacy stone kind reads as granite).
func StuffOf(k Kind) (StuffProps, bool) {
	if k == Stone {
		k = GraniteBlocks
	}
	if k >= numKinds || stuffCats[k] == 0 {
		return StuffProps{}, false
	}
	return stuffs[k], true
}

// Stuffs lists every material in a family set, in kind order.
func Stuffs(c StuffCategory) []Kind {
	var out []Kind
	for k := Kind(0); k < numKinds; k++ {
		if stuffCats[k]&c != 0 {
			out = append(out, k)
		}
	}
	return out
}

// madeOf is the families a stuffed product takes and the material it is
// made of when nothing else is said (older stacks, found items). wiki:
// Club (metallic, woody, stony), Knife, Spear, Parka, Tuque, Duster
// (leathery, fabric).
var madeOf = [numKinds]struct {
	cats StuffCategory
	def  Kind
}{
	Club:   {Woody | Metallic | Stony, Wood},
	Knife:  {Woody | Metallic | Stony, GraniteBlocks},
	Spear:  {Woody | Metallic | Stony, Wood},
	Parka:  {Leathery | Fabric, Leather},
	Tuque:  {Leathery | Fabric, Leather},
	Duster: {Leathery | Fabric, Leather},
	// DEF-2 (wiki: Mace, Gladius, Longsword, Plate armor, Simple helmet,
	// Advanced helmet: metal or wood; helmets metal).
	Mace:           {Woody | Metallic, Steel},
	Gladius:        {Woody | Metallic, Steel},
	Longsword:      {Woody | Metallic, Steel},
	PlateArmor:     {Metallic | Woody, Steel},
	SimpleHelmet:   {Metallic, Steel},
	AdvancedHelmet: {Metallic, Steel},
}

// MadeFrom is the material families stuffed product k takes (0: k is
// not made from a chosen material).
func MadeFrom(k Kind) StuffCategory {
	if k >= numKinds {
		return 0
	}
	return madeOf[k].cats
}

// DefaultStuff is the material product k is made of when the stack names
// none (0 for a product that takes no material).
func DefaultStuff(k Kind) Kind {
	if k >= numKinds {
		return 0
	}
	return madeOf[k].def
}

// StuffOfStack is the material s is made of: its own, else its kind's
// default (0 for an unstuffed item).
func (s Stack) StuffOfStack() Kind {
	if s.Stuff != 0 && MadeFrom(s.Kind) != 0 {
		return s.Stuff
	}
	return DefaultStuff(s.Kind)
}

// StuffFactor is material m's permille factor on stat f relative to
// product k's default material: a steel club against the wooden one.
// 1000 when either has no table row or the default's factor is 0.
func StuffFactor(k, m Kind, f func(StuffProps) int) int {
	p, ok := StuffOf(m)
	d, dok := StuffOf(DefaultStuff(k))
	if !ok || !dok || f(d) == 0 {
		return 1000
	}
	return f(p) * 1000 / f(d)
}

// String names a family set as a recipe lists it ("wood, metal or stone").
func (c StuffCategory) String() string {
	names := [...]string{"wood", "metal", "stone blocks", "fabric", "leather"}
	var parts []string
	for i, n := range names {
		if c&(1<<i) != 0 {
			parts = append(parts, n)
		}
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	out := parts[0]
	for _, p := range parts[1 : len(parts)-1] {
		out += ", " + p
	}
	return out + " or " + parts[len(parts)-1]
}
