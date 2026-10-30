package ebiten

import (
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"testing"
)

func TestSaveFromPauseThenLoad(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	saved := map[string]Simulation{}
	a.cfg.Saves = SaveHooks{
		Save: func(sim Simulation, slot string) error { saved[slot] = sim; return nil },
		List: func() ([]ports.SaveSlot, error) {
			var out []ports.SaveSlot
			for name := range saved {
				out = append(out, ports.SaveSlot{Name: name, Path: name, Date: "Year 1", Colonists: 3})
			}
			return out, nil
		},
		Load: func(path string) (Simulation, string, error) { return &stubSim{w: 4, h: 4}, path, nil },
	}
	a.finishBoot()
	quickStart(t, a)
	slot := a.opt.SaveSlot
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.save")
	if saved[slot] == nil {
		t.Fatalf("pause > Save should write slot %q: %v", slot, saved)
	}
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.load")
	if a.nav.screen() != ports.ScreenLoad || len(a.opt.Slots) != 1 {
		t.Fatalf("load screen %s with %d slots", a.nav.screen(), len(a.opt.Slots))
	}
	before := a.simsStarted
	click(t, a, "load.slot.0")
	if a.nav.screen() != ports.ScreenColony || a.simsStarted != before+1 || a.opt.SaveSlot != slot {
		t.Fatalf("after load: %s, sims %d, slot %q", a.nav.screen(), a.simsStarted, a.opt.SaveSlot)
	}
}

func TestSaveWritesViewAndLoadResumesIt(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 40, h: 40})
	views := map[string]camera.View{}
	a.cfg.Saves = SaveHooks{
		Save: func(Simulation, string) error { return nil },
		List: func() ([]ports.SaveSlot, error) {
			return []ports.SaveSlot{{Name: "s", Path: "s", Date: "Year 1", Colonists: 3}}, nil
		},
		Load:     func(path string) (Simulation, string, error) { return &stubSim{w: 40, h: 40}, path, nil },
		SaveView: func(slot string, v camera.View) error { views[slot] = v; return nil },
		LoadView: func(path string) (camera.View, bool) { v, ok := views[path]; return v, ok },
	}
	a.finishBoot()
	quickStart(t, a)
	r := a.renderer
	vp := a.mapViewport()
	r.cam.Zoom, r.cam.Placed = 1, true
	r.cam.OffsetX, r.cam.OffsetY = 300, 250
	want := r.cam.View(vp)
	if err := (optionsSaver{a}).Save("s"); err != nil {
		t.Fatal(err)
	}
	if got := views["s"]; got != want {
		t.Fatalf("saved view %+v, want %+v", got, want)
	}
	if _, _, err := (optionsSaver{a}).Load("s"); err != nil {
		t.Fatal(err)
	}
	a.renderer.ClampCamera(a.mapViewport())
	got := a.renderer.cam.View(a.mapViewport())
	if math.Abs(got.CX-want.CX) > 1e-6 || math.Abs(got.CY-want.CY) > 1e-6 || got.Zoom != want.Zoom {
		t.Fatalf("restored %+v, want %+v", got, want)
	}
	// a save without a view centres as before
	delete(views, "s")
	if _, _, err := (optionsSaver{a}).Load("s"); err != nil || a.renderer.cam.Placed {
		t.Fatalf("no view: err %v, placed %v", err, a.renderer.cam.Placed)
	}
}
