package ebiten

import (
	"math"
	"testing"

	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
)

// soundRecorder is a Sound that records what the director asks for.
type soundRecorder struct {
	have  map[string]bool // nil: every id exists
	plays []soundCall
	loops map[string]soundCall // latest per emitter key
	music []string
	vols  []config.Audio
	upd   int
}

type soundCall struct {
	id       string
	vol, pan float64
}

func (s *soundRecorder) Has(id string) bool { return s.have == nil || s.have[id] }
func (s *soundRecorder) Play(id string, vol, pan float64) {
	s.plays = append(s.plays, soundCall{id, vol, pan})
}
func (s *soundRecorder) Loop(key, id string, vol, pan float64) {
	if s.loops == nil {
		s.loops = map[string]soundCall{}
	}
	s.loops[key] = soundCall{id, vol, pan}
}
func (s *soundRecorder) Music(track string, _ float64) { s.music = append(s.music, track) }
func (s *soundRecorder) SetVolumes(v config.Audio)     { s.vols = append(s.vols, v) }
func (s *soundRecorder) Update()                       { s.upd++ }

func (s *soundRecorder) played(id string) []soundCall {
	var out []soundCall
	for _, p := range s.plays {
		if p.id == id {
			out = append(out, p)
		}
	}
	return out
}

func soundApp(sim *stubSim) (*App, *soundRecorder) {
	rec := &soundRecorder{}
	a := testApp(sim)
	a.cfg.Sound = rec
	a.finishBoot()
	a.startGame()
	a.sw, a.sh = 1280, 720
	return a, rec
}

// centreTile is the world tile under the middle of the colony view.
func centreTile(a *App) (float64, float64) {
	v := camerapkg.ColonyViewport(a.sw, a.sh)
	return a.renderer.screenToWorld(int((v.X0+v.X1)/2), int((v.Y0+v.Y1)/2))
}

func TestStrikeCueIsCentredAndOncePerSwing(t *testing.T) {
	t.Parallel()
	r, sim := workRenderer(tickAt(3, 46, 0.1))
	p := worker(game.ActionChop)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	if len(r.cueBus().Cues) != 0 {
		t.Fatal("no sound before the strike")
	}
	sim.tick = clock.Tick(tickAt(3, 46, creature.StrikeAt) + 46)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	if len(r.cueBus().Cues) != 1 || r.cueBus().Cues[0].ID != "sfx.chop" || r.cueBus().Cues[0].X != 6.5 {
		t.Fatalf("one chop at the trunk, got %+v", r.cueBus().Cues)
	}
	r.cueBus().Take()
	r.animateWork(game.WorldView{}, []game.PawnView{p}) // clock stopped: paused
	if len(r.cueBus().Cues) != 0 {
		t.Fatalf("a paused clock makes no sound: %+v", r.cueBus().Cues)
	}
}

func TestSilentActionsMakeNoSound(t *testing.T) {
	t.Parallel()
	r, sim := workRenderer(tickAt(3, 60, 0.1))
	p := worker(game.ActionResearch)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	sim.tick = clock.Tick(tickAt(3, 60, creature.StrikeAt) + 60)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	if len(r.cueBus().Cues) != 0 {
		t.Fatalf("research is quiet: %+v", r.cueBus().Cues)
	}
}

func TestDirectorPansAndCulls(t *testing.T) {
	t.Parallel()
	a, rec := soundApp(&stubSim{w: 200, h: 200})
	cx, cy := centreTile(a)
	r := a.renderer
	r.cue("sfx.chop", cx, cy, 1)
	r.cue("sfx.pick", cx+10, cy, 1)    // right of centre
	r.cue("sfx.hammer", cx+500, cy, 1) // far off screen
	a.colonyCues()
	chop, pick := rec.played("sfx.chop"), rec.played("sfx.pick")
	if len(chop) != 1 || math.Abs(chop[0].pan) > 0.05 {
		t.Fatalf("centre chop: %+v", chop)
	}
	if len(pick) != 1 || pick[0].pan <= 0.1 {
		t.Fatalf("pick to the right should pan right: %+v", pick)
	}
	if len(rec.played("sfx.hammer")) != 0 {
		t.Fatal("an off-screen hammer is out of earshot")
	}
}

