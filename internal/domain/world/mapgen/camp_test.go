package mapgen

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/faction"
)

// TestCampLayout: the same seed lays the same camp and seeds vary it; it
// is smaller than a settlement; every piece lies in the compound and the
// map, walls close it but for one gate, through which the yard, the beds
// and the store are reached; every post is open ground on the map, inside
// or just outside the walls and within the defenders' reach of the centre.
func TestCampLayout(t *testing.T) {
	t.Parallel()
	for _, tech := range []faction.Tech{faction.Neolithic, faction.Industrial} {
		for _, size := range [][2]int{{150, 150}, {60, 60}} {
			cfg := CompoundConfig{W: size[0], H: size[1], Tech: tech, Seed: 42}
			c := Camp(cfg)
			if !reflect.DeepEqual(c, Camp(cfg)) {
				t.Fatalf("tech %d: the same seed laid two camps", tech)
			}
			varied := false
			for s := uint64(1); s < 8 && !varied; s++ {
				o := cfg
				o.Seed = s
				varied = !reflect.DeepEqual(c, Camp(o))
			}
			if !varied {
				t.Errorf("tech %d: seeds never vary the camp", tech)
			}
			s := Settlement(cfg)
			if area := (c.X1 - c.X0 + 1) * (c.Y1 - c.Y0 + 1); area >= (s.X1-s.X0+1)*(s.Y1-s.Y0+1) {
				t.Errorf("tech %d: a camp of %d cells is not smaller than a settlement", tech, area)
			}
			if c.X0 < edgeMargin || c.Y0 < edgeMargin || c.X1 >= cfg.W-edgeMargin || c.Y1 >= cfg.H-edgeMargin {
				t.Fatalf("tech %d: camp %d,%d-%d,%d breaks the edge margin of a %v map", tech, c.X0, c.Y0, c.X1, c.Y1, size)
			}
			checkCamp(t, c, cfg)
		}
	}
}

func checkCamp(t *testing.T, c Compound, cfg CompoundConfig) {
	t.Helper()
	solid := compoundCells(t, c) // every piece inside the compound, none overlapping
	gates := 0
	for x := c.X0; x <= c.X1; x++ {
		for y := c.Y0; y <= c.Y1; y++ {
			if x != c.X0 && x != c.X1 && y != c.Y0 && y != c.Y1 {
				continue
			}
			p, ok := solid[Point{x, y}]
			if !ok || p.Kind != construction.Wall && p.Kind != construction.Door {
				t.Fatalf("the wall has a gap at %d,%d", x, y)
			}
			if p.Kind == construction.Door {
				gates++
			}
		}
	}
	if gates != 1 {
		t.Errorf("%d gates, want 1", gates)
	}
	shut := func(p Point) bool {
		s, ok := solid[p]
		return ok && (construction.DefOf(s.Kind).BlocksMove || s.Kind == construction.Door)
	}
	open := func(p Point) bool {
		s, ok := solid[p]
		return ok && construction.DefOf(s.Kind).BlocksMove
	}
	if out := reach(c, Point{c.X0 - 1, c.Y0 - 1}, shut); out[c.Centre] {
		t.Fatal("the yard is open without the door")
	}
	in := reach(c, Point{c.X0 - 1, c.Y0 - 1}, open)
	if !in[c.Centre] {
		t.Fatal("the door does not let anyone in")
	}
	beds, fires := 0, 0
	for _, p := range c.Pieces {
		switch p.Kind {
		case construction.Bed:
			beds++
			if !in[Point{p.X, p.Y + 2}] && !in[Point{p.X + 1, p.Y}] && !in[Point{p.X - 1, p.Y}] {
				t.Errorf("the bed at %d,%d cannot be reached", p.X, p.Y)
			}
		case construction.Campfire, construction.Table:
			fires++
		}
	}
	if beds != bedsFor(cfg.Tech) || fires != 1 {
		t.Errorf("tech %d: %d beds, %d campfires or tables", cfg.Tech, beds, fires)
	}
	if len(c.Loot) == 0 {
		t.Fatal("an empty store")
	}
	for _, l := range c.Loot {
		if l.Qty <= 0 || !in[Point{l.X, l.Y}] || l.X <= c.X0 || l.X >= c.X1 || l.Y <= c.Y0 || l.Y >= c.Y1 {
			t.Errorf("loot %v x%d at %d,%d is not in the yard", l.Kind, l.Qty, l.X, l.Y)
		}
	}
	inside, outside := 0, 0
	for _, p := range c.Posts {
		if p.X < 0 || p.Y < 0 || p.X >= cfg.W || p.Y >= cfg.H || !in[p] || shut(p) {
			t.Errorf("post %v is not open ground on the map", p)
		}
		if dx, dy := p.X-c.Centre.X, p.Y-c.Centre.Y; dx*dx+dy*dy > 8*8 {
			t.Errorf("post %v is out of the defenders' leash of %v", p, c.Centre)
		}
		if p.X > c.X0 && p.X < c.X1 && p.Y > c.Y0 && p.Y < c.Y1 {
			inside++
		} else {
			outside++
		}
	}
	if inside == 0 || outside == 0 {
		t.Errorf("posts %d inside, %d outside: want both", inside, outside)
	}
}
