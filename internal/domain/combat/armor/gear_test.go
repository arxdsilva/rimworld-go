package armor

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestMoveOffsetWiki: the wiki's move speed penalties (cells/s) and their
// permille of a human's 4.6 cells/s.
func TestMoveOffsetWiki(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		k        item.Kind
		hundreds int
		permille int32
	}{
		{item.FlakVest, -12, -26}, {item.FlakPants, -12, -26}, {item.MarineArmor, -25, -54},
		{item.PlateArmor, -80, -174}, {item.FlakJacket, 0, 0}, {item.MarineHelmet, 0, 0}, {item.Parka, 0, 0},
	} {
		if got := MoveOffset(c.k); got != c.hundreds {
			t.Errorf("%v: offset %d, want %d", c.k, got, c.hundreds)
		}
		if got := SpeedPermille(MoveOffset(c.k)); got != c.permille {
			t.Errorf("%v: %d permille, want %d", c.k, got, c.permille)
		}
	}
	// Penalties add before conversion: flak vest and pants -0.24 cells/s.
	if got := SpeedPermille(MoveOffset(item.FlakVest) + MoveOffset(item.FlakPants)); got != -52 {
		t.Errorf("flak set: %d permille, want -52", got)
	}
	if SpeedPermille(46) != 100 || SpeedPermille(0) != 0 {
		t.Error("SpeedPermille: 0.46 cells/s should be 10 % of 4.6")
	}
}

// TestOverallArmor: coverage-weighted, layers adding up per part, capped.
func TestOverallArmor(t *testing.T) {
	t.Parallel()
	if got := Overall(nil); got != (Set{}) {
		t.Fatalf("naked: %+v", got)
	}
	everywhere := Region(0x1ff)
	if got := Overall([]Layer{{Set: Set{Sharp: 400, Blunt: 100, Heat: 50}, Covers: everywhere}}); got != (Set{400, 100, 50}) {
		t.Errorf("one full layer: %+v", got)
	}
	// Two full layers of 1500 cap at 200 % on every part.
	full := Layer{Set: Set{1500, 1500, 1500}, Covers: everywhere}
	if got := Overall([]Layer{full, full}); got != (Set{MaxRating, MaxRating, MaxRating}) {
		t.Errorf("capped: %+v", got)
	}
	// A flak vest covers the torso group: 490 permille of hits.
	vest, _ := Apparel(item.FlakVest)
	if got := Overall([]Layer{vest}); got != (Set{490, 176, 132}) {
		t.Errorf("flak vest: %+v, want {490 176 132}", got)
	}
	// Flak pants add the legs (220): 550 × 220 / 1000 = 121.
	pants, _ := Apparel(item.FlakPants)
	if got := Overall([]Layer{vest, pants}); got.Sharp != 490+121 {
		t.Errorf("vest and pants: %+v", got)
	}
}

func TestRegionNames(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		r    Region
		want string
	}{
		{0, ""}, {coatCover, "torso, neck, shoulders, arms"}, {helmetCover, "head"},
		{capCover, "upper head"}, {CoverLegs, "legs"}, {vestCover, "torso, neck"},
	} {
		if got := c.r.Names(); got != c.want {
			t.Errorf("%b: %q, want %q", c.r, got, c.want)
		}
	}
}
