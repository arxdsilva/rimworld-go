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

func TestAimBlendFollowsAttackCycle(t *testing.T) {
	t.Parallel()
	if s, k, _ := aimFrame(0, 0, false); s != "_ready" || k != 0 {
		t.Fatalf("cycle 0: low ready (%s %v)", s, k)
	}
	if s, k, _ := aimFrame(aimAt, 0, false); s != "_aim" || k != 1 {
		t.Fatalf("cycle 0.6: shouldered (%s %v)", s, k)
	}
	prev := -1.0
	for c := 0.0; c <= 1; c += 0.02 {
		_, k, _ := aimFrame(c, 0, false)
		if k < prev {
			t.Fatalf("raise falls at cycle %.2f", c)
		}
		prev = k
	}
	p := shooter()
	p.AttackCycle = 0
	r := combatRenderer(100, []game.PawnView{p, targetAt(9, 5)}, game.WorldView{})
	a := r.combatPose(p, eastIdle)
	if !a.ok || a.suffix != "_ready" || a.held.key != "weapon/rifle_basic" || math.Abs(a.held.angle+lowReady) > 1e-9 {
		t.Fatalf("aiming starts at low ready: %+v", a)
	}
	p.AttackCycle = 0.7
	if a := r.combatPose(p, eastIdle); a.suffix != "_aim" || a.held.angle != 0 {
		t.Fatalf("shouldered past 60%%: %+v", a)
	}
	// walking to the target: legs keep walking, rifle stays ready
	if a := r.combatPose(p, moveState{Facing: faceEast, Blend: 1, Moving: true}); a.suffix != "" || a.held.stance != stanceHands {
		t.Fatalf("walking while aiming keeps the walk frames: %+v", a)
	}
	if a := r.combatPose(game.PawnView{ID: 3, Weapon: "rifle"}, eastIdle); a.ok {
		t.Fatal("not attacking: no combat layer")
	}
}

func TestFireFramesAnchoredToTrace(t *testing.T) {
	t.Parallel()
	p := shooter()
	p.AttackCycle = 1
	for _, c := range []struct {
		age  int
		want string
	}{{1, "_fire1"}, {0, "_fire1"}, {4, "_fire2"}, {7, "_fire3"}, {15, "_aim"}} {
		r := combatRenderer(100, []game.PawnView{p, targetAt(9, 5)}, game.WorldView{Traces: []game.TraceView{shot(c.age, false)}})
		a := r.combatPose(p, eastIdle)
		if a.suffix != c.want {
			t.Fatalf("trace age %d: %s, want %s", c.age, a.suffix, c.want)
		}
		if c.age <= 1 && a.dx >= 0 {
			t.Fatalf("recoil pushes back from the target: %v", a.dx)
		}
	}
	// a fast sim cannot skip the shot: 2 frames after the first sighting
	// it is still on fire1/fire2 even though 30 ticks went by
	r := combatRenderer(100, []game.PawnView{p, targetAt(9, 5)}, game.WorldView{})
	st := r.animState()
	st.trackReactions([]game.TraceView{shot(1, false)}, 100, 50)
	r.game.(*stubSim).tick = 131
	r.present = 52
	if a := r.combatPose(p, eastIdle); a.suffix != "_fire2" && a.suffix != "_fire1" {
		t.Fatalf("min visible frames: %s", a.suffix)
	}
	// another pawn's shot does not fire this one
	other := shot(1, false)
	other.Attacker, other.FromX = 7, 6
	r = combatRenderer(100, []game.PawnView{p}, game.WorldView{Traces: []game.TraceView{other}})
	p.AttackCycle = 0.1
	if a := r.combatPose(p, eastIdle); a.suffix != "_ready" {
		t.Fatalf("someone else's trace: %s", a.suffix)
	}
}

