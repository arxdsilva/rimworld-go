package ebiten

import (
	"math"
	"testing"

	"rimworld-go/internal/application/game"
)

func TestWalkCycleOneStridePerTile(t *testing.T) {
	t.Parallel()
	// two tiles walk the full 8-frame cycle; frames 0 and 4 are neutral
	seen := map[string]int{}
	for i := 0; i < 8; i++ {
		seen[walkFrame(float64(i)/4)]++
	}
	if seen[""] != 2 || seen["_walk1"] != 2 || seen["_walk2"] != 1 || seen["_walk4"] != 1 {
		t.Fatalf("cycle = %v", seen)
	}
	if walkFrame(2) != walkFrame(0) || walkFrame(-0.25) != walkFrame(1.75) {
		t.Fatal("cycle wraps every two tiles")
	}
	if walkPhase(pawnVisual{}, 0.5) != 0 || walkPhase(pawnVisual{steps: 3}, 0.5) != 2.5 {
		t.Fatal("phase counts tiles walked")
	}
}

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
	if !ok || st.burst != partChip {
		t.Fatal("cutting throws chips")
	}
	strikes, peak := 0, 0.0
	for f := uint64(0); f < uint64(st.period)*3; f++ {
		if workStrike(f, 7, st.period) {
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
