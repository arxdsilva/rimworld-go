package planet

import (
	"slices"
	"testing"
)

// landPair is two passable tiles of one landmass some hops apart.
func landPair(t *testing.T, p *Planet) (int, int) {
	t.Helper()
	a := -1
	for i := 0; i < p.TileCount(); i++ {
		if p.Land(i) {
			a = i
			break
		}
	}
	if a < 0 {
		t.Fatal("no passable tile")
	}
	// the farthest tile breadth-first within 12 hops over passable land
	seen := map[int]bool{a: true}
	front, far := []int{a}, a
	for d := 0; d < 12 && len(front) > 0; d++ {
		var next []int
		for _, x := range front {
			for _, nb := range p.Neighbours(x) {
				if !seen[int(nb)] && p.Land(int(nb)) {
					seen[int(nb)] = true
					next = append(next, int(nb))
				}
			}
		}
		if len(next) > 0 {
			far = slices.Min(next)
		}
		front = next
	}
	if far == a {
		t.Fatal("no second tile on the landmass")
	}
	return a, far
}

func TestRouteJoinsNeighboursOverLand(t *testing.T) {
	p := world(t, "route", 30)
	a, b := landPair(t, p)
	r := p.Route(a, b)
	if len(r) < 2 || r[0] != a || r[len(r)-1] != b {
		t.Fatalf("route %v does not run %d -> %d", r, a, b)
	}
	for i := 1; i < len(r); i++ {
		if !slices.Contains(p.Neighbours(r[i-1]), int32(r[i])) {
			t.Fatalf("route step %d -> %d joins non-neighbours", r[i-1], r[i])
		}
		if !p.Land(r[i]) {
			t.Fatalf("route crosses water at tile %d", r[i])
		}
	}
	if got := p.Route(a, a); !slices.Equal(got, []int{a}) {
		t.Fatalf("route to itself = %v", got)
	}
}

func TestRouteDeterministic(t *testing.T) {
	p := world(t, "route", 30)
	q := Build(Params{SeedText: "route", Coverage: 30, Rain: 2, Temp: 2, Pop: 1})
	a, b := landPair(t, p)
	r1, r2, r3 := p.Route(a, b), p.Route(a, b), q.Route(a, b)
	if !slices.Equal(r1, r2) || !slices.Equal(r1, r3) {
		t.Fatalf("routes differ: %v / %v / %v", r1, r2, r3)
	}
}

func TestRouteRejectsWaterAndBadTiles(t *testing.T) {
	p := world(t, "route", 30)
	a, _ := landPair(t, p)
	water := -1
	for i := 0; i < p.TileCount(); i++ {
		if p.Water[i] != Land {
			water = i
			break
		}
	}
	if water >= 0 && p.Route(a, water) != nil {
		t.Fatal("a route ends in the water")
	}
	if p.Route(-1, a) != nil || p.Route(a, p.TileCount()) != nil {
		t.Fatal("a route to an invalid tile")
	}
}

func TestRouteFromSeaIce(t *testing.T) {
	p := world(t, "caravan", 30)
	for i := 0; i < p.TileCount(); i++ {
		if p.Land(i) && !p.passable(i) {
			for _, nb := range p.Neighbours(i) {
				if p.Land(int(nb)) {
					if r := p.Route(i, int(nb)); len(r) != 2 {
						t.Fatalf("no route off sea ice %d -> %d: %v", i, nb, r)
					}
					return
				}
			}
		}
	}
	t.Skip("no sea ice next to land on this planet")
}
