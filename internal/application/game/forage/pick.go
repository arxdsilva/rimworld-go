package forage

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// Tie ranks of a tame animal's meal sources (a full tie of distance and
// meal goes to the lower rank, then the lower id).
const (
	RankCarcass = iota
	RankItem
	RankGrass
	RankPlant
	RankPrey
)

// Best is the best meal of one tier so far: the nearest, then the bigger
// meal (what it would eat of it, up to what it needs), then the lower rank
// and id. Prey is set for a hunt.
type Best struct {
	T          animal.FeedTarget
	Prey       entity.EntityID
	OK         bool
	d, v, rank int
	id         entity.EntityID
}

func (b *Best) offer(t animal.FeedTarget, prey entity.EntityID, d, v, rank int) {
	id := t.ID
	if prey != 0 {
		id = prey
	}
	if b.OK && (d > b.d || d == b.d && (v < b.v || v == b.v && (rank > b.rank || rank == b.rank && id >= b.id))) {
		return
	}
	*b = Best{T: t, Prey: prey, OK: true, d: d, v: v, rank: rank, id: id}
}

// Pick is a tame animal's food search (FEED-001): animal feed first, a
// meal only when no feed is in reach, each tier ordered as Best. One pass
// fills both slots.
type Pick struct{ feed, meal Best }

// Offer puts a candidate of tier into the search.
func (p *Pick) Offer(tier animal.Tier, t animal.FeedTarget, prey entity.EntityID, d, v, rank int) {
	if tier == animal.TierMeal {
		p.meal.offer(t, prey, d, v, rank)
		return
	}
	p.feed.offer(t, prey, d, v, rank)
}

// Within is the radius a feed source still has to be searched to: r, or
// the best feed's distance once there is one (a tie may still win on
// size). A meal never narrows it: any feed beats it.
func (p *Pick) Within(r int) int {
	if p.feed.OK {
		return min(r, p.feed.d)
	}
	return r
}

// Best is the pick: the best feed, else the best meal.
func (p *Pick) Best() Best {
	if p.feed.OK {
		return p.feed
	}
	return p.meal
}
