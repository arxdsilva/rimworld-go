package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// Combat: pawns wield a weapon (fists when unarmed), attacks resolve
// instantly with a deterministic roll, and every attack leaves a short
// trace for presentation. Hunting is a work type that needs a ranged
// weapon; drafted pawns attack on order and fire at hostiles in range.

// Order-menu reasons for combat.
const (
	ReasonNoHunt   = "hunting disabled"
	ReasonNoRanged = "needs a ranged weapon"
)

// TraceView is one recent attack for presentation. Age is ticks since
// it happened (0..projectile.Lifetime).
type TraceView struct {
	FromX, FromY int
	ToX, ToY     int
	Age          int
	Hit          bool
	Melee        bool
	Damage       int  // dealt on a hit
	Building     bool // a blow on a door or wall
	// Attacker and Target are the creatures involved (0 for a building
	// target); Weapon is the attacker's weapon name.
	Attacker, Target entity.EntityID
	Weapon           string
}

func weaponKindOf(k item.Kind) weapon.Kind {
	switch k {
	case item.Club:
		return weapon.Club
	case item.Knife:
		return weapon.Knife
	case item.Spear:
		return weapon.Spear
	case item.Rifle:
		return weapon.Rifle
	}
	return weapon.Fists
}

func (g *Game) weaponOf(id entity.EntityID) weapon.Kind {
	if k, ok := g.equip[id]; ok {
		return weaponKindOf(k)
	}
	return weapon.Fists
}

func weaponDefOf(g *Game, id entity.EntityID) weapon.Def { return weapon.DefOf(g.weaponOf(id)) }

func (g *Game) hasRanged(id entity.EntityID) bool { return weaponDefOf(g, id).Ranged }

// aimTicks is the warm-up before a pawn's first attack on a new target.
func aimTicks(d weapon.Def) int { return d.Cooldown / 3 }

// isPawn reports whether id is a colonist.
func (g *Game) isPawn(id entity.EntityID) bool {
	_, ok := g.names.Get(id)
	return ok
}

// standingPawn is a colonist who is alive and not downed.
func (g *Game) standingPawn(id entity.EntityID) bool {
	v, ok := g.vital.Get(id)
	return ok && v.Status == health.Alive && g.isPawn(id)
}

// combatTarget returns the position of a target that can still be
// attacked: a living animal or a standing colonist.
func (g *Game) combatTarget(id entity.EntityID) (Position, bool) {
	if a, ok := g.animals.Get(id); ok {
		if a.Dead() {
			return Position{}, false
		}
		return g.positions.Get(id)
	}
	if g.standingPawn(id) {
		return g.positions.Get(id)
	}
	return Position{}, false
}

// blocksSight: rock, walls and (closed) doors stop bullets.
func (g *Game) blocksSight(x, y int) bool {
	if g.terr.At(x, y) == terrain.Rock {
		return true
	}
	if bid := g.layerBldgAt(x, y, construction.LayerObject); bid != 0 {
		b, _ := g.bldgs.Get(bid)
		return b.Kind == construction.Wall || b.Kind == construction.Door
	}
	return false
}

