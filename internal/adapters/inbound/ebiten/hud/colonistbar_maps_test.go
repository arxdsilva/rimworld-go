package hud

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
)

func twoMaps(viewed game.MapID, pawns ...game.PawnView) (*Chrome, *fakeShell) {
	c, f := newFake(pawns...)
	f.set.Gameplay.ShowColonistBar = true
	f.env.Updates = 1
	f.env.Game = fakeGame{
		pawns:     pawns,
		maps:      []game.MapView{{ID: 0, Label: "Colony"}, {ID: 1, Label: "Mine site", Threat: true}},
		colonists: pawns,
		viewed:    viewed,
	}
	return c, f
}

func lay(c *Chrome, f *fakeShell) {
	st := hudlayout.Inputs{Colonists: c.Bar().Count(), BarGroups: c.Bar().GroupSizes(), GlobalH: 196}
	f.lay = hudlayout.Compute(1280, 720, st)
}

func TestSingleMapBarHasNoGroups(t *testing.T) {
	c, f := newFake(game.PawnView{ID: 1, Status: "alive"}, game.PawnView{ID: 2, Status: "alive"})
	f.set.Gameplay.ShowColonistBar = true
	if g := c.Bar().GroupSizes(); g != [hudlayout.MaxBarGroups]int{} {
		t.Fatalf("groups %v", g)
	}
	lay(c, f)
	if len(f.lay.BarGroups) != 0 || len(f.lay.BarSlots) != 2 {
		t.Fatalf("layout %+v", f.lay)
	}
	for _, b := range c.Bar().Buttons(1280, 720) {
		if _, ok := BarMapOf(b.ID); ok {
			t.Fatalf("a frame button on a single-map bar: %s", b.ID)
		}
	}
}

func TestTwoMapsGroupTheBar(t *testing.T) {
	c, f := twoMaps(0,
		game.PawnView{ID: 1, Map: 0, Status: "alive"},
		game.PawnView{ID: 2, Map: 1, Status: "alive"},
		game.PawnView{ID: 3, Map: 0, Status: "alive"},
		game.PawnView{ID: 4, Map: 1, Status: "alive", Prisoner: true})
	sizes := c.Bar().GroupSizes()
	if sizes[0] != 2 || sizes[1] != 1 || sizes[2] != 0 || c.Bar().Count() != 3 {
		t.Fatalf("sizes %v count %d", sizes, c.Bar().Count())
	}
	lay(c, f)
	var slots, frames int
	for _, b := range c.Bar().Buttons(1280, 720) {
		if m, ok := BarMapOf(b.ID); ok {
			frames++
			if (m == 0) != b.Selected {
				t.Errorf("map %d selected %v", m, b.Selected)
			}
		} else {
			slots++
		}
	}
	if slots != 3 || frames != 2 {
		t.Fatalf("%d slots, %d frames", slots, frames)
	}
	if ps := c.Bar().Pawns(); ps[0].ID != 1 || ps[1].ID != 3 || ps[2].ID != 2 {
		t.Fatalf("order %v", ps)
	}
}

func TestClickAwayJumpsOrRefuses(t *testing.T) {
	c, f := twoMaps(0,
		game.PawnView{ID: 1, Map: 0, X: 1, Y: 1, Status: "alive"},
		game.PawnView{ID: 2, Map: 1, X: 7, Y: 8, Status: "alive"})
	if c.Bar().ClickAway(1, false) || len(f.jumps) != 0 {
		t.Fatal("a colonist on this map is the caller's")
	}
	if !c.Bar().ClickAway(2, true) || len(f.jumps) != 0 || len(c.LiveToasts()) != 1 {
		t.Fatalf("shift across maps: jumps %v toasts %d", f.jumps, len(c.LiveToasts()))
	}
	if !c.Bar().ClickAway(2, false) || len(f.jumps) != 1 || f.jumps[0] != (ports.Spot{Map: 1, X: 7, Y: 8, Focus: 2}) {
		t.Fatalf("jumps %v", f.jumps)
	}
}

func TestOtherMapThreatRaisesAnAlert(t *testing.T) {
	c, f := twoMaps(0, game.PawnView{ID: 1, Status: "alive"})
	var got *Alert
	for _, al := range c.Alerts().Current() {
		if al.Cross {
			got = &al
		}
	}
	if got == nil || got.Label != "Threat on Mine site" || got.Map != 1 || got.Sev != SevDanger || got.Explain == "" {
		t.Fatalf("alert %+v", got)
	}
	c.Alerts().cycle(got.Key, 1)
	if len(f.jumps) != 1 || f.jumps[0].Map != 1 || f.jumps[0].X >= 0 {
		t.Fatalf("jumps %v", f.jumps)
	}
	// on the threatened map itself there is no cross-map alert
	f.env.Game = fakeGame{maps: f.env.Game.(fakeGame).maps, viewed: 1}
	f.env.Updates++
	for _, al := range c.Alerts().Current() {
		if al.Cross {
			t.Fatalf("cross alert on the viewed map: %+v", al)
		}
	}
}

func TestLetterJumpCarriesItsMap(t *testing.T) {
	c, f := twoMaps(0)
	c.Right().jumpToLetter(game.LetterView{Map: 1, X: 3, Y: 4, Focus: 9})
	if len(f.jumps) != 1 || f.jumps[0] != (ports.Spot{Map: 1, X: 3, Y: 4, Focus: 9}) {
		t.Fatalf("jumps %v", f.jumps)
	}
	c.Right().jumpToLetter(game.LetterView{Map: 1, X: 5, Y: 6})
	if f.jumps[1] != (ports.Spot{Map: 1, X: 5, Y: 6}) {
		t.Fatalf("jumps %v", f.jumps)
	}
}

func TestBarMapIDRoundTrip(t *testing.T) {
	if m, ok := BarMapOf(BarMapID(4)); !ok || m != 4 {
		t.Fatal("round trip")
	}
	if _, ok := BarMapOf("bar.pawn.4"); ok {
		t.Fatal("a pawn slot is not a map")
	}
}
