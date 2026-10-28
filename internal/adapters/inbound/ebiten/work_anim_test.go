package ebiten

import (
	"math"
	"slices"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// workRenderer is a renderer whose animT is the stub sim's tick.
func workRenderer(tick int) (*Renderer, *stubSim) {
	sim := &stubSim{tick: clock.Tick(tick)}
	return &Renderer{game: sim}, sim
}

// tickAt is a tick (>= 0) at which pawn id's swing of the given period
// is at phase ph.
func tickAt(id uint64, period int, ph float64) int {
	want := int(ph*float64(period)) + 1
	return ((want-int(id*13))%period+period)%period + 10*period
}

// worker is a pawn standing at (5,5) working the tile to its east.
func worker(action string) game.PawnView {
	return game.PawnView{ID: 3, X: 5, Y: 5, ActX: 6, ActY: 5, Action: action}
}

func TestWorkPoseTable(t *testing.T) {
	t.Parallel()
	actions := []string{
		game.ActionChop, game.ActionMine, game.ActionBuild, game.ActionDeconstruct, game.ActionRepair,
		game.ActionBreach, game.ActionSow, game.ActionHarvest, game.ActionCraft, game.ActionCook,
		game.ActionTailor, game.ActionButcher, game.ActionResearch, game.ActionTend, game.ActionTame,
		game.ActionEat, game.ActionRefuel, game.ActionHorseshoes, game.ActionClean,
	}
	for _, a := range actions {
		d, ok := workTable[a]
		if !ok || d.period <= 0 {
			t.Fatalf("%s: no table row or period", a)
		}
		if d.motion == motionNone {
			t.Fatalf("%s: no motion", a)
		}
		if a != game.ActionEat && a != game.ActionTend && a != game.ActionTame && a != game.ActionResearch && len(d.parts) == 0 {
			t.Fatalf("%s: throws no particles", a)
		}
		r, _ := workRenderer(0)
		if pose := r.workPose(worker(a), moveState{}); !pose.ok || pose.held.key != "" {
			t.Fatalf("%s: work pose must apply, holding nothing", a)
		}
	}
	// combat and carrying belong to other layers
	for _, a := range []string{game.ActionAim, game.ActionMelee, game.ActionRescue, game.ActionHaul, ""} {
		r, _ := workRenderer(0)
		if r.workPose(worker(a), moveState{}).ok {
			t.Fatalf("%q is not hands-on work", a)
		}
	}
	// old Acting strings still map when Action is empty
	for acting, want := range legacyActions {
		if _, _, ok := actionOf(game.PawnView{Acting: acting}); !ok {
			t.Fatalf("legacy %q (-> %s) lost", acting, want)
		}
	}
	// Action wins over Acting; workPose is for standing only
	if act, _, _ := actionOf(game.PawnView{Action: game.ActionMine, Acting: "cut"}); act != game.ActionMine {
		t.Fatal("Action beats Acting")
	}
	r, _ := workRenderer(0)
	if r.workPose(worker(game.ActionChop), moveState{Blend: 1}).ok {
		t.Fatal("no work pose while walking")
	}
}

// Each motion shapes the figure differently: swings wind back and drive
// in with a squash on the strike, reaches lean in and dip, the bench
// bobs, eating bows the head. None holds a tool.
func TestMotionTransforms(t *testing.T) {
	t.Parallel()
	at := func(action string, ph float64) actionPose {
		d := workTable[action]
		r, _ := workRenderer(tickAt(3, d.period, ph))
		return r.workPose(worker(action), moveState{})
	}
	wind, strike := at(game.ActionChop, 0.35), at(game.ActionChop, strikeAt+0.01)
	if wind.dx >= 0 || strike.squash <= wind.squash || strike.dx <= wind.dx || strike.rot <= 0 {
		t.Fatalf("swing: wound back %+v, then driven in, squashed and leaning in %+v", wind, strike)
	}
	for _, a := range []string{game.ActionTend, game.ActionSow, game.ActionHarvest} {
		lo, hi := at(a, strikeAt+0.5), at(a, strikeAt+0.01)
		if hi.squash <= lo.squash || hi.dx <= lo.dx || hi.dy <= lo.dy || hi.rot <= lo.rot {
			t.Fatalf("%s: lean in and dip on the strike: %+v vs %+v", a, hi, lo)
		}
	}
	if lo, hi := at(game.ActionCraft, strikeAt+0.5), at(game.ActionCraft, strikeAt+0.01); hi.dy <= lo.dy {
		t.Fatalf("bench: a bob: %+v vs %+v", hi, lo)
	}
	if lo, hi := at(game.ActionEat, strikeAt+0.5), at(game.ActionEat, strikeAt+0.01); hi.dy <= lo.dy {
		t.Fatalf("eat: the head bows to the meal: %+v vs %+v", hi, lo)
	}
}

// No tool is ever drawn (ART-027): for every work action an off-duty
// pawn's figure draws only its own layers.
func TestWorkDrawsNoTool(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 16, h: 16, tick: 100})
	r.art = figureArt()
	for act := range workTable {
		p := game.PawnView{ID: 3, X: 2, Y: 2, ActX: 3, ActY: 2, Action: act, Weapon: "rifle"}
		ps, h := r.standingPose(p, moveState{Facing: faceEast})
		if h.key != "" {
			t.Fatalf("%s: holds %q", act, h.key)
		}
		var keys []string
		r.lit.OnDraw = func(img *ebiten.Image, _ float32) { keys = append(keys, keyOf(r.art, img)) }
		r.drawPawnFigure(ebiten.NewImage(512, 512), p, 2, 2, 64, pawnFigurePose{face: faceEast, ps: ps, held: h})
		r.lit.OnDraw = nil
		for _, k := range keys {
			if !strings.HasPrefix(k, "pawn/") || k == staleHandKey {
				t.Fatalf("%s draws %s", act, k)
			}
		}
	}
}

