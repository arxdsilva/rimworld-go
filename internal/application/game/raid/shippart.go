package raid

import (
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Crashed ship parts (DEF-4, the CrashedShipPart threat). A blighting
// hull (our name for the core game's defoliating ship part) comes down
// some way from the colony with a group of dormant mechanoid guards round
// it, bought with the raid points. It just sits there, withering every
// plant within a radius that grows from 5 tiles to 100 over 20 days, until
// the colony destroys it (1200 hit points); then it breaks into salvage.
//
// Its guards sleep (animal.State.Dormant: no thought, no needs) until they
// are disturbed: a colonist comes within guardWake of one of them or
// within partWake of the hull's middle, or the hull or a guard is hurt
// (an EMP that reaches one counts). Then the whole group wakes, at once
// and for good: waking is a one-way latch, so nothing flaps back to sleep.
// The check runs every shipCheckEvery ticks over the few parts there are
// (no scan of the map); the blight sweeps a 24th of its disc an hour.

const (
	// ShipPartMinPoints: no ship part below this many raid points. wiki:
	// Events (crashed ship parts need 400 raid points).
	ShipPartMinPoints = 400
	// shipCheckEvery paces the wake check. original tuning
	shipCheckEvery = 60
	// guardWake: a colonist this close to a sleeping guard wakes the group.
	// wiki: Crashed ship part (a 5x5 area centred on each mechanoid).
	guardWake = 2
	// partWake: a colonist this close to the hull's middle wakes it too.
	// original tuning (the hull is 6x3)
	partWake = 5
	// shipRangeMin..shipRangeMax: how far from the colony's centre a part
	// comes down. original tuning
	shipRangeMin = 20
	shipRangeMax = 45
	shipTries    = 60
	// The blight's radius: blightStart tiles at the crash growing to
	// blightMax over blightDays. wiki: Defoliator ship.
	blightStart = 5
	blightMax   = 100
	blightDays  = 20
	// blightBands: the blight sweeps one row in this many each hour, so
	// its whole disc once a day. original tuning
	blightBands = 24
)

// shipSalvage is what a destroyed part leaves. wiki: Crashed ship part
// (60 steel, 10 plasteel, a component and an advanced component; its four
// slag chunks have no item here).
var shipSalvage = [...]struct {
	kind item.Kind
	qty  int
}{{item.Steel, 60}, {item.Plasteel, 10}, {item.Components, 1}, {item.AdvancedComponents, 1}}

// ShipPart is one crashed part (saved: MechSave.Parts). Field names are
// gob names: never rename.
type ShipPart struct {
	ID     entity.EntityID
	X, Y   int    // the building's anchor
	Landed uint64 // crash tick
	Awake  bool   // its guards are woken
	Guards []entity.EntityID
}

// ShipParts runs the crashed ship parts.
type ShipParts struct {
	h     Host
	w     Wild
	m     MechHost
	mechs *Mechs
	parts []ShipPart
}

// middle is the hull's middle tile.
func (p *ShipPart) middle() (int, int) {
	sz := construction.DefOf(construction.ShipPart).Size
	return p.X + sz[0]/2, p.Y + sz[1]/2
}

// Crash brings a part down with guards worth points at tick now; false
// when there was no room for it.
func (sp *ShipParts) Crash(points int, now uint64) bool {
	x, y, id, ok := sp.land()
	if !ok {
		return false
	}
	p := ShipPart{ID: id, X: x, Y: y, Landed: now}
	sz := construction.DefOf(construction.ShipPart).Size
	for i, s := range sp.mechs.Group(points) {
		// round the hull: along its south side, then its north side
		gx, gy := x+i%(sz[0]+2)-1, y+sz[1]
		if (i/(sz[0]+2))%2 == 1 {
			gy = y - 1
		}
		gid, ok := sp.mechs.spawnNear(s, gx, gy)
		if !ok {
			continue
		}
		a := sp.w.Animal(gid)
		a.Dormant, a.Mode, a.Target, a.Path, a.PathI = true, animal.Wander, 0, nil, 0
		sp.w.SetAnimal(gid, a)
		px, py, _ := sp.h.Pos(gid)
		sp.mechs.recs[gid] = MechRecord{Guard: true, PostX: px, PostY: py}
		p.Guards = append(p.Guards, gid)
	}
	sp.parts = append(sp.parts, p)
	mx, my := p.middle()
	sp.h.Threat("Crashed ship part", mx, my,
		"A blighting hull has crashed nearby. Every plant around it withers, wider each day. War machines sleep beside it: they wake if anyone comes close or harms them.")
	return true
}

// land picks a spot shipRangeMin..shipRangeMax from the colony's centre
// and sets the part down there.
func (sp *ShipParts) land() (int, int, entity.EntityID, bool) {
	hx, hy := sp.h.Home()
	w, h := sp.h.Size()
	for try := 0; try < shipTries; try++ {
		x, y := sp.h.Roll(w), sp.h.Roll(h)
		if d := max(abs(x-hx), abs(y-hy)); d < shipRangeMin || d > shipRangeMax {
			continue
		}
		if id, ok := sp.m.PlaceShipPart(x, y); ok {
			return x, y, id, true
		}
	}
	return 0, 0, 0, false
}

// Tick runs every part's wake check and blight at tick now (the game's
// animal phase, after the creature index is rebuilt).
func (sp *ShipParts) Tick(now uint64) {
	check, hour := now%shipCheckEvery == 0, now%clock.TicksPerHour == 0
	if len(sp.parts) == 0 || !check && !hour {
		return
	}
	for i := range sp.parts {
		p := &sp.parts[i]
		if check && !p.Awake && sp.disturbed(p) {
			sp.wakeAll(p, now)
		}
		if hour {
			sp.blight(p, now)
		}
	}
}

// disturbed: a guard was woken (hurt, or caught by an EMP) or a standing
// colonist came close to a guard or to the hull.
func (sp *ShipParts) disturbed(p *ShipPart) bool {
	mx, my := p.middle()
	if sp.m.ColonistNear(mx, my, partWake) {
		return true
	}
	for _, g := range p.Guards {
		x, y, ok := sp.h.Pos(g)
		if !ok {
			continue // broken
		}
		if !sp.w.Animal(g).Dormant || sp.m.ColonistNear(x, y, guardWake) {
			return true
		}
	}
	return false
}

// wakeAll wakes p's guards, for good.
func (sp *ShipParts) wakeAll(p *ShipPart, now uint64) {
	p.Awake = true
	woke := 0
	for _, g := range p.Guards {
		if _, _, ok := sp.h.Pos(g); !ok {
			continue
		}
		a := sp.w.Animal(g)
		if a.Dormant {
			wake(&a, now)
			sp.w.SetAnimal(g, a)
		}
		woke++
	}
	if woke > 0 {
		mx, my := p.middle()
		sp.h.Threat("Mechanoids awake", mx, my, "The ship part's guards have woken!")
	}
}

// Hit is building id taking damage at tick now (destroyed: it broke). A
// ship part's guards wake; a destroyed part leaves its salvage and frees
// its guards to go for the colony. Any other building: nothing.
func (sp *ShipParts) Hit(id entity.EntityID, destroyed bool, now uint64) {
	i := slices.IndexFunc(sp.parts, func(p ShipPart) bool { return p.ID == id })
	if i < 0 {
		return
	}
	p := &sp.parts[i]
	if !p.Awake {
		sp.wakeAll(p, now)
	}
	if !destroyed {
		return
	}
	for j, s := range shipSalvage {
		sp.m.Drop(s.kind, s.qty, p.X+j, p.Y+1)
	}
	for _, g := range p.Guards {
		if r, ok := sp.mechs.recs[g]; ok {
			r.Guard = false
			sp.mechs.recs[g] = r
		}
	}
	mx, my := p.middle()
	sp.h.Message(mx, my, "The ship part breaks apart. Its plants will grow back.")
	sp.parts = slices.Delete(sp.parts, i, i+1)
}

// BlightRadius is the blight's radius age ticks after the crash. wiki:
// Defoliator ship (5 tiles growing to 100 over 20 days).
func BlightRadius(age uint64) int {
	grow := uint64(blightMax-blightStart) * age / (blightDays * clock.TicksPerDay)
	return min(blightStart+int(grow), blightMax)
}

// blight withers the plants on this hour's band of rows of p's disc.
func (sp *ShipParts) blight(p *ShipPart, now uint64) {
	r := BlightRadius(now - p.Landed)
	cx, cy := p.middle()
	w, h := sp.h.Size()
	band := int(now/clock.TicksPerHour) % blightBands
	for y := max(cy-r, 0); y <= min(cy+r, h-1); y++ {
		if y%blightBands != band { // a row's band is fixed, whatever the radius
			continue
		}
		dy := y - cy
		for x := max(cx-r, 0); x <= min(cx+r, w-1); x++ {
			if dx := x - cx; dx*dx+dy*dy <= r*r {
				sp.m.KillPlant(x, y)
			}
		}
	}
}

// Parts is how many ship parts stand (the threat's own check).
func (sp *ShipParts) Parts() int { return len(sp.parts) }

func cloneParts(ps []ShipPart) []ShipPart {
	if len(ps) == 0 {
		return nil
	}
	out := make([]ShipPart, len(ps))
	for i, p := range ps {
		out[i] = p
		out[i].Guards = slices.Clone(p.Guards)
	}
	return out
}
