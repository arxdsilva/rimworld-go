package care

import "rimworld-go/internal/domain/sim/entity"

// targetKind is who a pawn is to the colony, for the rules that differ by
// it: who is rescued, to which bed, under which care group (SIM-063).
type targetKind uint8

const (
	targetNone     targetKind = iota
	targetColonist            // one of the colony's pawns
	targetPrisoner            // the colony's prisoner
	targetGuest               // a visitor, trader or ally of a faction
	targetStranger            // a guest of no faction (may ask to join)
	targetHostile             // a raider, not a prisoner
)

// kindOf sorts id into a targetKind.
func (c *Care) kindOf(id entity.EntityID) targetKind {
	switch {
	case c.h.IsPrisoner(id):
		return targetPrisoner
	case c.h.IsColonist(id):
		return targetColonist
	case c.gu != nil && c.gu.IsGuest(id):
		if c.gu.FactionOf(id) == 0 {
			return targetStranger
		}
		return targetGuest
	case c.pr != nil && c.pr.IsRaider(id):
		return targetHostile
	}
	return targetNone
}
