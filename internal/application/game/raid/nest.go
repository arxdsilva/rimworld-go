package raid

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Hive mounds (CBT-4). wiki: Hive, Infestation: an infestation raises
// hives under a roof in the dark and the warm (above -17 °C), out of
// which insects come; a hive spreads, a new one rising on a roofed tile
// within 10 tiles and never right beside another, until there are 30;
// the insects stay by their hive and the whole brood turns on anyone who
// comes within 10 tiles. Destroying a hive ends its spawning (its insects
// live on). How often a hive breeds and spreads is not published:
// hiveBreedEvery, hiveSpreadEvery and hiveBrood are original tuning.
// The brood waits dormant (the animal brain leaves it be) until woken.

// Nest is what hive mounds need from the game (value only).
type Nest interface {
	// HiveSpot: a hive mound may rise on (x, y): dry, open ground under
	// a roof, dark, and warmer than -17 °C (wiki: Infestation).
	HiveSpot(x, y int) bool
	// PlaceHive raises a hive mound on (x, y) (0: no room).
	PlaceHive(x, y int) entity.EntityID
	// HiveStands: hive mound id still stands.
	HiveStands(id entity.EntityID) bool
}

const (
	// HiveCap: no hive spreads once there are this many (wiki: Hive).
	HiveCap = 30
	// hiveSpread: a new hive rises within this many tiles; hiveApart:
	// never within this of another (wiki: Hive, 10 and the adjacent
	// cells).
	hiveSpread = 10
	hiveApart  = 1
	// hiveAggro: the brood wakes when a colonist comes this close (wiki:
	// Infestation, 10).
	hiveAggro = 10
	// hiveBrood is the insects a hive keeps; hiveBreedEvery the ticks
	// between two (plus up to half again); hiveSpreadEvery the ticks
	// between spreads (plus up to a day). original tuning
	hiveBrood       = 3
	hiveBreedEvery  = clock.TicksPerDay
	hiveSpreadEvery = 3 * clock.TicksPerDay
	// hiveEvery paces each hive's look (staggered by id).
	hiveEvery = 60
	// hiveInfest is how far from the rock face an infestation looks for
	// its first hive's spot. original tuning
	hiveInfest = 6
)

// Hive is one hive mound's state (saved in MechSave). Field names are
// gob names: never rename.
type Hive struct {
	ID            entity.EntityID
	X, Y          int
	Breed, Spread uint64            // ticks the next insect comes and the next hive rises
	Brood         []entity.EntityID // its living insects, ascending
}

// Hives lists the hive mounds standing (ascending ids).
func (hv *Hives) List() []Hive { return hv.hives }

// infest raises the first hive of an infestation near (x, y) at now and
// reports it (ok false: no dark, warm, roofed spot near).
func (hv *Hives) infest(x, y int, now uint64) (int, bool) {
	if hv.n == nil || len(hv.hives) >= HiveCap {
		return 0, false
	}
	for d := 0; d <= hiveInfest; d++ {
		for dy := -d; dy <= d; dy++ {
			for dx := -d; dx <= d; dx++ {
				if max(dx, -dx, dy, -dy) != d || !hv.apart(x+dx, y+dy) || !hv.n.HiveSpot(x+dx, y+dy) {
					continue
				}
				if i, ok := hv.raise(x+dx, y+dy, now); ok {
					return i, true
				}
			}
		}
	}
	return 0, false
}

// raise places a hive mound on (x, y) and records it.
func (hv *Hives) raise(x, y int, now uint64) (int, bool) {
	id := hv.n.PlaceHive(x, y)
	if id == 0 {
		return 0, false
	}
	hv.hives = append(hv.hives, Hive{ID: id, X: x, Y: y, Breed: now + hiveBreedEvery + uint64(hv.h.Roll(hiveBreedEvery/2)),
		Spread: now + hiveSpreadEvery + uint64(hv.h.Roll(clock.TicksPerDay))})
	slices.SortFunc(hv.hives, func(a, b Hive) int { return cmp.Compare(a.ID, b.ID) })
	return slices.IndexFunc(hv.hives, func(h Hive) bool { return h.ID == id }), true
}

// apart: no hive stands within hiveApart of (x, y).
func (hv *Hives) apart(x, y int) bool {
	for _, o := range hv.hives {
		if max(abs(o.X-x), abs(o.Y-y)) <= hiveApart {
			return false
		}
	}
	return true
}

// Step runs the hive mounds (the threat phase, once a tick): each on its
// paced look drops a destroyed hive, forgets dead insects, wakes the
// brood when a colonist comes near, breeds and spreads.
func (hv *Hives) Step(now uint64) {
	hv.now = now
	for i := 0; i < len(hv.hives); i++ {
		if (now+uint64(hv.hives[i].ID))%hiveEvery != 0 {
			continue
		}
		if !hv.n.HiveStands(hv.hives[i].ID) {
			hv.hives = slices.Delete(hv.hives, i, i+1) // destroyed: no more insects from it
			i--
			continue
		}
		hv.look(&hv.hives[i], now)
	}
}

// look is one paced look of hive hr.
func (hv *Hives) look(hr *Hive, now uint64) {
	hr.Brood = slices.DeleteFunc(hr.Brood, func(id entity.EntityID) bool {
		_, _, ok := hv.h.Pos(id)
		return !ok || hv.w.Animal(id).Dead()
	})
	if hv.m != nil && hv.m.ColonistNear(hr.X, hr.Y, hiveAggro) {
		for _, id := range hr.Brood {
			if a := hv.w.Animal(id); a.Dormant {
				wake(&a, now)
				hv.w.SetAnimal(id, a)
			}
		}
	}
	if now >= hr.Breed {
		hr.Breed = now + hiveBreedEvery + uint64(hv.h.Roll(hiveBreedEvery/2))
		if len(hr.Brood) < hiveBrood {
			hv.breed(hr)
		}
	}
	if now >= hr.Spread {
		hr.Spread = now + hiveSpreadEvery + uint64(hv.h.Roll(clock.TicksPerDay))
		hv.spread(*hr, now)
	}
}

// breed brings one insect out beside hive hr, dormant by its hive.
func (hv *Hives) breed(hr *Hive) {
	s := [...]animal.Species{animal.HiveBeetle, animal.HiveCrawler, animal.HiveBrute}[hv.h.Roll(3)]
	id, ok := hv.SpawnNear(s, hr.X, hr.Y)
	if !ok {
		return
	}
	a := hv.w.Animal(id)
	a.Dormant = true
	hv.w.SetAnimal(id, a)
	hr.Brood = append(hr.Brood, id)
}

// spread raises a new hive on a free spot within hiveSpread of hr.
func (hv *Hives) spread(hr Hive, now uint64) {
	if len(hv.hives) >= HiveCap {
		return
	}
	for try := 0; try < 12; try++ {
		x, y := hr.X+hv.h.Roll(2*hiveSpread+1)-hiveSpread, hr.Y+hv.h.Roll(2*hiveSpread+1)-hiveSpread
		if hv.h.InBounds(x, y) && hv.apart(x, y) && hv.n.HiveSpot(x, y) {
			if _, ok := hv.raise(x, y, now); ok {
				return
			}
		}
	}
}

// adopt makes the insects of an infestation the brood of hive i.
func (hv *Hives) adopt(i int, ids []entity.EntityID) {
	hv.hives[i].Brood = append(hv.hives[i].Brood, ids...)
	slices.Sort(hv.hives[i].Brood)
}
