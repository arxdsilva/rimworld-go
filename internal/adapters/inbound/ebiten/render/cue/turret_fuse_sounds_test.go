// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package cue

import (
	"strconv"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/testkit/allocs"
)

func sparkingTurret(id entity.EntityID, x, y int) game.BuildingView {
	return game.BuildingView{ID: id, Kind: "autocannon", X: x, Y: y, W: 2, H: 2, Sparking: true}
}

func fuseKeys(want map[string]Loop) []string {
	var ks []string
	for k, l := range want {
		if len(k) > 5 && k[:5] == "fuse:" {
			if l.ID != "amb.turret_fuse_hiss" {
				return []string{"bad id " + l.ID}
			}
			ks = append(ks, k)
		}
	}
	return ks
}

func TestFuseHissWhileSparking(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	want := map[string]Loop{}
	r.b.Machines(r.v, want, game.WorldView{Buildings: []game.BuildingView{sparkingTurret(7, 6, 6)}}, camera.ColonyViewport(1280, 720))
	l, ok := want["fuse:7"]
	if !ok {
		t.Fatalf("no fuse loop: %+v", want)
	}
	if l.ID != "amb.turret_fuse_hiss" || l.Vol <= 0 {
		t.Fatalf("loop %+v", l)
	}
}

func TestFuseHissUnpowered(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	b := sparkingTurret(7, 6, 6)
	b.Powered, b.PowerOut = false, 0
	want := map[string]Loop{}
	r.b.Machines(r.v, want, game.WorldView{Buildings: []game.BuildingView{b}}, camera.ColonyViewport(1280, 720))
	if _, ok := want["fuse:7"]; !ok {
		t.Fatalf("an unpowered sparking turret is silent: %+v", want)
	}
}

func TestFuseHissStopsAndNoFlap(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	vp := camera.ColonyViewport(1280, 720)
	frame := func(w game.WorldView) bool {
		want := map[string]Loop{}
		r.b.Machines(r.v, want, w, vp)
		_, ok := want["fuse:7"]
		return ok
	}
	with := func(sparking bool) game.WorldView {
		b := sparkingTurret(7, 6, 6)
		b.Sparking = sparking
		return game.WorldView{Buildings: []game.BuildingView{b}}
	}
	steps := []struct {
		w    game.WorldView
		want bool
	}{
		{with(true), true},
		{with(true), true},
		{with(false), false},
		{with(true), true},
		{game.WorldView{}, false}, // blew up or crated: gone from the list
	}
	for i, s := range steps {
		if got := frame(s.w); got != s.want {
			t.Fatalf("frame %d: hiss %v, want %v", i, got, s.want)
		}
	}
}

func TestFuseHissNearestOnly(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	vp := camera.ColonyViewport(1280, 720)
	var bs []game.BuildingView
	vols := map[string]float64{}
	for i := 0; i < 4; i++ {
		b := sparkingTurret(entity.EntityID(10+i), 6+3*i, 6)
		bs = append(bs, b)
		vol, _, ok := Spatial(r.v, float64(b.X)+1, float64(b.Y)+1, vp)
		if !ok {
			t.Fatalf("turret %d off screen in the fixture", b.ID)
		}
		vols["fuse:"+strconv.Itoa(10+i)] = vol
	}
	bs = append(bs, sparkingTurret(50, 100000, 100000)) // off screen
	want := map[string]Loop{}
	r.b.Machines(r.v, want, game.WorldView{Buildings: bs}, vp)
	ks := fuseKeys(want)
	if len(ks) != MaxMachineLoops {
		t.Fatalf("%d fuse loops, want %d: %v", len(ks), MaxMachineLoops, ks)
	}
	// every heard turret is at least as loud as every unheard one
	for k := range vols {
		if _, heard := want[k]; heard {
			continue
		}
		for _, h := range ks {
			if vols[h] < vols[k] {
				t.Fatalf("%s heard but the louder %s is not", h, k)
			}
		}
	}
	if _, ok := want["fuse:50"]; ok {
		t.Fatal("an off-screen turret hisses")
	}
}

