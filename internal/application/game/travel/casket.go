package travel

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The stasis casket (wiki: Cryptosleep casket): a colonist walks in, or a
// colonist carries a downed colonist, prisoner or tame animal in
// (command.EnterCasket; jobs EnterCasket and CarryToCasket; the wiki lets
// prisoners and animals in once downed). The sleeper leaves the map (the
// game's stash): no need falls, it does not age, and its wounds and
// diseases stand still (the game keeps it out of the patients index,
// Asleep). Eject (command.EjectCasket) lets it out at once; a casket that
// is destroyed or taken down breaks open the same way. Whoever comes out
// has cryptosleep sickness (body/cond_crypto.go; the wiki does not say
// animals get it, so they don't).
//
// A casket down to casketSparkHP hit points or fewer sparks for
// casketFuse ticks, then explodes: a flame burst (wiki: 10 damage, radius
// 2.66) that destroys it and throws its sleeper out. The fuse is one-way:
// once lit it goes off, repaired or not.
//
// The carry also serves the escape ship's caskets (game/ship, AX-8): a
// downed pawn carried to a free ship casket is laid in it there
// (Carriers.LayInShip).
const (
	casketSparkHP = 50  // wiki: Cryptosleep casket
	casketFuse    = 120 // wiki: about two seconds
)

// casketBlast is the casket's explosion (wiki: Cryptosleep casket).
func casketBlast() explosion.Def {
	return explosion.Def{Kind: explosion.Flame, Radius: 2660, Damage: 10}
}

// freeCasket reports casket id a standing, empty stasis casket, or (ship
// true) a free ship casket too.
func (o caskets) freeCasket(id entity.EntityID, ship bool) bool {
	b, ok := o.h.Building(id)
	switch {
	case !ok || b.Deconstruct:
		return false
	case b.Kind == construction.StasisCasket:
		return o.s.caskets[id] == 0
	}
	return ship && b.Kind == construction.ShipCasket && o.h.ShipCasketFree(id)
}

// carriable reports patient one a casket takes once downed: a colonist,
// a prisoner or a tame animal.
func (o caskets) carriable(patient entity.EntityID) bool {
	if !o.h.Downed(patient) {
		return false
	}
	if a, ok := o.h.Animal(patient); ok {
		return a.Tame
	}
	return o.h.IsPrisoner(patient) || o.isPawn(patient)
}

// isPawn reports id one of the colony's pawns.
func (o caskets) isPawn(id entity.EntityID) bool {
	for _, p := range o.h.PawnIDs() {
		if p == id {
			return true
		}
	}
	return false
}

// cmdEnter sends Pawn into stasis casket Target, or (Worker set) has it
// carry downed Worker to casket Target (a stasis or a ship casket).
func (o caskets) cmdEnter(c command.Command) {
	pawn, casket, patient := entity.EntityID(c.Pawn), entity.EntityID(c.Target), entity.EntityID(c.Worker)
	b, ok := o.h.Building(casket)
	if !ok || !o.freeCasket(casket, patient != 0) || !o.h.Standing(pawn) {
		return
	}
	if patient == 0 {
		if x, y, ok := o.h.BesideSpot(pawn, b.X, b.Y); ok {
			o.h.StartJob(pawn, job.Record{Kind: job.EnterCasket, Target: casket, TX: x, TY: y, Forced: true})
		}
		return
	}
	if !o.carriable(patient) {
		return
	}
	px, py := o.h.Pos(patient)
	if x, y, ok := o.h.BesideSpot(pawn, px, py); ok {
		o.h.StartJob(pawn, job.Record{Kind: job.CarryToCasket, Target: patient, TX: x, TY: y, Dest: casket, Forced: true})
	}
}

// doEnter is job.EnterCasket at the casket: the pawn lies down in it.
func (o caskets) doEnter(id entity.EntityID, j job.Record) {
	if !o.freeCasket(j.Target, false) || !o.h.Stash(id, j.Target) {
		o.end(id, j, job.TargetGone)
		return
	}
	o.s.setSleeper(j.Target, id)
	b, _ := o.h.Building(j.Target)
	o.h.Message(b.X, b.Y, "%s lay down in the stasis casket.", o.h.Name(id))
	o.h.Changed()
}

