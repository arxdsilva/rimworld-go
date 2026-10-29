package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/stain"
)

func homeCount(w WorldView) int {
	n := 0
	for _, v := range w.Home {
		n += int(v)
	}
	return n
}

func TestDesignateHomePaintClear(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	w0 := g.World()
	if len(w0.Home) != w0.TerrainW*g.grid.Height() || homeCount(w0) != 0 {
		t.Fatalf("fresh map Home: %d tiles of %d", homeCount(w0), len(w0.Home))
	}
	g.addDirt(tileIdx(g, 3, 3), 300)
	g.Enqueue(command.Command{Kind: command.DesignateHome, X: 5, Y: 4, X2: 2, Y2: 2, Enabled: true})
	g.Step(1)
	w1 := g.World()
	if homeCount(w1) != 12 || w1.HomeVer == w0.HomeVer || w1.Home[3*w1.TerrainW+3] != 1 {
		t.Fatalf("painted %d tiles, ver %d -> %d", homeCount(w1), w0.HomeVer, w1.HomeVer)
	}
	if !queued(g, 3, 3) {
		t.Fatal("dirty tile painted Home not queued")
	}
	// painting the same tiles again changes nothing
	g.paintHome(2, 2, 5, 4, true)
	if g.World().HomeVer != w1.HomeVer {
		t.Fatal("a no-op paint bumped the version")
	}
	g.Enqueue(command.Command{Kind: command.DesignateHome, X: 3, Y: 3, X2: 3, Y2: 4, Enabled: false})
	g.Step(1)
	w2 := g.World()
	if homeCount(w2) != 10 || w2.HomeVer == w1.HomeVer || queued(g, 3, 3) {
		t.Fatalf("cleared to %d tiles", homeCount(w2))
	}
	// off the map is clamped
	g.paintHome(-5, -5, 1, 1, true)
	if homeCount(g.World()) != 14 {
		t.Fatalf("clamped paint gave %d tiles", homeCount(g.World()))
	}
	g.paintHome(100, 100, 120, 120, true)
	// the view is a copy
	w3 := g.World()
	w3.Home[0] = 0
	if !g.inHome(0) {
		t.Fatal("the view aliases the Home area")
	}
	// clearing a map with no Home is a no-op
	h := dirtGame(t)
	h.paintHome(0, 0, 5, 5, false)
	if h.World().HomeVer != 0 {
		t.Fatal("clearing nothing bumped the version")
	}
}

func TestAutoHome(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	placeBuilt(t, g, construction.Lamp, construction.South, 10, 6)
	w := g.World()
	for _, c := range [][2]int{{8, 4}, {12, 8}, {10, 6}, {12, 4}} {
		if w.Home[c[1]*w.TerrainW+c[0]] != 1 {
			t.Fatalf("%v outside the auto Home", c)
		}
	}
	for _, c := range [][2]int{{7, 6}, {13, 6}, {10, 3}, {10, 9}} {
		if w.Home[c[1]*w.TerrainW+c[0]] == 1 {
			t.Fatalf("%v inside the auto Home", c)
		}
	}
	before := homeCount(w)
	placeBuilt(t, g, construction.Floor, construction.South, 22, 6)
	placeBuilt(t, g, construction.Grave, construction.South, 25, 3)
	if homeCount(g.World()) != before {
		t.Fatal("floors and graves grow the Home area")
	}
	// a new colony lands inside a 13x13 Home
	c := NewColony(Config{Seed: 2, Width: 48, Height: 40})
	cw := c.World()
	cx, cy := 24, 20
	for _, p := range [][2]int{{cx - 6, cy - 6}, {cx + 6, cy + 6}, {cx, cy}} {
		if cw.Home[p[1]*cw.TerrainW+p[0]] != 1 {
			t.Fatalf("landing Home misses %v", p)
		}
	}
	if homeCount(cw) < 169 {
		t.Fatalf("landing Home has %d tiles", homeCount(cw))
	}
}

func TestRoomFilthLowersScore(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	box(t, g, 2, 2, 7, 7, [2]int{4, 7})
	r0 := g.World().Rooms[0]
	if r0.Filth != 0 || r0.Cleanliness != "clean" || r0.FilthPenalty != 0 {
		t.Fatalf("clean room = %+v", r0)
	}
	for y := 3; y <= 6; y++ {
		for x := 3; x <= 6; x++ {
			g.addDirt(tileIdx(g, x, y), 150)
		}
	}
	g.stamp(3, 3, stain.Blood, 800)
	r1 := g.World().Rooms[0]
	sum := 16*150 + 800
	if r1.Filth != sum/16 || r1.Cleanliness != filth.Label(sum/16) || r1.Cleanliness != "filthy" {
		t.Fatalf("filthy room = %+v", r1)
	}
	if r1.FilthPenalty != filth.Penalty(sum, 16) || r1.FilthPenalty == 0 || r1.Score != r0.Score-r1.FilthPenalty {
		t.Fatalf("score %d (clean %d), penalty %d", r1.Score, r0.Score, r1.FilthPenalty)
	}
	if info, _ := g.RoomInfo(4, 4); info.Score != r1.Score {
		t.Fatal("RoomInfo disagrees")
	}
}

func TestSleptInAwfulRoomWhenFilthy(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	box(t, g, 2, 2, 7, 7, [2]int{4, 7})
	placeBuilt(t, g, construction.Bed, construction.South, 3, 3)
	bed := g.layerBldgAt(3, 3, construction.LayerObject)
	id, _ := g.SpawnNamedPawn(3, 3, "Ada")
	r := g.roomAt(3, 3)
	if s := g.roomScore(r); s < 0 || s >= filth.MaxPenalty {
		t.Fatalf("fixture bedroom scores %d", s)
	}
	g.sleptAt[id] = bed
	g.rest.Waking.After(id)
	if g.hasMemory(id, mood.SleptInAwfulRoom) {
		t.Fatal("a clean bedroom is awful")
	}
	for y := 3; y <= 6; y++ {
		for x := 3; x <= 6; x++ {
			g.addDirt(tileIdx(g, x, y), filth.Full)
		}
	}
	g.rest.Waking.After(id)
	if !g.hasMemory(id, mood.SleptInAwfulRoom) {
		t.Fatalf("a filthy bedroom (score %d) is not awful", g.roomScore(r))
	}
}
