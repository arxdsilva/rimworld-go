package travel

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// Caravan packing (wiki: Caravan): a forming caravan takes goods and
// tame pack animals. Everyone carries body size x 35 kg (a person 35 kg,
// an animal by its wiki body size); goods past that are left behind at
// the form. Haulers (and the members while they wait) carry the goods to
// the gathering spot and lead the animals there, where they are tied
// (job.LoadPods with BillIndex = the caravan). The goods ride as the
// caravan's cargo and come home with it; the animals leave the map with
// it. A heavy caravan is slower: the wiki's carried-mass factor runs
// from x2 empty to x0.5 at full load, so relative to an empty caravan
// (our base speed) it falls to a quarter at full load, linearly between
// (the wiki gives only the ends).

// Pack is a forming caravan's packing: goods still to load, animals
// still to lead in, and those already tied at the spot.
type Pack struct {
	Items   []Want
	Animals []entity.EntityID
	Tied    []entity.EntityID
}

// caravanKg is what a caravan of members people and animals can carry
// (wiki: body size x 35 kg).
func caravanKg(members int, animals []animal.Species) int {
	kg := members * caravan.PersonCarryKg
	for _, s := range animals {
		kg += caravan.PackKg(animal.BodySize(s))
	}
	return kg
}

// Pack registers forming caravan cid's goods and animals from list (the
// FormCaravan entries: "id" for an animal, "stack:qty" for goods; the
// members, people, are the game's), trimmed to what members people and
// the animals can carry (a message names what is left behind).
func (o Ops) Pack(cid uint64, list string, members int) {
	pk := &Pack{}
	var species []animal.Species
	tame := o.tame()
	var goods []item.Stack
	for _, f := range strings.Split(list, ",") {
		idStr, qtyStr, hasQty := strings.Cut(strings.TrimSpace(f), ":")
		n, err := strconv.ParseUint(idStr, 10, 64)
		id := entity.EntityID(n)
		switch {
		case err != nil:
		case hasQty:
			if st, ok := o.h.Stack(id); ok {
				if q, err := strconv.Atoi(qtyStr); err == nil && q > 0 {
					goods = append(goods, item.Stack{Kind: st.Kind, Qty: min(q, st.Qty)})
				}
			}
		case slices.Contains(tame, id) && !slices.Contains(pk.Animals, id):
			a, _ := o.h.Animal(id)
			pk.Animals = append(pk.Animals, id)
			species = append(species, a.Species)
		}
	}
	room, left := caravanKg(members, species)*1000, 0
	for _, st := range goods {
		g := st.Qty * st.Kind.MassGrams()
		if g > room {
			left++
			continue
		}
		room -= g
		k := slices.IndexFunc(pk.Items, func(w Want) bool { return w.Kind == st.Kind })
		if k < 0 {
			pk.Items = append(pk.Items, Want{Kind: st.Kind, Qty: st.Qty})
		} else {
			pk.Items[k].Qty += st.Qty
		}
	}
	if len(pk.Items) == 0 && len(pk.Animals) == 0 {
		return
	}
	if o.s.packs == nil {
		o.s.packs = map[uint64]*Pack{}
	}
	o.s.packs[cid] = pk
	if left > 0 {
		x, y := o.h.Home()
		o.h.Message(x, y, fmt.Sprintf("Too heavy for the caravan: %d picks were left behind.", left))
	}
}

// loaded reports caravan cid's packing done (or none).
func (o packing) loaded(cid uint64) bool {
	pk := o.s.packs[cid]
	return pk == nil || len(pk.Items) == 0 && len(pk.Animals) == 0
}

// finish ties caravan cid's packing off as it sets out: the tied animals
// leave the map with it (its cargo), the rest is dropped.
func (o packing) finish(cid uint64) {
	pk := o.s.packs[cid]
	if pk == nil {
		return
	}
	delete(o.s.packs, cid)
	for _, a := range pk.Tied {
		if st, ok := o.takeAnimal(a); ok {
			h := o.s.cargoOf(cid)
			h.Animals = append(h.Animals, Beast{ID: a, State: st})
		}
	}
}

// Pace is caravan cid's speed factor (permille) from its cargo against
// what members people and its animals carry: 1000 empty, 250 full.
func (o Ops) Pace(cid uint64, members int) int {
	h := o.s.cargo[cid]
	if h == nil {
		return 1000
	}
	var species []animal.Species
	for _, b := range h.Animals {
		species = append(species, b.State.Species)
	}
	g := 0
	for _, st := range h.Items {
		g += st.Qty * st.Kind.MassGrams()
	}
	c := max(1, caravanKg(members, species)*1000)
	return max(250, 1000-750*min(g, c)/c)
}

// spotOf is caravan cid's gathering spot (ok false: not gathering).
func (o packing) spotOf(cid uint64) (int, int, bool) {
	g := o.s.hitch[cid]
	if g == nil {
		return 0, 0, false
	}
	return g.X, g.Y, true
}

