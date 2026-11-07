package quest

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/clock"
)

const window = uint64(WindowDays) * day

func TestCadence_OneOfferPerWindow(t *testing.T) {
	r := rand.New(rand.NewPCG(20, 1))
	const windows = 400
	for run := range 200 {
		prev := uint64(0)
		for w := range windows {
			roll := r.Uint64N(window)
			switch run {
			case 0: // every offer as late as it can be
				roll = window - 1
			case 1: // alternate earliest and latest: the widest gaps
				roll = uint64(w%2) * (window - 1)
			}
			at := OfferTick(w, roll)
			if WindowOf(at) != w {
				t.Fatalf("window %d roll %d: offer at %d is in window %d", w, roll, at, WindowOf(at))
			}
			if at < MinDay*day {
				t.Fatalf("offer at day %d, before day %d", at/day, MinDay)
			}
			if w > 0 {
				gap := at - prev
				// 19.9 days at the reference's one-decimal rounding: the
				// gap in tenths of a day, floored, is at most 199.
				if gap*10/day > 199 {
					t.Fatalf("gap %d ticks (%.3f d) between windows %d and %d", gap, float64(gap)/float64(day), w-1, w)
				}
			}
			prev = at
		}
	}
	// The widest gap reaches 19.9 days.
	if gap := OfferTick(2, window-1) - OfferTick(1, 0); gap*10/day != 199 {
		t.Fatalf("widest gap %d ticks, want 19.9 d", gap)
	}
}

func TestCadence_EdgeRolls(t *testing.T) {
	if got := OfferTick(0, 0); got != MinDay*day {
		t.Fatalf("window 0 roll 0 = %d, want day %d", got, MinDay)
	}
	if OfferTick(-4, 0) != OfferTick(0, 0) {
		t.Fatal("a negative window is window 0")
	}
	if OfferTick(3, window+5) != OfferTick(3, 5) {
		t.Fatal("the roll wraps at the window")
	}
	if WindowOf(0) != 0 || WindowOf(window-1) != 0 || WindowOf(window) != 1 {
		t.Fatal("WindowOf edges")
	}
}

func TestRollStars(t *testing.T) {
	var got [4]int
	for roll := range 100 {
		got[RollStars(roll)]++
	}
	if got != [4]int{0, 50, 35, 15} {
		t.Fatalf("stars over 100 rolls = %v, want 50/35/15", got[1:])
	}
	if RollStars(-1) != 1 || RollStars(100) != 3 {
		t.Fatal("rolls out of range")
	}
}

func TestThreatPoints(t *testing.T) {
	cases := []struct {
		points   int
		stars    uint8
		permille int
		want     int
	}{
		{500, 1, 1000, 500},
		{500, 2, 1000, 1000},
		{500, 3, 1300, 1950},
		{500, 1, 700, 350},
		{20000, 3, 1300, 78000}, // no cap (QST-029)
		{35, 1, 700, 24},        // the floor is the caller's; variance applies after it
		{500, 0, 1000, 500},     // stars clamp to 1..3
		{500, 9, 1000, 1500},
		{500, 1, 100, 350}, // permille clamps to [700, 1300]
		{500, 1, 5000, 650},
		{-5, 2, 1000, 0},
	}
	for _, c := range cases {
		if got := ThreatPoints(c.points, c.stars, c.permille); got != c.want {
			t.Errorf("ThreatPoints(%d, %d, %d) = %d, want %d", c.points, c.stars, c.permille, got, c.want)
		}
	}
}

func TestJoinerWeight(t *testing.T) {
	anchors := map[int]int{0: 8000, 1: 2000, 5: 1000, 9: 400, 12: 0, 3: 1500}
	for n, want := range anchors {
		if got := JoinerWeight(n); got != want {
			t.Errorf("JoinerWeight(%d) = %d, want %d", n, got, want)
		}
	}
	prev := JoinerWeight(-1)
	for n := 0; n <= 40; n++ {
		w := JoinerWeight(n)
		if w > prev {
			t.Fatalf("JoinerWeight(%d) = %d rose from %d", n, w, prev)
		}
		if n >= 12 && w != 0 {
			t.Fatalf("JoinerWeight(%d) = %d, want 0 at 12 or more", n, w)
		}
		prev = w
	}
}

