package needs

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// TestFedAdultNeeds1Point6ADay (FOOD-1, wiki: Nutrition): a fed adult
// loses 1.6 nutrition a day, so a full bar empties in 0.625 day at the Fed
// rate; Hungry falls at half that and Ravenous at a quarter.
func TestFedAdultNeeds1Point6ADay(t *testing.T) {
	t.Parallel()
	if per := clock.TicksPerDay / HungerPeriod * HungerLoss(Full); per != 16000 {
		t.Fatalf("fed loss %d a day, want 16000 (1.6 nutrition)", per)
	}
	if HungerLoss(HungryAt) != HungerLoss(Full)/2 || HungerLoss(RavenousAt) != HungerLoss(Full)/4 || HungerLoss(0) != 0 {
		t.Fatal("the bands' falls: 100, 50, 25 %")
	}
	p, ticks := Pair{Hunger: Full, Rest: Full}, 0
	for now := uint64(1); p.Hunger > HungryAt; now++ {
		p = Decay(p, now)
		ticks++
	}
	// 75 % of the bar at 1.6 a day: 0.46875 day
	if want := clock.TicksPerDay * 75 / 160; ticks < want-HungerPeriod || ticks > want+HungerPeriod {
		t.Fatalf("full to hungry took %d ticks, want about %d", ticks, want)
	}
	if d := HungerPerHour*24 - 16000; d < -24 || d > 0 {
		t.Fatalf("the hourly fed loss %d", HungerPerHour)
	}
}

// TestDecayRatedNeutralIsDecay: the rated path at 1000 is Decay, and a
// gourmand's ×1.5 loses half again as much.
func TestDecayRatedNeutralIsDecay(t *testing.T) {
	t.Parallel()
	a, b, c := NewFull(), NewFull(), NewFull()
	var ch, cr, gh, gr int32
	for now := uint64(1); now <= 20000; now++ {
		a = Decay(a, now)
		b = DecayRated(b, now, 1000, 1000, &ch, &cr)
		c = DecayRated(c, now, 1500, 1000, &gh, &gr)
	}
	if a != b {
		t.Fatalf("neutral rated %+v, plain %+v", b, a)
	}
	if lost, base := Full-c.Hunger, Full-a.Hunger; lost*1000 < base*1450 {
		t.Fatalf("gourmand lost %d, a neutral pawn %d", lost, base)
	}
}

// TestHungerBandsNoFlap (FOOD-1): the hunger thought's band follows the
// level downwards at once but climbs back only BandGap past an edge, so
// food hovering at an edge never flips the thought every tick.
func TestHungerBandsNoFlap(t *testing.T) {
	t.Parallel()
	steps := []struct {
		hunger int
		want   Band
	}{
		{Full, Fed}, {HungryAt, Hungry}, {HungryAt + 1, Hungry}, {HungryAt + BandGap, Hungry}, {HungryAt + BandGap + 1, Fed},
		{RavenousAt, Ravenous}, {RavenousAt + BandGap/2, Ravenous}, {0, Malnourished}, {BandGap / 2, Malnourished},
		{BandGap + 1, Ravenous}, {Full, Fed},
	}
	p := NewFull()
	for i, s := range steps {
		p.Hunger = s.hunger
		if p = p.Settle(); p.Band != s.want {
			t.Fatalf("step %d: food %d gives band %d, want %d", i, s.hunger, p.Band, s.want)
		}
	}
	flips := 0
	p = Pair{Hunger: HungryAt}.Settle()
	for i := range 200 {
		p.Hunger = HungryAt + i%2*(BandGap/2) // hovering at the edge
		was := p.Band
		if p = p.Settle(); p.Band != was {
			flips++
		}
	}
	if flips != 0 {
		t.Fatalf("the band flipped %d times hovering at the Hungry edge", flips)
	}
}

// TestSeekFoodNoFlap (FOOD-1): a pawn wants food from 30 % (EatAt) until
// it is back at SatedAt; a small bite that lifts it just above 30 % does
// not stop it looking, and once sated it waits for the next 30 %.
func TestSeekFoodNoFlap(t *testing.T) {
	t.Parallel()
	p := Pair{Hunger: EatAt + 1}.Settle()
	if p.Hungry() {
		t.Fatal("above 30 % a fed pawn does not look for food")
	}
	p.Hunger = EatAt
	if p = p.Settle(); !p.Hungry() {
		t.Fatal("at 30 % it goes to eat")
	}
	p.Hunger = EatAt + 500 // one raw unit
	if p = p.Settle(); !p.Hungry() {
		t.Fatal("a bite short of sated: still hungry")
	}
	p.Hunger = SatedAt
	if p = p.Settle(); p.Hungry() {
		t.Fatal("sated: it stops")
	}
	changes, was := 0, p.Hungry()
	for now := uint64(1); p.Hunger > EatAt+HungerLoss(Full); now++ {
		if p = Decay(p, now); p.Hungry() != was {
			changes++
			was = p.Hungry()
		}
	}
	if changes != 0 {
		t.Fatalf("seeking changed %d times between sated and 30 %%", changes)
	}
}

// TestPortions: the units nearest to a full bar, at least one, at most
// the cap; a meal is one whatever the hunger (the excess is wasted).
func TestPortions(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ hunger, nut, most, want int }{
		{EatAt, 9000, 1, 1}, {0, 9000, 1, 1}, {EatAt, 500, 20, 14}, {0, 500, 20, 20}, {9900, 500, 20, 1}, {5000, 0, 20, 1},
	} {
		if got := Portions(c.hunger, c.nut, c.most); got != c.want {
			t.Errorf("Portions(%d, %d, %d) = %d, want %d", c.hunger, c.nut, c.most, got, c.want)
		}
	}
}

// TestFightBand: the social-fight hunger bands.
func TestFightBand(t *testing.T) {
	t.Parallel()
	for h, want := range map[int]int{Full: 0, CriticalHunger: 1, CriticalHunger / 2: 2, CriticalHunger / 4: 3, 0: 4} {
		if got := FightBand(h); got != want {
			t.Errorf("FightBand(%d) = %d, want %d", h, got, want)
		}
	}
}
