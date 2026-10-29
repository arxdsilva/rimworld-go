package care

import (
	"strconv"

	"rimworld-go/internal/domain/sim/entity"
)

// fakePrison is care.Prison over a fakeHost: raiders and visitors are
// sets, the prisoner beds are the host's buildings marked ForPrisoners,
// the arrest roll is a fixed answer, and what care asked is logged on the
// host's log (so the order of host and prison calls shows in one place).
type fakePrison struct {
	h        *fakeHost
	raiders  map[entity.EntityID]bool
	visitors map[entity.EntityID]bool
	armed    map[entity.EntityID]bool
	giveIn   bool // ArrestRoll's answer
}

func newFakePrison(h *fakeHost) *fakePrison {
	return &fakePrison{h: h, raiders: map[entity.EntityID]bool{}, visitors: map[entity.EntityID]bool{},
		armed: map[entity.EntityID]bool{}}
}

func (p *fakePrison) IsRaider(id entity.EntityID) bool     { return p.raiders[id] }
func (p *fakePrison) IsVisitor(id entity.EntityID) bool    { return p.visitors[id] }
func (p *fakePrison) PersonName(id entity.EntityID) string { return "N" + strconv.Itoa(int(id)) }

// PrisonBedFor: the nearest free bed for prisoners, as the game ranks it.
func (p *fakePrison) PrisonBedFor(target entity.EntityID, x, y int) (entity.EntityID, bool) {
	best, bestD := entity.EntityID(0), 1<<30
	for _, bid := range p.h.bldgIDs {
		b := p.h.bldgs[bid]
		if !b.ForPrisoners || !b.CanClaim(uint64(target)) {
			continue
		}
		if o, taken := p.h.owners[bid]; taken && o != target {
			continue
		}
		if d := manhattan(x, y, b.X, b.Y); d < bestD {
			best, bestD = bid, d
		}
	}
	return best, best != 0
}
func (p *fakePrison) DropGear(target entity.EntityID) {
	delete(p.armed, target)
	p.h.log = append(p.h.log, "drop gear")
}
func (p *fakePrison) MakePrisoner(target, captor entity.EntityID, arrested bool) {
	p.h.prisoners[target] = true
	if arrested {
		p.h.log = append(p.h.log, "arrested")
	} else {
		p.h.log = append(p.h.log, "prisoner")
	}
}
func (p *fakePrison) ArrestRoll(arrester, target entity.EntityID) bool { return p.giveIn }
func (p *fakePrison) StartFight(starter, other entity.EntityID) {
	p.h.log = append(p.h.log, "fight "+strconv.Itoa(int(starter))+">"+strconv.Itoa(int(other)))
}
