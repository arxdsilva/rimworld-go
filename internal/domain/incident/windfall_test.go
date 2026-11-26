// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/geology"
)

func TestSongStartsOnceAndEndsOnTime(t *testing.T) {
	t.Parallel()
	var st State
	if st.Song(0) || st.SongView(0) != 0 {
		t.Fatal("a fresh state has no song")
	}
	if st.StartSong(100, 0) || st.SongUntil != 0 {
		t.Fatal("a zero-length song never starts")
	}
	if !st.StartSong(100, 50) || st.SongUntil != 150 {
		t.Fatalf("start: until %d", st.SongUntil)
	}
	if st.StartSong(120, 500) || st.SongUntil != 150 {
		t.Fatal("no extend while it plays")
	}
	if !st.Song(149) || st.SongView(149) != 150 {
		t.Fatal("on until the last tick")
	}
	if st.Song(150) || st.SongView(150) != 0 {
		t.Fatal("off exactly at SongUntil")
	}
	if !st.StartSong(150, 10) || st.SongUntil != 160 {
		t.Fatal("a new song starts exactly when the old one ends")
	}
}

func TestSongBounds(t *testing.T) {
	t.Parallel()
	if got, want := SongTicks(0), uint64(175*clock.TicksPerDay/100); got != want {
		t.Fatalf("shortest %d, want %d", got, want)
	}
	if got, want := SongTicks(SongSpan()), uint64(225*clock.TicksPerDay/100); got != want {
		t.Fatalf("longest %d, want %d", got, want)
	}
	if SongTicks(-5) != SongTicks(0) {
		t.Fatal("a negative roll clamps to the shortest")
	}
}

func TestWindfallDefs(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		id                 ID
		key, name          string
		weight, cool, mind int
	}{
		{EscapePodCrash, "escape_pod_crash", "Escape pod crash", 4, 15, 6},
		{OreMeteor, "ore_meteor", "Ore meteor", 5, 12, 5},
		{SupplyDrop, "supply_drop", "Supply drop", 6, 10, 4},
		{SkySong, "sky_song", "Sky song", 4, 20, 8},
	} {
		d := DefOf(c.id)
		if d.Key != c.key || d.Name != c.name || d.Category != Nature || !d.Helpful ||
			d.Weight != c.weight || d.Cooldown != c.cool || d.MinDay != c.mind {
			t.Fatalf("%v: %+v", c.id, d)
		}
	}
	if uint64(DefOf(SkySong).Cooldown)*clock.TicksPerDay <= SongTicks(SongSpan()) {
		t.Fatal("the cooldown must outlast the longest song")
	}
}

func TestWindfallTables(t *testing.T) {
	t.Parallel()
	kinds := map[any]bool{}
	for _, s := range SupplyLoot {
		if s.Min <= 0 || s.Min > s.Max || s.Max > s.Kind.StackLimit() {
			t.Fatalf("supply %+v (limit %d)", s, s.Kind.StackLimit())
		}
		if kinds[s.Kind] {
			t.Fatalf("duplicate supply kind %v", s.Kind)
		}
		kinds[s.Kind] = true
	}
	for _, o := range MeteorOres {
		if o.Weight <= 0 || o.Ore == geology.NoOre {
			t.Fatalf("meteor ore %+v", o)
		}
	}
	if MeteorCellsMin > MeteorCellsMax || MeteorCellsMin <= 0 || MeteorRadius <= 0 || MeteorTries <= 0 {
		t.Fatal("meteor bounds")
	}
	if SupplyStacksMin > SupplyStacksMax || SupplyStacksMax > len(SupplyLoot) || SupplyStacksMin <= 0 {
		t.Fatal("supply stack bounds")
	}
	if CrashHurt <= 0 {
		t.Fatal("crash blow")
	}
}
