package ebiten

import (
	"math"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestAttackReactions(t *testing.T) {
	t.Parallel()
	traces := []game.TraceView{
		{FromX: 2, FromY: 5, ToX: 6, ToY: 5, Age: 1},              // rifle shot east
		{FromX: 6, FromY: 5, ToX: 7, ToY: 5, Age: 5, Melee: true}, // bite east
		{FromX: 2, FromY: 5, ToX: 6, ToY: 5, Age: 2, Hit: true},
	}
	dx, _, fl := attackReaction(traces, 2, 5)
	if dx >= 0 || fl != 0 {
		t.Fatalf("shooter recoils west: dx=%v flash=%v", dx, fl)
	}
	dx, _, fl = attackReaction(traces, 6, 5)
	if dx <= 0 || fl <= 0 {
		t.Fatalf("bitten-at target lunges/knocked east and flashes: dx=%v flash=%v", dx, fl)
	}
	if dx, dy, fl := attackReaction(traces, 0, 0); dx != 0 || dy != 0 || fl != 0 {
		t.Fatal("bystanders don't move")
	}
}

func TestWorkSwingAndStrike(t *testing.T) {
	t.Parallel()
	st, ok := styleFor("cut")
	if !ok || st.burst != partChip || st.tool != "tool/axe" || st.motion != motionSwing {
		t.Fatal("cutting swings an axe and throws chips")
	}
	// six frames: 1-2 wind-up, 3-4 swing, 5 strike, 6 recover
	want := map[float64]string{0.05: "_work1", 0.2: "_work2", 0.33: "_work3", 0.43: "_work4", strikeAt: "_work5", 0.8: "_work6"}
	for ph, w := range want {
		if got := workFrame(ph); got != w {
			t.Fatalf("workFrame(%v) = %s, want %s", ph, got, w)
		}
	}
	if toolAngle(faceWest, "_work1") != -toolAngle(faceEast, "_work1") {
		t.Fatal("west mirrors east")
	}
	if toolAngle(faceEast, "_work5") <= toolAngle(faceEast, "_work1") {
		t.Fatal("the tool comes down from wind-up to strike")
	}
	strikes, peak := 0, 0.0
	for f := 0.0; f < float64(st.period)*3; f++ {
		if workStrikeAt(f, f+1, 7, st.period) {
			strikes++
		}
		peak = math.Max(peak, workSwing(f, 7, st.period))
	}
	if strikes != 3 || peak < 0.99 {
		t.Fatalf("one strike per swing (%d), swing reaches 1 (%v)", strikes, peak)
	}
	if _, ok := styleFor(""); ok {
		t.Fatal("no style when idle")
	}
}

// walking at one tile per 15 ticks, the usual gait
var walkingState = moveState{Speed: 1.0 / float64(game.TicksPerCell), Blend: 1, Moving: true}

func TestWalkCycleFollowsStride(t *testing.T) {
	t.Parallel()
	// one cycle (12 frames) per 1.6 tiles: floor(stride/1.6*12) mod 12
	cases := map[float64]int{0: 0, 0.14: 1, 0.8: 6, 1.59: 11, 1.6: 0, 3.2 + 0.14: 1, -0.1: 11}
	for stride, want := range cases {
		if got := walkIndex(stride); got != want {
			t.Fatalf("walkIndex(%v) = %d, want %d", stride, got, want)
		}
	}
	m := walkingState
	m.Stride = 0.8
	if walkSuffix(m) != "_walk07" {
		t.Fatalf("suffix = %s", walkSuffix(m))
	}
	// a diagonal step is 1.41 tiles of stride: more frames per step than a straight one
	if d, s := walkIndex(math.Sqrt2)-walkIndex(0), walkIndex(1)-walkIndex(0); d <= s {
		t.Fatalf("diagonal advances %d frames, straight %d", d, s)
	}
}

func TestWalkBlendIdle(t *testing.T) {
	t.Parallel()
	m := walkingState
	m.Stride, m.Blend = 0.8, 0
	if walkSuffix(m) != "" {
		t.Fatalf("blend 0 is the static pose, got %q", walkSuffix(m))
	}
	if bobRaise(0, m.Speed, 0) != 0 {
		t.Fatal("idle does not bob")
	}
}

func TestBobPeaksAtContact(t *testing.T) {
	t.Parallel()
	frameMid := func(i int) float64 { return (float64(i) + 0.5) / 12 * strideLen }
	sp := walkingState.Speed
	c1, c7 := bobRaise(frameMid(0), sp, 1), bobRaise(frameMid(6), sp, 1)
	if math.Abs(c1-c7) > 1e-9 || c1 <= 0 {
		t.Fatalf("frames 01 and 07 are the two peaks: %v %v", c1, c7)
	}
	for i := 0; i < 12; i++ {
		if i != 0 && i != 6 && bobRaise(frameMid(i), sp, 1) >= c1 {
			t.Fatalf("frame %d bobs as high as a contact frame", i+1)
		}
	}
	if bobRaise(frameMid(3), sp, 1) > 0.5*c1 {
		t.Fatal("the body is low mid-way between contacts")
	}
	if bobRaise(frameMid(0), sp, 0.5) >= c1 {
		t.Fatal("a half walk-blend bobs less")
	}
}

func TestSwayAndLeanScaleWithSpeed(t *testing.T) {
	t.Parallel()
	slow, fast := walkingState, walkingState
	slow.Speed *= 0.5
	slow.Stride, fast.Stride = 0.4, 0.4 // quarter cycle: sway at its extreme
	if a, b := math.Abs(gaitSway(slow)), math.Abs(gaitSway(fast)); a >= b || b > 0.015+1e-9 || b == 0 {
		t.Fatalf("sway grows with speed and stays within +-0.015: %v %v", a, b)
	}
	if gaitLean(faceEast, slow) >= gaitLean(faceEast, fast) || gaitLean(faceEast, fast) <= 0 {
		t.Fatal("east lean grows with speed, forward = clockwise")
	}
	if gaitLean(faceWest, fast) != -gaitLean(faceEast, fast) {
		t.Fatal("west leans the other way")
	}
	if gaitLean(faceSouth, fast) != 0 {
		t.Fatal("no forward lean seen head-on")
	}
	stopping := moveState{Blend: 0.5, Speed: 0}
	if gaitLean(faceEast, stopping) >= 0 {
		t.Fatal("decelerating leans back")
	}
}

func TestHairFollowsHead(t *testing.T) {
	t.Parallel()
	mk := func(head [2]float64) *artSprite {
		return &artSprite{meta: artMeta{Attach: map[string][2]float64{"head": head}}}
	}
	c := &artCatalog{sprites: map[string]*artSprite{
		"pawn/body_east":        mk([2]float64{100, 50}),
		"pawn/body_east_walk07": mk([2]float64{100 + artTilePx/2, 50 - artTilePx/4}),
		"pawn/body_east_walk02": {meta: artMeta{Attach: map[string][2]float64{}}}, // no head metadata yet
	}}
	dx, dy := headDelta(c, faceEast, "_walk07")
	if math.Abs(dx-0.5) > 1e-9 || math.Abs(dy+0.25) > 1e-9 {
		t.Fatalf("hair moves by head(frame)-head(static) in tiles: %v %v", dx, dy)
	}
	for _, sfx := range []string{"", "_walk02", "_walk99"} {
		if dx, dy := headDelta(c, faceEast, sfx); dx != 0 || dy != 0 {
			t.Fatalf("missing head data falls back to no offset (%q): %v %v", sfx, dx, dy)
		}
	}
	if dx, dy := headDelta(nil, faceEast, "_walk07"); dx != 0 || dy != 0 {
		t.Fatal("no catalog, no offset")
	}
}

func TestWorkSixFramesOnAnimT(t *testing.T) {
	t.Parallel()
	const period = 40
	count := func(perFrame float64, frames int) int {
		n := 0
		for i, at := 0, 0.0; i < frames; i, at = i+1, at+perFrame {
			if workStrikeAt(at, at+perFrame, 3, period) {
				n++
			}
		}
		return n
	}
	if count(0, 400) != 0 {
		t.Fatal("paused (animT frozen): no strikes")
	}
	one, three := count(1, 400), count(3, 400)
	if one != 10 || three != 30 {
		t.Fatalf("strikes at 1x=%d (want 10), 3x=%d (want 30)", one, three)
	}
	seen := map[string]bool{}
	for ph := 0.0; ph < 1; ph += 0.01 {
		seen[workFrame(ph)] = true
	}
	if len(seen) != 6 {
		t.Fatalf("six distinct work frames: %v", seen)
	}
	if workPhase(10, 3, period) != workPhase(10+period, 3, period) {
		t.Fatal("the swing repeats every period of animT")
	}
}

func TestBedTransitionEases(t *testing.T) {
	t.Parallel()
	if smoothEase(100, 100, 12) != 0 || smoothEase(112, 100, 12) != 1 || smoothEase(500, 100, 12) != 1 {
		t.Fatal("eases from 0 to 1 over 12 present frames")
	}
	if k := smoothEase(106, 100, 12); math.Abs(k-0.5) > 1e-9 {
		t.Fatalf("midpoint = %v", k)
	}
	if smoothEase(101, 100, 12) >= 1.0/12 || smoothEase(111, 100, 12) <= 11.0/12 {
		t.Fatal("slow at both ends (smoothstep), not linear")
	}
	a := pose{px: 0.5, py: 0.55, rot: 0, sx: 1, sy: 1}
	b := pose{px: 0.5, py: 0.55, rot: math.Pi / 2, sx: 0.85, sy: 0.85, dx: 1, dy: 2}
	if got := lerpPose(a, b, 0); got != a {
		t.Fatalf("k=0 is the standing pose: %+v", got)
	}
	if got := lerpPose(a, b, 1); got != b {
		t.Fatalf("k=1 is the bed pose: %+v", got)
	}
	if got := lerpPose(a, b, 0.5); math.Abs(got.rot-math.Pi/4) > 1e-9 || math.Abs(got.dx-0.5) > 1e-9 {
		t.Fatalf("midway = %+v", got)
	}
}

func TestCarryLiftEases(t *testing.T) {
	t.Parallel()
	r := &Renderer{present: 50}
	st := r.animState()
	if k := st.carryEase(7, 50); k != 0 {
		t.Fatalf("lift starts on the ground: %v", k)
	}
	if k := st.carryEase(7, 56); math.Abs(k-0.5) > 1e-9 {
		t.Fatalf("halfway after 6 frames: %v", k)
	}
	if k := st.carryEase(7, 80); k != 1 {
		t.Fatalf("fully slung: %v", k)
	}
	st.dropCarry(7)
	if k := st.carryEase(7, 90); k != 0 {
		t.Fatal("a new pickup restarts the lift")
	}
	ground, slung := carryPoses(faceEast)
	if lerpPose(ground, slung, 0.5) == slung {
		t.Fatal("poses differ")
	}
}

func TestDustAtFootOnContact(t *testing.T) {
	t.Parallel()
	// contacts at frames 01 and 07: every half cycle (0.8 tiles) of stride
	if contactsCrossed(0.1, 0.7) != 0 || contactsCrossed(0.7, 0.9) != 1 || contactsCrossed(0.7, 1.7) != 2 {
		t.Fatal("one contact per 0.8 tiles of stride")
	}
	if contactsCrossed(0.9, 0.7) != 0 {
		t.Fatal("no dust walking backwards")
	}
	left := footPoint(5, 5, faceSouth, 0)
	right := footPoint(5, 5, faceSouth, 1)
	if left == right {
		t.Fatal("feet alternate")
	}
	for _, p := range [][2]float64{left, right} {
		if math.Abs(p[0]-5.5) > 0.25 || p[1] < 5.45 || p[1] > 5.8 {
			t.Fatalf("dust lands at the feet, got %v", p)
		}
	}
	r := &Renderer{}
	st := r.animState()
	m := walkingState
	m.X, m.Y, m.Stride = 5, 5, 0.7
	r.stepDust(st, 9, m, false) // first sighting: no puff
	m.Stride = 0.9
	r.stepDust(st, 9, m, false)
	if len(r.parts) != 1 || r.parts[0].kind != partStep {
		t.Fatalf("a contact puffs one step particle, got %d", len(r.parts))
	}
	r.stepDust(st, 9, m, false)
	if len(r.parts) != 1 {
		t.Fatal("same stride, no second puff")
	}
}

func TestCombatMinVisibleFrames(t *testing.T) {
	t.Parallel()
	bite := game.TraceView{FromX: 6, FromY: 5, ToX: 7, ToY: 5, Melee: true}
	// expired from the sim, but only 3 present frames ago: still lunging
	rec := []reactTrace{{TraceView: bite, sim: math.Inf(1), frames: 3}}
	if dx, _, _ := attackReactionF(rec, 6, 5); dx <= 0 {
		t.Fatal("lunge stays for at least 8 present frames")
	}
	rec[0].frames = 9
	if dx, _, _ := attackReactionF(rec, 6, 5); dx != 0 {
		t.Fatal("lunge ends after its stretched duration")
	}
	// a fast sim (age 20 ticks already) cannot swallow the flash either
	hit := game.TraceView{FromX: 2, FromY: 5, ToX: 6, ToY: 5, Hit: true}
	rec = []reactTrace{{TraceView: hit, sim: 20, frames: 2}}
	if _, _, fl := attackReactionF(rec, 6, 5); fl <= 0 {
		t.Fatal("hit flash survives a fast sim")
	}
	// at 1x the sim duration governs: unchanged
	rec = []reactTrace{{TraceView: bite, sim: 5, frames: 5}}
	a, _, _ := attackReactionF(rec, 6, 5)
	b, _, _ := attackReaction([]game.TraceView{{FromX: 6, FromY: 5, ToX: 7, ToY: 5, Melee: true, Age: 5}}, 6, 5)
	if math.Abs(a-b) > 1e-9 {
		t.Fatalf("1x matches the sim-timed reaction: %v %v", a, b)
	}
	// bookkeeping retains a trace after the sim drops it
	r := &Renderer{present: 100}
	st := r.animState()
	st.trackReactions([]game.TraceView{bite}, 1000, 100)
	if got := st.reactions(100, nil); len(got) != 1 {
		t.Fatalf("tracked = %d", len(got))
	}
	st.trackReactions(nil, 1001, 103)
	got := st.reactions(103, nil)
	if len(got) != 1 || got[0].frames != 3 || !math.IsInf(got[0].sim, 1) {
		t.Fatalf("expired trace kept 3 frames: %+v", got)
	}
	st.trackReactions(nil, 1100, 200)
	if len(st.reactions(200, nil)) != 0 {
		t.Fatal("old reactions are pruned")
	}
}

func TestTrotEightFrames(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for s := 0.0; s < strideLen; s += 0.01 {
		seen[trotSuffix(moveState{Stride: s, Blend: 1})] = true
	}
	if len(seen) != 8 || !seen["_walk01"] || !seen["_walk08"] {
		t.Fatalf("trot has 8 frames walk01..walk08: %v", seen)
	}
	if trotSuffix(moveState{Stride: 0.5}) != "" {
		t.Fatal("idle animals use the static pose")
	}
	if trotSuffix(moveState{Stride: strideLen + 0.01, Blend: 1}) != "_walk01" {
		t.Fatal("trot repeats every 1.6 tiles")
	}
	m := moveState{Stride: 0.2, Speed: 0.06, Blend: 1}
	if g, w := trotBob(m, true), trotBob(m, false); math.Abs(g-1.6*w) > 1e-9 || w <= 0 {
		t.Fatalf("fleeing gallop bobs 1.6x: %v vs %v", g, w)
	}
}

func TestBlockedPawnFacesBlocker(t *testing.T) {
	t.Parallel()
	pawns := []game.PawnView{{ID: 1, X: 3, Y: 3, BlockedBy: 2}, {ID: 2, X: 4, Y: 3}}
	if f := blockedFacing(pawns[0], pawns, nil, faceSouth); f != faceEast {
		t.Fatalf("faces the blocker to the east, got %v", f)
	}
	if f := blockedFacing(pawns[1], pawns, nil, faceWest); f != faceWest {
		t.Fatal("unblocked keeps its facing")
	}
	an := []game.AnimalView{{ID: 9, X: 3, Y: 1}}
	p := game.PawnView{ID: 5, X: 3, Y: 3, BlockedBy: 9}
	if f := blockedFacing(p, nil, an, faceSouth); f != faceNorth {
		t.Fatalf("animals can block too, got %v", f)
	}
}

func TestStandingPoseUsesMotion(t *testing.T) {
	t.Parallel()
	m := walkingState
	m.Stride = 0.1 // near contact
	ps := walkPose(faceEast, m)
	if ps.dy >= 0 || ps.rot <= 0 {
		t.Fatalf("east walker bobs up and leans forward: %+v", ps)
	}
	ps = walkPose(faceNorth, moveState{Stride: 0.4, Speed: walkingState.Speed, Blend: 1, Moving: true})
	if ps.dx == 0 {
		t.Fatal("north walker sways sideways")
	}
	if idle := walkPose(faceEast, moveState{}); idle.dx != 0 || idle.dy != 0 || idle.rot != 0 {
		t.Fatalf("idle has no gait motion: %+v", idle)
	}
}

func TestParticlesExpire(t *testing.T) {
	t.Parallel()
	r := &Renderer{}
	r.burst(partDust, 3, 3, 4, 99)
	if len(r.parts) != 4 {
		t.Fatalf("burst = %d", len(r.parts))
	}
	r.present += 200
	r.drawParticlesNoScreen()
	if len(r.parts) != 0 {
		t.Fatal("old particles are dropped")
	}
	for i := 0; i < maxParticles+50; i++ {
		r.burst(partChip, 0, 0, 1, uint64(i))
	}
	if len(r.parts) != maxParticles {
		t.Fatal("particle cap holds")
	}
}

func TestItemPopAndSway(t *testing.T) {
	t.Parallel()
	r := &Renderer{}
	w := game.WorldView{Items: []game.ItemView{{ID: 1}}}
	r.syncItems(w) // first sync: existing items never pop
	if r.itemPop(1) != 1 {
		t.Fatal("start items are settled")
	}
	r.present = 10
	r.syncItems(game.WorldView{Items: []game.ItemView{{ID: 1}, {ID: 2}}})
	if k := r.itemPop(2); k > 0.01 {
		t.Fatalf("a new drop starts small: %v", k)
	}
	r.present = 17
	if k := r.itemPop(2); k <= 1 {
		t.Fatalf("pop overshoots mid-way: %v", k)
	}
	r.present = 40
	if k := r.itemPop(2); math.Abs(k-1) > 1e-9 {
		t.Fatalf("pop settles at 1: %v", k)
	}
	if a, b := r.sway(1, 1, 0.05), r.sway(9, 1, 0.05); a == b || math.Abs(a) > 0.05 {
		t.Fatalf("sway varies by tile and stays bounded: %v %v", a, b)
	}
}
