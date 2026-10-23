package region

import (
	"testing"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/pkg/randx"
)

// layerQuery is the walkability a layer sees, for pathing.Find.
type layerQuery struct {
	w, h    int
	blocked []bool
	flags   []Flags
	l       Layer
}

func (q layerQuery) Width() int  { return q.w }
func (q layerQuery) Height() int { return q.h }
func (q layerQuery) Walkable(x, y int) bool {
	if x < 0 || y < 0 || x >= q.w || y >= q.h {
		return false
	}
	i := y*q.w + x
	return !q.blocked[i] && q.flags[i]&q.l.Blocks() == 0
}

// world is a random test map kept in step with a Map.
type world struct {
	w, h    int
	blocked []bool
	flags   []Flags
	m       *Map
}

func randomFlags(r *randx.Rand) Flags {
	switch r.IntN(6) {
	case 0:
		return Door
	case 1:
		return Fence
	case 2:
		return Gate
	case 3:
		return Trap
	case 4:
		return Door | Trap
	}
	return 0
}

func newWorld(r *randx.Rand, w, h int) *world {
	wd := &world{w: w, h: h, blocked: make([]bool, w*h), flags: make([]Flags, w*h), m: New(w, h)}
	density := 15 + r.IntN(40)
	for i := range wd.blocked {
		if r.IntN(100) < density {
			wd.set(i, true, 0)
		} else if r.IntN(100) < 8 {
			wd.set(i, false, randomFlags(r))
		}
	}
	return wd
}

func (wd *world) set(i int, blocked bool, f Flags) {
	wd.blocked[i], wd.flags[i] = blocked, f
	wd.m.SetBlocked(i%wd.w, i/wd.w, blocked)
	wd.m.SetFlags(i%wd.w, i/wd.w, f)
}

// edit is a random change: a wall line, a door, a fence or gate, a cell
// mined out, water drained or flooded (walkability flips).
func (wd *world) edit(r *randx.Rand) {
	x, y := r.IntN(wd.w), r.IntN(wd.h)
	switch r.IntN(5) {
	case 0: // a wall run
		n := 1 + r.IntN(8)
		horiz := r.IntN(2) == 0
		for k := 0; k < n; k++ {
			cx, cy := x, y
			if horiz {
				cx += k
			} else {
				cy += k
			}
			if cx < wd.w && cy < wd.h {
				wd.set(cy*wd.w+cx, true, 0)
			}
		}
	case 1: // mined / drained: open
		wd.set(y*wd.w+x, false, 0)
	case 2: // a door, fence, gate or trap
		wd.set(y*wd.w+x, false, randomFlags(r))
	case 3: // deep water / rock appears
		wd.set(y*wd.w+x, true, 0)
	default: // open a whole block (a room dug out)
		for dy := 0; dy < 3; dy++ {
			for dx := 0; dx < 3; dx++ {
				if x+dx < wd.w && y+dy < wd.h {
					wd.set((y+dy)*wd.w+x+dx, false, 0)
				}
			}
		}
	}
}

// check compares Reach with A* for random pairs in every layer.
func (wd *world) check(t testing.TB, r *randx.Rand, pairs int) {
	t.Helper()
	for l := Layer(0); l < Layers; l++ {
		q := layerQuery{w: wd.w, h: wd.h, blocked: wd.blocked, flags: wd.flags, l: l}
		for k := 0; k < pairs; k++ {
			sx, sy := r.IntN(wd.w), r.IntN(wd.h)
			tx, ty := r.IntN(wd.w), r.IntN(wd.h)
			if k%4 == 0 { // near pairs too
				tx, ty = min(wd.w-1, sx+r.IntN(3)), min(wd.h-1, sy+r.IntN(3))
			}
			_, want := pathing.Find(q, sx, sy, tx, ty)
			if got := wd.m.Reach(l, sx, sy, tx, ty); got != want {
				t.Fatalf("%dx%d layer %d (%d,%d)->(%d,%d): Reach=%v, A*=%v", wd.w, wd.h, l, sx, sy, tx, ty, got, want)
			}
			if got := wd.m.Label(l, tx, ty) >= 0; got != q.Walkable(tx, ty) {
				t.Fatalf("layer %d (%d,%d): labelled=%v, walkable=%v", l, tx, ty, got, !got)
			}
		}
	}
}

// TestRegionReachMatchesAStar: on random maps, after random edits
// (walls, doors, fences, gates, traps, mining, water), every layer's
// region answer equals what A* finds.
func TestRegionReachMatchesAStar(t *testing.T) {
	for seed := uint64(1); seed <= 60; seed++ {
		r := randx.New(seed)
		w, h := 1+r.IntN(70), 1+r.IntN(70)
		wd := newWorld(r, w, h)
		wd.check(t, r, 30)
		for e := 0; e < 25; e++ {
			for k := r.IntN(4); k >= 0; k-- {
				wd.edit(r)
			}
			wd.check(t, r, 12)
		}
	}
}

