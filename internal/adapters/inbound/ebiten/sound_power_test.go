package ebiten

import (
	"strconv"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 electricity sounds: machine loops from the building views, and
// one-shots from what changed between two frames.

var (
	sndKindWoodGen = construction.WoodGenerator.String()
	sndKindGeoGen  = construction.GeothermalGenerator.String()
	sndKindWind    = construction.WindTurbine.String()
	sndKindHeater  = construction.Heater.String()
	sndKindCooler  = construction.Cooler.String()
	sndKindSwitch  = construction.PowerSwitch.String()
	sndKindConduit = construction.PowerConduit.String()
)

// sndMachineAt is a 1x1 building view standing on tile (x, y).
func sndMachineAt(id entity.EntityID, kind string, x, y int) game.BuildingView {
	return game.BuildingView{ID: id, Kind: kind, X: x, Y: y, W: 1, H: 1}
}

func TestMachineLoopsPlayWhileTheMachineRuns(t *testing.T) {
	t.Parallel()
	a, _ := soundApp(&stubSim{w: 200, h: 200})
	cx, cy := centreTile(a)
	x, y := int(cx), int(cy)
	running := func(id entity.EntityID, kind string, f func(*game.BuildingView)) game.BuildingView {
		b := sndMachineAt(id, kind, x, y)
		f(&b)
		return b
	}
	cases := []struct {
		name  string
		b     game.BuildingView
		sound string // "" for silence
	}{
		{"wood generator burning and producing", running(1, sndKindWoodGen, func(b *game.BuildingView) { b.Burning, b.PowerOut = true, 800 }), "amb.generator_wood"},
		{"wood generator out of fuel", running(1, sndKindWoodGen, func(b *game.BuildingView) { b.PowerOut = 0 }), ""},
		{"wood generator switched off but still lit", running(1, sndKindWoodGen, func(b *game.BuildingView) { b.Burning = true }), ""},
		{"wood generator producing but not burning", running(1, sndKindWoodGen, func(b *game.BuildingView) { b.PowerOut = 800 }), ""},
		{"geothermal generator producing", running(2, sndKindGeoGen, func(b *game.BuildingView) { b.PowerOut = 1800 }), "amb.generator_geo"},
		{"geothermal generator during a flare", running(2, sndKindGeoGen, func(b *game.BuildingView) {}), ""},
		{"wind turbine turning", running(3, sndKindWind, func(b *game.BuildingView) { b.PowerOut = 600 }), "amb.wind_turbine"},
		{"wind turbine in calm air", running(3, sndKindWind, func(b *game.BuildingView) {}), ""},
		{"heater working", running(4, sndKindHeater, func(b *game.BuildingView) { b.Powered, b.PowerUse = true, 180 }), "amb.heater"},
		{"heater idle at target", running(4, sndKindHeater, func(b *game.BuildingView) { b.Powered, b.PowerUse = true, 20 }), ""},
		{"heater without power", running(4, sndKindHeater, func(b *game.BuildingView) {}), ""},
		{"cooler working", running(5, sndKindCooler, func(b *game.BuildingView) { b.Powered, b.PowerUse = true, 220 }), "amb.cooler"},
		{"cooler idle at target", running(5, sndKindCooler, func(b *game.BuildingView) { b.Powered, b.PowerUse = true, 20 }), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a.renderer.cam.Zoom = 1.5
			a.renderer.heard = game.WorldView{Hour: 12, Buildings: []game.BuildingView{tc.b}}
			want := map[string]loopWant{}
			a.colonyAmbience(want)
			var got []string
			for key, l := range want {
				if len(key) > 6 && key[:6] == "power:" {
					got = append(got, l.ID)
				}
			}
			switch {
			case tc.sound == "" && len(got) != 0:
				t.Fatalf("silent expected, heard %v", got)
			case tc.sound != "" && (len(got) != 1 || got[0] != tc.sound):
				t.Fatalf("want %s, heard %v", tc.sound, got)
			}
			if _, crackle := want["fire:"+strconv.FormatUint(uint64(tc.b.ID), 10)]; crackle && tc.b.Kind == sndKindWoodGen {
				t.Fatal("a generator chugs; it does not crackle like a campfire")
			}
		})
	}
}

func TestMachineLoopsStartAndStopWithTheState(t *testing.T) {
	t.Parallel()
	a, rec := soundApp(&stubSim{w: 200, h: 200})
	r := a.renderer
	cx, cy := centreTile(a)
	b := sndMachineAt(7, sndKindHeater, int(cx), int(cy))
	b.Powered, b.PowerUse = true, 180
	r.cam.Zoom = 1.5
	r.heard = game.WorldView{Hour: 12, Buildings: []game.BuildingView{b}}
	want := map[string]loopWant{}
	a.colonyAmbience(want)
	a.setLoops(want)
	l, ok := rec.loops["power:7"]
	if !ok || l.id != "amb.heater" || l.vol <= 0 {
		t.Fatalf("the heater hums while it works: %+v ok=%v", l, ok)
	}
	b.PowerUse = 20 // the room reached its target
	r.heard = game.WorldView{Hour: 12, Buildings: []game.BuildingView{b}}
	want = map[string]loopWant{}
	a.colonyAmbience(want)
	a.setLoops(want)
	if l := rec.loops["power:7"]; l.vol != 0 {
		t.Fatalf("the heater hum fades out when it idles: %+v", l)
	}
}

