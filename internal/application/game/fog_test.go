package game

import (
	"bytes"
	"slices"
	"testing"
	"time"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/terrain"
)

// fogCaveGame is a 30x20 map: soil west of x=10, rock from x=10 east, with
// a sealed 5x5 soil cave at x 11..15, y 6..10 whose west wall is the rock
// face at x=10. Fog is initialised from (3, 8).
func fogCaveGame(t testing.TB) *Game {
	t.Helper()
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	for y := 0; y < 20; y++ {
		for x := 10; x < 30; x++ {
			if !inCave(x, y) {
				g.setTerrain(x, y, terrain.Rock)
			}
		}
	}
	g.fogFromStart(3, 8)
	return g
}

func inCave(x, y int) bool { return x >= 11 && x <= 15 && y >= 6 && y <= 10 }

func caveHidden(g *Game) (hidden, shown int) {
	for y := 6; y <= 10; y++ {
		for x := 11; x <= 15; x++ {
			if g.Fogged(x, y) {
				hidden++
			} else {
				shown++
			}
		}
	}
	return hidden, shown
}

func fogMessage(g *Game, text string) bool {
	for _, m := range g.World().Messages {
		if m.Text == text {
			return true
		}
	}
	return false
}

func TestFogStartRevealedSealedCaveHidden(t *testing.T) {
	g := fogCaveGame(t)
	for _, c := range [][2]int{{3, 8}, {0, 0}, {9, 19}, {10, 0}, {10, 8}, {10, 19}} {
		if g.Fogged(c[0], c[1]) {
			t.Fatalf("(%d,%d) should be revealed (open ground or the rock face)", c[0], c[1])
		}
	}
	for _, c := range [][2]int{{11, 0}, {20, 15}, {29, 19}, {16, 8}} {
		if !g.Fogged(c[0], c[1]) {
			t.Fatalf("rock interior (%d,%d) should be hidden", c[0], c[1])
		}
	}
	if h, _ := caveHidden(g); h != 25 {
		t.Fatalf("sealed cave: %d of 25 cells hidden", h)
	}
	if g.Fogged(-1, 0) || g.Fogged(30, 0) {
		t.Fatal("out of bounds is never fogged")
	}
	if fogMessage(g, caveMessage) {
		t.Fatal("the initial reveal posts no message")
	}
}

func TestFogInitSetupOnly(t *testing.T) {
	legacy := fogCaveGame(t)
	legacy.fog = fogState{}
	legacy.initFog()
	if legacy.fog.hidden != nil || legacy.Fogged(20, 15) {
		t.Fatal("a legacy colony has no fog")
	}
	g := fogCaveGame(t)
	g.fog = fogState{}
	g.ng.setup = &Setup{}
	g.ng.landing, g.ng.hasLanding = [2]int{3, 8}, true
	g.initFog()
	if !g.Fogged(20, 15) || g.Fogged(3, 8) || g.fog.ver == 0 {
		t.Fatal("a Setup colony starts fogged around its landing spot")
	}
}

func TestFogMiningOpensCave(t *testing.T) {
	g := fogCaveGame(t)
	if _, err := g.SpawnNamedPawn(9, 8, "Bo"); err != nil {
		t.Fatal(err)
	}
	ver := g.World().FogVer
	g.Enqueue(command.Command{Kind: command.DesignateMine, X: 10, Y: 8, X2: 10, Y2: 8})
	g.Step(TicksPerCell*20 + MineWork + 10)
	if g.TerrainAt(10, 8) != "stone" {
		t.Fatalf("not mined: %s", g.TerrainAt(10, 8))
	}
	if h, _ := caveHidden(g); h != 0 {
		t.Fatalf("mining into the cave left %d cells hidden", h)
	}
	for _, c := range [][2]int{{16, 8}, {10, 5}, {16, 11}} {
		if g.Fogged(c[0], c[1]) {
			t.Fatalf("cave wall (%d,%d) should show", c[0], c[1])
		}
	}
	if !g.Fogged(17, 8) || !g.Fogged(20, 15) {
		t.Fatal("rock beyond the cave stays hidden")
	}
	if !fogMessage(g, caveMessage) {
		t.Fatal("opening a sealed cave posts a message")
	}
	if g.World().FogVer == ver {
		t.Fatal("FogVer must change with the fog")
	}
	// the flood expanded only the opening and the cave
	if g.fog.visited != 1+25 {
		t.Fatalf("reveal visited %d cells, want 26", g.fog.visited)
	}
}

