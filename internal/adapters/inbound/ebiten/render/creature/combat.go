package creature

import (
	"math"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Combat presentation (ART §27.2): attack and hit reactions, combat
// particles, aiming and firing poses, falls. Owned by the combat
// animation work; the dispatch in pawn.go calls in here.
//
// Timing (§27.2, C7): the wind-up follows AttackCycle; the strike (the
// lunge's peak, a club's blow, the muzzle kick) lands when the attack's
// trace is first seen (its birth tick, Tick-Age); the follow-through runs
// on animT from there. Every timed effect lasts at least
// minReactionFrames present frames.

// Combat timing in sim ticks, and reaction sizes in tiles.
const (
	aimAt        = 0.6 // AttackCycle at which an animal's wind-up turns to frame 2
	FireTicks    = 9.0 // the shot's muzzle kick and settle
	MuzzleKick   = 0.3 // radians the muzzle rises at the kick's peak
	recoilTicks  = 6.0
	recoilDist   = 0.08 // the body's recoil, tiles
	recoilPull   = 0.06 // the rifle pulled back along the aim, tiles
	MeleeTicks   = 10.0 // strike + follow-through
	strikeWindow = 30.0 // a strike older than this no longer shapes the pose
	hitTicks     = 12.0 // the flinch and the hit flash
	StaggerTicks = 10.0
	StaggerDist  = 0.12
	BloodSpeed   = 0.03
	casingSpeed  = 0.02
)

// --- reactions from attack traces ---------------------------------------

// minReactionFrames is how long a recoil, lunge or flash stays visible in
// present frames, however fast the simulation runs (ART §27 clocks).
const minReactionFrames = 8

// ReactTrace is a trace as the reaction sees it: sim is its age in sim
// ticks (+Inf once the sim dropped it), frames its age in present frames
// and born the sim tick it happened.
type ReactTrace struct {
	game.TraceView
	Sim, Frames float64
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

// AttackReaction offsets an actor at (x, y): rifle recoil, melee lunge,
// and knockback plus a flash when struck, timed by sim age alone.
func AttackReaction(traces []game.TraceView, x, y int) (dx, dy float64, flash float32) {
	rs := make([]ReactTrace, len(traces))
	for i, t := range traces {
		rs[i] = ReactTrace{TraceView: t, Sim: float64(t.Age), Frames: math.Inf(1)}
	}
	return AttackReactionF(rs, x, y)
}

// AttackReactionF is attackReaction with the minimum on-screen duration.
func AttackReactionF(traces []ReactTrace, x, y int) (dx, dy float64, flash float32) {
	ax, ay := attackerNudge(traces, x, y)
	sx, sy, flash := struckNudge(traces, x, y)
	return ax + sx, ay + sy, flash
}

// attackerNudge is the rifle recoil or melee lunge of an attacker at (x, y).
func attackerNudge(traces []ReactTrace, x, y int) (dx, dy float64) {
	for _, t := range traces {
		if t.FromX != x || t.FromY != y || (t.ToX == x && t.ToY == y) {
			continue
		}
		ux, uy := motion.Unit(float64(t.ToX-x), float64(t.ToY-y))
		switch {
		case t.Melee && reactionAge(t.Sim, t.Frames, 10) < 10:
			k := math.Sin(math.Pi * reactionAge(t.Sim, t.Frames, 10) / 10)
			dx, dy = dx+ux*0.22*k, dy+uy*0.22*k
		case !t.Melee && reactionAge(t.Sim, t.Frames, 6) < 6:
			k := 1 - reactionAge(t.Sim, t.Frames, 6)/6
			dx, dy = dx-ux*0.08*k, dy-uy*0.08*k
		}
	}
	return dx, dy
}

// struckNudge is the knockback and flash of a creature hit at (x, y).
func struckNudge(traces []ReactTrace, x, y int) (dx, dy float64, flash float32) {
	for _, t := range traces {
		if t.ToX != x || t.ToY != y || !t.Hit || (t.FromX == x && t.FromY == y) {
			continue
		}
		if age := reactionAge(t.Sim, t.Frames, 12); age < 12 {
			k := 1 - age/12
			ux, uy := motion.Unit(float64(x-t.FromX), float64(y-t.FromY))
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

// TrackReactions remembers every trace for minReactionFrames present
// frames after the sim drops it.
func (c *Set) TrackReactions(traces []game.TraceView, tick, present uint64) {
	for _, rec := range c.Recs {
		rec.live = false
	}
	for _, t := range traces {
		k := traceKey(t, tick)
		rec, ok := c.Recs[k]
		if !ok {
			rec = &reactRec{seen: present, born: k[2]}
			c.Recs[k] = rec
		}
		rec.trace, rec.live = t, true
	}
	for k, rec := range c.Recs {
		if !rec.live && present-rec.seen > 3*minReactionFrames {
			delete(c.Recs, k)
		}
	}
}

// Reactions appends the tracked traces, aged for display.
func (c *Set) Reactions(present uint64, dst []ReactTrace) []ReactTrace {
	for _, rec := range c.Recs {
		rt := ReactTrace{TraceView: rec.trace, Sim: math.Inf(1), Frames: float64(present - rec.seen), born: float64(rec.born)}
		if rec.live {
			rt.Sim = float64(rec.trace.Age)
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

// LastStrike is the newest attack by id and how long ago it landed, in
// display ticks (animT since its birth, stretched to at least
// minReactionFrames over dur). The first sighting of a trace is age 0.
func (c *Set) LastStrike(v *render.View, id entity.EntityID, x, y int, dur float64) (ReactTrace, float64, bool) {
	now := v.AnimT
	var best ReactTrace
	bestAge := math.Inf(1)
	for _, rt := range c.TraceList(v, v.W) {
		if !strikeBy(rt.TraceView, id, x, y) {
			continue
		}
		if age := reactionAge(math.Max(0, now-rt.born), rt.Frames, dur); age < bestAge {
			best, bestAge = rt, age
		}
	}
	return best, bestAge, bestAge < strikeWindow
}

// HitAge is how long ago creature id was last hit (HitTick), in display
// ticks; false when it never was.
func (c *Set) HitAge(v *render.View, id entity.EntityID, hitTick uint64, x, y int) (float64, bool) {
	if hitTick == 0 {
		return 0, false
	}
	el := math.Max(0, v.AnimT-float64(hitTick))
	frames := math.Inf(1)
	for _, rt := range c.TraceList(v, v.W) {
		mine := rt.Target == id || rt.Target == 0 && rt.ToX == x && rt.ToY == y
		if mine && rt.Hit && math.Abs(rt.born-float64(hitTick)) <= 1 {
			frames = math.Min(frames, rt.Frames)
		}
	}
	return reactionAge(el, frames, StaggerTicks), true
}

// staggerK is the stagger weight at a hit's age: a sharp jolt over 3
// ticks, back to rest by staggerTicks.
func staggerK(age float64) float64 {
	switch {
	case age < 3:
		return motion.EaseOut(age / 3)
	case age < StaggerTicks:
		t := (age - 3) / (StaggerTicks - 3)
		return 1 - t*t*(3-2*t)
	}
	return 0
}

// awayFrom is the unit direction from (fx, fy) to (x, y); on the same
// tile it is backwards from the facing.
func awayFrom(x, y, fx, fy int, f art.Facing) (float64, float64) {
	if ux, uy := motion.Unit(float64(x-fx), float64(y-fy)); ux != 0 || uy != 0 {
		return ux, uy
	}
	ux, uy := facingUnit(f)
	return -ux, -uy
}

func facingUnit(f art.Facing) (float64, float64) {
	switch f {
	case art.FaceEast:
		return 1, 0
	case art.FaceWest:
		return -1, 0
	case art.FaceNorth:
		return 0, -1
	}
	return 0, 1
}

// --- pose layers ----------------------------------------------------------

// HitReaction is the flinch layer while a hit is fresh (HitTick): a
// stagger and twist away from the attacker, with a flash.
func (c *Set) HitReaction(v *render.View, p game.PawnView, m motion.MoveState) ActionPose {
	age, ok := c.HitAge(v, p.ID, p.HitTick, p.X, p.Y)
	if !ok || age >= hitTicks {
		return ActionPose{}
	}
	ux, uy := awayFrom(p.X, p.Y, p.HitFromX, p.HitFromY, m.Facing)
	k := staggerK(age)
	a := ActionPose{Ok: true,
		Dx: ux * StaggerDist * k, Dy: uy * StaggerDist * k, Rot: ux * 0.1 * k,
		Flash: float32(1 - age/hitTicks)}
	// what the pawn holds is kept through the flinch: the aimed weapon,
	// a hauled load, or the carried weapon on duty
	for i := layerCombat; i <= layerCarry; i++ {
		if b := c.actionLayer(v, i, p, m); b.Ok && b.Held.Set {
			a.Held = b.Held
			return a
		}
	}
	if onDuty(p) {
		a.Held = ReadyHeld(p)
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
	KnifePush   = 0.10
	spearPush   = 0.18
	FistPush    = 0.14
	ClubRest    = 0.0
	ClubRaised  = 0.8
	ClubStrike  = -0.45
	chamberBack = -0.3 // the lunge weight while chambering a thrust
)

// MeleeMotion is a melee attack's shape: the body's lunge weight (-1 back
// .. 1 full reach), the weapon's turn and its push along the aim (tiles).
// The wind-up follows AttackCycle; the strike and follow-through follow
// the age of the last strike (age 0 on the trace). Knives and spears
// jab (pushed forward at the lunge), clubs swing down through a short
// arc, fists punch (a lunge; push is the punch's reach, which the melee
// trail follows).
func MeleeMotion(weapon string, cycle, age float64, struck bool) (lunge, angle, push float64) {
	if struck && age < MeleeTicks {
		lunge = 1
		if age >= 3 {
			lunge = 1 - (age-3)/(MeleeTicks-3)
		}
	} else if weapon != "fists" && weapon != "" {
		switch {
		case cycle < 0.5:
		case cycle < 0.8:
			t := (cycle - 0.5) / 0.3
			lunge = chamberBack * t * t * (3 - 2*t)
		default:
			lunge = chamberBack + 0.6*gfx.Clamp01((cycle-0.8)/0.2) // step in
		}
	}
	reach := math.Max(lunge, 0)
	switch weapon {
	case "knife":
		push = KnifePush * reach
	case "spear":
		push = spearPush * reach
	case "club":
		switch {
		case struck && age < 3:
			angle = ClubStrike
		case struck && age < MeleeTicks:
			angle = ClubStrike + (ClubRest-ClubStrike)*motion.EaseOut((age-3)/(MeleeTicks-3))
		default:
			t := gfx.Clamp01((cycle - 0.5) / 0.5)
			angle = ClubRest + (ClubRaised-ClubRest)*t*t*(3-2*t)
		}
	default: // fists
		push = FistPush * reach
	}
	return lunge, angle, push
}

// AimKick is the muzzle kick (radians) of a shot age display ticks old:
// up and back down over fireTicks from the trace's first sighting. The
// rifle is aimed for the whole attack cycle, so nothing else moves it.
func AimKick(age float64, struck bool) float64 {
	if struck && age < FireTicks {
		return MuzzleKick * math.Sin(math.Pi*age/FireTicks)
	}
	return 0
}

// creatureSpot is where creature id is drawn (its body centre), by its
// motion or else its tile.
func (c *Set) creatureSpot(v *render.View, id entity.EntityID) (Vec, bool) {
	if id == 0 {
		return Vec{}, false
	}
	if m, ok := c.motionOf(v, id); ok {
		return Vec{X: m.X + 0.5, Y: m.Y + 0.4}, true
	}
	for _, q := range v.Pawns {
		if q.ID == id {
			return Vec{X: float64(q.X) + 0.5, Y: float64(q.Y) + 0.4}, true
		}
	}
	for i := range v.W.Animals {
		if an := &v.W.Animals[i]; an.ID == id {
			return Vec{X: float64(an.X) + 0.5, Y: float64(an.Y) + 0.4}, true
		}
	}
	return Vec{}, false
}

// attackTarget is the point a pawn attacks: its ActTarget, else the
// target of its last strike.
func (c *Set) attackTarget(v *render.View, p game.PawnView, st ReactTrace, struck bool) (Vec, bool) {
	if pt, ok := c.creatureSpot(v, p.ActTarget); ok {
		return pt, true
	}
	if !struck {
		return Vec{}, false
	}
	if pt, ok := c.creatureSpot(v, st.Target); ok {
		return pt, true
	}
	return Vec{X: float64(st.ToX) + 0.5, Y: float64(st.ToY) + 0.4}, true
}

// CombatPose is the aim/fire/melee layer while attacking: the weapon is
// aimed at the target (carried when there is none), kicks and pulls back
// on a shot, jabs or swings on a blow.
func (c *Set) CombatPose(v *render.View, p game.PawnView, m motion.MoveState) ActionPose {
	dur := MeleeTicks
	if p.Weapon == "rifle" {
		dur = FireTicks
	}
	st, age, struck := c.LastStrike(v, p.ID, p.X, p.Y, dur)
	if !struck && p.Action != game.ActionAim && p.Action != game.ActionMelee {
		return ActionPose{}
	}
	a := ActionPose{Ok: true, Held: ReadyHeld(p)}
	tgt, hasT := c.attackTarget(v, p, st, struck)
	wx, wy := c.pawnAt(v, p)
	ux, uy := facingUnit(m.Facing)
	if hasT {
		if vx, vy := motion.Unit(tgt.X-wx-0.5, tgt.Y-wy-0.4); vx != 0 || vy != 0 {
			ux, uy = vx, vy
		}
		a.Held.Stance, a.Held.AimX, a.Held.AimY = StanceAimed, tgt.X, tgt.Y
	}
	moving := m.Blend >= motion.MinBlend
	if p.Action == game.ActionAim || p.Action != game.ActionMelee && struck && !st.Melee {
		a.Held.Angle = AimKick(age, struck)
		if struck && age < recoilTicks {
			k := 1 - age/recoilTicks
			a.Dx, a.Dy = -ux*recoilDist*k, -uy*recoilDist*k
			a.Held.Push = -recoilPull * k
		}
		return a
	}
	lunge, angle, push := MeleeMotion(p.Weapon, p.AttackCycle, age, struck)
	a.Held.Angle, a.Held.Push = angle, push
	if !moving {
		reach := meleeReach(p.Weapon)
		a.Dx, a.Dy, a.Rot = ux*reach*lunge, uy*reach*lunge, ux*0.06*lunge
	}
	return a
}

// CombatOffset is the knockback nudge applied on top of any layer for a
// simulation that does not stamp hits (HitTick); recoil and lunges are
// part of CombatPose and stagger of HitReaction.
func (c *Set) CombatOffset(v *render.View, p game.PawnView) (dx, dy float64, flash float32) {
	if p.HitTick != 0 {
		return 0, 0, 0
	}
	return struckNudge(c.Reactions(v.Present, nil), p.X, p.Y)
}

// --- falling --------------------------------------------------------------

// FallPose is a pawn lying on the bare ground, drawn as its standing
// south figure turned about the feet. Downed or dead it buckles (a short
// squash, sliding away from the last hit), then tips over away from that
// hit over fallFrames; asleep it just lies down.
func (c *Set) FallPose(v *render.View, p game.PawnView, vis Visual) motion.Pose {
	e := float64(v.Present - vis.LyingSince)
	side := 1.0
	if vis.Facing == art.FaceWest {
		side = -1
	}
	ps := StandPose()
	breath := 1 + 0.02*math.Sin(2*math.Pi*float64(v.Present+uint64(p.ID)*37)/110)
	if p.Status == "alive" {
		k := motion.EaseOut(e / FallFrames)
		lp := LyingPose(v.Art, side)
		ps.Rot, ps.Dx, ps.Dy, ps.Sy = lp.Rot*k, lp.Dx*k, lp.Dy*k, breath
		return ps
	}
	var ux, uy float64
	if p.HitTick != 0 {
		ux, uy = motion.Unit(float64(p.X-p.HitFromX), float64(p.Y-p.HitFromY))
		switch {
		case ux > 0.01:
			side = 1
		case ux < -0.01:
			side = -1
		}
	}
	total := float64(BuckleFrames + FallFrames)
	drift := 0.12 * motion.EaseOut(e/total)
	ps.Dx, ps.Dy = ux*drift, uy*drift
	ps.Flash = float32(0.5 * (1 - gfx.Clamp01(e/total)))
	if e < BuckleFrames {
		ps.Sy = 1 - 0.06*e/BuckleFrames
		return ps
	}
	k := motion.EaseOut((e - BuckleFrames) / FallFrames)
	lp := LyingPose(v.Art, side)
	ps.Rot = lp.Rot * k
	ps.Dx += lp.Dx * k
	ps.Dy += lp.Dy * k
	if p.Status != "dead" {
		ps.Sy = breath
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

// PawnHeldPoint is where an attachment of what a standing pawn holds is
// right now, in world tiles (with bare hands, the body's hold).
func (c *Set) PawnHeldPoint(v *render.View, p game.PawnView, attach string) (Vec, bool) {
	if p.Lying || p.CarriedBy != 0 || v.Art == nil {
		return Vec{}, false
	}
	wx, wy := c.pawnAt(v, p)
	m := c.motion(v, p.ID)
	m.Facing = c.PoseFacing(v, p, m)
	ps, h := c.StandingPose(v, p, m)
	rig := c.bodyRig(v, p, m.Facing)
	if rig.Mirror {
		ps = Mirrored(ps)
	}
	if h.Key == "" {
		attach = "hold"
	}
	return c.HeldPoint(v, rig, h, wx, wy, ps, attach)
}

// ShotPoints are a shot's muzzle and ejection port in world tiles: from
// the shooter's rifle when it is drawn, else just in front of the
// shooter's tile.
func (c *Set) ShotPoints(v *render.View, t game.TraceView, pawns []game.PawnView) (muzzle, port Vec) {
	ux, uy := motion.Unit(float64(t.ToX-t.FromX), float64(t.ToY-t.FromY))
	at := Vec{X: float64(t.FromX) + 0.5, Y: float64(t.FromY) + 0.3}
	muzzle, port = Vec{X: at.X + ux*0.5, Y: at.Y + uy*0.5}, Vec{X: at.X + ux*0.2, Y: at.Y + uy*0.2}
	p, ok := pawnByStrike(pawns, t)
	if !ok {
		return muzzle, port
	}
	mz, okM := c.PawnHeldPoint(v, p, "muzzle")
	if !okM {
		return muzzle, port
	}
	port = at
	if g, okG := c.PawnHeldPoint(v, p, "grip"); okG {
		port = Vec{X: g.X + (mz.X-g.X)*0.25, Y: g.Y + (mz.Y-g.Y)*0.25}
	}
	return mz, port
}

// CasingDir is where a spent case flies for a shot along (ux, uy): out
// of the rifle's right side and a little back.
func CasingDir(ux, uy float64) (float64, float64) {
	return -uy*0.7 - ux*0.3, ux*0.7 - uy*0.3
}

// AnimateCombat spawns, once per trace: hit splatter (blood sprayed away
// from the attacker, chips and sparks off buildings), muzzle smoke at the
// muzzle and an ejected shell casing.
func (c *Set) AnimateCombat(v *render.View, w game.WorldView, pawns []game.PawnView) {
	now := uint64(v.Tick)
	c.TrackReactions(w.Traces, now, v.Present)
	for _, t := range w.Traces {
		key := traceKey(t, now)
		if c.seenTrace[key] {
			continue
		}
		c.seenTrace[key] = true
		c.Host.TraceSound(t)
		seed := key[0] ^ key[1] ^ key[2]
		c.FloatTrace(v, t, seed)
		ux, uy := motion.Unit(float64(t.ToX-t.FromX), float64(t.ToY-t.FromY))
		switch {
		case t.Hit && t.Building:
			c.Burst(v, PartChip, float64(t.ToX)+0.5, float64(t.ToY)+0.4, 5, seed)
			if !t.Melee {
				c.Burst(v, PartSpark, float64(t.ToX)+0.5, float64(t.ToY)+0.4, 3, seed^0x5a)
			}
		case t.Hit: // Host.Spray: scaled by damage, ichor from insects
			kind, n, speed := c.Host.Spray(w, t.Target, t.Damage)
			x, y := float64(t.ToX)+0.5, float64(t.ToY)+0.35
			c.BurstDir(v, kind, x, y, n, seed, ux, uy, speed)
			if t.Melee { // a few drops flicked back toward the attacker
				c.BurstDir(v, kind, x, y, 2, seed^0xb10d, -ux, -uy, 0.6*BloodSpeed)
			}
		}
		if !t.Melee {
			mz, port := c.ShotPoints(v, t, pawns)
			c.BurstDir(v, PartSmoke, mz.X, mz.Y, 2, key[2], ux, uy, 0.004)
			cx, cy := CasingDir(ux, uy)
			c.BurstDir(v, PartCasing, port.X, port.Y, 1, seed^0xca5e, cx, cy, casingSpeed)
		}
	}
	if len(c.seenTrace) > 256 {
		c.seenTrace = map[[3]uint64]bool{}
	}
}
