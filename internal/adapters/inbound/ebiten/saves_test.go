package ebiten

import (
	"math"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
)

func TestSaveFromPauseThenLoad(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	saved := map[string]Simulation{}
	a.cfg.Saves = SaveHooks{
		Save: func(sim Simulation, m ports.SaveMeta) error { saved[m.Name] = sim; return nil },
		List: func() ([]ports.SaveSlot, error) {
			var out []ports.SaveSlot
			for name := range saved {
				out = append(out, ports.SaveSlot{Name: name, Path: name, Date: "Year 1", Colonists: 3})
			}
			return out, nil
		},
		Load: func(path string) (Simulation, ports.SaveMeta, error) { return &stubSim{w: 4, h: 4}, ports.SaveMeta{Name: path}, nil },
	}
	a.finishBoot()
	quickStart(t, a)
	slot := a.opt.Saves.SaveSlot
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.save")
	if saved[slot] == nil {
		t.Fatalf("pause > Save should write slot %q: %v", slot, saved)
	}
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.load")
	if a.nav.screen() != ports.ScreenLoad || len(a.opt.Saves.Slots) != 1 {
		t.Fatalf("load screen %s with %d slots", a.nav.screen(), len(a.opt.Saves.Slots))
	}
	before := a.simsStarted
	click(t, a, "load.slot.0")
	if a.nav.screen() != ports.ScreenColony || a.simsStarted != before+1 || a.opt.Saves.SaveSlot != slot {
		t.Fatalf("after load: %s, sims %d, slot %q", a.nav.screen(), a.simsStarted, a.opt.Saves.SaveSlot)
	}
}

func TestSaveWritesViewAndLoadResumesIt(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 160, h: 160}) // wider than the window at 15-px cells (UIS-080)
	views := map[string]map[game.MapID]camera.View{}
	a.cfg.Saves = SaveHooks{
		Save: func(Simulation, ports.SaveMeta) error { return nil },
		List: func() ([]ports.SaveSlot, error) {
			return []ports.SaveSlot{{Name: "s", Path: "s", Date: "Year 1", Colonists: 3}}, nil
		},
		Load:     func(path string) (Simulation, ports.SaveMeta, error) { return &stubSim{w: 160, h: 160}, ports.SaveMeta{Name: path}, nil },
		SaveView: func(slot string, v map[game.MapID]camera.View) error { views[slot] = v; return nil },
		LoadView: func(path string) (map[game.MapID]camera.View, bool) { v, ok := views[path]; return v, ok },
	}
	a.finishBoot()
	quickStart(t, a)
	r := a.renderer
	vp := a.mapViewport()
	r.cam.Zoom, r.cam.Placed = 1, true
	r.cam.OffsetX, r.cam.OffsetY = 300, 250
	want := r.cam.View(vp)
	if err := (optionsSaver{a}).Save(ports.SaveMeta{Name: "s"}); err != nil {
		t.Fatal(err)
	}
	if got := views["s"][0]; got != want {
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

// The everyone-gone dialog's Load game pushes the Load screen through the
// shell (hud/ending.go): the list is read again, so a save written since
// the main menu read it is there, and Back still leaves the screen.
func TestPushedLoadScreenListsTheSavesNow(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	var slots []ports.SaveSlot
	a.cfg.Saves = SaveHooks{
		Save: func(Simulation, ports.SaveMeta) error { return nil },
		List: func() ([]ports.SaveSlot, error) { return slots, nil },
		Load: func(path string) (Simulation, ports.SaveMeta, error) { return &stubSim{w: 4, h: 4}, ports.SaveMeta{Name: path}, nil },
	}
	a.finishBoot()
	quickStart(t, a)
	a.opt.Saves.RefreshSlots() // the list as the menu read it: empty
	slots = []ports.SaveSlot{{Name: "late", Path: "late", Date: "Year 1", Colonists: 1}}
	a.shell.Do(ports.Push{Screen: ports.ScreenLoad})
	if a.nav.screen() != ports.ScreenLoad || len(a.opt.Saves.Slots) != 1 || a.opt.Saves.Slots[0].Name != "late" {
		t.Fatalf("on %s with %+v, want the Load screen listing the late save", a.nav.screen(), a.opt.Saves.Slots)
	}
	click(t, a, "load.back") // the book leaves load.back to the screen intent
	if a.nav.screen() != ports.ScreenColony {
		t.Fatalf("Back from Load: on %s", a.nav.screen())
	}
}

// SAV-023: the Options "Saves folder" row opens the store's folder
// (-save-dir's when given), which compose puts in SaveHooks.Dir.
func TestSavesFolderRowOpensTheSaveDir(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.cfg.Saves.Dir = "/data/probe-saves"
	if got := (optionsHost{a}).Env().SavesDir; got != "/data/probe-saves" {
		t.Fatalf("SavesDir %q, want the hooks' Dir", got)
	}
}
