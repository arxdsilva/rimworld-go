// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

func dayTick(d int) uint64 { return uint64(d) * clock.TicksPerDay }

// TestSpendShares: 0 spends every point, otherwise the percent.
func TestSpendShares(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ pct, want int }{{0, 400}, {50, 200}, {100, 400}, {150, 600}} {
		if got := (Def{PointsPct: c.pct}).Spend(400); got != c.want {
			t.Errorf("PointsPct %d of 400 = %d, want %d", c.pct, got, c.want)
		}
	}
}

// TestThreatPickBounds: a roll off the die shows nothing; a nil check
// accepts every face.
func TestThreatPickBounds(t *testing.T) {
	t.Parallel()
	b := Core()
	for _, r := range []int{-1, ThreatRolls, 99} {
		if id, ok := b.PickThreat(dayTick(30), biome.TemperateForest, Odds{}, r, nil); ok {
			t.Errorf("roll %d showed %d", r, id)
		}
	}
	if id, ok := b.PickThreat(dayTick(30), biome.TemperateForest, Odds{}, 4, nil); !ok || id != Raid {
		t.Errorf("roll 4 = %d, %v; want Raid", id, ok)
	}
}

// TestThreatPickIgnoresCooldown: a mad animal inside its cooldown keeps its
// face (the caller checks Cooled at fire).
func TestThreatPickIgnoresCooldown(t *testing.T) {
	t.Parallel()
	b := Core()
	now := dayTick(30)
	want, _ := b.PickThreat(now, biome.TemperateForest, Odds{}, 6, nil)
	if want != MadAnimal {
		t.Fatalf("roll 6 = %d, want MadAnimal", want)
	}
	var st State
	st.Fired(MadAnimal, now-1)
	if DefOf(MadAnimal).Cooled(st, now) {
		t.Fatal("setup: mad animal should be cooling")
	}
	if got, _ := b.PickThreat(now, biome.TemperateForest, Odds{}, 6, nil); got != want {
		t.Errorf("pick changed to %d inside the cooldown", got)
	}
}

// TestThreatPickPointGate: a face whose event needs more points than are
// at hand goes to the filler.
func TestThreatPickPointGate(t *testing.T) {
	t.Parallel()
	b := Core()
	now := dayTick(30)
	for _, c := range []struct {
		pts  int
		want ID
	}{{0, MadAnimal}} {
		if id, _ := b.PickThreat(now, biome.TemperateForest, Odds{Points: c.pts}, 6, nil); id != c.want {
			t.Errorf("points %d: %d, want %d", c.pts, id, c.want)
		}
	}
	// A plugin-style book where the mad animal needs 300 points.
	gated := Core()
	gated.defs = append([]Def(nil), gated.defs...)
	gated.defs[MadAnimal].MinPoints = 300
	if id, _ := gated.PickThreat(now, biome.TemperateForest, Odds{Points: 299}, 6, nil); id != Raid {
		t.Errorf("299 points: %d, want the filler Raid", id)
	}
	if id, _ := gated.PickThreat(now, biome.TemperateForest, Odds{Points: 300}, 6, nil); id != MadAnimal {
		t.Errorf("300 points: %d, want MadAnimal", id)
	}
}

// TestThreatPickRing: a refused face falls round the ring from roll%3,
// the hive only from its first day; nothing allowed shows nothing.
func TestThreatPickRing(t *testing.T) {
	t.Parallel()
	b := Core()
	only := func(want ...ID) func(ID) bool {
		return func(id ID) bool {
			for _, w := range want {
				if id == w {
					return true
				}
			}
			return false
		}
	}
	bi := biome.TemperateForest
	// Roll 4 is a raid; refused, roll%3 = 1 starts at the manhunters.
	if id, _ := b.PickThreat(dayTick(30), bi, Odds{}, 4, only(PredatorPack, ManhunterPack)); id != ManhunterPack {
		t.Errorf("ring from 1: %d", id)
	}
	if id, _ := b.PickThreat(dayTick(30), bi, Odds{}, 4, only(HiveSwarm)); id != HiveSwarm {
		t.Errorf("hive after day 12: %d", id)
	}
	if id, ok := b.PickThreat(dayTick(5), bi, Odds{}, 4, only(HiveSwarm)); ok {
		t.Errorf("hive before its day: %d", id)
	}
	if _, ok := b.PickThreat(dayTick(30), bi, Odds{}, 4, func(ID) bool { return false }); ok {
		t.Error("nothing allowed still showed a threat")
	}
}

