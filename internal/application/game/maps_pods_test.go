package game

import (
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/roof"
)

// Transport pods onto open maps (MAP-10; wiki: Transport pod).

// podHold puts pawns and n steel straight into pod, boarding the pawns.
func podHold(t *testing.T, g *Game, pod entity.EntityID, n int, pawns ...entity.EntityID) {
	t.Helper()
	travelOf(g)
	s := g.worldSt.ax9.Export()
	if s == nil {
		s = &travel.Save{}
	}
	h := travel.Hold{Pawns: pawns}
	if n > 0 {
		h.Items = []item.Stack{{Kind: item.Steel, Qty: n}}
	}
	s.Holds = append(s.Holds, travel.PodHold{Pod: pod, Hold: h})
	g.worldSt.ax9 = travel.Import(s)
	for _, id := range pawns {
		if !g.stashPawn(id, pod) {
			t.Fatalf("could not board %d", id)
		}
	}
}

// siteWithBob opens a site map on a plain tile near home and keeps it
// alive with a second colonist; ada stays free to fly.
func siteWithBob(t *testing.T) (g *Game, tile int, ada, bob entity.EntityID) {
	t.Helper()
	g = worldGame(t)
	tile, ada, bob = plainNear(t, g), g.pawnIDs[0], g.pawnIDs[1]
	if _, ok := DevOpenMap(g, tile); !ok {
		t.Fatal("no site map")
	}
	arriveAt(t, g, tile, bob)
	g.Step(1)
	if onMap(g, bob) == g.maps[0] {
		t.Fatal("bob did not enter the site map")
	}
	return g, tile, ada, bob
}

// fly launches ada (boarded with steel) from home to tile in mode and
// steps until she is on a map other than the first (or the flight ends).
func fly(t *testing.T, g *Game, tile int, mode uint8, ada entity.EntityID, x2, y2 int) *mapState {
	t.Helper()
	x, y := ax9Free(t, g)
	pod, _ := ax9PodRig(t, g, x+1, y, 150)
	podHold(t, g, pod, 12, ada)
	if why, _ := g.LaunchReason(pod, tile, mode); why != "" {
		t.Fatalf("refused: %q", why)
	}
	g.Enqueue(command.Command{Kind: command.LaunchPods, Target: uint64(pod), Count: tile, Mode: mode, X2: x2, Y2: y2})
	for i := 0; i < 6*caravan.PodTicksPerTile+600; i++ {
		g.Step(1)
		if m := onMap(g, ada); m != nil && m != g.maps[0] {
			return m
		}
	}
	t.Fatalf("pods never landed: %+v", g.World().AX.Pods)
	return nil
}

func TestPodsLandOnOpenMap(t *testing.T) {
	t.Parallel()
	side := maps.SiteSide
	cases := []struct {
		name string
		mode uint8
		x, y int
		ok   func(x, y int) bool
	}{
		{"edge", command.PodArriveDrop, 0, 0, func(x, y int) bool { return min(x, y, side-1-x, side-1-y) <= 8 }},
		{"centre", command.PodArriveDropCentre, 0, 0, func(x, y int) bool { return max(x-side/2, side/2-x, y-side/2, side/2-y) <= 8 }},
		{"spot", command.PodArriveSpot, 40, 100, func(x, y int) bool { return max(x-40, 40-x, y-100, 100-y) <= 8 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g, tile, ada, _ := siteWithBob(t)
			m := fly(t, g, tile, c.mode, ada, c.x, c.y)
			p, _ := m.positions.Get(ada)
			if !c.ok(p.X, p.Y) {
				t.Fatalf("ada landed at (%d, %d)", p.X, p.Y)
			}
			steel := 0
			m.stacks.All(func(_ entity.EntityID, st item.Stack) {
				if st.Kind == item.Steel {
					steel += st.Qty
				}
			})
			if steel < 12 {
				t.Fatalf("%d steel came down with the pods", steel)
			}
			if g.isStashed(ada) || len(g.World().AX.Pods) != 0 || fleetOf(g).Of(ada) != nil {
				t.Fatal("ada is still aboard or in a caravan")
			}
		})
	}
}

// A spot under overhead mountain is moved to the nearest open cell.
func TestPodSpotAvoidsOverheadMountain(t *testing.T) {
	t.Parallel()
	g, _, _, _ := siteWithBob(t)
	m := g.maps[1]
	g.mapState = m
	defer func() { g.mapState = g.view }()
	x, y := 60, 60
	for dy := -3; dy <= 3; dy++ {
		for dx := -3; dx <= 3; dx++ {
			if g.grid.InBounds(x+dx, y+dy) {
				g.roofs[(y+dy)*g.grid.Width()+x+dx] = roof.Natural | roof.NaturalThick
			}
		}
	}
	lx, ly, ok := mapsGround{g}.Landing(x, y)
	if !ok || g.roofAt(lx, ly).Thick() || max(lx-x, x-lx, ly-y, y-ly) < 4 {
		t.Fatalf("landing (%d, %d) ok %v under rock", lx, ly, ok)
	}
}

