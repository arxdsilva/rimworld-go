package travel

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/economy/trade"
)

// Goods is what caravan cid carries, a copy (MAP-08: what a demand asks for).
func (o Ops) Goods(cid uint64) []item.Stack {
	if h := o.s.cargo[cid]; h != nil {
		return slices.Clone(h.Items)
	}
	return nil
}

// Yield gives qty of k from caravan cid's goods up: false when it has less.
func (o Ops) Yield(cid uint64, k item.Kind, qty int) bool {
	h := o.s.cargo[cid]
	if h == nil {
		return false
	}
	have := 0
	for _, st := range h.Items {
		if st.Kind == k {
			have += st.Qty
		}
	}
	if have < qty {
		return false
	}
	for i := 0; i < len(h.Items) && qty > 0; i++ {
		if h.Items[i].Kind != k {
			continue
		}
		n := min(qty, h.Items[i].Qty)
		h.Items[i].Qty -= n
		qty -= n
		if h.Items[i].Qty == 0 {
			h.Items = slices.Delete(h.Items, i, i+1)
			i--
		}
	}
	o.h.Changed()
	return true
}

// CarryLines adds lines to caravan cid's goods (a quest reward, QST-090).
func (o Ops) CarryLines(cid uint64, lines []trade.Line) {
	for _, l := range lines {
		o.Carry(cid, item.Stack{Kind: l.Kind, Qty: l.Qty})
	}
}

// Deliverable: stack st counts toward a request (QST-071): normal quality
// or better when it has one, and not tainted; wear does not matter.
func Deliverable(st item.Stack) bool {
	l, ok := st.QualityLevel()
	return !st.Tainted() && (!ok || l >= quality.Normal)
}

// HasLines: caravan cid carries every line in deliverable goods (a
// request, QST-2g).
func (o Ops) HasLines(cid uint64, lines []trade.Line) bool {
	h := o.s.cargo[cid]
	if h == nil {
		return len(lines) == 0
	}
	have := map[item.Kind]int{}
	for _, st := range h.Items {
		if Deliverable(st) {
			have[st.Kind] += st.Qty
		}
	}
	for _, l := range lines {
		if have[l.Kind] -= l.Qty; have[l.Kind] < 0 {
			return false
		}
	}
	return true
}

// TakeLines gives every line up from deliverable goods, or nothing when
// one is short.
func (o Ops) TakeLines(cid uint64, lines []trade.Line) bool {
	if !o.HasLines(cid, lines) {
		return false
	}
	h := o.s.cargo[cid]
	for _, l := range lines {
		qty := l.Qty
		for i := 0; i < len(h.Items) && qty > 0; i++ {
			if h.Items[i].Kind != l.Kind || !Deliverable(h.Items[i]) {
				continue
			}
			n := min(qty, h.Items[i].Qty)
			h.Items[i].Qty -= n
			qty -= n
			if h.Items[i].Qty == 0 {
				h.Items = slices.Delete(h.Items, i, i+1)
				i--
			}
		}
	}
	o.h.Changed()
	return true
}
