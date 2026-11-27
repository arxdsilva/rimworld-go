// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package gameread

import (
	"fmt"
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

func smallGame() *game.Game {
	return game.NewColony(game.Config{Seed: 7, Width: 40, Height: 30})
}

// Every reader is set, so the host never meets a nil reader from New.
func TestNewSetsEveryReader(t *testing.T) {
	q := New(nil)
	if q.Animals == nil || q.Map == nil || q.World == nil || q.Buildings == nil {
		t.Fatalf("New left a reader nil: %+v", q)
	}
}

// A reader over no game reads as empty and never panics.
func TestNilGameReadsEmpty(t *testing.T) {
	q := New(nil)
	if w, h := q.Map.Size(); w != 0 || h != 0 {
		t.Fatalf("Size = %d,%d", w, h)
	}
	if _, ok := q.Map.Terrain(0, 0); ok {
		t.Fatal("Terrain ok without a game")
	}
	if _, ok := q.Map.Temperature(0, 0); ok {
		t.Fatal("Temperature ok without a game")
	}
	if q.Map.Weather() != "" || q.Animals.Animals() != nil || q.World.Factions() != nil ||
		q.World.Sites() != nil || q.Buildings.Buildings() != nil {
		t.Fatal("a nil game read non-empty")
	}
}

// C1: values over a real small game match the exported reads.
func TestReadersMatchGame(t *testing.T) {
	g := smallGame()
	q := New(g)

	if w, h := q.Map.Size(); w != g.Width() || h != g.Height() || w != 40 || h != 30 {
		t.Fatalf("Size = %d,%d", w, h)
	}
	for _, c := range [][2]int{{0, 0}, {39, 29}, {13, 7}} {
		got, ok := q.Map.Terrain(c[0], c[1])
		if !ok || got != g.TerrainAt(c[0], c[1]) || got == "" {
			t.Fatalf("Terrain%v = %q,%v", c, got, ok)
		}
		temp, ok := q.Map.Temperature(c[0], c[1])
		if !ok || temp != g.TemperatureAt(c[0], c[1]) {
			t.Fatalf("Temperature%v = %d,%v", c, temp, ok)
		}
	}
	if q.Map.Weather() != g.Weather().Kind.String() || q.Map.Weather() == "" {
		t.Fatalf("Weather = %q", q.Map.Weather())
	}

	rows := g.Wildlife()
	want := map[string]game.WildlifeRow{}
	for _, r := range rows {
		want[r.Species] = r
	}
	an := q.Animals.Animals()
	if len(an) != len(rows) {
		t.Fatalf("animals %d, game %d", len(an), len(rows))
	}
	if !slices.IsSortedFunc(an, func(a, b scripting.Animal) int {
		switch {
		case a.Species < b.Species:
			return -1
		case a.Species > b.Species:
			return 1
		}
		return 0
	}) {
		t.Fatalf("animals not species ascending: %+v", an)
	}
	for _, a := range an {
		w := want[a.Species]
		if a.Label != w.Label || a.Count != w.Count || a.Predator != w.Predator {
			t.Fatalf("animal %+v, game %+v", a, w)
		}
	}

	var fs []scripting.Faction
	for _, f := range g.Factions() {
		if !f.Hidden {
			fs = append(fs, scripting.Faction{Name: f.Name, Goodwill: f.Goodwill, Hostile: f.Relation == "hostile"})
		}
	}
	if got := q.World.Factions(); !slices.Equal(got, fs) {
		t.Fatalf("factions %+v, want %+v", got, fs)
	}
	var ss []scripting.Site
	for _, s := range g.WorldSites() {
		ss = append(ss, scripting.Site{Name: s.Name, Faction: s.Faction})
	}
	if got := q.World.Sites(); !slices.Equal(got, ss) {
		t.Fatalf("sites %+v, want %+v", got, ss)
	}
	if got := q.Buildings.Buildings(); !slices.Equal(got, game.BuildingCounts(g)) {
		t.Fatalf("buildings %+v, want %+v", got, game.BuildingCounts(g))
	}
}

// C3: cells off the map give ok=false.
func TestOutOfBoundsCells(t *testing.T) {
	q := New(smallGame())
	for _, c := range [][2]int{{-1, 0}, {0, -1}, {40, 0}, {0, 30}, {1 << 20, 1 << 20}} {
		if s, ok := q.Map.Terrain(c[0], c[1]); ok || s != "" {
			t.Fatalf("Terrain%v = %q,%v", c, s, ok)
		}
		if n, ok := q.Map.Temperature(c[0], c[1]); ok || n != 0 {
			t.Fatalf("Temperature%v = %d,%v", c, n, ok)
		}
	}
}

// Readers never return more than MaxQueryRows and do not cache: a game
// that moves on is read afresh.
func TestReadersAreFresh(t *testing.T) {
	g := smallGame()
	q := New(g)
	before := q.Map.Weather()
	g.Step(600)
	if q.Map.Weather() != g.Weather().Kind.String() {
		t.Fatalf("Weather stale: %q (was %q)", q.Map.Weather(), before)
	}
	if len(q.Animals.Animals()) > scripting.MaxQueryRows || len(q.Buildings.Buildings()) > scripting.MaxQueryRows {
		t.Fatal("rows exceed MaxQueryRows")
	}
}

const over = scripting.MaxQueryRows + 7

// Q1: animals are sorted by species and cut at MaxQueryRows.
func TestAnimalsCapAndOrder(t *testing.T) {
	rows := make([]game.WildlifeRow, over)
	for i := range rows { // descending keys: the reader must sort ascending
		rows[i] = game.WildlifeRow{Species: fmt.Sprintf("s%04d", over-i), Count: i}
	}
	got := animalsOf(rows)
	if len(got) != scripting.MaxQueryRows {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Species != "s0001" || got[len(got)-1].Species != fmt.Sprintf("s%04d", scripting.MaxQueryRows) {
		t.Fatalf("order: first %q last %q", got[0].Species, got[len(got)-1].Species)
	}
}

// Q1: factions skip hidden rows, keep order, and are cut at MaxQueryRows.
func TestFactionsHiddenAndCap(t *testing.T) {
	small := []game.FactionView{
		{Name: "a", Goodwill: 5, Relation: "friendly"},
		{Name: "secret", Hidden: true},
		{Name: "b", Goodwill: -50, Relation: "hostile"},
	}
	want := []scripting.Faction{{Name: "a", Goodwill: 5}, {Name: "b", Goodwill: -50, Hostile: true}}
	if got := factionsOf(small); !slices.Equal(got, want) {
		t.Fatalf("got %+v", got)
	}
	rows := make([]game.FactionView, 0, over*2)
	for i := range over {
		rows = append(rows, game.FactionView{Name: fmt.Sprintf("f%04d", i)}, game.FactionView{Name: "h", Hidden: true})
	}
	got := factionsOf(rows)
	if len(got) != scripting.MaxQueryRows || got[0].Name != "f0000" || got[len(got)-1].Name != fmt.Sprintf("f%04d", scripting.MaxQueryRows-1) {
		t.Fatalf("len %d first %q", len(got), got[0].Name)
	}
}

// Q1: sites keep the game's order and are cut at MaxQueryRows.
func TestSitesCap(t *testing.T) {
	rows := make([]game.WorldSiteView, over)
	for i := range rows {
		rows[i] = game.WorldSiteView{Name: fmt.Sprintf("n%04d", i), Faction: "x"}
	}
	got := sitesOf(rows)
	if len(got) != scripting.MaxQueryRows || got[0].Name != "n0000" || got[len(got)-1].Name != fmt.Sprintf("n%04d", scripting.MaxQueryRows-1) {
		t.Fatalf("len %d", len(got))
	}
}

// Q1: building rows are cut at MaxQueryRows, and a short list is whole.
func TestBuildingsCap(t *testing.T) {
	rows := make([]scripting.BuildingCount, over)
	for i := range rows {
		rows[i] = scripting.BuildingCount{Kind: fmt.Sprintf("k%04d", i), Count: 1}
	}
	got := capBuildings(rows)
	if len(got) != scripting.MaxQueryRows || got[len(got)-1].Kind != fmt.Sprintf("k%04d", scripting.MaxQueryRows-1) {
		t.Fatalf("len %d", len(got))
	}
	if n := len(capBuildings(rows[:3])); n != 3 {
		t.Fatalf("short list len %d", n)
	}
	if capBuildings(nil) != nil && len(capBuildings(nil)) != 0 {
		t.Fatal("nil list not empty")
	}
}

// The real game's building counts are non-empty (the start kit) and the
// reader returns them unchanged.
func TestBuildingsOverRealGame(t *testing.T) {
	g := smallGame()
	p := g.Colonists()[0]
	for i := range 3 {
		g.Apply(command.Command{Kind: command.DesignateWall, X: p.X + 2 + i, Y: p.Y + 2})
	}
	g.Step(6000)
	got := New(g).Buildings.Buildings()
	if len(got) == 0 {
		t.Fatal("no building stands after designating walls and running 6000 ticks")
	}
	if want := game.BuildingCounts(g); !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i, b := range got {
		if b.Kind == "" || b.Count <= 0 || i > 0 && got[i-1].Kind >= b.Kind {
			t.Fatalf("bad rows %+v", got)
		}
	}
}

// fireHost reads the queries inside Fire, as a script would.
type fireHost struct {
	q    scripting.Queries
	seen []string
	size [2]int
}

func (h *fireHost) Uses(hk scripting.Hook) bool { return hk == scripting.OnHour }
func (h *fireHost) Fire(_ scripting.Event, _ scripting.World, _ scripting.PawnReader) scripting.Result {
	h.seen = append(h.seen, h.q.Map.Weather())
	h.size[0], h.size[1] = h.q.Map.Size()
	return scripting.Result{}
}

// The readers answer inside a real scripted call (the hour hook).
func TestReadersInsideFire(t *testing.T) {
	g := smallGame()
	h := &fireHost{q: New(g)}
	game.UseScripts(g, h)
	g.Step(3700)
	if len(h.seen) == 0 || h.seen[0] == "" || h.size != [2]int{40, 30} {
		t.Fatalf("Fire saw %v size %v", h.seen, h.size)
	}
}
