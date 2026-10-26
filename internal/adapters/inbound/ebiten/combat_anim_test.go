package ebiten

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// combatRenderer is a renderer at sim tick `tick` drawing these pawns and
// world (traces untracked, so aged by the sim like a capture).
func combatRenderer(tick uint64, pawns []game.PawnView, w game.WorldView) *Renderer {
	r := New(&stubSim{w: 64, h: 64, tick: clock.Tick(tick)})
	r.art = fakeCombatArt()
	r.drawPawns, r.drawW = pawns, w
	return r
}

var eastIdle = moveState{Facing: faceEast}

func shooter() game.PawnView {
	return game.PawnView{ID: 1, X: 5, Y: 5, Weapon: "rifle", Drafted: true, Action: game.ActionAim, ActTarget: 2}
}

func targetAt(x, y int) game.PawnView { return game.PawnView{ID: 2, X: x, Y: y, Hostile: true} }

// shot is a trace by pawn 1 on pawn 2, first seen with this Age.
func shot(age int, melee bool) game.TraceView {
	return game.TraceView{FromX: 5, FromY: 5, ToX: 9, ToY: 5, Age: age, Hit: true, Melee: melee,
		Attacker: 1, Target: 2, Damage: 9}
}

// aimCentre is the posed body centre of an east-facing figure at (wx, wy)
// on screen: where an aimed item is placed from.
func aimCentre(r *Renderer, rig heldRig, ps pose, wx, wy, tile float64) pt {
	c, _ := bodyCentre(rig.body.meta)
	sx, sy := r.worldToScreenF(wx, wy)
	bx, by, s := artOrigin(rig.body.meta, float64(sx), float64(sy), tile)
	return apply(ps.geo(float64(sx), float64(sy), tile), pt{bx + c[0]*s, by + c[1]*s})
}

// The rifle is aimed at the target for the whole attack cycle (no low
// ready to blend from: the carried pose already reads low); only the
// shot's muzzle kick moves it.
func TestAimBlendFollowsAttackCycle(t *testing.T) {
	t.Parallel()
	if k := aimKick(0, false); k != 0 {
		t.Fatalf("no shot: no kick (%v)", k)
	}
	if k := aimKick(fireTicks/2, true); math.Abs(k-muzzleKick) > 1e-9 {
		t.Fatalf("the kick peaks mid-shot: %v", k)
	}
	if k := aimKick(fireTicks, true); k != 0 {
		t.Fatalf("settled by fireTicks: %v", k)
	}
	p := shooter()
	r := combatRenderer(100, []game.PawnView{p, targetAt(9, 5)}, game.WorldView{})
	var first actionPose
	for i, c := range []float64{0, 0.3, 0.6, 0.99} {
		p.AttackCycle = c
		a := r.combatPose(p, eastIdle)
		if !a.ok || a.held.key != "weapon/rifle_basic" || a.held.stance != stanceAimed || a.held.angle != 0 || a.held.push != 0 {
			t.Fatalf("cycle %.2f: aimed, steady: %+v", c, a)
		}
		if i == 0 {
			first = a
		} else if a != first {
			t.Fatalf("cycle %.2f: the aim does not follow the cycle: %+v vs %+v", c, a, first)
		}
	}
	// walking to the target: still aimed
	if a := r.combatPose(p, moveState{Facing: faceEast, Blend: 1, Moving: true}); a.held.stance != stanceAimed || a.held.angle != 0 {
		t.Fatalf("walking while aiming keeps the aim: %+v", a)
	}
	if a := r.combatPose(game.PawnView{ID: 3, Weapon: "rifle"}, eastIdle); a.ok {
		t.Fatal("not attacking: no combat layer")
	}
}