// A strike's particles spawn at the work spot: the worked tile's face
// towards the pawn, whichever side the pawn works it from.
func TestWorkParticlesAtWorkSpot(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		px, py int
		x, y   float64
	}{
		{5, 5, 6.5 - workFace, 5.45}, // from the west
		{7, 5, 6.5 + workFace, 5.45}, // from the east
		{6, 4, 6.5, 5.45 - workFace}, // from the north
		{6, 6, 6.5, 5.45 + workFace}, // from the south
		{6, 5, 6.5, 5.45},            // standing on it
	} {
		for _, act := range []string{game.ActionChop, game.ActionSow, game.ActionHarvest, game.ActionClean, game.ActionBuild} {
			d := workTable[act]
			r, sim := workRenderer(tickAt(3, d.period, 0.1))
			p := game.PawnView{ID: 3, X: c.px, Y: c.py, ActX: 6, ActY: 5, Action: act}
			r.animateWork(game.WorldView{}, []game.PawnView{p})
			sim.tick = clock.Tick(tickAt(3, d.period, strikeAt) + d.period)
			r.animateWork(game.WorldView{}, []game.PawnView{p})
			if len(r.parts) == 0 {
				t.Fatalf("%s from (%d,%d): no particles", act, c.px, c.py)
			}
			for _, q := range r.parts {
				if math.Abs(q.X-c.x) > 1e-9 || math.Abs(q.Y-c.y) > 1e-9 {
					t.Fatalf("%s from (%d,%d): particle at %v,%v, want %v,%v", act, c.px, c.py, q.X, q.Y, c.x, c.y)
				}
			}
		}
	}
}

