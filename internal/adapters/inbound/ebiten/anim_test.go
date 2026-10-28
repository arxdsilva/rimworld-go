package ebiten

import (
	"math"
	"slices"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
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
	_, st, ok := actionOf(game.PawnView{Acting: "cut"}) // old Acting strings still map
	if !ok || st.Parts[0].Kind != partChip || st.Motion != motionSwing {
		t.Fatal("cutting swings and throws chips")
	}
	// the body winds back, drives in hard on the strike at strikeAt, then
	// eases out
	if k := workSwing(float64(tickAt(7, st.Period, 0.30)), 7, st.Period); k >= 0 {
		t.Fatalf("wound back before the strike: %v", k)
	}
	if k := workSwing(float64(tickAt(7, st.Period, strikeAt+0.01)), 7, st.Period); k < 0.8 {
		t.Fatalf("driven in on the strike: %v", k)
	}
	strikes, peak := 0, 0.0
	for f := 0.0; f < float64(st.Period)*3; f++ {
		if workStrikeAt(f, f+1, 7, st.Period) {
			strikes++
		}
		peak = math.Max(peak, workSwing(f, 7, st.Period))
	}
	if strikes != 3 || peak < 0.99 {
		t.Fatalf("one strike per swing (%d), swing reaches 1 (%v)", strikes, peak)
	}
	if _, _, ok := actionOf(game.PawnView{}); ok {
		t.Fatal("no style when idle")
	}
}

// walking at one tile per 15 ticks, the usual gait
var walkingState = moveState{Speed: 1.0 / float64(game.TicksPerCell), Blend: 1, Moving: true}

// Hair, beard and hat are drawn in the body's own pose: a lean, squash or
// fall moves them exactly as it moves the head on the body.
func TestHairFollowsHead(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 16, h: 16, tick: 100})
	r.art = figureArt()
	var keys []string
	p := game.PawnView{ID: 1, X: 2, Y: 2, Look: look.Look{Beard: 1}, Apparel: []string{"tuque"}}
	for _, k := range []string{"pawn/beard_short_south", "pawn/tuque_knit_south"} {
		r.art.Sprites[k] = fakeSprite([2]int{160, 297}, pt{16, 153}, nil)
	}
	ps := standPose()
	ps.Rot, ps.Sy, ps.Dx = 0.3, 0.9, 0.1
	r.lit.OnDraw = func(img *ebiten.Image, _ float32) { keys = append(keys, keyOf(r.art, img)) }
	r.drawFigure(ebiten.NewImage(256, 256), p, 2, 2, 64, ps)
	for _, k := range []string{"pawn/body_south", "pawn/beard_short_south", "pawn/hair_short_south", "pawn/tuque_knit_south"} {
		if !slices.Contains(keys, k) {
			t.Fatalf("%s not drawn: %v", k, keys)
		}
	}
	// every layer shares the figure's transform: the same canvas point
	// lands on the same screen point
	sx, sy := r.worldToScreenF(2, 2)
	g := func(key string) pt {
		sp := r.art.Sprite(key)
		x, y, s := artOrigin(sp.Meta, float64(sx), float64(sy), 64)
		pg := ps.Geo(float64(sx), float64(sy), 64)
		return apply(pg, pt{x + 80*s, y + 122*s})
	}
	if a, b := g("pawn/body_south"), g("pawn/hair_short_south"); !near2(a, b, 1e-9) {
		t.Fatalf("hair rides the head: %v vs %v", a, b)
	}
}