func TestFireAnchoredToTrace(t *testing.T) {
	t.Parallel()
	p := shooter()
	p.AttackCycle = 1
	angle := func(age int) actionPose {
		r := combatRenderer(100, []game.PawnView{p, targetAt(9, 5)}, game.WorldView{Traces: []game.TraceView{shot(age, false)}})
		return r.combatPose(p, eastIdle)
	}
	// the shot kicks the muzzle up from the trace's first sighting and
	// settles by fireTicks; recoil pushes the body back from the target
	// and pulls the rifle back along the aim at once
	if a := angle(0); a.held.angle != 0 || a.dx >= 0 || a.held.push >= 0 {
		t.Fatalf("age 0: on the target, recoiling: %+v", a)
	}
	mid := angle(4)
	if mid.held.angle < 0.25 {
		t.Fatalf("mid-shot muzzle kick: %v", mid.held.angle)
	}
	if a := angle(15); a.held.angle != 0 || a.dx != 0 || a.held.push != 0 {
		t.Fatalf("settled after the shot: %+v", a)
	}
	// the pull moves the rifle back towards the shooter
	r := combatRenderer(100, []game.PawnView{p, targetAt(9, 5)}, game.WorldView{})
	rig, ps := rigFor(r, faceEast)
	h := heldItem{key: "weapon/rifle_basic", stance: stanceAimed, aimX: 9.5, aimY: 5.4}
	rest, _ := r.heldPoint(rig, h, 5, 5, ps, "muzzle")
	h.push = angle(0).held.push
	pulled, _ := r.heldPoint(rig, h, 5, 5, ps, "muzzle")
	if pulled.x >= rest.x {
		t.Fatalf("recoil pulls the rifle back: %v -> %v", rest, pulled)
	}
	// a fast sim cannot skip the shot: 2 frames after the first sighting
	// it is still kicking even though 30 ticks went by
	st := r.animState()
	st.trackReactions([]game.TraceView{shot(1, false)}, 100, 50)
	r.game.(*stubSim).tick = 131
	r.present = 52
	if a := r.combatPose(p, eastIdle); a.held.angle <= 0 || a.dx >= 0 {
		t.Fatalf("min visible frames: %+v", a)
	}
	// another pawn's shot does not fire this one
	other := shot(1, false)
	other.Attacker, other.FromX = 7, 6
	r = combatRenderer(100, []game.PawnView{p}, game.WorldView{Traces: []game.TraceView{other}})
	p.AttackCycle = 0.1
	if a := r.combatPose(p, eastIdle); a.held.angle != 0 || a.dx != 0 || a.held.push != 0 {
		t.Fatalf("someone else's trace: %+v", a)
	}
}

// The aimed rifle points from the body centre at the target all the way
// round, flipped when aiming left so it stays upright, behind the body
// only when aiming into the north half.
func TestRifleAimsAtTarget(t *testing.T) {
	t.Parallel()
	p := shooter()
	p.AttackCycle = 0.8
	r := combatRenderer(100, []game.PawnView{p, targetAt(9, 3)}, game.WorldView{})
	a := r.combatPose(p, eastIdle)
	if a.held.stance != stanceAimed || a.held.aimX != 9.5 || a.held.aimY != 3.4 {
		t.Fatalf("aims at the target's body: %+v", a.held)
	}
	sp := r.art.sprite("weapon/rifle_basic").meta.Attach
	h := a.held
	for _, f := range []facing{faceEast, faceWest, faceSouth, faceNorth} {
		rig, ps := rigFor(r, f)
		c := aimCentre(r, rig, ps, 5, 5, 64)
		for i := 0; i < 72; i++ {
			// the target walks round the shooter
			ang := 2*math.Pi*float64(i)/72 + 0.01
			h.aimX, h.aimY = 5.5+4*math.Cos(ang), 5.0+4*math.Sin(ang)
			pl, ok := r.heldPlace(rig, h, 5, 5, 64, ps)
			if !ok {
				t.Fatal("placement")
			}
			gr, mz := apply(pl.g, sp["grip"]), apply(pl.g, sp["muzzle"])
			tx, ty := r.worldToScreenF(h.aimX, h.aimY)
			want := math.Atan2(float64(ty)-c[1], float64(tx)-c[0])
			if got := math.Atan2(mz[1]-c[1], mz[0]-c[0]); math.Abs(wrapAngle(got-want)) > 0.03 {
				t.Fatalf("%v step %d: muzzle %.3f rad from the body centre, target %.3f", f, i, got, want)
			}
			if got := math.Atan2(mz[1]-gr[1], mz[0]-gr[0]); math.Abs(wrapAngle(got-want)) > 0.03 {
				t.Fatalf("%v step %d: the rifle lies along the aim: %.3f vs %.3f", f, i, got, want)
			}
			if d := math.Hypot(mz[0]-c[0], mz[1]-c[1]); d < aimReach*64 {
				t.Fatalf("%v step %d: the muzzle is out past %.2f tile: %.1f px", f, i, aimReach, d)
			}
			// upright: the sprite's top (towards smaller image y) stays up
			pv := heldPivot(r.art.sprite(h.key), h.stance)
			top, mid := apply(pl.g, pt{pv[0], pv[1] - 10}), apply(pl.g, pt{pv[0], pv[1]})
			if math.Abs(math.Cos(want)) > 0.2 && top[1] > mid[1] {
				t.Fatalf("%v step %d (%.2f rad): upside down", f, i, want)
			}
			if north := math.Sin(want) < 0; pl.behind != north {
				t.Fatalf("%v step %d: behind=%v, aiming north=%v", f, i, pl.behind, north)
			}
		}
	}
}

