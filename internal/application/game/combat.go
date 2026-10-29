package game

import (
	"slices"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Combat's game side: weapons, wounds and death, hunting work, and the
// attack traces' read model. Attack resolution, defending, traps,
// breaching and turrets are the combat package (combat_host.go).

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
	// FromBuilding: a sentry turret fired it (PLAY-024); Attacker is then
	// the turret's building id.
	FromBuilding bool
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

// hurt applies damage to an animal or a colonist.
func (g *Game) hurt(target entity.EntityID, dmg int, by entity.EntityID) {
	g.hurtAs(target, dmg, by, woundByAttacker, health.CauseInjury)
}

// woundByAttacker: hurtAs picks the wound kind from the attacker's weapon.
const woundByAttacker body.WoundKind = 255

// hurtAs is hurt with a wound kind for humanlikes (fire.go: burns) and the
// cause of death should it kill.
func (g *Game) hurtAs(target entity.EntityID, dmg int, by entity.EntityID, kind body.WoundKind, cause health.Cause) {
	now := uint64(g.clock.Now())
	if a, ok := g.animals.Get(target); ok {
		a.Injury += dmg
		g.woundAnimal(target, &a, dmg, kind, by, now) // animal_health.go (MED-09): a wound on its body
		a.Attacker = by
		was := g.chaseTarget(a)
		g.wakeDormant(&a) // vault.go: a hurt vault sleeper wakes
		g.provoked(target, &a, by)
		g.animalHurt(target, &a, by, now) // animal_needs.go: wakes it, then it fights back or runs
		g.animals.Set(target, a)
		if t := g.chaseTarget(a); t != 0 && t != was && !a.Dead() {
			g.noteChase(target, t) // chase.go: it turned on a colonist
		}
		if a.Dead() {
			g.killAnimal(target, by)
		}
		return
	}
	v, ok := g.vital.Get(target)
	if !ok || v.Status == health.Dead {
		return
	}
	b := g.ensureBody(target, &v) // body.go: a humanlike takes a wound on a part
	if b != nil && dmg > 0 {
		if kind == woundByAttacker {
			kind = g.woundKindBy(by)
		}
		g.woundBody(b, dmg, kind, now)
		g.bodyChanged(target, b, &v) // Injury = the body's damage
	} else {
		v.Injury += dmg
		g.vital.Set(target, v)
	}
	g.woundOpened(target)
	if by != 0 {
		g.onHarmed(target, by) // social.go: the victim's opinion of the attacker
	}
	switch {
	case v.Injury >= health.MaxHP || b != nil && b.VitalLost():
		g.dieBy(target, cause, by)
	case v.Status == health.Alive && g.downsNow(target, v, b): // bleed.go: consciousness, moving or pain shock
		g.down(target, health.CauseInjury)
	}
	g.mind.Downed(target) // mind: a downed brawler ends the fight
}

// die kills pawn id with no known killer.
func (g *Game) die(id entity.EntityID, cause health.Cause) { g.dieBy(id, cause, 0) }

// dieBy kills pawn id; killer is who dealt the last blow (0: none).
func (g *Game) dieBy(id entity.EntityID, cause health.Cause, killer entity.EntityID) {
	g.stainDeath(id) // stains.go
	g.interrupt(id, job.Cancelled)
	g.care.Doctor.PatientDied(id) // care/rescue.go (SIM-063): a carrier drops a patient that died
	g.releaseBed(id)
	delete(g.draft, id)
	g.logEvent(id, logEntry{Kind: logDied})
	v, _ := g.vital.Get(id)
	v.Status, v.Cause = health.Dead, cause
	g.vital.Set(id, v)
	g.mind.Downed(id) // mind: the dead keep no break
	g.spawnCorpse(id)
	g.dropRaiderGear(id)        // the weapon falls beside the body
	g.forbidRaidBody(id)        // forbid.go: a raider's body starts forbidden
	g.socialOnDeath(id, killer) // social_grief.go: relations, grief, killer opinion
	g.bondsOnDeath(id)          // bonds.go: bonded animals lose their master
	g.mourn(id)
}

// heal: humanlikes heal by the wound, an hour at a time, in the medical
// phase (stepCare, infection.go; MED-02), which also stands them up. On
// its cadence this only catches a hurt pawn the patients index does not
// know: one with no body yet (an older save) or an Injury set from
// outside; ensureBody turns that injury into wounds and it joins the
// patients.
func (g *Game) heal(id entity.EntityID, v *health.Record, now uint64) {
	if v.Injury <= 0 || (now+uint64(id))%health.HealTicks != 0 {
		return
	}
	if b := g.bodies[id]; b != nil && (g.indexedPatient(id) || v.Injury == b.Damage()) {
		return
	}
	if b := g.ensureBody(id, v); b != nil {
		g.bodyChanged(id, b, v) // patients.go: joins the index while it needs care
	}
}

// hostile: an animal currently attacking a colonist.
func (g *Game) hostile(a animal.State) bool {
	return !a.Tame && a.Mode == animal.Attack && a.Target != 0 && g.isPawn(a.Target) && !g.isRaider(a.Target)
}

// huntDownedFirst is added to a standing animal's distance in assignHunt,
// so any downed one is nearer.
const huntDownedFirst = 1 << 20

// assignHunt sends a hunter after the nearest designated animal (a downed
// one first: the hunter finishes it), first fetching a free ranged weapon
// if it has none.
// The next nearest animal is tried when one can't be started (out of
// reach, postponed: SIM-027), at most workTries animals.
func (g *Game) assignHunt(id entity.EntityID) bool {
	if !g.anyAnimal(func(_ entity.EntityID, a *animal.State) bool { return a.Designated && !a.Dead() }) {
		return false // jobindex.go: nothing marked for hunting
	}
	pos, _ := g.positions.Get(id)
	var tried []entity.EntityID
	for range workTries {
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
			if slices.Contains(tried, aid) || g.postponedFor(id, job.Hunt, aid, 0, 0) {
				continue
			}
			ap, _ := g.positions.Get(aid)
			d := manhattan(pos.X, pos.Y, ap.X, ap.Y)
			if !a.Downed {
				d += huntDownedFirst // a downed animal is finished first (animal_health.go)
			}
			if d < bestD {
				best, bestD, bp = aid, d, ap
			}
		}
		if best == 0 {
			return false
		}
		if !g.hasRanged(id) {
			return g.assignEquip(id, item.Rifle)
		}
		if g.startJob(id, job.Record{Kind: job.Hunt, Target: best, TX: bp.X, TY: bp.Y, WorkLeft: combat.AimTicks(weaponDefOf(g, id))}) {
			return true
		}
		tried = append(tried, best)
	}
	return false
}

