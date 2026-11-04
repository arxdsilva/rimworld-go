package sense

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Wander is free time's fallback: a stroll to a random reachable tile nearby.
type Wander struct {
	walks   Walks
	strolls Strolls
}

// NewWander is the component over its roles.
func NewWander(w Walks, s Strolls) Wander { return Wander{w, s} }

// Pick starts a stroll and a linger of linger ticks at its end.
func (c Wander) Pick(id entity.EntityID, linger int) bool {
	px, py := c.walks.Pos(id)
	r := 5
	if c.strolls.Roams(id) {
		r = 9
	}
	for range 8 {
		x := px + c.strolls.Roll(2*r+1) - r
		y := py + c.strolls.Roll(2*r+1) - r
		if x == px && y == py || !c.strolls.OpenTile(id, x, y) {
			continue
		}
		if c.walks.TileTaken(x, y) {
			continue
		}
		if !c.walks.Reachable(id, x, y) {
			continue
		}
		if c.walks.StartJob(id, job.Record{Kind: job.Wander, TX: x, TY: y, WorkLeft: linger}) {
			return true
		}
	}
	return false
}
