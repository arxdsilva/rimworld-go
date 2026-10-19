package ebiten

import (
	"testing"
)

func TestSaveFromPauseThenLoad(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	saved := map[string]Simulation{}
	a.cfg.Saves = SaveHooks{
		Save: func(sim Simulation, slot string) error { saved[slot] = sim; return nil },
		List: func() ([]SaveSlot, error) {
			var out []SaveSlot
			for name := range saved {
				out = append(out, SaveSlot{Name: name, Path: name, Date: "Year 1", Colonists: 3})
			}
			return out, nil
		},
		Load: func(path string) (Simulation, string, error) { return &stubSim{w: 4, h: 4}, path, nil },
	}
	a.finishBoot()
	click(t, a, "menu.newgame")
	for i := 0; i < 4; i++ {
		click(t, a, "wizard.next")
	}
	click(t, a, "wizard.start")
	slot := a.saveSlot
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.save")
	if saved[slot] == nil {
		t.Fatalf("pause > Save should write slot %q: %v", slot, saved)
	}
	a.apply(intent{kind: intentPause})
	click(t, a, "pause.load")
	if a.nav.screen() != screenLoad || len(a.slots) != 1 {
		t.Fatalf("load screen %s with %d slots", a.nav.screen(), len(a.slots))
	}
	before := a.simsStarted
	click(t, a, "load.slot.0")
	if a.nav.screen() != screenColony || a.simsStarted != before+1 || a.saveSlot != slot {
		t.Fatalf("after load: %s, sims %d, slot %q", a.nav.screen(), a.simsStarted, a.saveSlot)
	}
}
