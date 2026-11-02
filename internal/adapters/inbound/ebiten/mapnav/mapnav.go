// Package mapnav is the pure logic of moving between open maps (MAP-1): the
// colonist bar's groups, the next and previous map for the cycle keys, and
// the map to show when the one on screen closes. It has no ebiten import and
// reads only views.
package mapnav

import "rimworld-go/internal/application/game"

// Group is one open map and the colonists standing on it.
type Group struct {
	Map     game.MapID
	Label   string
	Kind    string
	Threat  bool // hostiles on the map
	Current bool // the map on screen
	Pawns   []game.PawnView
}

// Groups splits colonists by the map they stand on, in map order. Every
// open map has a group (empty when nobody is there); colonists on a map
// that is not open are dropped. cur marks the Current group.
func Groups(maps []game.MapView, colonists []game.PawnView, cur game.MapID) []Group {
	out := make([]Group, len(maps))
	at := make(map[game.MapID]int, len(maps))
	for i, m := range maps {
		out[i] = Group{Map: m.ID, Label: m.Label, Kind: m.Kind, Threat: m.Threat, Current: m.ID == cur}
		at[m.ID] = i
	}
	for _, p := range colonists {
		if i, ok := at[p.Map]; ok {
			out[i].Pawns = append(out[i].Pawns, p)
		}
	}
	return out
}

// Next is the map after cur (dir > 0) or before it (dir < 0), wrapping.
// An unknown cur gives the first (or last) map; with no maps, cur.
func Next(maps []game.MapView, cur game.MapID, dir int) game.MapID {
	n := len(maps)
	if n == 0 {
		return cur
	}
	for i, m := range maps {
		if m.ID != cur {
			continue
		}
		if dir < 0 {
			return maps[(i+n-1)%n].ID
		}
		return maps[(i+1)%n].ID
	}
	if dir < 0 {
		return maps[n-1].ID
	}
	return maps[0].ID
}

// Fallback is the map to show once closed goes away: the first colony
// (the lowest id still open), else any other map; 0 when none is left.
func Fallback(maps []game.MapView, closed game.MapID) game.MapID {
	for _, m := range maps {
		if m.ID != closed {
			return m.ID
		}
	}
	return 0
}
