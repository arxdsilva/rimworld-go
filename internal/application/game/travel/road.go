package travel

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
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
