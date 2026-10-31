package zone

import (
	"math/bits"
	"strings"

	"rimworld-go/internal/domain/economy/item"
)

// Storage filters (PRD-4). A storage zone takes the item kinds in its
// Filter, one bit per kind, once the player has edited it (Custom); until
// then it goes by the older Food, Wood, Stone, Corpses and Chunks
// switches (Effective), so older saves and state hashes stay as they
// were. The filter tree groups the kinds for the settings window: a node
// is a category path ("Foods/Meals") or an item kind's name. The tree's
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

// Root is the filter-tree node naming every kind a zone may take
// (SetStockpileFilter's "allow all" and "clear all").
const Root = "All"

// categories is the filter tree's inner nodes, in the shape of the
// vanilla storage tree (STO-2: Foods, Manufactured, Raw resources,
// Items, Weapons, Apparel, Corpses, Chunks, Bones; the names and the
// grouping below them are our own). Every storable kind sits under
// exactly one of them (TestFilterTreeFilesEveryStorableKind). There are
// no unfinished items, and a mechanoid wreck is an animal corpse kind
// (Stack.Animal names the machine), so neither has a node of its own.
func categories() []category {
	return []category{
		{path: "Foods"},
		{"Foods/Meals", []item.Kind{item.Meal, item.FineMeal, item.LavishMeal, item.SurvivalMeal, item.Pemmican, item.Food,
			item.NutrientPasteMeal}},
		{path: "Foods/Raw food"},
		{"Foods/Raw food/Plant matter", []item.Kind{item.Berries, item.Potatoes, item.Corn, item.Wheat, item.Beans, item.Lettuce,
			item.Radishes, item.Pumpkins, item.Brewgrain, item.Rice, item.Strawberries, item.AgaveFruit}},
		{"Foods/Raw food/Meat", []item.Kind{item.Meat, item.HumanMeat, item.InsectMeat}},
		{"Foods/Raw food/Animal products", []item.Kind{item.Milk, item.Egg, item.FertileEgg}},
		{"Foods/Animal feed", []item.Kind{item.Kibble, item.Hay}},
		{path: "Manufactured"},
		{"Manufactured/Medicine", []item.Kind{item.HerbalMedicine, item.IndustrialMedicine, item.NanomedKit}},
		{path: "Manufactured/Drugs"},
		{"Manufactured/Drugs/Social drugs", []item.Kind{item.Beer, item.AshleafTea}},
		{"Manufactured/Drugs/Hard drugs", []item.Kind{item.SparkTablet, item.AshFlake, item.Silverdust, item.BattleTonic}},
		{"Manufactured/Drugs/Medical drugs", []item.Kind{item.NumbrootTincture, item.ProphylTablet}},
		{"Manufactured/Textiles", []item.Kind{item.Leather, item.Cloth, item.Wool, item.Straw}},
		{"Manufactured/Components", []item.Kind{item.Components, item.AdvancedComponents}},
		{path: "Manufactured/Body parts"},
		{"Manufactured/Body parts/Prosthetics", []item.Kind{item.PegLeg, item.ProstheticLeg, item.ProstheticArm, item.WoodenHand,
			item.Denture, item.HearingTrumpet, item.ProstheticFoot, item.GlassEye, item.WoodenNose, item.ServoArm, item.ServoLeg,
			item.OpticImplant, item.CochlearImplant, item.SpinalBrace, item.PumpHeart, item.SynthStomach, item.ValveHeart}},
		{"Manufactured/Body parts/Natural organs", []item.Kind{item.Kidney, item.Lung, item.Heart, item.Liver}},
		{path: "Raw resources"},
		{"Raw resources/Wood", []item.Kind{item.Wood}},
		{"Raw resources/Stone blocks", []item.Kind{item.SandstoneBlocks, item.GraniteBlocks, item.LimestoneBlocks, item.SlateBlocks,
			item.MarbleBlocks, item.Stone}},
		{"Raw resources/Metals", []item.Kind{item.Steel, item.Silver, item.Gold, item.Plasteel, item.Uranium}},
		{"Raw resources/Chemicals", []item.Kind{item.EmberOil, item.Chemfuel, item.Reagent}},
		{"Raw resources/Brewing and leaves", []item.Kind{item.Wort, item.AshleafLeaf, item.Hops}},
		{path: "Items"},
		{"Items/Shells", []item.Kind{item.MortarShells, item.IncendiaryShells, item.EMPShells, item.SmokeShells}},
		{"Items/Parts", []item.Kind{item.ReinforcedBarrel}},
		{"Items/Carpets", item.Carpets()},
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
		{path: "Corpses"},
		{"Corpses/Human", []item.Kind{item.CorpseHuman}},
		{"Corpses/Animal", []item.Kind{item.CorpseAnimal, item.CorpseDustbuck, item.CorpseRidgewolf}},
		{"Chunks", []item.Kind{item.SandstoneChunk, item.GraniteChunk, item.LimestoneChunk, item.SlateChunk, item.MarbleChunk}},
		{"Bones", []item.Kind{item.Bones}},
	}
}

// Node is one line of the filter tree, in tree order: a category (Path
// "Foods/Meals", Kinds every kind under it at any depth) or a leaf (Kind
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

