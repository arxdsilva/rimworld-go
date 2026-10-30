package zone

import (
	"math/bits"
	"strings"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// Storage filters (PRD-4). A storage zone takes the item kinds in its
// Filter, one bit per kind, once the player has edited it (Custom); until
// then it goes by the older Food, Wood, Stone, Corpses and Chunks
// switches (Effective), so older saves and state hashes stay as they
// were. The filter tree groups the kinds for the settings window: a node
// is a category path ("Food/Meals") or an item kind's name. The tree's
// names and grouping are our own (the wiki publishes none).

// Filter is a set of item kinds (one bit per item.Kind). The zero value
// takes nothing; it is comparable and saved as four words.
type Filter [4]uint64

// Has reports kind k in f.
func (f Filter) Has(k item.Kind) bool { return f[k>>6]&(1<<(k&63)) != 0 }

// With is f with kind k added (on) or taken out.
func (f Filter) With(k item.Kind, on bool) Filter {
	if on {
		f[k>>6] |= 1 << (k & 63)
	} else {
		f[k>>6] &^= 1 << (k & 63)
	}
	return f
}

// Or is every kind in f or g.
func (f Filter) Or(g Filter) Filter {
	for i := range f {
		f[i] |= g[i]
	}
	return f
}

// AndNot is every kind of f not in g.
func (f Filter) AndNot(g Filter) Filter {
	for i := range f {
		f[i] &^= g[i]
	}
	return f
}

// Covers reports every kind of g is in f.
func (f Filter) Covers(g Filter) bool { return g.AndNot(f) == Filter{} }

// Any reports f and g share a kind.
func (f Filter) Any(g Filter) bool {
	for i := range f {
		if f[i]&g[i] != 0 {
			return true
		}
	}
	return false
}

// Len is how many kinds f holds.
func (f Filter) Len() int {
	n := 0
	for _, w := range f {
		n += bits.OnesCount64(w)
	}
	return n
}

// filterOf is the set of kinds.
func filterOf(ks ...item.Kind) (f Filter) {
	for _, k := range ks {
		f = f.With(k, true)
	}
	return f
}

// filterWhere is the set of known kinds keep admits.
func filterWhere(keep func(item.Kind) bool) (f Filter) {
	for k := item.Kind(0); k.String() != "unknown"; k++ {
		if keep(k) {
			f = f.With(k, true)
		}
	}
	return f
}

// category is one inner node of the filter tree: its path and the kinds
// filed directly under it. Rows are in tree order (a parent before its
// children), so the table reads as the settings window shows it.
type category struct {
	path  string
	kinds []item.Kind
}

// categories is the filter tree's inner nodes. Every storable kind sits
// under exactly one of them (TestFilterTreeFilesEveryStorableKind);
// human corpses and bones are not storable (graves and pyres take them).
func categories() []category {
	return []category{
		{path: "Food"},
		{"Food/Meals", []item.Kind{item.Meal, item.FineMeal, item.LavishMeal, item.SurvivalMeal, item.Pemmican, item.Food,
			item.NutrientPasteMeal}},
		{"Food/Plant food", []item.Kind{item.Berries, item.Potatoes, item.Corn, item.Wheat, item.Beans, item.Lettuce,
			item.Radishes, item.Pumpkins, item.Brewgrain}},
		{"Food/Animal products", []item.Kind{item.Meat, item.Milk, item.Egg, item.FertileEgg}},
		{"Food/Animal feed", []item.Kind{item.Kibble}},
		{path: "Materials"},
		{"Materials/Wood", []item.Kind{item.Wood}},
		{"Materials/Stone chunks", []item.Kind{item.SandstoneChunk, item.GraniteChunk, item.LimestoneChunk, item.SlateChunk, item.MarbleChunk}},
		{"Materials/Stone blocks", []item.Kind{item.SandstoneBlocks, item.GraniteBlocks, item.LimestoneBlocks, item.SlateBlocks,
			item.MarbleBlocks, item.Stone}},
		{"Materials/Metals", []item.Kind{item.Steel, item.Silver, item.Gold, item.Plasteel, item.Uranium}},
		{"Materials/Textiles", []item.Kind{item.Leather, item.Cloth, item.Wool, item.Straw}},
		{"Materials/Fuel and chemicals", []item.Kind{item.EmberOil, item.Chemfuel, item.Reagent}},
		{"Materials/Brewing and leaves", []item.Kind{item.Wort, item.AshleafLeaf}},
		{path: "Manufactured"},
		{"Manufactured/Components", []item.Kind{item.Components, item.AdvancedComponents}},
		{"Manufactured/Carpets", item.Carpets()},
		{"Manufactured/Prosthetics", []item.Kind{item.PegLeg, item.ProstheticLeg, item.ProstheticArm, item.WoodenHand, item.Denture,
			item.HearingTrumpet, item.ProstheticFoot, item.GlassEye, item.WoodenNose, item.ServoArm, item.ServoLeg, item.OpticImplant,
			item.CochlearImplant, item.SpinalBrace, item.PumpHeart, item.SynthStomach, item.ValveHeart}},
		{"Manufactured/Organs", []item.Kind{item.Kidney, item.Lung, item.Heart, item.Liver}},
		{"Manufactured/Shells", []item.Kind{item.MortarShells, item.IncendiaryShells, item.EMPShells, item.SmokeShells}},
		{"Manufactured/Parts", []item.Kind{item.ReinforcedBarrel}},
		{path: "Weapons"},
		{"Weapons/Melee", []item.Kind{item.Club, item.Knife, item.Spear, item.Mace, item.Gladius, item.Longsword, item.Ikwa}},
		{"Weapons/Ranged", []item.Kind{item.ShortBow, item.RecurveBow, item.Greatbow, item.Pila, item.Rifle, item.Revolver,
			item.Autopistol, item.MachinePistol, item.PumpShotgun, item.ChainShotgun, item.HeavySMG, item.LMG, item.AssaultRifle,
			item.SniperRifle, item.Minigun, item.IncendiaryLauncher, item.EMPLauncher, item.SmokeLauncher, item.ChargeRifle,
			item.ChargeLance, item.RocketLauncher, item.TripleRocketLauncher}},
		{"Weapons/Grenades", []item.Kind{item.FragGrenades, item.MolotovCocktails, item.EMPGrenades}},
		{path: "Apparel"},
		{"Apparel/Clothing", []item.Kind{item.Parka, item.WoolParka, item.Duster, item.Tuque}},
		{"Apparel/Armor", []item.Kind{item.FlakVest, item.FlakPants, item.FlakJacket, item.PlateArmor, item.MarineArmor}},
		{"Apparel/Headgear", []item.Kind{item.SimpleHelmet, item.AdvancedHelmet, item.MarineHelmet}},
		{"Apparel/Utility", []item.Kind{item.ShieldBelt, item.SmokepopPack}},
		{"Medicine", []item.Kind{item.HerbalMedicine, item.IndustrialMedicine, item.NanomedKit}},
		{path: "Drugs"},
		{"Drugs/Social drugs", []item.Kind{item.Beer, item.AshleafTea}},
		{"Drugs/Hard drugs", []item.Kind{item.SparkTablet, item.AshFlake, item.Silverdust, item.BattleTonic}},
		{"Drugs/Medical drugs", []item.Kind{item.NumbrootTincture, item.ProphylTablet}},
		{path: "Corpses"},
		{"Corpses/Animal corpses", []item.Kind{item.CorpseAnimal, item.CorpseDustbuck, item.CorpseRidgewolf}},
	}
}

// Node is one line of the filter tree, in tree order: a category (Path
// "Food/Meals", Kinds every kind under it at any depth) or a leaf (Kind
// set, Path the kind's name).
type Node struct {
	Path  string
	Label string // the last path segment, or the kind's name
	Depth int    // 0 for a top category
	Leaf  bool
	Kind  item.Kind // leaves only
	Kinds Filter    // every kind under the node (a leaf: its own)
}

// Tree is the filter tree, a category before its sub-categories and
// then its kinds.
func Tree() []Node {
	cats := categories()
	var out []Node
	for i, c := range cats {
		depth := strings.Count(c.path, "/")
		all := filterOf(c.kinds...)
		for _, sub := range cats[i+1:] {
			if strings.HasPrefix(sub.path, c.path+"/") {
				all = all.Or(filterOf(sub.kinds...))
			}
		}
		label := c.path[strings.LastIndexByte(c.path, '/')+1:]
		out = append(out, Node{Path: c.path, Label: label, Depth: depth, Kinds: all})
		for _, k := range c.kinds {
			out = append(out, Node{Path: k.String(), Label: k.String(), Depth: depth + 1, Leaf: true, Kind: k,
				Kinds: filterOf(k)})
		}
	}
	return out
}

// Resolve is the kinds under filter-tree node name: a category path, a
// top category's name or an item kind's name (ok false for none).
func Resolve(name string) (Filter, bool) {
	if k, ok := item.ParseKind(name); ok && Storable(k) {
		return filterOf(k), true
	}
	var f Filter
	found := false
	for _, c := range categories() {
		if c.path == name || strings.HasPrefix(c.path, name+"/") {
			f, found = f.Or(filterOf(c.kinds...)), true
		}
	}
	return f, found
}

// Storable is every kind the tree files (what storage may ever take).
func Storable(k item.Kind) bool { return sets.storable.Has(k) }

// Names is f as the settings window lists it: every category whose kinds
// are all in f (at any depth), then each kind of f whose own category is
// only partly in it, in tree order.
func (f Filter) Names() []string {
	var out []string
	cats := categories()
	for _, c := range cats {
		all := filterOf(c.kinds...)
		for _, sub := range cats {
			if strings.HasPrefix(sub.path, c.path+"/") {
				all = all.Or(filterOf(sub.kinds...))
			}
		}
		if all != (Filter{}) && f.Covers(all) {
			out = append(out, c.path)
		}
	}
	for _, c := range cats {
		own := filterOf(c.kinds...)
		if own == (Filter{}) || f.Covers(own) {
			continue
		}
		for _, k := range c.kinds {
			if f.Has(k) {
				out = append(out, k.String())
			}
		}
	}
	return out
}

// The older switches' kinds (a zone's Effective filter before any edit):
// a stockpile takes the Food, Wood and Stone kinds it allows and every
// other storable kind (goods); a dumping stockpile takes animal corpses
// (Corpses), stone chunks (Chunks) and what its Food, Wood and Stone
// switches allow, never goods.
type kindSets struct {
	storable, food, wood, stone, carcasses, chunks, goods Filter
}

// sets is worked out once (Allows runs for every loose item in the
// hauling search) and only read afterwards.
var sets = func() (s kindSets) {
	for _, c := range categories() {
		s.storable = s.storable.Or(filterOf(c.kinds...))
	}
	s.food = filterWhere(func(k item.Kind) bool { return k.IsFood() || k.IsCorpse() || k == item.FertileEgg }).And(s.storable)
	s.wood = filterOf(item.Wood)
	s.stone = filterWhere(func(k item.Kind) bool { return k == item.Stone || item.IsChunk(k) || item.IsStoneBlocks(k) })
	s.carcasses = filterWhere(func(k item.Kind) bool { return k.IsCorpse() && k != item.CorpseHuman })
	s.chunks = filterWhere(item.IsChunk)
	s.goods = s.storable.AndNot(s.food).AndNot(s.wood).AndNot(s.stone)
	return s
}()

// And is every kind in both f and g.
func (f Filter) And(g Filter) Filter {
	for i := range f {
		f[i] &= g[i]
	}
	return f
}

// legacy is the filter a zone's older switches give (Effective before any
// edit).
func legacy(z Zone) Filter {
	var f Filter
	if z.Food {
		f = f.Or(sets.food)
	}
	if z.Wood {
		f = f.Or(sets.wood)
	}
	if z.Stone {
		f = f.Or(sets.stone)
	}
	if z.Kind == KindDump {
		f = f.AndNot(sets.carcasses).AndNot(sets.chunks)
		if z.Corpses {
			f = f.Or(sets.carcasses)
		}
		if z.Chunks {
			f = f.Or(sets.chunks)
		}
		return f
	}
	return f.Or(sets.goods)
}

// Effective is the kinds storage zone z takes: its Filter once edited,
// else what its older switches give. Non-storage zones take nothing.
func (z Zone) Effective() Filter {
	switch {
	case !z.Kind.Storage():
		return Filter{}
	case z.Custom:
		return z.Filter
	}
	return legacy(z)
}

// Allows reports storage zone z takes items of kind k (the kind alone;
// AllowsStack adds the special filters).
func (z Zone) Allows(k item.Kind) bool {
	if !z.Kind.Storage() {
		return false
	}
	if z.Custom {
		return z.Filter.Has(k)
	}
	return legacy(z).Has(k)
}

// SetAllowed allows (on) or forbids every kind of set in z, turning the
// zone's older switches into an explicit Filter first.
func (z *Zone) SetAllowed(set Filter, on bool) {
	f := z.Effective()
	if on {
		f = f.Or(set.And(sets.storable))
	} else {
		f = f.AndNot(set)
	}
	z.Filter, z.Custom = f, true
}

// PasteFrom copies src's filter, special filters and priority onto z.
func (z *Zone) PasteFrom(src Zone) {
	z.Filter, z.Custom, z.Special, z.Priority = src.Effective(), true, src.Special, src.Priority
}

// Special is a storage zone's special filters (PRD-4). The zero value
// takes everything (today's behaviour): rotten stacks, tainted apparel,
// any hit points and any quality.
type Special struct {
	// NoRotten refuses spoiled stacks; NoTainted apparel a dead body wore.
	NoRotten, NoTainted bool
	// HPMin..HPMax is the hit-point range in percent, for kinds with hit
	// points (HPMax 0: 100).
	HPMin, HPMax uint8
	// QualityMin..QualityMax is the quality range, for kinds with a
	// quality (QualityMax is stored one up: 0 is Legendary).
	QualityMin, QualityMax uint8
}

// Special filter indexes (SetStockpileSpecial's Index): the two toggles
// (Enabled allows) and the two ranges (low..high).
const (
	SpecialRotten = iota
	SpecialTainted
	SpecialHP
	SpecialQuality
	numSpecials
)

// SpecialNames are the toggles by index, as ZoneView.Special lists the
// ones that are on.
func SpecialNames() []string { return []string{"allow rotten", "allow tainted apparel"} }

// HPRange is the allowed hit points in percent.
func (s Special) HPRange() (lo, hi int) {
	hi = 100
	if s.HPMax > 0 {
		hi = int(s.HPMax)
	}
	return int(s.HPMin), hi
}

// QualityRange is the allowed quality levels.
func (s Special) QualityRange() (lo, hi quality.Level) {
	hi = quality.Legendary
	if s.QualityMax > 0 {
		hi = quality.Level(s.QualityMax - 1)
	}
	return quality.Level(s.QualityMin), hi
}

// Set applies special filter i: a toggle allows (on) or refuses; a range
// takes lo..hi (percent for hit points, quality levels for quality),
// clamped and ordered. An unknown index changes nothing.
func (s Special) Set(i int, on bool, lo, hi int) Special {
	switch i {
	case SpecialRotten:
		s.NoRotten = !on
	case SpecialTainted:
		s.NoTainted = !on
	case SpecialHP:
		lo, hi = clampRange(lo, hi, 0, 100)
		s.HPMin, s.HPMax = uint8(lo), uint8(max(hi, 1))
		if hi == 100 {
			s.HPMax = 0
		}
	case SpecialQuality:
		lo, hi = clampRange(lo, hi, 0, int(quality.Legendary))
		s.QualityMin, s.QualityMax = uint8(lo), uint8(hi+1)
		if hi == int(quality.Legendary) {
			s.QualityMax = 0
		}
	}
	return s
}

func clampRange(lo, hi, floor, ceil int) (int, int) {
	lo, hi = min(max(lo, floor), ceil), min(max(hi, floor), ceil)
	if hi < lo {
		lo, hi = hi, lo
	}
	return lo, hi
}

// Names lists the toggles that are on (SpecialNames), then each range
// the player narrowed ("hp 50-100%", "quality 3-6": quality levels,
// 0 awful to 6 legendary).
func (s Special) Names() []string {
	names := SpecialNames()
	var out []string
	if !s.NoRotten {
		out = append(out, names[SpecialRotten])
	}
	if !s.NoTainted {
		out = append(out, names[SpecialTainted])
	}
	if lo, hi := s.HPRange(); lo > 0 || hi < 100 {
		out = append(out, "hp "+itoa(lo)+"-"+itoa(hi)+"%")
	}
	if lo, hi := s.QualityRange(); lo > quality.Awful || hi < quality.Legendary {
		out = append(out, "quality "+itoa(int(lo))+"-"+itoa(int(hi)))
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for ; n > 0; n /= 10 {
		i--
		b[i] = byte('0' + n%10)
	}
	return string(b[i:])
}

// Takes reports stack st passes the special filters (the kind is checked
// by Allows).
func (s Special) Takes(st item.Stack) bool {
	if s == (Special{}) {
		return true
	}
	if s.NoRotten && st.Rotten() || s.NoTainted && st.Tainted() {
		return false
	}
	if mhp := st.MaxHP(); mhp > 0 {
		lo, hi := s.HPRange()
		if pct := st.HP() * 100 / mhp; pct < lo || pct > hi {
			return false
		}
	}
	if l, ok := quality.FromStored(st.Quality); ok && item.HasQuality(st.Kind) {
		lo, hi := s.QualityRange()
		if l < lo || l > hi {
			return false
		}
	}
	return true
}

// AllowsStack reports storage zone z takes stack st: its kind and its
// special filters.
func (z Zone) AllowsStack(st item.Stack) bool { return z.Allows(st.Kind) && z.Special.Takes(st) }

// Row is one line of the filter tree for the settings window: a category
// path or an item kind's name (Leaf), its depth, and whether a filter
// allows all of it (On) or only some (Partial).
type Row struct {
	Path, Label string
	Depth       int
	Leaf        bool
	On, Partial bool
}

// Rows is the filter tree in display order with each line's state under
// names (Filter.Names, as ZoneView.Filter carries it).
func Rows(names []string) []Row {
	var allowed Filter
	for _, name := range names {
		if set, ok := Resolve(name); ok {
			allowed = allowed.Or(set)
		}
	}
	tree := Tree()
	out := make([]Row, len(tree))
	for i, n := range tree {
		on := allowed.Covers(n.Kinds)
		out[i] = Row{Path: n.Path, Label: n.Label, Depth: n.Depth, Leaf: n.Leaf, On: on, Partial: !on && allowed.Any(n.Kinds)}
	}
	return out
}

// NewShelf is a new shelf's storage (wiki: Shelf): Preferred priority,
// every storable kind but stone chunks and corpses, its overlay hidden.
func NewShelf() Zone {
	z := Zone{Kind: KindShelf, Hidden: true, Priority: PriorityPreferred}
	z.SetAllowed(sets.storable.AndNot(sets.chunks).AndNot(sets.carcasses), true)
	return z
}
