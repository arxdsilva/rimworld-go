package combat

import (
	"cmp"
	"fmt"
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/fire"
	"rimworld-go/internal/domain/world/plant"
)

// Blasts are explosions (DEF-2): a grenade's or launcher's shot bursting
// where it lands (later IEDs, mortar shells and rockets). A blast reaches
// the cells within its radius that the centre can see (rock, walls and
// closed doors shelter what is behind them, and take the blast
// themselves): creatures there are wounded through their armor by the
// distance-scaled damage (explosion.Def.At), buildings damaged by what
// they are (explosion.Def.Against: a bomb hits passable buildings x2,
// impassable ones x4 and walls x8), an EMP stuns the sentry turrets it
// reaches (unless they adapted to the last one), and smoke leaves a cloud
// that spoils aim through it. CBT-5: a bomb also wears the loose stacks
// and tears the plants it reaches, a flame blast lights every cell it
// reaches by what burns there (and the creatures standing in it), and the
// shells and grenades either reaches cook off in turn (the fuse queue,
// Traps). Every roll is on the main stream and the creatures are taken in
// id order and the cells in rows, so a blast is the same on every run.
type Blasts struct {
	h       Host
	sight   *Sight
	st      *Strikes
	turrets *Turrets
	debris  Debris            // CBT-5: loose stacks, plants and fuel (nil: blasts leave them be)
	seen    []Seen            // Near's reused buffer
	hit     []blastBuilding   // the buildings one blast reaches, reused
	cells   []blastCell       // the cells one blast reaches, reused
	loose   []entity.EntityID // Debris.Loose's reused buffer
	smoke   []explosion.Cloud // live smoke clouds (saved: Export)
	traps   *Traps            // DEF-4: a blast lights the buried charges it reaches
}

// Debris is what a blast reaches on the ground besides creatures and
// buildings (CBT-5): loose stacks, plants and whatever burns. A
// consumer-owned role (Roles.Debris); every method passes values.
type Debris interface {
	// Loose appends the stacks lying loose on (x, y) to buf, in item
	// order (the ground index: no scan).
	Loose(x, y int, buf []entity.EntityID) []entity.EntityID
	// Stack is stack id (ok false: gone).
	Stack(id entity.EntityID) (item.Stack, bool)
	// PutStack stores stack id's new state (its wear); DropStack destroys
	// it.
	PutStack(id entity.EntityID, st item.Stack)
	DropStack(id entity.EntityID)
	// Fuel is the best flammability (permille) on (x, y): its plant,
	// grass, buildings and loose stacks (0: nothing burns there).
	Fuel(x, y int) int
	// Kindle lights a fire on (x, y), fuel or not (with nothing to burn
	// it soon dies out).
	Kindle(x, y int)
	// HarmPlant adds pct percent damage to the plant on (x, y): it dies
	// at plant.DamageDead.
	HarmPlant(x, y, pct int)
}

// blastBuilding is a building a blast reaches, its kind and its distance
// (thousandths of a tile).
type blastBuilding struct {
	id   entity.EntityID
	kind construction.Kind
	dist int
}

// blastCell is a cell a blast reaches and its distance (thousandths of a
// tile).
type blastCell struct{ x, y, dist int }

// Explode bursts d at (x, y), set off by by (0: nobody) with weapon wk.
func (b *Blasts) Explode(by entity.EntityID, wk weapon.Kind, x, y int, d explosion.Def) {
	h := b.h
	now := h.Now()
	h.Burst(x, y, now, d)
	if d.Kind == explosion.Smoke {
		b.smoke = append(b.smoke, explosion.Cloud{X: x, Y: y, Radius: d.Radius, Until: now + explosion.SmokeTicks})
	}
	var hit []Seen
	if d.Flesh() {
		hit = b.creatures(by, wk, x, y, d)
	}
	if !d.Structural() && d.Kind != explosion.EMP {
		return
	}
	// a fresh copy: a destroyed turret's own blast runs inside and reuses
	// the buffer
	cells := slices.Clone(b.reach(x, y, d))
	b.buildings(cells, d, now)
	if b.debris != nil && d.Kind != explosion.EMP {
		b.ground(x, y, cells, d, hit)
	}
}

// reaches: the blast at (x, y) reaches the cell (tx, ty): inside its
// radius and in sight of the centre.
func (b *Blasts) reaches(x, y, tx, ty int, d explosion.Def) bool {
	return d.Within(tx-x, ty-y) && (tx == x && ty == y || b.sight.Line(x, y, tx, ty))
}

