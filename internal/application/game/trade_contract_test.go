package game

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// TestEncodeTradeOfferRoundTrip: an offer survives the trip through
// command.Command.Name (buys and sells, any order, the empty offer), and
// malformed text, unknown kinds, zero quantities and repeats are refused.
func TestEncodeTradeOfferRoundTrip(t *testing.T) {
	t.Parallel()
	for _, offer := range [][]trade.Delta{
		nil,
		{{Kind: item.Steel, Qty: 25}},
		{{Kind: item.Components, Qty: -3}, {Kind: item.Meal, Qty: 12}, {Kind: item.Rifle, Qty: -1}},
		{{Kind: item.Wood, Qty: 1000000}, {Kind: item.Food, Qty: -999}},
	} {
		s := EncodeTradeOffer(offer)
		got, err := DecodeTradeOffer(s)
		if err != nil || !reflect.DeepEqual(got, offer) && (len(got) != 0 || len(offer) != 0) {
			t.Fatalf("%q decodes to %v, %v; want %v", s, got, err, offer)
		}
		// Command must stay comparable (a map key): the offer is a string
		_ = map[command.Command]bool{{Kind: command.ProposeTrade, Name: s}: true}
	}
	want := fmt.Sprintf("%d:-4,%d:2", item.Steel, item.Meal)
	if s := EncodeTradeOffer([]trade.Delta{{Kind: item.Steel, Qty: -4}, {Kind: item.Meal, Qty: 2}}); s != want {
		t.Fatalf("encoding %q, want %q", s, want)
	}
	for _, bad := range []string{"x", "1", "1:", ":2", "1:2,", "1:0", "1:2,1:3", "250:1", "-1:2", "1:2:3", "1;2", "1: 2"} {
		if got, err := DecodeTradeOffer(bad); err == nil {
			t.Errorf("%q decoded to %v", bad, got)
		}
	}
}

// TestTradeIncidentsDoNotFireYet: on a legacy colony (no set-up, no
// factions, no radio console) the trader caravan and the orbital trader
// never fit, so its nature rolls are unchanged by PLAY-024.
func TestTradeIncidentsDoNotFireYet(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 3, Width: 40, Height: 30})
	g.Step(500)
	now := uint64(g.clock.Now()) + 30*clock.TicksPerDay // past both events' first day
	for _, id := range []incident.ID{incident.TraderCaravan, incident.OrbitalTrader} {
		if g.incidentFits(id, now) {
			t.Errorf("%s fits", incident.DefOf(id).Key)
		}
		if g.FireIncident(incident.DefOf(id).Key) {
			t.Errorf("%s fired", incident.DefOf(id).Key)
		}
	}
	if d := incident.DefOf(incident.TraderCaravan); d.Weight != 14 || d.Cooldown != 9 || d.MinDay != 4 || d.Category != incident.Nature {
		t.Fatalf("trader caravan %+v", d)
	}
	if d := incident.DefOf(incident.OrbitalTrader); d.Weight != 12 || d.Cooldown != 7 || d.MinDay != 6 || d.Category != incident.Nature {
		t.Fatalf("orbital trader %+v", d)
	}
}

// decodeSave reads back the state a save wrote.
func decodeSave(t *testing.T, g *Game) saveState {
	t.Helper()
	var buf bytes.Buffer
	if err := g.Save(&buf, "trade"); err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	var s saveState
	if err := dec.Decode(&info); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestTradeNilBlockKeepsHash: a colony that never traded and has no
// turrets saves nil Trade and Turrets blocks, and a save round trip keeps
// its StateHash (so legacy goldens hold).
func TestTradeNilBlockKeepsHash(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 5, Width: 40, Height: 30})
	g.Step(500)
	if s := decodeSave(t, g); s.Trade != nil || s.Turrets != nil {
		t.Fatalf("blocks: trade %v, turrets %v", s.Trade, s.Turrets)
	}
	before := g.StateHash()
	h := saveLoad(t, g)
	if after := h.StateHash(); after != before {
		t.Fatalf("hash moved over a round trip: %x -> %x", before, after)
	}
	g.Step(500)
	h.Step(500)
	if g.StateHash() != h.StateHash() {
		t.Fatal("the loaded game drifted")
	}
}

// TestTradeCommandsInertInPhase0: the trade and comms commands route to
// their handlers and change nothing while no session is open; the views
// report none.
func TestTradeCommandsInertInPhase0(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 6, Width: 40, Height: 30})
	before := g.StateHash()
	for _, k := range []command.Kind{command.ProposeTrade, command.CloseTrade, command.CommsCallTrader, command.CommsGift,
		command.CommsRequestCaravan, command.CommsRequestAid, command.CloseComms} {
		g.Enqueue(command.Command{Kind: k, Index: 1, Count: 40, Name: EncodeTradeOffer([]trade.Delta{{Kind: item.Steel, Qty: 5}})})
	}
	g.ApplyPending()
	if g.StateHash() != before {
		t.Fatal("an inert command changed the state")
	}
	if _, ok := g.TradeView(); ok {
		t.Fatal("no trade session is open")
	}
	if _, ok := g.CommsView(); ok {
		t.Fatal("no comms session is open")
	}
}

// TestPlay024BuildingsAndOrders: the new kinds build (steel only), the
// turret reports its idle aim, and a console offers "Use radio console",
// refused while it has no power (PLAY-024 Phase 1).
func TestPlay024BuildingsAndOrders(t *testing.T) {
	t.Parallel()
	g := conduitGame()
	for _, k := range []construction.Kind{construction.SentryTurret, construction.TradeBeacon, construction.RadioConsole} {
		if construction.DefaultStuff(k) != item.Steel || construction.StuffOK(k, item.Wood) {
			t.Fatalf("%s must be steel only", k)
		}
	}
	tur := buildNow(t, g, construction.SentryTurret, item.Steel, construction.South, 2, 2)
	buildNow(t, g, construction.RadioConsole, item.Steel, construction.South, 6, 2)
	if construction.MaxHP(construction.SentryTurret, item.Steel) != 140 || construction.CoverPct(construction.SentryTurret) != 30 {
		t.Fatal("turret hit points or cover")
	}
	var found bool
	for _, b := range g.World().Buildings {
		if b.ID == tur {
			found = true
			if b.AimDir != 0 || b.ShotAge != -1 {
				t.Fatalf("idle turret view: aim %d, shot age %d", b.AimDir, b.ShotAge)
			}
		}
	}
	if !found {
		t.Fatal("turret not in the world view")
	}
	pawn, err := g.SpawnPawn(6, 5)
	if err != nil {
		t.Fatal(err)
	}
	var use *OrderOption
	opts := g.OrdersAt(pawn, 6, 2)
	for i := range opts {
		if opts[i].Label == "Use radio console" {
			use = &opts[i]
		}
	}
	if use == nil || !use.Disabled || use.Reason != ReasonNoPower || use.Cmd.Order != command.OrderUseComms {
		t.Fatalf("console order: %+v", use)
	}
	if job.Trade.String() != "trade" || job.UseComms.String() != "use radio console" {
		t.Fatal("job names")
	}
}