func TestRewardBudget(t *testing.T) {
	cases := []struct{ points, scale, pct, want int }{
		{500, 100, 100, 1100}, // shot Q24's +22 goodwill
		{500, 200, 100, 550},  // twice the threat scale, half the budget
		{500, 100, 200, 2200},
		{500, 0, 100, 11000}, // threat scale floors at 10 %
		{500, 5, 100, 11000},
		{500, 100, 0, 0},
		{0, 100, 100, 0},
		{-50, 100, 100, 0},
	}
	for _, c := range cases {
		if got := RewardBudget(c.points, c.scale, c.pct); got != c.want {
			t.Errorf("RewardBudget(%d, %d, %d) = %d, want %d", c.points, c.scale, c.pct, got, c.want)
		}
	}
	if GoodwillFor(RewardBudget(500, 100, 100)) != 22 {
		t.Fatal("500 points must pay +22 goodwill (Q24)")
	}
}

func TestGoodwillFor(t *testing.T) {
	cases := map[int]int{-10: 1, 0: 1, 49: 1, 50: 1, 100: 2, 1100: 22, 1250: 25, 99999: 25}
	for b, want := range cases {
		if got := GoodwillFor(b); got != want {
			t.Errorf("GoodwillFor(%d) = %d, want %d", b, got, want)
		}
	}
}

func TestValue(t *testing.T) {
	lines := []trade.Line{{Kind: item.Gold, Qty: 3}, {Kind: item.Steel, Qty: 10}}
	if got, want := Value(lines), 3*item.Gold.Value()+10*item.Steel.Value(); got != want {
		t.Fatalf("Value = %d, want %d", got, want)
	}
	if Value(nil) != 0 {
		t.Fatal("no lines are worth 0")
	}
}

// testPool is a mixed pool: cheap, dear, capped, and a worthless kind.
func testPool() []PoolItem {
	return []PoolItem{
		{Kind: item.Steel}, {Kind: item.Components}, {Kind: item.IndustrialMedicine, Max: 7},
		{Kind: item.Silver}, {Kind: item.AdvancedComponents, Max: 3}, {Kind: item.Kind(255)},
	}
}

func isFiller(k item.Kind) bool { return k == item.Gold || k == item.Plasteel || k == item.Uranium }

func TestPickRewards_Invariants(t *testing.T) {
	r := rand.New(rand.NewPCG(80, 86))
	roll := func(n int) int { return r.IntN(n) }
	triples, choiceRuns := 0, 0
	for i := range 20_000 {
		k := KindID(1 + i%(int(NumKinds)-1))
		in := RewardIn{
			Budget:   r.IntN(60_000) - 100,
			Def:      DefOf(k),
			Goodwill: r.IntN(2) == 0,
			SiteLoot: r.IntN(2) == 0,
		}
		if r.IntN(3) > 0 {
			in.Pool = testPool()
		}
		if r.IntN(2) == 0 {
			in.Pawn = r.Uint64() | 1
		}
		opts := PickRewards(in, roll)
		if in.Def.Choice {
			choiceRuns++
			if len(opts) == 3 {
				triples++
			}
		}
		checkOptions(t, in, opts)
	}
	// Choice kinds give three options half the time.
	if p := triples * 100 / choiceRuns; p < 47 || p > 53 {
		t.Fatalf("three options %d %% of choice runs, want ~50", p)
	}
}

