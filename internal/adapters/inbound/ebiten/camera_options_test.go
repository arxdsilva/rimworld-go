package ebiten

import (
	"testing"

	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/platform/config"
)

// pauseCase feeds a quiet world, then a newer one, and reports whether the
// game paused itself.
func pausedBy(mode string, next game.WorldView) bool {
	r := New(&stubSim{w: 8, h: 8})
	r.smooth.Prefs.Mode = mode
	r.watchThreats(game.WorldView{Messages: []game.MessageView{{Tick: 1, Text: "old", Danger: true}},
		Letters: []game.LetterView{{ID: 1, Kind: "threat"}}}) // there at the start: never pauses
	if r.speed == speedPaused {
		return true
	}
	r.watchThreats(next)
	return r.speed == speedPaused
}

func TestPauseModes(t *testing.T) {
	t.Parallel()
	oldM := game.MessageView{Tick: 1, Text: "old", Danger: true}
	oldL := game.LetterView{ID: 1, Kind: "threat"}
	danger := game.WorldView{Messages: []game.MessageView{oldM, {Tick: 9, Text: "wolves", Danger: true}}, Letters: []game.LetterView{oldL}}
	threatLetter := game.WorldView{Messages: []game.MessageView{oldM}, Letters: []game.LetterView{oldL, {ID: 2, Kind: "threat"}}}
	cautionLetter := game.WorldView{Messages: []game.MessageView{oldM}, Letters: []game.LetterView{oldL, {ID: 3, Kind: "caution"}}}
	introLetter := game.WorldView{Messages: []game.MessageView{oldM}, Letters: []game.LetterView{oldL, {ID: 4, Kind: "info", Intro: true}}}
	cases := []struct {
		mode                    string
		danger, threat, caution bool
		intro                   bool
	}{
		{config.AutoPauseNever, false, false, false, false},
		{config.AutoPauseMajor, false, true, false, false},
		{config.AutoPauseThreat, true, true, false, false},
		{config.AutoPauseLetter, true, true, true, false},
	}
	for _, c := range cases {
		for name, got := range map[string][2]bool{
			"danger":  {pausedBy(c.mode, danger), c.danger},
			"threat":  {pausedBy(c.mode, threatLetter), c.threat},
			"caution": {pausedBy(c.mode, cautionLetter), c.caution},
			"intro":   {pausedBy(c.mode, introLetter), c.intro},
		} {
			if got[0] != got[1] {
				t.Errorf("mode %s, %s: paused %v, want %v", c.mode, name, got[0], got[1])
			}
		}
	}
}

func TestPauseNoReplayOfSameLetter(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 8, h: 8})
	r.smooth.Prefs.Mode = config.AutoPauseLetter
	r.watchThreats(game.WorldView{})
	w := game.WorldView{Letters: []game.LetterView{{ID: 5, Kind: "good"}}}
	r.watchThreats(w)
	r.speed = speedNormal
	r.watchThreats(w)
	if r.speed != speedNormal {
		t.Fatal("a letter pauses once")
	}
}

func TestUnpauseSpeed(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 8, h: 8})
	r.setSpeed(speedVeryFast)
	r.togglePause()
	r.togglePause()
	if r.speed != speedNormal {
		t.Fatalf("normal mode resumes at normal: %v", r.speed)
	}
	r.smooth.Prefs.UnpauseLast = true
	r.setSpeed(speedVeryFast)
	r.togglePause()
	if r.speed != speedPaused {
		t.Fatal("paused")
	}
	r.togglePause()
	if r.speed != speedVeryFast {
		t.Fatalf("last mode resumes at the speed before: %v", r.speed)
	}
	r.setSpeed(speedFast)
	r.watchThreats(game.WorldView{})
	r.watchThreats(game.WorldView{Messages: []game.MessageView{{Tick: 3, Danger: true}}}) // paused by a threat
	r.togglePause()
	if r.speed != speedFast {
		t.Fatalf("an automatic pause remembers the speed too: %v", r.speed)
	}
}

func TestCameraPauseOnLoad(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		opt  bool
		tick uint64
		want bool
	}{{true, 500, true}, {false, 500, false}, {true, 0, false}} {
		sim := &stubSim{w: 8, h: 8}
		sim.tick = clock.Tick(c.tick)
		a := testApp(sim)
		a.settings.General.PauseOnLoad = c.opt
		a.startSession(sim)
		if got := a.renderer.speed == speedPaused; got != c.want {
			t.Errorf("%+v: paused %v", c, got)
		}
		a.settings.General.PauseOnLoad = true
		a.syncRenderer() // a later settings change never pauses the running game
		a.renderer.speed = speedNormal
		a.syncRenderer()
		if a.renderer.speed != speedNormal {
			t.Errorf("%+v: a settings change paused the game", c)
		}
	}
}

