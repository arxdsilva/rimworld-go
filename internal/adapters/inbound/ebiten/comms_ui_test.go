package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func testCommsView() game.CommsView {
	return game.CommsView{
		OperatorName: "Ada", BeaconSilver: 120,
		Factions: []game.CommsFactionView{
			{ID: 3, Name: "Ridge", Goodwill: 20, Relation: "friendly",
				Gift:    game.CommsActionView{Allowed: true, Cost: 25},
				Caravan: game.CommsActionView{Allowed: true, Cost: 12},
				Aid:     game.CommsActionView{Reason: "cooling down: 2 days", Cost: 30}},
			{ID: 4, Name: "Pit", Goodwill: -80, Relation: "hostile"},
		},
		Orbital: game.OrbitalView{Present: true, Ship: "Tern", Kind: "curio merchant", HoursLeft: 30, CallOK: true},
	}
}

func TestCommsDisabledReasons(t *testing.T) {
	t.Parallel()
	s := &dealSim{cv: testCommsView(), cok: true}
	a, f := dealApp(t, s)
	f.Frame(t, a)
	w, ok := a.uiSt().Find("comms.3.aid")
	if !ok || !w.Disabled || w.Reason != "cooling down: 2 days" {
		t.Fatalf("aid = %+v ok=%v", w, ok)
	}
	if w, _ := a.uiSt().Find("comms.3.caravan"); w.Disabled || w.Label != "Request caravan · 12 goodwill" {
		t.Fatalf("caravan = %+v", w)
	}
	if _, ok := a.uiSt().Find("comms.4.gift"); ok {
		t.Fatal("a hostile faction shows no actions")
	}
	s.cv.Orbital = game.OrbitalView{}
	f.Frame(t, a)
	if _, ok := a.uiSt().Find("comms.call"); ok {
		t.Fatal("no ship, no call button")
	}
	s.cv.Orbital = game.OrbitalView{Present: true, CallReason: "needs a trade beacon"}
	f.Frame(t, a)
	if w, _ := a.uiSt().Find("comms.call"); !w.Disabled || w.Reason != "needs a trade beacon" {
		t.Fatalf("call = %+v", w)
	}
}

func TestCommsGiftEmitsCommand(t *testing.T) {
	t.Parallel()
	s := &dealSim{cv: testCommsView(), cok: true}
	a, f := dealApp(t, s)
	f.Frame(t, a)
	clickID(t, a, f, "comms.3.giftmore")
	clickID(t, a, f, "comms.3.gift")
	var got *command.Command
	for i := range s.cmds {
		if s.cmds[i].Kind == command.CommsGift {
			got = &s.cmds[i]
		}
	}
	if got == nil || got.Index != 3 || got.Count != 50 {
		t.Fatalf("cmds = %+v", s.cmds)
	}
	d := &commsDraft{view: game.CommsView{BeaconSilver: 45}, gift: map[int]int{}}
	d.stepGift(1, 10)
	d.stepGift(1, 10)
	if d.giftOf(1) != 45 {
		t.Fatalf("gift clamps to the silver in reach: %d", d.giftOf(1))
	}
}

func TestCommsCallShipOpensTrade(t *testing.T) {
	t.Parallel()
	s := &dealSim{cv: testCommsView(), cok: true}
	a, f := dealApp(t, s)
	f.Frame(t, a)
	clickID(t, a, f, "comms.call")
	found := false
	for _, c := range s.cmds {
		found = found || c.Kind == command.CommsCallTrader
	}
	if !found {
		t.Fatalf("cmds = %+v", s.cmds)
	}
	clickID(t, a, f, "modal."+commsModalID+".close")
	last := s.cmds[len(s.cmds)-1]
	if last.Kind != command.CloseComms {
		t.Fatalf("close emits %v", last.Kind)
	}
}