// TestRegionIncrementalMatchesFresh: an edited map's labels partition the
// cells exactly as a map built from scratch does.
func TestRegionIncrementalMatchesFresh(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		r := randx.New(seed)
		w, h := 10+r.IntN(60), 10+r.IntN(60)
		wd := newWorld(r, w, h)
		for e := 0; e < 30; e++ {
			wd.edit(r)
			if e%5 != 4 {
				continue
			}
			fresh := New(w, h)
			for i := range wd.blocked {
				fresh.SetBlocked(i%w, i/w, wd.blocked[i])
				fresh.SetFlags(i%w, i/w, wd.flags[i])
			}
			for l := Layer(0); l < Layers; l++ {
				seen := map[[2]int32]bool{}
				back := map[int32]int32{}
				for i := 0; i < w*h; i++ {
					a, b := wd.m.Label(l, i%w, i/w), fresh.Label(l, i%w, i/w)
					if (a < 0) != (b < 0) {
						t.Fatalf("seed %d layer %d cell %d: %d vs %d", seed, l, i, a, b)
					}
					if a < 0 {
						continue
					}
					if !seen[[2]int32{a, b}] {
						if o, ok := back[b]; ok && o != a {
							t.Fatalf("seed %d layer %d: component split differs at cell %d", seed, l, i)
						}
						seen[[2]int32{a, b}], back[b] = true, a
					}
				}
				fwd := map[int32]int32{}
				for k := range seen {
					if o, ok := fwd[k[0]]; ok && o != k[1] {
						t.Fatalf("seed %d layer %d: component merge differs", seed, l)
					}
					fwd[k[0]] = k[1]
				}
			}
		}
	}
}

// TestRegionBlockedWordsDiff: feeding a whole bitset marks exactly the
// changed cells.
func TestRegionBlockedWordsDiff(t *testing.T) {
	const w, h = 40, 33
	m := New(w, h)
	words := make([]uint64, (w*h+63)/64)
	set := func(x, y int) { i := y*w + x; words[i>>6] |= 1 << (uint(i) & 63) }
	for y := 0; y < h; y++ {
		set(20, y) // a wall splitting the map
	}
	m.SetBlockedWords(words)
	if m.Reach(Colonist, 0, 0, 39, 0) {
		t.Fatal("reached across a full wall")
	}
	words[0] = 0
	words[(5*w+20)>>6] &^= 1 << (uint(5*w+20) & 63) // a gap
	m.SetBlockedWords(words)
	if !m.Reach(Colonist, 0, 0, 39, 0) {
		t.Fatal("gap not seen")
	}
}

// TestRegionLayers: a door shuts hostiles and wild animals out, a fence
// every animal, a gate only wild ones, a trap only sappers.
func TestRegionLayers(t *testing.T) {
	const w, h = 20, 5
	cases := []struct {
		f    Flags
		open [Layers]bool
	}{
		{Door, [Layers]bool{Colonist: true, Tame: true}},
		{Fence, [Layers]bool{Colonist: true, Hostile: true, Sapper: true}},
		{Gate, [Layers]bool{Colonist: true, Hostile: true, Sapper: true, Tame: true}},
		{Trap, [Layers]bool{Colonist: true, Hostile: true, Tame: true, Wild: true}},
	}
	for _, c := range cases {
		m := New(w, h)
		for y := 0; y < h; y++ {
			m.SetBlocked(10, y, true)
		}
		m.SetBlocked(10, 2, false)
		m.SetFlags(10, 2, c.f)
		for l := Layer(0); l < Layers; l++ {
			if got := m.Reach(l, 0, 0, 19, 4); got != c.open[l] {
				t.Errorf("flags %d layer %d: reach=%v, want %v", c.f, l, got, c.open[l])
			}
		}
		// standing on the shut cell itself: either side is reachable
		if !m.Reach(Wild, 10, 2, 19, 4) || !m.Reach(Wild, 10, 2, 0, 0) {
			t.Errorf("flags %d: a creature on the cell must step off it", c.f)
		}
	}
}

// FuzzRegionReachability: arbitrary maps and edits, Reach == A*.
func FuzzRegionReachability(f *testing.F) {
	for _, s := range []uint64{1, 2, 3, 99} {
		f.Add(s, uint8(17), uint8(23))
	}
	f.Fuzz(func(t *testing.T, seed uint64, w8, h8 uint8) {
		w, h := 1+int(w8)%48, 1+int(h8)%48
		r := randx.New(seed)
		wd := newWorld(r, w, h)
		wd.check(t, r, 8)
		for e := 0; e < 6; e++ {
			wd.edit(r)
			wd.check(t, r, 6)
		}
	})
}