func checkOptions(t *testing.T, in RewardIn, opts []Reward) {
	t.Helper()
	if len(opts) != 1 && len(opts) != 3 || len(opts) == 3 && !in.Def.Choice {
		t.Fatalf("%d options for kind %d (choice %v)", len(opts), in.Def.Kind, in.Def.Choice)
	}
	budget := max(in.Budget, 1)
	for _, o := range opts {
		if len(o.Lines) == 0 && o.Goodwill == 0 && o.Pawn == 0 && !o.SiteLoot {
			t.Fatalf("empty option %+v", o)
		}
		if o.Pawn != 0 && (o.Pawn != in.Pawn || len(o.Lines) > 0 || o.Goodwill != 0) {
			t.Fatalf("pawn option %+v with pawn seed %d", o, in.Pawn)
		}
		if o.Goodwill != 0 {
			if !in.Goodwill || o.Goodwill != GoodwillFor(budget) {
				t.Fatalf("goodwill %d (allowed %v, budget %d)", o.Goodwill, in.Goodwill, budget)
			}
			if !o.GoodwillOnly() && (!in.Def.GoodwillPlusItems || len(o.Lines) > 0 || o.Pawn != 0) {
				t.Fatalf("goodwill option carries more than goodwill: %+v", o)
			}
			if o.SiteLoot != (in.Def.GoodwillPlusItems && in.SiteLoot) {
				t.Fatalf("goodwill option SiteLoot %v for kind %d", o.SiteLoot, in.Def.Kind)
			}
			continue
		}
		if o.SiteLoot {
			t.Fatalf("site loot outside a goodwill option: %+v", o)
		}
		if o.Pawn != 0 {
			continue
		}
		if o.Value != Value(o.Lines) || o.Value*10 < budget*9 {
			t.Fatalf("items worth %d (shown %d) for budget %d", Value(o.Lines), o.Value, budget)
		}
		if o.Value > budget+item.Gold.Value() {
			t.Fatalf("items worth %d overshoot budget %d by more than one filler unit", o.Value, budget)
		}
		for _, l := range o.Lines {
			if l.Qty <= 0 || l.Qty > l.Kind.StackLimit() || l.Kind.Value() <= 0 {
				t.Fatalf("bad line %+v", l)
			}
		}
	}
}

func TestItemLines_RandomPartStopsAt5500(t *testing.T) {
	r := rand.New(rand.NewPCG(84, 84))
	roll := func(n int) int { return r.IntN(n) }
	pool := testPool()
	for range 5000 {
		budget := 1 + r.IntN(100_000)
		lines := itemLines(pool, budget, roll)
		random := 0
		for _, l := range lines {
			if !isFiller(l.Kind) {
				random += l.Kind.Value() * l.Qty
			}
			for _, p := range pool {
				if p.Kind == l.Kind && p.Max > 0 && l.Qty > p.Max {
					t.Fatalf("line %+v over its pool max %d", l, p.Max)
				}
			}
		}
		if random > min(budget, randomCap) {
			t.Fatalf("random lines worth %d for budget %d, want ≤ %d", random, budget, min(budget, randomCap))
		}
		if v := Value(lines); v < budget {
			t.Fatalf("lines worth %d below budget %d", v, budget)
		}
	}
}

func TestPickRewards_OneOptionKinds(t *testing.T) {
	in := RewardIn{Budget: 1000, Def: DefOf(Stash), Pool: testPool(), Goodwill: true, Pawn: 9}
	seen := map[string]bool{}
	for roll := range 100 {
		rolls := []int{roll}
		opts := PickRewards(in, func(n int) int {
			if len(rolls) > 0 {
				v := rolls[0] % n
				rolls = rolls[1:]
				return v
			}
			return 0
		})
		switch o := opts[0]; {
		case o.Pawn != 0:
			seen["pawn"] = true
		case o.Goodwill != 0:
			seen["goodwill"] = true
		default:
			seen["items"] = true
		}
	}
	if len(seen) != 3 {
		t.Fatalf("one-option kinds seen %v, want items, goodwill and pawn", seen)
	}
	// Three options take items, goodwill and pawn, in that order.
	in.Def = DefOf(Camp)
	in.SiteLoot = true
	opts := PickRewards(in, func(n int) int { return n - 1 })
	if len(opts) != 3 || len(opts[0].Lines) == 0 || opts[1].Goodwill == 0 || !opts[1].SiteLoot || opts[2].Pawn != 9 {
		t.Fatalf("camp's three options = %+v", opts)
	}
	// Only items allowed: no draw for the kind, and still an option.
	in = RewardIn{Budget: 0, Def: DefOf(Talks)}
	opts = PickRewards(in, func(int) int { return 0 })
	if len(opts) != 1 || len(opts[0].Lines) == 0 {
		t.Fatalf("zero budget options = %+v", opts)
	}
}

