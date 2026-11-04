package sense

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/sim/entity"
)

// BackToPack puts a carried stack back in the pack it came from (CSE-037); false:
// it did not come from one, the pack has no room for it, or the rule is off, and
// the caller drops it as usual. Whichever way, it is no longer "from the pack":
// once it lands, it is an ordinary stack. A stack that was being put away stays
// marked, unless the player took the mark off.
func (g Gear) BackToPack(id, carry entity.EntityID) bool {
	if from, ok := g.s.FromPack[id]; !ok || from != carry {
		return false
	}
	delete(g.s.FromPack, id)
	st, ok := g.r.Stack(carry)
	if !ok || st.Carrier != id || !g.s.Rules.On(gameplay.PutBack) || st.Kind.MassGrams()*st.Qty > g.Room(id) {
		return false
	}
	j := g.r.Job(id)
	g.r.RemoveQty(carry, st.Qty)
	p := g.s.Packs[id]
	p.Items = slices.Clone(p.Items)
	entry := p.Add(st)
	if j.Kind == job.UnloadPack && j.Count == modePack {
		if i := slices.IndexFunc(p.Items, func(e inventory.Entry) bool { return e.ID == entry }); i >= 0 {
			p.Items[i].Unload = true
		}
	}
	g.s.Packs[id] = p
	return true
}
