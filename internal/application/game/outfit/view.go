package outfit

import (
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Views lists the policies as the Assign window shows them (shared: read
// only; worked out once per edit, the views read it every frame).
func (b *Book) Views() []gearview.Policy {
	if b.views != nil {
		return b.views
	}
	for _, p := range b.Policies() {
		v := gearview.Policy{ID: uint32(p.ID), Name: p.Name, NoTainted: p.Special.NoTainted, Default: p.ID == b.def}
		for _, n := range zone.Tree() {
			if n.Leaf && apparel.Has(n.Kind) && p.Allow.Has(n.Kind) {
				v.Allowed = append(v.Allowed, n.Kind.String())
			}
		}
		for _, k := range Materials() {
			if !p.NoStuff.Has(k) {
				v.Stuffs = append(v.Stuffs, k.String())
			}
		}
		v.HPMin, v.HPMax = p.Special.HPRange()
		lo, hi := p.Special.QualityRange()
		v.QualityMin, v.QualityMax = uint8(lo), uint8(hi)
		b.views = append(b.views, v)
	}
	return b.views
}

// View adds colonist id's policy to v (PawnView.Gear).
func (b *Book) View(id entity.EntityID, v gearview.Pawn) gearview.Pawn {
	p := b.Of(id)
	v.ApparelPolicy, v.ApparelPolicyName = uint32(p.ID), p.Name
	return v
}

// Materials are the materials garments are made of, in kind order (the
// editor's material list).
func Materials() []item.Kind {
	var cats item.StuffCategory
	for _, n := range zone.Tree() {
		if n.Leaf && apparel.Has(n.Kind) {
			cats |= item.MadeFrom(n.Kind)
		}
	}
	return item.Stuffs(cats)
}
