package combat

import (
	"cmp"
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Fire foam and the AX-5 defences (.cursor/handoffs/AX-5-foam-security.md).
//
// A foam burst (explosion.Foam) puts out every fire it reaches and leaves
// fire foam on the ground (stain.Foam: nothing burns under it until it is
// cleaned or washed away); it harms nobody. Foam comes from:
//   - the foam popper: a fire within PopperReach lights its fuse, and
//     PopperFuse ticks later it bursts over PopperRadius and is spent; a
//     blueprint is laid in its place (the wiki's one-use device with its
//     rebuild option), so a constructor re-arms it by building it again;
//   - the foam charge, a buried charge like the others (ied.go);
//   - the foam turret: a powered nozzle that sprays FoamSpray blobs at the
//     fires it sees, one charge a spray, then waits weapon.FoamCooldown;
//     chemfuel hauled by a colonist refills its charges (rearm.go);
//   - a foam shell lobbed by a mortar or cooking off, and a smashed popper
//     or foam turret.
// The rocket battery (battery.go) is manned like a mortar and fires a
// volley of rockets at the player's target from a full load only, then
// needs a reload of chemfuel and its recharge.

const (
	// FoamSpill is the foam a burst leaves on each cell it reaches (of
	// stain.Full). original tuning.
	FoamSpill = 600
	// The foam popper: a fire within PopperReach tiles lights it, it
	// sparks PopperFuse ticks, then bursts over PopperRadius. wiki:
	// Firefoam popper (3 tiles, 150 ticks, a 10-tile burst; in game 9.9)
	PopperReach  = 3900
	PopperFuse   = 150
	PopperRadius = 9900
	// PopperScanEvery: an armed popper looks for fire once in this many
	// ticks, staggered by id. original tuning.
	PopperScanEvery = 30
	// FoamChargeRadius is the foam charge's burst. wiki: IED firefoam trap
	FoamChargeRadius = 9900
	// FoamTurretCharges is a foam turret's full load (one a spray) and
	// FoamTurretFuel the chemfuel a charge costs, tenths of a unit before
	// the /9 scaling every cost here takes (7.2). wiki: Foam turret
	FoamTurretCharges = 10
	FoamTurretFuel    = 72
	// RocketVolley is a rocket battery's full load: one volley. wiki:
	// Rocketswarm launcher
	RocketVolley = 12
	// Smashed: a foam turret bursts damageless foam over 4 tiles; a
	// rocket battery blows up for 50 bomb over 6.9 tiles (by the turrets'
	// TurretBlastPct roll). wiki: Foam turret, Rocketswarm launcher
	FoamTurretBurstRadius = 4000
	RocketBlastRadius     = 6900
)

// foamDef is a foam burst of radius r (thousandths of a tile).
func foamDef(r int) explosion.Def { return explosion.Def{Kind: explosion.Foam, Radius: r} }

// LoadOf is the shots a chemfuel turret of kind k holds when full: a foam
// turret's charges (0: a barrel turret).
func LoadOf(k construction.Kind) int {
	if k == construction.FoamTurret {
		return FoamTurretCharges
	}
	return 0
}

// Charges is what turret id (of kind k) has left of its LoadOf.
func (t *Turrets) Charges(id entity.EntityID, k construction.Kind) int {
	return max(0, LoadOf(k)-t.arms[id].Fired)
}

// wantsRearm: turret id (of kind k) asks for a haul. A barrel turret at a
// quarter barrel, a foam turret at a quarter of its charges. Each is
// filled to full, so it asks once (no flapping between asking and not).
func (t *Turrets) wantsRearm(id entity.EntityID, k construction.Kind) bool {
	if k == construction.FoamTurret {
		return t.Charges(id, k) <= FoamTurretCharges/4
	}
	return t.Barrel(id, k) <= 250
}

// chemfuel is the chemfuel units turret id (of kind k) takes to fill up
// (0: not a chemfuel turret): a foam turret 7.2 a charge rounded up, /9
// as every cost here. wiki: Foam turret
func (t *Turrets) chemfuel(id entity.EntityID, k construction.Kind) int {
	if k == construction.FoamTurret {
		return (t.arms[id].Fired*FoamTurretFuel + 89) / 90
	}
	return 0
}

// Spent reports turret id (of kind k) unable to fire until a rearm: a
// barrel fired out, a foam turret out of charges.
func (t *Turrets) Spent(id entity.EntityID, k construction.Kind) bool {
	if k == construction.FoamTurret { // a spray under way finishes
		return t.arms[id].Fired >= FoamTurretCharges && t.aims[id].Burst == 0
	}
	return t.arms[id].Fired >= BarrelOf(k)
}

// cool keeps turret id quiet until tick until (a foam turret's wait after
// it sprayed).
func (t *Turrets) cool(id entity.EntityID, until uint64) {
	if t.arms == nil {
		t.arms = map[entity.EntityID]Arm{}
	}
	r := t.arms[id]
	r.Cool = until
	t.arms[id] = r
}

// foamStep is a foam turret's tick at (x, y), powered, not held and with a
// charge: idle, it looks for the fires it can see within its nozzle's
// range on its scan tick and, finding one, spends a charge and starts a
// spray; spraying, it puts one blob every FoamSpray.BurstTicks on the
// fires it sees, nearest first and in turn (Burst counts the blobs), and
// after the last (or once no fire is left) it waits FoamCooldown.
func (t *Turrets) foamStep(id entity.EntityID, x, y int, now uint64) {
	a := t.aims[id]
	d := weapon.DefOf(weapon.FoamSpray)
	if a.Burst == 0 && (now+uint64(id))%TurretScanEvery != 0 || a.Burst > 0 && now < a.ReadyAt {
		return
	}
	cells := t.fires(x, y, d.Range)
	if len(cells) == 0 {
		if a.Burst > 0 { // the fires are out: the spray stops
			a.Burst, a.ReadyAt = 0, 0
			t.setAim(id, a)
			t.cool(id, now+uint64(d.Cooldown))
		}
		return
	}
	if a.Burst == 0 {
		t.wear(id) // a spray takes a charge
	}
	c := cells[int(a.Burst)%len(cells)]
	a.Dir = aimFrame(x, y, c[0], c[1], a.Dir)
	t.st.shots.Turret(id, weapon.FoamSpray, x, y, 0, c[0], c[1])
	a.ShotAt, a.ReadyAt = now, now+uint64(d.BurstTicks)
	if a.Burst++; int(a.Burst) >= d.Burst {
		a.Burst, a.ReadyAt = 0, 0
		t.cool(id, now+uint64(d.Cooldown))
	}
	t.setAim(id, a)
}

// fires is the burning cells within r tiles (straight line) of (x, y) that
// the turret there can see, nearest first (ties y then x), in the reused
// buffer.
func (t *Turrets) fires(x, y, r int) [][2]int {
	t.fireBuf = t.fireBuf[:0]
	fm := t.foam
	if fm == nil || !fm.FiresNear(x, y, r) {
		return t.fireBuf
	}
	for cy := y - r; cy <= y+r; cy++ {
		for cx := x - r; cx <= x+r; cx++ {
			if fm.Burning(cx, cy) > 0 && weapon.Reaches(weapon.Def{Range: r}, cx-x, cy-y) && t.sight.Line(x, y, cx, cy) {
				t.fireBuf = append(t.fireBuf, [2]int{cx, cy})
			}
		}
	}
	slices.SortStableFunc(t.fireBuf, func(p, q [2]int) int {
		dp, dq := (p[0]-x)*(p[0]-x)+(p[1]-y)*(p[1]-y), (q[0]-x)*(q[0]-x)+(q[1]-y)*(q[1]-y)
		return cmp.Compare(dp, dq)
	})
	return t.fireBuf
}

// smother is a foam burst d at (x, y): every cell it reaches (in sight of
// the centre) has its fire put out and takes FoamSpill of foam.
func (b *Blasts) smother(x, y int, d explosion.Def) {
	if b.foam == nil {
		return
	}
	for _, c := range b.reach(x, y, d) {
		if b.foam.Burning(c.x, c.y) > 0 {
			b.foam.Douse(c.x, c.y)
		}
		b.foam.Spill(c.x, c.y, FoamSpill)
	}
}

// Poppers are the foam poppers watching for fire (AX-5): each armed one
// looks once in PopperScanEvery ticks (staggered) for a fire within
// PopperReach and, seeing one, lights its fuse in the trap queue
// (Traps.light: PopperFuse later it bursts and is re-laid as a
// blueprint). A popper whose fuse burns is never lit twice, so it cannot
// flap.
type Poppers struct {
	h     Host
	f     Field
	foam  Foamer
	traps *Traps
	ids   []entity.EntityID // the poppers, ascending, refreshed every doorListEvery ticks
	idsAt uint64
	idsOK bool
}

func isPopper(k construction.Kind) bool { return k == construction.FoamPopper }

// list is the poppers, ascending (a building scan at most once in
// doorListEvery ticks, as the mortars').
func (p *Poppers) list(now uint64) []entity.EntityID {
	if p.idsOK && now >= p.idsAt && now < p.idsAt+doorListEvery {
		return p.ids
	}
	if p.f != nil {
		p.ids = p.f.AppendBuilt(isPopper, p.ids[:0])
	}
	p.idsAt, p.idsOK = now, true
	return p.ids
}

// phase lights the poppers that sense a fire.
func (p *Poppers) phase(now uint64) {
	if p.foam == nil {
		return
	}
	for _, id := range p.list(now) {
		if (now+uint64(id))%PopperScanEvery != 0 || p.traps.Burning(id) {
			continue
		}
		if b, ok := p.h.Building(id); ok && b.Kind == construction.FoamPopper && p.senses(b.X, b.Y) {
			p.traps.light(id, b.Kind, b.X, b.Y)
		}
	}
}

// senses reports a fire within PopperReach of (x, y).
func (p *Poppers) senses(x, y int) bool {
	r := (PopperReach + 999) / 1000
	if !p.foam.FiresNear(x, y, r) {
		return false
	}
	d := explosion.Def{Radius: PopperReach}
	for cy := y - r; cy <= y+r; cy++ {
		for cx := x - r; cx <= x+r; cx++ {
			if d.Within(cx-x, cy-y) && p.foam.Burning(cx, cy) > 0 {
				return true
			}
		}
	}
	return false
}
