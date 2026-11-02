package game

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"slices"
	"strconv"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/statehash"
	"rimworld-go/internal/domain/world/terrain"
)

// The mirror (MAP-0): a colony and its twin step the same ticks, the twin
// with a second, empty map open. Every colony-wide table is walked once a
// tick per map pass, so any work done twice (needs, patients, caravans, the
// storyteller, letters) or any shared random stream a quiet map draws from
// would make the first map's state drift. Map 0's save (without the other
// maps) must match the lone colony's field for field.

// addQuietMap opens an empty 20x20 map on g that spawns nothing: no pawns,
// animals or wildlife (wild false), and bare rough stone, so no wild plant
// sprouts (sproutWild is genuinely per map, but a sprout takes an id from
// the global allocator and shifts map 0's later ids). Its weather is held
// (it is hashed per map and never draws g.rng anyway).
func addQuietMap(g *Game) {
	cur := g.mapState
	m := newMapState(Config{Width: 20, Height: 20, Biome: g.biome}, uint32(len(g.maps)))
	for i := range m.terr.Cells {
		m.terr.Cells[i] = terrain.Stone
	}
	m.weatherFix = true
	g.mapState = m
	initMapHosts(g)
	g.maps = append(g.maps, m)
	g.mapState = cur
}

// mapZeroFields is g's save as hashed fields (StateHash's normalised form)
// without the other open maps.
func mapZeroFields(t *testing.T, g *Game) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	if err := g.Save(&buf, "mirror"); err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	var s saveState
	if err := dec.Decode(&info); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	f := normalisedFields(info, &s)
	delete(f, "ExtraMaps") // the twin's quiet map
	delete(f, "View")
	return f
}

// sameMapZero fails with the differing save fields of a and b.
func sameMapZero(t *testing.T, label string, a, b *Game) {
	t.Helper()
	fa, fb := mapZeroFields(t, a), mapZeroFields(t, b)
	if statehash.OfFields(fa) == statehash.OfFields(fb) {
		return
	}
	var diff []string
	for k, v := range fa {
		if statehash.OfFields(map[string]any{k: v}) != statehash.OfFields(map[string]any{k: fb[k]}) {
			diff = append(diff, k)
		}
	}
	for k := range fb {
		if _, ok := fa[k]; !ok {
			diff = append(diff, k)
		}
	}
	slices.Sort(diff)
	if slices.Contains(diff, "NextEntity") {
		t.Fatalf("%s: the quiet map allocated entity ids (a spawn on map 1, not double processing); fields %v", label, diff)
	}
	t.Fatalf("%s: map 0 drifts with a second map open, ids matching: double processing or a shared stream drawn; fields %v", label, diff)
}

// stepMirror steps a and b n ticks in chunks, comparing map 0 after each.
func stepMirror(t *testing.T, a, b *Game, n, chunk int) {
	t.Helper()
	for done := 0; done < n; done += chunk {
		a.Step(chunk)
		b.Step(chunk)
		sameMapZero(t, "tick "+strconv.FormatUint(uint64(a.Tick()), 10), a, b)
	}
}

// A legacy colony at work, hunting and under a raid, through two medical
// hours: needs, wildlife, wounds, bleeding, the threat clock.
func TestMirrorLegacyColonyIgnoresQuietMap(t *testing.T) {
	t.Parallel()
	build := func() *Game {
		g := NewColony(Config{Seed: 9, Width: 48, Height: 40})
		cx, cy := g.Width()/2, g.Height()/2
		g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: cx + 1, Y: cy + 3, X2: cx + 4, Y2: cy + 5})
		g.Enqueue(command.Command{Kind: command.DesignateHunt, X: 0, Y: 0, X2: g.Width() - 1, Y2: g.Height() - 1})
		g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 0, Y: 0, X2: g.Width() - 1, Y2: g.Height() - 1})
		g.SpawnStack(item.Parka, 1, cx+5, cy)
		return g
	}
	a, b := build(), build()
	addQuietMap(b)
	sameMapZero(t, "start", a, b)
	stepMirror(t, a, b, 600, 300)
	a.Raid(4)
	b.Raid(4)
	stepMirror(t, a, b, 5400, 900)
}

// A setup colony (storyteller, factions, landing pods) from before the
// landing through its first hours: the arrival, losses, raids over and
// the storyteller run on the first map's pass only.
func TestMirrorSetupColonyIgnoresQuietMap(t *testing.T) {
	t.Parallel()
	build := func() *Game {
		return buildSetup(t, setupFor(t, "broken_hull", func(s *Setup) { s.MapSize = 60 }))
	}
	a, b := build(), build()
	addQuietMap(b)
	sameMapZero(t, "start", a, b)
	stepMirror(t, a, b, 6000, 1000)
	a.Raid(3)
	b.Raid(3)
	stepMirror(t, a, b, 3000, 1000)
}

// A colony with a trader caravan visiting, the orbital ship in range, a
// requested caravan and a trade pod on the way: visitors, trade and the
// world phase are the colony's, on the first map's pass.
func TestMirrorTradeAndVisitorsIgnoreQuietMap(t *testing.T) {
	t.Parallel()
	a, b := tradeRich(t), tradeRich(t)
	addQuietMap(b)
	sameMapZero(t, "start", a, b)
	stepMirror(t, a, b, 6000, 1000)
}
