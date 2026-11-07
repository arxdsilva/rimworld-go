package quest

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
)

// Reward building behind PickRewards (QST-080..086). Every number here is
// ours unless a row says otherwise.
const (
	// randomCap: random lines stop at 5,500 of value; the rest of the
	// budget is filler (QST-084).
	randomCap = 5500
	// maxRandomLines is the most random lines an items option aims for;
	// maxTries bounds the pool draws, so the draw count stays small.
	maxRandomLines = 4
	maxTries       = 8
	// One option's kind is picked by these weights (ours); three options
	// take items, goodwill and pawn in that order, as far as allowed, and
	// items again for the rest.
	weightItems    = 60
	weightGoodwill = 25
	weightPawn     = 15
)

// optKind is what one option pays.
type optKind uint8

const (
	optItems optKind = iota
	optGoodwill
	optPawn
)

// pickOptions builds n options from in. A budget below 1 is 1, so an
// items option always holds at least one filler unit.
func pickOptions(in RewardIn, n int, roll func(n int) int) []Reward {
	budget := max(in.Budget, 1)
	kinds := optionKinds(in, n, roll)
	out := make([]Reward, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, option(in, budget, k, roll))
	}
	return out
}

// optionKinds is the kind of each of the n options.
func optionKinds(in RewardIn, n int, roll func(n int) int) []optKind {
	avail := []optKind{optItems}
	if in.Goodwill {
		avail = append(avail, optGoodwill)
	}
	if in.Pawn != 0 {
		avail = append(avail, optPawn)
	}
	if n == 1 {
		return []optKind{pickKind(avail, roll)}
	}
	out := make([]optKind, n)
	for i := range out {
		out[i] = avail[i%len(avail)]
	}
	return out
}

// pickKind picks one of avail by weight; no draw when only items are
// allowed.
func pickKind(avail []optKind, roll func(n int) int) optKind {
	if len(avail) == 1 {
		return avail[0]
	}
	total := 0
	for _, k := range avail {
		total += kindWeight(k)
	}
	r := roll(total)
	for _, k := range avail {
		if r < kindWeight(k) {
			return k
		}
		r -= kindWeight(k)
	}
	return avail[len(avail)-1]
}

func kindWeight(k optKind) int {
	switch k {
	case optGoodwill:
		return weightGoodwill
	case optPawn:
		return weightPawn
	}
	return weightItems
}

// option builds one option of kind k. Value is the items' market value;
// a goodwill or pawn option shows 0 (the caller may show a pawn's value).
func option(in RewardIn, budget int, k optKind, roll func(n int) int) Reward {
	switch k {
	case optGoodwill:
		return Reward{
			Goodwill: GoodwillFor(budget),
			SiteLoot: in.Def.GoodwillPlusItems && in.SiteLoot,
		}
	case optPawn:
		return Reward{Pawn: in.Pawn}
	}
	lines := itemLines(in.Pool, budget, roll)
	return Reward{Lines: lines, Value: Value(lines)}
}

// itemLines draws random pool lines up to min(budget, randomCap), each a
// 40-80 % share of what is left (the last takes all of it), then pays the
// rest in one filler kind, rounded up: the total is at least the budget
// and at most one filler unit over it.
func itemLines(pool []PoolItem, budget int, roll func(n int) int) []trade.Line {
	var lines []trade.Line
	random, got, made := min(budget, randomCap), 0, 0
	if len(pool) > 0 {
		want := 1 + roll(maxRandomLines)
		for try := 0; try < maxTries && made < want; try++ {
			p := pool[roll(len(pool))]
			v, left := p.Kind.Value(), random-got
			if v <= 0 || v > left {
				continue
			}
			target := left
			if made < want-1 {
				target = left * (40 + roll(41)) / 100
			}
			limit := p.Max
			if limit <= 0 {
				limit = p.Kind.StackLimit()
			}
			qty := min(max(target/v, 1), limit)
			lines = addLine(lines, 0, p.Kind, qty, limit)
			got += qty * v
			made++
		}
	}
	if rest := budget - got; rest > 0 {
		fillers := [...]item.Kind{item.Gold, item.Plasteel, item.Uranium}
		f := fillers[roll(len(fillers))]
		v := f.Value()
		lines = addLine(lines, len(lines), f, (rest+v-1)/v, f.StackLimit())
	}
	return lines
}

// addLine adds qty of k to lines: it tops up a line of k at or after
// index from up to limit, then appends lines of at most limit each.
func addLine(lines []trade.Line, from int, k item.Kind, qty, limit int) []trade.Line {
	for i := from; i < len(lines) && qty > 0; i++ {
		if lines[i].Kind != k || lines[i].Building != 0 || lines[i].Qty >= limit {
			continue
		}
		add := min(limit-lines[i].Qty, qty)
		lines[i].Qty += add
		qty -= add
	}
	for qty > 0 {
		n := min(qty, limit)
		lines = append(lines, trade.Line{Kind: k, Qty: n})
		qty -= n
	}
	return lines
}