func TestMuzzleFlashAtMuzzle(t *testing.T) {
	t.Parallel()
	p := shooter()
	p.AttackCycle = 1
	tr := shot(1, false)
	w := game.WorldView{Traces: []game.TraceView{tr}}
	r := combatRenderer(100, []game.PawnView{p, targetAt(9, 5)}, w)
	r.animateCombat(w, r.drawPawns) // first sighting: the strike frame
	mz, ok := r.traceMuzzle(tr)
	want, ok2 := r.pawnHeldPoint(p, "muzzle")
	if !ok || !ok2 || mz != want {
		t.Fatalf("flash at the drawn muzzle: %v %v", mz, want)
	}
	hand, _ := r.pawnHeldPoint(game.PawnView{ID: 9, X: 5, Y: 5}, "hold")
	if math.Hypot(mz.x-hand.x, mz.y-hand.y) < 0.2 {
		t.Fatalf("the muzzle is out at the barrel's end: %v vs hand %v", mz, hand)
	}
	if mz.x < 5.5+aimReach {
		t.Fatalf("the muzzle is out towards the target: %v", mz)
	}
	smoke := 0
	for _, pt := range r.parts {
		if pt.kind == partSmoke {
			smoke++
			if math.Hypot(pt.x-mz.x, pt.y-mz.y) > 1e-9 {
				t.Fatalf("smoke at the muzzle: %v,%v vs %v", pt.x, pt.y, mz)
			}
		}
	}
	if smoke == 0 {
		t.Fatal("muzzle smoke")
	}
	screen := ebiten.NewImage(800, 600)
	r.drawTraces(screen, w, 64)
	// without art the shot leaves from in front of the shooter's tile
	r.art = nil
	m, _ := r.shotPoints(tr, r.drawPawns)
	if m.x != 6 || m.y != 5.3 {
		t.Fatalf("fallback muzzle %v", m)
	}
}

func TestShellCasingParticle(t *testing.T) {
	t.Parallel()
	for _, u := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {0.7, 0.7}} {
		cx, cy := casingDir(u[0], u[1])
		if right := -u[1]*cx + u[0]*cy; right <= 0 {
			t.Fatalf("shot %v: the case flies out of the right side (%v,%v)", u, cx, cy)
		}
		if cx*u[0]+cy*u[1] >= 0 {
			t.Fatalf("shot %v: and a little back", u)
		}
	}
	w := game.WorldView{Traces: []game.TraceView{shot(1, false), {FromX: 1, FromY: 1, ToX: 2, ToY: 1, Age: 1, Melee: true, Attacker: 4}}}
	r := combatRenderer(100, []game.PawnView{shooter(), targetAt(9, 5)}, w)
	count := func() int {
		n := 0
		for _, p := range r.parts {
			if p.kind == partCasing {
				n++
			}
		}
		return n
	}
	r.animateCombat(w, r.drawPawns)
	if count() != 1 {
		t.Fatalf("one case per shot, none per blow: %d", count())
	}
	r.animateCombat(w, r.drawPawns)
	if count() != 1 {
		t.Fatal("a trace ejects once")
	}
}

