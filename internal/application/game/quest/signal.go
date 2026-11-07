package quest

import (
	qd "rimworld-go/internal/domain/quest"

	"rimworld-go/internal/domain/sim/entity"
)

// SignalKind is what happened. Append-only: kinds switch on it.
type SignalKind uint8

const (
	SigNone SignalKind = iota
	// SigCaravanArrived: Caravan reached world Tile (maps Fleet.Arrived).
	SigCaravanArrived
	// SigSiteOpened: a site map Map opened at world Tile.
	SigSiteOpened
	// SigSiteLost: the world object at Tile was destroyed or abandoned.
	SigSiteLost
	// SigSiteCensus: map Map's census; Count is the hostiles standing.
	SigSiteCensus
	// SigMapClosed: map Map closed.
	SigMapClosed
	// SigPawnJoined, SigPawnFreed, SigPawnDied, SigPawnLeft: Pawn joined
	// the colony, was freed, died, or left the map for good.
	SigPawnJoined
	SigPawnFreed
	SigPawnDied
	SigPawnLeft
	// SigFactionDefeated: Faction lost its last settlement.
	SigFactionDefeated
	// SigFulfil and SigNegotiate come from Apply (Fulfil, Negotiate): the
	// player hands over a request's goods, or opens talks, with Caravan.
	SigFulfil
	SigNegotiate
)

// byTile: the kind's Tile is set and routes the signal.
func (k SignalKind) byTile() bool {
	return k == SigCaravanArrived || k == SigSiteOpened || k == SigSiteLost
}

// Signal is a typed game event. It reaches only the live quests it
// concerns, found through indexes: Quest directly when set, else every
// quest with a part on Map, a part bound to Pawn, a site, delivery or talk
// part on Tile (tile kinds only, see byTile), or Faction as giver or
// target. Zero fields route nothing.
type Signal struct {
	Kind  SignalKind
	Quest qd.ID
	Map   uint32
	Pawn  entity.EntityID
	// Tile is a world tile (routes only the tile kinds).
	Tile    int
	Caravan uint64
	// Faction is 1 + index (0: none), as qd.Quest.Giver.
	Faction int
	Count   int
}
