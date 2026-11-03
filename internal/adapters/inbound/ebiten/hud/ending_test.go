package hud

import (
	"slices"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

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

// dispatchShell is the fake with the shell's intents, which the ending's
// buttons need.
type dispatchShell struct{ *fakeShell }

func (dispatchShell) Do(ports.Intent) {}

// MAP-22: a launch with everyone aboard is both endings: the ship's screen
// opens first, it offers New wanderers when the game has them, and the
// everyone-gone screen does not come on top. It re-arms when someone comes.
func TestShipEndingOpensFirstWhenEveryoneLeft(t *testing.T) {
	t.Parallel()
	_, f := newFake()
	r := New(dispatchShell{f}).Right()
	r.declareEnding() // a session that had colonists
	f.w.AX.ShipLaunched, f.w.AX.EveryoneGone, f.w.AX.WanderersMax, f.w.AX.WanderersLeft = true, true, 5, 7
	f.w.AX.Escaped = []string{"Ada"}
	r.declareEnding()
	if len(f.ui.Modals) != 1 || f.ui.Modals[0].ID != EndingModalID {
		t.Fatalf("modals %+v, want the ship ending", f.ui.Modals)
	}
	if !slices.ContainsFunc(f.ui.Modals[0].Buttons, func(b uikit.ModalButton) bool { return b.ID == GoneWanderersID }) {
		t.Fatalf("buttons %+v lack New wanderers", f.ui.Modals[0].Buttons)
	}
	f.ui.Modals = nil
	for range 100 {
		r.declareEnding()
	}
	if f.ui.ModalOpen() {
		t.Fatal("a second screen came for the same ending")
	}
	f.w.AX.EveryoneGone = false
	r.declareEnding()
	f.w.AX.EveryoneGone = true
	r.declareEnding()
	if len(f.ui.Modals) != 1 || f.ui.Modals[0].ID != GoneModalID {
		t.Fatalf("modals %+v, want the everyone-gone screen for the second time", f.ui.Modals)
	}
}

// With someone left behind only the ship's screen opens, without the button.
func TestShipEndingWithSomeoneLeftBehind(t *testing.T) {
	t.Parallel()
	_, f := newFake()
	r := New(dispatchShell{f}).Right()
	f.w.AX.ShipLaunched, f.w.AX.WanderersMax, f.w.AX.WanderersLeft = true, 5, 7
	f.w.AX.Escaped = []string{"Ada"}
	r.declareEnding()
	if len(f.ui.Modals) != 1 || f.ui.Modals[0].ID != EndingModalID {
		t.Fatalf("modals %+v, want the ship ending", f.ui.Modals)
	}
	if slices.ContainsFunc(f.ui.Modals[0].Buttons, func(b uikit.ModalButton) bool { return b.ID == GoneWanderersID }) {
		t.Fatal("New wanderers with a colonist still here")
	}
}
