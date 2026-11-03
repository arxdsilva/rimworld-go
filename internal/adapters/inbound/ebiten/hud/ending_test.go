package hud

import "testing"

// The everyone-gone screen (MAP-14) opens once when AX.EveryoneGone turns
// true, does not reopen while it stays true after Keep watching (no flap),
// and opens again when everyone is gone a second time.
func TestGoneScreenOpensOnceAndRearms(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	r := c.Right()
	f.w.AX.EveryoneGone = true
	r.declareEnding()
	if f.ui.ModalOpen() {
		t.Fatal("the screen opened on a session that never had a colonist")
	}
	f.w.AX.EveryoneGone = false
	r.declareEnding()
	if f.ui.ModalOpen() {
		t.Fatal("the screen opened with colonists alive")
	}
	f.w.AX.EveryoneGone = true
	r.declareEnding()
	if len(f.ui.Modals) != 1 || f.ui.Modals[0].ID != GoneModalID {
		t.Fatalf("modals %+v, want the everyone-gone screen", f.ui.Modals)
	}
	if h := f.ui.Modals[0].H; h <= 0 || h > 400 {
		t.Fatalf("modal height %v does not fit its text", h)
	}
	f.ui.Modals = nil // Keep watching
	for i := 0; i < 100; i++ {
		r.declareEnding()
	}
	if f.ui.ModalOpen() {
		t.Fatal("the screen reopened while everyone stays gone")
	}
	f.w.AX.EveryoneGone = false
	r.declareEnding()
	f.w.AX.EveryoneGone = true
	r.declareEnding()
	if len(f.ui.Modals) != 1 {
		t.Fatalf("%d modals after everyone was gone a second time, want 1", len(f.ui.Modals))
	}
}