func TestMachineLoopsAreSpatialAndCapped(t *testing.T) {
	t.Parallel()
	a, _ := soundApp(&stubSim{w: 200, h: 200})
	r := a.renderer
	cx, cy := centreTile(a)
	x, y := int(cx), int(cy)
	var bs []game.BuildingView
	for i := 0; i < 5; i++ { // five generators in a row, the first dead centre
		b := sndMachineAt(entity.EntityID(10+i), sndKindGeoGen, x+i, y)
		b.PowerOut = 1800
		bs = append(bs, b)
	}
	far := sndMachineAt(99, sndKindGeoGen, x+500, y) // out of earshot
	far.PowerOut = 1800
	bs = append(bs, far)
	r.cam.Zoom = 1.5
	r.heard = game.WorldView{Hour: 12, Buildings: bs}
	want := map[string]loopWant{}
	a.colonyAmbience(want)
	n := 0
	for key, l := range want {
		if len(key) > 6 && key[:6] == "power:" {
			n++
			if key == "power:99" {
				t.Fatal("a machine far off screen is not heard")
			}
			if l.Vol <= 0 || l.Vol > 1 {
				t.Fatalf("%s volume %v", key, l.Vol)
			}
		}
	}
	if n != maxMachineLoops {
		t.Fatalf("%d generator loops at once, want the nearest %d", n, maxMachineLoops)
	}
	if _, ok := want["power:10"]; !ok {
		t.Fatal("the generator in the middle of the view is the one heard")
	}
	if right := want["power:11"]; right.Pan <= want["power:10"].Pan {
		t.Fatalf("a generator to the right pans right of the centre one: %+v vs %+v", right, want["power:10"])
	}
	r.cam.Zoom = r.MinZoomFor(a.mapViewport())
	want = map[string]loopWant{}
	a.colonyAmbience(want)
	for key := range want {
		if len(key) > 6 && key[:6] == "power:" {
			t.Fatalf("%s: machines are silent fully zoomed out", key)
		}
	}
}

// A switch clicks when its state flips and not otherwise.
func TestSwitchClicksWhenPowerOnFlips(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 20, h: 20})
	sw := func(on bool) game.WorldView {
		b := sndMachineAt(5, sndKindSwitch, 4, 6)
		b.PowerOn, b.PowerWantOn = on, on
		return game.WorldView{Buildings: []game.BuildingView{b}}
	}
	r.listenWorld(sw(true), nil)
	r.listenWorld(sw(true), nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("an unchanged switch cued %+v", c)
	}
	r.listenWorld(sw(false), nil)
	c := r.takeCues()
	if len(c) != 1 || c[0].ID != "sfx.switch_click" || c[0].X != 4.5 || c[0].Y != 6.5 || c[0].Global {
		t.Fatalf("a switch thrown off clicks where it stands, got %+v", c)
	}
	r.listenWorld(sw(false), nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("the click is heard once, got %+v", c)
	}
	r.listenWorld(sw(true), nil)
	if c := r.takeCues(); len(c) != 1 || c[0].ID != "sfx.switch_click" {
		t.Fatalf("a switch thrown on clicks too, got %+v", c)
	}
}

func TestSwitchClickIgnoresEverythingElse(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 20, h: 20})
	view := func(swOn, wantOn bool, heaterOn bool, extra bool) game.WorldView {
		s := sndMachineAt(5, sndKindSwitch, 4, 6)
		s.PowerOn, s.PowerWantOn = swOn, wantOn
		h := sndMachineAt(6, sndKindHeater, 8, 8)
		h.PowerOn, h.PowerWantOn = heaterOn, heaterOn
		w := game.WorldView{Buildings: []game.BuildingView{s, h}}
		if extra {
			n := sndMachineAt(7, sndKindSwitch, 1, 1)
			w.Buildings = append(w.Buildings, n)
		}
		return w
	}
	r.listenWorld(view(true, true, true, false), nil)
	// a colonist is on the way to flick it: wanted flips, the switch does not
	r.listenWorld(view(true, false, true, false), nil)
	// a heater switched off at its own toggle is not a switch
	r.listenWorld(view(true, false, false, false), nil)
	// a new switch built is not a click
	r.listenWorld(view(true, false, false, true), nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("no switch changed state, got %+v", c)
	}
	// a switch deconstructed is not a click either
	r.listenWorld(game.WorldView{}, nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("a removed switch cued %+v", c)
	}
}

// sndLetterAt is a letter titled title at tile (x, y), id id.
func sndLetterAt(id uint64, title string, x, y int) game.LetterView {
	return game.LetterView{ID: id, Title: title, Kind: "threat", X: x, Y: y}
}

