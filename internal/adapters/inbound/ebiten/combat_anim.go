package ebiten

import (
	"math"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Combat presentation (ART §27.2): attack and hit reactions, combat
// particles, aiming and firing poses, falls. Owned by the combat
// animation work; the dispatch in pawn_anim.go calls in here.
//
// Timing (§27.2, C7): the wind-up follows AttackCycle; the strike (the
// lunge's peak, a club's blow, the muzzle kick) lands when the attack's
// trace is first seen (its birth tick, Tick-Age); the follow-through runs
// on animT from there. Every timed effect lasts at least
// minReactionFrames present frames.

// Combat timing in sim ticks, and reaction sizes in tiles.
const (
	aimAt        = 0.6 // AttackCycle at which an animal's wind-up turns to frame 2
	fireTicks    = 9.0 // the shot's muzzle kick and settle
	muzzleKick   = 0.3 // radians the muzzle rises at the kick's peak
	recoilTicks  = 6.0
	recoilDist   = 0.08 // the body's recoil, tiles
	recoilPull   = 0.06 // the rifle pulled back along the aim, tiles
	meleeTicks   = 10.0 // strike + follow-through
	strikeWindow = 30.0 // a strike older than this no longer shapes the pose
	hitTicks     = 12.0 // the flinch and the hit flash
	staggerTicks = 10.0
	staggerDist  = 0.12
	bloodSpeed   = 0.03
	casingSpeed  = 0.02
)

// --- reactions from attack traces ---------------------------------------

// minReactionFrames is how long a recoil, lunge or flash stays visible in
// present frames, however fast the simulation runs (ART §27 clocks).
const minReactionFrames = 8

// reactTrace is a trace as the reaction sees it: sim is its age in sim
// ticks (+Inf once the sim dropped it), frames its age in present frames
// and born the sim tick it happened.
type reactTrace struct {
	game.TraceView
	sim, frames float64
	born        float64
}

type reactRec struct {
	trace game.TraceView
	seen  uint64 // present frame first seen
	born  uint64 // sim tick of the attack
	live  bool
}

// reactionAge is the age a reaction of duration dur ticks is shown at: the
// sim age, but never advancing faster than dur over minReactionFrames.
func reactionAge(sim, frames, dur float64) float64 {
	return math.Min(sim, frames*dur/minReactionFrames)
}

// attackReaction offsets an actor at (x, y): rifle recoil, melee lunge,
// and knockback plus a flash when struck, timed by sim age alone.
func attackReaction(traces []game.TraceView, x, y int) (dx, dy float64, flash float32) {
	rs := make([]reactTrace, len(traces))
	for i, t := range traces {
		rs[i] = reactTrace{TraceView: t, sim: float64(t.Age), frames: math.Inf(1)}
	}
	return attackReactionF(rs, x, y)
}

// attackReactionF is attackReaction with the minimum on-screen duration.
func attackReactionF(traces []reactTrace, x, y int) (dx, dy float64, flash float32) {
	ax, ay := attackerNudge(traces, x, y)
	sx, sy, flash := struckNudge(traces, x, y)
	return ax + sx, ay + sy, flash
}

// attackerNudge is the rifle recoil or melee lunge of an attacker at (x, y).
func attackerNudge(traces []reactTrace, x, y int) (dx, dy float64) {
	for _, t := range traces {
		if t.FromX != x || t.FromY != y || (t.ToX == x && t.ToY == y) {
			continue
		}
		ux, uy := unit(float64(t.ToX-x), float64(t.ToY-y))
		switch {
		case t.Melee && reactionAge(t.sim, t.frames, 10) < 10:
			k := math.Sin(math.Pi * reactionAge(t.sim, t.frames, 10) / 10)
			dx, dy = dx+ux*0.22*k, dy+uy*0.22*k
		case !t.Melee && reactionAge(t.sim, t.frames, 6) < 6:
			k := 1 - reactionAge(t.sim, t.frames, 6)/6
			dx, dy = dx-ux*0.08*k, dy-uy*0.08*k
		}
	}
	return dx, dy
}

// struckNudge is the knockback and flash of a creature hit at (x, y).
func struckNudge(traces []reactTrace, x, y int) (dx, dy float64, flash float32) {
	for _, t := range traces {
		if t.ToX != x || t.ToY != y || !t.Hit || (t.FromX == x && t.FromY == y) {
			continue
		}
		if age := reactionAge(t.sim, t.frames, 12); age < 12 {
			k := 1 - age/12
			ux, uy := unit(float64(x-t.FromX), float64(y-t.FromY))
			dx, dy = dx+ux*0.1*k, dy+uy*0.1*k
			if f := float32(k); f > flash {
				flash = f
			}
		}
	}
	return dx, dy, flash
}

func traceKey(t game.TraceView, tick uint64) [3]uint64 {
	return [3]uint64{uint64(t.FromX)<<20 | uint64(t.FromY), uint64(t.ToX)<<20 | uint64(t.ToY), tick - uint64(t.Age)}
}

// trackReactions remembers every trace for minReactionFrames present
// frames after the sim drops it.
func (st *animState) trackReactions(traces []game.TraceView, tick, present uint64) {
	for _, rec := range st.recs {
		rec.live = false
	}
	for _, t := range traces {
		k := traceKey(t, tick)
		rec, ok := st.recs[k]
		if !ok {
			rec = &reactRec{seen: present, born: k[2]}
			st.recs[k] = rec
		}
		rec.trace, rec.live = t, true
	}
	for k, rec := range st.recs {
		if !rec.live && present-rec.seen > 3*minReactionFrames {
			delete(st.recs, k)
		}
	}
}

// reactions appends the tracked traces, aged for display.
func (st *animState) reactions(present uint64, dst []reactTrace) []reactTrace {
	for _, rec := range st.recs {
		rt := reactTrace{TraceView: rec.trace, sim: math.Inf(1), frames: float64(present - rec.seen), born: float64(rec.born)}
		if rec.live {
			rt.sim = float64(rec.trace.Age)
		}
		dst = append(dst, rt)
	}
	return dst
}

// strikeBy reports whether t is an attack by creature id standing at
// (x, y) (by tile when the trace names no attacker).
func strikeBy(t game.TraceView, id entity.EntityID, x, y int) bool {
	if t.Attacker != 0 {
		return t.Attacker == id
	}
	return t.FromX == x && t.FromY == y && (t.ToX != x || t.ToY != y)
}

// lastStrike is the newest attack by id and how long ago it landed, in
// display ticks (animT since its birth, stretched to at least
// minReactionFrames over dur). The first sighting of a trace is age 0.
func (r *Renderer) lastStrike(id entity.EntityID, x, y int, dur float64) (reactTrace, float64, bool) {
	now := r.animT()
	var best reactTrace
	bestAge := math.Inf(1)
	for _, rt := range r.traceList(r.drawW) {
		if !strikeBy(rt.TraceView, id, x, y) {
			continue
		}
		if age := reactionAge(math.Max(0, now-rt.born), rt.frames, dur); age < bestAge {
			best, bestAge = rt, age
		}
	}
	return best, bestAge, bestAge < strikeWindow
}

// hitAge is how long ago creature id was last hit (HitTick), in display
// ticks; false when it never was.
func (r *Renderer) hitAge(id entity.EntityID, hitTick uint64, x, y int) (float64, bool) {
	if hitTick == 0 {
		return 0, false
	}
	el := math.Max(0, r.animT()-float64(hitTick))
	frames := math.Inf(1)
	for _, rt := range r.traceList(r.drawW) {
		mine := rt.Target == id || rt.Target == 0 && rt.ToX == x && rt.ToY == y
		if mine && rt.Hit && math.Abs(rt.born-float64(hitTick)) <= 1 {
			frames = math.Min(frames, rt.frames)
		}
	}
	return reactionAge(el, frames, staggerTicks), true
}

// staggerK is the stagger weight at a hit's age: a sharp jolt over 3
// ticks, back to rest by staggerTicks.
func staggerK(age float64) float64 {
	switch {
	case age < 3:
		return easeOut(age / 3)
	case age < staggerTicks:
		t := (age - 3) / (staggerTicks - 3)
		return 1 - t*t*(3-2*t)
	}
	return 0
}

// awayFrom is the unit direction from (fx, fy) to (x, y); on the same
// tile it is backwards from the facing.
func awayFrom(x, y, fx, fy int, f facing) (float64, float64) {
	if ux, uy := unit(float64(x-fx), float64(y-fy)); ux != 0 || uy != 0 {
		return ux, uy
	}
	ux, uy := facingUnit(f)
	return -ux, -uy
}

func facingUnit(f facing) (float64, float64) {
	switch f {
	case faceEast:
		return 1, 0
	case faceWest:
		return -1, 0
	case faceNorth:
		return 0, -1
	}
	return 0, 1
}

// --- pose layers ----------------------------------------------------------

// hitReaction is the flinch layer while a hit is fresh (HitTick): a
// stagger and twist away from the attacker, with a flash.
func (r *Renderer) hitReaction(p game.PawnView, m moveState) actionPose {
	age, ok := r.hitAge(p.ID, p.HitTick, p.X, p.Y)
	if !ok || age >= hitTicks {
		return actionPose{}
	}
	ux, uy := awayFrom(p.X, p.Y, p.HitFromX, p.HitFromY, m.Facing)
	k := staggerK(age)
	a := actionPose{ok: true,
		dx: ux * staggerDist * k, dy: uy * staggerDist * k, rot: ux * 0.1 * k,
		flash: float32(1 - age/hitTicks)}
	// what the pawn holds is kept through the flinch: the aimed weapon,
	// a hauled load, or the carried weapon on duty
	for _, layer := range []func(game.PawnView, moveState) actionPose{r.combatPose, r.carryPose} {
		if b := layer(p, m); b.ok && b.held.set {
			a.held = b.held
			return a
		}
	}
	if onDuty(p) {
		a.held = readyHeld(p)
	}
	return a
}

// meleeReach is how far (tiles) the body lunges into a strike.
func meleeReach(weapon string) float64 {
	switch weapon {
	case "knife":
		return 0.16
	case "spear":
		return 0.26
	}
	return 0.12
}

// Melee item motion: how far a thrust pushes the weapon along the aim
// (tiles, at full lunge), and a club's short arc about the aimed angle
// (radians, positive raising its head).
const (
	knifePush   = 0.10
	spearPush   = 0.18
	fistPush    = 0.14
	clubRest    = 0.0
	clubRaised  = 0.8
	clubStrike  = -0.45
	chamberBack = -0.3 // the lunge weight while chambering a thrust
)

// meleeMotion is a melee attack's shape: the body's lunge weight (-1 back
// .. 1 full reach), the weapon's turn and its push along the aim (tiles).
// The wind-up follows AttackCycle; the strike and follow-through follow
// the age of the last strike (age 0 on the trace). Knives and spears
// jab (pushed forward at the lunge), clubs swing down through a short
// arc, fists punch (a lunge; push is the punch's reach, which the melee
// trail follows).
func meleeMotion(weapon string, cycle, age float64, struck bool) (lunge, angle, push float64) {
	if struck && age < meleeTicks {
		lunge = 1
		if age >= 3 {
			lunge = 1 - (age-3)/(meleeTicks-3)
		}
	} else if weapon != "fists" && weapon != "" {
		switch {
		case cycle < 0.5:
		case cycle < 0.8:
			t := (cycle - 0.5) / 0.3
			lunge = chamberBack * t * t * (3 - 2*t)
		default:
			lunge = chamberBack + 0.6*clamp01((cycle-0.8)/0.2) // step in
		}
	}
	reach := math.Max(lunge, 0)
	switch weapon {
	case "knife":
		push = knifePush * reach
	case "spear":
		push = spearPush * reach
	case "club":
		switch {
		case struck && age < 3:
			angle = clubStrike
		case struck && age < meleeTicks:
			angle = clubStrike + (clubRest-clubStrike)*easeOut((age-3)/(meleeTicks-3))
		default:
			t := clamp01((cycle - 0.5) / 0.5)
			angle = clubRest + (clubRaised-clubRest)*t*t*(3-2*t)
		}
	default: // fists
		push = fistPush * reach
	}
	return lunge, angle, push
}

// aimKick is the muzzle kick (radians) of a shot age display ticks old:
// up and back down over fireTicks from the trace's first sighting. The
// rifle is aimed for the whole attack cycle, so nothing else moves it.
func aimKick(age float64, struck bool) float64 {
	if struck && age < fireTicks {
		return muzzleKick * math.Sin(math.Pi*age/fireTicks)
	}
	return 0
}

// creatureSpot is where creature id is drawn (its body centre), by its
// motion or else its tile.
func (r *Renderer) creatureSpot(id entity.EntityID) (vec, bool) {
	if id == 0 {
		return vec{}, false
	}
	if m, ok := r.motionOf(id); ok {
		return vec{m.X + 0.5, m.Y + 0.4}, true
	}
	for _, q := range r.drawPawns {
		if q.ID == id {
			return vec{float64(q.X) + 0.5, float64(q.Y) + 0.4}, true
		}
	}
	for i := range r.drawW.Animals {
		if an := &r.drawW.Animals[i]; an.ID == id {
			return vec{float64(an.X) + 0.5, float64(an.Y) + 0.4}, true
		}
	}
	return vec{}, false
}

// attackTarget is the point a pawn attacks: its ActTarget, else the
// target of its last strike.
func (r *Renderer) attackTarget(p game.PawnView, st reactTrace, struck bool) (vec, bool) {
	if v, ok := r.creatureSpot(p.ActTarget); ok {
		return v, true
	}
	if !struck {
		return vec{}, false
	}
	if v, ok := r.creatureSpot(st.Target); ok {
		return v, true
	}
	return vec{float64(st.ToX) + 0.5, float64(st.ToY) + 0.4}, true
}

// combatPose is the aim/fire/melee layer while attacking: the weapon is
// aimed at the target (carried when there is none), kicks and pulls back
// on a shot, jabs or swings on a blow.
func (r *Renderer) combatPose(p game.PawnView, m moveState) actionPose {
	dur := meleeTicks
	if p.Weapon == "rifle" {
		dur = fireTicks
	}
	st, age, struck := r.lastStrike(p.ID, p.X, p.Y, dur)
	if !struck && p.Action != game.ActionAim && p.Action != game.ActionMelee {
		return actionPose{}
	}
	a := actionPose{ok: true, held: readyHeld(p)}
	tgt, hasT := r.attackTarget(p, st, struck)
	wx, wy := r.interpWorld(p)
	ux, uy := facingUnit(m.Facing)
	if hasT {
		if vx, vy := unit(tgt.x-wx-0.5, tgt.y-wy-0.4); vx != 0 || vy != 0 {
			ux, uy = vx, vy
		}
		a.held.stance, a.held.aimX, a.held.aimY = stanceAimed, tgt.x, tgt.y
	}
	moving := m.Blend >= minBlend
	if p.Action == game.ActionAim || p.Action != game.ActionMelee && struck && !st.Melee {
		a.held.angle = aimKick(age, struck)
		if struck && age < recoilTicks {
			k := 1 - age/recoilTicks
			a.dx, a.dy = -ux*recoilDist*k, -uy*recoilDist*k
			a.held.push = -recoilPull * k
		}
		return a
	}
	lunge, angle, push := meleeMotion(p.Weapon, p.AttackCycle, age, struck)
	a.held.angle, a.held.push = angle, push
	if !moving {
		reach := meleeReach(p.Weapon)
		a.dx, a.dy, a.rot = ux*reach*lunge, uy*reach*lunge, ux*0.06*lunge
	}
	return a
}

// combatOffset is the knockback nudge applied on top of any layer for a
// simulation that does not stamp hits (HitTick); recoil and lunges are
// part of combatPose and stagger of hitReaction.
func (r *Renderer) combatOffset(p game.PawnView) (dx, dy float64, flash float32) {
	if p.HitTick != 0 {
		return 0, 0, 0
	}
	return struckNudge(r.animState().reactions(r.present, nil), p.X, p.Y)
}

// --- falling --------------------------------------------------------------

// fallPose is a pawn lying on the bare ground, drawn as its standing
// south figure turned about the feet. Downed or dead it buckles (a short
// squash, sliding away from the last hit), then tips over away from that
// hit over fallFrames; asleep it just lies down.
func (r *Renderer) fallPose(p game.PawnView, vis pawnVisual) pose {
	e := float64(r.present - vis.lyingSince)
	side := 1.0
	if vis.facing == faceWest {
		side = -1
	}
	ps := standPose()
	breath := 1 + 0.02*math.Sin(2*math.Pi*float64(r.present+uint64(p.ID)*37)/110)
	if p.Status == "alive" {
		k := easeOut(e / fallFrames)
		lp := lyingPose(r.art, side)
		ps.rot, ps.dx, ps.dy, ps.sy = lp.rot*k, lp.dx*k, lp.dy*k, breath
		return ps
	}
	var ux, uy float64
	if p.HitTick != 0 {
		ux, uy = unit(float64(p.X-p.HitFromX), float64(p.Y-p.HitFromY))
		switch {
		case ux > 0.01:
			side = 1
		case ux < -0.01:
			side = -1
		}
	}
	total := float64(buckleFrames + fallFrames)
	drift := 0.12 * easeOut(e/total)
	ps.dx, ps.dy = ux*drift, uy*drift
	ps.flash = float32(0.5 * (1 - clamp01(e/total)))
	if e < buckleFrames {
		ps.sy = 1 - 0.06*e/buckleFrames
		return ps
	}
	k := easeOut((e - buckleFrames) / fallFrames)
	lp := lyingPose(r.art, side)
	ps.rot = lp.rot * k
	ps.dx += lp.dx * k
	ps.dy += lp.dy * k
	if p.Status != "dead" {
		ps.sy = breath
	}
	return ps
}

// --- weapon points and particles -----------------------------------------

// pawnByStrike finds the pawn that made a trace.
func pawnByStrike(pawns []game.PawnView, t game.TraceView) (game.PawnView, bool) {
	for _, p := range pawns {
		if strikeBy(t, p.ID, p.X, p.Y) && (t.Attacker != 0 || !p.Lying) {
			return p, true
		}
	}
	return game.PawnView{}, false
}

// pawnHeldPoint is where an attachment of what a standing pawn holds is
// right now, in world tiles (with bare hands, the body's hold).
func (r *Renderer) pawnHeldPoint(p game.PawnView, attach string) (vec, bool) {
	if p.Lying || p.CarriedBy != 0 || r.art == nil {
		return vec{}, false
	}
	wx, wy := r.interpWorld(p)
	m := r.pawnMotion(p.ID)
	m.Facing = r.poseFacing(p, m)
	ps, h := r.standingPose(p, m)
	rig := r.rigOf(p, m.Facing)
	if rig.mirror {
		ps = mirrored(ps)
	}
	if h.key == "" {
		attach = "hold"
	}
	return r.heldPoint(rig, h, wx, wy, ps, attach)
}

// shotPoints are a shot's muzzle and ejection port in world tiles: from
// the shooter's rifle when it is drawn, else just in front of the
// shooter's tile.
func (r *Renderer) shotPoints(t game.TraceView, pawns []game.PawnView) (muzzle, port vec) {
	ux, uy := unit(float64(t.ToX-t.FromX), float64(t.ToY-t.FromY))
	c := vec{float64(t.FromX) + 0.5, float64(t.FromY) + 0.3}
	muzzle, port = vec{c.x + ux*0.5, c.y + uy*0.5}, vec{c.x + ux*0.2, c.y + uy*0.2}
	p, ok := pawnByStrike(pawns, t)
	if !ok {
		return muzzle, port
	}
	mz, okM := r.pawnHeldPoint(p, "muzzle")
	if !okM {
		return muzzle, port
	}
	port = c
	if g, okG := r.pawnHeldPoint(p, "grip"); okG {
		port = vec{g.x + (mz.x-g.x)*0.25, g.y + (mz.y-g.y)*0.25}
	}
	return mz, port
}

// casingDir is where a spent case flies for a shot along (ux, uy): out
// of the rifle's right side and a little back.
func casingDir(ux, uy float64) (float64, float64) {
	return -uy*0.7 - ux*0.3, ux*0.7 - uy*0.3
}

// animateCombat spawns, once per trace: hit splatter (blood sprayed away
// from the attacker, chips and sparks off buildings), muzzle smoke at the
// muzzle and an ejected shell casing.
func (r *Renderer) animateCombat(w game.WorldView, pawns []game.PawnView) {
	st := r.animState()
	now := uint64(r.game.Tick())
	st.trackReactions(w.Traces, now, r.present)
	for _, t := range w.Traces {
		key := traceKey(t, now)
		if r.seenTrace[key] {
			continue
		}
		r.seenTrace[key] = true
		r.traceSound(t)
		seed := key[0] ^ key[1] ^ key[2]
		r.floatTrace(t, seed)
		ux, uy := unit(float64(t.ToX-t.FromX), float64(t.ToY-t.FromY))
		switch {
		case t.Hit && t.Building:
			r.burst(partChip, float64(t.ToX)+0.5, float64(t.ToY)+0.4, 5, seed)
			if !t.Melee {
				r.burst(partSpark, float64(t.ToX)+0.5, float64(t.ToY)+0.4, 3, seed^0x5a)
			}
		case t.Hit: // blood_draw.go: scaled by damage, ichor from insects
			kind, x, y := sprayKind(w, t.Target), float64(t.ToX)+0.5, float64(t.ToY)+0.35
			n, speed := sprayFor(t.Damage)
			r.burstDir(kind, x, y, n, seed, ux, uy, speed)
			if t.Melee { // a few drops flicked back toward the attacker
				r.burstDir(kind, x, y, 2, seed^0xb10d, -ux, -uy, 0.6*bloodSpeed)
			}
		}
		if !t.Melee {
			mz, port := r.shotPoints(t, pawns)
			r.burstDir(partSmoke, mz.x, mz.y, 2, key[2], ux, uy, 0.004)
			cx, cy := casingDir(ux, uy)
			r.burstDir(partCasing, port.x, port.y, 1, seed^0xca5e, cx, cy, casingSpeed)
		}
	}
	if len(r.seenTrace) > 256 {
		r.seenTrace = map[[3]uint64]bool{}
	}
}
