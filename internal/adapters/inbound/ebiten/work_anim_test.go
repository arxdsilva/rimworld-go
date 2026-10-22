package ebiten

import (
	"fmt"
	"strings"
	"testing"

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
		if d.set == "" && d.fallback == motionNone {
			t.Fatalf("%s: neither pose set nor generic fallback", a)
		}
		if d.set != "" && (d.strike < 1 || d.strike > d.frames) {
			t.Fatalf("%s: strike frame %d outside 1..%d", a, d.strike, d.frames)
		}
		if a != game.ActionEat && a != game.ActionTend && a != game.ActionTame && a != game.ActionResearch && len(d.parts) == 0 {
			t.Fatalf("%s: throws no particles", a)
		}
		r, _ := workRenderer(0)
		if pose := r.workPose(worker(a), moveState{}); !pose.ok || !pose.held.set {
			t.Fatalf("%s: work pose must apply and set the held item", a)
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

func TestSetFrameOrdering(t *testing.T) {
	t.Parallel()
	if got := setFrame("_chop", 6, 4, strikeAt); got != "_chop4" {
		t.Fatalf("strike at strikeAt: %s", got)
	}
	seen := map[string]bool{}
	prev := 0
	for ph := 0.0; ph < 1; ph += 0.005 {
		f := setFrame("_chop", 6, 4, ph)
		var n int
		fmt.Sscanf(f, "_chop%d", &n)
		if n < prev {
			t.Fatalf("frames go backwards at %v: %s", ph, f)
		}
		prev = n
		seen[f] = true
	}
	if len(seen) != 6 {
		t.Fatalf("all six chop frames show: %v", seen)
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
		if q.kind != partChip || q.x != 6.5 || q.y != 5.45 {
			t.Fatalf("chips fly from the trunk tile: %+v", q)
		}
	}
	// strike frame, axe in both hands
	r.mo.alpha = 0
	a := r.workPose(p, moveState{})
	if a.suffix != "_chop4" || a.held.key != "tool/axe" || a.held.stance != stanceHands {
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
		switch q.kind {
		case partSpark:
			sparks++
			if q.vx > 0.05 { // worked tile is east: sparks glance back west
				t.Fatalf("sparks glance back: vx=%v", q.vx)
			}
		case partDust:
			dust++
		}
	}
	if sparks == 0 || dust == 0 {
		t.Fatalf("sparks %d and dust %d", sparks, dust)
	}
	a := r.workPose(p, moveState{})
	if a.suffix != "_mine4" || a.held.key != "tool/pickaxe" || a.held.stance != stanceHands {
		t.Fatalf("mine pose: %+v", a)
	}
}

func TestSowKneels(t *testing.T) {
	t.Parallel()
	r, sim := workRenderer(tickAt(3, 48, 0.1))
	p := worker(game.ActionSow)
	a := r.workPose(p, moveState{})
	if !strings.HasPrefix(a.suffix, "_plant") || a.held.key != "tool/trowel" || a.squash < 0.08 {
		t.Fatalf("sow kneels and plants with a trowel: %+v", a)
	}
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	sim.tick = clock.Tick(tickAt(3, 48, strikeAt) + 48)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	if len(r.parts) == 0 || r.parts[0].kind != partSoil || r.parts[0].x != 5.5 || r.parts[0].y != 5.6 {
		t.Fatalf("soil flies at the hand: %+v", r.parts)
	}
	// plant frames missing: kneel, then the generic frames
	r.art = &artCatalog{sprites: map[string]*artSprite{"pawn/body_east_kneel": {}}}
	if a := r.workPose(p, moveState{}); a.suffix != "_kneel" {
		t.Fatalf("falls back to _kneel: %q", a.suffix)
	}
	r.art = &artCatalog{sprites: map[string]*artSprite{}}
	if a := r.workPose(p, moveState{}); !strings.HasPrefix(a.suffix, "_work") {
		t.Fatalf("falls back to generic work frames: %q", a.suffix)
	}
}

// TestCleanSweeps: cleaning (SIM-021) is the low pick pose with a broom,
// upright, throwing dust.
func TestCleanSweeps(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(0)
	d := workTable[game.ActionClean]
	if d.set != "_pick" || d.tool != "tool/broom" || d.stance != stanceUpright || len(d.parts) == 0 || d.parts[0].kind != partDust {
		t.Fatalf("clean row: %+v", d)
	}
	a := r.workPose(worker(game.ActionClean), moveState{})
	if !a.ok || !strings.HasPrefix(a.suffix, "_pick") || a.held.key != "tool/broom" || a.squash <= 0 {
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
	if a.suffix != "_pick3" || a.held.key != "tool/sickle" || a.squash <= 0 {
		t.Fatalf("harvest crouches and picks with a sickle: %+v", a)
	}
	sim.tick = clock.Tick(tickAt(3, 44, 0.1))
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	sim.tick = clock.Tick(tickAt(3, 44, strikeAt) + 44)
	r.animateWork(game.WorldView{}, []game.PawnView{p})
	if len(r.parts) == 0 || r.parts[0].kind != partLeaf {
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
		if a := r.workPose(p, moveState{}); !strings.HasPrefix(a.suffix, "_bench") || a.held.key != "" || !a.held.set {
			t.Fatalf("%s: bench frames with bare hands: %+v", c.action, a)
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
		if len(r.parts) == 0 || r.parts[0].kind != c.part {
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
	if r.parts[0].x != 6.5 || r.parts[0].y != 4.9 {
		t.Fatalf("steam rises over the campfire: %+v", r.parts[0])
	}
}

func TestTendKneelsWithBandage(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(tickAt(3, 60, 0.6))
	a := r.workPose(worker(game.ActionTend), moveState{})
	if !strings.HasPrefix(a.suffix, "_tend") || a.held.key != "tool/bandage" || a.squash < 0.1 {
		t.Fatalf("tend: %+v", a)
	}
}

func TestTameOffersHand(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(tickAt(3, 64, 0.2))
	a := r.workPose(worker(game.ActionTame), moveState{})
	if !strings.HasPrefix(a.suffix, "_offer") || a.held.key != "" || !a.held.set || a.squash <= 0 {
		t.Fatalf("tame crouches, bare hand out: %+v", a)
	}
}

func TestEatBowlSpoon(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(tickAt(3, 50, 0.2))
	// Spoon in the right hand, bowl level in the left (hand_l).
	a := r.workPose(worker(game.ActionEat), moveState{})
	if !strings.HasPrefix(a.suffix, "_eat") || a.held.key != "tool/spoon" || a.held.off != "tool/bowl" {
		t.Fatalf("eat: %+v", a)
	}
}

func TestCarryPoseWhileWalking(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(0)
	p := game.PawnView{ID: 1, Carrying: "wood"}
	for _, stride := range []float64{0, 0.14, 0.8, 1.59} {
		m := walkingState
		m.Stride = stride
		a := r.carryPose(p, m)
		want := fmt.Sprintf("_carry_walk%02d", walkIndex(stride)+1)
		if !a.ok || a.suffix != want {
			t.Fatalf("stride %v: suffix %q, want %q", stride, a.suffix, want)
		}
		if a.held.key != itemSprite("wood") || a.held.stance != stanceLoad {
			t.Fatalf("the load is the item sprite between the hands: %+v", a.held)
		}
	}
	// no carry-walk art: the plain walk frame stays
	r.art = &artCatalog{sprites: map[string]*artSprite{}}
	m := walkingState
	m.Stride = 0.8
	if a := r.carryPose(p, m); !a.ok || a.suffix != "" {
		t.Fatalf("falls back to the walk frame: %+v", a)
	}
}

func TestCarryPoseStanding(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(0)
	a := r.carryPose(game.PawnView{ID: 1, Carrying: "stone"}, moveState{})
	if !a.ok || a.suffix != "_carry" || a.held.stance != stanceLoad {
		t.Fatalf("standing carry: %+v", a)
	}
	if r.carryPose(game.PawnView{ID: 1}, moveState{}).ok {
		t.Fatal("nothing carried, no carry pose")
	}
}

func TestRescueKeepsPatientDrawn(t *testing.T) {
	t.Parallel()
	r, _ := workRenderer(0)
	a := r.carryPose(game.PawnView{ID: 1, Carrying: "pawn", Action: game.ActionRescue}, walkingState)
	if !a.ok || !strings.HasPrefix(a.suffix, "_carry_walk") {
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
