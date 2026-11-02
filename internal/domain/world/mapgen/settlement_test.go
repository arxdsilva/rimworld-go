package mapgen

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
)

// compoundCells maps each cell to the solid (non-floor) piece standing on
// it, failing on two solid pieces sharing a cell or a piece outside the
// compound.
func compoundCells(t *testing.T, c Compound) map[Point]Piece {
	t.Helper()
	solid := map[Point]Piece{}
	for _, p := range c.Pieces {
		cells, _, _ := construction.Footprint(p.Kind, p.Facing, p.X, p.Y)
		for _, xy := range cells {
			at := Point{xy[0], xy[1]}
			if at.X < c.X0 || at.X > c.X1 || at.Y < c.Y0 || at.Y > c.Y1 {
				t.Fatalf("%v at %v lies outside the compound %d,%d-%d,%d", p.Kind, at, c.X0, c.Y0, c.X1, c.Y1)
			}
			if construction.DefOf(p.Kind).Layer == construction.LayerFloor {
				continue
			}
			if old, ok := solid[at]; ok {
				t.Fatalf("%v and %v share %v", old.Kind, p.Kind, at)
			}
			solid[at] = p
		}
	}
	return solid
}

// reach floods the compound from start; blocked stops it.
func reach(c Compound, start Point, blocked func(Point) bool) map[Point]bool {
	seen := map[Point]bool{start: true}
	queue := []Point{start}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, d := range [4]Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := Point{p.X + d.X, p.Y + d.Y}
			if n.X < c.X0-1 || n.X > c.X1+1 || n.Y < c.Y0-1 || n.Y > c.Y1+1 || seen[n] || blocked(n) {
				continue
			}
			seen[n] = true
			queue = append(queue, n)
		}
	}
	return seen
}

// TestSettlementLayoutDeterministicAndPlaced: the same seed lays the same
// compound; seeds vary it; it sits inside the map's margin; walls and two
// gates close it all round; through the doors every room and the store
// are reached; tribes build in wood without turrets, industrial factions
// in stone and steel with turrets and components in the store.
func TestSettlementLayoutDeterministicAndPlaced(t *testing.T) {
	t.Parallel()
	for _, tech := range []faction.Tech{faction.Neolithic, faction.Industrial} {
		cfg := CompoundConfig{W: 150, H: 150, Tech: tech, Seed: 42}
		c := Settlement(cfg)
		if !reflect.DeepEqual(c, Settlement(cfg)) {
			t.Fatalf("tech %d: the same seed laid two compounds", tech)
		}
		varied := false
		for s := uint64(1); s < 8 && !varied; s++ {
			cfg.Seed = s
			varied = !reflect.DeepEqual(c, Settlement(cfg))
		}
		if !varied {
			t.Errorf("tech %d: seeds never vary the compound", tech)
		}
		if c.X0 < edgeMargin || c.Y0 < edgeMargin || c.X1 >= 150-edgeMargin || c.Y1 >= 150-edgeMargin {
			t.Fatalf("tech %d: compound %d,%d-%d,%d breaks the edge margin", tech, c.X0, c.Y0, c.X1, c.Y1)
		}
		solid := compoundCells(t, c)
		gates := 0
		for x := c.X0; x <= c.X1; x++ {
			for y := c.Y0; y <= c.Y1; y++ {
				if x != c.X0 && x != c.X1 && y != c.Y0 && y != c.Y1 {
					continue
				}
				p, ok := solid[Point{x, y}]
				if !ok || p.Kind != construction.Wall && p.Kind != construction.Door {
					t.Fatalf("tech %d: the outer wall has a gap at %d,%d", tech, x, y)
				}
				if p.Kind == construction.Door {
					gates++
				}
			}
		}
		if gates != 2 {
			t.Errorf("tech %d: %d gates, want 2", tech, gates)
		}
		walls := func(p Point) bool {
			s, ok := solid[p]
			return ok && construction.DefOf(s.Kind).BlocksMove || ok && s.Kind == construction.Door
		}
		if out := reach(c, Point{c.X0 - 1, c.Y0 - 1}, walls); out[c.Centre] {
			t.Fatalf("tech %d: the yard is open without a door", tech)
		}
		in := reach(c, c.Centre, func(p Point) bool {
			s, ok := solid[p]
			return ok && construction.DefOf(s.Kind).BlocksMove
		})
		beds, benches, turrets := 0, 0, 0
		for _, p := range c.Pieces {
			switch p.Kind {
			case construction.Bed:
				beds++
				if !in[Point{p.X, p.Y}] {
					t.Errorf("tech %d: the bed at %d,%d cannot be reached", tech, p.X, p.Y)
				}
			case construction.CraftingSpot, construction.MachiningTable:
				benches++
			case construction.SentryTurret:
				turrets++
			case construction.Wall:
				if tech == faction.Neolithic && p.Stuff != item.Wood || tech == faction.Industrial && p.Stuff != item.Steel && !item.IsStoneBlocks(p.Stuff) {
					t.Errorf("tech %d: a %v wall", tech, p.Stuff)
				}
			}
		}
		if beds != 2*roomsFor(tech) || benches != 1 {
			t.Errorf("tech %d: %d beds, %d benches", tech, beds, benches)
		}
		if (turrets > 0) != (tech == faction.Industrial) {
			t.Errorf("tech %d: %d turrets", tech, turrets)
		}
		has := map[item.Kind]bool{}
		for _, l := range c.Loot {
			has[l.Kind] = true
			if l.Qty <= 0 || !in[Point{l.X, l.Y}] {
				t.Errorf("tech %d: loot %v x%d at %d,%d out of reach", tech, l.Kind, l.Qty, l.X, l.Y)
			}
		}
		if !has[item.Silver] || has[item.Components] != (tech == faction.Industrial) {
			t.Errorf("tech %d: store %v", tech, c.Loot)
		}
		if len(c.Posts) == 0 {
			t.Fatal("no posts")
		}
		for _, p := range c.Posts {
			if !in[p] || walls(p) {
				t.Errorf("tech %d: post %v is not open ground inside", tech, p)
			}
		}
	}
}
