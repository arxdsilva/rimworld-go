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