// reach lists the cells the blast at (x, y) reaches, in rows (the reused
// buffer).
func (b *Blasts) reach(x, y int, d explosion.Def) []blastCell {
	h := b.h
	b.cells = b.cells[:0]
	r := d.Reach()
	for cy := y - r; cy <= y+r; cy++ {
		for cx := x - r; cx <= x+r; cx++ {
			if h.InBounds(cx, cy) && b.reaches(x, y, cx, cy, d) {
				b.cells = append(b.cells, blastCell{cx, cy, explosion.Dist(cx-x, cy-y)})
			}
		}
	}
	return b.cells
}

// creatures wounds every living creature the blast reaches, in id order,
// and reports them.
func (b *Blasts) creatures(by entity.EntityID, wk weapon.Kind, x, y int, d explosion.Def) []Seen {
	h := b.h
	b.seen = h.Near(x, y, d.Reach(), Everyone, b.seen[:0])
	slices.SortFunc(b.seen, func(p, q Seen) int { return cmp.Compare(p.ID, q.ID) })
	// a fresh copy: wounds may set off another blast that reuses seen
	hit := slices.Clone(b.seen)
	n := 0
	for _, s := range hit {
		if !b.reaches(x, y, s.X, s.Y, d) || !b.alive(s.ID) {
			continue
		}
		hit[n] = s
		n++
		dmg := d.At(explosion.Dist(s.X-x, s.Y-y))
		l := h.Absorb(s.ID, damage.Hit{Weapon: weapon.DefOf(wk).Name, Form: damage.FormOf(d.Type()), Amount: dmg, Pen: d.Pen}, h.Roll)
		h.LogStrike(by, s.ID, wk, true, l.Amount)
		b.wound(by, s.ID, x, y, s.X, s.Y, l)
	}
	return hit[:n]
}

// alive: id still has a tile and is not a carcass.
func (b *Blasts) alive(id entity.EntityID) bool {
	if a, ok := b.h.Animal(id); ok && a.Dead() {
		return false
	}
	_, _, ok := b.h.Pos(id)
	return ok
}

// wound applies what got through armor (CBT-1: the hit's form picks the
// wound: shrapnel cuts, flame burns).
func (b *Blasts) wound(by, target entity.EntityID, x, y, tx, ty int, l armor.Landed) {
	h := b.h
	if l.Stopped() {
		return
	}
	h.Hit(target, h.Now(), x, y, l.Amount)
	h.Stain(target, x, y, tx, ty, l.Amount)
	h.HurtHit(target, l, by)
}

// buildings damages (or, for an EMP, stuns) the object-layer buildings
// on cells, each once, in scan order.
func (b *Blasts) buildings(cells []blastCell, d explosion.Def, now uint64) {
	h := b.h
	b.hit = b.hit[:0]
	for _, c := range cells {
		id, k := h.Object(c.x, c.y)
		if id == 0 || slices.ContainsFunc(b.hit, func(o blastBuilding) bool { return o.id == id }) {
			continue
		}
		b.hit = append(b.hit, blastBuilding{id, k, c.dist})
	}
	hit := slices.Clone(b.hit) // a destroyed turret's own blast runs inside
	for _, o := range hit {
		if d.Kind == explosion.EMP {
			if _, ok := b.turrets.At(o.id); ok && !explosion.Adapted(b.turrets.aims[o.id].StunnedUntil, now) {
				b.turrets.Stun(o.id, now+uint64(d.StunTicks(o.dist)))
			}
			continue
		}
		if bd, ok := h.Building(o.id); ok && b.traps != nil && b.traps.chain(o.id, bd.Kind, bd.X, bd.Y) {
			continue // DEF-4: a buried charge goes off in turn
		}
		h.DamageBuilding(o.id, d.Against(o.dist, targetOf(o.kind)))
	}
}

// targetOf is what a building of kind k is to a blast (wiki: Damage
// types): a wall, a building that blocks movement, or one creatures pass.
func targetOf(k construction.Kind) explosion.Target {
	switch {
	case k == construction.Wall:
		return explosion.Wall
	case construction.DefOf(k).BlocksMove:
		return explosion.Impassable
	}
	return explosion.Passable
}

// ground is what the blast at (x, y) does on cells beyond creatures and
// buildings (CBT-5): it wears or destroys loose stacks, sets off shells
// and grenades, tears plants, and a flame blast lights the cells by what
// burns there (hit: the creatures it reached, whose cells catch by
// explosion.CreatureFlammability; the centre always catches).
func (b *Blasts) ground(x, y int, cells []blastCell, d explosion.Def, hit []Seen) {
	plants := d.Against(0, explosion.Plant) > 0 && b.sight.f != nil
	for _, c := range cells {
		b.stacks(c, d)
		if plants {
			if s, _, ok := b.sight.f.Plant(c.x, c.y); ok {
				if pct := d.PlantPct(c.dist, plant.DefOf(s).Trunk); pct > 0 {
					b.debris.HarmPlant(c.x, c.y, pct)
				}
			}
		}
		if d.Ignites() {
			b.kindle(c, c.x == x && c.y == y, hit)
		}
	}
}

