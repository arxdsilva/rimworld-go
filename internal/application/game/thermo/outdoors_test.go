package thermo

import (
	"math/rand/v2"
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/temperature"
)

// unroof takes the roof off the next n roofed cells of room r.
func unroof(f *fake, r, n int) {
	for i, g := range f.grid {
		if g == r && n > 0 && f.roofs[i].Roofed() {
			f.roofs[i] = 0
			n--
		}
	}
	f.rvr++
}

// TestOutdoorsForWork (TEMP-6, wiki: Rooms): a room is outdoors for work
// past 25 % unroofed or past 100 unroofed cells.
func TestOutdoorsForWork(t *testing.T) {
	f := newFake(40, 40, [3]int{2, 2, 10}, [3]int{14, 14, 20})
	m := model(f, 200, 2)
	unroof(f, 0, 25) // 25 of 100: not more than 25 %
	if m.OutdoorsForWork(0) {
		t.Fatal("25 % unroofed counts as outdoors for work")
	}
	unroof(f, 0, 1) // 26 %
	if !m.OutdoorsForWork(0) {
		t.Fatal("26 % unroofed is not outdoors for work")
	}
	unroof(f, 1, 100) // 100 of 400 (25 %): neither rule
	if m.OutdoorsForWork(1) {
		t.Fatal("100 unroofed cells of 400 is outdoors for work")
	}
	unroof(f, 1, 1) // 101 cells
	if !m.OutdoorsForWork(1) {
		t.Fatal("101 unroofed cells is not outdoors for work")
	}
	if m.OutdoorsForWork(room.Wall) {
		t.Fatal("a wall is outdoors for work")
	}
}

// TestPsychologicallyOutdoors (TEMP-6, wiki: Rooms): 300 or more unroofed
// cells, or open to the map edge with at least half of it unroofed.
func TestPsychologicallyOutdoors(t *testing.T) {
	f := newFake(40, 40, [3]int{2, 2, 10}, [3]int{14, 14, 20})
	m := model(f, 200, 2)
	unroof(f, 0, 100) // a fully open 10x10 yard: under 300 cells, no edge
	if m.PsychOutdoors(0) {
		t.Fatal("an open walled 10x10 yard feels outdoors")
	}
	unroof(f, 1, 299)
	if m.PsychOutdoors(1) {
		t.Fatal("299 unroofed cells feel outdoors")
	}
	unroof(f, 1, 1)
	if !m.PsychOutdoors(1) {
		t.Fatal("300 unroofed cells don't feel outdoors")
	}
	if !m.PsychOutdoors(room.Outdoors) {
		t.Fatal("the open map doesn't feel outdoors")
	}
	for i, g := range f.grid { // roof over most of the outside
		if g == room.Outdoors && i%4 != 0 {
			f.roofs[i] = roof.Built
		}
	}
	f.rvr++
	cells, open, _ := m.openOf(room.Outdoors)
	if want := open >= psychOpenCells; m.PsychOutdoors(room.Outdoors) != want || open*2 >= cells {
		t.Fatalf("outdoors %d open of %d: judged %v", open, cells, m.PsychOutdoors(room.Outdoors))
	}
}

// TestSpotMatchesFullScan (TEMP-6): the cached search finds exactly what
// the full-map scan it replaced found, on random maps, bearable sets and
// blocked cells, with the open air bearable in half the trials (AIL-003).
func TestSpotMatchesFullScan(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for trial := range 200 {
		rects := [][3]int{}
		for x := 1; x+6 < 60; x += 7 + rng.IntN(3) {
			for y := 1; y+6 < 40; y += 7 + rng.IntN(3) {
				if rng.IntN(3) > 0 {
					rects = append(rects, [3]int{x, y, 2 + rng.IntN(4)})
				}
			}
		}
		f := newFake(60, 40, rects...)
		m := model(f, 200, len(rects))
		warm := map[int]bool{}
		for r := range rects {
			warm[r] = rng.IntN(3) == 0
		}
		blocked := map[int]bool{}
		for range 60 {
			blocked[rng.IntN(60*40)] = true
		}
		open := trial%2 == 1
		bearable := func(r int) bool { return r >= 0 && warm[r] || r == room.Outdoors && open }
		free := func(x, y int) bool { return !blocked[y*60+x] }
		far := rng.IntN(20)
		reach := func(x, y int) bool { return (x+y)%7 != far%7 }
		px, py := rng.IntN(60), rng.IntN(40)
		gx, gy, gok := m.Spot(px, py, bearable, free, reach)
		wx, wy, wok := fullScan(f.grid, 60, px, py, bearable, free, reach)
		if gx != wx || gy != wy || gok != wok {
			t.Fatalf("trial %d from %d,%d: cached %d,%d %v, scan %d,%d %v", trial, px, py, gx, gy, gok, wx, wy, wok)
		}
	}
}