func TestStabAnchoredToTrace(t *testing.T) {
	t.Parallel()
	p := game.PawnView{ID: 1, X: 5, Y: 5, Weapon: "knife", Action: game.ActionMelee, ActTarget: 2}
	near := targetAt(6, 5)
	// wind-up by AttackCycle: guard, chamber back, step in
	var dxs []float64
	for _, c := range []float64{0.3, 0.7, 0.99} {
		p.AttackCycle = c
		r := combatRenderer(100, []game.PawnView{p, near}, game.WorldView{})
		dxs = append(dxs, r.combatPose(p, eastIdle).dx)
	}
	if dxs[0] != 0 || dxs[1] >= 0 || dxs[2] <= dxs[1] {
		t.Fatalf("guard, chamber back, step in: %v", dxs)
	}
	// strike on the trace (age 0..2 full reach), withdrawn by meleeTicks
	p.AttackCycle = 0.05
	var pushes []float64
	for _, age := range []int{1, 5, 8, 20} {
		tr := game.TraceView{FromX: 5, FromY: 5, ToX: 6, ToY: 5, Age: age, Melee: true, Hit: true, Attacker: 1, Target: 2, Weapon: "knife"}
		r := combatRenderer(100, []game.PawnView{p, near}, game.WorldView{Traces: []game.TraceView{tr}})
		a := r.combatPose(p, eastIdle)
		pushes = append(pushes, a.held.push)
		if age == 1 {
			if a.held.stance != stanceAimed || a.held.key != "weapon/knife" || a.dx <= 0 {
				t.Fatalf("the knife points at the target and the stab lunges: %+v", a)
			}
		}
	}
	if math.Abs(pushes[0]-knifePush) > 1e-9 || pushes[1] >= pushes[0] || pushes[2] >= pushes[1] || pushes[3] != 0 {
		t.Fatalf("thrust out on the trace, withdrawn after: %v", pushes)
	}
}

func TestSpearThrustReach(t *testing.T) {
	t.Parallel()
	lunge := func(weapon string) (actionPose, *Renderer) {
		p := game.PawnView{ID: 1, X: 5, Y: 5, Weapon: weapon, Action: game.ActionMelee, ActTarget: 2}
		tr := game.TraceView{FromX: 5, FromY: 5, ToX: 6, ToY: 5, Age: 1, Melee: true, Attacker: 1, Target: 2}
		r := combatRenderer(100, []game.PawnView{p, targetAt(6, 5)}, game.WorldView{Traces: []game.TraceView{tr}})
		return r.combatPose(p, eastIdle), r
	}
	spear, r := lunge("spear")
	knife, _ := lunge("knife")
	if spear.dx <= knife.dx || spear.held.push <= knife.held.push {
		t.Fatalf("the spear thrusts further: %.3f/%.3f vs knife %.3f/%.3f", spear.dx, spear.held.push, knife.dx, knife.held.push)
	}
	rig, ps := rigFor(r, faceEast)
	h := heldItem{key: "weapon/spear", stance: stanceAimed, aimX: 6.5, aimY: 5.4}
	tip1, ok1 := r.heldPoint(rig, h, 5, 5, ps, "tip")
	h.push = spear.held.push
	ps.dx = spear.dx
	tip4, ok4 := r.heldPoint(rig, h, 5, 5, ps, "tip")
	if !ok1 || !ok4 || tip4.x <= tip1.x+0.15 {
		t.Fatalf("full reach drives the tip forward: %v -> %v", tip1, tip4)
	}
	if tip4.x < 6 {
		t.Fatalf("the spear tip reaches the next tile: %v", tip4)
	}
}

