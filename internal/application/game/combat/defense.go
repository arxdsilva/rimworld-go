package combat

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Defences. Raiders and wild animals cannot open the colony's doors, so a
// closed-off colony makes raiders bash through the nearest door or wall
// (Breach). Spike traps spring on raiders and predators, then are re-laid
// as blueprints (Traps). Sandbags and wall corners give cover through
// Sight.Cover; building damage and repair stay on the game.

const (
	// BreachCooldown is the ticks between blows on a door, wall or rock.
	BreachCooldown = 60
	// TrapDamage is the injury a sprung spike trap deals.
	TrapDamage = 32
	// trapChance is the percent chance a raider or predator springs a trap
	// it walks over.
	trapChance = 75
	// buttDamage is a gun's blow on a door or wall (rifle butt).
	buttDamage = 7
	// digMin is a sapper's least blow on rock.
	digMin = 10
)

// Traps are the spike traps and the buried charges (IEDs, DEF-3:
// ied.go). The colony's own people never set them off: they know where
// their traps are.
type Traps struct {
	h      Host
	blasts *Blasts
	fuses  []Fuse // charges burning down (saved)
}

// At is the built trap (spike trap or charge) on (x, y) (0: none).
func (t *Traps) At(x, y int) entity.EntityID {
	if bid, k := t.h.Object(x, y); bid != 0 && isTrap(k) {
		return bid
	}
	return 0
}

// isTrap reports a trap kind: the spike trap and the four charges.
func isTrap(k construction.Kind) bool { return k == construction.SpikeTrap || isIED(k) }

// Has reports a trap built anywhere (a building scan: the animal step
// checks it only for a trap victim).
func (t *Traps) Has() bool { return t.h.AnyBuilt(isTrap) }

// Spring: a raider or predator stepped on (x, y); a spike trap there
// catches it on a roll of the main stream and is re-laid as a blueprint; a
// charge lights its fuse (ied.go).
func (t *Traps) Spring(victim entity.EntityID, x, y int) {
	h := t.h
	tid := t.At(x, y)
	if tid == 0 {
		return
	}
	if _, k := h.Object(x, y); isIED(k) {
		t.light(tid, k, x, y)
		return
	}
	if h.Roll(100) >= trapChance {
		return
	}
	b, _ := h.Building(tid)
	who := h.Name(victim)
	if a, ok := h.Animal(victim); ok {
		who = "a " + a.Species.String()
	}
	h.Message(x, y, fmt.Sprintf("A spike trap caught %s.", who))
	h.Trace(projectile.Trace{FromX: x, FromY: y, ToX: x, ToY: y,
		Tick: h.Now(), Hit: true, Melee: true, Damage: TrapDamage})
	h.RemoveBuilding(tid)
	h.PlaceFrame(construction.SpikeTrap, b.Stuff, construction.South, x, y) // re-lay it
	h.Trapped(victim, TrapDamage)
	h.Hurt(victim, TrapDamage, 0)
}

// Underfoot springs traps under raiders and predators that just moved
// (one object lookup per mover: no building scan).
func (t *Traps) Underfoot(movers []entity.EntityID) {
	for _, id := range movers {
		if x, y, ok := t.h.Pos(id); ok {
			t.Spring(id, x, y)
		}
	}
}

// Victim reports animals that spring traps: wild predators and
// manhunters.
func (t *Traps) Victim(a animal.State) bool {
	return !a.Tame && (animal.DefOf(a.Species).Predator || a.Enraged(t.h.Now()))
}

// Breach is how raiders shut out of the colony break in.
type Breach struct {
	h Host
	b Breaches
}

// Start sends a raider that cannot reach (tx, ty) to break in: it plans a
// route as if walls and doors (and rock, for a sapper) were gone and
// attacks the first one on it.
func (br *Breach) Start(id entity.EntityID, tx, ty int) bool {
	h := br.h
	sx, sy, _ := h.Pos(id)
	path, ok := br.b.BreachRoute(id, tx, ty)
	if !ok {
		return false
	}
	q := br.b.Query(id)
	for _, p := range path {
		if !q.Walkable(p.X, p.Y) {
			bid, _ := h.Object(p.X, p.Y)
			if bid == 0 && !h.Rock(p.X, p.Y) {
				return false // a trap in the way: plan again later
			}
			rec := job.Record{Kind: job.Breach, Target: bid, DestX: p.X, DestY: p.Y, TX: sx, TY: sy, WorkLeft: BreachCooldown}
			return h.StartJob(id, rec)
		}
		sx, sy = p.X, p.Y
	}
	return false
}

// Do: a raider beside a door or wall hits it on cooldown (a sapper's job
// on rock has no target: dig).
func (br *Breach) Do(id entity.EntityID, j job.Record) {
	if j.Target == 0 {
		br.dig(id, j)
		return
	}
	h := br.h
	b, ok := h.Building(j.Target)
	if !ok {
		j.State = job.Complete
		h.SetJob(id, j)
		return
	}
	if j.State == job.Goto {
		return // still walking up (movement advances it)
	}
	px, py, _ := h.Pos(id)
	if !touches(px, py, b) {
		j.State, j.Fail = job.Fail, job.Unreachable
		h.SetJob(id, j)
		return
	}
	if j.WorkLeft > 0 {
		j.WorkLeft--
		h.SetJob(id, j)
		return
	}
	j.WorkLeft = BreachCooldown
	h.SetJob(id, j)
	d := weapon.DefOf(h.Weapon(id))
	dmg := d.Damage
	if d.Ranged {
		dmg = buttDamage
	}
	now := h.Now()
	h.Trace(projectile.Trace{FromX: px, FromY: py, ToX: b.X, ToY: b.Y,
		Tick: now, Hit: true, Melee: true, Damage: dmg, Building: true,
		Attacker: id, Weapon: d.Name})
	h.Struck(id, 0, now)
	h.DamageBuilding(j.Target, dmg)
}

// dig: a sapper beside a rock tile hacks at it on cooldown.
func (br *Breach) dig(id entity.EntityID, j job.Record) {
	h := br.h
	if !h.Rock(j.DestX, j.DestY) {
		j.State = job.Complete
		h.SetJob(id, j)
		return
	}
	if j.State == job.Goto {
		return
	}
	px, py, _ := h.Pos(id)
	if abs(px-j.DestX) > 1 || abs(py-j.DestY) > 1 {
		j.State, j.Fail = job.Fail, job.Unreachable
		h.SetJob(id, j)
		return
	}
	if j.WorkLeft > 0 {
		j.WorkLeft--
		h.SetJob(id, j)
		return
	}
	j.WorkLeft = BreachCooldown
	h.SetJob(id, j)
	d := weapon.DefOf(h.Weapon(id))
	dmg := max(digMin, d.Damage)
	now := h.Now()
	h.Trace(projectile.Trace{FromX: px, FromY: py, ToX: j.DestX, ToY: j.DestY,
		Tick: now, Hit: true, Melee: true, Damage: dmg, Building: true,
		Attacker: id, Weapon: d.Name})
	h.Struck(id, 0, now)
	h.DigRock(j.DestX, j.DestY, dmg)
}

// touches reports (x, y) next to (or on) a building's footprint.
func touches(x, y int, b construction.Building) bool {
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, c := range cells {
		if abs(c[0]-x) <= 1 && abs(c[1]-y) <= 1 {
			return true
		}
	}
	return false
}
