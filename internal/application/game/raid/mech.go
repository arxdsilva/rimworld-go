package raid

import (
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	raiddef "rimworld-go/internal/domain/raid"
	"rimworld-go/internal/domain/sim/entity"
)

// Mechanoids (DEF-4). Harrow, Spindle, Bulwark and Quill are hostile
// machines, not animals: they live in the game's animal store (they are
// creatures on the map, drawn and hit like one), but a mechanoid never
// runs the animal brain. It has no hunger, rest or taming, never flees and
// never goes down; it fights with its own weapon (domain animal.Mech),
// its plating is armor, an EMP blast stuns it (then it adapts), it does
// not bleed, and it leaves a wreck that shreds into salvage, not meat.
//
// Think is a mechanoid's tick, run from the game's animal phase in place
// of the animal brain. Like a raider it goes for the nearest colonist (or
// an ally, or a sentry turret well closer), closing until its weapon
// reaches, then attacks on its weapon's cycle with the weapon's burst.
// It looks for a new target only on its decision cycle
// (animal.DecideEvery, staggered by id) or when its target falls: never a
// scan of every creature per tick. A ship part's guard (shippart.go) holds
// near its post, taking on colonists within GuardLeash of it and letting
// them go beyond GuardRelease (hysteresis: no chase/return flapping).

const (
	// MechRaidMinPoints: no mechanoid raid below this many raid points.
	// wiki: Events (the mechanoid faction's raid weight is 0 until 300).
	MechRaidMinPoints = 300
	// mechGroupMax caps one group. original tuning
	mechGroupMax = 12
	// GuardLeash and GuardRelease: a woken guard takes on colonists this
	// close to its post and lets them go beyond the release. wiki:
	// Crashed ship part (awakened guards stay within about 30 tiles);
	// the release margin is original tuning.
	GuardLeash   = 30
	GuardRelease = 36
	// guardHome: an idle guard walks back once this far from its post.
	guardHome = 4
	// mechWanderEvery paces an idle raid mechanoid's walk to the colony.
	mechWanderEvery = 240
	// mechRepathEvery paces a chasing mechanoid's path to a moving target.
	mechRepathEvery = 30
)

// mechKinds are the mechanoids a group is drawn from, cheapest last.
var mechKinds = [...]animal.Species{animal.Harrow, animal.Spindle, animal.Bulwark, animal.Quill}

// MechHost is what the mechanoids need beyond Host and Wild (each value
// only, ≤ 8 methods).
type MechHost interface {
	// Shoot makes id attack target with its own weapon wk (combat's
	// Strikes: a blow at once, a gun's shot flies).
	Shoot(id, target entity.EntityID, wk weapon.Kind)
	// GoTo paths mechanoid id (state a) towards (x, y) and returns the
	// state with the path (a value: the animal phase's record must not
	// reach the heap through an interface call).
	GoTo(id entity.EntityID, a animal.State, x, y int) animal.State
	// Line: (tx, ty) is in sight of (x, y) (rock, walls and closed doors
	// block it).
	Line(x, y, tx, ty int) bool
	// MechsNear appends every mechanoid within Chebyshev r of (x, y), in
	// no particular order (the creature index: no scan of every creature).
	MechsNear(x, y, r int, buf []entity.EntityID) []entity.EntityID
	// ColonistNear: a standing colonist within Chebyshev r of (x, y) (the
	// creature index).
	ColonistNear(x, y, r int) bool
	// PlaceShipPart sets a ship part down whole with its anchor on (x, y)
	// (ok false: no room there).
	PlaceShipPart(x, y int) (entity.EntityID, bool)
	// KillPlant withers the plant on (x, y), if there is one.
	KillPlant(x, y int)
	// Drop leaves qty of kind on (x, y), the colony's to haul.
	Drop(kind item.Kind, qty, x, y int)
}

// MechRecord is one mechanoid's state beyond its animal record (saved:
// MechSave). Field names are gob names: never rename.
type MechRecord struct {
	// StunUntil: an EMP stunned it until this tick; AdaptUntil: it shrugs
	// off EMP until this tick (wiki: Mechanoid, 2200 ticks).
	StunUntil, AdaptUntil uint64
	// Shots left in the burst under way (0: none).
	Shots int
	// Guard: a ship part's guard, posted at (PostX, PostY).
	Guard        bool
	PostX, PostY int
}

