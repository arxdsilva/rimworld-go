package trade

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/pkg/randx"
)

// The exported API below is the PLAY-024 contract the trade application
// files and the trade dialog build against.

// TraderKind is a kind of trader (append only, saved).
type TraderKind uint8

const (
	// BulkHauler: a caravan of raw goods, food and materials.
	BulkHauler TraderKind = iota
	// Quartermaster: a caravan of weapons, apparel, medicine and parts.
	Quartermaster
	// CurioMerchant: an orbital ship of rare and costly goods.
	CurioMerchant
	NumKinds
)

// Side is which way goods move, seen from the colony.
type Side uint8

const (
	// Buy: the colony buys from the trader (the trader's asking price).
	Buy Side = iota
	// Sell: the colony sells to the trader (the trader's offer).
	Sell
)

// Def is a trader kind's static definition. SilverMin..SilverMax is the
// silver it arrives with; Specialty lists the item categories
// (item.Kind.Category) it pays SpecialtyPct more for.
type Def struct {
	Kind                 TraderKind
	Key, Label           string
	Orbital              bool
	SilverMin, SilverMax int
	Specialty            []string
}

// Price rules (hundredths of a silver, percent; PLAY024_PLAN.md).
const (
	// BuyMarkupPct is the markup over value at Social 0 and goodwill 0;
	// each Social level takes off SocialStepCenti hundredths of a point and
	// each goodwill point BuyGoodwillMilli thousandths of a point, down to
	// BuyFloorPct.
	BuyMarkupPct     = 45
	BuyFloorPct      = 6
	BuyGoodwillMilli = 60 // 0.06 points per goodwill
	// SellCutPct is the trader's cut below value at Social 0 and goodwill
	// 0, shrinking the same way (SellGoodwillMilli per goodwill point) down
	// to SellFloorPct.
	SellCutPct        = 50
	SellFloorPct      = 10
	SellGoodwillMilli = 50 // 0.05 points per goodwill
	// SocialStepCenti is 1.2 points per Social level, in hundredths.
	SocialStepCenti = 120
	// SellCapPct: a sell price is never above this share of the buy price.
	SellCapPct = 90
	// SpecialtyPct is the extra a trader pays for its specialty.
	SpecialtyPct = 10
)

// Line is one stock row: Qty units of Kind. Building is a crated
// building's construction kind (AX-1; Kind item.MinifiedBuilding), 0 for
// any other goods.
type Line struct {
	Kind     item.Kind
	Qty      int
	Building uint8
}

// Stock is a trader's goods (sorted by item kind, silver not among them)
// and its silver.
type Stock struct {
	Kind   TraderKind
	Lines  []Line
	Silver int
}

// Terms are what moves prices: the negotiator's Social level, the
// faction's goodwill, and whether the faction is hostile (hostile
// factions don't trade).
type Terms struct {
	Social   int
	Goodwill int
	Hostile  bool
}

// Delta is one line of an offer: Qty > 0 the colony buys Qty units of
// Kind, Qty < 0 it sells -Qty. Silver never appears: Settle balances it.
type Delta struct {
	Kind     item.Kind
	Qty      int
	Building uint8 // a crated building's kind (AX-1), as Line.Building
}

// Reject is why Settle refused an offer (RejectNone: accepted).
type Reject uint8

const (
	RejectNone Reject = iota
	// RejectEmpty: nothing is offered.
	RejectEmpty
	// RejectHostile: the faction is hostile.
	RejectHostile
	// RejectUnaffordable: the side paying has too little silver.
	RejectUnaffordable
	// RejectOverStock: more is asked than the trader has, or offered than
	// the colony has.
	RejectOverStock
	// RejectUnwanted: the trader won't take that kind of goods.
	RejectUnwanted
)

// String is the player-facing reason ("" for RejectNone).
func (r Reject) String() string {
	switch r {
	case RejectEmpty:
		return "nothing offered"
	case RejectHostile:
		return "they won't trade with you"
	case RejectUnaffordable:
		return "not enough silver"
	case RejectOverStock:
		return "not enough goods"
	case RejectUnwanted:
		return "they don't want that"
	}
	return ""
}

