package combat

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// A badly hurt turret's fuse (MAP-21; wiki: Mini-turret, Autocannon turret,
// Uranium slug turret): the first fall below TurretFusePct of its hit points
// rolls TurretBlastPct on the defence stream. Armed, it sparks for
// TurretFuseTicks and blows up; destroyed before that, it blows up at once.
// Unarmed, it just breaks. The fall is the one roll: more wounds below the
// mark roll nothing, and only a repair above it makes a new one possible.
// The fuse is not stopped by a repair. The wiki's raiders running from the
// blast are left out.
const (
	TurretFusePct   = 20
	TurretFuseTicks = 240
)

// Sparking reports turret id's fuse burning.
func (t *Turrets) Sparking(id entity.EntityID) bool { return t.spark(id) >= 0 }

// spark is turret id's place among the burning fuses (-1: none).
func (t *Turrets) spark(id entity.EntityID) int {
	return slices.IndexFunc(t.sparks, func(f Fuse) bool { return f.ID == id })
}

// Hurt tells of a blow that took turret b from before hit points to what it
// has now.
func (t *Turrets) Hurt(id entity.EntityID, b construction.Building, before int) {
	if b.Kind == construction.FoamPopper || b.Kind == construction.FoamTurret {
		return // AX-5: foam bursts as it breaks, always (Blast)
	}
	full := construction.MaxHP(b.Kind, b.Stuff)
	if b.HP()*100 >= full*TurretFusePct || before*100 < full*TurretFusePct {
		return
	}
	if t.h.DefenseRoll(100) >= TurretBlastPct {
		return
	}
	t.sparks = append(t.sparks, Fuse{ID: id, Kind: b.Kind, X: b.X, Y: b.Y, At: t.h.Now() + TurretFuseTicks})
	if b.HP() > 0 {
		t.h.Message(b.X, b.Y, "The %s is sparking: it will blow up!", construction.DefOf(b.Kind).Name)
	}
}

// burnFuses destroys the turrets whose fuse ran out, which is what sets them
// off (Blast), and forgets the fuse of one that left another way (crated).
func (t *Turrets) burnFuses(now uint64) {
	if len(t.sparks) == 0 {
		return
	}
	for _, f := range slices.Clone(t.sparks) { // a blast may light or end others
		if t.spark(f.ID) < 0 {
			continue
		}
		b, ok := t.h.Building(f.ID)
		switch {
		case !ok:
			t.sparks = slices.Delete(t.sparks, t.spark(f.ID), t.spark(f.ID)+1)
		case f.At <= now:
			t.h.DamageBuilding(f.ID, b.HP())
		}
	}
}

// Blast: turret b (already removed) is destroyed. Foam always bursts; any
// other blows up when its fuse burns: a bomb through the blast path (armor,
// buildings, a turret among them may light in turn).
func (t *Turrets) Blast(id entity.EntityID, b construction.Building) {
	delete(t.aims, id)
	delete(t.arms, id)
	h := t.h
	if t.blasts == nil {
		return
	}
	switch b.Kind { // AX-5: a smashed popper or foam turret bursts in foam, always
	case construction.FoamPopper:
		h.Message(b.X, b.Y, "The foam popper burst as it broke.")
		t.blasts.Explode(0, weapon.MortarFoam, b.X, b.Y, foamDef(PopperRadius))
		return
	case construction.FoamTurret:
		h.Message(b.X, b.Y, "The foam turret burst as it broke.")
		t.blasts.Explode(0, weapon.FoamSpray, b.X, b.Y, foamDef(FoamTurretBurstRadius))
		return
	}
	i := t.spark(id)
	if i < 0 {
		return
	}
	t.sparks = slices.Delete(t.sparks, i, i+1)
	h.Message(b.X, b.Y, "The %s blew up!", construction.DefOf(b.Kind).Name)
	t.blasts.Explode(0, gunOf(b.Kind), b.X, b.Y, turretBlast(b.Kind))
}

// turretBlast is the bomb a turret of kind k goes off with.
func turretBlast(k construction.Kind) explosion.Def {
	r := turretBlastRadius
	switch k {
	case construction.RocketBattery:
		r = RocketBlastRadius
	case construction.Autocannon, construction.UraniumSlugTurret:
		r = heavyBlastRadius
	}
	return explosion.Def{Kind: explosion.Bomb, Radius: r, Damage: TurretBlastDamage, Pen: turretBlastPen}
}

// sparkAt is the sparking turret whose blast would catch (x, y), its tile
// and its blast's reach in whole tiles (ok false: none; EAI-012).
func (t *Turrets) sparkAt(x, y int) (lx, ly, reach int, ok bool) {
	for _, f := range t.sparks {
		if b := turretBlast(f.Kind); b.Within(x-f.X, y-f.Y) {
			return f.X, f.Y, b.Reach(), true
		}
	}
	return 0, 0, 0, false
}

// sparkNear is the sparking turret within its blast's reach+2 of (x, y)
// (Chebyshev; the step-out band), its tile and reach, and whether (x, y)
// is in the blast itself (ok false: none).
func (t *Turrets) sparkNear(x, y int) (lx, ly, reach int, in, ok bool) {
	if lx, ly, reach, in = t.sparkAt(x, y); in {
		return lx, ly, reach, true, true
	}
	for _, f := range t.sparks {
		if r := turretBlast(f.Kind).Reach(); weapon.Distance(x, y, f.X, f.Y) <= r+2 {
			return f.X, f.Y, r, false, true
		}
	}
	return 0, 0, 0, false, false
}