// Mechs runs the mechanoids.
type Mechs struct {
	h     Host
	w     Wild
	m     MechHost
	recs  map[entity.EntityID]MechRecord
	ships *ShipParts
	near  []entity.EntityID // EMP's reused buffer
}

// newMechs builds the mechanoids and the ship parts that bring them.
func newMechs(h Host, r Roles) (*Mechs, *ShipParts) {
	ms := &Mechs{h: h, w: r.Wild, m: r.Mech, recs: make(map[entity.EntityID]MechRecord)}
	ms.ships = &ShipParts{h: h, w: r.Wild, m: r.Mech, mechs: ms}
	return ms, ms.ships
}

// Think is mechanoid id's tick (state a, written back by the caller).
func (ms *Mechs) Think(id entity.EntityID, a *animal.State, now uint64) {
	x, y, ok := ms.h.Pos(id)
	m, mech := animal.MechOf(a.Species)
	if !ok || !mech {
		return
	}
	r := ms.recs[id]
	ms.think(id, a, &r, m, x, y, now)
	ms.recs[id] = r
}

// think is Think on mechanoid id's record r (m its row, (x, y) its tile).
func (ms *Mechs) think(id entity.EntityID, a *animal.State, r *MechRecord, m animal.Mech, x, y int, now uint64) {
	a.Errand, a.Mode, a.Manhunter = false, animal.Attack, HiveForever
	if now < r.StunUntil { // EMP: it stands dead still until it reboots
		a.Path, a.PathI = nil, 0
		return
	}
	wd := weapon.DefOf(m.Weapon)
	if a.Target != 0 && !ms.holds(a.Target, *r) {
		a.Target, r.Shots = 0, 0
	}
	if a.Target == 0 || (now+uint64(id))%animal.DecideEvery == 0 {
		// a fallen target was dropped above; one still worth fighting is
		// kept unless another is found (a guard's leash hysteresis)
		if t := ms.pick(x, y, *r); t != a.Target && (t != 0 || a.Target == 0) {
			a.Target, r.Shots = t, 0
			a.Cooldown = max(a.Cooldown, wd.Warmup) // aims at a new target
		}
	}
	if a.Target == 0 {
		ms.idle(id, a, x, y, *r, now)
		return
	}
	tx, ty, ok := ms.targetAt(a.Target, x, y)
	if !ok {
		a.Target = 0
		return
	}
	if ms.h.CanHit(x, y, wd, tx, ty) {
		a.Path, a.PathI = nil, 0
		if a.Cooldown == 0 {
			ms.fire(id, a, r, m.Weapon, wd)
		}
		return
	}
	if now < a.RetryAt && a.PathI >= len(a.Path) {
		return // backing off after a failed path search
	}
	if a.PathI >= len(a.Path) || weapon.Distance(a.GoalX, a.GoalY, tx, ty) > 1 || (now+uint64(id))%mechRepathEvery == 0 {
		*a = ms.m.GoTo(id, *a, tx, ty)
	}
}

// fire is one shot (or blow) of the burst under way, then the wait: the
// burst's gap, or the weapon's cycle once the burst is spent.
func (ms *Mechs) fire(id entity.EntityID, a *animal.State, r *MechRecord, wk weapon.Kind, wd weapon.Def) {
	if r.Shots <= 0 {
		r.Shots = max(wd.Burst, 1)
	}
	ms.w.SetAnimal(id, *a) // the attack reads and may touch the stored record
	ms.m.Shoot(id, a.Target, wk)
	*a = ms.w.Animal(id)
	if r.Shots--; r.Shots > 0 {
		a.Cooldown = max(wd.BurstTicks, 1)
		return
	}
	a.Cooldown = wd.Cooldown
	if wd.Ranged {
		a.Cooldown += wd.Warmup
	}
}

// pick is the target for a mechanoid at (x, y): the nearest colonist or
// ally (a guard's only within GuardLeash of its post), or a sentry turret
// in sight and well closer.
func (ms *Mechs) pick(x, y int, r MechRecord) entity.EntityID {
	t, tx, ty, ok := ms.h.RaidTarget(x, y)
	if ok && r.Guard && weapon.Distance(r.PostX, r.PostY, tx, ty) > GuardLeash {
		ok = false
	}
	cd := 1 << 20
	if ok {
		cd = weapon.Distance(x, y, tx, ty)
	} else {
		t = 0
	}
	if b, _, _, tok := ms.h.TurretFor(x, y, cd, SightRange); tok && (!r.Guard || ms.nearPost(r, x, y)) {
		return b
	}
	return t
}

