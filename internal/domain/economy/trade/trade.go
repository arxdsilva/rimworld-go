// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/pkg/mathx"
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
	TribalSettlement
	IndustrialSettlement
	SpacerSettlement
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

// Def describes a trader's stock categories and available silver.
type Def struct {
	Kind                 TraderKind
	Key, Label           string
	Orbital              bool
	SilverMin, SilverMax int
	Specialty            []string
}

// Published Core trade pricing. All price quotes use hundredths of silver.
const (
	BuyBasePct               = 140
	SellBasePct              = 60
	SocialStepCenti          = 150
	MaxTradeImprovementCenti = 3950
)

// Plugin goods in stock (PLG-026): a plugin kind whose def opts in
// (item.Def.TraderStock > 0) is stocked by the traders that want it.
const (
	// pluginStockPct is the chance a trader carries an opted-in plugin
	// kind it wants (original tuning).
	pluginStockPct = 50
)

// Line is one stock row: Qty units of Kind. Building is a crated
// building's construction kind (AX-1; Kind item.MinifiedBuilding), 0 for
// any other goods.
type Line struct {
	Kind     item.Kind
	Qty      int
	Building uint8
	Lots     []Lot // saved physical variants within the stock row
}

// Stock is a trader's goods (sorted by item kind, silver not among them)
// and its silver.
type Stock struct {
	Kind      TraderKind
	Lines     []Line
	Silver    int
	NextLotID uint64
}

// Terms are what moves prices: the negotiator's Social level, the
// faction's goodwill, and whether the faction is hostile (hostile
// factions don't trade).
type Terms struct {
	Social       int
	Goodwill     int
	Hostile      bool
	Inspired     bool
	ImproveCenti int          // +2% TPI at a faction settlement, applied after the cap
	FactionType  faction.Type // gift recipient; pricing remains goodwill-independent
}

// Delta is one line of an offer: Qty > 0 the colony buys Qty units of
// Kind, Qty < 0 it sells -Qty. Silver never appears: Settle balances it.
type Delta struct {
	Kind     item.Kind
	Qty      int
	Building uint8  // a crated building's kind (AX-1), as Line.Building
	StackID  uint64 // colony stack for sale/gift, trader lot for purchase
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
	// RejectBadQty: a line's quantity, or a goods' summed quantity, is
	// beyond MaxOfferQty.
	RejectBadQty
)

// MaxOfferQty bounds one offer line and one goods' summed quantity (ours:
// no stock reaches it, and MaxOfferQty times the highest centi price stays
// far below MaxInt64, so Settle's and GiftValue's products cannot overflow).
const MaxOfferQty = 1_000_000

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
	case RejectBadQty:
		return "impossible quantity"
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

// TradedValue sums the market value of purchases and sales for goodwill.
// The book preserves plugin item prices and crate construction kinds.
func (r Result) TradedValue(b item.Book) int {
	total := 0
	for _, lines := range [][]Line{r.Buy, r.Sell} {
		for _, line := range lines {
			v, ok := addCenti(total, line.OrdinaryQty(), line.Goods().Value(b))
			if !ok {
				return 0
			}
			total = v
			for _, lot := range line.Lots {
				market := stackMarketValue(b, lot.Stack)
				if line.Kind == item.MinifiedBuilding {
					market = line.Goods().Value(b)
				}
				total, ok = addCenti(total, lot.Stack.Qty, market)
				if !ok {
					return 0
				}
			}
		}
	}
	return total
}

