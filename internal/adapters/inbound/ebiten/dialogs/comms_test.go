package dialogs

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"

	"rimworld-go/internal/application/game"
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
