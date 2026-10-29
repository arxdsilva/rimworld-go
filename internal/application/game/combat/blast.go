package combat

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Blasts are explosions (DEF-2): a grenade's or launcher's shot bursting
// where it lands (later IEDs, mortar shells and rockets). A blast reaches
// the cells within its radius that the centre can see (rock, walls and
// closed doors shelter what is behind them, and take the blast
// themselves): creatures there are wounded through their armor by the
// distance-scaled damage (explosion.Def.At), buildings damaged (bomb
// damage doubled), a flame blast sets the centre alight, an EMP stuns the
// sentry turrets it reaches, and smoke leaves a cloud that spoils aim
// through it. Every roll is on the main stream and the creatures are
// taken in id order, so a blast is the same on every run.
type Blasts struct {
	h       Host
	sight   *Sight
	st      *Strikes
	turrets *Turrets
	seen    []Seen            // Near's reused buffer
	hit     []blastBuilding   // the buildings one blast reaches, reused
	smoke   []explosion.Cloud // live smoke clouds (saved: Export)
}

// blastBuilding is a building a blast reaches and its distance
// (thousandths of a tile).
type blastBuilding struct {
	id   entity.EntityID
	dist int
}

// Explode bursts d at (x, y), set off by by (0: nobody) with weapon wk.
func (b *Blasts) Explode(by entity.EntityID, wk weapon.Kind, x, y int, d explosion.Def) {
	h := b.h
	now := h.Now()
	h.Burst(x, y, now, d)
	if d.Ignites() {
		h.Ignite(x, y)
	}
	if d.Kind == explosion.Smoke {
		b.smoke = append(b.smoke, explosion.Cloud{X: x, Y: y, Radius: d.Radius, Until: now + explosion.SmokeTicks})
	}
	if d.Flesh() {
		b.creatures(by, wk, x, y, d)
	}
	b.buildings(x, y, d, now)
}

// reaches: the blast at (x, y) reaches the cell (tx, ty): inside its
// radius and in sight of the centre.
func (b *Blasts) reaches(x, y, tx, ty int, d explosion.Def) bool {
	return d.Within(tx-x, ty-y) && (tx == x && ty == y || b.sight.Line(x, y, tx, ty))
}

// creatures wounds every living creature the blast reaches, in id order.
func (b *Blasts) creatures(by entity.EntityID, wk weapon.Kind, x, y int, d explosion.Def) {
	h := b.h
	b.seen = h.Near(x, y, d.Reach(), Everyone, b.seen[:0])
	slices.SortFunc(b.seen, func(p, q Seen) int { return cmp.Compare(p.ID, q.ID) })
	// a fresh copy: wounds may set off another blast that reuses seen
	hit := slices.Clone(b.seen)
	for _, s := range hit {
		if !b.reaches(x, y, s.X, s.Y, d) || !b.alive(s.ID) {
			continue
		}
		dmg := d.At(explosion.Dist(s.X-x, s.Y-y))
		res := b.st.armorRoll(s.ID, d.Type(), dmg, d.Pen, h.Roll)
		h.LogStrike(by, s.ID, wk, true, res.Damage)
		b.wound(by, s.ID, x, y, s.X, s.Y, d, res)
	}
}

// alive: id still has a tile and is not a carcass.
func (b *Blasts) alive(id entity.EntityID) bool {
	if a, ok := b.h.Animal(id); ok && a.Dead() {
		return false
	}
	_, _, ok := b.h.Pos(id)
	return ok
}

// wound applies what got through armor: shrapnel cuts, flame burns.
func (b *Blasts) wound(by, target entity.EntityID, x, y, tx, ty int, d explosion.Def, res armor.Result) {
	h := b.h
	if res.Damage <= 0 {
		return
	}
	kind := body.Cut
	switch {
	case d.Type() == damage.Heat:
		kind = body.Burn
	case res.Type == damage.Blunt:
		kind = body.Bruise
	}
	h.Hit(target, h.Now(), x, y, res.Damage)
	h.Stain(target, x, y, tx, ty, res.Damage)
	h.HurtAs(target, res.Damage, by, kind, health.CauseInjury)
}

// buildings damages (or, for an EMP, stuns) the object-layer buildings
// the blast reaches, each once, in scan order.
func (b *Blasts) buildings(x, y int, d explosion.Def, now uint64) {
	h := b.h
	if d.Building(0) == 0 && d.Kind != explosion.EMP {
		return
	}
	b.hit = b.hit[:0]
	r := d.Reach()
	for cy := y - r; cy <= y+r; cy++ {
		for cx := x - r; cx <= x+r; cx++ {
			if !h.InBounds(cx, cy) || !b.reaches(x, y, cx, cy, d) {
				continue
			}
			id, _ := h.Object(cx, cy)
			if id == 0 || slices.ContainsFunc(b.hit, func(o blastBuilding) bool { return o.id == id }) {
				continue
			}
			b.hit = append(b.hit, blastBuilding{id, explosion.Dist(cx-x, cy-y)})
		}
	}
	hit := slices.Clone(b.hit) // a destroyed turret's own blast runs inside
	for _, o := range hit {
		if d.Kind == explosion.EMP {
			if _, ok := b.turrets.At(o.id); ok {
				b.turrets.Stun(o.id, now+uint64(d.StunTicks(o.dist)))
			}
			continue
		}
		h.DamageBuilding(o.id, d.Building(o.dist))
	}
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

// Import restores saved smoke clouds.
func (b *Blasts) Import(saved []explosion.Cloud) { b.smoke = append(b.smoke[:0], saved...) }
