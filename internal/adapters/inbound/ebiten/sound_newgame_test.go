package ebiten

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// Sounds of the new-game flow (NG-1 §7.4): slider ticks, wizard page
// swishes, the landing pods and the water beds.

func TestSliderPlaysTick(t *testing.T) {
	t.Parallel()
	a, f, u := newKit(t)
	rec := &soundRecorder{}
	a.cfg.Sound = rec
	gap := func() { a.snd.frame += cueGap } // past the one-sound rate limit

	// A discrete slider ticks on every step it moves.
	stop := 0.0
	dr := rect{100, 200, 300, 40}
	dsp := sliderSpec{Stops: []string{"Low", "Normal", "High"}}
	setHook(u, func(a *App) { a.uiSlider("d", dr, &stop, dsp) })
	dt := sliderTrack(dr)
	f.press(t, a, int(dt.X+dt.W*0.5), 205)
	f.letGo(t, a, int(dt.X+dt.W*0.5), 205)
	if stop != 1 || len(rec.played("ui.slider_tick")) != 1 {
		t.Fatalf("one step, one tick: stop=%v plays=%+v", stop, rec.plays)
	}
	gap()
	f.press(t, a, int(dt.X+dt.W*0.5), 205) // the same stop again: silent
	f.letGo(t, a, int(dt.X+dt.W*0.5), 205)
	if len(rec.played("ui.slider_tick")) != 1 {
		t.Fatalf("no move, no tick: %+v", rec.plays)
	}
	gap()
	f.key(t, a, "ArrowRight", 0)
	if stop != 2 || len(rec.played("ui.slider_tick")) != 2 {
		t.Fatalf("a key step ticks: stop=%v plays=%+v", stop, rec.plays)
	}

	// A continuous slider ticks once per step crossed, not per frame.
	val := 50.0
	r := rect{100, 100, 300, 30}
	sp := sliderSpec{Min: 0, Max: 100, Step: 10}
	setHook(u, func(a *App) { a.uiSlider("s", r, &val, sp) })
	track := sliderTrack(r)
	at := func(frac float32) int { return int(track.X + track.W*frac) }
	rec.plays = nil
	gap()
	f.press(t, a, at(0.5), 110)
	f.letGo(t, a, at(0.5), 110)
	gap()
	f.press(t, a, at(0.52), 110) // 52 is still inside the 50..60 step
	f.letGo(t, a, at(0.52), 110)
	if n := len(rec.played("ui.slider_tick")); n != 0 {
		t.Fatalf("a nudge inside one step is silent, got %d ticks", n)
	}
	gap()
	f.press(t, a, at(0.75), 110) // across two steps: one tick for the move
	f.letGo(t, a, at(0.75), 110)
	if n := len(rec.played("ui.slider_tick")); n != 1 {
		t.Fatalf("crossing steps ticks once per move, got %d", n)
	}
}

func TestWizardPagesPlay(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	rec := &soundRecorder{}
	a.cfg.Sound = rec
	click(t, a, "menu.newgame")
	f.frame(t, a)
	press(t, a, f, "scen.row.ember_clan")
	f.frame(t, a)
	if n := len(rec.played("ui.page")); n != 0 {
		t.Fatalf("choosing a scenario is not a page turn: %d", n)
	}
	click(t, a, "wizard.next")
	if a.nav.screen() != screenStoryteller || len(rec.played("ui.page")) != 1 {
		t.Fatalf("Next turns the page: screen=%s plays=%+v", a.nav.screen(), rec.plays)
	}
	click(t, a, "wizard.back")
	if len(rec.played("ui.page")) != 2 {
		t.Fatalf("Back turns the page: %+v", rec.plays)
	}
}

