package ebiten

import (
	"math"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Combat presentation (ART §27.2): attack and hit reactions, combat
// particles, aiming and firing poses, falls. Owned by the combat
// animation work; the dispatch in pawn_anim.go calls in here.
//
// Timing (§27.2, C7): wind-up frames follow AttackCycle; strike frames
// land when the attack's trace is first seen (its birth tick, Tick-Age);
// the follow-through runs on animT from there. Every timed effect lasts
// at least minReactionFrames present frames.

// Combat timing in sim ticks, and reaction sizes in tiles.
const (
	aimAt        = 0.6  // AttackCycle at which the rifle is shouldered
	lowReady     = 0.55 // radians the muzzle drops at low ready
	fireTicks    = 9.0  // fire1..3
	lowerTicks   = 12.0 // after fire3, easing back towards low ready
	recoilTicks  = 6.0
	recoilDist   = 0.08
	meleeTicks   = 10.0 // strike + follow-through
	strikeWindow = 30.0 // a strike older than this no longer shapes the pose
	hitTicks     = 12.0 // flinch1..3 and the hit flash
	staggerTicks = 10.0
	staggerDist  = 0.12
	fallStep     = 4.0 // present frames per fall frame
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

// flinchFrame is the flinch frame at a hit's age.
func flinchFrame(age float64) string {
	switch {
	case age < 3:
		return "_flinch1"
	case age < 7:
		return "_flinch2"
	}
	return "_flinch3"
}

// hitReaction is the flinch layer while a hit is fresh (HitTick): flinch
// frames and a stagger away from the attacker, with a flash.
func (r *Renderer) hitReaction(p game.PawnView, m moveState) actionPose {
	age, ok := r.hitAge(p.ID, p.HitTick, p.X, p.Y)
	if !ok || age >= hitTicks {
		return actionPose{}
	}
	ux, uy := awayFrom(p.X, p.Y, p.HitFromX, p.HitFromY, m.Facing)
	k := staggerK(age)
	a := actionPose{ok: true, suffix: flinchFrame(age),
		dx: ux * staggerDist * k, dy: uy * staggerDist * k, rot: ux * 0.1 * k,
		flash: float32(1 - age/hitTicks)}
	if onDuty(p) {
		a.held = readyHeld(p)
	}
	return a
}

// meleeSet is a melee weapon's pose set and frame count (strike at 4 of
// 6; punches strike on 2 or 3 of 4).
func meleeSet(weapon string) (string, int) {
	switch weapon {
	case "knife":
		return "_stab", 6
	case "spear":
		return "_thrust", 6
	case "club":
		return "_swing", 6
	}
	return "_punch", 4
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

// meleeFrame is the melee frame and lunge weight (-1 back .. 1 full
// reach): wind-up by AttackCycle, strike and follow-through by the age
// of the last strike. alt picks the cross over the jab.
func meleeFrame(weapon string, cycle, age float64, struck, alt bool) (string, float64) {
	set, n := meleeSet(weapon)
	fr := func(i int) string { return set + strconv.Itoa(i) }
	if struck && age < meleeTicks {
		k := 1.0
		if age >= 3 {
			k = 1 - (age-3)/(meleeTicks-3)
		}
		if n == 4 {
			switch {
			case age < 3 && alt:
				return fr(3), k
			case age < 3:
				return fr(2), k
			}
			return fr(4), k
		}
		switch {
		case age < 3:
			return fr(4), k
		case age < 6:
			return fr(5), k
		}
		return fr(6), k
	}
	if n == 4 {
		return fr(1), 0
	}
	switch {
	case cycle < 0.5:
		return fr(1), 0
	case cycle < 0.8:
		t := (cycle - 0.5) / 0.3
		return fr(2), -0.3 * t * t * (3 - 2*t) // chamber back
	}
	return fr(3), -0.3 + 0.6*clamp01((cycle-0.8)/0.2) // step in
}

// aimFrame is the rifle frame, how far it is raised (0 low ready .. 1
// shouldered) and the muzzle rise (radians): ready blends to aim over
// AttackCycle 0..aimAt, fire1..3 follow the shot, then it eases down.
func aimFrame(cycle, age float64, struck bool) (string, float64, float64) {
	if struck && age < fireTicks {
		rise := 0.3 * math.Sin(math.Pi*age/fireTicks)
		switch {
		case age < 2:
			return "_fire1", 1, rise
		case age < 5:
			return "_fire2", 1, rise
		}
		return "_fire3", 1, rise
	}
	k := clamp01(cycle / aimAt)
	if struck {
		k = math.Max(k, clamp01(1-(age-fireTicks)/lowerTicks))
	}
	k = k * k * (3 - 2*k)
	if k >= 0.5 {
		return "_aim", k, 0
	}
	return "_ready", k, 0
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
	for _, an := range r.drawW.Animals {
		if an.ID == id {
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

// combatPose is the aim/fire/melee layer while attacking.
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
	}
	moving := m.Blend >= minBlend
	a.held.aim, a.held.aimX, a.held.aimY = hasT, tgt.x, tgt.y
	if p.Action == game.ActionAim || p.Action != game.ActionMelee && struck && !st.Melee {
		sfx, raise, rise := aimFrame(p.AttackCycle, age, struck)
		a.held.angle = rise - (1-raise)*lowReady
		if struck && age < recoilTicks {
			k := 1 - age/recoilTicks
			a.dx, a.dy = -ux*recoilDist*k, -uy*recoilDist*k
		}
		if !moving {
			a.suffix = sfx
		}
		return a
	}
	alt := int64(st.born)%2 == 1
	sfx, lunge := meleeFrame(p.Weapon, p.AttackCycle, age, struck, alt)
	a.held.aim = hasT && (p.Weapon == "knife" || p.Weapon == "spear") // thrusts point at the target
	if !moving {
		reach := meleeReach(p.Weapon)
		a.suffix = sfx
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

// fallPose is a pawn lying on the bare ground. Downed or dead it plays
// fall1..4 upright (sliding away from the last hit), then tips over
// away from that hit; asleep it just lies down. upright reports a
// figure drawn in its facing with the frame suffix; otherwise the plain
// south figure.
func (r *Renderer) fallPose(p game.PawnView, vis pawnVisual) (suffix string, ps pose, upright bool) {
	e := float64(r.present - vis.lyingSince)
	side := 1.0
	if vis.facing == faceWest {
		side = -1
	}
	ps = pose{px: figurePivotX, py: figurePivotY}
	breath := 1 + 0.02*math.Sin(2*math.Pi*float64(r.present+uint64(p.ID)*37)/110)
	if p.Status == "alive" {
		k := easeOut(e / fallFrames)
		ps.rot, ps.dx, ps.dy, ps.sy = side*math.Pi/2*k, -side*0.42*k, 0.1*k, breath
		return "", ps, false
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
	buckle := 4 * fallStep
	total := buckle + fallFrames
	drift := 0.12 * easeOut(e/total)
	ps.dx, ps.dy = ux*drift, uy*drift
	ps.flash = float32(0.5 * (1 - clamp01(e/total)))
	if e < buckle {
		ps.sy = 1 - 0.06*e/buckle
		return "_fall" + strconv.Itoa(1+int(e/fallStep)), ps, true
	}
	k := easeOut((e - buckle) / fallFrames)
	ps.rot = side * math.Pi / 2 * k
	ps.dx += -side * 0.42 * k
	ps.dy += 0.1 * k
	if p.Status != "dead" {
		ps.sy = breath
	}
	return "_fall4", ps, true
}

// drawFigureFacing draws a pawn's layers in a facing and body frame,
// hair and hats riding the frame's head (turned with the pose).
func (r *Renderer) drawFigureFacing(screen *ebiten.Image, p game.PawnView, f facing, suffix string, wx, wy, tile float64, ps pose) {
	hdx, hdy := headDelta(r.art, f, suffix)
	c, s := math.Cos(ps.rot), math.Sin(ps.rot)
	headPs := ps
	headPs.dx += hdx*c - hdy*s
	headPs.dy += hdx*s + hdy*c
	blood := r.pawnBlood(p) // body and apparel only (blood_draw.go)
	for _, ly := range r.pawnLayers(r.look(p), f, p.Apparel...) {
		lp := ps
		if isHeadLayer(ly.key) {
			lp = headPs
		}
		r.figureBlood = layerBlood(ly.key, blood)
		r.drawSpritePose(screen, r.framed(ly.key, suffix), ly.tint, wx, wy, tile, lp)
	}
	r.figureBlood = 0
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
// right now, in world tiles ("tip" with bare hands is the hand).
func (r *Renderer) pawnHeldPoint(p game.PawnView, attach string) (vec, bool) {
	if p.Lying || p.CarriedBy != 0 || r.art == nil {
		return vec{}, false
	}
	wx, wy := r.interpWorld(p)
	m := r.pawnMotion(p.ID)
	m.Facing = r.poseFacing(p, m)
	ps, suffix, h := r.standingPose(p, m)
	if h.key == "" {
		attach = "hand"
	}
	return r.heldPoint(m.Facing, suffix, h, wx, wy, ps, attach)
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
