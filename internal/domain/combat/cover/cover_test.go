package cover

import "testing"

type grid map[[2]int]Kind

func (g grid) CoverAt(x, y int) Kind { return g[[2]int{x, y}] }

func TestPermille(t *testing.T) {
	t.Parallel()
	if Permille(None) != 0 || Permille(Kind(99)) != 0 {
		t.Fatal("none")
	}
	if Permille(Sandbags) != 550 || Permille(Barricade) != 550 || Permille(Chunk) != 500 || Permille(Tree) != 250 || Permille(Wall) != 750 {
		t.Fatal("ordering")
	}
}

func TestBestOnShooterSide(t *testing.T) {
	t.Parallel()
	g := grid{{4, 5}: Tree, {5, 4}: Sandbags, {4, 4}: Chunk, {6, 5}: Wall, {5, 6}: Wall}
	// shooter up-left of target (5,5): candidates (4,5) (5,4) (4,4)
	if k, p := Best(g, 0, 0, 5, 5); k != Sandbags || p != Permille(Sandbags) {
		t.Fatalf("diag: %v %d", k, p)
	}
	// shooter due west: only (4,5)
	if k, _ := Best(g, 0, 5, 5, 5); k != Tree {
		t.Fatalf("west: %v", k)
	}
	// shooter due north: only (5,4)
	if k, _ := Best(g, 5, 0, 5, 5); k != Sandbags {
		t.Fatalf("north: %v", k)
	}
	// shooter east: wall at (6,5) wins; walls behind target ignored
	if k, _ := Best(g, 9, 5, 5, 5); k != Wall {
		t.Fatalf("east: %v", k)
	}
	if k, p := Best(g, 5, 5, 5, 5); k != None || p != 0 {
		t.Fatal("same tile")
	}
	if k, _ := Best(grid{}, 0, 0, 5, 5); k != None {
		t.Fatal("empty")
	}
	// tie keeps first found (horizontal before vertical)
	tie := grid{{4, 5}: Barricade, {5, 4}: Sandbags}
	if k, _ := Best(tie, 0, 0, 5, 5); k != Sandbags && k != Barricade {
		t.Fatal("tie")
	} else if k != Barricade {
		t.Fatalf("tie order: %v", k)
	}
}

func TestDefenceKinds(t *testing.T) {
	t.Parallel()
	want := map[Kind]int{Door: 750, Cactus: 350, Fence: 250, Bush: 200, Stool: 200, Table: 400, Chair: 350, Turret: 400, HeavyTurret: 500}
	for k, p := range want {
		if Permille(k) != p {
			t.Errorf("kind %d: %d, want %d", k, Permille(k), p)
		}
	}
	g := grid{{4, 5}: Chair, {4, 4}: Door}
	if k, p, x, y := BestAt(g, 0, 0, 5, 5); k != Door || p != 750 || x != 4 || y != 4 {
		t.Fatalf("BestAt: %v %d (%d,%d)", k, p, x, y)
	}
	if k, _, x, y := BestAt(grid{}, 0, 0, 5, 5); k != None || x != 5 || y != 5 {
		t.Fatalf("BestAt none: %v (%d,%d)", k, x, y)
	}
}