// Fed far, near, mid, nearest: the third evicts the far one (full array), and
// the fourth shifts the others down; the two nearest win, loudest first.
func TestFuseHissKeepsLoudestWhenFedOutOfOrder(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	vp := camera.ColonyViewport(1280, 720)
	type spot struct {
		id   entity.EntityID
		x, y int
	}
	spots := []spot{{1, 6, 6}, {2, 6, 6}, {3, 6, 6}, {4, 6, 6}}
	// order them by distance from the view centre using Spatial itself
	vol := func(s spot) float64 {
		v, _, _ := Spatial(r.v, float64(s.x)+1, float64(s.y)+1, vp)
		return v
	}
	// find positions with strictly increasing volume along a row
	cands := []spot{}
	for x := 0; x < 60; x += 2 {
		cands = append(cands, spot{0, x, 6})
	}
	var onScreen []spot
	for _, c := range cands {
		if _, _, ok := Spatial(r.v, float64(c.x)+1, float64(c.y)+1, vp); ok {
			onScreen = append(onScreen, c)
		}
	}
	// pick four distinct-volume spots, sorted quietest first
	var pick []spot
	for _, c := range onScreen {
		if len(pick) == 0 || vol(c) != vol(pick[len(pick)-1]) {
			pick = append(pick, c)
		}
	}
	if len(pick) < 4 {
		t.Fatalf("fixture: only %d distinct volumes", len(pick))
	}
	sortByVol := func(s []spot) {
		for i := 1; i < len(s); i++ {
			for j := i; j > 0 && vol(s[j-1]) > vol(s[j]); j-- {
				s[j-1], s[j] = s[j], s[j-1]
			}
		}
	}
	sortByVol(pick)
	pick = pick[len(pick)-4:] // the four loudest distinct spots, quietest first
	for i := range spots {
		spots[i].x, spots[i].y = pick[i].x, pick[i].y
	}
	// ids 1..4 quietest..loudest; feed far(1), near(3), mid(2), nearest(4)
	order := []int{0, 2, 1, 3}
	var bs []game.BuildingView
	for _, i := range order {
		bs = append(bs, sparkingTurret(spots[i].id, spots[i].x, spots[i].y))
	}
	want := map[string]Loop{}
	r.b.Machines(r.v, want, game.WorldView{Buildings: bs}, vp)
	ks := fuseKeys(want)
	if len(ks) != 2 {
		t.Fatalf("keys %v", ks)
	}
	if _, ok := want["fuse:4"]; !ok {
		t.Fatalf("nearest missing: %v", ks)
	}
	if _, ok := want["fuse:3"]; !ok {
		t.Fatalf("second nearest missing: %v", ks)
	}
	if want["fuse:4"].Vol < want["fuse:3"].Vol {
		t.Fatalf("volumes out of order: %+v", want)
	}
}

func TestFuseHissBesideMachineHum(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	g := game.BuildingView{ID: 21, Kind: WoodGeneratorKind, X: 5, Y: 5, W: 1, H: 1, Burning: true, PowerOut: 100, Sparking: true}
	want := map[string]Loop{}
	r.b.Machines(r.v, want, game.WorldView{Buildings: []game.BuildingView{g}}, camera.ColonyViewport(1280, 720))
	if _, ok := want["power:21"]; !ok {
		t.Fatalf("hum lost: %+v", want)
	}
	if _, ok := want["fuse:21"]; !ok {
		t.Fatalf("hiss lost: %+v", want)
	}
}

// Not parallel: allocs.Fewest measures the whole process.
func TestFuseHissAllocs(t *testing.T) {
	r := newBus(1)
	vp := camera.ColonyViewport(1280, 720)
	quiet := make([]game.BuildingView, 200)
	for i := range quiet {
		quiet[i] = game.BuildingView{ID: entity.EntityID(1000 + i), Kind: "wall", X: i % 60, Y: i / 60}
	}
	spark := append([]game.BuildingView(nil), quiet...)
	spark[3] = sparkingTurret(7, 6, 6)
	spark[4] = sparkingTurret(8, 9, 6)
	want := map[string]Loop{}
	measure := func(bs []game.BuildingView) float64 {
		w := game.WorldView{Buildings: bs}
		return allocs.Fewest(20, func() {
			clear(want)
			r.b.Machines(r.v, want, w, vp)
		})
	}
	base, hot := measure(quiet), measure(spark)
	if len(fuseKeys(want)) != 2 {
		t.Fatalf("fixture: want 2 fuse keys, got %v", fuseKeys(want))
	}
	t.Logf("allocs quiet=%v sparking=%v", base, hot)
	if hot-base > 4 { // at most 2 per sparking key (the key string)
		t.Fatalf("sparking turrets add %v allocations over %v", hot-base, base)
	}
}