func TestChopStrikeParticlesAtTrunk(t *testing.T) {
	t.Parallel()
	r, sim := workRenderer(tickAt(3, 46, 0.1))
	p := worker(game.ActionChop)
	r.animateWork(game.WorldView{}, []game.PawnView{p}) // arms prev
	if len(r.parts) != 0 {
		t.Fatal("no particles before the strike")
	}
	sim.tick = clock.Tick(tickAt(3, 46, strikeAt) + 46)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	if len(r.parts) != 4 {
		t.Fatalf("one swing throws 4 chips, got %d", len(r.parts))
	}
	for _, q := range r.parts {
		if q.Kind != partChip || q.X != 6.5-workFace || q.Y != 5.45 {
			t.Fatalf("chips fly from the trunk's face: %+v", q)
		}
	}
	// the strike: leaning in, no axe drawn
	r.mo.Alpha = 0
	a := r.workPose(p, moveState{})
	if a.held.key != "" {
		t.Fatalf("chop pose: %+v", a)
	}
	if a.dx <= 0 {
		t.Fatalf("leans towards the trunk: dx=%v", a.dx)
	}
}

func TestMineSparks(t *testing.T) {
	t.Parallel()
	r, sim := workRenderer(tickAt(3, 50, 0.1))
	p := worker(game.ActionMine)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	sim.tick = clock.Tick(tickAt(3, 50, strikeAt) + 50)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	var sparks, dust int
	for _, q := range r.parts {
		switch q.Kind {
		case partSpark:
			sparks++
			if q.Vx > 0.05 { // worked tile is east: sparks glance back west
				t.Fatalf("sparks glance back: vx=%v", q.Vx)
			}
		case partDust:
			dust++
		}
	}
	if sparks == 0 || dust == 0 {
		t.Fatalf("sparks %d and dust %d", sparks, dust)
	}
	if a := r.workPose(p, moveState{}); !a.ok || a.held.key != "" {
		t.Fatalf("mine pose: %+v", a)
	}
}

func TestSowKneels(t *testing.T) {
	t.Parallel()
	r, sim := workRenderer(tickAt(3, 48, 0.1))
	p := worker(game.ActionSow)
	a := r.workPose(p, moveState{})
	if a.held.key != "" || a.squash < 0.08 {
		t.Fatalf("sow kneels and plants, nothing in hand: %+v", a)
	}
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	sim.tick = clock.Tick(tickAt(3, 48, strikeAt) + 48)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	if len(r.parts) == 0 || r.parts[0].Kind != partSoil || r.parts[0].X != 6.5-workFace || r.parts[0].Y != 5.45 {
		t.Fatalf("soil flies at the sown spot: %+v", r.parts)
	}
}

// TestCleanSweeps: cleaning (SIM-021) is the low reach, throwing dust.
func TestCleanSweeps(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(0)
	d := workTable[game.ActionClean]
	if d.motion != motionReach || len(d.parts) == 0 || d.parts[0].kind != partDust {
		t.Fatalf("clean row: %+v", d)
	}
	a := r.workPose(worker(game.ActionClean), moveState{})
	if !a.ok || a.held.key != "" || a.squash <= 0 {
		t.Fatalf("clean pose %+v", a)
	}
	if actionSounds[game.ActionClean] != "sfx.sweep" {
		t.Fatalf("clean sound %q", actionSounds[game.ActionClean])
	}
}

func TestHarvestPicks(t *testing.T) {
	t.Parallel()
	r, sim := workRenderer(tickAt(3, 44, strikeAt))
	p := worker(game.ActionHarvest)
	a := r.workPose(p, moveState{})
	if a.held.key != "" || a.squash <= 0 || a.dx <= 0 {
		t.Fatalf("harvest crouches and picks: %+v", a)
	}
	sim.tick = clock.Tick(tickAt(3, 44, 0.1))
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	sim.tick = clock.Tick(tickAt(3, 44, strikeAt) + 44)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	if len(r.parts) == 0 || r.parts[0].Kind != partLeaf {
		t.Fatal("leaves fly")
	}
}

