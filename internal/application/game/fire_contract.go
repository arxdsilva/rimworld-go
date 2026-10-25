package game

import "rimworld-go/internal/domain/world/fire"

// PLAY-015 fire contract (fire.go fills the behaviour).

// FireView is one burning cell.
type FireView struct {
	X, Y int
	Size int // 1..100
}

// fireState is the active fires. Stub: FIRE-SIM.
type fireState struct {
	fires []fire.Fire
	ver   uint64
}

// fireAt is the fire size on (x, y) (0: none).
func (g *Game) fireAt(x, y int) int {
	for _, f := range g.fire.fires {
		if f.X == x && f.Y == y {
			return int(f.Size)
		}
	}
	return 0
}

// fireAvoidCost is the extra path cost (percent of a plain step) of
// stepping onto (x, y) because of fire there. Stub: FIRE-JOBS fills it;
// AREA-CORE wires it (with the area penalty) into the costed path query.
func (g *Game) fireAvoidCost(x, y int) int {
	if g.fireAt(x, y) > 0 {
		return 1900
	}
	return 0
}