// kindle lights cell c (always at the centre) by the chance of what burns
// there: its fuel, or a creature standing in it.
func (b *Blasts) kindle(c blastCell, centre bool, hit []Seen) {
	chance := b.debris.Fuel(c.x, c.y)
	if slices.ContainsFunc(hit, func(s Seen) bool { return s.X == c.x && s.Y == c.y }) {
		chance = max(chance, explosion.CreatureFlammability)
	}
	if centre || chance > 0 && b.h.Roll(1000) < chance {
		b.debris.Kindle(c.x, c.y)
	}
}

// stacks wears the loose stacks on cell c by the blast's item damage
// (made things lose hit points and go when none are left; raw stacks that
// burn go to a blow of explosion.LooseHP) and sets off the shells and
// grenades a bomb or flame reaches.
func (b *Blasts) stacks(c blastCell, d explosion.Def) {
	db := b.debris
	b.loose = db.Loose(c.x, c.y, b.loose[:0])
	dmg := d.Item(c.dist)
	for _, id := range b.loose {
		st, ok := db.Stack(id)
		if !ok {
			continue
		}
		if _, cooks := cookOff(st.Kind); cooks {
			if d.Lights() {
				b.traps.cook(id, c.x, c.y)
			}
			continue
		}
		switch {
		case dmg == 0:
		case st.MaxHP() > 0:
			if st.Wear += dmg; st.WornOut() {
				db.DropStack(id)
			} else {
				db.PutStack(id, st)
			}
		case dmg >= explosion.LooseHP && fire.ItemFlammability(st.Kind) > 0:
			db.DropStack(id)
		}
	}
}

// cookOff is the weapon whose blast a stack of k goes off with when a
// blast or a fire sets it off (ok false: it does not): a mortar shell
// bursts as the mortar's, a grenade as one thrown.
func cookOff(k item.Kind) (weapon.Kind, bool) {
	switch k {
	case item.FragGrenades:
		return weapon.FragGrenade, true
	case item.MolotovCocktails:
		return weapon.Molotov, true
	case item.EMPGrenades:
		return weapon.EMPGrenade, true
	}
	return weapon.ForShell(k)
}

// InFire sets off the shells and grenades lying loose on the burning
// cell (x, y) (the game's fire step, per burning cell: one ground-index
// lookup). Each goes off explosion.CookOffFuse ticks later.
func (b *Blasts) InFire(x, y int) {
	if b.debris == nil {
		return
	}
	b.loose = b.debris.Loose(x, y, b.loose[:0])
	for _, id := range b.loose {
		if st, ok := b.debris.Stack(id); ok {
			if _, cooks := cookOff(st.Kind); cooks {
				b.traps.cook(id, x, y)
			}
		}
	}
}

// cookOffNow sets off the stack fuse f names: gone, used or carried off
// meanwhile, nothing happens; otherwise the stack goes and bursts where
// it lies, as one of its own.
func (b *Blasts) cookOffNow(f Fuse) {
	st, ok := b.debris.Stack(f.ID)
	if !ok || st.Carrier != 0 {
		return
	}
	wk, ok := cookOff(st.Kind)
	if !ok {
		return
	}
	b.debris.DropStack(f.ID)
	b.h.Message(st.X, st.Y, fmt.Sprintf("The %s cooked off.", st.Kind))
	b.Explode(0, wk, st.X, st.Y, weapon.DefOf(wk).Blast)
}

// SmokeFactor is the aim factor (permille) for a shot from (ax, ay) to
// (tx, ty): attack.SmokeFactor through a live smoke cloud, else neutral.
func (b *Blasts) SmokeFactor(ax, ay, tx, ty int) int {
	now := b.h.Now()
	for _, c := range b.smoke {
		if c.Until > now && c.Crosses(ax, ay, tx, ty) {
			return attack.SmokeFactor
		}
	}
	return 1000
}

// prune drops the smoke clouds that have thinned away.
func (b *Blasts) prune(now uint64) {
	if len(b.smoke) == 0 {
		return
	}
	b.smoke = slices.DeleteFunc(b.smoke, func(c explosion.Cloud) bool { return c.Until <= now })
}

// Export is the smoke clouds for the save (nil when none).
func (b *Blasts) Export() []explosion.Cloud {
	if len(b.smoke) == 0 {
		return nil
	}
	return slices.Clone(b.smoke)
}

// Views is the live smoke clouds for the world view (CBT-5: the renderer
// draws each one until Until, explosion.SmokeTicks after its burst; nil
// when none).
func (b *Blasts) Views() []explosion.Cloud { return b.Export() }

// Import restores saved smoke clouds.
func (b *Blasts) Import(saved []explosion.Cloud) { b.smoke = append(b.smoke[:0], saved...) }
