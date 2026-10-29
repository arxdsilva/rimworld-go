package ebiten

import (
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