// assignEquip walks to the nearest free weapon of kind k.
func (g *Game) assignEquip(id entity.EntityID, k item.Kind) bool {
	sid, x, y := g.nearestFree(id, k)
	if sid == 0 {
		return false
	}
	return g.startJob(id, job.Record{Kind: job.Equip, Target: sid, TX: x, TY: y})
}

// doEquip takes one weapon or garment from the stack and drops the old
// one; a garment the player ordered ("Wear X") makes its slot theirs.
func (g *Game) doEquip(id entity.EntityID, j job.Record) {
	st, ok := g.stacks.Get(j.Target)
	if !ok || st.Qty <= 0 || st.Carrier != 0 || !st.Kind.IsWeapon() && !st.Kind.IsApparel() {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	if st.Kind.IsApparel() {
		g.wear(id, j.Target, st, j.Forced) // apparel.go
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
			FromBuilding: t.FromBuilding, Attacker: t.Attacker, Target: t.Target, Weapon: t.Weapon})
	}
	return out
}

// walkableNear clamps (x, y) to the map and finds a walkable tile within
// r of it.
func (g *Game) walkableNear(x, y, r int) (int, int, bool) {
	x, y = clampInt(x, 0, g.grid.Width()-1), clampInt(y, 0, g.grid.Height()-1)
	for d := 0; d <= r; d++ {
		for dy := -d; dy <= d; dy++ {
			for dx := -d; dx <= d; dx++ {
				if g.dryWalkable(x+dx, y+dy) {
					if _, taken := g.reserve.TileOwner(x+dx, y+dy); !taken {
						return x + dx, y + dy, true
					}
				}
			}
		}
	}
	return 0, 0, false
}
