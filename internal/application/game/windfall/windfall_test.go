// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windfall

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/pkg/mathx"
)

const hx, hy = 30, 30 // the fake colony's home

type drop struct {
	k    item.Kind
	qty  int
	x, y int
}

type letter struct {
	title  string
	x, y   int
	format string
	args   []any
}

// fakeHost is an in-memory Host: a set of cells a meteor may land on, a
// set a supply stack may land on, and a recorded song, cheer and post log.
type fakeHost struct {
	colonists int
	free      map[[2]int]bool // MeteorFree
	dropOK    map[[2]int]bool // DropFree before the taken rule
	refuse    bool            // StartSong refuses although no song plays
	songUntil uint64
	rocks     [][2]int
	ores      []geology.Ore
	drops     []drop
	starts    [][2]uint64
	cheers    [][2]uint64
	letters   []letter
}

func (f *fakeHost) Home() (int, int)         { return hx, hy }
func (f *fakeHost) Colonists() int           { return f.colonists }
func (f *fakeHost) MeteorFree(x, y int) bool { return f.free[[2]int{x, y}] }
func (f *fakeHost) Rock(x, y int, o geology.Ore) {
	f.rocks, f.ores = append(f.rocks, [2]int{x, y}), append(f.ores, o)
}
func (f *fakeHost) DropFree(x, y int, taken [][2]int) bool {
	if !f.dropOK[[2]int{x, y}] {
		return false
	}
	for _, t := range taken {
		if mathx.Abs(t[0]-x) <= 1 && mathx.Abs(t[1]-y) <= 1 {
			return false
		}
	}
	return true
}
func (f *fakeHost) Drop(k item.Kind, qty, x, y int) { f.drops = append(f.drops, drop{k, qty, x, y}) }
func (f *fakeHost) SongEnds(now uint64) uint64 {
	if now < f.songUntil {
		return f.songUntil
	}
	return 0
}
func (f *fakeHost) StartSong(now, ticks uint64) bool {
	f.starts = append(f.starts, [2]uint64{now, ticks})
	if f.refuse || now < f.songUntil {
		return false
	}
	f.songUntil = now + ticks
	return true
}
func (f *fakeHost) Cheer(now, until uint64) int {
	f.cheers = append(f.cheers, [2]uint64{now, until})
	return f.colonists
}
func (f *fakeHost) Letter(title string, x, y int, format string, args ...any) {
	f.letters = append(f.letters, letter{title, x, y, format, args})
}

// failDice fails the test on any draw.
type failDice struct{ t *testing.T }

func (d failDice) IntN(int) int { d.t.Helper(); d.t.Fatal("drew from the dice"); return 0 }

// fixedDice answers every draw with its highest (top) or lowest value.
type fixedDice struct {
	top   bool
	draws int
}

func (d *fixedDice) IntN(n int) int {
	d.draws++
	if d.top {
		return n - 1
	}
	return 0
}

func cheb(x, y int) int { return max(mathx.Abs(x-hx), mathx.Abs(y-hy)) }

// field is every cell within r of home.
func field(r int) map[[2]int]bool {
	m := map[[2]int]bool{}
	for y := hy - r; y <= hy+r; y++ {
		for x := hx - r; x <= hx+r; x++ {
			m[[2]int{x, y}] = true
		}
	}
	return m
}

func cells(cs ...[2]int) map[[2]int]bool {
	m := map[[2]int]bool{}
	for _, c := range cs {
		m[c] = true
	}
	return m
}

// island is n cells in a row at distance 6 east of home.
func island(n int) map[[2]int]bool {
	m := map[[2]int]bool{}
	for i := range n {
		m[[2]int{hx + 6 + i, hy}] = true
	}
	return m
}