func cameraRenderer() (*Renderer, camerapkg.Viewport) {
	r := New(&stubSim{w: 64, h: 64})
	v := camerapkg.Viewport{X1: 800, Y1: 600}
	r.cam = camerapkg.Camera{Zoom: 1, Placed: true, OffsetX: 500, OffsetY: 500}
	return r, v
}

func TestCameraZoomToMouse(t *testing.T) {
	t.Parallel()
	r, v := cameraRenderer()
	r.applyCamera(v, camInput{wheel: 1, wx: 100, wy: 50})
	if r.smooth.AnchorX != 100 || r.smooth.AnchorY != 50 {
		t.Fatalf("zooms about the mouse: %v,%v", r.smooth.AnchorX, r.smooth.AnchorY)
	}
	r.smooth.Prefs.CenterZoom = true
	r.applyCamera(v, camInput{wheel: 1, wx: 100, wy: 50})
	if r.smooth.AnchorX != 400 || r.smooth.AnchorY != 300 {
		t.Fatalf("zoom-to-mouse off zooms about the middle: %v,%v", r.smooth.AnchorX, r.smooth.AnchorY)
	}
}

func TestCameraDragScale(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		pct  int
		want float64
	}{{0, 20}, {100, 20}, {200, 40}, {50, 10}} {
		r, v := cameraRenderer()
		r.smooth.Prefs.Drag = c.pct
		r.applyCamera(v, camInput{dragX: -20})
		if got := r.cam.OffsetX - 500; got != c.want {
			t.Errorf("drag %d%%: moved %v, want %v", c.pct, got, c.want)
		}
	}
}

func TestZoomOutToWorld(t *testing.T) {
	t.Parallel()
	r, v := cameraRenderer()
	r.smooth.Prefs.WorldOut = true
	if r.applyCamera(v, camInput{wheel: -1, wx: 1, wy: 1}) {
		t.Fatal("zoomed in at zoom 1: no world")
	}
	r.cam.Zoom = r.MinZoomFor(v)
	r.smooth.Zooming = false
	if r.applyCamera(v, camInput{wheel: 1}) {
		t.Fatal("zooming in never opens the world")
	}
	if !r.applyCamera(v, camInput{wheel: -1}) {
		t.Fatal("zooming out at the floor opens the world")
	}
	r.smooth.Prefs.WorldOut = false
	if r.applyCamera(v, camInput{zoom: -1}) {
		t.Fatal("option off")
	}
}

func TestShakeFromBlastsAndLightning(t *testing.T) {
	t.Parallel()
	r, _ := cameraRenderer()
	r.smooth.Prefs.Shake = 1
	r.cueBus().Voice.View = camerapkg.Viewport{X1: 800, Y1: 600}
	blast := func(tick uint64, x int) game.WorldView {
		return game.WorldView{Bursts: []game.BurstView{{X: x, Y: 9, Tick: tick, Blast: "bomb", Radius: 20}}}
	}
	r.watchThreats(blast(5, 10)) // present at the start
	if r.smooth.Fx.Active() {
		t.Fatal("old blasts do not shake")
	}
	r.watchThreats(blast(9, 10))
	if !r.smooth.Fx.Active() {
		t.Fatal("a new blast shakes")
	}
	r.smooth.Fx.Clear()
	r.watchThreats(blast(12, 60)) // far from the view
	if r.smooth.Fx.Active() {
		t.Fatal("a far blast does not")
	}
	r.smooth.Prefs.Shake = 0
	r.watchThreats(blast(15, 10))
	r.smooth.Shake(10)
	if r.smooth.Fx.Active() {
		t.Fatal("0 % is no shake")
	}
}

func TestShakeIsPresentationOnly(t *testing.T) {
	t.Parallel()
	r, _ := cameraRenderer()
	r.smooth.Prefs.Shake = 1
	r.smooth.Shake(10)
	r.smooth.Fx.Step(0.02)
	before := r.cam
	fxBegin(r)
	if r.cam == before {
		t.Fatal("the frame is shaken")
	}
	fxEnd(r)
	if r.cam != before {
		t.Fatalf("the real camera is untouched: %+v -> %+v", before, r.cam)
	}
	r.reducedMotion = true
	fxStep(r, 0.016)
	if r.smooth.Fx.Active() {
		t.Fatal("reduced motion draws no shake")
	}
}

func TestCameraJumpGlides(t *testing.T) {
	t.Parallel()
	r, v := cameraRenderer()
	r.smooth.Prefs.Glide = true
	fxStep(r, 0.016)
	r.centerOnView(30, 30, v, v)
	fxStep(r, 0.016)
	if !r.smooth.Fx.Active() {
		t.Fatal("a jump glides when smooth jumps is on")
	}
	r2, v2 := cameraRenderer()
	fxStep(r2, 0.016)
	r2.centerOnView(30, 30, v2, v2)
	fxStep(r2, 0.016)
	if r2.smooth.Fx.Active() {
		t.Fatal("off: the jump is instant")
	}
}
