package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestFireClassBoundaries(t *testing.T) {
	for size, want := range map[int]string{1: "small", 33: "small", 34: "medium", 66: "medium", 67: "large", 100: "large"} {
		if got := FireClassOf(size); got != want {
			t.Errorf("size %d: %s, want %s", size, got, want)
		}
	}
}

func TestFireFrameAnimatesAndStaggers(t *testing.T) {
	seen := map[int]bool{}
	for p := uint64(0); p < 40; p++ {
		f := FireFrameOf(p, 3, 4)
		if f < 0 || f >= FireFrames {
			t.Fatalf("frame %d out of range", f)
		}
		seen[f] = true
	}
	if len(seen) != FireFrames {
		t.Errorf("frames used %v, want all %d", seen, FireFrames)
	}
	same := true
	for x := 0; x < 8; x++ {
		if FireFrameOf(0, x, 0) != FireFrameOf(0, 0, 0) {
			same = false
		}
	}
	if same {
		t.Error("neighbouring cells flicker in step")
	}
	if k := FireSpriteKey(80, 0, 0, 0); k[:len("effect/fire_large_")] != "effect/fire_large_" {
		t.Errorf("key %q", k)
	}
}

func TestFireEmissionAndLightScaleWithSize(t *testing.T) {
	if !(FireEmberEvery(10) > FireEmberEvery(50) && FireEmberEvery(50) > FireEmberEvery(90)) {
		t.Error("embers do not get more frequent with size")
	}
	if !(FireSmokeEvery(10) > FireSmokeEvery(50) && FireSmokeEvery(50) > FireSmokeEvery(90)) {
		t.Error("smoke does not get more frequent with size")
	}
	if !(FireLightReach(10) < FireLightReach(50) && FireLightReach(50) < FireLightReach(90)) {
		t.Error("light reach does not grow with size")
	}
	if !(FireLightStrength(10) < FireLightStrength(50) && FireLightStrength(50) < FireLightStrength(90)) {
		t.Error("light strength does not grow with size")
	}
}

func TestFireClustersOnePerBlaze(t *testing.T) {
	fires := []game.FireView{{X: 1, Y: 1, Size: 20}, {X: 2, Y: 2, Size: 70}, {X: 3, Y: 2, Size: 10}, {X: 20, Y: 20, Size: 5}}
	cs := FireClusters(fires)
	if len(cs) != 2 {
		t.Fatalf("clusters %d, want 2", len(cs))
	}
	if cs[0].N != 3 || cs[0].MaxSize != 70 || cs[0].Key != [2]int{1, 1} {
		t.Errorf("big cluster %+v", cs[0])
	}
	if cs[1].N != 1 || cs[1].X != 20.5 {
		t.Errorf("lone cluster %+v", cs[1])
	}
	if FireClusters(nil) != nil {
		t.Error("no fire, no cluster")
	}
	if !(FireLoopGain(cs[0]) > FireLoopGain(cs[1])) || FireLoopGain(cs[0]) > 1 {
		t.Error("loop gain does not grow with the blaze or exceeds 1")
	}
}

func TestAshKey(t *testing.T) {
	if AshKey(0) != "overlay/ash_0" || AshKey(4) != "overlay/ash_1" {
		t.Errorf("%s %s", AshKey(0), AshKey(4))
	}
}

func TestBlazeLightScalesWithSizeAndRebuilds(t *testing.T) {
	t.Parallel()
	const w, h = 12, 12
	small := BlazeLight(nil, []game.FireView{{X: 6, Y: 6, Size: 10}}, w, h)
	large := BlazeLight(nil, []game.FireView{{X: 6, Y: 6, Size: 90}}, w, h)
	if !(large[6*w+6] > small[6*w+6]) {
		t.Errorf("centre light small %v large %v", small[6*w+6], large[6*w+6])
	}
	if small[6*w+9] != 0 || large[6*w+9] == 0 {
		t.Errorf("reach: small at 3 tiles %v, large %v", small[6*w+9], large[6*w+9])
	}
	if large[6*w+7] >= large[6*w+6] {
		t.Error("light does not fall off")
	}
	// the buffer is reused and cleared when the fire goes out or sits on the edge
	again := BlazeLight(large, []game.FireView{{X: 0, Y: 0, Size: 90}}, w, h)
	if again[6*w+6] != 0 || again[0] == 0 {
		t.Error("stale light kept or edge fire lost")
	}
	if len(BlazeLight(again, nil, w, h)) != 0 {
		t.Error("no fire, no grid")
	}
}