func TestDirectorLimitsRepeats(t *testing.T) {
	t.Parallel()
	a, rec := soundApp(&stubSim{w: 200, h: 200})
	cx, cy := centreTile(a)
	for i := 0; i < 7; i++ {
		a.renderer.cue("sfx.chop", cx+float64(i)*0.2, cy, 1)
	}
	a.colonyCues()
	if n := len(rec.played("sfx.chop")); n != cue.MaxPerID {
		t.Fatalf("%d chops in one frame, want %d", n, cue.MaxPerID)
	}
	a.snd.Frame++
	a.renderer.cue("sfx.chop", cx, cy, 1)
	a.colonyCues()
	if n := len(rec.played("sfx.chop")); n != cue.MaxPerID {
		t.Fatalf("a chop one frame later should wait for the gap, got %d plays", n)
	}
}

func TestDoorAndTreeCues(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 20, h: 20})
	door := game.BuildingView{ID: 1, Kind: "door", X: 4, Y: 4}
	tree := game.PlantView{ID: 2, X: 8, Y: 8}
	chopper := game.PawnView{ID: 3, X: 7, Y: 8, ActX: 8, ActY: 8, Action: game.ActionChop}
	w := game.WorldView{Buildings: []game.BuildingView{door}, Plants: []game.PlantView{tree}}
	r.listenWorld(w, []game.PawnView{chopper})
	walker := game.PawnView{ID: 4, X: 4, Y: 4}
	r.listenWorld(game.WorldView{Buildings: []game.BuildingView{door}}, []game.PawnView{walker})
	ids := map[string]bool{}
	for _, c := range r.cueBus().Take() {
		ids[c.ID] = true
	}
	if !ids["sfx.door_open"] || !ids["sfx.tree_fall"] {
		t.Fatalf("want a door opening and a tree falling, got %v", ids)
	}
	r.listenWorld(game.WorldView{Buildings: []game.BuildingView{door}}, nil)
	if c := r.cueBus().Take(); len(c) != 1 || c[0].ID != "sfx.door_close" {
		t.Fatalf("want the door closing, got %+v", c)
	}
}

func TestMusicFollowsThreat(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 50, h: 50}
	a, rec := soundApp(sim)
	a.soundFrame()
	if last := rec.music[len(rec.music)-1]; last != trackCalm {
		t.Fatalf("colony music %q, want calm", last)
	}
	sim.pawns = []game.PawnView{{ID: entity.EntityID(9), X: 3, Y: 3, Hostile: true}}
	a.soundFrame()
	if last := rec.music[len(rec.music)-1]; last != trackTension {
		t.Fatalf("raid music %q, want tension", last)
	}
	sim.pawns = nil
	a.renderer.present = 100
	a.renderer.threatAt = a.renderer.present
	a.renderer.present += tensionHold - 1
	a.soundFrame()
	if last := rec.music[len(rec.music)-1]; last != trackTension {
		t.Fatal("tension holds after the raid")
	}
	a.renderer.present += 2
	a.soundFrame()
	if last := rec.music[len(rec.music)-1]; last != trackCalm {
		t.Fatal("calm returns once the hold is over")
	}
}

func TestMenuMusicFallsBackToCalm(t *testing.T) {
	t.Parallel()
	rec := &soundRecorder{have: map[string]bool{trackCalm: true, "amb.campfire": true}}
	a := testApp(&stubSim{w: 4, h: 4})
	a.cfg.Sound = rec
	a.finishBoot()
	a.soundFrame()
	if rec.music[0] != trackCalm {
		t.Fatalf("menu without a menu theme plays %q, want calm", rec.music[0])
	}
	if _, ok := rec.loops["menu:fire"]; !ok {
		t.Fatal("the menu campfire crackles")
	}
	if _, ok := rec.loops["menu:night"]; ok {
		t.Fatal("no night bed when it is not in the sound list")
	}
}

func TestNightAmbience(t *testing.T) {
	t.Parallel()
	a, rec := soundApp(&stubSim{w: 50, h: 50})
	a.renderer.heard = game.WorldView{Hour: 23, Season: "summer", OutdoorTemp: 180}
	a.soundFrame()
	if rec.loops["amb.night"].vol <= 0 {
		t.Fatal("crickets on a warm summer night")
	}
	if _, ok := rec.loops["amb.day"]; ok {
		t.Fatal("no daytime birds at night")
	}
	a.renderer.heard = game.WorldView{Hour: 12, Season: "summer", OutdoorTemp: 220}
	a.soundFrame()
	if rec.loops["amb.night"].vol != 0 || rec.loops["amb.day"].vol <= 0 {
		t.Fatalf("noon: %+v", rec.loops)
	}
}

func TestClickSounds(t *testing.T) {
	t.Parallel()
	rec := &soundRecorder{}
	a := testApp(&stubSim{w: 4, h: 4})
	a.cfg.Sound = rec
	a.uiSound(uikit.Button{ID: "menu.newgame"})
	a.uiSound(uikit.Button{ID: "menu.plugins", Disabled: true})
	a.uiSound(uikit.Button{ID: "options.master.inc"})
	if len(rec.played("ui.click")) != 1 || len(rec.played("ui.deny")) != 1 || len(rec.plays) != 2 {
		t.Fatalf("plays: %+v", rec.plays)
	}
}

