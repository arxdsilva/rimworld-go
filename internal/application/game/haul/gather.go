package haul

import "rimworld-go/internal/domain/sim/entity"

// Radius is how far (Chebyshev tiles) the next pile a load gathers may
// lie from the last one picked up (original tuning).
const Radius = 8

// Cand is a pile a load may gather: its stack id, cell and quantity.
type Cand struct {
	ID   entity.EntityID
	X, Y int
	Qty  int
}

// Pick is a pile a load gathers and how many it takes from it.
type Pick struct {
	ID   entity.EntityID
	Take int
}

// Plan appends to out the piles a load that starts with pile first
// gathers from cands, up to capacity in all (first's share included): a
// greedy chain, each next pile the nearest within radius of the last one
// taken, ties to the lower id, so a hauler sweeps a cluster without
// detours. The last pile may be taken in part. Plan reorders cands (taken
// piles move to the end) and allocates nothing beyond out.
func Plan(out []Pick, first Cand, cands []Cand, capacity, radius int) []Pick {
	left := capacity - min(first.Qty, capacity)
	x, y, n := first.X, first.Y, len(cands)
	for left > 0 {
		best, bestD := -1, 0
		for i, c := range cands[:n] {
			d := max(abs(c.X-x), abs(c.Y-y))
			if c.ID == first.ID || c.Qty <= 0 || d > radius {
				continue
			}
			if best < 0 || d < bestD || d == bestD && c.ID < cands[best].ID {
				best, bestD = i, d
			}
		}
		if best < 0 {
			break
		}
		c := cands[best]
		take := min(c.Qty, left)
		out = append(out, Pick{ID: c.ID, Take: take})
		left -= take
		x, y, n = c.X, c.Y, n-1
		cands[best], cands[n] = cands[n], cands[best]
	}
	return out
}

// Pickups is picks as a job record's Pickups: two entries per pile, the
// stack id then the count (nil for none). The slice is new, so the job
// never shares a caller's buffer.
func Pickups(picks []Pick) []entity.EntityID {
	if len(picks) == 0 {
		return nil
	}
	out := make([]entity.EntityID, 0, 2*len(picks))
	for _, p := range picks {
		out = append(out, p.ID, entity.EntityID(p.Take))
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ClaimAll claims every pile in a job's Pickups for hauler id when the job
// starts; false when one is taken.
func ClaimAll(c Claimer, id entity.EntityID, pickups []entity.EntityID) bool {
	for i := 0; i+1 < len(pickups); i += 2 {
		if !c.TryEntity(id, pickups[i]) {
			return false
		}
	}
	return true
}
