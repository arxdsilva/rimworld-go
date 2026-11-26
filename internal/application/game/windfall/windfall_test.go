// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windfall

import (
	"fmt"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
)

// fakeHost records every call as a transcript line.
type fakeHost struct {
	colonists  int
	songOn     bool
	songRefuse bool
	noSpot     int // PodSpot fails from this call number (1-based) on; 0: never
	crashFail  bool
	free       func(x, y int) bool
	spotCalls  int
	freeCalls  int
	log        []string
	letters    []string
	drops      []string
	rocks      [][2]int
	rockOre    []geology.Ore
	freeAsked  [][2]int
	songTicks  uint64
}

func (f *fakeHost) Home() (int, int) { return 50, 40 }
func (f *fakeHost) Size() (int, int) { return 100, 80 }
func (f *fakeHost) PodSpot(cx, cy int, taken [][2]int) (int, int, bool) {
	f.spotCalls++
	f.log = append(f.log, fmt.Sprintf("spot %d,%d taken %d", cx, cy, len(taken)))
	if f.noSpot > 0 && f.spotCalls >= f.noSpot {
		return 0, 0, false
	}
	return cx + 2 + len(taken), cy, true
}
func (f *fakeHost) MeteorFree(x, y int) bool {
	f.freeCalls++
	f.freeAsked = append(f.freeAsked, [2]int{x, y})
	return f.free != nil && f.free(x, y)
}
func (f *fakeHost) Rock(x, y int, ore geology.Ore) {
	f.log = append(f.log, fmt.Sprintf("rock %d,%d %d", x, y, ore))
	f.rocks = append(f.rocks, [2]int{x, y})
	f.rockOre = append(f.rockOre, ore)
}
func (f *fakeHost) Drop(k item.Kind, qty, x, y int) {
	s := fmt.Sprintf("drop %d x%d @%d,%d", k, qty, x, y)
	f.log = append(f.log, s)
	f.drops = append(f.drops, s)
}
func (f *fakeHost) Colonists() int { return f.colonists }
func (f *fakeHost) Crash(x, y, hurt int) (entity.EntityID, bool) {
	f.log = append(f.log, fmt.Sprintf("crash %d,%d %d", x, y, hurt))
	if f.crashFail {
		return 0, false
	}
	return 77, true
}
func (f *fakeHost) Name(id entity.EntityID) string { return "Ana" }
func (f *fakeHost) SongOn(now uint64) bool         { return f.songOn }
func (f *fakeHost) StartSong(now, ticks uint64) bool {
	f.songTicks = ticks
	f.log = append(f.log, fmt.Sprintf("song %d %d", now, ticks))
	return !f.songRefuse
}
func (f *fakeHost) Letter(title string, x, y int, focus entity.EntityID, format string, args ...any) {
	s := fmt.Sprintf("%s @%d,%d f%d: %s", title, x, y, focus, fmt.Sprintf(format, args...))
	f.log = append(f.log, "letter "+title)
	f.letters = append(f.letters, s)
}

// script replays rolls and records the n of each draw.
type script struct {
	rolls []int
	i     int
	ns    []int
}

func (s *script) IntN(n int) int {
	s.ns = append(s.ns, n)
	v := 0
	if s.i < len(s.rolls) {
		v = s.rolls[s.i]
	}
	s.i++
	return v % n
}

func TestFitsNeedsColonistsAndNoSong(t *testing.T) {
	ids := []incident.ID{incident.EscapePodCrash, incident.OreMeteor, incident.SupplyDrop, incident.SkySong}
	d := &script{}
	for _, id := range ids {
		if New(&fakeHost{colonists: 0}, d).Fits(id, 10) {
			t.Fatalf("%v fits with no colonists", id)
		}
		if !New(&fakeHost{colonists: 3}, d).Fits(id, 10) {
			t.Fatalf("%v does not fit with colonists", id)
		}
	}
	if New(&fakeHost{colonists: 3, songOn: true}, d).Fits(incident.SkySong, 10) {
		t.Fatal("sky song fits while it plays")
	}
	if !New(&fakeHost{colonists: 3, songOn: true}, d).Fits(incident.OreMeteor, 10) {
		t.Fatal("the meteor should not care about the song")
	}
	if New(&fakeHost{colonists: 3}, d).Fits(incident.RivalRaid, 10) {
		t.Fatal("a foreign id fits")
	}
	if len(d.ns) != 0 {
		t.Fatalf("Fits drew dice: %v", d.ns)
	}
}