// Resolve is the kinds under filter-tree node name: Root, a category
// path, a top category's name or an item kind's name (ok false for none).
func Resolve(name string) (Filter, bool) {
	if name == Root {
		return sets.storable, true
	}
	if k, ok := item.ParseKind(name); ok && Storable(k) {
		return filterOf(k), true
	}
	return resolvePath(name)
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
// other storable kind of before STO-2 (goods); a dumping stockpile takes
// animal corpses (Corpses), stone chunks (Chunks) and what its Food, Wood
// and Stone switches allow, never goods. Human corpses and bones became
// storable with STO-2 (Corpses/Human, Bones); the switches never take
// them, so older zones store what they did.
type kindSets struct {
	storable, old, food, wood, stone, carcasses, chunks, goods Filter
	corpses, rawFood, bodies                                   Filter
}

// sets is worked out once (Allows runs for every loose item in the
// hauling search) and only read afterwards.
var sets = func() (s kindSets) {
	for _, c := range categories() {
		s.storable = s.storable.Or(filterOf(c.kinds...))
	}
	s.bodies = filterOf(item.CorpseHuman, item.Bones)
	s.old = s.storable.AndNot(s.bodies)
	s.food = filterWhere(func(k item.Kind) bool { return k.IsFood() || k.IsCorpse() || k == item.FertileEgg }).And(s.old)
	s.wood = filterOf(item.Wood)
	s.stone = filterWhere(func(k item.Kind) bool { return k == item.Stone || item.IsChunk(k) || item.IsStoneBlocks(k) })
	s.carcasses = filterWhere(func(k item.Kind) bool { return k.IsCorpse() && k != item.CorpseHuman })
	s.chunks = filterWhere(item.IsChunk)
	s.goods = s.old.AndNot(s.food).AndNot(s.wood).AndNot(s.stone)
	s.corpses = filterWhere(item.Kind.IsCorpse).Or(s.bodies)
	s.rawFood, _ = resolvePath("Foods/Raw food")
	return s
}()

// resolvePath is the kinds under category path p (no kind names, no
// Root), for the sets above.
func resolvePath(p string) (Filter, bool) {
	var f Filter
	found := false
	for _, c := range categories() {
		if c.path == p || strings.HasPrefix(c.path, p+"/") {
			f, found = f.Or(filterOf(c.kinds...)), true
		}
	}
	return f, found
}

// And is every kind in both f and g.
func (f Filter) And(g Filter) Filter {
	for i := range f {
		f[i] &= g[i]
	}
	return f
}

// Universe is every kind a storage zone of kind k may ever take: a
// stockpile or dumping stockpile anything storable; a shelf no chunks,
// corpses or bones (wiki: Shelf); a hopper raw food only (wiki: Hopper);
// a grave or sarcophagus corpses and bones (wiki: Grave). Other kinds
// take nothing.
func Universe(k Kind) Filter {
	switch k {
	case KindStockpile, KindDump:
		return sets.storable
	case KindShelf:
		return sets.storable.AndNot(sets.chunks).AndNot(sets.corpses)
	case KindHopper:
		return sets.rawFood
	case KindGrave:
		return sets.corpses
	}
	return Filter{}
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
// else what its older switches give. Zones without settings take nothing.
func (z Zone) Effective() Filter {
	switch {
	case !z.Kind.Settings():
		return Filter{}
	case z.Custom:
		return z.Filter
	}
	return legacy(z)
}

// Allows reports storage zone z takes items of kind k (the kind alone;
// AllowsStack adds the special filters). A grave's kinds are its
// settings, not a hauling destination (Kind.Storage).
func (z Zone) Allows(k item.Kind) bool {
	if !z.Kind.Settings() {
		return false
	}
	if z.Custom {
		return z.Filter.Has(k)
	}
	return legacy(z).Has(k)
}

// SetAllowed allows (on) or forbids every kind of set in z (within its
// Universe), turning the zone's older switches into an explicit Filter
// first. SetAllowed(Root's set, on) is "allow all" or "clear all".
func (z *Zone) SetAllowed(set Filter, on bool) {
	f := z.Effective()
	if on {
		f = f.Or(set.And(Universe(z.Kind)))
	} else {
		f = f.AndNot(set)
	}
	z.Filter, z.Custom = f, true
}

// PasteFrom copies src's filter (within z's Universe), special filters
// and priority onto z.
func (z *Zone) PasteFrom(src Zone) {
	z.Filter, z.Custom, z.Special, z.Priority = src.Effective().And(Universe(z.Kind)), true, src.Special, src.Priority
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
func Rows(names []string) []Row { return RowsFor(KindStockpile, names) }

// RowsFor is Rows for a storage zone of kind k: only the nodes holding a
// kind of its Universe (a hopper's raw food, a grave's corpses), each
// judged on those kinds alone.
func RowsFor(k Kind, names []string) []Row {
	var allowed Filter
	for _, name := range names {
		if set, ok := Resolve(name); ok {
			allowed = allowed.Or(set)
		}
	}
	u := Universe(k)
	tree := Tree()
	out := make([]Row, 0, len(tree))
	for _, n := range tree {
		kinds := n.Kinds.And(u)
		if kinds == (Filter{}) {
			continue
		}
		on := allowed.Covers(kinds)
		out = append(out, Row{Path: n.Path, Label: n.Label, Depth: n.Depth, Leaf: n.Leaf, On: on, Partial: !on && allowed.Any(kinds)})
	}
	return out
}