// inflight is what haulers are bringing caravan cid now, per item kind,
// and the animals being led.
func (o packing) inflight(cid uint64) (map[item.Kind]int, []entity.EntityID) {
	by := map[item.Kind]int{}
	var led []entity.EntityID
	for _, pid := range o.h.PawnIDs() {
		j := o.job(pid)
		if j.Kind != job.LoadPods || !j.Active() || j.Dest != 0 || uint64(j.BillIndex) != cid {
			continue
		}
		if j.Count == 0 {
			led = append(led, j.Target)
			continue
		}
		sid := j.Carry
		if sid == 0 {
			sid = j.Target
		}
		if st, ok := o.h.Stack(sid); ok {
			by[st.Kind] += j.Count
		}
	}
	return by, led
}

// assign starts id on the first thing caravan cid still waits for.
func (o packing) assign(id entity.EntityID, cid uint64) bool {
	pk := o.s.packs[cid]
	x, y, ok := o.spotOf(cid)
	if pk == nil || !ok {
		return false
	}
	by, led := o.inflight(cid)
	for _, w := range pk.Items {
		n := min(w.Qty-by[w.Kind], item.CarryLimit)
		if n <= 0 {
			continue
		}
		if sid, sx, sy, ok := o.h.Stock(id, w.Kind, n); ok &&
			o.h.StartJob(id, job.Record{Kind: job.LoadPods, Target: sid, TX: sx, TY: sy, DestX: x, DestY: y, Count: n, BillIndex: int(cid)}) {
			return true
		}
	}
	for _, a := range pk.Animals {
		if slices.Contains(led, a) || slices.ContainsFunc(sortedKeys(o.s.roped), func(h entity.EntityID) bool { return o.s.roped[h].ID == a }) {
			continue
		}
		if _, ok := o.h.Animal(a); !ok {
			continue
		}
		ax, ay := o.h.Pos(a)
		if o.h.StartJob(id, job.Record{Kind: job.LoadPods, Target: a, TX: ax, TY: ay, DestX: x, DestY: y, BillIndex: int(cid)}) {
			return true
		}
	}
	return false
}

// AssignPack starts hauler id on what a forming caravan still waits for
// (Haul work), caravans in id order.
func (o Ops) AssignPack(id entity.EntityID) bool {
	for _, cid := range sortedKeys(o.s.packs) {
		if packing(o).assign(id, cid) {
			return true
		}
	}
	return false
}

// doPack is one step of a caravan's job.LoadPods: fetch the goods or the
// animal, then set it down (or tie it) at the spot.
func (o packing) doPack(id entity.EntityID, j job.Record) {
	cid := uint64(j.BillIndex)
	pk := o.s.packs[cid]
	switch {
	case j.Count == 0 && j.Carry == 0: // reach the animal
		ax, ay := o.h.Pos(j.Target)
		px, py := o.h.Pos(id)
		if pk == nil || !slices.Contains(pk.Animals, j.Target) {
			o.end(id, j, 0)
			return
		}
		if max(abs(ax-px), abs(ay-py)) > 1 {
			if j.WorkLeft++; j.WorkLeft > 6 {
				o.end(id, j, job.Unreachable)
				return
			}
			o.walk(id, j, ax, ay)
			return
		}
		st, ok := o.takeAnimal(j.Target)
		if !ok {
			o.end(id, j, job.TargetGone)
			return
		}
		o.s.rope(id, Beast{ID: j.Target, State: st})
		j.Carry = j.Target
		o.walk(id, j, j.DestX, j.DestY)
	case j.Count == 0: // tie it at the spot
		b, roped := o.s.roped[id]
		delete(o.s.roped, id)
		if roped {
			x, y := o.h.Pos(id)
			o.h.PutAnimal(b.ID, b.State, x, y)
		}
		if roped && pk != nil && b.ID == j.Carry {
			pk.Animals = slices.DeleteFunc(pk.Animals, func(a entity.EntityID) bool { return a == b.ID })
			pk.Tied = append(pk.Tied, b.ID)
		}
		o.end(id, j, 0)
	case j.Carry == 0: // pick the goods up
		carry, _, n, ok := o.h.PickUp(id, j.Target, j.Count)
		if !ok || pk == nil {
			o.end(id, j, job.TargetGone)
			return
		}
		j.Carry, j.Count = carry, n
		o.walk(id, j, j.DestX, j.DestY)
	default: // load them
		st, held := o.h.Stack(j.Carry)
		if !held {
			o.end(id, j, job.TargetGone)
			return
		}
		o.h.Spend(j.Carry)
		if pk == nil || !o.h.Caravan(cid) {
			x, y := o.h.Pos(id)
			o.h.DropStack(st, x, y)
			o.end(id, j, job.TargetGone)
			return
		}
		o.s.cargoOf(cid).addItem(st)
		for i := range pk.Items {
			if pk.Items[i].Kind == st.Kind {
				pk.Items[i].Qty -= st.Qty
			}
		}
		pk.Items = slices.DeleteFunc(pk.Items, func(w Want) bool { return w.Qty <= 0 })
		o.end(id, j, 0)
	}
	o.h.Changed()
}

// packTick forgets the packing of caravans called off.
func (o packing) packTick() {
	for _, cid := range sortedKeys(o.s.packs) {
		if !o.h.Caravan(cid) {
			delete(o.s.packs, cid)
		}
	}
}