// doCarry is job.CarryToCasket: at the patient pick it up and head for
// the casket; at the casket lay it in.
func (o caskets) doCarry(id entity.EntityID, j job.Record) {
	b, ok := o.h.Building(j.Dest)
	if j.Carry == 0 {
		if !ok || !o.freeCasket(j.Dest, true) || !o.carriable(j.Target) {
			o.end(id, j, job.TargetGone)
			return
		}
		x, y, ok := o.h.BesideSpot(id, b.X, b.Y)
		if !ok {
			o.end(id, j, job.Unreachable)
			return
		}
		o.h.Pick(id, j.Target)
		j.Carry = j.Target
		o.walk(id, j, x, y)
		return
	}
	o.h.PutDown(id) // put down here first, whatever happens next
	switch {
	case !o.freeCasket(j.Dest, true):
		o.end(id, j, job.TargetGone)
		return
	case b.Kind == construction.ShipCasket: // game/ship lays it in and says so
		if !o.h.LayInShip(j.Carry, j.Dest) {
			o.end(id, j, job.TargetGone)
			return
		}
	case !o.h.Stash(j.Carry, j.Dest):
		o.end(id, j, job.TargetGone)
		return
	default:
		o.s.setSleeper(j.Dest, j.Carry)
		o.h.Message(b.X, b.Y, "%s was laid in the stasis casket.", o.h.Name(j.Carry))
	}
	o.end(id, j, 0)
	o.h.Changed()
}

// eject lets casket id's sleeper out beside it (at (x, y), the casket's
// cell, when it is gone), dazed by cryptosleep sickness.
func (o caskets) eject(id entity.EntityID, x, y int) {
	p := o.s.caskets[id]
	if p == 0 {
		return
	}
	delete(o.s.caskets, id)
	if cx, cy, ok := o.h.WalkableNear(x, y, 4); ok && o.h.Unstash(p, cx, cy) {
		o.h.Sicken(p) // an animal is left alone (the game sickens people only)
		o.h.Message(cx, cy, "%s came out of the stasis casket.", o.h.Name(p))
	}
	o.h.Changed()
}

// cmdEject opens casket Target.
func (o caskets) cmdEject(c command.Command) {
	id := entity.EntityID(c.Target)
	if b, ok := o.h.Building(id); ok && b.Kind == construction.StasisCasket {
		o.eject(id, b.X, b.Y)
	}
}

// fuseTick lights the fuse of every stasis casket down to casketSparkHP
// and blows those whose fuse has burnt (every loadUpkeep ticks).
func (o caskets) fuseTick(now uint64) {
	for _, id := range o.casketIDs() {
		b, ok := o.h.Building(id)
		if !ok {
			continue
		}
		at, lit := o.s.fuses[id]
		switch {
		case !lit && b.HP() <= casketSparkHP:
			if o.s.fuses == nil {
				o.s.fuses = map[entity.EntityID]uint64{}
			}
			o.s.fuses[id] = now + casketFuse
			o.h.Message(b.X, b.Y, "The stasis casket sparks and hisses!")
		case lit && now >= at:
			delete(o.s.fuses, id)
			o.h.Remove(id) // Gone throws the sleeper out
			o.h.Explode(b.X, b.Y, casketBlast())
		}
	}
}

// casketIDs is the stasis caskets, in building order (cached per
// structure version).
func (o caskets) casketIDs() []entity.EntityID {
	c := &o.s.casketList
	if c.ok && c.ver == o.h.StructVer() {
		return c.ids
	}
	c.ids, c.ok, c.ver = c.ids[:0], true, o.h.StructVer()
	for _, id := range o.h.BuildingIDs() {
		if b, ok := o.h.Building(id); ok && b.Kind == construction.StasisCasket {
			c.ids = append(c.ids, id)
		}
	}
	return c.ids
}

// Gone is a building taken away (destroyed, deconstructed, crated or
// launched): a casket breaks open, a pod spills what it held.
func (o Ops) Gone(id entity.EntityID, b construction.Building) {
	switch b.Kind {
	case construction.StasisCasket:
		delete(o.s.fuses, id)
		caskets(o).eject(id, b.X, b.Y)
	case construction.TransportPod:
		if h := o.s.holds[id]; h != nil {
			delete(o.s.holds, id)
			o.dropHold(h, b.X, b.Y)
		}
	case construction.MineralScanner:
		delete(o.s.scans, id)
	}
}
