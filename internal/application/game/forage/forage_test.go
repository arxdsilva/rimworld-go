package forage

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/region"
)

// grid is a fake label map: one row of labels per y, -1 shut.
type grid [][]int32

func (g grid) Label(_ region.Layer, x, y int) int32 {
	if y < 0 || y >= len(g) || x < 0 || x >= len(g[y]) {
		return -1
	}
	return g[y][x]
}

func TestReach(t *testing.T) {
	t.Parallel()
	m := grid{
		{0, 0, -1, 1, 1},
		{0, -1, -1, 1, 1},
		{0, 0, -1, 1, 1},
	}
	fence := make([]uint8, 15)
	fence[2] = 1 // (2, 0) is a fence
	if Reach(m, region.Wild, -1, 0, 5, 3, nil) != nil || Reach(m, region.Wild, 5, 0, 5, 3, nil) != nil {
		t.Error("off the map: no filter")
	}
	ok := Reach(m, region.Wild, 0, 0, 5, 3, fence)
	for _, c := range []struct {
		x, y int
		want bool
	}{{1, 0, true}, {0, 2, true}, {3, 0, false}, {2, 0, false}, {1, 1, true}, {2, 1, true}, {-1, 0, false}, {0, 3, false}} {
		if ok(c.x, c.y) != c.want {
			t.Errorf("(%d,%d) = %v, want %v", c.x, c.y, !c.want, c.want)
		}
	}
	// standing on a shut cell: a neighbour's area
	if on := Reach(m, region.Wild, 2, 1, 5, 3, nil); on(1, 0) || !on(4, 0) { // north-east (3, 0) comes first
		t.Error("on a shut cell the first open neighbour's area counts")
	}
	if boxed := Reach(grid{{-1, -1, -1}, {-1, -1, -1}, {-1, -1, -1}}, region.Wild, 1, 1, 3, 3, nil); boxed(0, 0) {
		t.Error("boxed in: nothing in reach")
	}
}

func TestPickFeedBeforeMeal(t *testing.T) {
	t.Parallel()
	var p Pick
	if p.Best().OK || p.Within(30) != 30 {
		t.Fatal("empty pick")
	}
	meal := animal.FeedTarget{Kind: animal.FeedItem, ID: 7, X: 1}
	p.Offer(animal.TierMeal, meal, 0, 1, 5000, RankItem)
	if b := p.Best(); !b.OK || b.T != meal || p.Within(30) != 30 {
		t.Fatalf("only a meal: it, and no narrower search: %+v", b)
	}
	far := animal.FeedTarget{Kind: animal.FeedGrass, X: 20}
	p.Offer(animal.TierFeed, far, 0, 20, 1500, RankGrass)
	if b := p.Best(); b.T != far || p.Within(30) != 20 {
		t.Fatalf("feed beats a nearer meal: %+v", b)
	}
	// nearest, then bigger, then lower rank, then lower id
	near := animal.FeedTarget{Kind: animal.FeedItem, ID: 9, X: 5}
	p.Offer(animal.TierFeed, near, 0, 5, 100, RankItem)
	p.Offer(animal.TierFeed, animal.FeedTarget{Kind: animal.FeedItem, ID: 3}, 0, 5, 50, RankItem)
	p.Offer(animal.TierFeed, animal.FeedTarget{Kind: animal.FeedPlant, ID: 2}, 0, 5, 100, RankPlant)
	p.Offer(animal.TierFeed, animal.FeedTarget{Kind: animal.FeedItem, ID: 11}, 0, 5, 100, RankItem)
	if b := p.Best(); b.T != near {
		t.Fatalf("tie order: %+v", b)
	}
	p.Offer(animal.TierFeed, animal.FeedTarget{}, entity.EntityID(4), 5, 100, RankCarcass)
	if b := p.Best(); b.Prey != 4 {
		t.Fatalf("a prey of lower rank wins the tie: %+v", b)
	}
}

func TestMemoryMarks(t *testing.T) {
	t.Parallel()
	var m Memory
	m.GaveUp(1, 3, 4, 100)
	if !Skips(m.Marks(1), 3, 4, 100) || Skips(m.Marks(1), 3, 5, 100) || Skips(m.Marks(2), 3, 4, 100) {
		t.Fatal("a mark holds its spot for its animal")
	}
	if Skips(m.Marks(1), 3, 4, 100+ForgetTicks) {
		t.Fatal("it expires after ForgetTicks")
	}
	for i := 0; i < maxMarks+2; i++ {
		m.GaveUp(1, 10+i, 0, 200)
	}
	if len(m.Marks(1)) != maxMarks || Skips(m.Marks(1), 10, 0, 200) || !Skips(m.Marks(1), 10+maxMarks+1, 0, 200) {
		t.Fatalf("at most maxMarks, the oldest dropped: %+v", m.Marks(1))
	}
	m.GaveUp(2, 1, 1, 200)
	m.Searching(3, 100)
	m.Prune(300, func(id entity.EntityID) bool { return id != 2 })
	if len(m.Marks(2)) != 0 || len(m.Marks(1)) != maxMarks || !m.IsFar(3) {
		t.Fatal("prune drops the gone, keeps the live")
	}
	m.Prune(200+ForgetTicks, func(entity.EntityID) bool { return false })
	if len(m.Skip) != 0 || len(m.Far) != 0 {
		t.Fatalf("all expired or gone: %+v", m)
	}
}

// No flap: Searching keeps the far search between StarveSearch and
// StarveSearchEnd, and a meal ends it.
func TestMemorySearchingNoFlap(t *testing.T) {
	t.Parallel()
	var m Memory
	if m.Searching(1, animal.StarveSearch) || m.IsFar(1) {
		t.Fatal("at the threshold: not yet")
	}
	if !m.Searching(1, animal.StarveSearch-1) || !m.Searching(1, animal.StarveSearchEnd-1) {
		t.Fatal("below 15 %, held until 25 %")
	}
	if m.Searching(1, animal.StarveSearchEnd) || m.Searching(1, animal.StarveSearchEnd-1) {
		t.Fatal("ends at 25 % and does not restart above 15 %")
	}
	m.Searching(1, 10)
	m.Ate(1)
	if m.IsFar(1) || m.Searching(1, animal.StarveSearch+1) {
		t.Fatal("a meal ends it")
	}
}
