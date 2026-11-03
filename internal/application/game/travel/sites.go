package travel

import (
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// World tab helpers moved here from the game package (AX-9 pays for its
// game lines with them): the settlement list, names and member lists.

// SiteView is one settlement a caravan or pods may head for (the game's
// WorldSiteView).
type SiteView struct {
	Tile     int
	Name     string
	Faction  string
	Relation string // "friendly", "neutral" or "hostile"
	// Visitable: a caravan is received there (a human people, not hostile).
	Visitable bool
}

// RelationWord is a relation's word in lists.
func RelationWord(r faction.Relation) string {
	switch r {
	case faction.Hostile:
		return "hostile"
	case faction.Friendly:
		return "friendly"
	}
	return "neutral"
}

// SettlementViews lists every settlement of the peoples fs (the defeated
// left out), faction by faction, capital first; name names a tile whose
// faction recorded no name ("" none: "a settlement of <faction>").
func SettlementViews(fs []faction.Faction, name func(t int) string) []SiteView {
	var out []SiteView
	for i := range fs {
		f := &fs[i]
		if f.Defeated {
			continue
		}
		for k, t := range f.Settlements {
			n := ""
			if k < len(f.SettlementNames) {
				n = f.SettlementNames[k]
			}
			if n == "" {
				n = name(t)
			}
			if n == "" {
				n = "a settlement of " + f.Name
			}
			out = append(out, SiteView{Tile: t, Name: n, Faction: f.Name, Relation: RelationWord(f.Relation),
				Visitable: f.Type.Human() && f.Relation != faction.Hostile})
		}
	}
	return out
}

// JoinNames is "A", "A and B" or "A, B and C" ("Nobody" for none).
func JoinNames(ns []string) string {
	switch len(ns) {
	case 0:
		return "Nobody"
	case 1:
		return ns[0]
	}
	return strings.Join(ns[:len(ns)-1], ", ") + " and " + ns[len(ns)-1]
}

// ParseIDs reads a comma-separated id list, keeping those keep accepts,
// each once, ascending.
func ParseIDs(list string, keep func(entity.EntityID) bool) []entity.EntityID {
	var out []entity.EntityID
	for _, f := range strings.Split(list, ",") {
		n, err := strconv.ParseUint(strings.TrimSpace(f), 10, 64)
		if id := entity.EntityID(n); err == nil && !slices.Contains(out, id) && keep(id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// EdgeTile is a walkable map-edge cell of a w x h map from which reach
// says the colony can be got to: up to 200 edge cells drawn with roll
// (side, then place), dry ones only, at most 12 of them tried for reach;
// with none reachable, the first dry one (ok false: none dry at all).
func EdgeTile(w, h int, roll func(n int) int, dry, reach func(x, y int) bool) (int, int, bool) {
	var fx, fy int
	found, checks := false, 0
	for try := 0; try < 200; try++ {
		var x, y int
		switch roll(4) {
		case 0:
			x, y = roll(w), 0
		case 1:
			x, y = roll(w), h-1
		case 2:
			x, y = 0, roll(h)
		default:
			x, y = w-1, roll(h)
		}
		if !dry(x, y) {
			continue
		}
		if !found {
			fx, fy, found = x, y, true
		}
		if checks++; checks > 12 {
			break
		}
		if reach(x, y) {
			return x, y, true
		}
	}
	return fx, fy, found
}

// Ring is the first cell around (x, y), ring by ring out to r (Chebyshev;
// each ring row by row, top to bottom, left to right), that ok accepts.
func Ring(x, y, r int, ok func(x, y int) bool) (int, int, bool) {
	for d := 0; d <= r; d++ {
		for dy := -d; dy <= d; dy++ {
			for dx := -d; dx <= d; dx++ {
				if max(mathx.Abs(dx), mathx.Abs(dy)) == d && ok(x+dx, y+dy) {
					return x + dx, y + dy, true
				}
			}
		}
	}
	return 0, 0, false
}

// ExitBeside is a cell beside cells (each cell's eight neighbours in
// turn) that open accepts and busy does not, else the first open but busy
// one (ok false: none open): where a casket's occupant climbs out (moved
// out of package game by AX-9).
func ExitBeside(cells [][2]int, open, busy func(x, y int) bool) (int, int, bool) {
	var fallback [2]int
	found := false
	for _, c := range cells {
		for _, d := range [8][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
			x, y := c[0]+d[0], c[1]+d[1]
			switch {
			case !open(x, y):
			case !busy(x, y):
				return x, y, true
			case !found:
				fallback, found = [2]int{x, y}, true
			}
		}
	}
	return fallback[0], fallback[1], found
}

// CaravanView is one caravan on the World tab (the game's CaravanView);
// Progress is the percent to its next tile.
type CaravanView struct {
	ID      uint64
	Members []string
	// Health is each member's short health note, parallel to Members
	// ("bleeding", "flu", "food poisoning"; "" when well; WS-M).
	Health     []string
	Tile, Dest int
	Progress   int
	State      string
}