// TestThreatPickRingTarget: the ring never falls back to an event aimed at
// a world object (a patch moving the manhunters off the map).
func TestThreatPickRingTarget(t *testing.T) {
	t.Parallel()
	b := Core()
	b.defs = append([]Def(nil), b.defs...)
	b.defs[ManhunterPack].Target = WorldObject
	notRaid := func(id ID) bool { return id == PredatorPack || id == ManhunterPack }
	if id, ok := b.PickThreat(dayTick(30), biome.TemperateForest, Odds{}, 4, notRaid); !ok || id != PredatorPack {
		t.Errorf("ring from 1 past the world-object manhunters: %d, %v; want PredatorPack", id, ok)
	}
	if id, ok := b.PickThreat(dayTick(30), biome.TemperateForest, Odds{}, 4, func(id ID) bool { return id == ManhunterPack }); ok {
		t.Errorf("only the world-object manhunters allowed: showed %d", id)
	}
}

// TestThreatPickNoFiller: with no filler an unclaimed face shows nothing,
// and a book too short for the ring skips what it lacks.
func TestThreatPickNoFiller(t *testing.T) {
	t.Parallel()
	b := Core()
	b.defs = append([]Def(nil), b.defs...)
	b.defs[Raid].Fill = 0
	b.defs[Raid].Category = Nature
	if id, ok := b.PickThreat(dayTick(30), biome.TemperateForest, Odds{}, 4, nil); ok {
		t.Errorf("no filler showed %d", id)
	}
	short := Book{defs: b.defs[:HiveSwarm]}
	if id, ok := short.PickThreat(dayTick(30), biome.TemperateForest, Odds{}, 4, nil); ok {
		t.Errorf("short book showed %d", id)
	}
}

// TestEligibleGates: the generic pick skips an event below its points, out
// of season, aimed at a world object or drawn only inside the raid face.
func TestEligibleGates(t *testing.T) {
	t.Parallel()
	base := Def{ID: 0, Category: Nature, Weight: 5}
	list := func(mod func(*Def)) []Def {
		d := base
		mod(&d)
		return []Def{d}
	}
	now := dayTick(30)
	bi := biome.TemperateForest
	for _, c := range []struct {
		name string
		mod  func(*Def)
		o    Odds
		want bool
	}{
		{"plain", func(*Def) {}, Odds{}, true},
		{"points short", func(d *Def) { d.MinPoints = 300 }, Odds{Points: 299}, false},
		{"points met", func(d *Def) { d.MinPoints = 300 }, Odds{Points: 300}, true},
		{"winter only in summer", func(d *Def) { d.Season = WinterOnly }, Odds{}, false},
		{"winter only in winter", func(d *Def) { d.Season = WinterOnly }, Odds{Winter: true}, true},
		{"not winter in winter", func(d *Def) { d.Season = NotWinter }, Odds{Winter: true}, false},
		{"world object", func(d *Def) { d.Target = WorldObject }, Odds{}, false},
		{"under raid", func(d *Def) { d.UnderRaid = true }, Odds{}, false},
	} {
		got := totalOdds(list(c.mod), Nature, State{}, now, bi, c.o, nil) > 0
		if got != c.want {
			t.Errorf("%s: eligible %v, want %v", c.name, got, c.want)
		}
		_, picked := pickOdds(list(c.mod), Nature, State{}, now, bi, c.o, 0, nil)
		if picked != c.want {
			t.Errorf("%s: picked %v, want %v", c.name, picked, c.want)
		}
	}
}

