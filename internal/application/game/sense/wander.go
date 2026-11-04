package sense

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/entity"
)

const shelterRadius = 12 // how far a pawn out in bad weather looks for a roof

// Wander is free time's fallback: a stroll to a random reachable tile nearby.
// With ShelterWander on it stays under a roof in bad weather (CSE-041); with
// PoliteWander on it keeps out of private rooms (CSE-042). Both off, it draws
// the same numbers and picks the same tiles as before they existed.
type Wander struct {
	walks   Walks
	strolls Strolls
	rules   gameplay.Settings
	shelter Shelter
	manners Manners
}

// NewWander is the component over its roles; sh and m are read only when
// their rules are on.
func NewWander(w Walks, s Strolls, rules gameplay.Settings, sh Shelter, m Manners) Wander {
	return Wander{w, s, rules, sh, m}
}

// Pick starts a stroll and a linger of linger ticks at its end.
func (c Wander) Pick(id entity.EntityID, linger int) bool {
	px, py := c.walks.Pos(id)
	hide := c.rules.On(gameplay.ShelterWander) && c.shelter.ShouldHide(id)
	if hide && !c.shelter.Roofed(px, py) {
		return c.seekRoof(id, px, py, linger) // else stay put
	}
	polite := c.rules.On(gameplay.PoliteWander)
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
		if hide && !c.covered(px, py, x, y) || polite && c.manners.Rude(id, x, y) {
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

// covered: the goal is a roofed Home cell and the straight way there is
// roofed at its quarter points (a path through open sky is no shelter).
func (c Wander) covered(px, py, x, y int) bool {
	if !c.shelter.Roofed(x, y) || !c.shelter.Home(x, y) {
		return false
	}
	for q := 1; q < 4; q++ {
		if !c.shelter.Roofed(px+(x-px)*q/4, py+(y-py)*q/4) {
			return false
		}
	}
	return true
}

// seekRoof walks a pawn out in the weather to the nearest roofed Home cell
// within shelterRadius, ring by ring (no flood fill, no random draw).
func (c Wander) seekRoof(id entity.EntityID, px, py, linger int) bool {
	polite := c.rules.On(gameplay.PoliteWander)
	tries := 0
	for r := 1; r <= shelterRadius; r++ {
		for dy := -r; dy <= r; dy++ {
			step := 2 * r
			if dy == -r || dy == r {
				step = 1
			}
			for dx := -r; dx <= r; dx += step {
				x, y := px+dx, py+dy
				if !c.shelter.Roofed(x, y) || !c.shelter.Home(x, y) || !c.strolls.OpenTile(id, x, y) || c.walks.TileTaken(x, y) {
					continue
				}
				if polite && c.manners.Rude(id, x, y) {
					continue
				}
				if tries++; tries > 6 {
					return false
				}
				if c.walks.Reachable(id, x, y) && c.walks.StartJob(id, job.Record{Kind: job.Wander, TX: x, TY: y, WorkLeft: linger}) {
					return true
				}
			}
		}
	}
	return false
}