// Result is a settled offer. SilverCenti is the colony's net silver change
// in hundredths (positive: it receives), Silver the same in whole silver as
// it moves (rounded toward the trader). Buy and Sell are the goods moving
// to and from the colony. Goodwill is what a gift earns.
type Result struct {
	Reject      Reject
	SilverCenti int
	Silver      int
	Buy, Sell   []Line
	Goodwill    int
}

// OK reports an accepted offer.
func (r Result) OK() bool { return r.Reject == RejectNone }

var defs = [NumKinds]Def{
	BulkHauler:    {Kind: BulkHauler, Key: "bulk_hauler", Label: "Bulk hauler", SilverMin: 900, SilverMax: 1500, Specialty: []string{"Food", "Materials"}},
	Quartermaster: {Kind: Quartermaster, Key: "quartermaster", Label: "Quartermaster", SilverMin: 1200, SilverMax: 2000, Specialty: []string{"Weapons", "Apparel"}},
	CurioMerchant: {Kind: CurioMerchant, Key: "curio_merchant", Label: "Curio merchant", Orbital: true, SilverMin: 1600, SilverMax: 2600, Specialty: []string{"Manufactured"}},
}

// DefOf is trader kind k's definition (the zero Def for an unknown kind).
func DefOf(k TraderKind) Def {
	if k >= NumKinds {
		return Def{}
	}
	return defs[k]
}

// entry is one stock candidate: Min..Max units, present with chance Pct.
type entry struct {
	kind     item.Kind
	min, max int
	pct      int
}

var pools = [NumKinds][]entry{
	BulkHauler: {
		{item.Wood, 80, 200, 100}, {item.Steel, 40, 120, 100}, {item.Cloth, 40, 120, 100},
		{item.SurvivalMeal, 20, 60, 100}, {item.Leather, 20, 60, 80}, {item.Potatoes, 30, 90, 80},
		{item.Corn, 30, 90, 70}, {item.Wheat, 30, 90, 70}, {item.Beans, 20, 60, 60},
		{item.Meat, 20, 60, 60}, {item.Wool, 20, 60, 50}, {item.HerbalMedicine, 5, 15, 90},
		{item.Components, 2, 6, 70}, {item.SlateBlocks, 30, 100, 40},
		// AX-7: books (wiki: Books, sold by trade caravans and orbital
		// traders); counts and chances are original tuning.
		{item.Novel, 1, 3, 50}, {item.Textbook, 1, 2, 40},
	},
	Quartermaster: {
		{item.Rifle, 1, 3, 100}, {item.Club, 1, 4, 80}, {item.Knife, 2, 5, 80}, {item.Spear, 2, 5, 80},
		{item.Parka, 1, 3, 90}, {item.Tuque, 2, 5, 90}, {item.Duster, 1, 3, 80}, {item.WoolParka, 1, 2, 60},
		{item.IndustrialMedicine, 3, 8, 90}, {item.HerbalMedicine, 5, 12, 80},
		{item.Components, 2, 6, 70}, {item.Steel, 20, 60, 60},
		// TEMP-4: the layered core clothing, sold at the wiki market values
		// (item.Value); counts and chances are original tuning.
		{item.TShirt, 2, 5, 80}, {item.ButtonDownShirt, 1, 3, 60}, {item.Pants, 2, 5, 80}, {item.Jacket, 1, 2, 60},
		{item.CowboyHat, 1, 2, 50}, {item.BowlerHat, 1, 2, 40}, {item.Tribalwear, 1, 3, 40},
	},
	CurioMerchant: {
		{item.Gold, 20, 60, 100}, {item.NanomedKit, 2, 5, 90}, {item.ServoArm, 1, 1, 60},
		{item.ServoLeg, 1, 1, 60}, {item.OpticImplant, 1, 1, 50}, {item.Beer, 4, 12, 80},
		{item.SparkTablet, 3, 8, 70}, {item.AshleafTea, 4, 10, 70}, {item.CarpetWool, 2, 6, 80},
		{item.IndustrialMedicine, 2, 6, 60},
		// AX-8: now and then a mind core, the ship computer core's heart
		// (wiki: Persona core, found through trade and quests); the chance
		// is original tuning.
		{item.MindCore, 1, 1, 15},
		// AX-7: books (see the bulk hauler's).
		{item.Novel, 1, 3, 70}, {item.Textbook, 1, 3, 60}, {item.Schematic, 1, 2, 50},
	},
}

