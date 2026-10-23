package region

import (
	"testing"

	"rimworld-go/internal/domain/world/room"
	"rimworld-go/pkg/randx"
)

type roomWorld struct {
	w, h                    int
	boundary, stone, smooth []bool
	r                       *Rooms
}

func (rw *roomWorld) set(i int, b, s, sm bool) {
	rw.boundary[i], rw.stone[i], rw.smooth[i] = b, s, sm
	rw.r.SetCell(i%rw.w, i/rw.w, b, s, sm)
}

func newRoomWorld(r *randx.Rand, w, h int) *roomWorld {
	rw := &roomWorld{w: w, h: h, boundary: make([]bool, w*h), stone: make([]bool, w*h), smooth: make([]bool, w*h), r: NewRooms(w, h)}
	for i := 0; i < w*h; i++ {
		b := r.IntN(100) < 20
		rw.set(i, b, b && r.IntN(2) == 0, !b && r.IntN(5) == 0)
	}
	// a few walled boxes: rooms
	for k := r.IntN(6); k > 0; k-- {
		rw.box(r)
	}
	return rw
}

func (rw *roomWorld) box(r *randx.Rand) {
	x0, y0 := r.IntN(rw.w), r.IntN(rw.h)
	x1, y1 := min(rw.w-1, x0+2+r.IntN(10)), min(rw.h-1, y0+2+r.IntN(10))
	stone := r.IntN(2) == 0
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			i := y*rw.w + x
			if x == x0 || x == x1 || y == y0 || y == y1 {
				rw.set(i, true, stone, false)
			} else {
				rw.set(i, false, false, r.IntN(3) == 0)
			}
		}
	}
}

func (rw *roomWorld) edit(r *randx.Rand) {
	i := r.IntN(rw.w * rw.h)
	switch r.IntN(6) {
	case 0:
		rw.box(r)
	case 1, 2: // a wall or door built, rock appears
		rw.set(i, true, r.IntN(2) == 0, false)
	case 3: // mined / deconstructed
		rw.set(i, false, false, false)
	case 4: // floor laid / smoothed
		if !rw.boundary[i] {
			rw.set(i, false, false, !rw.smooth[i])
		}
	default: // a wall rebuilt in another material
		if rw.boundary[i] {
			rw.set(i, true, !rw.stone[i], false)
		}
	}
}

// check compares the incremental rooms with room.Detect and brute counts;
// prev is the previous grid, for Same and EachChanged.
func (rw *roomWorld) check(t *testing.T, prev []int) []int {
	t.Helper()
	w, h := rw.w, rw.h
	rw.r.Update()
	grid, sizes := room.Detect(w, h, func(x, y int) bool { return rw.boundary[y*w+x] })
	if rw.r.Count() != len(sizes) {
		t.Fatalf("%dx%d: %d rooms, Detect %d", w, h, rw.r.Count(), len(sizes))
	}
	for i, want := range grid {
		if got := rw.r.At(i%w, i/w); got != want {
			t.Fatalf("%dx%d cell (%d,%d): room %d, Detect %d", w, h, i%w, i/w, got, want)
		}
	}
	want := make([]Counts, len(sizes))
	for i, id := range grid {
		if id < 0 {
			continue
		}
		c := &want[id]
		c.Cells++
		if rw.smooth[i] {
			c.Smooth++
		}
		x, y := i%w, i/w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if nx >= 0 && ny >= 0 && nx < w && ny < h && rw.boundary[ny*w+nx] {
				c.Walls++
				if rw.stone[ny*w+nx] {
					c.StoneWalls++
				}
			}
		}
	}
	for id := range want {
		if got := rw.r.Counts(id); got != want[id] {
			t.Fatalf("room %d counts %+v, want %+v", id, got, want[id])
		}
		seen := 0
		rw.r.EachCell(id, func(i int) {
			if grid[i] != id {
				t.Fatalf("room %d lists cell %d of room %d", id, i, grid[i])
			}
			seen++
		})
		if seen != int(want[id].Cells) {
			t.Fatalf("room %d lists %d cells, has %d", id, seen, want[id].Cells)
		}
	}
	if prev != nil {
		changed := make([]bool, w*h)
		rw.r.EachChanged(func(i int) { changed[i] = true })
		for i := range grid {
			if grid[i] != prev[i] && !changed[i] {
				t.Fatalf("cell %d changed %d -> %d unreported", i, prev[i], grid[i])
			}
		}
		for id := range want {
			old := rw.r.Same(id)
			if old < 0 {
				continue
			}
			for i := range grid {
				if (grid[i] == id) != (prev[i] == old) {
					t.Fatalf("room %d said same as old %d, cell %d differs", id, old, i)
				}
			}
		}
	}
	return grid
}

// TestRoomsIncrementalMatchesDetect: after every random edit (walls,
// doors, rock, mining, floors, rebuilt walls) the incremental rooms equal
// room.Detect: grid, numbering and contents; Same and EachChanged are
// sound.
func TestRoomsIncrementalMatchesDetect(t *testing.T) {
	sames := 0
	for seed := uint64(1); seed <= 40; seed++ {
		r := randx.New(seed)
		w, h := 1+r.IntN(60), 1+r.IntN(60)
		rw := newRoomWorld(r, w, h)
		prev := rw.check(t, nil)
		for e := 0; e < 40; e++ {
			for k := r.IntN(3); k >= 0; k-- {
				rw.edit(r)
			}
			prev = rw.check(t, prev)
			for id := 0; id < rw.r.Count(); id++ {
				if rw.r.Same(id) >= 0 {
					sames++
				}
			}
		}
	}
	if sames == 0 {
		t.Fatal("no room ever recognised as unchanged")
	}
}