// Pods launched from a site map fly back to the colony: the range is
// measured from the site's tile and they come down at the drop spot.
func TestPodsFromSiteMapBackToColony(t *testing.T) {
	t.Parallel()
	g, tile, ada, bob := siteWithBob(t)
	home := g.colonyTile()
	site := g.maps[1]
	g.mapState = site
	x, y := ax9Free(t, g)
	pod, _ := ax9PodRig(t, g, x+1, y, 150)
	g.mapState = g.view
	podHold(t, g, pod, 9, ada) // ada boards on her own map
	g.mapState = site
	if _, d := g.LaunchReason(pod, home, command.PodArriveDrop); d != 2 {
		t.Fatalf("distance %d from the site, want 2", d)
	}
	g.Enqueue(command.Command{Kind: command.LaunchPods, Target: uint64(pod), Count: home, Mode: command.PodArriveDrop, Map: site.id, MapSet: true})
	g.mapState = g.view
	for i := 0; i < 4*caravan.PodTicksPerTile+600 && onMap(g, ada) != g.maps[0]; i++ {
		g.Step(1)
	}
	if onMap(g, ada) != g.maps[0] || g.isStashed(ada) {
		t.Fatalf("ada did not reach the colony: pods %+v", g.World().AX.Pods)
	}
	if onMap(g, bob) != site || g.maps[1].tile != tile {
		t.Fatal("bob left the site")
	}
}

func TestPodLaunchReasons(t *testing.T) {
	t.Parallel()
	g, tile, ada, _ := siteWithBob(t)
	x, y := ax9Free(t, g)
	pod, _ := ax9PodRig(t, g, x+1, y, 150)
	podHold(t, g, pod, 0, ada)
	plain := plainNear(t, g)
	for _, p := range testPlanet().Neighbours(g.colonyTile()) {
		if int(p) != tile && g.settlementFaction(int(p)) == 0 && !fleetOf(g).MapOpen(int(p)) {
			plain = int(p)
			break
		}
	}
	if why, _ := g.LaunchReason(pod, plain, command.PodArriveSpot); why == "" {
		t.Fatal("a landing spot was allowed on a tile with no map")
	}
	if why, _ := g.LaunchReason(pod, tile, command.PodArriveSpot); why != "" {
		t.Fatalf("spot on an open map: %q", why)
	}
	if why, _ := g.LaunchReason(pod, g.colonyTile(), command.PodArriveFound); why == "" {
		t.Fatal("founding on the colony's own tile was allowed")
	}
	if why, _ := g.LaunchReason(pod, landable(t, g, 1)[0], command.PodArriveFound); why == "" {
		t.Fatal("the default cap of one colony allowed a second")
	}
	SetMaxColonies(g, 5)
	if why, _ := g.LaunchReason(pod, landable(t, g, 1)[0], command.PodArriveFound); why != "" {
		t.Fatalf("founding on open ground: %q", why)
	}

}

func TestPodsOnMapSameSeed(t *testing.T) {
	t.Parallel()
	run := func() uint64 {
		g, tile, ada, _ := siteWithBob(t)
		fly(t, g, tile, command.PodArriveSpot, ada, 70, 70)
		g.Step(300)
		return g.StateHash()
	}
	if a, b := run(), run(); a != b {
		t.Fatalf("hashes %x and %x", a, b)
	}
}

// Pods set down their people and found a colony on open ground, once the
// colony cap allows it.
func TestPodsFoundColony(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	SetMaxColonies(g, 5)
	tile, ada := landable(t, g, 1)[0], g.pawnIDs[0]
	x, y := ax9Free(t, g)
	pod, _ := ax9PodRig(t, g, x+1, y, 150)
	podHold(t, g, pod, 5, ada)
	g.Enqueue(command.Command{Kind: command.LaunchPods, Target: uint64(pod), Count: tile, Mode: command.PodArriveFound})
	for i := 0; i < 6*caravan.PodTicksPerTile+600 && len(g.maps) < 2; i++ {
		g.Step(1)
	}
	if len(g.maps) != 2 || g.maps[1].tile != tile || g.Maps()[1].Kind != "colony" {
		t.Fatalf("no colony on tile %d: %d maps, pods %+v", tile, len(g.maps), g.World().AX.Pods)
	}
	if m := onMap(g, ada); m != g.maps[1] {
		t.Fatal("ada is not in the new colony")
	}
}