func TestBenchActions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		action string
		part   partKind
	}{
		{game.ActionCraft, partShaving}, {game.ActionCook, partSteam}, {game.ActionTailor, partShaving},
		{game.ActionResearch, partNone}, {game.ActionButcher, partBlood},
	}
	for _, c := range cases {
		d := workTable[c.action]
		r, sim := workRenderer(tickAt(3, d.period, 0.1))
		p := worker(c.action)
		if a := r.workPose(p, moveState{}); workTable[c.action].motion != motionBench || !a.ok || a.held.key != "" {
			t.Fatalf("%s: bench motion, nothing in hand: %+v", c.action, a)
		}
		r.animateWork(game.WorldView{}, []game.PawnView{p})
		sim.tick = clock.Tick(tickAt(3, d.period, strikeAt) + d.period)
		r.animateWork(game.WorldView{}, []game.PawnView{p})
		if c.part == partNone {
			if len(r.parts) != 0 {
				t.Fatalf("%s throws nothing", c.action)
			}
			continue
		}
		if len(r.parts) == 0 || r.parts[0].Kind != c.part {
			t.Fatalf("%s: want particle %v, got %+v", c.action, c.part, r.parts)
		}
	}
	// the stove top versus the campfire
	r, sim := workRenderer(tickAt(3, 40, 0.1))
	p := worker(game.ActionCook)
	p.Acting = "work at campfire"
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	sim.tick = clock.Tick(tickAt(3, 40, strikeAt) + 40)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	if r.parts[0].X != 6.5 || r.parts[0].Y != 4.9 {
		t.Fatalf("steam rises over the campfire: %+v", r.parts[0])
	}
}

func TestTendKneels(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(tickAt(3, 60, 0.6))
	a := r.workPose(worker(game.ActionTend), moveState{})
	if a.held.key != "" || a.squash < 0.1 || a.dx <= 0 {
		t.Fatalf("tend: %+v", a)
	}
}

func TestTameOffersHand(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(tickAt(3, 64, 0.2))
	a := r.workPose(worker(game.ActionTame), moveState{})
	if !a.ok || a.held.key != "" || a.squash <= 0 {
		t.Fatalf("tame crouches, bare hand out: %+v", a)
	}
}

// Eating shows nothing in hand (no bowl or spoon): the head bobs.
func TestEatShowsNothingInHand(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(tickAt(3, 50, 0.2))
	a := r.workPose(worker(game.ActionEat), moveState{})
	if !a.ok || a.held.key != "" || a.dx <= 0 {
		t.Fatalf("eat: %+v", a)
	}
}

// A hauled item is the item's own sprite, centred on the body and raised
// a little, drawn in front of it; it rides the walking figure.
func TestLoadInFrontOfBody(t *testing.T) {
	t.Parallel()
	r := artRenderer()
	p := game.PawnView{ID: 1, Carrying: "wood"}
	for _, stride := range []float64{0, 0.14, 0.8, 1.59} {
		m := walkingState
		m.Stride = stride
		a := r.carryPose(p, m)
		if !a.ok || a.dx != 0 || a.dy != 0 || a.rot != 0 {
			t.Fatalf("stride %v: the load rides the walking figure (gait from walkPose): %+v", stride, a)
		}
		if a.held.key != itemSprite("wood") || a.held.stance != stanceLoad {
			t.Fatalf("the load is the item sprite: %+v", a.held)
		}
	}
	if a := r.carryPose(game.PawnView{ID: 1, Carrying: "stone"}, moveState{}); !a.ok || a.held.stance != stanceLoad || !a.held.set {
		t.Fatalf("standing carry: %+v", a)
	}
	if r.carryPose(game.PawnView{ID: 1}, moveState{}).ok {
		t.Fatal("nothing carried, no carry pose")
	}
	// the walking carrier still bobs: the pose stacks on the gait
	m := walkingState
	m.Stride = 0.5 / walkFrames * strideLen
	r.mo.Alpha = 0
	ps, h := r.standingPose(p, m)
	if ps.Dy >= 0 || h.stance != stanceLoad {
		t.Fatalf("a walking carrier bobs with its load: %+v %+v", ps, h)
	}
	// placement: centred on the body, at the chest (just under the body
	// centre, clear of the face), in front
	h = heldItem{key: "item/wood_pile", stance: stanceLoad}
	for _, f := range []facing{faceSouth, faceEast, faceWest} {
		rig, ps := rigFor(r, f)
		pl, ok := r.heldPlace(rig, h, 1, 1, 64, ps)
		if !ok || pl.behind {
			t.Fatalf("%v: the load in front of the body: %+v", f, pl)
		}
		mid := heldPivotOn(r, pl, h)
		c := aimCentre(r, rig, ps, 1, 1, 64)
		if math.Abs(mid[0]-c[0]) > 1e-6 || mid[1] < c[1] || mid[1]-c[1] > 0.15*64 {
			t.Fatalf("%v: load centre %v, body centre %v", f, mid, c)
		}
		if w := pl.g.Element(0, 0); math.Abs(math.Abs(w)*artTilePx-0.5*64) > 1e-6 {
			t.Fatalf("%v: the load is about half a tile: %v px", f, math.Abs(w)*artTilePx)
		}
	}
}

