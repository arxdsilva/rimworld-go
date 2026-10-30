package haul

import "rimworld-go/internal/domain/sim/entity"

// Batched material delivery (HAUL-001): a load feeds a route of frames.
const (
	// RouteReach is how far (Chebyshev tiles) the next frame of a route
	// may be from the last one.
	RouteReach = 12
	// RouteFrames caps the frames one load feeds.
	RouteFrames = 8
)

// Share is the part of a hauler's load promised to one frame's slot.
type Share struct {
	Frame entity.EntityID
	Qty   int
}

// Shares splits a load of take over route's slots in order, each up to
// its need (needs[i]); slots the load doesn't reach get no share.
func Shares(route []entity.EntityID, needs []int, take int) []Share {
	out := make([]Share, 0, len(route))
	for i, key := range route {
		q := min(take, needs[i])
		if q <= 0 {
			break
		}
		out = append(out, Share{key, q})
		take -= q
	}
	return out
}

// Route is the slot keys of shares, in order: a delivery job's Route.
func Route(shares []Share) []entity.EntityID {
	out := make([]entity.EntityID, len(shares))
	for i, s := range shares {
		out[i] = s.Frame
	}
	return out
}