func TestRifleAimsAtTarget(t *testing.T) {
	t.Parallel()
	p := shooter()
	p.AttackCycle = 0.8
	r := combatRenderer(100, []game.PawnView{p, targetAt(9, 3)}, game.WorldView{})
	a := r.combatPose(p, eastIdle)
	if !a.held.aim || a.held.aimX != 9.5 || a.held.aimY != 3.4 {
		t.Fatalf("aims at the target's body: %+v", a.held)
	}
	sp := r.art.sprite("weapon/rifle_basic").meta.Attach
	h := a.held
	prev := math.NaN()
	for i := 0; i <= 40; i++ {
		// the target walks along an arc in front of the shooter
		ang := -0.6 + 1.2*float64(i)/40
		h.aimX, h.aimY = 5.5+4*math.Cos(ang), 5.0+4*math.Sin(ang)
		g, _, ok := r.heldGeo(faceEast, "_aim", h, false, 5, 5, 64, standPose)
		if !ok {
			t.Fatal("placement")
		}
		gr, mz := apply(g, sp["grip"]), apply(g, sp["muzzle"])
		got := math.Atan2(mz[1]-gr[1], mz[0]-gr[0])
		tx, ty := r.worldToScreenF(h.aimX, h.aimY)
		want := math.Atan2(float64(ty)-gr[1], float64(tx)-gr[0])
		if math.Abs(wrapAngle(got-want)) > 0.03 {
			t.Fatalf("step %d: muzzle %.3f rad, target %.3f", i, got, want)
		}
		if !math.IsNaN(prev) && math.Abs(got-prev) > 0.06 {
			t.Fatalf("aim jumps %.3f rad between steps", got-prev)
		}
		prev = got
	}
	// out of reach behind: clamped, never flipped around
	h.aimX, h.aimY = 0, 5.2
	g, _, _ := r.heldGeo(faceEast, "_aim", h, false, 5, 5, 64, standPose)
	gr, mz := apply(g, sp["grip"]), apply(g, sp["muzzle"])
	if mz[0] <= gr[0] {
		t.Fatal("an east-facing pawn keeps its muzzle east")
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
	hand, _ := r.pawnHeldPoint(game.PawnView{ID: 9, X: 5, Y: 5}, "hand")
	if math.Hypot(mz.x-hand.x, mz.y-hand.y) < 0.2 {
		t.Fatalf("the muzzle is out at the barrel's end: %v vs hand %v", mz, hand)
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

func TestStabFramesAnchoredToTrace(t *testing.T) {
	t.Parallel()
	p := game.PawnView{ID: 1, X: 5, Y: 5, Weapon: "knife", Action: game.ActionMelee, ActTarget: 2}
	near := targetAt(6, 5)
	for _, c := range []struct {
		cycle float64
		want  string
	}{{0.3, "_stab1"}, {0.7, "_stab2"}, {0.9, "_stab3"}} {
		p.AttackCycle = c.cycle
		r := combatRenderer(100, []game.PawnView{p, near}, game.WorldView{})
		if a := r.combatPose(p, eastIdle); a.suffix != c.want {
			t.Fatalf("wind-up at cycle %.1f: %s want %s", c.cycle, a.suffix, c.want)
		}
	}
	p.AttackCycle = 0.05
	var strikeDX float64
	for _, c := range []struct {
		age  int
		want string
	}{{1, "_stab4"}, {5, "_stab5"}, {8, "_stab6"}, {20, "_stab1"}} {
		tr := game.TraceView{FromX: 5, FromY: 5, ToX: 6, ToY: 5, Age: c.age, Melee: true, Hit: true, Attacker: 1, Target: 2, Weapon: "knife"}
		r := combatRenderer(100, []game.PawnView{p, near}, game.WorldView{Traces: []game.TraceView{tr}})
		a := r.combatPose(p, eastIdle)
		if a.suffix != c.want {
			t.Fatalf("trace age %d: %s want %s", c.age, a.suffix, c.want)
		}
		if c.age == 1 {
			strikeDX = a.dx
			if !a.held.aim || a.held.key != "weapon/knife" {
				t.Fatalf("the knife points at the target: %+v", a.held)
			}
		}
	}
	if strikeDX <= 0 {
		t.Fatal("the stab lunges towards the target")
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
	if spear.suffix != "_thrust4" || spear.dx <= knife.dx {
		t.Fatalf("the spear thrusts further: %s %.3f vs knife %.3f", spear.suffix, spear.dx, knife.dx)
	}
	h := heldItem{key: "weapon/spear", stance: stanceHands}
	tip1, ok1 := r.heldPoint(faceEast, "_thrust1", h, 5, 5, standPose, "tip")
	tip4, ok4 := r.heldPoint(faceEast, "_thrust4", h, 5, 5, standPose, "tip")
	if !ok1 || !ok4 || tip4.x <= tip1.x+0.15 {
		t.Fatalf("full reach drives the tip forward: %v -> %v", tip1, tip4)
	}
	if tip4.x < 6 {
		t.Fatalf("the spear tip reaches the next tile: %v", tip4)
	}
}

func TestClubSwing(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for c := 0.0; c < 1; c += 0.05 {
		s, _ := meleeFrame("club", c, 0, false, false)
		seen[s] = true
	}
	for age := 0.0; age < meleeTicks; age++ {
		s, _ := meleeFrame("club", 0, age, true, false)
		seen[s] = true
	}
	for i := 1; i <= 6; i++ {
		if !seen["_swing"+string(rune('0'+i))] {
			t.Fatalf("all six swing frames: %v", seen)
		}
	}
	if s, k := meleeFrame("club", 0, 0, true, false); s != "_swing4" || k != 1 {
		t.Fatalf("the strike frame lands on the trace: %s %v", s, k)
	}
	// one hand, along the forearm: cocked up, then driven down and forward
	r := artRenderer()
	h := heldItem{key: "weapon/club", stance: stanceHands}
	sp := r.art.sprite("weapon/club").meta.Attach
	g1, _, _ := r.heldGeo(faceEast, "_swing1", h, false, 2, 2, 64, standPose)
	g4, _, _ := r.heldGeo(faceEast, "_swing4", h, false, 2, 2, 64, standPose)
	if tip, grip := apply(g1, sp["tip"]), apply(g1, sp["grip"]); tip[1] >= grip[1] {
		t.Fatal("swing1: the club is raised")
	}
	if tip, grip := apply(g4, sp["tip"]), apply(g4, sp["grip"]); tip[0] <= grip[0] {
		t.Fatal("swing4: the club strikes forward")
	}
}

func TestPunch(t *testing.T) {
	t.Parallel()
	if s, _ := meleeFrame("fists", 0.9, 0, false, false); s != "_punch1" {
		t.Fatalf("guard while winding up: %s", s)
	}
	if s, _ := meleeFrame("fists", 0, 1, true, false); s != "_punch2" {
		t.Fatalf("jab: %s", s)
	}
	if s, _ := meleeFrame("fists", 0, 1, true, true); s != "_punch3" {
		t.Fatalf("cross: %s", s)
	}
	if s, _ := meleeFrame("fists", 0, 5, true, false); s != "_punch4" {
		t.Fatalf("back to guard: %s", s)
	}
	p := game.PawnView{ID: 1, X: 5, Y: 5, Weapon: "fists", Action: game.ActionMelee, ActTarget: 2}
	tr := game.TraceView{FromX: 5, FromY: 5, ToX: 6, ToY: 5, Age: 1, Melee: true, Attacker: 1, Target: 2}
	r := combatRenderer(100, []game.PawnView{p, targetAt(6, 5)}, game.WorldView{Traces: []game.TraceView{tr}})
	a := r.combatPose(p, eastIdle)
	if a.suffix != "_punch2" && a.suffix != "_punch3" || a.held.key != "" || !a.held.set {
		t.Fatalf("a bare-handed punch: %+v", a)
	}
	// the melee trail follows the fist through the punch frames
	r.drawTraces(ebiten.NewImage(800, 600), r.drawW, 64)
}

func TestHitFlinchDirectional(t *testing.T) {
	t.Parallel()
	p := game.PawnView{ID: 1, X: 5, Y: 5, Weapon: "rifle", Drafted: true, HitTick: 99, HitFromX: 3, HitFromY: 5, HitDamage: 9}
	at := func(tick uint64, q game.PawnView) actionPose {
		return combatRenderer(tick, []game.PawnView{q}, game.WorldView{}).hitReaction(q, eastIdle)
	}
	a := at(100, p)
	if !a.ok || a.suffix != "_flinch1" || a.dx <= 0 || a.dy != 0 || a.flash <= 0 {
		t.Fatalf("hit from the west: flinch, stagger east, flash: %+v", a)
	}
	if a.held.key != "weapon/rifle_basic" {
		t.Fatal("a drafted pawn keeps its rifle up while flinching")
	}
	if a := at(102, p); math.Abs(a.dx-staggerDist) > 1e-9 {
		t.Fatalf("stagger peaks at %.2f tile: %v", staggerDist, a.dx)
	}
	if a := at(104, p); a.suffix != "_flinch2" {
		t.Fatalf("flinch2: %s", a.suffix)
	}
	if a := at(107, p); a.suffix != "_flinch3" {
		t.Fatalf("flinch3: %s", a.suffix)
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

func TestFallFramesThenLie(t *testing.T) {
	t.Parallel()
	r := combatRenderer(100, nil, game.WorldView{})
	r.present = 1000
	p := game.PawnView{ID: 1, X: 5, Y: 5, Lying: true, Status: "downed", HitTick: 90, HitFromX: 8, HitFromY: 5}
	at := func(e uint64, q game.PawnView) (string, pose, bool) {
		return r.fallPose(q, pawnVisual{facing: faceEast, lying: true, lyingSince: r.present - e})
	}
	want := []string{"_fall1", "_fall2", "_fall3", "_fall4"}
	for i, w := range want {
		s, ps, up := at(uint64(i)*uint64(fallStep), p)
		if s != w || !up || ps.rot != 0 {
			t.Fatalf("frame %d: %s rot %v", i, s, ps.rot)
		}
	}
	s, ps, _ := at(200, p)
	if s != "_fall4" || math.Abs(ps.rot+math.Pi/2) > 1e-9 {
		t.Fatalf("hit from the east: falls west: %s %v", s, ps.rot)
	}
	p.HitFromX = 2
	if _, ps, _ := at(200, p); math.Abs(ps.rot-math.Pi/2) > 1e-9 {
		t.Fatalf("hit from the west: falls east: %v", ps.rot)
	}
	if _, ps, _ := at(2, p); ps.dx <= 0 || ps.flash <= 0 {
		t.Fatalf("slides away from the hit and flashes while going down: %+v", ps)
	}
	// asleep on the ground: just lies down, no fall frames
	if s, _, up := at(2, game.PawnView{ID: 1, Lying: true, Status: "alive"}); s != "" || up {
		t.Fatal("sleepers lie down without falling")
	}
	screen := ebiten.NewImage(512, 512)
	for _, st := range []string{"downed", "dead", "alive"} {
		q := p
		q.Status = st
		r.drawLyingPawn(screen, q, pawnVisual{facing: faceEast, lyingSince: r.present - 3}, 2, 2, 64)
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
