package furnish

import (
	"testing"

	"rimworld-go/internal/application/game/cadence"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

type fakeHost struct {
	ids     []entity.EntityID
	b       map[entity.EntityID]construction.Building
	rate    int
	air     int
	recount int
}

func (f *fakeHost) BuildingIDs() []entity.EntityID { return f.ids }
func (f *fakeHost) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := f.b[id]
	return b, ok
}
func (f *fakeHost) SetBuilding(id entity.EntityID, b construction.Building) { f.b[id] = b }
func (f *fakeHost) GrowRate(int, int) int                                   { return f.rate }
func (f *fakeHost) RoomsChanged()                                           { f.recount++ }
func (f *fakeHost) Air(int, int) int                                        { return f.air }

func newHost() *fakeHost {
	return &fakeHost{ids: []entity.EntityID{1, 2}, rate: 1000, air: 200, b: map[entity.EntityID]construction.Building{
		1: {Kind: construction.PlantPot}, 2: {Kind: construction.Chair}}}
}

// noon is a growing hour, on the hour.
func noon(day uint64) uint64 { return day*clock.TicksPerDay + 12*clock.TicksPerHour }

// at is the pots' slot (cadence) in the hour starting at t: Tick runs
// then only.
func at(t uint64) uint64 { return cadence.Next(t, PotEvery, cadence.OffPot, 0) }

// TestPotSowsAndGrows (AX-2): SetPlant picks a pot's plant only (not a
// chair, not an unknown plant); a grower sows it once (ToSow, Sow); the
// plant grows in the growing hours at full rate
// to grown in its grow days of growing time, not at night or without
// light, and rooms are recounted at each tenth.
func TestPotSowsAndGrows(t *testing.T) {
	t.Parallel()
	h := newHost()
	p := NewPots(h)
	if p.SetPlant(2, construction.PotRose) || p.SetPlant(1, construction.PotPlant(9)) {
		t.Fatal("sowed a chair or an unknown plant")
	}
	if !p.SetPlant(1, construction.PotDaylily) || h.b[1].Plant != uint8(construction.PotDaylily) || h.recount != 1 {
		t.Fatalf("sow: %+v recounts %d", h.b[1], h.recount)
	}
	if p.Tick(at(noon(0))); h.b[1].Growth != 0 {
		t.Fatal("an unsown pot grew")
	}
	if id, x, y, work, ok := p.ToSow(5, 5); !ok || id != 1 || x != 0 || y != 0 || work != 540 {
		t.Fatalf("ToSow %d (%d,%d) work %d %v, want pot 1 (wiki: Daylily, 540)", id, x, y, work, ok)
	}
	if !p.Sow(1) || h.b[1].Growth != Sprout || p.Sow(1) {
		t.Fatal("sowing: once, to a sprout")
	}
	if _, _, _, _, ok := p.ToSow(5, 5); ok {
		t.Fatal("a sown pot still waits")
	}
	p.Tick(at(noon(0)) + 1)            // not the pots' slot
	p.Tick(at(6 * clock.TicksPerHour)) // 06:00: growing
	g0 := h.b[1].Growth
	p.Tick(at(noon(0) + 10*clock.TicksPerHour)) // 22:00: resting
	if h.b[1].Growth != g0 {
		t.Fatal("grew at night")
	}
	hours := 0
	for d := uint64(0); h.b[1].Growth < 1000 && d < 30; d++ {
		for hr := uint64(plant6); hr < plant19; hr++ {
			p.Tick(at(d*clock.TicksPerDay + hr*clock.TicksPerHour))
			hours++
		}
	}
	// 3 grow days of growing time is 72 h at full rate (77 with the hourly step rounded down)
	if h.b[1].Growth != 1000 || hours < 70 || hours > 80 {
		t.Fatalf("grown %d after %d growing hours, want 1000 near 72", h.b[1].Growth, hours)
	}
	if h.recount < 10 {
		t.Fatalf("%d recounts, want one per tenth", h.recount)
	}
	h.b[1] = construction.Building{Kind: construction.PlantPot, Plant: uint8(construction.PotRose), Growth: Sprout}
	h.rate = 0
	p.Tick(at(noon(5)))
	if h.b[1].Growth != Sprout {
		t.Fatal("grew without light")
	}
	if !p.SetPlant(1, construction.PotNone) || h.b[1].Plant != 0 {
		t.Fatal("emptying the pot")
	}
}

const plant6, plant19 = 6, 19

func TestGrowStep(t *testing.T) {
	t.Parallel()
	if Grow(construction.PotRose, 0, 1000) != 8 || Grow(construction.PotRose, 999, 1000) != 1000 || Grow(construction.PotNone, 5, 1000) != 5 {
		t.Fatal("grow step: a rose 2500/300000 per hour, capped, none for no plant")
	}
}

// TestPotFrostDoesNotFlap (AX-2): a pot plant dies below PotFrost and is
// sown again only at PotSowAir or warmer, so air between the two neither
// kills a plant nor gets one sown: no sow-and-freeze loop.
func TestPotFrostDoesNotFlap(t *testing.T) {
	t.Parallel()
	h := newHost()
	p := NewPots(h)
	h.b[1] = construction.Building{Kind: construction.PlantPot, Plant: uint8(construction.PotRose), Growth: 500}
	h.air = PotFrost + 1
	p.Tick(at(noon(0)))
	if h.b[1].Growth == 0 {
		t.Fatal("died above the frost line")
	}
	h.air = PotFrost - 1
	p.Tick(at(noon(0) + clock.TicksPerHour*12)) // at night too
	if h.b[1].Growth != 0 {
		t.Fatal("survived the frost")
	}
	sows := 0
	for i, air := range []int{PotFrost + 1, PotSowAir - 1, PotFrost + 5, PotSowAir - 20} {
		h.air = air
		if _, _, _, _, ok := p.ToSow(0, 0); ok {
			sows++
		}
		p.Tick(at(noon(uint64(1 + i))))
	}
	if sows != 0 {
		t.Fatalf("sown %d times in the cold band", sows)
	}
	h.air = PotSowAir
	if _, _, _, _, ok := p.ToSow(0, 0); !ok {
		t.Fatal("not sown once warm again")
	}
}
