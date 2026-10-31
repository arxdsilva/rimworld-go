package farm

import (
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
)

// TestKillCountRoundTrip: a notice's count reads back for every cause, and
// another species' notice never matches.
func TestKillCountRoundTrip(t *testing.T) {
	t.Parallel()
	for _, s := range []plant.Species{plant.Corn, plant.JungleTree, plant.Pine, plant.Rice} {
		for _, c := range []Cause{Cold, Heat, Rot, Starved} {
			for _, k := range []int{1, 2, 37} {
				if got, ok := KillCount(KillText(s, k, c), s, c); !ok || got != k {
					t.Fatalf("KillCount(KillText(%v, %d, %v)) = %d, %v", s, k, c, got, ok)
				}
			}
		}
	}
	if _, ok := KillCount("Frost killed the corn (5 plants)", plant.Wheat, Cold); ok {
		t.Fatal("another species' notice matched")
	}
	if KillText(plant.Rice, 3, Rot) != "The rice died of old age (3 plants)" {
		t.Fatal(KillText(plant.Rice, 3, Rot))
	}
	var fp Pass
	fp.Died(4, plant.Plant{Species: plant.Moss}, Withered)
	fp.Died(5, plant.Plant{Species: plant.Rice}, Rot)
	if !fp.Gone[4] || !fp.Gone[5] || len(fp.Kills) != 1 {
		t.Fatalf("pass %+v", fp)
	}
}

// TestGrowLight (wiki: Sun lamp, Plants): the open sky is full light; under
// a roof a plain lamp gives at most 50 %, a sun lamp in sight 100 % within
// 5.5 tiles.
func TestGrowLight(t *testing.T) {
	t.Parallel()
	all := func(_, _, _, _ int) bool { return true }
	none := func(_, _, _, _ int) bool { return false }
	lamps := []Lamp{{X: 10, Y: 10}}
	for _, c := range []struct {
		roofed bool
		art    float64
		lamps  []Lamp
		x      int
		sees   Sight
		want   int
	}{
		{false, 0, nil, 0, all, 1000},
		{true, 0.7, nil, 0, all, 500},
		{true, 0.3, nil, 0, all, 300},
		{true, 0.7, lamps, 15, all, 1000},
		{true, 0.7, lamps, 15, none, 500},
		{true, 0, lamps, 18, all, 500},
		{true, 0, lamps, 30, all, 0},
	} {
		if got := GrowLight(c.roofed, c.art, c.lamps, c.x, 10, c.sees); got != c.want {
			t.Errorf("%+v: %d, want %d", c, got, c.want)
		}
	}
	if RipeIn(13) != 24 || RipeIn(26) != 48 {
		t.Errorf("RipeIn(13) = %d", RipeIn(13))
	}
}

// TestBlightRolls: the strike takes the origin always and thins out to its
// radius; spread lands within the wiki's 4 tiles, never on the plant
// itself, about once in SpreadOdds passes.
func TestBlightRolls(t *testing.T) {
	t.Parallel()
	near, far := 0, 0
	for r := uint64(0); r < 1000; r++ {
		if !Struck(0, r) {
			t.Fatal("the origin escaped")
		}
		if Struck(StrikeRadius*StrikeRadius+1, r) {
			t.Fatal("struck beyond the radius")
		}
		if Struck(4, plant.Roll(r, 1, 1)) {
			near++
		}
		if Struck(100, plant.Roll(r, 2, 2)) {
			far++
		}
	}
	if near <= far || far == 0 {
		t.Errorf("near %d far %d", near, far)
	}
	hits := 0
	for r := uint64(0); r < 48000; r++ {
		dx, dy, ok := SpreadTo(plant.Roll(r, 3, 3))
		if !ok {
			continue
		}
		hits++
		if dx*dx+dy*dy > plant.BlightRadius*plant.BlightRadius || dx == 0 && dy == 0 {
			t.Fatalf("spread to %d,%d", dx, dy)
		}
	}
	if hits < 500 || hits > 1000 {
		t.Errorf("%d spreads in 48000 passes", hits)
	}
}

// TestSprout: wild plants sprout by the biome's weights until the chunk
// holds its biome's density; wild healroot only in its lands; never in
// the ice sheet.
func TestSprout(t *testing.T) {
	t.Parallel()
	got := map[plant.Species]int{}
	n := 0
	for now := uint64(0); now < 200000; now++ {
		s, x, y, ok := Sprout(biome.TemperateForest, 16, 32, 32, 48, 0, now, 3)
		if !ok {
			continue
		}
		n++
		got[s]++
		if x < 16 || x >= 32 || y < 32 || y >= 48 {
			t.Fatalf("sprout off the chunk at %d,%d", x, y)
		}
	}
	if n < 2000 || n > 5000 || got[plant.Tree] <= got[plant.BerryBush] || got[plant.Healroot] == 0 {
		t.Errorf("%d sprouts: %v", n, got)
	}
	for now := uint64(0); now < 20000; now++ {
		if s, _, _, ok := Sprout(biome.Desert, 0, 0, 16, 16, 0, now, 1); ok && s == plant.Healroot {
			t.Fatal("healroot in the desert")
		}
		if _, _, _, ok := Sprout(biome.TemperateForest, 0, 0, 16, 16, 256, now, 1); ok {
			t.Fatal("a full chunk sprouted")
		}
		if _, _, _, ok := Sprout(biome.IceSheet, 0, 0, 16, 16, 0, now, 1); ok {
			t.Fatal("the ice sheet sprouted")
		}
	}
}
