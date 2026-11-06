package game

import (
	"testing"

	"rimworld-go/internal/application/game/bodytemp"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// AIL-1: no exposure state holds a colonist down forever.

// heatwaveColony: Ada downed by 80 % heatstroke in her own bed in a
// bedroom held at hot (tenths), a cool room held at 20 °C with a free
// bed, and Bo, a doctor, outside in 22 °C air. pin re-holds both rooms'
// air (the test's stand-in for a heatwave and a cooler).
func heatwaveColony(t *testing.T, hot temperature.Temp) (g *Game, ada, bo entity.EntityID, pin func()) {
	t.Helper()
	g, ada = def4Game(t, 30, 14, 4, 3)
	g.climate = temperature.Climate{YearMean: 220}
	bo, err := g.SpawnNamedPawn(10, 11, "Bo")
	if err != nil {
		t.Fatal(err)
	}
	hut(t, g, 1, 1, 7, 6)
	hut(t, g, 13, 1, 19, 6)
	own := buildNow(t, g, construction.Bed, item.Wood, construction.South, 4, 3)
	buildNow(t, g, construction.Bed, item.Wood, construction.South, 16, 3)
	assignOwner(g, own, ada)
	b, _ := g.bldgs.Get(own)
	x, y := g.sleepCellFor(ada, b)
	g.positions.Set(ada, Position{X: x, Y: y})
	g.ensureRooms()
	hotRoom, coolRoom := g.roomAt(4, 3), g.roomAt(16, 3)
	if hotRoom < 0 || coolRoom < 0 || hotRoom == coolRoom {
		t.Fatalf("rooms %d %d", hotRoom, coolRoom)
	}
	pin = func() {
		g.ensureRooms()
		g.thermo.Temps[g.roomAt(4, 3)], g.thermo.Temps[g.roomAt(16, 3)] = hot, 200
	}
	pin()
	g.exposure[ada] = bodytemp.Sev{Heat: exposure.Full / 100 * 80}
	g.expose()
	if v, _ := g.vital.Get(ada); v.Status != health.Downed || v.Cause != health.CauseHeatstroke || g.bedUnder(ada) != own {
		t.Fatalf("Ada downed by heatstroke in her own bed: %+v bed %d", v, g.bedUnder(ada))
	}
	return g, ada, bo, pin
}

// TestHeatstrokeVictimInHotBedIsCarriedToACoolRoomAndStands: in 30, 32
// and 34 °C (held: hotter than her comfortable 26 °C, short of 36 °C
// where it builds) she would lie forever; a doctor carries her once to
// the cool room's bed, where she recovers and stands, and nobody is
// carried twice.
func TestHeatstrokeVictimInHotBedIsCarriedToACoolRoomAndStands(t *testing.T) {
	t.Parallel()
	for _, hot := range []temperature.Temp{300, 320, 340} {
		g, ada, bo, pin := heatwaveColony(t, hot)
		if pv := pawnViewOf(g, ada); pv.ExposureNote == "" {
			t.Fatalf("%d: the view says why she isn't recovering", hot)
		}
		carries, carrying, stood := 0, false, uint64(0)
		for tick := 0; tick < 12*clock.TicksPerHour && stood == 0; tick += 50 {
			g.Step(50)
			pin()
			g.SetNeeds(bo, needs.Full, needs.Full)
			if now := g.care.Carry.Patient(bo) == ada; now != carrying {
				carrying = now
				if now {
					carries++
				}
			}
			if v, _ := g.vital.Get(ada); v.Status == health.Alive {
				stood = uint64(tick)
			}
		}
		if stood == 0 || carries != 1 || g.roomAt(posOf(g, ada).X, posOf(g, ada).Y) != g.roomAt(16, 3) {
			t.Fatalf("%d: stood after %d ticks, carried %d times, in room %d", hot, stood, carries, g.roomAt(posOf(g, ada).X, posOf(g, ada).Y))
		}
		t.Logf("%d: carried once, stood after %.1f h", hot, float64(stood)/clock.TicksPerHour)
	}
}

// TestExposureDownedStandsOnceItCan (AIL-002): a pawn downed by
// heatstroke that could not stand the last time its stage changed (another
// cause held it) stands on the next climate interval once nothing holds
// it, though no stage changes any more.
func TestExposureDownedStandsOnceItCan(t *testing.T) {
	t.Parallel()
	g, ada := coldGame(t, 220)
	v, _ := g.vital.Get(ada)
	g.ensureBody(ada, &v)
	v.Status, v.Cause = health.Downed, health.CauseHeatstroke
	g.vital.Set(ada, v)
	g.expose()
	if v, _ := g.vital.Get(ada); v.Status != health.Alive {
		t.Fatalf("nothing holds her: she stands, %+v", v)
	}
}

func pawnViewOf(g *Game, id entity.EntityID) PawnView {
	for _, pv := range g.Snapshot() {
		if pv.ID == id {
			return pv
		}
	}
	return PawnView{}
}
