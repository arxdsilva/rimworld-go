package bodytemp

import (
	"slices"

	"rimworld-go/internal/domain/ai/pathing"
)

// spotTries is how many of the nearest candidate spots get a path check.
const spotTries = 12

// Spot is the nearest bearable cell for a pawn at (x, y) suffering
// exposure, on a map of width w whose cells map to rooms by rooms:
// bearable says a room's air will do, free a cell is walkable and open to
// the pawn, reach the pawn can walk there. Candidates go nearest first
// (ties by cell index), and only the spotTries nearest are path-checked.
// ok is false when there is none.
func Spot(rooms []int, w, x, y int, bearable func(room int) bool, free, reach func(x, y int) bool) (int, int, bool) {
	type spot struct{ i, d int }
	var spots []spot
	for i, r := range rooms {
		if !bearable(r) || !free(i%w, i/w) {
			continue
		}
		spots = append(spots, spot{i, pathing.Dist(x, y, i%w, i/w)})
	}
	slices.SortFunc(spots, func(a, b spot) int {
		if a.d != b.d {
			return a.d - b.d
		}
		return a.i - b.i
	})
	for _, s := range spots[:min(len(spots), spotTries)] {
		if reach(s.i%w, s.i/w) {
			return s.i % w, s.i / w, true
		}
	}
	return 0, 0, false
}
