package ebiten

import (
	"testing"

	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/application/game"
)

// bareRenderer is a Renderer with nothing wired but its one scene (map 0),
// for tests that set the art, the clock or a simulation by hand.
func bareRenderer() *Renderer {
	r := &Renderer{}
	r.scenes.start(r, 0)
	return r
}

// mapsSim is stubSim with several open maps: the world and pawns it
// reports are the viewed map's (MAP-0's read-model rule).
type mapsSim struct {
	*stubSim
	maps   []game.MapView
	viewed game.MapID
	worlds map[game.MapID]game.WorldView
	pawns  map[game.MapID][]game.PawnView
}

func newMapsSim(ids ...game.MapID) *mapsSim {
	s := &mapsSim{stubSim: &stubSim{w: 40, h: 30}, worlds: map[game.MapID]game.WorldView{}, pawns: map[game.MapID][]game.PawnView{}}
	for _, id := range ids {
		s.maps = append(s.maps, game.MapView{ID: id, Label: "Colony", Kind: "colony"})
	}
	return s
}

func (s *mapsSim) Maps() []game.MapView      { return s.maps }
func (s *mapsSim) ViewMap(id game.MapID)     { s.viewed = id }
func (s *mapsSim) ViewedMap() game.MapID     { return s.viewed }
func (s *mapsSim) World() game.WorldView     { return s.worlds[s.viewed] }
func (s *mapsSim) Snapshot() []game.PawnView { return s.pawns[s.viewed] }

// view switches the simulation's view and runs one Update's advance, where
// the renderer follows it.
func (s *mapsSim) view(r *Renderer, id game.MapID) {
	s.ViewMap(id)
	r.advance()
}

func TestSceneCamerasStaySeparate(t *testing.T) {
	t.Parallel()
	sim := newMapsSim(0, 1)
	r := New(sim)
	v := camerapkg.ColonyViewport(1280, 800)
	r.ClampCamera(v)
	r.cam.Zoom = 1.5
	r.cam.OffsetX, r.cam.OffsetY = 300, 200
	r.ClampCamera(v)
	home := r.cam

	sim.view(r, 1)
	if r.cam.Placed || r.cam.Zoom != 1 {
		t.Fatalf("a map seen for the first time starts unplaced at zoom 1: %+v", r.cam)
	}
	r.ClampCamera(v)
	r.cam.Zoom = 0.75
	r.ClampCamera(v)
	other := r.cam
	if other == home {
		t.Fatal("the second map's camera is its own")
	}

	sim.view(r, 0)
	if r.cam != home {
		t.Fatalf("back home the camera is where it was: %+v, want %+v", r.cam, home)
	}
	sim.view(r, 1)
	if r.cam != other {
		t.Fatalf("the second map's camera is kept: %+v, want %+v", r.cam, other)
	}
}

// TestSceneReentryClearsMotion: a pawn that walked while its map was
// parked appears where it stands, without sliding from where it was seen.
func TestSceneReentryClearsMotion(t *testing.T) {
	t.Parallel()
	sim := newMapsSim(0, 1)
	sim.pawns[0] = []game.PawnView{{ID: 7, X: 2, Y: 2}}
	r := New(sim)
	for range 3 {
		r.advance()
	}
	sim.view(r, 1)
	if len(sceneOf(r, 0).last) != 0 || len(sceneOf(r, 0).mo.Tracks) != 0 {
		t.Fatal("a parked scene keeps no motion")
	}
	sim.pawns[0] = []game.PawnView{{ID: 7, X: 9, Y: 5}} // it walked while unseen
	sim.view(r, 0)
	if v := r.last[7]; v.fromX != 9 || v.fromY != 5 {
		t.Fatalf("re-entered, the pawn starts where it stands, got from (%d,%d)", v.fromX, v.fromY)
	}
	if x, y := r.interpWorld(sim.pawns[0][0]); x != 9 || y != 5 {
		t.Fatalf("drawn at (%v,%v), want (9,5)", x, y)
	}
	if len(r.parts) != 0 || len(r.floaters) != 0 {
		t.Fatal("no particles or floaters carried in")
	}
}