// fullScan is the warmth search before TEMP-6 (bodytemp.Spot): every cell
// of the map, nearest spotTries path-checked.
func fullScan(rooms []int, w, x, y int, bearable func(int) bool, free, reach func(x, y int) bool) (int, int, bool) {
	var spots []spot
	for i, r := range rooms {
		if bearable(r) && free(i%w, i/w) {
			spots = append(spots, spot{i, pathing.Dist(x, y, i%w, i/w)})
		}
	}
	slices.SortFunc(spots, func(a, b spot) int {
		if a.d != b.d {
			return a.d - b.d
		}
		return a.i - b.i
	})
	for _, s := range spots[:min(len(spots), spotTries)] {
		if reach(s.i%w, s.i/w) {
			return s.i % w, s.i / w, true
		}
	}
	return 0, 0, false
}

// TestSpotOpensOnlyNearRooms: the search never asks about cells of a
// bearable room farther than the twelve nearest candidates.
func TestSpotOpensOnlyNearRooms(t *testing.T) {
	f := newFake(80, 20, [3]int{2, 2, 5}, [3]int{60, 2, 5})
	m := model(f, 200, 2)
	asked := map[int]int{}
	free := func(x, y int) bool { asked[f.grid[y*80+x]]++; return true }
	all := func(int, int) bool { return true }
	if x, _, ok := m.Spot(0, 0, func(r int) bool { return r >= 0 }, free, all); !ok || x != 2 {
		t.Fatalf("nearest spot x %d %v", x, ok)
	}
	if asked[1] != 0 || asked[0] != 25 {
		t.Fatalf("cells asked per room %v, want only the near room's 25", asked)
	}
	if _, _, ok := m.Spot(0, 0, func(int) bool { return false }, free, all); ok {
		t.Fatal("found a spot with no bearable room")
	}
}

// TestIgniteChance (TEMP-6, wiki: Temperature): nothing below 235 C, the
// chance rising with the air above it.
func TestIgniteChance(t *testing.T) {
	if IgniteChance(IgniteAt-1) != 0 || IgniteChance(IgniteAt) != igniteMin || IgniteChance(5000) <= IgniteChance(3000) ||
		IgniteChance(20_000) != igniteMax {
		t.Fatalf("chances %d %d %d %d", IgniteChance(IgniteAt-1), IgniteChance(IgniteAt), IgniteChance(3000), IgniteChance(20_000))
	}
}

// TestHotRoomsAndCooldown: hot rooms are listed, their cells walked, and
// the ignition cooldown holds IgniteCooldown ticks (saved).
func TestHotRoomsAndCooldown(t *testing.T) {
	f := newFake(30, 30, [3]int{2, 2, 4}, [3]int{10, 10, 3})
	m := model(f, 200, 2)
	m.Temps[1] = 2400
	if got := m.HotRooms(nil, IgniteAt); !slices.Equal(got, []int{1}) || m.Hottest() != 2400 {
		t.Fatalf("hot rooms %v hottest %v", got, m.Hottest())
	}
	m.Residual = map[int]temperature.Temp{0: 3000}
	if m.Hottest() != 3000 {
		t.Fatalf("residual air not counted: %v", m.Hottest())
	}
	n := 0
	m.EachCell(1, func(int) bool { n++; return n < 4 })
	if n != 4 {
		t.Fatalf("walked %d cells, want a stop at 4", n)
	}
	if !m.MayIgnite(0) {
		t.Fatal("cooldown before any ignition")
	}
	m.Ignited(1000)
	if m.MayIgnite(1000+IgniteCooldown-1) || !m.MayIgnite(1000+IgniteCooldown) {
		t.Fatal("cooldown length")
	}
	s := m.Export()
	m2 := New(f)
	m2.Import(m.Temps, m.Residual, s)
	if m2.MayIgnite(1000 + IgniteCooldown - 1) {
		t.Fatal("the cooldown is lost on load")
	}
}

// TestAirIsTheRoomsOrTheOpenAirs (AIL-003): a room's held air, the open
// air for room.Outdoors, and nothing for a wall or an unknown room.
func TestAirIsTheRoomsOrTheOpenAirs(t *testing.T) {
	m := model(newFake(20, 10, [3]int{2, 2, 4}), 200, 1)
	m.Temps[0] = 310
	for _, c := range []struct {
		r    int
		want temperature.Temp
		ok   bool
	}{{0, 310, true}, {room.Outdoors, 150, true}, {room.Wall, 0, false}, {5, 0, false}} {
		if got, ok := m.Air(c.r, 150); got != c.want || ok != c.ok {
			t.Errorf("Air(%d) = %d %v, want %d %v", c.r, got, ok, c.want, c.ok)
		}
	}
}