func TestFogSmallOpeningNoMessage(t *testing.T) {
	g := fogCaveGame(t)
	g.setTerrain(10, 2, terrain.Stone) // a niche in the face: only its ring shows
	if g.Fogged(11, 2) || !g.Fogged(12, 2) {
		t.Fatal("a mined face cell shows the rock behind it, no further")
	}
	if fogMessage(g, caveMessage) {
		t.Fatal("a niche is not a cave")
	}
	g.setTerrain(11, 2, terrain.Stone)
	if g.Fogged(12, 2) {
		t.Fatal("digging on reveals the next rock")
	}
	ver := g.fog.ver
	g.setTerrain(3, 3, terrain.Gravel)  // open ground changing: nothing to reveal
	g.setTerrain(20, 15, terrain.Stone) // deep inside hidden rock, not connected
	if g.fog.ver != ver || !g.Fogged(20, 15) {
		t.Fatal("changes away from the explored area reveal nothing")
	}
}

func TestFogHidesThingsInReadModels(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	for y := 0; y < 20; y++ {
		for x := 10; x < 30; x++ {
			if !inCave(x, y) {
				g.setTerrain(x, y, terrain.Rock)
			}
		}
	}
	if !g.BuildNow("bed", "wood", "south", 14, 9) {
		t.Fatal("bed")
	}
	g.placeFrame(construction.Wall, 12, 10)
	g.SpawnStack(item.Wood, 7, 13, 7)
	g.SpawnStack(item.Wood, 3, 2, 2)
	g.SpawnPlant(12, 6)
	g.SpawnPlant(4, 4)
	g.SpawnAnimal("dustbuck", 14, 7)
	g.SpawnAnimal("dustbuck", 5, 5)
	col, _ := g.SpawnNamedPawn(3, 8, "Ada")
	g.Raid(1)
	rid := g.raiderIDs[0]
	g.positions.Set(rid, Position{X: 13, Y: 9})
	g.fogFromStart(3, 8)

	w := g.World()
	if len(w.Items) != 1 || w.Wood != 3 {
		t.Fatalf("hidden items leak: %d items, wood %d", len(w.Items), w.Wood)
	}
	if len(w.Plants) != 1 || len(w.Animals) != 1 || len(w.Buildings) != 0 || len(w.Frames) != 0 {
		t.Fatalf("hidden things leak: plants %d animals %d buildings %d frames %d",
			len(w.Plants), len(w.Animals), len(w.Buildings), len(w.Frames))
	}
	snap := g.Snapshot()
	if len(snap) != 1 || snap[0].ID != col {
		t.Fatalf("the colonist shows, the raider in the dark doesn't: %d pawns", len(snap))
	}
	// the simulation still sees the truth
	if g.stockOf(item.Wood) != 10 || len(g.animalIDs) != 2 {
		t.Fatal("fog must not touch the simulation")
	}

	g.setTerrain(10, 8, terrain.Stone)
	w = g.World()
	if len(w.Items) != 2 || w.Wood != 10 || len(w.Plants) != 2 || len(w.Animals) != 2 ||
		len(w.Buildings) != 1 || len(w.Frames) != 1 {
		t.Fatalf("revealed things missing: %d items, %d plants, %d animals, %d buildings, %d frames",
			len(w.Items), len(w.Plants), len(w.Animals), len(w.Buildings), len(w.Frames))
	}
	found := false
	for _, p := range g.Snapshot() {
		found = found || p.ID == rid
	}
	if !found {
		t.Fatal("the raider shows once revealed")
	}
}