func TestClubSwing(t *testing.T) {
	t.Parallel()
	_, rest, _ := meleeMotion("club", 0.2, 0, false)
	_, raised, _ := meleeMotion("club", 0.99, 0, false)
	_, struck, _ := meleeMotion("club", 0, 0, true)
	_, after, _ := meleeMotion("club", 0, meleeTicks-1, true)
	if rest != clubRest || raised < clubRaised-0.01 || struck != clubStrike || after <= struck {
		t.Fatalf("rest %v, wound up %v, struck on the trace %v, recovering %v", rest, raised, struck, after)
	}
	if lunge, _, _ := meleeMotion("club", 0, 0, true); lunge != 1 {
		t.Fatalf("the strike lands on the trace: lunge %v", lunge)
	}
	if raised-struck > 1.4 {
		t.Fatalf("a short arc about the aim: %v .. %v", struck, raised)
	}
	// aimed east: cocked up, then driven down and forward
	r := artRenderer()
	rig, ps := rigFor(r, faceEast)
	sp := r.art.sprite("weapon/club").meta.Attach
	h := heldItem{key: "weapon/club", stance: stanceAimed, aimX: 6.5, aimY: 2.4}
	at := func(angle float64) (tip, grip pt) {
		h.angle = angle
		pl, _ := r.heldPlace(rig, h, 2, 2, 64, ps)
		return apply(pl.g, sp["tip"]), apply(pl.g, sp["grip"])
	}
	if tip, grip := at(raised); tip[1] >= grip[1] {
		t.Fatal("wound up: the club is raised")
	}
	if tip, grip := at(struck); tip[0] <= grip[0] || tip[1] <= grip[1] {
		t.Fatal("struck: the club comes down forward")
	}
	// aimed west (flipped upright): raised is still up
	h.aimX = -2.5
	if tip, grip := at(raised); tip[1] >= grip[1] || tip[0] >= grip[0] {
		t.Fatal("aimed west: the club is raised, pointing west")
	}
}

func TestPunch(t *testing.T) {
	t.Parallel()
	if lunge, _, push := meleeMotion("fists", 0.9, 0, false); lunge != 0 || push != 0 {
		t.Fatalf("guard while winding up: %v %v", lunge, push)
	}
	if _, _, push := meleeMotion("fists", 0, 1, true); push != fistPush {
		t.Fatalf("the fist shoots out on the trace: %v", push)
	}
	p := game.PawnView{ID: 1, X: 5, Y: 5, Weapon: "fists", Action: game.ActionMelee, ActTarget: 2}
	tr := game.TraceView{FromX: 5, FromY: 5, ToX: 6, ToY: 5, Age: 1, Melee: true, Attacker: 1, Target: 2}
	r := combatRenderer(100, []game.PawnView{p, targetAt(6, 5)}, game.WorldView{Traces: []game.TraceView{tr}})
	a := r.combatPose(p, eastIdle)
	if a.held.key != "" || !a.held.set || a.held.push <= 0 || a.dx <= 0 {
		t.Fatalf("a bare-handed punch: %+v", a)
	}
	rig, ps := rigFor(r, faceEast)
	if _, ok := r.heldPlace(rig, a.held, 5, 5, 64, ps); ok {
		t.Fatal("fists: no sprite drawn")
	}
	// the melee trail follows the punch's reach
	if pts := r.meleeTrail(tr); len(pts) < 2 || pts[1].x <= pts[0].x {
		t.Fatalf("trail out along the punch: %v", pts)
	}
	r.drawTraces(ebiten.NewImage(800, 600), r.drawW, 64)
}