// lineOfSight walks the Bresenham line between two tiles, endpoints
// excluded.
func (g *Game) lineOfSight(ax, ay, bx, by int) bool {
	dx, dy := bx-ax, by-ay
	sx, sy := 1, 1
	if dx < 0 {
		dx, sx = -dx, -1
	}
	if dy < 0 {
		dy, sy = -dy, -1
	}
	err := dx - dy
	x, y := ax, ay
	for {
		if x == bx && y == by {
			return true
		}
		if !(x == ax && y == ay) && g.blocksSight(x, y) {
			return false
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}
}

// canHit reports whether attacker (with weapon d) can attack the target
// at tp from where it stands.
func (g *Game) canHit(from Position, d weapon.Def, tp Position) bool {
	dist := weapon.Distance(from.X, from.Y, tp.X, tp.Y)
	if !weapon.InRange(d, dist) {
		return false
	}
	return !d.Ranged || dist <= 1 || g.lineOfSight(from.X, from.Y, tp.X, tp.Y)
}

// strike resolves one attack and records its trace.
func (g *Game) strike(attacker, target entity.EntityID, wk weapon.Kind) {
	d := weapon.DefOf(wk)
	ap, _ := g.positions.Get(attacker)
	tp, ok := g.positions.Get(target)
	if !ok {
		return
	}
	dist := weapon.Distance(ap.X, ap.Y, tp.X, tp.Y)
	chance := weapon.HitChance(d, dist)
	if d.Ranged {
		chance = chance * (100 - g.coverFor(ap, tp)) / 100
	}
	hit := g.rng.IntN(100) < chance
	tr := projectile.Trace{
		FromX: ap.X, FromY: ap.Y, ToX: tp.X, ToY: tp.Y,
		Tick: uint64(g.clock.Now()), Hit: hit, Melee: !d.Ranged,
		Attacker: attacker, Target: target, Weapon: d.Name,
	}
	if hit {
		tr.Damage = d.Damage
	}
	g.traces = append(g.traces, tr)
	g.lastStrike[attacker] = strikeRec{Target: target, Tick: tr.Tick}
	if hit {
		g.lastHit[target] = hitRec{Tick: tr.Tick, FromX: ap.X, FromY: ap.Y, Damage: d.Damage}
		g.hurt(target, d.Damage, attacker)
	}
}

// hurt applies damage to an animal or a colonist.
func (g *Game) hurt(target entity.EntityID, dmg int, by entity.EntityID) {
	now := uint64(g.clock.Now())
	if a, ok := g.animals.Get(target); ok {
		a.Injury += dmg
		a.Attacker = by
		if animal.DefOf(a.Species).Skittish && !a.Tame {
			a.Mode, a.FleeUntil, a.Path = animal.Flee, now+900, nil
		}
		g.provoked(target, &a, by)
		g.animals.Set(target, a)
		if a.Dead() {
			g.killAnimal(target, by)
		}
		return
	}
	v, ok := g.vital.Get(target)
	if !ok || v.Status == health.Dead {
		return
	}
	v.Injury += dmg
	g.vital.Set(target, v)
	g.woundOpened(target)
	switch {
	case v.Injury >= health.MaxHP:
		g.die(target, health.CauseInjury)
	case v.Injury >= health.DownedInjury && v.Status == health.Alive:
		g.down(target, health.CauseInjury)
	}
}

func (g *Game) die(id entity.EntityID, cause health.Cause) {
	g.interrupt(id, job.Cancelled)
	g.releaseBed(id)
	delete(g.draft, id)
	v, _ := g.vital.Get(id)
	v.Status, v.Cause = health.Dead, cause
	g.vital.Set(id, v)
	g.spawnCorpse(id)
	g.dropRaiderGear(id) // the weapon falls beside the body
	g.mourn(id)
}

// heal regenerates injury over time and stands injured pawns back up.
func (g *Game) heal(id entity.EntityID, v *health.Record, now uint64) {
	if v.Injury <= 0 {
		return
	}
	every := uint64(health.HealTicks)
	j, _ := g.jobs.Get(id)
	switch {
	case g.inBed(id):
		every /= 2
	case v.Status == health.Downed:
		every *= 2
	case j.Kind == job.Sleep && j.State == job.Do && g.sleptAt[id] != 0:
		every /= 2
	}
	if g.tended(id) {
		every /= 2
	}
	if (now+uint64(id))%every != 0 {
		return
	}
	v.Injury--
	if v.Status == health.Downed && v.Cause == health.CauseInjury && v.Injury <= health.RecoverInjury {
		v.Status, v.Cause = health.Alive, health.CauseNone
	}
	g.vital.Set(id, *v)
}

// doCombat runs a Hunt or Attack job: close in until the target is in
// range (and in sight for guns), then attack on cooldown.
func (g *Game) doCombat(id entity.EntityID, j job.Record, now uint64) {
	tp, ok := g.combatTarget(j.Target)
	if ok && j.Kind == job.Hunt {
		a, _ := g.animals.Get(j.Target)
		ok = a.Designated
	}
	if !ok {
		j.State, j.Path = job.Complete, nil
		g.jobs.Set(id, j)
		return
	}
	pos, _ := g.positions.Get(id)
	wk := g.weaponOf(id)
	d := weapon.DefOf(wk)
	if j.WorkLeft > 0 {
		j.WorkLeft--
	}
	if g.canHit(pos, d, tp) {
		j.State, j.Path, j.PathI = job.Do, nil, 0
		if j.WorkLeft == 0 {
			g.strike(id, j.Target, wk)
			j.WorkLeft = d.Cooldown
		}
		g.jobs.Set(id, j)
		return
	}
	stale := j.State != job.Goto || j.PathI >= len(j.Path) ||
		weapon.Distance(j.TX, j.TY, tp.X, tp.Y) > 1 || (now+uint64(id))%60 == 0
	if stale {
		path, ok := g.pathTo(id, tp.X, tp.Y)
		if !ok {
			j.State, j.Fail, j.Path = job.Fail, job.Unreachable, nil
			g.jobs.Set(id, j)
			return
		}
		j.Path, j.PathI, j.TX, j.TY = path, 0, tp.X, tp.Y
		j.State = job.Goto
		if len(path) == 0 {
			j.State = job.Do
		}
	}
	g.jobs.Set(id, j)
}

// autoEngage makes an idle drafted pawn attack the nearest hostile it
// can hit from where it stands.
func (g *Game) autoEngage(id entity.EntityID) {
	pos, _ := g.positions.Get(id)
	d := weaponDefOf(g, id)
	best, bestD := entity.EntityID(0), 1<<30
	for _, aid := range g.animalIDs {
		a, ok := g.animals.Get(aid)
		if !ok || !g.hostile(a) {
			continue
		}
		ap, _ := g.positions.Get(aid)
		dist := weapon.Distance(pos.X, pos.Y, ap.X, ap.Y)
		if dist < bestD && g.canHit(pos, d, ap) {
			best, bestD = aid, dist
		}
	}
	for _, rid := range g.raiderIDs {
		if !g.standingRaider(rid) {
			continue
		}
		rp, _ := g.positions.Get(rid)
		dist := weapon.Distance(pos.X, pos.Y, rp.X, rp.Y)
		if dist < bestD && g.canHit(pos, d, rp) {
			best, bestD = rid, dist
		}
	}
	if best == 0 {
		return
	}
	ap, _ := g.positions.Get(best)
	g.startJob(id, job.Record{Kind: job.Attack, Target: best, TX: ap.X, TY: ap.Y, WorkLeft: aimTicks(d)})
}

// hostile: an animal currently attacking a colonist.
func (g *Game) hostile(a animal.State) bool {
	return !a.Tame && a.Mode == animal.Attack && a.Target != 0 && g.isPawn(a.Target) && !g.isRaider(a.Target)
}

// assignHunt sends a hunter after the nearest designated animal, first
// fetching a free ranged weapon if it has none.
func (g *Game) assignHunt(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	best, bestD := entity.EntityID(0), 1<<30
	var bp Position
	for _, aid := range g.animalIDs {
		a, ok := g.animals.Get(aid)
		if !ok || !a.Designated || a.Dead() {
			continue
		}
		if _, taken := g.reserve.EntityOwner(aid); taken {
			continue
		}
		ap, _ := g.positions.Get(aid)
		if d := manhattan(pos.X, pos.Y, ap.X, ap.Y); d < bestD {
			best, bestD, bp = aid, d, ap
		}
	}
	if best == 0 {
		return false
	}
	if !g.hasRanged(id) {
		return g.assignEquip(id, item.Rifle)
	}
	return g.startJob(id, job.Record{Kind: job.Hunt, Target: best, TX: bp.X, TY: bp.Y, WorkLeft: aimTicks(weaponDefOf(g, id))})
}

// assignEquip walks to the nearest free weapon of kind k.
func (g *Game) assignEquip(id entity.EntityID, k item.Kind) bool {
	sid, x, y := g.nearestFree(id, k)
	if sid == 0 {
		return false
	}
	return g.startJob(id, job.Record{Kind: job.Equip, Target: sid, TX: x, TY: y})
}

// doEquip takes one weapon from the stack and drops the old one.
func (g *Game) doEquip(id entity.EntityID, j job.Record) {
	st, ok := g.stacks.Get(j.Target)
	if !ok || st.Qty <= 0 || st.Carrier != 0 || !st.Kind.IsWeapon() && !st.Kind.IsApparel() {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	if st.Kind.IsApparel() {
		g.wear(id, j.Target, st)
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	pos, _ := g.positions.Get(id)
	if old, ok := g.equip[id]; ok {
		g.dropStack(old, 1, pos.X, pos.Y)
	}
	g.equip[id] = st.Kind
	if st.Qty == 1 {
		g.deleteItem(j.Target)
	} else {
		st.Qty--
		g.stacks.Set(j.Target, st)
	}
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// Equipped returns the wielded weapon item kind name, "" when unarmed.
func (g *Game) Equipped(id entity.EntityID) string {
	if k, ok := g.equip[id]; ok {
		return k.String()
	}
	return ""
}

func (g *Game) traceViews() []TraceView {
	now := uint64(g.clock.Now())
	out := make([]TraceView, 0, len(g.traces))
	for _, t := range g.traces {
		if now-t.Tick >= projectile.Lifetime {
			continue
		}
		out = append(out, TraceView{FromX: t.FromX, FromY: t.FromY, ToX: t.ToX, ToY: t.ToY,
			Age: int(now - t.Tick), Hit: t.Hit, Melee: t.Melee, Damage: t.Damage, Building: t.Building,
			Attacker: t.Attacker, Target: t.Target, Weapon: t.Weapon})
	}
	return out
}

// defend reacts to an animal attacking an undrafted colonist: armed
// colonists fight back, unarmed ones run. It reports whether it acted.
func (g *Game) defend(id entity.EntityID, j job.Record) bool {
	pos, _ := g.positions.Get(id)
	var threat entity.EntityID
	var tp Position
	best := 4
	for _, aid := range g.animalIDs {
		a, ok := g.animals.Get(aid)
		if !ok || a.Mode != animal.Attack || a.Target != id {
			continue
		}
		ap, _ := g.positions.Get(aid)
		if d := weapon.Distance(pos.X, pos.Y, ap.X, ap.Y); d < best {
			threat, tp, best = aid, ap, d
		}
	}
	for _, rid := range g.raiderIDs { // raiders shoot from further off
		rj, _ := g.jobs.Get(rid)
		if !g.standingRaider(rid) || rj.Kind != job.Attack || rj.Target != id {
			continue
		}
		rp, _ := g.positions.Get(rid)
		if d := weapon.Distance(pos.X, pos.Y, rp.X, rp.Y); d < 14 && (threat == 0 || d < best) {
			threat, tp, best = rid, rp, d
		}
	}
	if threat == 0 {
		return false
	}
	if _, armed := g.equip[id]; armed {
		if j.Kind == job.Attack && j.Target == threat && j.State != job.Fail {
			return true
		}
		g.interrupt(id, job.Interrupted)
		g.startJob(id, job.Record{Kind: job.Attack, Target: threat, TX: tp.X, TY: tp.Y, WorkLeft: aimTicks(weaponDefOf(g, id))})
		return true
	}
	now := uint64(g.clock.Now())
	if j.Kind == job.Move && j.State == job.Goto && (now+uint64(id))%30 != 0 {
		return true // keep running
	}
	fx, fy := animal.FleeGoal(pos.X, pos.Y, tp.X, tp.Y, 7)
	fx, fy, ok := g.walkableNear(fx, fy, 3)
	if !ok {
		return false
	}
	g.interrupt(id, job.Interrupted)
	g.startJob(id, job.Record{Kind: job.Move, TX: fx, TY: fy})
	return true
}

// walkableNear clamps (x, y) to the map and finds a walkable tile within
// r of it.
func (g *Game) walkableNear(x, y, r int) (int, int, bool) {
	x, y = clampInt(x, 0, g.grid.Width()-1), clampInt(y, 0, g.grid.Height()-1)
	for d := 0; d <= r; d++ {
		for dy := -d; dy <= d; dy++ {
			for dx := -d; dx <= d; dx++ {
				if g.grid.Walkable(x+dx, y+dy) {
					if _, taken := g.reserve.TileOwner(x+dx, y+dy); !taken {
						return x + dx, y + dy, true
					}
				}
			}
		}
	}
	return 0, 0, false
}