func TestLoadBehindFacingNorth(t *testing.T) {
	t.Parallel()
	r := New(&stubSim{w: 16, h: 16, tick: 100})
	r.art = figureArt()
	p := game.PawnView{ID: 1, X: 2, Y: 2, Carrying: "wood"}
	for _, f := range []facing{faceNorth, faceSouth, faceEast, faceWest} {
		ps, h := r.standingPose(p, moveState{Facing: f})
		var keys []string
		r.lit.OnDraw = func(img *ebiten.Image, _ float32) { keys = append(keys, keyOf(r.art, img)) }
		r.drawPawnFigure(ebiten.NewImage(512, 512), p, 2, 2, 64, pawnFigurePose{face: f, ps: ps, held: h})
		r.lit.OnDraw = nil
		body := slices.IndexFunc(keys, func(k string) bool { return strings.HasPrefix(k, "pawn/body_") })
		load := slices.Index(keys, "item/wood_pile")
		if body < 0 || load < 0 || (load < body) != (f == faceNorth) {
			t.Fatalf("%v: body %d load %d (%v)", f, body, load, keys)
		}
	}
}

func TestRescueKeepsPatientDrawn(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(0)
	a := r.carryPose(game.PawnView{ID: 1, Carrying: "pawn", Action: game.ActionRescue}, walkingState)
	if !a.ok {
		t.Fatalf("rescuer carries: %+v", a)
	}
	if !a.held.set || a.held.key != "" {
		t.Fatalf("bare hands; drawCarried draws the patient: %+v", a.held)
	}
	if r.workPose(game.PawnView{ID: 1, Carrying: "pawn", Action: game.ActionRescue}, moveState{}).ok {
		t.Fatal("rescue is not a work pose")
	}
}

func TestWorkPausedFreezes(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(tickAt(3, 46, 0.3))
	p := worker(game.ActionChop)
	first := r.workPose(p, moveState{})
	for i := 0; i < 50; i++ { // animT does not advance
		r.animateWork(game.WorldView{}, []game.PawnView{p})
		if got := r.workPose(p, moveState{}); got != first {
			t.Fatalf("pose changed while paused: %+v vs %+v", got, first)
		}
	}
	if len(r.parts) != 0 {
		t.Fatal("no strikes while paused")
	}
}

func TestWorkThreeTimesFasterAtThreeX(t *testing.T) {
	t.Parallel()
	strikes := func(perFrame int) int {
		r, sim := workRenderer(100)
		p := worker(game.ActionChop)
		n := 0
		for i := 0; i < 230; i++ {
			sim.tick += clock.Tick(perFrame)
			before := len(r.parts)
			r.animateWork(game.WorldView{}, []game.PawnView{p})
			if len(r.parts) > before {
				n++
			}
		}
		return n
	}
	one, three := strikes(1), strikes(3)
	if one < 3 || three < 3*one-1 || three > 3*one+1 {
		t.Fatalf("strikes at 1x=%d, 3x=%d", one, three)
	}
}