func TestAddLine_SplitsAtLimit(t *testing.T) {
	lines := addLine(nil, 0, item.Gold, 2500, 1000)
	if len(lines) != 3 || lines[2].Qty != 500 {
		t.Fatalf("lines = %+v", lines)
	}
	lines = addLine(lines, 0, item.Gold, 700, 1000)
	if len(lines) != 4 || lines[2].Qty != 1000 || lines[3].Qty != 200 {
		t.Fatalf("top-up = %+v", lines)
	}
	// From skips earlier lines.
	lines = addLine([]trade.Line{{Kind: item.Gold, Qty: 1}}, 1, item.Gold, 5, 1000)
	if len(lines) != 2 || lines[0].Qty != 1 {
		t.Fatalf("from = %+v", lines)
	}
}

// offer is one generated offer in the determinism run.
type offer struct {
	At      uint64
	Kind    KindID
	Stars   uint8
	Points  int
	Rewards []Reward
}

// generate runs the pure generator over days: one offer per window, a kind
// picked by weight (joining kinds scaled by colony size), stars, points,
// a budget and rewards, all from one seeded stream.
func generate(seed uint64, days int, colonists int) []offer {
	r := rand.New(rand.NewPCG(seed, 39))
	roll := func(n int) int { return r.IntN(n) }
	defs := Defs()
	var out []offer
	for w := 0; OfferTick(w, 0) < uint64(days)*day; w++ {
		at := OfferTick(w, r.Uint64N(window))
		total := 0
		weights := make([]int, len(defs))
		for i, d := range defs {
			weights[i] = d.Weight
			if d.Joins {
				weights[i] = d.Weight * JoinerWeight(colonists) / 1000
			}
			total += weights[i]
		}
		pick := roll(total)
		k := KindNone
		for i, wt := range weights {
			if pick < wt {
				k = KindID(i)
				break
			}
			pick -= wt
		}
		stars := RollStars(roll(100))
		points := ThreatPoints(400+w*50, stars, 700+roll(601))
		in := RewardIn{
			Budget: RewardBudget(points, 100, 100), Def: defs[k], Pool: testPool(),
			Goodwill: true, Pawn: r.Uint64() | 1,
		}
		out = append(out, offer{at, k, stars, points, PickRewards(in, roll)})
	}
	return out
}

func TestGenerator_TwoYearsDeterministic(t *testing.T) {
	days := 2 * clock.DaysPerYear
	a, b := generate(7, days, 4), generate(7, days, 4)
	if len(a) != days/WindowDays {
		t.Fatalf("%d offers in %d days, want %d", len(a), days, days/WindowDays)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed and inputs gave a different sequence")
	}
	if reflect.DeepEqual(a, generate(8, days, 4)) {
		t.Fatal("a different seed gave the same sequence")
	}
	for _, o := range generate(7, days, 12) {
		if DefOf(o.Kind).Joins {
			t.Fatalf("joining kind %d offered at 12 colonists", o.Kind)
		}
		if o.Kind == OreLump || o.Kind == KindNone {
			t.Fatalf("kind %d offered by the cadence", o.Kind)
		}
	}
}