// nearPost: (x, y) is within GuardLeash of r's post.
func (ms *Mechs) nearPost(r MechRecord, x, y int) bool {
	return weapon.Distance(r.PostX, r.PostY, x, y) <= GuardLeash
}

// holds: target t is still worth fighting: a creature standing (a guard's
// within GuardRelease of its post), or a building (a turret: targetAt
// drops it once it is gone).
func (ms *Mechs) holds(t entity.EntityID, r MechRecord) bool {
	tx, ty, ok := ms.h.Pos(t)
	if !ok {
		return true // a turret: targetAt drops it once it is gone
	}
	if ms.h.Vital(t).Status != health.Alive {
		return false
	}
	return !r.Guard || weapon.Distance(r.PostX, r.PostY, tx, ty) <= GuardRelease
}

// targetAt is target t's tile: a creature's, or the turret's that pick
// found (asked again from the mechanoid's tile).
func (ms *Mechs) targetAt(t entity.EntityID, x, y int) (int, int, bool) {
	if tx, ty, ok := ms.h.Pos(t); ok {
		return tx, ty, true
	}
	if b, bx, by, ok := ms.h.TurretFor(x, y, 1<<20, SightRange); ok && b == t {
		return bx, by, true
	}
	return 0, 0, false
}

// idle walks a mechanoid with no one to fight: a guard back to its post,
// a raider on to the colony.
func (ms *Mechs) idle(id entity.EntityID, a *animal.State, x, y int, r MechRecord, now uint64) {
	if a.PathI < len(a.Path) || now < a.NextThink {
		return
	}
	a.NextThink = now + mechWanderEvery + uint64(ms.h.Roll(mechWanderEvery))
	if r.Guard {
		if weapon.Distance(x, y, r.PostX, r.PostY) > guardHome {
			*a = ms.m.GoTo(id, *a, r.PostX, r.PostY)
		}
		return
	}
	hx, hy := ms.h.Home()
	if weapon.Distance(x, y, hx, hy) > guardHome {
		*a = ms.m.GoTo(id, *a, hx, hy)
	}
}

// EMP is an EMP blast d at (x, y) on tick now (every explosion reaches the
// game's burst; only an EMP does anything here): every mechanoid inside
// the radius and in sight of the centre is stunned for the blast's stun
// at its distance, unless it is still adapted from the last one; then it
// adapts. A dormant guard it reaches wakes. Mechanoids are taken in id
// order.
func (ms *Mechs) EMP(x, y int, d explosion.Def, now uint64) {
	if d.Kind != explosion.EMP {
		return
	}
	ms.near = ms.m.MechsNear(x, y, d.Reach(), ms.near[:0])
	slices.Sort(ms.near)
	for _, id := range ms.near {
		mx, my, ok := ms.h.Pos(id)
		if !ok || !d.Within(mx-x, my-y) || (mx != x || my != y) && !ms.m.Line(x, y, mx, my) {
			continue
		}
		ticks := d.StunTicks(explosion.Dist(mx-x, my-y))
		r := ms.recs[id]
		if ticks <= 0 || now < r.AdaptUntil {
			continue
		}
		r.StunUntil, r.AdaptUntil, r.Shots = now+uint64(ticks), now+animal.EMPAdaptTicks, 0
		ms.recs[id] = r
		a := ms.w.Animal(id)
		a.Path, a.PathI = nil, 0
		wake(&a, now)
		ms.w.SetAnimal(id, a)
	}
}

// wake rouses a dormant mechanoid: from now on it fights.
func wake(a *animal.State, now uint64) {
	if !a.Dormant {
		return
	}
	a.Dormant = false
	a.Manhunter, a.Mode = HiveForever, animal.Attack
	a.NextThink, a.NextDecide = now, now
}

// Stunned: mechanoid id is stunned by an EMP at tick now (the read model).
func (ms *Mechs) Stunned(id entity.EntityID, now uint64) bool { return now < ms.recs[id].StunUntil }

