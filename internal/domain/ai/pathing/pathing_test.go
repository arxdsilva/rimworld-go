package pathing

import (
	"testing"

	"rimworld-go/internal/domain/world/grid"
)

func TestFindAlreadyThere(t *testing.T) {
	g := grid.New(4, 4)
	p, ok := Find(g, 1, 1, 1, 1)
	if !ok {
		t.Fatal("already-there must succeed")
	}
	if len(p) != 0 {
		t.Fatalf("already-there path = %#v, want empty", p)
	}
}

func TestFindStraightLine(t *testing.T) {
	g := grid.New(6, 4)
	p, ok := Find(g, 0, 1, 4, 1)
	if !ok {
		t.Fatal("expected a path")
	}
	if len(p) != 4 {
		t.Fatalf("len = %d, want 4; path=%v", len(p), p)
	}
	if p[len(p)-1] != (Point{X: 4, Y: 1}) {
		t.Fatalf("last = %v, want (4,1)", p[len(p)-1])
	}
	if p[0] == (Point{X: 0, Y: 1}) {
		t.Fatal("path must not include the start cell")
	}
}

func TestFindAvoidsBlockedTiles(t *testing.T) {
	g := grid.New(5, 3)
	g.SetWalkable(1, 1, false)
	g.SetWalkable(2, 1, false)
	g.SetWalkable(3, 1, false)
	p, ok := Find(g, 0, 1, 4, 1)
	if !ok {
		t.Fatal("expected a detour")
	}
	for _, step := range p {
		if !g.Walkable(step.X, step.Y) {
			t.Fatalf("path entered blocked tile %v", step)
		}
	}
	if p[len(p)-1] != (Point{X: 4, Y: 1}) {
		t.Fatalf("last = %v", p[len(p)-1])
	}
}

func TestFindUnreachable(t *testing.T) {
	g := grid.New(5, 3)
	for y := 0; y < 3; y++ {
		g.SetWalkable(2, y, false)
	}
	if _, ok := Find(g, 0, 1, 4, 1); ok {
		t.Fatal("unreachable should fail")
	}
}

func TestFindBlockedGoal(t *testing.T) {
	g := grid.New(4, 4)
	g.SetWalkable(3, 3, false)
	if _, ok := Find(g, 0, 0, 3, 3); ok {
		t.Fatal("blocked goal should fail")
	}
}

func TestFindIsDeterministic(t *testing.T) {
	g := grid.New(8, 8)
	g.SetWalkable(3, 3, false)
	g.SetWalkable(4, 2, false)
	a, oka := Find(g, 1, 1, 6, 6)
	b, okb := Find(g, 1, 1, 6, 6)
	if !oka || !okb {
		t.Fatal("expected paths")
	}
	if len(a) != len(b) {
		t.Fatalf("len %d != %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("step %d diverged: %v vs %v", i, a[i], b[i])
		}
	}
}

func TestFindShortestChebyshev(t *testing.T) {
	g := grid.New(10, 10)
	p, ok := Find(g, 2, 2, 5, 4)
	if !ok {
		t.Fatal("expected path")
	}
	if len(p) != 3 {
		t.Fatalf("len = %d, want chebyshev 3", len(p))
	}
}

func TestFindAllowsUnwalkableStart(t *testing.T) {
	g := grid.New(4, 4)
	g.SetWalkable(0, 0, false)
	p, ok := Find(g, 0, 0, 2, 0)
	if !ok {
		t.Fatal("should path off a blocked start tile")
	}
	if p[len(p)-1] != (Point{X: 2, Y: 0}) {
		t.Fatalf("last = %v", p[len(p)-1])
	}
}