func TestHitFlinchDirectional(t *testing.T) {
	t.Parallel()
	p := game.PawnView{ID: 1, X: 5, Y: 5, Weapon: "rifle", Drafted: true, HitTick: 99, HitFromX: 3, HitFromY: 5, HitDamage: 9}
	at := func(tick uint64, q game.PawnView) actionPose {
		return combatRenderer(tick, []game.PawnView{q}, game.WorldView{}).hitReaction(q, eastIdle)
	}
	a := at(100, p)
	if !a.ok || a.dx <= 0 || a.dy != 0 || a.rot <= 0 || a.flash <= 0 {
		t.Fatalf("hit from the west: flinch, stagger east, flash: %+v", a)
	}
	if a.held.key != "weapon/rifle_basic" {
		t.Fatal("a drafted pawn keeps its rifle up while flinching")
	}
	if a := at(102, p); math.Abs(a.dx-staggerDist) > 1e-9 {
		t.Fatalf("stagger peaks at %.2f tile: %v", staggerDist, a.dx)
	}
	if a, b := at(104, p), at(107, p); a.dx <= b.dx || b.dx <= 0 || a.flash <= b.flash {
		t.Fatalf("the stagger and flash ease out: %+v then %+v", a, b)
	}
	if a := at(109, p); a.dx != 0 {
		t.Fatalf("stagger over by %v ticks: %v", staggerTicks, a.dx)
	}
	if a := at(130, p); a.ok {
		t.Fatal("old hits do not flinch")
	}
	q := p
	q.HitFromX, q.HitFromY = 5, 8
	if a := at(101, q); a.dy >= 0 || a.dx != 0 {
		t.Fatalf("hit from the south: stagger north: %+v", a)
	}
	if a := at(101, game.PawnView{ID: 1}); a.ok {
		t.Fatal("never hit: no flinch")
	}
	// hits are not double-counted by the old knockback
	r := combatRenderer(100, []game.PawnView{p}, game.WorldView{})
	if dx, dy, fl := r.combatOffset(p); dx != 0 || dy != 0 || fl != 0 {
		t.Fatal("stamped hits stagger in hitReaction only")
	}
}

func TestBloodSpraysAwayFromAttacker(t *testing.T) {
	t.Parallel()
	mean := func(fromX int) float64 {
		tr := game.TraceView{FromX: fromX, FromY: 5, ToX: 6, ToY: 5, Age: 1, Hit: true, Melee: true, Attacker: 1, Target: 2}
		w := game.WorldView{Traces: []game.TraceView{tr}}
		r := combatRenderer(100, nil, w)
		r.animateCombat(w, nil)
		var sum float64
		n := 0
		for _, p := range r.parts {
			if p.kind == partBlood {
				sum += p.vx
				n++
			}
		}
		if n == 0 {
			t.Fatal("blood on a hit")
		}
		return sum / float64(n)
	}
	if mean(5) <= 0 {
		t.Fatal("hit from the west: blood flies east")
	}
	if mean(7) >= 0 {
		t.Fatal("hit from the east: blood flies west")
	}
}