func TestFireForeignIDDoesNothing(t *testing.T) {
	h, d := &fakeHost{colonists: 3}, &script{}
	if New(h, d).Fire(incident.RivalRaid, 5) || len(h.log) != 0 || len(d.ns) != 0 {
		t.Fatalf("foreign id acted: %v %v", h.log, d.ns)
	}
}

func TestPodCrashLandsDownedStranger(t *testing.T) {
	h := &fakeHost{colonists: 2}
	if !New(h, &script{}).Fire(incident.EscapePodCrash, 5) {
		t.Fatal("crash did not fire")
	}
	want := []string{"spot 50,40 taken 0", fmt.Sprintf("crash 52,40 %d", incident.CrashHurt), "letter Escape pod crash"}
	if !reflect.DeepEqual(h.log, want) {
		t.Fatalf("log = %v, want %v", h.log, want)
	}
	if len(h.letters) != 1 || h.letters[0][:len("Escape pod crash @52,40 f77")] != "Escape pod crash @52,40 f77" {
		t.Fatalf("letters = %v", h.letters)
	}

	none := &fakeHost{colonists: 2, noSpot: 1}
	if New(none, &script{}).Fire(incident.EscapePodCrash, 5) || len(none.letters) != 0 {
		t.Fatal("no spot must give false and no letter")
	}
	bad := &fakeHost{colonists: 2, crashFail: true}
	if New(bad, &script{}).Fire(incident.EscapePodCrash, 5) || len(bad.letters) != 0 {
		t.Fatal("a refused crash must give false and no letter")
	}
}

func TestSupplyDropDistinctForbiddenStacks(t *testing.T) {
	for _, stacks := range []int{2, 3} {
		h := &fakeHost{colonists: 2}
		// first roll picks the count; then (kind pick, qty roll) pairs, all 0
		// except a last-kind pick to prove the shrinking list.
		rolls := []int{stacks - incident.SupplyStacksMin}
		for i := 0; i < stacks; i++ {
			rolls = append(rolls, 1, i)
		}
		if !New(h, &script{rolls: rolls}).Fire(incident.SupplyDrop, 5) {
			t.Fatal("supply did not fire")
		}
		if len(h.drops) != stacks {
			t.Fatalf("stacks = %d, want %d: %v", len(h.drops), stacks, h.drops)
		}
		kinds, tiles := map[item.Kind]bool{}, map[string]bool{}
		for _, s := range h.drops {
			var k item.Kind
			var q, x, y int
			if _, err := fmt.Sscanf(s, "drop %d x%d @%d,%d", &k, &q, &x, &y); err != nil {
				t.Fatal(err)
			}
			if kinds[k] {
				t.Fatalf("kind %d twice", k)
			}
			kinds[k] = true
			tiles[fmt.Sprint(x, y)] = true
			ok := false
			for _, l := range incident.SupplyLoot {
				if l.Kind == k && q >= l.Min && q <= l.Max {
					ok = true
				}
			}
			if !ok {
				t.Fatalf("drop %q outside its range", s)
			}
		}
		if len(tiles) != stacks {
			t.Fatalf("tiles not distinct: %v", h.drops)
		}
		if len(h.letters) != 1 {
			t.Fatalf("letters = %v", h.letters)
		}
	}

	none := &fakeHost{colonists: 2, noSpot: 1}
	if New(none, &script{}).Fire(incident.SupplyDrop, 5) || len(none.drops) != 0 || len(none.letters) != 0 {
		t.Fatal("no spot: want false, no drop, no letter")
	}
	part := &fakeHost{colonists: 2, noSpot: 2}
	if !New(part, &script{rolls: []int{1}}).Fire(incident.SupplyDrop, 5) || len(part.drops) != 1 || len(part.letters) != 1 {
		t.Fatalf("a later missing spot keeps what landed: %v", part.log)
	}
}

func openField(x, y int) bool { return x >= 0 && y >= 0 && x < 100 && y < 80 }

