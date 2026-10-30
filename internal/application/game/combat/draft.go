package combat

import (
	"cmp"
	"fmt"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/inventory"
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
	draft Drafting // CBT-2: auto-undraft
	shots *Shots
	set   map[entity.EntityID]Draft // saved
	queue []Throw                   // saved: orders waiting out the warmup
	tac   *Tactics                  // CBT-4: a step into cover (engage.go)
	gear  *Gear                     // CBT-3: the weapon's make goes with it
}

// Arms is a colonist's hands: what it wields.
type Arms interface {
	// Disarm leaves id unarmed (its last grenade thrown, a weapon put
	// down).
	Disarm(id entity.EntityID)
	// Wielded is the item kind id wields (read only while it is armed:
	// Creatures.Weapon is not fists); Wield puts kind k in its hands
	// (CBT-2: its spare drawn).
	Wielded(id entity.EntityID) item.Kind
	Wield(id entity.EntityID, k item.Kind)
	// DropAt puts stack st down on its tile, the colony's (CBT-3: a
	// weapon keeps its quality, stuff and wear).
	DropAt(st item.Stack)
}

// Draft is one colonist's drafted switches and its grenade belt. All
// fields are exported for the save.
type Draft struct {
	// NoFire: fire at will off. Hold: hold position.
	NoFire, Hold bool
	// Grenades carried (0 with a grenade wielded from before belts: one).
	Grenades int
	// CBT-2 (append only): Spare is the weapon carried beside the one
	// wielded (DropWeapon); QuietFrom the tick the colonist's quiet began
	// (idle and drafted, no threat on the map) and IdleAt the last idle
	// tick it was seen (the auto-undraft timer).
	Spare             inventory.Slot
	QuietFrom, IdleAt uint64
	// CBT-4 (append only): Response is the undrafted threat response
	// (response.go); CoverAt the tick before which it looks for cover
	// again (tactics.go), stepping to (CoverX, CoverY).
	Response       Response
	CoverAt        uint64
	CoverX, CoverY int
}

// DraftSave is one Draft on disk.
type DraftSave struct {
	ID entity.EntityID
	D  Draft
}