// TestSceneReentryReplaysNoSounds: what differs between two maps is not a
// door opening, a tree falling or a pawn's death, and the map left's
// pending thunder stays there.
func TestSceneReentryReplaysNoSounds(t *testing.T) {
	t.Parallel()
	sim := newMapsSim(0, 1)
	door := game.BuildingView{ID: 1, Kind: "door", X: 4, Y: 4}
	tree := game.PlantView{ID: 2, X: 8, Y: 8}
	chopper := game.PawnView{ID: 3, X: 7, Y: 8, ActX: 8, ActY: 8, Action: game.ActionChop}
	walker := game.PawnView{ID: 4, X: 4, Y: 4}
	sim.worlds[0] = game.WorldView{Buildings: []game.BuildingView{door}, Plants: []game.PlantView{tree}}
	sim.pawns[0] = []game.PawnView{chopper, walker} // the door stands open, the tree is being cut
	sim.worlds[1] = game.WorldView{Buildings: []game.BuildingView{door}}
	r := New(sim)
	r.advance()
	r.advance()
	r.cueBus().Take()
	r.cueBus().ThunderAt = r.present + 5

	for _, id := range []game.MapID{1, 0, 1} {
		sim.view(r, id)
		r.advance()
		for _, c := range r.cueBus().Take() {
			t.Fatalf("switching to map %d cued %q", id, c.ID)
		}
		if r.heard.TerrainW != sim.World().TerrainW || len(r.heard.Buildings) != len(sim.World().Buildings) {
			t.Fatal("the scene hears the map it shows")
		}
	}
	if r.cueBus().ThunderAt != 0 {
		t.Fatal("the map left's thunder does not follow")
	}

	// the same map still cues its own changes
	sim.pawns[1] = []game.PawnView{walker}
	r.advance()
	got := r.cueBus().Take()
	if len(got) != 1 || got[0].ID != "sfx.door_open" {
		t.Fatalf("a door opening on the shown map still sounds, got %+v", got)
	}
}

// TestScenePruneAndColdBakes: only the two most recent scenes keep their
// pipeline and snow bakes; a closed map's scene goes.
func TestScenePruneAndColdBakes(t *testing.T) {
	t.Parallel()
	sim := newMapsSim(0, 1, 2)
	r := New(sim)
	r.pipe()
	sim.view(r, 1)
	sim.view(r, 2)
	if sceneOf(r, 0).wp != nil {
		t.Fatal("the coldest scene freed its pipeline and bakes")
	}
	if sceneOf(r, 1).wp == nil || sceneOf(r, 2).wp == nil {
		t.Fatal("the two most recent scenes keep their bakes")
	}
	if len(r.scenes.byID) != 3 {
		t.Fatalf("every open map keeps its scene, got %d", len(r.scenes.byID))
	}
	cam0 := sceneOf(r, 0).cam
	sim.view(r, 0)
	if r.wp == nil || r.cam != cam0 {
		t.Fatal("a cold scene rebuilds its pipeline and keeps its camera")
	}

	sim.maps = sim.maps[:2] // map 2 closed
	sim.view(r, 1)
	if _, ok := r.scenes.byID[2]; ok {
		t.Fatal("a closed map's scene is pruned")
	}
	for _, id := range r.scenes.order {
		if id == 2 {
			t.Fatal("a pruned scene leaves the recency order")
		}
	}
}

// TestSceneCarriesOverlayChoices: the overlays, zones display and minimap
// the player chose stay as they were on the next map.
func TestSceneCarriesOverlayChoices(t *testing.T) {
	t.Parallel()
	sim := newMapsSim(0, 1)
	r := New(sim)
	r.overlays().Roof.On = true
	r.overlays().Zones.Hide = true
	r.mini.Off = true
	sim.view(r, 1)
	if !r.overlays().Roof.On || !r.overlays().Zones.Hide || !r.mini.Off {
		t.Fatal("the overlay and minimap choices follow the player")
	}
}

// TestSceneViewsRoundTrip: every map's camera is saved, and a load puts
// each back when its map is next shown, also one never visited between
// the load and the next save.
func TestSceneViewsRoundTrip(t *testing.T) {
	t.Parallel()
	sim := newMapsSim(0, 1, 2)
	r := New(sim)
	v := camerapkg.ColonyViewport(1280, 800)
	r.cam.Zoom = 1.25
	r.ClampCamera(v)
	sim.view(r, 1)
	r.cam.Zoom = 0.8
	r.ClampCamera(v)
	saved := r.scenes.views(v)
	if len(saved) != 2 {
		t.Fatalf("the placed cameras are saved: %+v", saved)
	}

	sim2 := newMapsSim(0, 1, 2)
	r2 := New(sim2)
	r2.scenes.resumeViews(saved)
	r2.ClampCamera(v)
	if got := r2.cam.View(v); !near(got, saved[0]) {
		t.Fatalf("map 0 resumes at %+v, want %+v", got, saved[0])
	}
	if again := r2.scenes.views(v); !near(again[1], saved[1]) {
		t.Fatalf("a map not shown since the load keeps its saved camera: %+v", again)
	}
	sim2.view(r2, 1)
	r2.ClampCamera(v)
	if got := r2.cam.View(v); !near(got, saved[1]) {
		t.Fatalf("map 1 resumes at %+v, want %+v", got, saved[1])
	}
}

func near(a, b camerapkg.View) bool {
	d := func(x, y float64) bool { return x-y < 1e-6 && y-x < 1e-6 }
	return d(a.CX, b.CX) && d(a.CY, b.CY) && d(a.Zoom, b.Zoom)
}

// sceneOf is id's scene (test helper over sceneSet).
func sceneOf(r *Renderer, id game.MapID) *mapScene { return r.scenes.byID[id] }