// TestPointGateNoFlap: an event fired on day 10 stays out through its
// cooldown however the points swing around its minimum; afterwards it
// follows the points.
func TestPointGateNoFlap(t *testing.T) {
	t.Parallel()
	d := Def{ID: 0, Category: Nature, Weight: 5, MinPoints: 300, Cooldown: 10}
	var st State
	st.Fired(0, dayTick(10))
	bi := biome.TemperateForest
	for day := 10; day < 20; day++ {
		for _, pts := range []int{299, 301} {
			if totalOdds([]Def{d}, Nature, st, dayTick(day), bi, Odds{Points: pts}, nil) > 0 {
				t.Fatalf("day %d, %d points: eligible inside the cooldown", day, pts)
			}
		}
	}
	for day := 20; day < 24; day++ {
		for _, pts := range []int{299, 301} {
			got := totalOdds([]Def{d}, Nature, st, dayTick(day), bi, Odds{Points: pts}, nil) > 0
			if got != (pts >= 300) {
				t.Fatalf("day %d, %d points: eligible %v", day, pts, got)
			}
		}
	}
}

// TestSeasonGateNoFlap: the season turning back inside the cooldown never
// re-enables the event.
func TestSeasonGateNoFlap(t *testing.T) {
	t.Parallel()
	d := Def{ID: 0, Category: Nature, Weight: 5, Season: WinterOnly, Cooldown: 10}
	var st State
	st.Fired(0, dayTick(10))
	for day := 10; day < 20; day++ {
		for _, winter := range []bool{true, false} {
			if totalOdds([]Def{d}, Nature, st, dayTick(day), biome.TemperateForest, Odds{Winter: winter}, nil) > 0 {
				t.Fatalf("day %d winter %v: eligible inside the cooldown", day, winter)
			}
		}
	}
	if totalOdds([]Def{d}, Nature, st, dayTick(20), biome.TemperateForest, Odds{Winter: true}, nil) == 0 {
		t.Error("not eligible in winter after the cooldown")
	}
}

// TestLastFired: the latest event of the category wins; a tie goes to the
// UnderRaid one, then the higher ID; an empty state has none.
func TestLastFired(t *testing.T) {
	t.Parallel()
	b := Core()
	var st State
	if _, _, ok := b.LastFired(st, Threat); ok {
		t.Fatal("empty state reported a fired threat")
	}
	st.Fired(MadAnimal, 100)
	st.Fired(TraderCaravan, 900) // another category
	if d, at, ok := b.LastFired(st, Threat); !ok || d.ID != MadAnimal || at != 100 {
		t.Errorf("one fired: %v %d %v", d.ID, at, ok)
	}
	st.Fired(Raid, 200)
	if d, _, _ := b.LastFired(st, Threat); d.ID != Raid {
		t.Errorf("latest: %v, want Raid", d.ID)
	}
	st.Fired(CrashedShipPart, 200) // ties Raid; UnderRaid wins
	if d, _, _ := b.LastFired(st, Threat); d.ID != CrashedShipPart {
		t.Errorf("tie: %v, want the UnderRaid CrashedShipPart", d.ID)
	}
	st.Fired(MechRaid, 200) // two UnderRaid tie; higher ID wins
	if d, at, _ := b.LastFired(st, Threat); d.ID != MechRaid || at != 200 {
		t.Errorf("under-raid tie: %v, want MechRaid", d.ID)
	}
	st.Fired(HiveSwarm, 200) // a plain threat does not beat an UnderRaid at the tie
	if d, _, _ := b.LastFired(st, Threat); d.ID != MechRaid {
		t.Errorf("plain tie: %v, want MechRaid", d.ID)
	}
	if d, _, _ := b.LastFired(st, Nature); d.ID != TraderCaravan {
		t.Errorf("nature: %v", d.ID)
	}
}