func TestFallRotatesThenLies(t *testing.T) {
	t.Parallel()
	r := combatRenderer(100, nil, game.WorldView{})
	r.present = 1000
	p := game.PawnView{ID: 1, X: 5, Y: 5, Lying: true, Status: "downed", HitTick: 90, HitFromX: 8, HitFromY: 5}
	at := func(e uint64, q game.PawnView) pose {
		return r.fallPose(q, pawnVisual{facing: faceEast, lying: true, lyingSince: r.present - e})
	}
	// buckling: upright, squashing
	for _, e := range []uint64{0, 4, 8, 15} {
		if ps := at(e, p); ps.rot != 0 || (e > 0 && ps.sy >= 1) {
			t.Fatalf("buckling at %d: %+v", e, ps)
		}
	}
	// then the rotation runs over fallFrames, monotonically
	prev := 0.0
	for e := uint64(buckleFrames); e <= buckleFrames+fallFrames; e++ {
		rot := math.Abs(at(e, p).rot)
		if rot < prev {
			t.Fatalf("the fall turns back at %d: %v < %v", e, rot, prev)
		}
		prev = rot
	}
	if math.Abs(prev-math.Pi/2) > 1e-9 {
		t.Fatalf("flat after fallFrames: %v", prev)
	}
	if ps := at(200, p); math.Abs(ps.rot+math.Pi/2) > 1e-9 || math.Abs(ps.dx-lyingPose(r.art, -1).dx+0.12) > 1e-9 {
		t.Fatalf("hit from the east: falls west, slid 0.12 away from the hit: %+v", ps)
	}
	p.HitFromX = 2
	if ps := at(200, p); math.Abs(ps.rot-math.Pi/2) > 1e-9 {
		t.Fatalf("hit from the west: falls east: %v", ps.rot)
	}
	if ps := at(2, p); ps.dx <= 0 || ps.flash <= 0 {
		t.Fatalf("slides away from the hit and flashes while going down: %+v", ps)
	}
	// asleep on the ground: lies down at once, no buckle
	if ps := at(2, game.PawnView{ID: 1, Lying: true, Status: "alive"}); ps.rot <= 0 {
		t.Fatalf("sleepers lie down without buckling: %+v", ps)
	}
	// every lying pawn draws its standing south figure, turned
	screen := ebiten.NewImage(512, 512)
	for _, st := range []string{"downed", "dead", "alive"} {
		q := p
		q.Status = st
		var keys []string
		r.lit.onDraw = func(img *ebiten.Image, _ float32) { keys = append(keys, keyOf(r.art, img)) }
		r.drawLyingPawn(screen, q, pawnVisual{facing: faceEast, lyingSince: r.present - 3}, 2, 2, 64)
		r.lit.onDraw = nil
		if len(keys) == 0 || keys[0] != "pawn/body_south" {
			t.Fatalf("%s: lying figure is the south sprite: %v", st, keys)
		}
	}
}

func TestAnimalBiteKickSynced(t *testing.T) {
	t.Parallel()
	frame := func(an game.AnimalView, traces ...game.TraceView) (string, pose) {
		r := combatRenderer(100, nil, game.WorldView{Animals: []game.AnimalView{an}, Traces: traces})
		return r.animalFrame(an), r.animalPose(an)
	}
	wolf := game.AnimalView{ID: 7, Species: "ridgewolf", X: 5, Y: 5, Action: game.ActionBite, AttackCycle: 0.3, Health: 100}
	if s, _ := frame(wolf); s != "_bite1" {
		t.Fatalf("wind-up: %s", s)
	}
	wolf.AttackCycle = 0.8
	if s, _ := frame(wolf); s != "_bite2" {
		t.Fatalf("jaws open: %s", s)
	}
	bite := game.TraceView{FromX: 5, FromY: 5, ToX: 6, ToY: 5, Age: 1, Melee: true, Hit: true, Attacker: 7, Target: 2, Weapon: "bite"}
	s, ps := frame(wolf, bite)
	if s != "_bite3" || ps.dx <= 0 {
		t.Fatalf("the snap lands on the trace and lunges at the target: %s %v", s, ps.dx)
	}
	bite.Age = 6
	if s, _ := frame(wolf, bite); s != "_bite4" {
		t.Fatalf("recover: %s", s)
	}
	buck := game.AnimalView{ID: 8, Species: "dustbuck", X: 5, Y: 5, Action: game.ActionKick, AttackCycle: 1}
	kick := game.TraceView{FromX: 5, FromY: 5, ToX: 4, ToY: 5, Age: 1, Melee: true, Attacker: 8, Weapon: "hooves"}
	if s, ps := frame(buck, kick); s != "_kick3" || ps.dx >= 0 {
		t.Fatalf("kick west lands on the trace: %s %v", s, ps.dx)
	}
	buck.Action = ""
	if s, _ := frame(buck, kick); s != "_kick3" {
		t.Fatalf("a strike by weapon alone picks the kick: %s", s)
	}
	graze := game.AnimalView{ID: 9, Species: "dustbuck", X: 2, Y: 2, Action: game.ActionGraze}
	seen := map[string]bool{}
	for tick := uint64(100); tick < 200; tick += 5 {
		r := combatRenderer(tick, nil, game.WorldView{Animals: []game.AnimalView{graze}})
		seen[r.animalFrame(graze)] = true
	}
	if !seen["_graze1"] || !seen["_graze2"] || len(seen) != 2 {
		t.Fatalf("grazing alternates graze1/graze2: %v", seen)
	}
}

