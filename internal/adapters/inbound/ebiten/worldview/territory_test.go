// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package worldview

import (
	"slices"
	"sync"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

var factionParams = planet.Params{SeedText: "world-territory", Coverage: 5, Rain: 2, Temp: 2, Pop: 1,
	Factions: []faction.Slot{{Type: faction.Settlers, Count: 2}, {Type: faction.Clans, Count: 2}, {Type: faction.Drifters, Count: 1}}}

// factionPlanet is a small world with peoples of its own (testPlanet has none).
var factionPlanet = sync.OnceValue(func() *planet.Planet {
	b := planet.NewBuilder(factionParams)
	for {
		if _, _, done := b.Step(50000); done {
			return b.Planet()
		}
	}
})

// worldSites lists every settlement of the test planet as the game's sites.
func worldSites(p *planet.Planet) []game.WorldSiteView {
	var out []game.WorldSiteView
	for i := 0; i < p.TileCount(); i++ {
		if p.Settlement[i] >= 0 {
			out = append(out, game.WorldSiteView{Tile: i, Name: "S", Faction: "F"})
		}
	}
	return out
}

// territoryGame has WorldSites and no caravans; its factions are named by
// the game, not by the planet.
func territoryGame(t *testing.T) *fakeGame {
	t.Helper()
	p := factionPlanet()
	g := &fakeGame{params: factionParams, hasMap: true, sites: worldSites(p)}
	g.maps = []game.MapView{{ID: 0, Label: "Colony", Kind: "colony", Tile: landTile(t, 0), Colonists: 3}}
	g.world = game.WorldMapView{Tile: landTile(t, 0)}
	for i := range p.Factions {
		g.factions = append(g.factions, game.FactionView{ID: i, Name: "People " + string(rune('A'+i))})
	}
	return g
}

// newTerr is a fake shell over g on the founded faction planet.
func newTerr(g Reader) (*fakeShell, *View) {
	sh, v := newFake(g)
	sh.env.Planet = factionPlanet()
	return sh, v
}

// chartUp runs frames until the globe stands.
func chartUp(t *testing.T, sh *fakeShell, v *View) {
	t.Helper()
	v.Show("")
	for i := 0; i < 3000 && v.g == nil; i++ {
		frame(sh, v, -1, -1, inputframe.Raw{})
	}
	if v.g == nil {
		t.Fatal("never charted")
	}
	frame(sh, v, -1, -1, inputframe.Raw{})
}

// pair is a tile owned by someone and a tile owned by nobody.
func pair(t *testing.T, v *View) (owned, free int) {
	t.Helper()
	owned, free = -1, -1
	for i := 0; i < v.chart.planet.TileCount(); i++ {
		if v.terr.owner(i) >= 0 && owned < 0 {
			owned = i
		}
		if v.terr.owner(i) < 0 && free < 0 {
			free = i
		}
	}
	if owned < 0 || free < 0 {
		t.Skipf("owned %d free %d", owned, free)
	}
	return owned, free
}

func rowValue(lines []line, label string) string {
	for _, l := range lines {
		if l.label == label {
			return l.value
		}
	}
	return "<none>"
}

func TestWorldTerritoryBeforeAnyCaravan(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"founded", "loaded"} {
		t.Run(name, func(t *testing.T) {
			g := territoryGame(t)
			sh, v := newTerr(g)
			if name == "loaded" {
				sh.env.Planet = nil
			}
			chartUp(t, sh, v)
			owned, free := pair(t, v)
			_, lines, _ := (inspector{v}).inspectTile(owned)
			want := ownerName(g.factions, v.terr.owner(owned))
			if got := rowValue(lines, "Territory"); got != want || want == "" {
				t.Fatalf("owned tile shows %q, want %q", got, want)
			}
			if _, lines, _ = (inspector{v}).inspectTile(free); rowValue(lines, "Territory") != "Unclaimed" {
				t.Fatalf("free tile shows %q", rowValue(lines, "Territory"))
			}
			if v.terr.on || v.terr.pushes != 1 {
				t.Fatalf("on %v pushes %d before the toggle", v.terr.on, v.terr.pushes)
			}
			clickWidget(t, sh, v, TerritoryID)
			if !v.terr.on || !v.terr.tinted || v.terr.pushes != 2 || v.terr.t.Owners() == nil {
				t.Fatalf("on %v tinted %v pushes %d", v.terr.on, v.terr.tinted, v.terr.pushes)
			}
		})
	}
}

