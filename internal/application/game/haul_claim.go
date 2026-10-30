package game

import (
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-017: haulers carry only the colony's own things. Stacks found on
// the map (ruin loot, stone chunks, a predator's kill) are Unclaimed and
// wait for the player's haul mark; only haulToStockpile (tick.go) leaves
// them, every other job uses them. Whatever a colonist picks up, puts
// down or stores is the colony's (doHaul, dropCarry, mergeOrSet).

// spawnUnclaimed drops a stack found on the map (kind k, qty) at (x, y),
// merging into a like found stack there; forbid also forbids it (raid
// loot).
func (g *Game) spawnUnclaimed(k item.Kind, qty, x, y int, forbid bool) {
	sid, err := g.SpawnStack(k, qty, x, y)
	if err != nil {
		return
	}
	if st, ok := g.stacks.Get(sid); ok {
		st.Unclaimed, st.Forbidden = true, forbid
		g.mergeOrSet(sid, st)
	}
}

// claim makes st the colony's: no longer found, no haul mark.
func claim(st *item.Stack) { st.Unclaimed, st.HaulMarked = false, false }

// leftFound reports a stack haulers leave alone by themselves: found on
// the map and not marked for hauling.
func leftFound(st *item.Stack) bool { return st.Unclaimed && !st.HaulMarked }

// setHaulMark applies a SetHaulMark command: it marks (or unmarks) a found
// ground stack for hauling; the colony's own stacks take no mark.
func (g *Game) setHaulMark(id entity.EntityID, on bool) {
	st, ok := g.stacks.Get(id)
	if !ok || st.Carrier != 0 || !st.Unclaimed || st.HaulMarked == on {
		return
	}
	st.HaulMarked = on
	g.mergeOrSet(id, st)
}

// markHaul marks or unmarks every visible found ground stack in the
// rectangle (DesignateHaul; cancelRect unmarks).
func (g *Game) markHaul(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	for _, id := range append([]entity.EntityID(nil), g.itemIDs...) {
		st, ok := g.stacks.Get(id)
		if ok && st.Unclaimed && st.HaulMarked != on && st.X >= minX && st.X <= maxX && st.Y >= minY && st.Y <= maxY &&
			(!on || !g.fogged(st.X, st.Y)) {
			g.setHaulMark(id, on)
		}
	}
}

// migrateClaims marks an older save's loose stone chunks and animal
// carcasses unclaimed: those lying outside any stockpile, not carried and
// not reserved. Anything else stays the colony's.
func (g *Game) migrateClaims() {
	for _, id := range g.itemIDs {
		st, ok := g.stacks.Get(id)
		if !ok || st.Carrier != 0 || st.Unclaimed || !item.IsChunk(st.Kind) && !isCarcass(st.Kind) {
			continue
		}
		if _, stored := g.zoneOf(zone.KindStockpile, st.X, st.Y); stored {
			continue
		}
		if _, taken := g.reserve.EntityOwner(id); taken {
			continue
		}
		st.Unclaimed = true
		g.putStack(id, st)
	}
}

// isCarcass reports an animal's corpse (any species, legacy kinds too).
func isCarcass(k item.Kind) bool { return k.IsCorpse() && k != item.CorpseHuman }