// neolithicBanned reports goods a neolithic faction cannot make or carry.
func neolithicBanned(it item.Kind) bool { return it == item.Rifle || it == item.Components }

// Generate is a fresh stock for a trader of kind k from a faction of
// technology tech, drawn from rng only. Neolithic traders carry no rifles
// or components.
func Generate(k TraderKind, tech faction.Tech, rng *randx.Rand) Stock {
	d := DefOf(k)
	st := Stock{Kind: k}
	if k >= NumKinds {
		return st
	}
	st.Silver = d.SilverMin + rng.IntN(d.SilverMax-d.SilverMin+1)
	for _, e := range pools[k] {
		// Draw for every entry, banned or not, so the stream does not
		// depend on the tech level.
		qty := e.min + rng.IntN(e.max-e.min+1)
		roll := rng.IntN(100)
		if tech == faction.Neolithic && neolithicBanned(e.kind) {
			continue
		}
		if roll < e.pct {
			st.AddGoods(Goods{Kind: e.kind}, qty)
		}
	}
	for _, c := range cratePools[k] { // goods.go (AX-1): drawn after the goods, so their stream is unchanged
		qty := c.min + rng.IntN(c.max-c.min+1)
		if rng.IntN(100) < c.pct {
			st.AddGoods(Goods{item.MinifiedBuilding, uint8(c.kind)}, qty)
		}
	}
	return st
}

// Wants reports whether a trader of kind k buys goods of kind it. Silver
// is not a good: it settles every trade.
func Wants(k TraderKind, it item.Kind) bool {
	if k >= NumKinds || it.Value() <= 0 || it == item.Silver || it == item.MinifiedBuilding { // AX-1: crates are only sold to the colony
		return false
	}
	c := it.Category()
	switch k {
	case BulkHauler:
		return c == "Food" || c == "Materials" || it == item.HerbalMedicine || it == item.IndustrialMedicine || it == item.Components
	case Quartermaster:
		return c == "Weapons" || c == "Apparel" || c == "Medicine" && !it.IsDrug() || it == item.Components || it == item.Steel
	default:
		return c == "Manufactured" || c == "Medicine" || it == item.Gold || it == item.Wool
	}
}