func TestCarryLiftEases(t *testing.T) {
	t.Parallel()
	r := &Renderer{present: 50}
	st := r.animState()
	if k := st.CarryEase(7, 50); k != 0 {
		t.Fatalf("lift starts on the ground: %v", k)
	}
	if k := st.CarryEase(7, 56); math.Abs(k-0.5) > 1e-9 {
		t.Fatalf("halfway after 6 frames: %v", k)
	}
	if k := st.CarryEase(7, 80); k != 1 {
		t.Fatalf("fully slung: %v", k)
	}
	st.DropCarry(7)
	if k := st.CarryEase(7, 90); k != 0 {
		t.Fatal("a new pickup restarts the lift")
	}
	c := fakeCombatArt()
	ground, slung := carryPoses(c, faceEast)
	if lerpPose(ground, slung, 0.5) == slung {
		t.Fatal("poses differ")
	}
	// lying on the ground on its side; slung over the rescuer's back
	// attachment, a little above it
	if math.Abs(ground.Rot-math.Pi/2) > 1e-9 || ground != lyingPose(c, 1) {
		t.Fatalf("ground pose lies on its side: %+v", ground)
	}
	if wg, _ := carryPoses(c, faceWest); math.Abs(wg.Rot+math.Pi/2) > 1e-9 {
		t.Fatalf("facing west it lies the other way: %+v", wg)
	}
	_, backY, _ := bodyAttach(c.Sprite("pawn/body_south"), "back")
	if top := pawnFeetY + slung.Dy; top >= backY || top < backY-0.2 {
		t.Fatalf("slung across the back (%v) at %v", backY, top)
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
	if len(r.parts) != 1 || r.parts[0].Kind != partStep {
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
	rec := []reactTrace{{TraceView: bite, Sim: math.Inf(1), Frames: 3}}
	if dx, _, _ := attackReactionF(rec, 6, 5); dx <= 0 {
		t.Fatal("lunge stays for at least 8 present frames")
	}
	rec[0].Frames = 9
	if dx, _, _ := attackReactionF(rec, 6, 5); dx != 0 {
		t.Fatal("lunge ends after its stretched duration")
	}
	// a fast sim (age 20 ticks already) cannot swallow the flash either
	hit := game.TraceView{FromX: 2, FromY: 5, ToX: 6, ToY: 5, Hit: true}
	rec = []reactTrace{{TraceView: hit, Sim: 20, Frames: 2}}
	if _, _, fl := attackReactionF(rec, 6, 5); fl <= 0 {
		t.Fatal("hit flash survives a fast sim")
	}
	// at 1x the sim duration governs: unchanged
	rec = []reactTrace{{TraceView: bite, Sim: 5, Frames: 5}}
	a, _, _ := attackReactionF(rec, 6, 5)
	b, _, _ := attackReaction([]game.TraceView{{FromX: 6, FromY: 5, ToX: 7, ToY: 5, Melee: true, Age: 5}}, 6, 5)
	if math.Abs(a-b) > 1e-9 {
		t.Fatalf("1x matches the sim-timed reaction: %v %v", a, b)
	}
	// bookkeeping retains a trace after the sim drops it
	r := &Renderer{present: 100}
	st := r.animState()
	st.TrackReactions([]game.TraceView{bite}, 1000, 100)
	if got := st.Reactions(100, nil); len(got) != 1 {
		t.Fatalf("tracked = %d", len(got))
	}
	st.TrackReactions(nil, 1001, 103)
	got := st.Reactions(103, nil)
	if len(got) != 1 || got[0].Frames != 3 || !math.IsInf(got[0].Sim, 1) {
		t.Fatalf("expired trace kept 3 frames: %+v", got)
	}
	st.TrackReactions(nil, 1100, 200)
	if len(st.Reactions(200, nil)) != 0 {
		t.Fatal("old reactions are pruned")
	}
}

// TestTrotEightFrames: the trot runs walk01..walk08 once per cycle of
// phase. PLAY-022 re-expresses the old pins: the cycle spans the set's own
// gait (staticShow, without a motion track) instead of the pawns' 1.6
// tiles, it starts on walk03 (a neutral frame), and the renderer adds no
// bob to an animal (the baked walk carries the body's rise; the old 1.6x
// flee bounce is gone).
func TestTrotEightFrames(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for ph := 0.0; ph < 1; ph += 0.01 {
		seen[trotSuffix(ph)] = true
	}
	if len(seen) != 8 || !seen["_walk01"] || !seen["_walk08"] {
		t.Fatalf("trot has 8 frames walk01..walk08: %v", seen)
	}
	if trotSuffix(1.01) != "_walk01" || trotSuffix(2.0/8+1e-9) != "_walk03" {
		t.Fatal("the frame is the phase's fraction")
	}
	const gait = 0.816 // the deer's
	walk := moveState{Stride: 0, Blend: 1}
	if s := staticShow(game.AnimalView{ID: 1}, walk, 0, gait); s.Suffix != "_walk03" {
		t.Fatalf("a walk starts on walk03, got %q", s.Suffix)
	}
	walk.Stride = gait
	if s := staticShow(game.AnimalView{ID: 1}, walk, 0, gait); s.Suffix != "_walk03" {
		t.Fatalf("the trot repeats every gait (%.3f tiles), got %q", gait, s.Suffix)
	}
	if s := staticShow(game.AnimalView{ID: 1}, moveState{Stride: 0.5}, 0, gait); s.Walk || s.Suffix != "" {
		t.Fatal("idle animals use the static pose")
	}
	r := combatRenderer(100, nil, game.WorldView{})
	for _, gt := range []string{game.GaitAmble, game.GaitWalk, game.GaitRun} {
		an := game.AnimalView{ID: 3, X: 4, Y: 4, Gait: gt, Mode: "fleeing"}
		if ps := r.animalPose(an); ps.Dy != 0 {
			t.Fatalf("%s: the renderer bobs an animal by %v", gt, ps.Dy)
		}
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