func TestPodCuesFollowArrivals(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 200, h: 200}
	a, rec := soundApp(sim)
	r := a.renderer
	cx, cy := centreTile(a)
	px, py := int(cx), int(cy)
	const land = 1000
	world := func(landed bool) game.WorldView {
		return game.WorldView{Arrivals: []game.ArrivalView{{X: px, Y: py, Tick: land, Landed: landed}}}
	}
	ids := func() []string {
		var out []string
		for _, c := range r.takeCues() {
			out = append(out, c.id)
		}
		return out
	}
	at := func(tick uint64, landed bool) []string {
		sim.tick = clock.Tick(tick)
		r.listenArrivals(world(landed))
		return ids()
	}

	if got := at(0, false); len(got) != 0 {
		t.Fatalf("a pod far from landing is quiet: %v", got)
	}
	if got := at(land-podFallLead-1, false); len(got) != 0 {
		t.Fatalf("not yet falling: %v", got)
	}
	if got := at(land-podFallLead, false); !slices.Equal(got, []string{"sfx.pod_fall"}) {
		t.Fatalf("the descent starts: %v", got)
	}
	if got := at(land-10, false); len(got) != 0 {
		t.Fatalf("the fall is heard once: %v", got)
	}
	if got := at(land, true); !slices.Equal(got, []string{"sfx.pod_land", "sfx.pod_open"}) {
		t.Fatalf("landing, then the hatch: %v", got)
	}
	if got := at(land+500, true); len(got) != 0 {
		t.Fatalf("each sound once: %v", got)
	}

	// A pod first seen after it landed (a loaded game) stays silent.
	r.memo.pods = nil
	if got := at(land+200, true); len(got) != 0 {
		t.Fatalf("an old landing is silent: %v", got)
	}

	// The cues reach the speakers where the pod is, and only when on screen.
	r.memo.pods = nil
	sim.tick = clock.Tick(land - 30)
	r.listenArrivals(world(false))
	a.colonyCues()
	if n := len(rec.played("sfx.pod_fall")); n != 1 {
		t.Fatalf("the on-screen pod's fall is played: %+v", rec.plays)
	}
	r.memo.pods = nil
	far := game.WorldView{Arrivals: []game.ArrivalView{{X: px + 900, Y: py, Tick: land}}}
	r.listenArrivals(far)
	rec.plays = nil
	a.colonyCues()
	if len(rec.plays) != 0 {
		t.Fatalf("an off-screen pod is out of earshot: %+v", rec.plays)
	}
}

func TestWaterAmbienceFollowsScreen(t *testing.T) {
	t.Parallel()
	const side = 200
	var wet uint8
	for i, n := range terrainNames {
		if n == "shallow_water" {
			wet = uint8(i)
		}
	}
	a, rec := soundApp(&stubSim{w: side, h: side})
	terr := make([]uint8, side*side) // soil everywhere...
	for y := 0; y < side; y++ {
		for x := 0; x < 100; x++ {
			terr[y*side+x] = wet // ...except water west of x = 100
		}
	}
	r := a.renderer
	r.heard = game.WorldView{Hour: 12, Season: "summer", OutdoorTemp: 200, Terrain: terr, TerrainW: side}
	look := func(tileX float64) {
		v := colonyViewport(a.sw, a.sh)
		r.cam.zoom = 1
		r.cam.offsetX = tileX*float64(TileSize) - (v.x0+v.x1)/2
		r.cam.offsetY = 100*float64(TileSize) - (v.y0+v.y1)/2
		a.snd.frame += cueGap
		a.soundFrame()
	}
	vol := func(key string) float64 { return rec.loops[key].vol }

	look(160) // inland: dry land only
	if _, ok := rec.loops["amb.water_river"]; ok {
		t.Fatalf("no river bed on dry land: %+v", rec.loops)
	}
	if _, ok := rec.loops["amb.water_shore"]; ok {
		t.Fatalf("no shore bed on dry land: %+v", rec.loops)
	}
	look(40) // out on the lake: all water, no shoreline in view
	if vol("amb.water_river") < 0.9 {
		t.Fatalf("open water fills the river bed: %+v", rec.loops)
	}
	if _, ok := rec.loops["amb.water_shore"]; ok {
		t.Fatalf("no shore bed away from the coast: %+v", rec.loops)
	}
	look(100) // on the coast: half water, a shoreline down the middle
	river, shore := vol("amb.water_river"), vol("amb.water_shore")
	if river <= 0 || shore <= 0 {
		t.Fatalf("the coast has both beds: %+v", rec.loops)
	}
	look(125) // the water drifts to the view's edge: quieter
	if vol("amb.water_river") >= river {
		t.Fatalf("less water in view is quieter: %v then %v", river, vol("amb.water_river"))
	}
	look(160) // and gone when the camera leaves it: the bed is told to fade out
	if got, ok := rec.loops["amb.water_river"]; !ok || got.vol != 0 {
		t.Fatalf("the river bed fades out with the water: %+v", rec.loops)
	}
}