func isSpecialty(k TraderKind, it item.Kind) bool {
	c := it.Category()
	for _, s := range DefOf(k).Specialty {
		if s == c {
			return true
		}
	}
	return false
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Price is one unit of it on side s with a trader of kind k under terms
// t, in hundredths of a silver. Silver is always at face value (100). A
// hostile faction, or a kind without a market value, has no price (0).
func Price(it item.Kind, k TraderKind, s Side, t Terms) int {
	return PriceOf(Goods{Kind: it}, k, s, t)
}

// PriceOf is Price for goods g: a crated building is worth CrateValue.
func PriceOf(g Goods, k TraderKind, s Side, t Terms) int {
	it := g.Kind
	if it == item.Silver {
		return 100
	}
	v := g.Value() * 100
	if v <= 0 || t.Hostile {
		return 0
	}
	social := clamp(t.Social, 0, 20)
	// Rates are in hundredths of a percentage point (10000 = 100 %).
	markup := max(BuyMarkupPct*100-SocialStepCenti*social-BuyGoodwillMilli/10*t.Goodwill, BuyFloorPct*100)
	buy := v * (10000 + markup) / 10000
	if s == Buy {
		return buy
	}
	cut := max(SellCutPct*100-SocialStepCenti*social-SellGoodwillMilli/10*t.Goodwill, SellFloorPct*100)
	sell := v * (10000 - cut) / 10000
	if isSpecialty(k, it) {
		sell = sell * (100 + SpecialtyPct) / 100
	}
	return min(sell, buy*SellCapPct/100)
}

// Settle checks offer against the colony's counts (colony), the trader's
// stock and silver, and terms t, and returns what moves. A gift moves only
// the colony's goods (and silver) to the trader, for goodwill; no silver
// comes back.
func Settle(offer []Delta, colony map[item.Kind]int, st Stock, t Terms, gift bool) Result {
	if t.Hostile {
		return Result{Reject: RejectHostile}
	}
	net := map[Goods]int{}
	for _, d := range offer {
		net[Goods{d.Kind, d.Building}] += d.Qty
	}
	keys := make([]Goods, 0, len(net))
	for g, q := range net {
		if q != 0 {
			keys = append(keys, g)
		}
	}
	if len(keys) == 0 {
		return Result{Reject: RejectEmpty}
	}
	slices.SortFunc(keys, Goods.Compare)
	stock := map[Goods]int{}
	for _, l := range st.Lines {
		stock[l.Goods()] += l.Qty
	}
	var res Result
	var buyC, sellC int // hundredths
	for _, g := range keys {
		q, it := net[g], g.Kind
		switch {
		case q > 0:
			if gift || it == item.Silver {
				return Result{Reject: RejectUnwanted}
			}
			if stock[g] < q {
				return Result{Reject: RejectOverStock}
			}
			buyC += q * PriceOf(g, st.Kind, Buy, t)
			res.Buy = append(res.Buy, Line{Kind: it, Qty: q, Building: g.Building})
		default:
			q = -q
			if g.Building != 0 || colony[it] < q { // the colony sells plain goods only
				return Result{Reject: RejectOverStock}
			}
			if gift {
				if it.Value() <= 0 {
					return Result{Reject: RejectUnwanted}
				}
			} else if !Wants(st.Kind, it) {
				return Result{Reject: RejectUnwanted}
			}
			sellC += q * Price(it, st.Kind, Sell, t)
			res.Sell = append(res.Sell, Line{Kind: it, Qty: q})
		}
	}
	if gift {
		res.Goodwill = faction.GiftGoodwill(GiftValue(offer))
		return res
	}
	res.SilverCenti = sellC - buyC
	if res.SilverCenti < 0 {
		res.Silver = -((-res.SilverCenti + 99) / 100) // colony pays, rounded up
		if colony[item.Silver] < -res.Silver {
			return Result{Reject: RejectUnaffordable}
		}
	} else {
		res.Silver = res.SilverCenti / 100 // trader pays, rounded down
		if st.Silver < res.Silver {
			return Result{Reject: RejectUnaffordable}
		}
	}
	return res
}

// GiftValue is the market value, in whole silver, of what the colony
// gives in offer (its negative lines; silver counts at face value).
func GiftValue(offer []Delta) int {
	total := 0
	for _, d := range offer {
		if d.Qty < 0 {
			total += -d.Qty * d.Kind.Value()
		}
	}
	return total
}

// Add adds qty (negative: removes) of kind k to st, keeping its lines
// sorted by kind and dropping empty ones (moved from the game package, AX-8).
func (st *Stock) Add(k item.Kind, qty int) { st.AddGoods(Goods{Kind: k}, qty) }

// AddGoods is Add for goods g (a crated building keeps its own line,
// AX-1); lines sort by kind, then building.
func (st *Stock) AddGoods(g Goods, qty int) {
	i, ok := slices.BinarySearchFunc(st.Lines, g, func(l Line, g Goods) int { return l.Goods().Compare(g) })
	if ok {
		st.Lines[i].Qty += qty
		if st.Lines[i].Qty <= 0 {
			st.Lines = slices.Delete(st.Lines, i, i+1)
		}
		return
	}
	if qty > 0 {
		st.Lines = slices.Insert(st.Lines, i, Line{Kind: g.Kind, Qty: qty, Building: g.Building})
	}
}