func TestFitsNeverRolls(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		id   incident.ID
		h    fakeHost
		want bool
	}{
		{"meteor open ground", incident.OreMeteor, fakeHost{colonists: 2, free: field(20)}, true},
		{"meteor island of the minimum", incident.OreMeteor, fakeHost{colonists: 1, free: island(incident.MeteorCellsMin)}, true},
		{"meteor island too small", incident.OreMeteor, fakeHost{colonists: 1, free: island(incident.MeteorCellsMin - 1)}, false},
		{"meteor only inside the minimum distance", incident.OreMeteor, fakeHost{colonists: 1, free: field(incident.MeteorMinDist - 1)}, false},
		{"meteor no colonists", incident.OreMeteor, fakeHost{free: field(20)}, false},
		{"drop open ground", incident.SupplyDrop, fakeHost{colonists: 1, dropOK: field(incident.SupplyRadius)}, true},
		{"drop two spots", incident.SupplyDrop, fakeHost{colonists: 1, dropOK: cells([2]int{hx + 3, hy}, [2]int{hx - 3, hy})}, true},
		{"drop one spot", incident.SupplyDrop, fakeHost{colonists: 1, dropOK: cells([2]int{hx + 3, hy})}, false},
		{"drop two touching cells", incident.SupplyDrop, fakeHost{colonists: 1, dropOK: cells([2]int{hx + 3, hy}, [2]int{hx + 4, hy})}, false},
		{"drop beyond the radius", incident.SupplyDrop, fakeHost{colonists: 1, dropOK: cells([2]int{hx + 13, hy}, [2]int{hx - 13, hy})}, false},
		{"drop no colonists", incident.SupplyDrop, fakeHost{dropOK: field(incident.SupplyRadius)}, false},
		{"song idle", incident.SkySong, fakeHost{colonists: 1}, true},
		{"song playing", incident.SkySong, fakeHost{colonists: 1, songUntil: 1000}, false},
		{"song no colonists", incident.SkySong, fakeHost{}, false},
		{"not a windfall", incident.Flu, fakeHost{colonists: 1, free: field(20), dropOK: field(20)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := c.h
			if got := New(&h, failDice{t}).Fits(c.id, 500); got != c.want {
				t.Fatalf("Fits = %v, want %v", got, c.want)
			}
			if len(h.rocks)+len(h.drops)+len(h.starts)+len(h.cheers)+len(h.letters) != 0 {
				t.Fatalf("Fits changed the host: %+v", h)
			}
		})
	}
}

// fourConnected reports cs form one four-connected cluster.
func fourConnected(cs [][2]int) bool {
	if len(cs) == 0 {
		return false
	}
	seen := [][2]int{cs[0]}
	for i := 0; i < len(seen); i++ {
		for _, d := range [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
			n := [2]int{seen[i][0] + d[0], seen[i][1] + d[1]}
			if slices.Contains(cs, n) && !slices.Contains(seen, n) {
				seen = append(seen, n)
			}
		}
	}
	return len(seen) == len(cs)
}

func TestMeteorFire(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		top  bool
		n    int
		ore  geology.Ore
		name string
	}{{false, incident.MeteorCellsMin, geology.Steel, "lowest"}, {true, incident.MeteorCellsMax, geology.Gold, "highest"}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := &fakeHost{colonists: 1, free: field(20)}
			d := &fixedDice{top: c.top}
			if !New(h, d).Fire(incident.OreMeteor, 500) {
				t.Fatal("meteor refused on open ground")
			}
			if d.draws != 2 || len(h.rocks) != c.n {
				t.Fatalf("draws %d, rocks %v, want 2 draws and %d cells", d.draws, h.rocks, c.n)
			}
			for i, r := range h.rocks {
				if !h.free[r] || cheb(r[0], r[1]) < incident.MeteorMinDist || cheb(r[0], r[1]) > incident.MeteorRadius || h.ores[i] != c.ore {
					t.Fatalf("cell %v ore %v: off the free band or wrong ore (want %v)", r, h.ores[i], c.ore)
				}
				if slices.Index(h.rocks, r) != i {
					t.Fatalf("cell %v raised twice", r)
				}
			}
			if !fourConnected(h.rocks) {
				t.Fatalf("cells %v are not one four-connected cluster", h.rocks)
			}
			if len(h.letters) != 1 || h.letters[0].title != "Ore meteor" || [2]int{h.letters[0].x, h.letters[0].y} != h.rocks[0] {
				t.Fatalf("letters %+v", h.letters)
			}
		})
	}
}

// The cluster never grows inside MeteorMinDist, even when the seed's free
// neighbour lies there.
func TestMeteorStaysInBand(t *testing.T) {
	t.Parallel()
	free := map[[2]int]bool{}
	for x := hx - incident.MeteorRadius - 3; x <= hx; x++ {
		free[[2]int{x, hy}] = true // a row from beyond the radius to home
	}
	h := &fakeHost{colonists: 1, free: free}
	if !New(h, &fixedDice{top: true}).Fire(incident.OreMeteor, 500) {
		t.Fatal("meteor refused on the row")
	}
	if len(h.rocks) != incident.MeteorCellsMax {
		t.Fatalf("rocks %v", h.rocks)
	}
	for _, r := range h.rocks {
		if d := cheb(r[0], r[1]); d < incident.MeteorMinDist || d > incident.MeteorRadius {
			t.Fatalf("cell %v at distance %d", r, d)
		}
	}
}

func TestMeteorNoSiteDrawsNothing(t *testing.T) {
	t.Parallel()
	for name, h := range map[string]*fakeHost{
		"small island": {colonists: 1, free: island(incident.MeteorCellsMin - 1)},
		"no colonists": {free: field(20)},
	} {
		if New(h, failDice{t}).Fire(incident.OreMeteor, 500) || len(h.rocks)+len(h.letters) != 0 {
			t.Fatalf("%s: meteor fired: %+v", name, h)
		}
	}
}

