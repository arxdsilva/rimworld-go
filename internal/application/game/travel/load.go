package travel

import (
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// Loading (wiki: Transport pod): the player picks who and what goes
// (command.LoadPods); the picked colonists walk into the pods themselves,
// haulers bring the goods and lead the animals in (job.LoadPods, Haul
// work). A pod lifts 150 kg; a pick that would overfill the group is left
// behind with a message.

// loadUpkeep is how often boarders are sent again (ticks).
const loadUpkeep = 60

// cmdLoad replaces pod Target's group manifest with c.Name's list ("id"
// a colonist or tame animal, "id:qty" an amount from a stack); Enabled
// false unloads the group instead.
func (o pods) cmdLoad(c command.Command) {
	gr, ok := o.groupOf(entity.EntityID(c.Target))
	if !ok {
		return
	}
	b, _ := o.h.Building(gr.leader)
	if !c.Enabled {
		for _, p := range gr.pods {
			o.spill(p)
		}
		delete(o.s.wants, gr.leader)
		o.h.Message(b.X, b.Y, "The transport pods were unloaded.")
		o.h.Changed()
		return
	}
	m, left := o.parseManifest(c.Name, capKg(gr)-o.loadKg(gr))
	if o.s.wants == nil {
		o.s.wants = map[entity.EntityID]*Manifest{}
	}
	o.s.wants[gr.leader] = &m
	if left > 0 {
		o.h.Message(b.X, b.Y, "Too heavy for the pods: %d picks were left behind.", left)
	}
	o.h.Changed()
}

// parseManifest reads a load list into a manifest that fits room kg,
// and how many picks did not fit.
func (o pods) parseManifest(list string, room int) (Manifest, int) {
	var m Manifest
	left, grams := 0, 0
	fits := func(g int) bool {
		if grams+g > room*1000 {
			left++
			return false
		}
		grams += g
		return true
	}
	pawns, beasts := o.h.PawnIDs(), o.tame()
	for _, f := range strings.Split(list, ",") {
		idStr, qtyStr, hasQty := strings.Cut(strings.TrimSpace(f), ":")
		n, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil || n == 0 {
			continue
		}
		id := entity.EntityID(n)
		switch {
		case hasQty:
			st, ok := o.h.Stack(id)
			q, err := strconv.Atoi(qtyStr)
			if !ok || err != nil || q <= 0 {
				continue
			}
			q = min(q, st.Qty)
			if !fits(q * st.Kind.MassGrams()) {
				continue
			}
			k := slices.IndexFunc(m.Items, func(w Want) bool { return w.Kind == st.Kind })
			if k < 0 {
				m.Items = append(m.Items, Want{Kind: st.Kind, Qty: q})
			} else {
				m.Items[k].Qty += q
			}
		case slices.Contains(pawns, id) && o.h.Standing(id) && !slices.Contains(m.Pawns, id):
			if fits(humanKg * 1000) {
				m.Pawns = append(m.Pawns, id)
			}
		case slices.Contains(beasts, id) && !slices.Contains(m.Animals, id):
			a, _ := o.h.Animal(id)
			if fits(animalKg(a.Species) * 1000) {
				m.Animals = append(m.Animals, id)
			}
		}
	}
	return m, left
}

// spill sets everything pod holds down beside it.
func (o pods) spill(pod entity.EntityID) {
	h := o.s.holds[pod]
	if h == nil {
		return
	}
	delete(o.s.holds, pod)
	b, ok := o.h.Building(pod)
	x, y := b.X, b.Y
	if !ok {
		x, y = o.h.Home()
	}
	o.dropHold(h, x, y)
}

// dropHold sets a hold down around (x, y): passengers step out, animals
// are let go, goods are set down (in stacks of at most a stack's limit).
func (o core) dropHold(h *Hold, x, y int) {
	for i, id := range h.Pawns {
		if cx, cy, ok := o.h.WalkableNear(x+i%3-1, y+i/3%3-1, 4); ok {
			o.h.Unstash(id, cx, cy)
		}
	}
	for _, b := range h.Animals {
		if cx, cy, ok := o.h.WalkableNear(x, y, 5); ok {
			o.h.PutAnimal(b.ID, b.State, cx, cy)
		}
	}
	for _, st := range h.Items {
		lim := max(1, st.Kind.StackLimit())
		for q := st.Qty; q > 0; q -= lim {
			part := st
			part.Qty = min(q, lim)
			o.h.DropStack(part, x, y)
		}
	}
	o.h.Changed()
}

// loadTick sends the picked colonists that are not on their way yet to
// board, and lets go of animals whose hauler stopped leading them.
func (o pods) loadTick() {
	for _, h := range sortedKeys(o.s.roped) {
		b := o.s.roped[h]
		j, here := o.h.Job(h)
		if !here || j.Kind == job.LoadPods && j.Carry == b.ID && j.Active() {
			continue // MAP-0: a hauler on another map is its own pass's
		}
		delete(o.s.roped, h)
		x, y := o.h.Pos(h)
		if cx, cy, ok := o.h.WalkableNear(x, y, 5); ok {
			o.h.PutAnimal(b.ID, b.State, cx, cy)
		}
	}
	for _, leader := range sortedKeys(o.s.wants) {
		gr, ok := o.groupOf(leader)
		if !ok {
			continue
		}
		for _, id := range o.s.wants[leader].Pawns {
			j := o.job(id)
			if !o.h.Standing(id) || j.Kind == job.LoadPods && j.Active() || j.Active() && !j.Kind.IsIdle() {
				continue
			}
			pod := o.podWithRoom(gr, humanKg)
			b, _ := o.h.Building(pod)
			o.h.StartJob(id, job.Record{Kind: job.LoadPods, Dest: pod, TX: b.X, TY: b.Y, Forced: true})
		}
	}
}

// inflight is what haulers are bringing to gr now: kilograms, and per
// item kind the amount.
func (o pods) inflight(gr *group) (int, map[item.Kind]int) {
	kg, by := 0, map[item.Kind]int{}
	for _, pid := range o.h.PawnIDs() {
		j := o.job(pid)
		if j.Kind != job.LoadPods || !j.Active() || j.Count <= 0 || !slices.Contains(gr.pods, j.Dest) {
			continue
		}
		sid := j.Carry
		if sid == 0 {
			sid = j.Target
		}
		if st, ok := o.h.Stack(sid); ok {
			by[st.Kind] += j.Count
			kg += (j.Count*st.Kind.MassGrams() + 999) / 1000
		}
	}
	return kg, by
}

// leading reports animal a being fetched or led to a pod now.
func (o pods) leading(a entity.EntityID) bool {
	for _, b := range o.s.roped {
		if b.ID == a {
			return true
		}
	}
	for _, pid := range o.h.PawnIDs() {
		if j := o.job(pid); j.Kind == job.LoadPods && j.Count == 0 && j.Target == a && j.Active() {
			return true
		}
	}
	return false
}

// AssignLoad starts a hauler on the first goods or animal a pod group
// still waits for (Haul work).
func (o pods) AssignLoad(id entity.EntityID) bool {
	for _, leader := range sortedKeys(o.s.wants) {
		gr, ok := o.groupOf(leader)
		if !ok {
			continue
		}
		m := o.s.wants[leader]
		kg, by := o.inflight(gr)
		free := capKg(gr) - o.loadKg(gr) - kg
		for _, w := range m.Items {
			n := min(w.Qty-by[w.Kind], free*1000/w.Kind.MassGrams(), item.CarryLimit)
			if n <= 0 {
				continue
			}
			sid, x, y, ok := o.h.Stock(id, w.Kind, n)
			if !ok {
				continue
			}
			pod := o.podWithRoom(gr, (n*w.Kind.MassGrams()+999)/1000)
			b, _ := o.h.Building(pod)
			if o.h.StartJob(id, job.Record{Kind: job.LoadPods, Target: sid, TX: x, TY: y, Dest: pod, DestX: b.X, DestY: b.Y, Count: n}) {
				return true
			}
		}
		for _, a := range m.Animals {
			st, ok := o.h.Animal(a)
			if !ok || o.leading(a) {
				continue
			}
			pod := o.podWithRoom(gr, animalKg(st.Species))
			b, _ := o.h.Building(pod)
			x, y := o.h.Pos(a)
			if o.h.StartJob(id, job.Record{Kind: job.LoadPods, Target: a, TX: x, TY: y, Dest: pod, DestX: b.X, DestY: b.Y}) {
				return true
			}
		}
	}
	return false
}

// walk sends id's job j on to (x, y): false when there is no way.
func (o core) walk(id entity.EntityID, j job.Record, x, y int) bool {
	path, ok := o.h.PathTo(id, x, y)
	if !ok {
		j.State, j.Fail = job.Fail, job.Unreachable
		o.h.SetJob(id, j)
		return false
	}
	j.TX, j.TY, j.Path, j.PathI, j.State = x, y, path, 0, job.Goto
	if len(path) == 0 {
		j.State = job.Do
	}
	o.h.SetJob(id, j)
	return true
}

// end ends id's job j (complete, or failed with why).
func (o core) end(id entity.EntityID, j job.Record, why job.FailReason) {
	j.State = job.Complete
	if why != 0 {
		j.State, j.Fail = job.Fail, why
	}
	o.h.SetJob(id, j)
}

// doLoad is one step of job.LoadPods: board, fetch or lead in, deliver.
func (o pods) doLoad(id entity.EntityID, j job.Record) {
	gr, ok := o.groupOf(j.Dest)
	m := (*Manifest)(nil)
	if ok {
		m = o.s.wants[gr.leader]
	}
	switch {
	case j.Target == 0 && j.Carry == 0: // a colonist boards
		if m == nil || !slices.Contains(m.Pawns, id) {
			o.end(id, j, 0)
			return
		}
		if !o.h.Stash(id, j.Dest) {
			o.end(id, j, job.Blocked)
			return
		}
		m.Pawns = slices.DeleteFunc(m.Pawns, func(p entity.EntityID) bool { return p == id })
		h := o.s.holdOf(j.Dest)
		h.Pawns = append(h.Pawns, id)
	case j.Count == 0 && j.Carry == 0: // reach the animal
		o.fetchAnimal(id, j, m)
	case j.Count == 0: // lead it in
		b, roped := o.s.roped[id]
		delete(o.s.roped, id)
		if !roped || b.ID != j.Carry {
			o.end(id, j, 0)
			return
		}
		if !ok {
			x, y := o.h.Pos(id)
			o.h.PutAnimal(b.ID, b.State, x, y)
			o.end(id, j, job.TargetGone)
			return
		}
		h := o.s.holdOf(j.Dest)
		h.Animals = append(h.Animals, b)
		if m != nil {
			m.Animals = slices.DeleteFunc(m.Animals, func(a entity.EntityID) bool { return a == b.ID })
		}
		o.end(id, j, 0)
	case j.Carry == 0: // pick the goods up
		carry, _, n, picked := o.h.PickUp(id, j.Target, j.Count)
		if !picked || !ok {
			o.end(id, j, job.TargetGone)
			return
		}
		j.Carry, j.Count = carry, n
		o.walk(id, j, j.DestX, j.DestY)
	default: // set them in
		st, held := o.h.Stack(j.Carry)
		if held && !ok {
			x, y := o.h.Pos(id)
			o.h.DropStack(st, x, y)
		}
		if held {
			o.h.Spend(j.Carry)
		}
		if !held || !ok {
			o.end(id, j, job.TargetGone)
			return
		}
		o.s.holdOf(o.podWithRoom(gr, (st.Qty*st.Kind.MassGrams()+999)/1000)).addMeal(st, o.h.MealRule())
		if m != nil {
			for i := range m.Items {
				if m.Items[i].Kind == st.Kind {
					m.Items[i].Qty -= st.Qty
				}
			}
			m.Items = slices.DeleteFunc(m.Items, func(w Want) bool { return w.Qty <= 0 })
		}
		o.end(id, j, 0)
	}
	o.h.Changed()
}

// fetchAnimal: beside the animal, rope it (it leaves the map with the
// hauler) and head for the pod; else follow it, a few times at most.
func (o pods) fetchAnimal(id entity.EntityID, j job.Record, m *Manifest) {
	a := j.Target
	if m == nil || !slices.Contains(m.Animals, a) {
		o.end(id, j, 0)
		return
	}
	ax, ay := o.h.Pos(a)
	px, py := o.h.Pos(id)
	if max(mathx.Abs(ax-px), mathx.Abs(ay-py)) > 1 {
		if j.WorkLeft++; j.WorkLeft > 6 {
			o.end(id, j, job.Unreachable)
			return
		}
		o.walk(id, j, ax, ay)
		return
	}
	st, ok := o.takeAnimal(a)
	if !ok {
		o.end(id, j, job.TargetGone)
		return
	}
	o.s.rope(id, Beast{ID: a, State: st})
	j.Carry = a
	o.walk(id, j, j.DestX, j.DestY)
}