func TestFogWorldLayer(t *testing.T) {
	g := New(Config{Seed: 1, Width: 8, Height: 8})
	if w := g.World(); w.Fog != nil || w.FogVer != 0 {
		t.Fatal("no fog: empty layer")
	}
	g = fogCaveGame(t)
	w := g.World()
	if len(w.Fog) != 30*20 || w.Fog[8*30+12] != 1 || w.Fog[8*30+3] != 0 {
		t.Fatalf("fog layer wrong: len %d", len(w.Fog))
	}
	if again := g.World(); &again.Fog[0] != &w.Fog[0] {
		t.Fatal("an unchanged layer keeps its identity")
	}
	g.setTerrain(10, 8, terrain.Stone)
	w2 := g.World()
	if &w2.Fog[0] == &w.Fog[0] || w.Fog[8*30+12] != 1 || w2.Fog[8*30+12] != 0 || w2.FogVer == w.FogVer {
		t.Fatal("a reveal hands out a fresh layer and never rewrites the old one")
	}
}

func TestFogPlacementAndOrders(t *testing.T) {
	g := fogCaveGame(t)
	id, _ := g.SpawnNamedPawn(9, 8, "Bo")
	if ok, r := g.CanPlace("wall", 12, 8); ok || r != ReasonFogged {
		t.Fatalf("build on hidden soil: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceStuff("wall", "wood", 12, 8); ok || r != ReasonFogged {
		t.Fatalf("build on hidden soil: %v %q", ok, r)
	}
	if ok, _ := g.CanPlace("wall", 3, 8); !ok {
		t.Fatal("explored ground stays buildable")
	}
	// mining may be designated into the dark (hidden rock and hidden cave
	// alike, so the marks don't give the cave away)
	g.Enqueue(command.Command{Kind: command.DesignateMine, X: 11, Y: 3, X2: 12, Y2: 8})
	g.ApplyPending()
	if !g.mines[[2]int{11, 3}] || !g.mines[[2]int{12, 8}] {
		t.Fatal("mine designations are allowed on hidden cells")
	}
	// other designations skip hidden things
	pid, _ := g.SpawnPlant(13, 8)
	g.fog.hidden[(8*30+13)>>6] |= 1 << uint((8*30+13)&63) // keep it hidden
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: 11, Y: 6, X2: 15, Y2: 10})
	g.ApplyPending()
	if p, _ := g.plants.Get(pid); p.Cut {
		t.Fatal("a hidden plant can't be marked")
	}
	// orders on a hidden tile
	opts := g.OrdersAt(id, 12, 8)
	if len(opts) != 1 || !opts[0].Disabled || opts[0].Reason != ReasonFogged {
		t.Fatalf("orders on a hidden tile: %+v", opts)
	}
	if _, why := g.forcedJob(id, command.OrderCut, 13, 8, 0); why != ReasonFogged {
		t.Fatalf("forced job on a hidden tile: %q", why)
	}
	if _, why := g.forcedJob(id, command.OrderMine, 11, 3, 0); why == ReasonFogged {
		t.Fatal("hidden rock can be ordered mined")
	}
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 13, Y: 8})
	g.ApplyPending()
	if j, _ := g.jobs.Get(id); j.Forced {
		t.Fatal("a move into the dark is refused")
	}
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 5, Y: 8})
	g.ApplyPending()
	if j, _ := g.jobs.Get(id); !j.Forced {
		t.Fatal("a move on explored ground is taken")
	}
	// revealing the cave drops the marks that turned out not to be rock
	g.setTerrain(10, 8, terrain.Stone)
	if g.mines[[2]int{12, 8}] || !g.mines[[2]int{11, 3}] {
		t.Fatal("revealed non-rock loses its mine mark; hidden rock keeps it")
	}
}

