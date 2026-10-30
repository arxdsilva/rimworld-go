package craft

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// IngredientRetry is how long a bill whose ingredients could not be found
// waits before it searches again (PRD-1: the retry cooldown, so a bill
// with nothing to gather never rescans every tick; original tuning — the
// wiki publishes none).
const IngredientRetry = 500

// pick is one stack an ingredient search takes from: how many (take),
// and, while planning, what is left of it (left) and how far it lies
// from the bench's work spot (d).
type pick struct {
	id   entity.EntityID
	kind item.Kind
	x, y int
	take int
	left int
	d    int
	st   item.Stack
}

// planner searches the colony's stacks for a bill's ingredients.
type planner struct{ h Host }

// planBuf is an ingredient search's scratch: the candidate stacks and the
// picks, kept between searches so the tick path reuses them.
type planBuf struct{ cands, picks []pick }

// fits reports a stack the single-stack search found that bill bl may
// still use: not a forbidden kind, inside the bill's radius of spot and
// in its ingredient source, if it names one (RES-1).
func (p planner) fits(bl recipe.Bill, st item.Stack, spot [2]int) bool {
	return bl.Allows(st.Kind) && (bl.Radius <= 0 || pathing.Dist(spot[0], spot[1], st.X, st.Y) <= bl.Radius) &&
		(bl.Source == 0 || p.h.InSource(bl.Source, st.X, st.Y))
}

// usable is how much of a stack pawn id may gather for bill bl (0: none):
// lying free, not forbidden or spoilt (a rotten body still burns),
// allowed by the bill, inside its radius and source; the part other
// pawns haven't claimed (RES-1: two crafters share a stack).
func (p planner) usable(id, sid entity.EntityID, st item.Stack, bl recipe.Bill, spot [2]int) int {
	if st.Carrier != 0 || st.Forbidden || st.Qty <= 0 || st.Rotten() && st.Kind != item.CorpseHuman {
		return 0
	}
	if !p.fits(bl, st, spot) {
		return 0
	}
	return p.h.Claims().FreeFor(id, sid, st.Qty)
}

// plan gathers d's ingredients for pawn id (0: anyone) from the stacks
// nearest the work spot, ties by id: each ingredient from as many stacks
// as it takes, except one that takes up to Max from a single stack (a
// drying batch). The picks come grouped by kind (one carried load per
// run), in buf until the next search; ok is false when an ingredient
// can't be had.
func (p planner) plan(id entity.EntityID, bl recipe.Bill, d recipe.Def, spot [2]int, buf *planBuf) ([]pick, bool) {
	cands, out := buf.cands[:0], buf.picks[:0]
	defer func() { buf.cands, buf.picks = cands[:0], out[:0] }()
	var inBuf [4]recipe.Input
	ins := d.AppendInputs(inBuf[:0])
	for _, sid := range p.h.ItemIDs() {
		st, ok := p.h.Stack(sid)
		if !ok || !acceptedBy(ins, st) {
			continue
		}
		left := p.usable(id, sid, st, bl, spot)
		if left <= 0 {
			continue
		}
		cands = append(cands, pick{id: sid, kind: st.Kind, x: st.X, y: st.Y, left: left,
			d: pathing.Dist(spot[0], spot[1], st.X, st.Y), st: st})
	}
	slices.SortFunc(cands, func(a, b pick) int {
		if c := cmp.Compare(a.d, b.d); c != 0 {
			return c
		}
		return cmp.Compare(a.id, b.id)
	})
	for _, in := range ins {
		var ok bool
		if out, ok = takeFor(in, cands, out); !ok {
			return out, false
		}
	}
	return groupByKind(out), true
}

// acceptedBy reports a stack one of ins takes.
func acceptedBy(ins []recipe.Input, st item.Stack) bool {
	for _, in := range ins {
		if in.AcceptsStack(st) {
			return true
		}
	}
	return false
}

// takeFor picks input in's quantity from cands (nearest first), lowering
// what is left of each so the next input doesn't count it twice.
func takeFor(in recipe.Input, cands, out []pick) ([]pick, bool) {
	only := item.Kind(0)
	if in.Stuff != 0 { // PRD-2: one material only, the nearest there is enough of
		var ok bool
		if only, ok = enoughOf(in, cands); !ok {
			return out, false
		}
	}
	need := in.Qty
	for i := range cands {
		c := &cands[i]
		if c.left <= 0 || !in.AcceptsStack(c.st) || only != 0 && c.kind != only {
			continue
		}
		if in.Max > 0 { // one stack, up to Max of it
			if c.left < in.Qty || c.left != c.st.Qty {
				continue
			}
			c.take, c.left = in.Take(c.left), 0
			return append(out, *c), true
		}
		take := min(need, c.left)
		c.left -= take
		need -= take
		pk := *c
		pk.take = take
		out = append(out, pk)
		if need == 0 {
			return out, true
		}
	}
	return out, false
}

// enoughOf is the material, nearest first, the candidates hold at least
// in.Qty of (a stuffed product is made of one material).
func enoughOf(in recipe.Input, cands []pick) (item.Kind, bool) {
	var have [256]int
	for _, c := range cands {
		if c.left > 0 && in.AcceptsStack(c.st) {
			have[c.kind] += c.left
		}
	}
	for _, c := range cands {
		if c.left > 0 && have[c.kind] >= in.Qty && in.AcceptsStack(c.st) {
			return c.kind, true
		}
	}
	return 0, false
}

// groupByKind orders picks so each kind's run is consecutive (kinds by
// first appearance), keeping the nearest-first order inside a run, and
// folds two picks of one stack (two inputs taking the same kind) into one.
func groupByKind(ps []pick) []pick {
	var buf [8]item.Kind
	kinds := buf[:0]
	for _, p := range ps {
		if !slices.Contains(kinds, p.kind) {
			kinds = append(kinds, p.kind)
		}
	}
	if len(kinds) > 1 {
		slices.SortStableFunc(ps, func(a, b pick) int {
			return cmp.Compare(slices.Index(kinds, a.kind), slices.Index(kinds, b.kind))
		})
	}
	out := ps[:0]
	for _, p := range ps {
		if i := slices.IndexFunc(out, func(o pick) bool { return o.id == p.id }); i >= 0 {
			out[i].take += p.take
			continue
		}
		out = append(out, p)
	}
	return out
}

// available reports whether bill bl at a bench with work spot spot can
// have its ingredients now (the read model's "waiting for ingredients").
// It allocates nothing for a single-input recipe: the read model asks
// for every bill on every view build.
func (p planner) available(bl recipe.Bill, d recipe.Def, spot [2]int) bool {
	if len(d.Inputs) > 0 {
		var buf planBuf
		_, ok := p.plan(0, bl, d, spot, &buf)
		return ok
	}
	in := d.Input
	if sid, st := p.h.Ingredient(0, in); sid != 0 && p.fits(bl, st, spot) {
		return true
	}
	n := 0
	var have [256]int // PRD-2: a stuffed input counts each material on its own
	for _, sid := range p.h.ItemIDs() {
		st, ok := p.h.Stack(sid)
		if !ok || !in.AcceptsStack(st) {
			continue
		}
		free := p.usable(0, sid, st, bl, spot)
		if free <= 0 {
			continue
		}
		if in.Max > 0 { // one stack, up to Max of it
			if free >= in.Qty && free == st.Qty {
				return true
			}
			continue
		}
		if in.Stuff != 0 {
			if have[st.Kind] += free; have[st.Kind] >= in.Qty {
				return true
			}
			continue
		}
		if n += free; n >= in.Qty {
			return true
		}
	}
	return false
}
