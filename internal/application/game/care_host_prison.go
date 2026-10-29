package game

import (
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
)

// carePrison is care.Prison (care/deps.go): what capture and arrest need
// from the simulation (SIM-062). Prisoner state, the raid roster and the
// prison RNG stream stay here.
type carePrison struct{ g *Game }

func (h carePrison) IsRaider(id entity.EntityID) bool     { return h.g.isRaider(id) }
func (h carePrison) IsVisitor(id entity.EntityID) bool    { return h.g.visitor(id) }
func (h carePrison) PersonName(id entity.EntityID) string { return h.g.personName(id) }
func (h carePrison) PrisonBedFor(target entity.EntityID, x, y int) (entity.EntityID, bool) {
	return h.g.prisonBedFor(target, Position{X: x, Y: y})
}
func (h carePrison) DropGear(target entity.EntityID) { h.g.dropRaiderGear(target) }
func (h carePrison) MakePrisoner(target, captor entity.EntityID, arrested bool) {
	if arrested && h.g.visitor(target) {
		if f := h.g.factionOf(h.g.personOf(target).Faction); f != nil {
			faction.Adjust(f, -arrestGoodwill, "visitor arrested")
		}
	}
	h.g.makePrisoner(target, captor)
}
func (h carePrison) ArrestRoll(arrester, target entity.EntityID) bool {
	return h.g.stream(streamPrison).IntN(1000) < h.g.arrestChance(arrester, target)
}
func (h carePrison) StartFight(starter, other entity.EntityID) { h.g.startSocialFight(starter, other) }
