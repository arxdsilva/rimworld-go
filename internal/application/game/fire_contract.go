package game

import "rimworld-go/internal/domain/world/fire"

// PLAY-015 fire contract (fire.go fills the behaviour).

// FireView is one burning cell.
type FireView struct {
	X, Y int
	Size int // 1..100
}

// fireState is the active fires (fire.go): fires sorted y then x, the
// only saved part; rt holds derived lookups and recent lightning strikes
// (fire.go, lightning.go), never saved.
type fireState struct {
	fires []fire.Fire
	ver   uint64
	rt    fireRuntime
}

// fireAt is the fire size on (x, y) (0: none).
func (g *Game) fireAt(x, y int) int {
	if i := g.fireIndex(x, y); i >= 0 {
		return int(g.fire.fires[i].Size)
	}
	return 0
}

// fireAvoidCost is the extra path cost (percent of a plain step) of
// stepping onto (x, y) because of fire there. Stub: FIRE-JOBS fills it;
// AREA-CORE wires it (with the area penalty) into the costed path query.
func (g *Game) fireAvoidCost(x, y int) int {
	return g.fireDangerCost(x, y) // fire_danger.go
}