func TestShortCircuitSoundsOnItsLetterNotOnDeconstruction(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 20, h: 20}
	r := New(sim)
	grid := func(conduits ...int) game.WorldView {
		var w game.WorldView
		for _, x := range conduits {
			w.Buildings = append(w.Buildings, sndMachineAt(entity.EntityID(100+x), sndKindConduit, x, 5))
		}
		return w
	}
	r.listenWorld(grid(3, 4, 5), nil)
	// the player deconstructs a conduit: quiet
	r.listenWorld(grid(3, 5), nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("a deconstructed conduit cued %+v", c)
	}
	// a conduit burns out and its letter arrives with it
	w := grid(3)
	w.Letters = []game.LetterView{sndLetterAt(1, "Short circuit", 5, 5)}
	r.listenWorld(w, nil)
	c := r.takeCues()
	if len(c) != 1 || c[0].ID != "sfx.short_circuit" || c[0].X != 5.5 || c[0].Y != 5.5 || c[0].Global {
		t.Fatalf("a short circuit zaps where the conduit was, got %+v", c)
	}
	// the letter stays in the stack: it is heard once
	r.listenWorld(w, nil)
	r.listenWorld(w, nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("the same letter cued again: %+v", c)
	}
	// other letters are not short circuits
	w.Letters = append(w.Letters, sndLetterAt(2, "Solar flare", -1, -1), sndLetterAt(3, "Visitors", 2, 2))
	r.listenWorld(w, nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("another letter cued %+v", c)
	}
	// a second circuit later is heard again
	w.Letters = append(w.Letters, sndLetterAt(4, "Short circuit", 3, 5))
	r.listenWorld(w, nil)
	if c := r.takeCues(); len(c) != 1 || c[0].X != 3.5 {
		t.Fatalf("the next short circuit is heard, got %+v", c)
	}
}

func TestShortCircuitLettersAlreadyThereAreSilent(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 20, h: 20, tick: 100000}
	r := New(sim)
	// a loaded game opens with an old short-circuit letter in the stack
	old := game.WorldView{Letters: []game.LetterView{sndLetterAt(7, "Short circuit", 5, 5)}}
	r.listenWorld(old, nil)
	r.listenWorld(old, nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("a letter from before the game was loaded cued %+v", c)
	}
	// a stale letter showing up late (an old save's stack) is silent too
	stale := old
	stale.Letters = append(stale.Letters, game.LetterView{ID: 8, Title: "Short circuit", X: 2, Y: 2, Tick: 10})
	r.listenWorld(stale, nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("a letter from long ago cued %+v", c)
	}
	// a letter without a place has nowhere to sound
	nowhere := old
	nowhere.Letters = append(nowhere.Letters, game.LetterView{ID: 9, Title: "Short circuit", X: -1, Y: -1, Tick: uint64(sim.tick)})
	r.listenWorld(nowhere, nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("a letter without a place cued %+v", c)
	}
}

func TestSolarFlareBuzzesOncePerFlare(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 20, h: 20})
	flare := func(on bool) game.WorldView { return game.WorldView{SolarFlare: on} }
	r.listenWorld(flare(false), nil)
	r.listenWorld(flare(false), nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("calm sun cued %+v", c)
	}
	r.listenWorld(flare(true), nil)
	c := r.takeCues()
	if len(c) != 1 || c[0].ID != "ui.solar_flare" || !c[0].Global {
		t.Fatalf("the flare starts with one global buzz, got %+v", c)
	}
	for range 5 {
		r.listenWorld(flare(true), nil)
	}
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("the buzz repeated during the flare: %+v", c)
	}
	r.listenWorld(flare(false), nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("the end of a flare is silent, got %+v", c)
	}
	r.listenWorld(flare(true), nil)
	if c := r.takeCues(); len(c) != 1 {
		t.Fatalf("the next flare buzzes again, got %+v", c)
	}
}

func TestSolarFlareAlreadyOnWhenLoadedIsSilent(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 20, h: 20})
	r.listenWorld(game.WorldView{SolarFlare: true}, nil)
	r.listenWorld(game.WorldView{SolarFlare: true}, nil)
	if c := r.takeCues(); len(c) != 0 {
		t.Fatalf("a flare already under way when the game loaded cued %+v", c)
	}
}

// The cues reach the speakers: a global flare buzz plays unattenuated and
// a placed click is panned by where the switch is.
func TestPowerCuesReachTheSoundBus(t *testing.T) {
	t.Parallel()
	a, rec := soundApp(&stubSim{w: 200, h: 200, tick: clock.Tick(500)})
	r := a.renderer
	cx, cy := centreTile(a)
	r.cam.Zoom = 1.5
	r.cue("sfx.switch_click", cx+6, cy, 1)
	r.globalCue("ui.solar_flare")
	a.colonyCues()
	click, buzz := rec.played("sfx.switch_click"), rec.played("ui.solar_flare")
	if len(click) != 1 || click[0].pan <= 0.05 {
		t.Fatalf("a click right of centre pans right: %+v", click)
	}
	if len(buzz) != 1 || buzz[0].vol != 1 || buzz[0].pan != 0 {
		t.Fatalf("the flare buzz is global: %+v", buzz)
	}
}