// Forget drops a mechanoid that left the map or broke.
func (ms *Mechs) Forget(id entity.EntityID) { delete(ms.recs, id) }

// Group spends points on a mechanoid group: a kind it can afford at a
// time, drawn on the main stream, until it can afford none (at least one,
// at most mechGroupMax). wiki: Mechanoid (combat power per kind).
func (ms *Mechs) Group(points int) []animal.Species {
	var out []animal.Species
	var fit [len(mechKinds)]animal.Species
	for len(out) < mechGroupMax {
		n := 0
		for _, s := range mechKinds {
			if m, _ := animal.MechOf(s); m.CombatPower <= points {
				fit[n] = s
				n++
			}
		}
		if n == 0 {
			break
		}
		s := fit[ms.h.Roll(n)]
		m, _ := animal.MechOf(s)
		points -= m.CombatPower
		out = append(out, s)
	}
	if len(out) == 0 {
		out = append(out, animal.Quill) // the cheapest
	}
	return out
}

// Raid sends a mechanoid group worth points in by arrival (drop pods land
// on the colony, at an edge or scattered round it; walking in, from an
// edge) at tick now. It returns how many came.
func (ms *Mechs) Raid(points int, arr raiddef.Arrival, now uint64) int {
	ex, ey, ok := ms.landing(arr)
	if !ok {
		return 0
	}
	n := 0
	for _, s := range ms.Group(points) {
		x, y := ex, ey
		if arr == raiddef.DropPodScatter {
			hx, hy := ms.h.Home()
			x, y = hx+ms.h.Roll(2*scatterRange+1)-scatterRange, hy+ms.h.Roll(2*scatterRange+1)-scatterRange
		}
		id, ok := ms.spawnNear(s, x, y)
		if !ok {
			continue
		}
		a := ms.w.Animal(id)
		a.Manhunter, a.Mode, a.NextDecide = HiveForever, animal.Attack, now
		ms.w.SetAnimal(id, a)
		n++
	}
	if n > 0 {
		ms.h.Threat("Mechanoid raid", ex, ey, "War machines are coming for the colony! An EMP blast will stun them for a while.")
	}
	return n
}

// scatterRange: scattered drop pods land this far round the colony.
// original tuning
const scatterRange = 15

// landing is where a group arriving by arr comes down: the colony's
// centre, or an edge the colony can be reached from.
func (ms *Mechs) landing(arr raiddef.Arrival) (int, int, bool) {
	switch arr {
	case raiddef.DropPodCentre, raiddef.DropPodScatter:
		hx, hy := ms.h.Home()
		return ms.h.WalkableNear(hx, hy, guardHome+2)
	}
	return ms.w.EdgeTile()
}

// spawnNear places s on a walkable tile within two of (x, y).
func (ms *Mechs) spawnNear(s animal.Species, x, y int) (entity.EntityID, bool) {
	for try := 0; try < 12; try++ {
		sx, sy := x+ms.h.Roll(5)-2, y+ms.h.Roll(5)-2
		if !ms.h.InBounds(sx, sy) || ms.h.Rock(sx, sy) {
			continue
		}
		if id, err := ms.w.SpawnAnimal(s, sx, sy); err == nil {
			return id, true
		}
	}
	return 0, false
}

// MechSave is the mechanoids' and ship parts' saved state.
type MechSave struct {
	Mechs map[entity.EntityID]MechRecord
	Parts []ShipPart
}

// Export is the state for the save (nil when there is none).
func (ms *Mechs) Export() *MechSave {
	if len(ms.recs) == 0 && len(ms.ships.parts) == 0 {
		return nil
	}
	s := &MechSave{Parts: cloneParts(ms.ships.parts)}
	if len(ms.recs) > 0 {
		s.Mechs = make(map[entity.EntityID]MechRecord, len(ms.recs))
		for id, r := range ms.recs {
			s.Mechs[id] = r
		}
	}
	return s
}

// Import restores saved state (nil: none, as an older save).
func (ms *Mechs) Import(s *MechSave) {
	clear(ms.recs)
	ms.ships.parts = nil
	if s == nil {
		return
	}
	for id, r := range s.Mechs {
		ms.recs[id] = r
	}
	ms.ships.parts = cloneParts(s.Parts)
}