func TestFogDeconstructOpensRoom(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	// a sealed 3x3 room of walls at 10..14
	for y := 10; y <= 14; y++ {
		for x := 10; x <= 14; x++ {
			if x == 10 || x == 14 || y == 10 || y == 14 {
				if !g.BuildNow("wall", "wood", "south", x, y) {
					t.Fatalf("wall %d,%d", x, y)
				}
			}
		}
	}
	g.fogFromStart(2, 2)
	if !g.Fogged(12, 12) || g.Fogged(10, 12) {
		t.Fatal("the room is hidden, its walls show")
	}
	g.removeBuilding(g.layerBldgAt(10, 12, construction.LayerObject))
	if g.Fogged(12, 12) || g.Fogged(11, 11) {
		t.Fatal("taking a wall down reveals the room")
	}
	if fogMessage(g, caveMessage) {
		t.Fatal("nine cells are no cave")
	}
}

func TestFogDeepWaterSeen(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	for y := 0; y < 10; y++ {
		for x := 8; x < 12; x++ {
			g.setTerrain(x, y, terrain.DeepWater)
		}
	}
	g.fogFromStart(2, 5)
	if g.Fogged(10, 5) || g.Fogged(18, 5) {
		t.Fatal("open water and the far shore are seen")
	}
}

func TestFogSaveRoundTrip(t *testing.T) {
	g := fogCaveGame(t)
	g.setTerrain(10, 2, terrain.Stone)
	var buf bytes.Buffer
	if err := g.Save(&buf, "fog"); err != nil {
		t.Fatal(err)
	}
	l, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(l.fog.hidden, g.fog.hidden) || l.World().FogVer == 0 {
		t.Fatal("fog lost in a round trip")
	}
	if l.Fogged(12, 8) != g.Fogged(12, 8) || l.Fogged(11, 2) != g.Fogged(11, 2) {
		t.Fatal("fogged tiles differ after a load")
	}
	// an older save (no Fog field) and a legacy colony have no fog
	old := New(Config{Seed: 1, Width: 8, Height: 8})
	buf.Reset()
	if err := old.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	l, _, err = Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if l.fog.hidden != nil || l.World().Fog != nil {
		t.Fatal("an older save has no fog")
	}
	// no fog saves no Fog field, so StateHash (which skips empty fields)
	// pins of legacy colonies don't move
	if old.exportFog() != nil {
		t.Fatal("a colony without fog exports no fog")
	}
	// a bitset that doesn't fit the map is dropped
	l.importFog([]uint64{1, 2, 3})
	if l.fog.hidden != nil {
		t.Fatal("a mis-sized fog bitset loads as no fog")
	}
}

// fogBigCave: a 64x64 rock map with a sealed 20x20 cave (400 cells), its
// west wall the face of an open strip at x < 10.
func fogBigCave(t testing.TB) *Game {
	g := New(Config{Seed: 1, Width: 64, Height: 64})
	for y := 0; y < 64; y++ {
		for x := 10; x < 64; x++ {
			if x < 11 || x > 30 || y < 20 || y > 39 {
				g.setTerrain(x, y, terrain.Rock)
			}
		}
	}
	g.fogFromStart(3, 30)
	return g
}

func TestFogRevealCostBounded(t *testing.T) {
	g := fogBigCave(t)
	saved := slices.Clone(g.fog.hidden)
	g.setTerrain(10, 30, terrain.Stone)
	if g.fog.visited != 401 || g.Fogged(20, 30) {
		t.Fatalf("visited %d", g.fog.visited)
	}
	start := time.Now()
	const n = 50
	for range n {
		copy(g.fog.hidden, saved)
		g.fogOpen(10, 30)
	}
	if per := time.Since(start) / n; per > time.Millisecond*5 { // -race headroom; the benchmark pins 1 ms
		t.Fatalf("reveal of a 400-cell cave took %v", per)
	}
}

func BenchmarkFogRevealCave(b *testing.B) {
	g := fogBigCave(b)
	before := slices.Clone(g.fog.hidden)
	g.setTerrain(10, 30, terrain.Stone) // the opening: the cave shows
	b.ResetTimer()
	for range b.N {
		copy(g.fog.hidden, before) // hidden again (64 words)
		g.messages = g.messages[:0]
		g.fogOpen(10, 30)
	}
}
