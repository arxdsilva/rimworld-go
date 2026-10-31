package animal

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// TestBreedingWikiNumbers (FOOD-6, wiki: Cow, Husky, Thrumbo, Chicken):
// a species' gestation, litter, stages and lifespan are its vanilla
// counterpart's.
func TestBreedingWikiNumbers(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		s                              Species
		gest                           uint64
		litMin, litMax, juv, adult, ld int
	}{
		{Cow, 666 * clock.TicksPerDay / 100, 1, 1, 12, 20, 22 * 60},
		{Husky, 10 * clock.TicksPerDay, 1, 3, 15, 30, 12 * 60},
		{Glimmerstag, 20 * clock.TicksPerDay, 1, 1, 45, 80, 220 * 60},    // a thrumbo's
		{Woolhorn, 666 * clock.TicksPerDay / 100, 1, 1, 15, 20, 15 * 60}, // a muffalo's
		{Chicken, 0, 1, 1, 7, 12, 6 * 60},
	} {
		l := LifeOf(c.s)
		if GestTicks(c.s) != c.gest || l.LitterMin != c.litMin || l.LitterMax != c.litMax ||
			l.JuvenileDays != c.juv || l.AdultDays != c.adult || l.LifeDays != c.ld {
			t.Errorf("%s: life %+v, gestation %d", c.s, l, GestTicks(c.s))
		}
	}
}

// TestEveryBreederHasWikiRow: every species that breeds by live birth
// has a gestation, and every living species has its stages in order.
func TestEveryBreederHasWikiRow(t *testing.T) {
	t.Parallel()
	for s := Species(0); s < numSpecies; s++ {
		l := LifeOf(s)
		if IsMech(s) {
			continue
		}
		if Breeds(s) && !Layer(s) && l.GestCenti <= 0 {
			t.Errorf("%s: a live bearer without a gestation", s)
		}
		if l.JuvenileDays <= 0 || l.JuvenileDays >= l.AdultDays {
			t.Errorf("%s: stages %d / %d", s, l.JuvenileDays, l.AdultDays)
		}
		if Juvenile(s, l.JuvenileDays-1) || !Juvenile(s, l.JuvenileDays) || StageSize(s, l.JuvenileDays) != juvenileSize {
			t.Errorf("%s: the juvenile stage starts at %d days", s, l.JuvenileDays)
		}
	}
}