func TestDayWeight(t *testing.T) {
	t.Parallel()
	for h, want := range map[float64]float64{0: 0, 5.75: 0.5, 12: 1, 19.75: 0.5, 22: 0} {
		if got := dayWeight(h); math.Abs(got-want) > 1e-9 {
			t.Fatalf("dayWeight(%v) = %v, want %v", h, got, want)
		}
	}
}

func TestDirectorFadesWithZoom(t *testing.T) {
	t.Parallel()
	a, rec := soundApp(&stubSim{w: 48, h: 40})
	r := a.renderer
	v := camerapkg.ColonyViewport(a.sw, a.sh)
	hear := func(zoom, dx float64) []soundCall {
		r.cam.Zoom = zoom
		rec.plays = nil
		a.snd.Frame += cueGap
		cx, cy := centreTile(a)
		r.cue("sfx.chop", cx+dx, cy, 1)
		a.colonyCues()
		return rec.played("sfx.chop")
	}
	near, mid := hear(1.5, 0), hear(0.5, 0)
	if len(near) != 1 || len(mid) != 1 || mid[0].vol >= near[0].vol*0.6 {
		t.Fatalf("zoomed out should be much quieter: near %+v mid %+v", near, mid)
	}
	nearR, midR := hear(1.5, 4), hear(0.5, 1.3) // about the same spot on screen
	if len(nearR) != 1 || len(midR) != 1 || midR[0].pan >= nearR[0].pan {
		t.Fatalf("zoomed out should pan narrower: near %+v mid %+v", nearR, midR)
	}
	if got := hear(r.MinZoomFor(v), 0); len(got) != 0 {
		t.Fatalf("fully zoomed out effects are silent, got %+v", got)
	}
	r.cam.Zoom = 1.5
	cx, cy := centreTile(a)
	r.heard = game.WorldView{Hour: 12, Buildings: []game.BuildingView{{ID: 5, Kind: "campfire", X: int(cx), Y: int(cy), Burning: true}}}
	want := map[string]cue.Loop{}
	a.colonyAmbience(want)
	if _, ok := want["fire:5"]; !ok {
		t.Fatal("the campfire crackles zoomed in")
	}
	r.cam.Zoom = r.MinZoomFor(v)
	want = map[string]cue.Loop{}
	a.colonyAmbience(want)
	if _, ok := want["fire:5"]; ok {
		t.Fatal("a campfire is silent fully zoomed out")
	}
}

// terrainWorld is a 4x1 strip of one terrain with the given marks.
func terrainWorld(ground string, mines, smooths [][2]int) game.WorldView {
	code := uint8(0)
	for i, n := range game.TerrainNames() {
		if n == ground {
			code = uint8(i)
		}
	}
	return game.WorldView{TerrainW: 4, Terrain: []uint8{code, code, code, code}, Mines: mines, Smooths: smooths}
}

func TestMineAndSmoothCuesOnlyOnTerrainChange(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 4, h: 1})
	mark := [][2]int{{1, 0}}
	// cancelling a mine mark leaves the rock: no break sound
	r.listenWorld(terrainWorld("rock", mark, nil), nil)
	r.listenWorld(terrainWorld("rock", nil, nil), nil)
	if c := r.cueBus().Take(); len(c) != 0 {
		t.Fatalf("cancelled mine cued %+v", c)
	}
	// mined out: the rock is gone
	r.listenWorld(terrainWorld("rock", mark, nil), nil)
	r.listenWorld(terrainWorld("stone", nil, nil), nil)
	if c := r.cueBus().Take(); len(c) != 1 || c[0].ID != "sfx.rock_break" || c[0].X != 1.5 {
		t.Fatalf("mined rock cues %+v", c)
	}
	// smoothing: silent when cancelled, a chisel strike when done
	r.listenWorld(terrainWorld("rock", nil, mark), nil)
	r.listenWorld(terrainWorld("rock", nil, nil), nil)
	if c := r.cueBus().Take(); len(c) != 0 {
		t.Fatalf("cancelled smoothing cued %+v", c)
	}
	r.listenWorld(terrainWorld("rock", nil, mark), nil)
	r.listenWorld(terrainWorld("rock_smooth", nil, nil), nil)
	if c := r.cueBus().Take(); len(c) != 1 || c[0].ID != "sfx.pick" {
		t.Fatalf("smoothed rock cues %+v", c)
	}
}