func TestAnimalFlinch(t *testing.T) {
	t.Parallel()
	an := game.AnimalView{ID: 7, Species: "ridgewolf", X: 5, Y: 5, Action: game.ActionBite, HitTick: 99, HitFromX: 5, HitFromY: 2}
	r := combatRenderer(100, nil, game.WorldView{Animals: []game.AnimalView{an}})
	if s := r.animalFrame(an); s != "_flinch1" {
		t.Fatalf("flinch1: %s", s)
	}
	if ps := r.animalPose(an); ps.dy <= 0 || ps.flash <= 0 {
		t.Fatalf("hit from the north: staggers south and flashes: %+v", ps)
	}
	r = combatRenderer(105, nil, game.WorldView{Animals: []game.AnimalView{an}})
	if s := r.animalFrame(an); s != "_flinch2" {
		t.Fatalf("flinch2: %s", s)
	}
	r = combatRenderer(130, nil, game.WorldView{Animals: []game.AnimalView{an}})
	if s := r.animalFrame(an); s == "_flinch1" || s == "_flinch2" {
		t.Fatal("the flinch ends")
	}
	// a sim without hit stamps: the trace knockback still applies
	old := game.AnimalView{ID: 7, X: 6, Y: 5}
	r = combatRenderer(100, nil, game.WorldView{})
	r.animState().trackReactions([]game.TraceView{{FromX: 5, FromY: 5, ToX: 6, ToY: 5, Age: 1, Hit: true}}, 100, r.present)
	if ps := r.animalPose(old); ps.dx <= 0 || ps.flash <= 0 {
		t.Fatalf("fallback knockback: %+v", ps)
	}
}

// A flinch keeps what the pawn holds: a hauled load stays in its arms,
// an aimed rifle stays aimed.
func TestLoadKeptDuringFlinch(t *testing.T) {
	t.Parallel()
	p := game.PawnView{ID: 1, X: 5, Y: 5, Carrying: "wood", HitTick: 99, HitFromX: 3, HitFromY: 5}
	r := combatRenderer(100, []game.PawnView{p}, game.WorldView{})
	a := r.hitReaction(p, eastIdle)
	if !a.ok || a.dx <= 0 || a.held.key != itemSprite("wood") || a.held.stance != stanceLoad {
		t.Fatalf("the load is kept through the flinch: %+v", a)
	}
	if _, h := r.standingPose(p, eastIdle); h.key != itemSprite("wood") {
		t.Fatalf("the drawn figure keeps its load: %+v", h)
	}
	q := shooter()
	q.HitTick, q.HitFromX, q.HitFromY = 99, 3, 5
	r = combatRenderer(100, []game.PawnView{q, targetAt(9, 5)}, game.WorldView{})
	if a := r.hitReaction(q, eastIdle); a.held.stance != stanceAimed || a.held.key != "weapon/rifle_basic" {
		t.Fatalf("an aiming pawn stays aimed through the flinch: %+v", a.held)
	}
	// off duty and empty-handed: nothing
	idle := game.PawnView{ID: 1, X: 5, Y: 5, Weapon: "rifle", HitTick: 99, HitFromX: 3, HitFromY: 5}
	r = combatRenderer(100, []game.PawnView{idle}, game.WorldView{})
	if a := r.hitReaction(idle, eastIdle); a.held.key != "" {
		t.Fatalf("off duty: nothing held while flinching: %+v", a.held)
	}
}
