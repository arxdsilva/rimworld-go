package mapgen

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/faction"
)

// TestCellCampDeterministicAndEnclosed: the same seed lays the same camp
// and cell; the cell's inside is open floor in bounds, and from it only
// the one door leads out: with the door shut the flood stays inside, with
// it open it leaves.
func TestCellCampDeterministicAndEnclosed(t *testing.T) {
	t.Parallel()
	for _, tech := range []faction.Tech{faction.Neolithic, faction.Industrial} {
		for seed := uint64(1); seed <= 40; seed++ {
			cfg := CompoundConfig{W: 150, H: 150, Tech: tech, Seed: seed}
			c, cell := CellCamp(cfg)
			c2, cell2 := CellCamp(cfg)
			if !reflect.DeepEqual(c, c2) || cell != cell2 {
				t.Fatalf("tech %d seed %d: two layouts differ", tech, seed)
			}
			solid := compoundCells(t, c) // also: in the compound, no overlaps
			if cell.X0 <= c.X0 || cell.X1 >= c.X1 || cell.Y0 <= c.Y0 || cell.Y1 >= c.Y1 {
				t.Fatalf("seed %d: cell %+v outside the compound %d,%d-%d,%d", seed, cell, c.X0, c.Y0, c.X1, c.Y1)
			}
			if p, ok := solid[cell.Inside]; ok {
				t.Fatalf("seed %d: the inside is blocked by %v", seed, p.Kind)
			}
			if cell.Inside.X <= cell.X0 || cell.Inside.X >= cell.X1 || cell.Inside.Y <= cell.Y0 || cell.Inside.Y >= cell.Y1 {
				t.Fatalf("seed %d: inside %v is not inside %+v", seed, cell.Inside, cell)
			}
			if d, ok := solid[cell.Door]; !ok || d.Kind != construction.Door {
				t.Fatalf("seed %d: no door at %v", seed, cell.Door)
			}
			inCell := func(p Point) bool {
				return p.X > cell.X0 && p.X < cell.X1 && p.Y > cell.Y0 && p.Y < cell.Y1
			}
			shut := reach(c, cell.Inside, func(p Point) bool {
				s, b := solid[p]
				return b && (s.Kind == construction.Wall || s.Kind == construction.Door)
			})
			for p := range shut {
				if !inCell(p) {
					t.Fatalf("seed %d: shut in, the flood reached %v", seed, p)
				}
			}
			if len(shut) != cellInner*cellInner {
				t.Fatalf("seed %d: %d cells inside, want %d", seed, len(shut), cellInner*cellInner)
			}
			open := reach(c, cell.Inside, func(p Point) bool {
				s, b := solid[p]
				return b && s.Kind != construction.Door || p == cell.Door && false
			})
			out := false
			for p := range open {
				out = out || !inCell(p)
			}
			if !out {
				t.Fatalf("seed %d: the door leads nowhere", seed)
			}
			for _, p := range c.Posts {
				if _, b := solid[p]; b || inCell(p) {
					t.Fatalf("seed %d: post %v is blocked or inside the cell", seed, p)
				}
			}
			if len(c.Posts) == 0 || c.Posts[len(c.Posts)-1] != (Point{cell.Door.X, cell.Door.Y - 1}) {
				t.Fatalf("seed %d: no guard at the door: %v", seed, c.Posts)
			}
			for _, l := range c.Loot {
				if _, b := solid[Point{l.X, l.Y}]; b {
					t.Fatalf("seed %d: loot %+v lies on a solid piece", seed, l)
				}
			}
		}
	}
}

// TestCellCampSeedsVary: seeds move the camp.
func TestCellCampSeedsVary(t *testing.T) {
	t.Parallel()
	a, ca := CellCamp(CompoundConfig{W: 150, H: 150, Tech: faction.Neolithic, Seed: 1})
	for s := uint64(2); s < 12; s++ {
		b, cb := CellCamp(CompoundConfig{W: 150, H: 150, Tech: faction.Neolithic, Seed: s})
		if ca != cb || !reflect.DeepEqual(a, b) {
			return
		}
	}
	t.Fatal("seeds never vary the cell camp")
}