func TestWorldTerritoryFollowsLoss(t *testing.T) {
	t.Parallel()
	g := territoryGame(t)
	sh, v := newTerr(g)
	chartUp(t, sh, v)
	p := v.chart.planet
	f, _ := p.SettlementFaction(g.sites[0].Tile)
	tiles := slices.Clone(v.terr.t.Tiles(f))
	if len(tiles) == 0 {
		t.Skip("faction owns nothing")
	}
	ver := v.terr.t.Version()
	frame(sh, v, -1, -1, inputframe.Raw{})
	if v.terr.t.Version() != ver {
		t.Fatal("a frame with no change rebuilt")
	}
	var kept []game.WorldSiteView
	for _, s := range g.sites {
		if sf, _ := p.SettlementFaction(s.Tile); sf != f {
			kept = append(kept, s)
		}
	}
	g.sites = kept
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	if got := v.terr.t.Version(); got != ver+1 {
		t.Fatalf("version %d after the loss, want %d", got, ver+1)
	}
	for _, ti := range tiles {
		if v.terr.owner(int(ti)) == f {
			t.Fatalf("tile %d still held by the lost people", ti)
		}
	}
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	if v.terr.t.Version() != ver+1 {
		t.Fatal("an unchanged live set rebuilt")
	}
}

func TestWorldTerritoryPushOncePerVersion(t *testing.T) {
	t.Parallel()
	g := territoryGame(t)
	sh, v := newTerr(g)
	chartUp(t, sh, v)
	clickWidget(t, sh, v, TerritoryID)
	base := v.terr.pushes
	for i := 0; i < 5; i++ {
		v.read.dirty = true
		frame(sh, v, -1, -1, inputframe.Raw{})
	}
	if v.terr.pushes != base {
		t.Fatalf("pushed %d times for one version", v.terr.pushes-base)
	}
	// a new version pushes once
	g.sites = g.sites[1:]
	v.read.dirty = true
	frame(sh, v, -1, -1, inputframe.Raw{})
	frame(sh, v, -1, -1, inputframe.Raw{})
	if v.terr.pushes != base+1 {
		t.Fatalf("pushes %d, want %d", v.terr.pushes, base+1)
	}
	// a changed type list under the same territory version still pushes
	pv := v.terr.pushVersion()
	v.terr.types = slices.Clone(v.terr.types)
	v.terr.types[0] ^= faction.Type(1)
	if v.terr.pushVersion() == pv {
		t.Fatal("the pushed version ignores the types")
	}
	v.terr.push(v.g)
	if v.terr.pushes != base+2 {
		t.Fatal("a type change was not pushed")
	}
	// a rebuilt globe is given the tint again
	old := v.g
	v.g = nil
	frame(sh, v, -1, -1, inputframe.Raw{})
	if v.g == nil || v.g == old {
		t.Fatal("globe not rebuilt")
	}
	if v.terr.to != v.g || v.terr.pushes != base+3 {
		t.Fatalf("to-new-globe %v pushes %d", v.terr.to == v.g, v.terr.pushes)
	}
}

// The view keeps its own territory: the planet's index is not touched.
func TestWorldTerritoryIsNotTheGamePlanets(t *testing.T) {
	t.Parallel()
	p := factionPlanet()
	before := planet.TerritoryVersion(p)
	g := territoryGame(t)
	sh, v := newTerr(g)
	chartUp(t, sh, v)
	if planet.TerritoryVersion(p) != before {
		t.Fatal("the view rebuilt the planet's own territory")
	}
}

func TestWorldTerritoryOffClearsTheTint(t *testing.T) {
	t.Parallel()
	sh, v := newTerr(territoryGame(t))
	chartUp(t, sh, v)
	clickWidget(t, sh, v, TerritoryID)
	clickWidget(t, sh, v, TerritoryID)
	if v.terr.on || v.terr.tinted {
		t.Fatalf("on %v tinted %v", v.terr.on, v.terr.tinted)
	}
}

// The Territory row is on the first screen of the tile card, no scrolling.
func TestWorldTerritoryRowIsVisibleUnscrolled(t *testing.T) {
	t.Parallel()
	sh, v := newTerr(territoryGame(t))
	chartUp(t, sh, v)
	owned, _ := pair(t, v)
	_, lines, gizmos := (inspector{v}).inspectTile(owned)
	idx := slices.IndexFunc(lines, func(l line) bool { return l.label == "Territory" })
	if idx < 0 {
		t.Fatal("no Territory row")
	}
	in := inspectorRect(sh.Layout()).Inset(inspPad)
	body := in.H - 24 - float32((len(gizmos)+gizmoCols-1)/gizmoCols)*(gizmoH+4) - 4
	if bottom := float32((idx + 1) * lineH); bottom > body {
		t.Fatalf("row %d ends at %v, below the visible %v", idx, bottom, body)
	}
}

func TestWorldTerritoryUnknownOwnerName(t *testing.T) {
	t.Parallel()
	g := territoryGame(t)
	g.factions = nil
	sh, v := newTerr(g)
	chartUp(t, sh, v)
	owned, _ := pair(t, v)
	if _, lines, _ := (inspector{v}).inspectTile(owned); rowValue(lines, "Territory") != "Unknown people" {
		t.Fatalf("got %q", rowValue(lines, "Territory"))
	}
}