var defs = [NumKinds]Def{
	BulkHauler:           {Kind: BulkHauler, Key: "bulk_hauler", Label: "Bulk hauler", SilverMin: 900, SilverMax: 1500, Specialty: []string{"Food", "Materials"}},
	Quartermaster:        {Kind: Quartermaster, Key: "quartermaster", Label: "Quartermaster", SilverMin: 1200, SilverMax: 2000, Specialty: []string{"Weapons", "Apparel"}},
	CurioMerchant:        {Kind: CurioMerchant, Key: "curio_merchant", Label: "Curio merchant", Orbital: true, SilverMin: 1600, SilverMax: 2600, Specialty: []string{"Manufactured"}},
	TribalSettlement:     {Kind: TribalSettlement, Key: "tribal_settlement", Label: "Tribal settlement", SilverMin: 400, SilverMax: 1500},
	IndustrialSettlement: {Kind: IndustrialSettlement, Key: "industrial_settlement", Label: "Industrial settlement", SilverMin: 800, SilverMax: 3000},
	SpacerSettlement:     {Kind: SpacerSettlement, Key: "spacer_settlement", Label: "Spacer settlement", SilverMin: 800, SilverMax: 3000},
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
		{item.ServoLeg, 1, 1, 60}, {item.OpticImplant, 1, 1, 50},
		{item.Beer, 4, 12, 80},
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

// MED-14 devices have their own draw sequence. The historical merchant
// stock and the caller's RNG stream retain their existing identities.
var curioMed14 = [...]entry{
	{item.BlissCircuit, 1, 1, 30}, {item.NerveDampener, 1, 1, 30},
	{item.AcidFailsafe, 1, 1, 15}, {item.AcousticImplant, 1, 1, 50},
	{item.ServoClaw, 1, 1, 20}, {item.CochlearImplant, 1, 1, 40},
	{item.ServoJaw, 1, 1, 40}, {item.SynthTongue, 1, 1, 40},
	{item.ApexArm, 1, 1, 10}, {item.ApexLeg, 1, 1, 10},
	{item.ApexEye, 1, 1, 10},
}

// neolithicBanned reports goods a neolithic faction cannot make or carry.
func neolithicBanned(it item.Kind) bool { return it == item.Rifle || it == item.Components }

// Generate is a fresh stock for a trader of kind k from a faction of
// technology tech, drawn from rng only. Neolithic traders carry no rifles
// or components. The run's plugin kinds that opt in (item.Def.TraderStock)
// and that k wants are drawn last, in kind order, so a run without them
// draws exactly the core stream.
func Generate(b item.Book, k TraderKind, tech faction.Tech, rng *randx.Rand) Stock {
	d := DefOf(k)
	st := Stock{Kind: k}
	if k >= NumKinds {
		return st
	}
	if k >= TribalSettlement {
		return generateSettlement(tech, k, rng)
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
	var medical *randx.Rand
	if k == CurioMerchant {
		medical = rng.Clone() // fork before plugin draws: core medical stock is plugin-independent
	}
	for _, it := range b.Plugins() {
		n := b.Def(it).TraderStock
		if n <= 0 || !Wants(b, k, it) {
			continue
		}
		qty := 1 + rng.IntN(n)
		if rng.IntN(100) < pluginStockPct {
			st.AddGoods(Goods{Kind: it}, qty)
		}
	}
	if medical != nil {
		for _, e := range curioMed14 {
			qty := e.min + medical.IntN(e.max-e.min+1)
			if medical.IntN(100) < e.pct {
				st.AddGoods(Goods{Kind: e.kind}, qty)
			}
		}
	}
	return st
}

// Wants reports whether a trader of kind k buys goods of kind it, by its
// value and category in the run's book b (a plugin kind is Materials or
// Manufactured, item.Book.Category). Silver is not a good: it settles
// every trade.
func Wants(b item.Book, k TraderKind, it item.Kind) bool {
	if k >= NumKinds || b.Value(it) <= 0 || it == item.Silver || it == item.MinifiedBuilding { // AX-1: crates are only sold to the colony
		return false
	}
	c := b.Category(it)
	switch k {
	case TribalSettlement, IndustrialSettlement, SpacerSettlement:
		return true
	case BulkHauler:
		return c == "Food" || c == "Materials" || it == item.HerbalMedicine || it == item.IndustrialMedicine || it == item.Components
	case Quartermaster:
		return c == "Weapons" || c == "Apparel" || c == "Medicine" && !it.IsDrug() || it == item.Components || it == item.Steel
	default:
		return c == "Manufactured" || c == "Medicine" || it == item.Gold || it == item.Wool
	}
}

// TradePriceImprovementCenti is the published negotiator adjustment.
// Settlement TPI is added after the normal (potentially inspired) cap.
func TradePriceImprovementCenti(t Terms) int {
	base := mathx.Clamp(t.Social, 0, 20) * SocialStepCenti
	if t.Inspired {
		base += 1800
	}
	return min(base, MaxTradeImprovementCenti) + mathx.Clamp(t.ImproveCenti, 0, 200)
}

// Price is one unit of it on side s with a trader of kind k under terms
// t, in hundredths of a silver. Silver is always at face value (100). A
// hostile faction, or a kind without a market value, has no price (0).
func Price(b item.Book, it item.Kind, k TraderKind, s Side, t Terms) int {
	return PriceOf(b, Goods{Kind: it}, k, s, t)
}

// PriceOf quotes an item (including a crated building) at its market value.
func PriceOf(b item.Book, g Goods, k TraderKind, side Side, t Terms) int {
	return PriceMarket(g, g.Value(b), k, side, t)
}

// PriceMarket uses the public 140% buy and 60% sell coefficients.
// All intermediates remain centi-silver; callers settle the signed net.
func PriceMarket(g Goods, market int, _ TraderKind, side Side, t Terms) int {
	if g.Kind == item.Silver {
		return 100
	}
	if t.Hostile {
		return 0
	}
	if market < 0 {
		return -1
	}
	if market == 0 {
		return 0
	}
	tpi := TradePriceImprovementCenti(t)
	baseBuy, ok := addCenti(0, market, BuyBasePct)
	if !ok {
		return -1
	}
	product, ok := addCenti(0, baseBuy, 10000-tpi)
	if !ok {
		return -1
	}
	buy := product / 10000
	if side == Buy {
		return buy
	}
	baseSell, ok := addCenti(0, market, SellBasePct)
	if !ok {
		return -1
	}
	product, ok = addCenti(0, baseSell, 10000+tpi)
	if !ok {
		return -1
	}
	sell := product / 10000
	switch {
	case g.Kind.IsWeapon():
		sell /= 5
	case g.Kind == item.MinifiedBuilding:
		switch construction.Kind(g.Building) {
		case construction.SmallSculpture, construction.LargeSculpture, construction.GrandSculpture:
			product, ok = addCenti(0, sell, 110)
		default:
			product, ok = addCenti(0, sell, 70)
		}
		if !ok {
			return -1
		}
		sell = product / 100
	}
	return min(sell, buy)
}

// RoundedSilver converts a signed full-offer net to nearest whole silver.
func RoundedSilver(centi int) int {
	n, rem := centi/100, centi%100
	if rem >= 50 {
		n++
	}
	if rem <= -50 {
		n--
	}
	return n
}

// addCenti checks an accumulated quote before multiplying a quantity by
// its price. Both input and subtotal must be nonnegative.
func addCenti(total, qty, unit int) (int, bool) {
	const limit = int(^uint(0) >> 1)
	if total < 0 || qty < 0 || unit < 0 || unit != 0 && qty > (limit-total)/unit {
		return 0, false
	}
	return total + qty*unit, true
}

// Settle checks offer against the colony's counts (colony), the trader's
// stock and silver, and terms t, and returns what moves. A gift moves only
// the colony's goods (and silver) to the trader, for goodwill; no silver
// comes back.
func Settle(b item.Book, offer []Delta, colony map[item.Kind]int, st Stock, t Terms, gift bool) Result {
	if t.Hostile {
		return Result{Reject: RejectHostile}
	}
	net := map[Goods]int{}
	for _, d := range offer {
		// Stack-specific goods are settled only by SettlePhysical.
		if d.StackID != 0 {
			return Result{Reject: RejectUnwanted}
		}
		if d.Qty < -MaxOfferQty || d.Qty > MaxOfferQty {
			return Result{Reject: RejectBadQty}
		}
		g := Goods{d.Kind, d.Building}
		net[g] += d.Qty // |net| <= 2*MaxOfferQty before the check: no overflow
		if net[g] < -MaxOfferQty || net[g] > MaxOfferQty {
			return Result{Reject: RejectBadQty}
		}
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
		n, ok := addCenti(stock[l.Goods()], l.Qty, 1)
		if !ok {
			return Result{Reject: RejectOverStock}
		}
		stock[l.Goods()] = n
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
			var ok bool
			buyC, ok = addCenti(buyC, q, PriceOf(b, g, st.Kind, Buy, t))
			if !ok {
				return Result{Reject: RejectOverStock}
			}
			res.Buy = append(res.Buy, Line{Kind: it, Qty: q, Building: g.Building})
		default:
			q = -q
			if g.Building != 0 || colony[it] < q { // the colony sells plain goods only
				return Result{Reject: RejectOverStock}
			}
			if gift {
				if b.Value(it) <= 0 {
					return Result{Reject: RejectUnwanted}
				}
			} else if !Wants(b, st.Kind, it) {
				return Result{Reject: RejectUnwanted}
			}
			var ok bool
			sellC, ok = addCenti(sellC, q, Price(b, it, st.Kind, Sell, t))
			if !ok {
				return Result{Reject: RejectOverStock}
			}
			res.Sell = append(res.Sell, Line{Kind: it, Qty: q})
		}
	}
	if gift {
		transferred := make([]Delta, len(res.Sell))
		for i, l := range res.Sell {
			transferred[i] = Delta{Kind: l.Kind, Building: l.Building, Qty: -l.Qty}
		}
		value, ok := checkedGiftValue(b, transferred, t)
		if !ok {
			return Result{Reject: RejectOverStock}
		}
		res.Goodwill = faction.GiftGoodwill(
			faction.Faction{Type: t.FactionType, Goodwill: t.Goodwill}, value)
		return res
	}
	res.SilverCenti = sellC - buyC
	res.Silver = RoundedSilver(res.SilverCenti)
	if res.Silver < 0 && colony[item.Silver] < -res.Silver ||
		res.Silver > 0 && st.Silver < res.Silver {
		return Result{Reject: RejectUnaffordable}
	}
	return res
}

// GiftValue returns the value of outgoing goods for diplomacy, after
// delivery-method and Social selling factors. Overflow rejects the offer.
func GiftValue(b item.Book, offer []Delta, t Terms) int {
	value, _ := checkedGiftValue(b, offer, t)
	return value
}

func checkedGiftValue(b item.Book, offer []Delta, t Terms) (int, bool) {
	method := GiftVisitor
	if t.ImproveCenti == 200 {
		method = GiftSettlement
	}
	centi := 0
	for _, d := range offer {
		if d.Qty < -MaxOfferQty || d.Qty > MaxOfferQty {
			return 0, false
		}
		if d.Qty >= 0 {
			continue
		}
		unit := GiftGoodsCenti(b, Goods{Kind: d.Kind, Building: d.Building}, method, t.Social)
		var ok bool
		if unit < 0 {
			return 0, false
		}
		centi, ok = addCenti(centi, -d.Qty, unit)
		if !ok {
			return 0, false
		}
	}
	return centi / 100, true
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

// Deliveries contains purchased goods and positive settlement silver, in
// the native drop order. It never includes an outgoing silver payment.
func (r Result) Deliveries() []Line {
	if r.Silver <= 0 {
		return r.Buy
	}
	return append([]Line{{Kind: item.Silver, Qty: r.Silver}}, r.Buy...)
}