// Throw is a grenade order waiting out its warmup: colonist ID throws at
// (X, Y) at tick At. Walk (CBT-2, append only): it is still walking into
// range (At is set once it gets there).
type Throw struct {
	ID   entity.EntityID
	X, Y int
	At   uint64
	Walk bool
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
// one; it drops what it does and throws once the warmup is over. A cell
// beyond the grenade's range (wiki: Weapons, 12.9 tiles) is walked
// towards first (CBT-2: a forced move; the throw waits until it is in
// range, and is dropped if another order replaces the walk). It reports
// whether the order was taken.
func (d *Drafts) Throw(id entity.EntityID, x, y int) bool {
	h := d.h
	px, py, ok := h.Pos(id)
	wk := h.Weapon(id)
	if !ok || !isGrenade(wk) || d.Grenades(id) == 0 || !h.StandingPawn(id) || !h.CanViolence(id) || !h.InBounds(x, y) ||
		d.gear.Shielded(id) { // CBT-3: a shield belt's wearer throws nothing out
		return false
	}
	def := weapon.DefOf(wk)
	d.queue = slices.DeleteFunc(d.queue, func(t Throw) bool { return t.ID == id })
	h.Interrupt(id, job.Cancelled)
	if !weapon.ReachesAt(def, px, py, x, y) {
		if !h.StartJob(id, job.Record{Kind: job.Move, TX: x, TY: y, Forced: true}) {
			h.Message(px, py, fmt.Sprintf("%s can't get close enough to throw.", h.Name(id)))
			return false
		}
		d.queue = append(d.queue, Throw{ID: id, X: x, Y: y, Walk: true})
		return true
	}
	d.queue = append(d.queue, Throw{ID: id, X: x, Y: y, At: h.Now() + uint64(def.Warmup)})
	return true
}

// walk advances a throw still walking into range: once there the walk
// stops and the warmup starts; a walk another order replaced (or that
// failed) drops the throw. It reports whether t is kept.
func (d *Drafts) walk(t *Throw, now uint64) bool {
	h := d.h
	j, _ := h.Job(t.ID)
	if j.Kind != job.Move || !j.Forced || j.TX != t.X || j.TY != t.Y || !j.Active() {
		return false
	}
	px, py, ok := h.Pos(t.ID)
	wk := h.Weapon(t.ID)
	if !ok || !isGrenade(wk) || !weapon.ReachesAt(weapon.DefOf(wk), px, py, t.X, t.Y) {
		return ok && isGrenade(wk)
	}
	h.Interrupt(t.ID, job.Cancelled) // in range: it stops and throws
	t.Walk, t.At = false, now+uint64(weapon.DefOf(wk).Warmup)
	return true
}

// phase throws the grenades whose warmup is over, in the order given, and
// walks the others into range.
func (d *Drafts) phase(now uint64) {
	if len(d.queue) == 0 {
		return
	}
	keep := d.queue[:0]
	var due []Throw
	for _, t := range d.queue {
		switch {
		case t.Walk:
			if d.walk(&t, now) {
				keep = append(keep, t)
			}
		case t.At <= now:
			due = append(due, t)
		default:
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
	dist := weapon.Range(px, py, t.X, t.Y)
	if !weapon.ReachesAt(def, px, py, t.X, t.Y) {
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
	case command.DropWeapon:
		d.DropWeapon(id, c.Enabled)
	case command.PopSmokepop:
		d.gear.Pop(id)
	case command.SetThreatResponse:
		if c.X >= int(RespondDefault) && c.X <= int(RespondIgnore) {
			d.SetResponse(id, Response(c.X))
		}
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

// Auto-undraft (CBT-2). wiki: Drafting: a drafted colonist with no threat
// on the map and no orders for AutoUndraft ticks is released. The quiet
// starts over whenever the colonist has something to do (an order, an
// attack it started itself) or a hostile is on the map (asked once in
// threatEvery ticks, and again before the release); the release needs the whole stretch unbroken, and
// nothing ever drafts a colonist again but the player, so it cannot flap.
const (
	AutoUndraft = 10_000
	// threatEvery: original tuning (the wiki gives no cadence).
	threatEvery = 60
)

// idle is drafted colonist id standing idle at now (Strikes.AutoEngage);
// threat asks whether a hostile is on the map. It releases id from the
// draft after AutoUndraft quiet ticks and reports that it did.
func (d *Drafts) idle(id entity.EntityID, now uint64, threat func() bool) bool {
	s := d.set[id]
	switch {
	case s.IdleAt+1 != now || s.QuietFrom == 0:
		s.QuietFrom = now // busy since its last idle tick (or new): the quiet starts over
	case (now+uint64(id))%threatEvery == 0 && threat():
		s.QuietFrom = now
	}
	s.IdleAt = now
	release := now-s.QuietFrom >= AutoUndraft && d.draft != nil
	switch {
	case release && threat(): // asked again at the end: a hostile just arrived
		s.QuietFrom, release = now, false
	case release:
		s.QuietFrom, s.IdleAt = 0, 0
	}
	d.put(id, s)
	if release {
		d.draft.Undraft(id)
	}
	return release
}

// DropWeapon has colonist id let its weapon go (CBT-2, command
// DropWeapon): on the ground where it stands (a grenade belt whole), then
// it draws the spare weapon it carries, if any; with nothing wielded the
// spare goes down instead. stow puts the wielded weapon in the spare slot
// instead, drawing the spare it held (a swap). Raiders and animals take
// no such order.
func (d *Drafts) DropWeapon(id entity.EntityID, stow bool) {
	h := d.h
	x, y, ok := h.Pos(id)
	if d.arms == nil || !ok || !h.StandingPawn(id) || h.StandingRaider(id) {
		return
	}
	if _, beast := h.Animal(id); beast {
		return
	}
	cur, armed := d.arms.Wielded(id), h.Weapon(id) != weapon.Fists
	var st item.Stack
	if armed {
		st = d.gear.Release(id, cur, d.Drop(id, cur)) // the belt is forgotten, the make kept
		st.X, st.Y = x, y
		d.arms.Disarm(id)
	}
	s := d.set[id]
	spare := s.Spare
	s.Spare = inventory.Slot{}
	switch {
	case armed && stow:
		s.Spare.Stow(st)
	case armed:
		d.arms.DropAt(st)
	case !stow && !spare.Empty(): // unarmed: the spare goes down
		d.arms.DropAt(spare.Stack(x, y))
		spare = inventory.Slot{}
	}
	d.put(id, s)
	if !spare.Empty() {
		drawn := spare.Stack(x, y)
		d.arms.Wield(id, drawn.Kind)
		d.belt(id, drawn.Qty, drawn.Kind)
		d.gear.Hold(id, drawn)
	}
}

// Spare is the weapon colonist id carries beside the one it wields (the
// read model; ok false: none).
func (d *Drafts) Spare(id entity.EntityID) (k item.Kind, qty int, ok bool) {
	s := d.set[id].Spare
	return s.Kind, s.Qty, !s.Empty()
}

// DropSpare puts colonist id's spare weapon down where it lies (it died).
func (d *Drafts) DropSpare(id entity.EntityID) {
	s := d.set[id]
	x, y, ok := d.h.Pos(id)
	if d.arms == nil || !ok || s.Spare.Empty() {
		return
	}
	st := s.Spare.Stack(x, y)
	s.Spare = inventory.Slot{}
	d.put(id, s)
	d.arms.DropAt(st)
}