func TestOreMeteorClusterOnFreeCells(t *testing.T) {
	for n := incident.MeteorCellsMin; n <= incident.MeteorCellsMax; n++ {
		h := &fakeHost{colonists: 2, free: openField}
		rolls := []int{0, n - incident.MeteorCellsMin, 30, 30}
		if !New(h, &script{rolls: rolls}).Fire(incident.OreMeteor, 5) {
			t.Fatal("meteor did not fire")
		}
		if len(h.rocks) != n {
			t.Fatalf("cells = %d, want %d", len(h.rocks), n)
		}
		seen := map[[2]int]bool{}
		for i, c := range h.rocks {
			if seen[c] || !openField(c[0], c[1]) {
				t.Fatalf("bad cell %v", c)
			}
			seen[c] = true
			if i > 0 && !adjacent(c, seen, h.rocks[:i]) {
				t.Fatalf("cell %v not 4-connected to the cluster", c)
			}
			if h.rockOre[i] != h.rockOre[0] {
				t.Fatal("more than one ore")
			}
		}
		if h.rockOre[0] != incident.MeteorOres[0].Ore {
			t.Fatalf("ore = %v", h.rockOre[0])
		}
		if len(h.letters) != 1 {
			t.Fatalf("letters = %v", h.letters)
		}
	}

	// the heaviest ore draw reaches the last entry
	h := &fakeHost{colonists: 2, free: openField}
	New(h, &script{rolls: []int{11, 0, 30, 30}}).Fire(incident.OreMeteor, 5)
	if h.rockOre[0] != geology.Gold {
		t.Fatalf("roll 11 ore = %v, want gold", h.rockOre[0])
	}
}

func adjacent(c [2]int, _ map[[2]int]bool, before [][2]int) bool {
	for _, b := range before {
		dx, dy := b[0]-c[0], b[1]-c[1]
		if dx*dx+dy*dy == 1 {
			return true
		}
	}
	return false
}

func TestOreMeteorCentresWithinRadiusAndBounded(t *testing.T) {
	h := &fakeHost{colonists: 2} // nothing is free
	d := &script{rolls: []int{0, 0, 0, 60, 60, 7, 13}}
	if New(h, d).Fire(incident.OreMeteor, 5) {
		t.Fatal("meteor fired with no free cell")
	}
	if len(h.rocks) != 0 || len(h.letters) != 0 {
		t.Fatal("rock or letter without a cluster")
	}
	if h.freeCalls != incident.MeteorTries {
		t.Fatalf("MeteorFree calls = %d, want %d", h.freeCalls, incident.MeteorTries)
	}
	if h.freeCalls > incident.MeteorTries*(1+4*incident.MeteorCellsMax) {
		t.Fatal("search exceeds its bound")
	}
	for _, c := range h.freeAsked {
		if abs(c[0]-50) > incident.MeteorRadius || abs(c[1]-40) > incident.MeteorRadius {
			t.Fatalf("centre %v beyond the radius", c)
		}
	}
}

func TestOreMeteorSmallPocketRejected(t *testing.T) {
	// only two cells are free: a cluster below the minimum is never accepted
	free := func(x, y int) bool { return (x == 50 && y == 40) || (x == 51 && y == 40) }
	h := &fakeHost{colonists: 2, free: free}
	if New(h, &script{rolls: []int{0, 3, 30, 30}}).Fire(incident.OreMeteor, 5) || len(h.rocks) != 0 {
		t.Fatal("a two-cell pocket must not become a meteor")
	}
	if h.freeCalls > incident.MeteorTries*(1+4*incident.MeteorCellsMax) {
		t.Fatal("search exceeds its bound")
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestSkySongStartsOnce(t *testing.T) {
	span := incident.SongSpan()
	h := &fakeHost{colonists: 2}
	d := &script{rolls: []int{7}}
	if !New(h, d).Fire(incident.SkySong, 1000) {
		t.Fatal("song did not fire")
	}
	if len(d.ns) != 1 || d.ns[0] != span+1 {
		t.Fatalf("dice n = %v, want [%d]", d.ns, span+1)
	}
	r := 7 % (span + 1)
	if h.songTicks != incident.SongTicks(r) {
		t.Fatalf("ticks = %d, want %d", h.songTicks, incident.SongTicks(r))
	}
	if len(h.letters) != 1 {
		t.Fatalf("letters = %v", h.letters)
	}

	ref := &fakeHost{colonists: 2, songRefuse: true}
	if New(ref, &script{}).Fire(incident.SkySong, 1000) || len(ref.letters) != 0 {
		t.Fatal("a refused song must give false and no letter")
	}
}

func TestFireDeterministic(t *testing.T) {
	ids := []incident.ID{incident.EscapePodCrash, incident.OreMeteor, incident.SupplyDrop, incident.SkySong}
	for _, id := range ids {
		run := func() ([]string, []string) {
			h := &fakeHost{colonists: 2, free: openField}
			New(h, &script{rolls: []int{3, 1, 4, 1, 5, 9, 2, 6}}).Fire(id, 77)
			return h.log, h.letters
		}
		l1, t1 := run()
		l2, t2 := run()
		if !reflect.DeepEqual(l1, l2) || !reflect.DeepEqual(t1, t2) || len(l1) == 0 {
			t.Fatalf("%v differs between runs: %v vs %v", id, l1, l2)
		}
	}
}
