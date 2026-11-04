package dialogs

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/input"
)

func TestCommsGiftClampsToSilverInReach(t *testing.T) {
	t.Parallel()
	d := &commsDraft{view: game.CommsView{BeaconSilver: 45}, gift: map[int]int{}}
	d.stepGift(1, 10)
	d.stepGift(1, 10)
	if d.giftOf(1) != 45 {
		t.Fatalf("gift clamps to the silver in reach: %d", d.giftOf(1))
	}
}

func TestDialogsDeclareTheirWidgets(t *testing.T) {
	t.Parallel()
	s, dl := newFake()
	tv, cv := testTradeView(), game.CommsView{
		OperatorName: "Ada", BeaconSilver: 120,
		Factions: []game.CommsFactionView{
			{ID: 3, Name: "Ridge", Goodwill: 20, Relation: "friendly", Gift: game.CommsActionView{Allowed: true}},
			{ID: 4, Name: "Pit", Goodwill: -80, Relation: "hostile"},
		},
		Orbital: game.OrbitalView{Present: true, Ship: "Tern", CallOK: true},
	}
	s.tv = &tv
	s.frame(dl)
	s.frame(dl)
	if _, ok := s.kit.Find("trade." + item.Wood.String() + ".take.1"); !ok {
		t.Error("no trade widget")
	}
	s.tv, s.cv = nil, &cv
	s.frame(dl)
	s.frame(dl)
	for _, id := range []string{"comms.3.gift", "comms.call"} {
		if _, ok := s.kit.Find(id); !ok {
			t.Errorf("no widget %q", id)
		}
	}
	if _, ok := s.kit.Find("comms.4.gift"); ok {
		t.Error("a hostile faction shows no actions")
	}
	s.sites = []game.WorldSiteView{{Name: "Ridge camp", Faction: "Ridge", Relation: "friendly", Tile: 7, Visitable: true}}
	dl.OpenFormCaravan(func() int { return -1 })
	s.frame(dl)
	s.frame(dl)
	if _, ok := s.kit.Find("form.dest.7"); !ok {
		t.Error("no destination widget for the site")
	}
}

// CTL-018: the gift's + and - take Ctrl and Shift too, inside 1..silver.
func TestCommsGiftStepsFollowCtrlAndShift(t *testing.T) {
	t.Parallel()
	s, dl := newFake()
	cv := game.CommsView{OperatorName: "Ada", BeaconSilver: 5000,
		Factions: []game.CommsFactionView{{ID: 3, Name: "Ridge", Goodwill: 20, Relation: "friendly", Gift: game.CommsActionView{Allowed: true}}}}
	s.cv = &cv
	s.frame(dl)
	s.frame(dl)
	press := func(id string, mods input.Mod) {
		s.f.Raw.Mods = mods
		if !s.kit.Click(id) {
			t.Fatalf("no widget %s", id)
		}
		s.frame(dl)
	}
	press("comms.3.giftmore", 0)
	if got := dl.comms.giftOf(3); got != CommsGiftStart+10 {
		t.Fatalf("plain: %d", got)
	}
	press("comms.3.giftmore", input.ModCtrl)
	if got := dl.comms.giftOf(3); got != CommsGiftStart+110 {
		t.Fatalf("ctrl: %d", got)
	}
	press("comms.3.giftmore", input.ModCtrl|input.ModShift)
	press("comms.3.giftmore", input.ModCtrl|input.ModShift)
	press("comms.3.giftmore", input.ModCtrl|input.ModShift)
	press("comms.3.giftmore", input.ModCtrl|input.ModShift)
	press("comms.3.giftmore", input.ModCtrl|input.ModShift)
	if got := dl.comms.giftOf(3); got != 5000 {
		t.Fatalf("stops at the silver in reach: %d", got)
	}
	for i := 0; i < 2; i++ {
		press("comms.3.giftless", input.ModCtrl|input.ModShift)
	}
	if got := dl.comms.giftOf(3); got != 1 {
		t.Fatalf("stops at 1: %d", got)
	}
}
