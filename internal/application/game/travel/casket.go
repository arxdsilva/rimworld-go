package travel

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The stasis casket (wiki: Cryptosleep casket): a colonist walks in, or
// carries a downed colonist or prisoner in (command.EnterCasket; jobs
// EnterCasket and CarryToCasket). The sleeper leaves the map (the game's
// stash): no need falls, it does not age, and its wounds and diseases
// stand still (the game keeps it out of the patients index, Asleep).
// Eject (command.EjectCasket) lets it out at once; a casket that is
// destroyed or taken down breaks open the same way. Waking sickness and
// the explosion of a badly damaged casket are not modelled.

// freeCasket reports casket id standing and empty.
func (o caskets) freeCasket(id entity.EntityID) bool {
	b, ok := o.h.Building(id)
	return ok && b.Kind == construction.StasisCasket && !b.Deconstruct && o.s.caskets[id] == 0
}

// cmdEnter sends Pawn into casket Target, or (Worker set) has it carry
// downed pawn Worker there.
func (o caskets) cmdEnter(c command.Command) {
	pawn, casket, patient := entity.EntityID(c.Pawn), entity.EntityID(c.Target), entity.EntityID(c.Worker)
	b, ok := o.h.Building(casket)
	if !ok || !o.freeCasket(casket) || !o.h.Standing(pawn) {
		return
	}
	if patient == 0 {
		x, y, ok := o.h.BesideSpot(pawn, b.X, b.Y)
		if ok {
			o.h.StartJob(pawn, job.Record{Kind: job.EnterCasket, Target: casket, TX: x, TY: y, Forced: true})
		}
		return
	}
	if !o.h.Downed(patient) {
		return
	}
	px, py := o.h.Pos(patient)
	if x, y, ok := o.h.BesideSpot(pawn, px, py); ok {
		o.h.StartJob(pawn, job.Record{Kind: job.CarryToCasket, Target: patient, TX: x, TY: y, Dest: casket, Forced: true})
	}
}

// doEnter is job.EnterCasket at the casket: the pawn lies down in it.
func (o caskets) doEnter(id entity.EntityID, j job.Record) {
	if !o.freeCasket(j.Target) || !o.h.Stash(id, j.Target) {
		o.end(id, j, job.TargetGone)
		return
	}
	o.s.setSleeper(j.Target, id)
	b, _ := o.h.Building(j.Target)
	o.h.Message(b.X, b.Y, o.h.Name(id)+" lay down in the stasis casket.")
	o.h.Changed()
}

// doCarry is job.CarryToCasket: at the patient pick it up and head for
// the casket; at the casket lay it in.
func (o caskets) doCarry(id entity.EntityID, j job.Record) {
	b, ok := o.h.Building(j.Dest)
	if j.Carry == 0 {
		if !ok || !o.freeCasket(j.Dest) || !o.h.Downed(j.Target) {
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
	if !o.freeCasket(j.Dest) || !o.h.Stash(j.Carry, j.Dest) {
		o.end(id, j, job.TargetGone)
		return
	}
	o.s.setSleeper(j.Dest, j.Carry)
	o.h.Message(b.X, b.Y, o.h.Name(j.Carry)+" was laid in the stasis casket.")
	o.end(id, j, 0)
	o.h.Changed()
}

// eject lets casket id's sleeper out beside it (at (x, y), the casket's
// cell, when it is gone).
func (o caskets) eject(id entity.EntityID, x, y int) {
	p := o.s.caskets[id]
	if p == 0 {
		return
	}
	delete(o.s.caskets, id)
	if cx, cy, ok := o.h.WalkableNear(x, y, 4); ok && o.h.Unstash(p, cx, cy) {
		o.h.Message(cx, cy, o.h.Name(p)+" came out of the stasis casket.")
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

// Gone is a building taken away (destroyed, deconstructed, crated or
// launched): a casket breaks open, a pod spills what it held.
func (o Ops) Gone(id entity.EntityID, b construction.Building) {
	switch b.Kind {
	case construction.StasisCasket:
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
