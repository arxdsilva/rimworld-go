package care

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// orderPrison is care.Prison for the order tests: raiders and visitors by
// set, one optional prisoner bed, names "R<digit>" for anyone not a
// colonist, and a record of what capture asked it to do.
type orderPrison struct {
	h        *fakeHost
	raider   map[entity.EntityID]bool
	visitor  map[entity.EntityID]bool
	prisonBd entity.EntityID // 0: no free prisoner bed
	log      []string
}

func newOrderPrison(h *fakeHost) *orderPrison {
	return &orderPrison{h: h, raider: map[entity.EntityID]bool{}, visitor: map[entity.EntityID]bool{}}
}

func (p *orderPrison) IsRaider(id entity.EntityID) bool  { return p.raider[id] }
func (p *orderPrison) IsVisitor(id entity.EntityID) bool { return p.visitor[id] }
func (p *orderPrison) PersonName(id entity.EntityID) string {
	if slices.Contains(p.h.pawns, id) {
		return p.h.Name(id)
	}
	return "R" + string(rune('0'+id%10))
}
func (p *orderPrison) PrisonBedFor(target entity.EntityID, x, y int) (entity.EntityID, bool) {
	return p.prisonBd, p.prisonBd != 0
}
func (p *orderPrison) DropGear(target entity.EntityID) { p.log = append(p.log, "drop gear") }
func (p *orderPrison) MakePrisoner(target, captor entity.EntityID, arrested bool) {
	p.log = append(p.log, "prisoner")
}
func (p *orderPrison) ArrestRoll(arrester, target entity.EntityID) bool { return true }
func (p *orderPrison) StartFight(starter, other entity.EntityID)        { p.log = append(p.log, "fight") }

// outsider adds a non-colonist pawn (raider, prisoner or visitor) on
// (x, y) with vital status from the caller.
func (h *fakeHost) outsider(id entity.EntityID, x, y int) {
	h.raiders = append(h.raiders, id)
	h.pos[id] = [2]int{x, y}
}

// labels lists the options' labels, with the reason after a disabled one.
func labels(opts []Option) []string {
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		l := o.Label
		if o.Disabled {
			l += " (" + o.Reason + ")"
		}
		out = append(out, l)
	}
	return out
}

// optionFor is the option issuing order o on target (ok false: none).
func optionFor(opts []Option, o command.Order, target entity.EntityID) (Option, bool) {
	for _, op := range opts {
		if op.Cmd.Kind == command.Prioritize && op.Cmd.Order == o && op.Cmd.Target == uint64(target) {
			return op, true
		}
	}
	return Option{}, false
}

// jobOn is an active job of kind k on target.
func jobOn(k job.Kind, target entity.EntityID) job.Record {
	return job.Record{Kind: k, Target: target, State: job.Goto}
}
