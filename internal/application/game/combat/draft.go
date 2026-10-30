package combat

import (
	"cmp"
	"fmt"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Drafted control (DEF-4): fire at will, hold position and grenades.
//
// Fire at will is on unless the player turns it off (command
// SetFireAtWill): off, an idle drafted colonist never starts an attack of
// its own (Strikes.AutoEngage), only on an order. Hold position (command
// SetHoldPosition) keeps a drafted colonist on its tile: it shoots what it
// can hit from there and never chases (Strikes.Do completes an attack it
// started itself once the target is out of reach). Both are switches, not
// thresholds: they change only on a command, so they cannot flap.
//
// Grenades: a colonist that equips a grenade stack takes the whole stack
// as a belt (Carry); every grenade thrown, on an order (ThrowGrenade) or
// at a hostile on its own, spends one, and the last one leaves it
// unarmed. An order waits the grenade's warmup (wiki: Weapons, 90 ticks)
// and throws through the blast path (Shots.Lob, Blasts): the grenade
// scatters by explosion.ForcedMiss at range (wiki: Combat). Raiders'
// grenades are their kit and never run out.
type Drafts struct {
	h     Host
	arms  Arms
	shots *Shots
	set   map[entity.EntityID]Draft // saved
	queue []Throw                   // saved: orders waiting out the warmup
}

// Arms takes a colonist's spent weapon away.
type Arms interface {
	// Disarm leaves id unarmed (its last grenade thrown).
	Disarm(id entity.EntityID)
}

// Draft is one colonist's drafted switches and its grenade belt. All
// fields are exported for the save.
type Draft struct {
	// NoFire: fire at will off. Hold: hold position.
	NoFire, Hold bool
	// Grenades carried (0 with a grenade wielded from before belts: one).
	Grenades int
}

// DraftSave is one Draft on disk.
type DraftSave struct {
	ID entity.EntityID
	D  Draft
}

// Throw is a grenade order waiting out its warmup: colonist ID throws at
// (X, Y) at tick At.
type Throw struct {
	ID   entity.EntityID
	X, Y int
	At   uint64
}

// isGrenade reports a thrown grenade weapon.
func isGrenade(wk weapon.Kind) bool {
	return wk == weapon.FragGrenade || wk == weapon.Molotov || wk == weapon.EMPGrenade
}

func (d *Drafts) put(id entity.EntityID, s Draft) {
	if s == (Draft{}) {
		delete(d.set, id)
		return
	}
	if d.set == nil {
		d.set = map[entity.EntityID]Draft{}
	}
	d.set[id] = s
}

// SetFireAtWill turns colonist id's fire at will on or off.
func (d *Drafts) SetFireAtWill(id entity.EntityID, on bool) {
	s := d.set[id]
	s.NoFire = !on
	d.put(id, s)
	if j, _ := d.h.Job(id); !on && j.Kind == job.Attack && !j.Forced && j.Active() {
		d.h.Interrupt(id, job.Cancelled) // the shooting it started itself stops
	}
}

// SetHold turns colonist id's hold position on or off.
func (d *Drafts) SetHold(id entity.EntityID, on bool) {
	s := d.set[id]
	s.Hold = on
	d.put(id, s)
}

// FireAtWill: id shoots on its own (the default).
func (d *Drafts) FireAtWill(id entity.EntityID) bool { return !d.set[id].NoFire }

// Holding: id holds position.
func (d *Drafts) Holding(id entity.EntityID) bool { return d.set[id].Hold }

// Grenades is how many grenades id carries.
func (d *Drafts) Grenades(id entity.EntityID) int {
	if n := d.set[id].Grenades; n > 0 {
		return n
	}
	if isGrenade(d.h.Weapon(id)) {
		return 1
	}
	return 0
}

// Carry is how many of a stack of qty of kind k colonist id takes when it
// equips it: a grenade stack whole (its belt), anything else one.
func (d *Drafts) Carry(id entity.EntityID, k item.Kind, qty int) int {
	if !isGrenade(weapon.ForItem(k)) || qty <= 1 {
		d.belt(id, 1, k)
		return 1
	}
	d.belt(id, qty, k)
	return qty
}

// belt records id's grenades (n of k; none for any other weapon).
func (d *Drafts) belt(id entity.EntityID, n int, k item.Kind) {
	s := d.set[id]
	s.Grenades = 0
	if isGrenade(weapon.ForItem(k)) {
		s.Grenades = n
	}
	d.put(id, s)
}

// Drop is how many of k colonist id puts down when it lets its weapon go
// (its grenades, else one); the belt is forgotten.
func (d *Drafts) Drop(id entity.EntityID, k item.Kind) int {
	n := max(1, d.set[id].Grenades)
	if !isGrenade(weapon.ForItem(k)) {
		n = 1
	}
	d.belt(id, 0, k)
	return n
}

// spend uses one of thrower id's grenades (d is what it threw): a
// colonist's belt runs down and the last one leaves it unarmed; a raider's
// kit never runs out.
func (d *Drafts) spend(id entity.EntityID, wk weapon.Kind) {
	if !isGrenade(wk) || d.h.StandingRaider(id) {
		return
	}
	if _, beast := d.h.Animal(id); beast {
		return
	}
	s := d.set[id]
	if s.Grenades > 1 {
		s.Grenades--
		d.put(id, s)
		return
	}
	s.Grenades = 0
	d.put(id, s)
	if d.arms != nil {
		d.arms.Disarm(id)
	}
}

// Throw orders colonist id to throw a grenade at (x, y): it must carry
// one and the cell lie within the grenade's range (wiki: Weapons, 12.9
// tiles); it drops what it does and throws once the warmup is over. It
// reports whether the order was taken.
func (d *Drafts) Throw(id entity.EntityID, x, y int) bool {
	h := d.h
	px, py, ok := h.Pos(id)
	wk := h.Weapon(id)
	if !ok || !isGrenade(wk) || d.Grenades(id) == 0 || !h.StandingPawn(id) || !h.CanViolence(id) || !h.InBounds(x, y) {
		return false
	}
	def := weapon.DefOf(wk)
	if !weapon.InRange(def, weapon.Distance(px, py, x, y)) {
		h.Message(px, py, fmt.Sprintf("%s can't throw that far.", h.Name(id)))
		return false
	}
	d.queue = slices.DeleteFunc(d.queue, func(t Throw) bool { return t.ID == id })
	h.Interrupt(id, job.Cancelled)
	d.queue = append(d.queue, Throw{ID: id, X: x, Y: y, At: h.Now() + uint64(def.Warmup)})
	return true
}

// phase throws the grenades whose warmup is over, in the order given.
func (d *Drafts) phase(now uint64) {
	if len(d.queue) == 0 {
		return
	}
	keep := d.queue[:0]
	var due []Throw
	for _, t := range d.queue {
		if t.At <= now {
			due = append(due, t)
		} else {
			keep = append(keep, t)
		}
	}
	d.queue = keep
	for _, t := range due {
		d.throw(t)
	}
}

// throw lobs t's grenade from where its thrower stands now, if it still
// can: it lands within the forced miss of the aim (explosion.ForcedMiss).
func (d *Drafts) throw(t Throw) {
	h := d.h
	px, py, ok := h.Pos(t.ID)
	wk := h.Weapon(t.ID)
	if !ok || !isGrenade(wk) || !h.StandingPawn(t.ID) {
		return
	}
	def := weapon.DefOf(wk)
	dist := weapon.Distance(px, py, t.X, t.Y)
	if !weapon.InRange(def, dist) {
		return // it moved off
	}
	lx, ly := t.X, t.Y
	if rad := explosion.ForcedMiss(def.ForcedMiss, dist); rad > 0 {
		lx, ly = t.X+h.Roll(2*rad+1)-rad, t.Y+h.Roll(2*rad+1)-rad
	}
	d.shots.Lob(t.ID, wk, px, py, t.X, t.Y, lx, ly)
	h.Struck(t.ID, 0, h.Now())
	d.spend(t.ID, wk)
}

// Apply carries out a drafted-control command on colonist c.Pawn
// (SetFireAtWill, SetHoldPosition, ThrowGrenade; the game routes them
// here); raiders and animals take none.
func (d *Drafts) Apply(c command.Command) {
	id := entity.EntityID(c.Pawn)
	if _, beast := d.h.Animal(id); beast || d.h.StandingRaider(id) {
		return
	}
	if _, _, ok := d.h.Pos(id); !ok {
		return
	}
	switch c.Kind {
	case command.SetFireAtWill:
		d.SetFireAtWill(id, c.Enabled)
	case command.SetHoldPosition:
		d.SetHold(id, c.Enabled)
	case command.ThrowGrenade:
		d.Throw(id, c.X, c.Y)
	}
}

// Forget drops colonist id's switches and orders (it left the colony).
func (d *Drafts) Forget(id entity.EntityID) {
	delete(d.set, id)
	d.queue = slices.DeleteFunc(d.queue, func(t Throw) bool { return t.ID == id })
}

// export is the switches, belts and waiting throws for the save.
func (d *Drafts) export() ([]DraftSave, []Throw) {
	var out []DraftSave
	for id, s := range d.set {
		out = append(out, DraftSave{ID: id, D: s})
	}
	slices.SortFunc(out, func(a, b DraftSave) int { return cmp.Compare(a.ID, b.ID) })
	return out, slices.Clone(d.queue)
}

// importDrafts restores saved switches, belts and throws.
func (d *Drafts) importDrafts(s []DraftSave, q []Throw) {
	d.set, d.queue = nil, append(d.queue[:0], q...)
	for _, v := range s {
		d.put(v.ID, v.D)
	}
}