func TestSupplyFire(t *testing.T) {
	t.Parallel()
	h := &fakeHost{colonists: 1, dropOK: field(incident.SupplyRadius)}
	if !New(h, &fixedDice{top: true}).Fire(incident.SupplyDrop, 500) {
		t.Fatal("drop refused on open ground")
	}
	if len(h.drops) != incident.SupplyStacksMax {
		t.Fatalf("drops %+v", h.drops)
	}
	var kinds []item.Kind
	for i, d := range h.drops {
		if slices.Contains(kinds, d.k) || d.qty <= 0 || !h.dropOK[[2]int{d.x, d.y}] {
			t.Fatalf("drop %+v: repeated kind, empty or off the free cells", d)
		}
		kinds = append(kinds, d.k)
		for _, e := range h.drops[:i] {
			if mathx.Abs(e.x-d.x) <= 1 && mathx.Abs(e.y-d.y) <= 1 {
				t.Fatalf("drops %+v and %+v touch", e, d)
			}
		}
	}
	if len(h.letters) != 1 || h.letters[0].title != "Supply drop" || !slices.Equal(h.letters[0].args, []any{3}) ||
		h.letters[0].x != h.drops[0].x || h.letters[0].y != h.drops[0].y {
		t.Fatalf("letters %+v", h.letters)
	}
}

// Two spots and three kinds picked: the first two land, the letter says 2.
func TestSupplyFireTwoSpots(t *testing.T) {
	t.Parallel()
	h := &fakeHost{colonists: 1, dropOK: cells([2]int{hx + 3, hy}, [2]int{hx - 3, hy})}
	if !New(h, &fixedDice{top: true}).Fire(incident.SupplyDrop, 500) {
		t.Fatal("drop refused on two spots")
	}
	if len(h.drops) != 2 || h.drops[0].k == h.drops[1].k || len(h.letters) != 1 || !slices.Equal(h.letters[0].args, []any{2}) {
		t.Fatalf("drops %+v letters %+v", h.drops, h.letters)
	}
}

func TestSupplyOneSpotDrawsNothing(t *testing.T) {
	t.Parallel()
	h := &fakeHost{colonists: 1, dropOK: cells([2]int{hx + 3, hy})}
	if New(h, failDice{t}).Fire(incident.SupplyDrop, 500) || len(h.drops)+len(h.letters) != 0 {
		t.Fatalf("drop fired on one spot: %+v", h)
	}
}

func TestSongFire(t *testing.T) {
	t.Parallel()
	const now = 7000
	h := &fakeHost{colonists: 3}
	d := &fixedDice{}
	if !New(h, d).Fire(incident.SkySong, now) {
		t.Fatal("song refused")
	}
	if len(h.starts) != 1 || h.starts[0][0] != now {
		t.Fatalf("starts %v", h.starts)
	}
	ticks := h.starts[0][1]
	if ticks != incident.SongTicks(func(int) int { return 0 }) || d.draws != 1 {
		t.Fatalf("ticks %d after %d draws", ticks, d.draws)
	}
	if !slices.Equal(h.cheers, [][2]uint64{{now, now + ticks}}) || h.songUntil != now+ticks {
		t.Fatalf("cheers %v, song until %d, want until %d", h.cheers, h.songUntil, now+ticks)
	}
	if len(h.letters) != 1 || h.letters[0].title != "Sky song" || [2]int{h.letters[0].x, h.letters[0].y} != [2]int{hx, hy} ||
		!slices.Equal(h.letters[0].args, []any{incident.SongDays(ticks)}) {
		t.Fatalf("letters %+v", h.letters)
	}
	// While it plays: no fit, no fire, no draw.
	w := New(h, failDice{t})
	if w.Fits(incident.SkySong, now+1) || w.Fire(incident.SkySong, now+1) || len(h.starts) != 1 || len(h.letters) != 1 {
		t.Fatalf("song refired while playing: %+v", h)
	}
	// Once it ends it fits again.
	if !w.Fits(incident.SkySong, now+ticks) {
		t.Fatal("song does not fit after it ended")
	}
}

// A song the state refuses cheers no one and posts nothing.
func TestSongRefusedByState(t *testing.T) {
	t.Parallel()
	h := &fakeHost{colonists: 1, refuse: true}
	if New(h, &fixedDice{}).Fire(incident.SkySong, 10) || len(h.cheers)+len(h.letters) != 0 {
		t.Fatalf("refused song cheered or posted: %+v", h)
	}
	h = &fakeHost{}
	if New(h, failDice{t}).Fire(incident.SkySong, 10) || len(h.starts) != 0 {
		t.Fatalf("song without colonists: %+v", h)
	}
}

func TestFireOtherIDDoesNothing(t *testing.T) {
	t.Parallel()
	h := &fakeHost{colonists: 1, free: field(20), dropOK: field(20)}
	if New(h, failDice{t}).Fire(incident.Flu, 10) || len(h.rocks)+len(h.drops)+len(h.starts)+len(h.letters) != 0 {
		t.Fatalf("Fire(Flu) acted: %+v", h)
	}
}
