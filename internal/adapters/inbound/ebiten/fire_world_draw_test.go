package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestFireClassBoundaries(t *testing.T) {
	for size, want := range map[int]string{1: "small", 33: "small", 34: "medium", 66: "medium", 67: "large", 100: "large"} {
		if got := fireClassOf(size); got != want {
			t.Errorf("size %d: %s, want %s", size, got, want)
		}
	}
}

func TestFireFrameAnimatesAndStaggers(t *testing.T) {
	seen := map[int]bool{}
	for p := uint64(0); p < 40; p++ {
		f := fireFrameOf(p, 3, 4)
		if f < 0 || f >= fireFrames {
			t.Fatalf("frame %d out of range", f)
		}
		seen[f] = true
	}
	if len(seen) != fireFrames {
		t.Errorf("frames used %v, want all %d", seen, fireFrames)
	}
	same := true
	for x := 0; x < 8; x++ {
		if fireFrameOf(0, x, 0) != fireFrameOf(0, 0, 0) {
			same = false
		}
	}
	if same {
		t.Error("neighbouring cells flicker in step")
	}
	if k := fireSpriteKey(80, 0, 0, 0); k[:len("effect/fire_large_")] != "effect/fire_large_" {
		t.Errorf("key %q", k)
	}
}

func TestFireEmissionAndLightScaleWithSize(t *testing.T) {
	if !(fireEmberEvery(10) > fireEmberEvery(50) && fireEmberEvery(50) > fireEmberEvery(90)) {
		t.Error("embers do not get more frequent with size")
	}
	if !(fireSmokeEvery(10) > fireSmokeEvery(50) && fireSmokeEvery(50) > fireSmokeEvery(90)) {
		t.Error("smoke does not get more frequent with size")
	}
	if !(fireLightReach(10) < fireLightReach(50) && fireLightReach(50) < fireLightReach(90)) {
		t.Error("light reach does not grow with size")
	}
	if !(fireLightStrength(10) < fireLightStrength(50) && fireLightStrength(50) < fireLightStrength(90)) {
		t.Error("light strength does not grow with size")
	}
}

func TestFireClustersOnePerBlaze(t *testing.T) {
	fires := []game.FireView{{X: 1, Y: 1, Size: 20}, {X: 2, Y: 2, Size: 70}, {X: 3, Y: 2, Size: 10}, {X: 20, Y: 20, Size: 5}}
	cs := fireClusters(fires)
	if len(cs) != 2 {
		t.Fatalf("clusters %d, want 2", len(cs))
	}
	if cs[0].N != 3 || cs[0].MaxSize != 70 || cs[0].Key != [2]int{1, 1} {
		t.Errorf("big cluster %+v", cs[0])
	}
	if cs[1].N != 1 || cs[1].X != 20.5 {
		t.Errorf("lone cluster %+v", cs[1])
	}
	if fireClusters(nil) != nil {
		t.Error("no fire, no cluster")
	}
	if !(fireLoopGain(cs[0]) > fireLoopGain(cs[1])) || fireLoopGain(cs[0]) > 1 {
		t.Error("loop gain does not grow with the blaze or exceeds 1")
	}
}

func TestStrikeFlashAndThunder(t *testing.T) {
	r := &Renderer{present: 100}
	r.onSimStrike(5, 6, 4000)
	if !r.env.flashSeen || r.env.flashAt != 100 {
		t.Errorf("flash %+v", r.env)
	}
	if r.thunderAt <= 100 {
		t.Errorf("thunder at %d, want after the flash", r.thunderAt)
	}
	cues := r.takeCues()
	if len(cues) != 1 || cues[0].id != "sfx.lightning_strike" || cues[0].x != 5.5 {
		t.Errorf("cues %+v", cues)
	}
}

func TestExtinguishOnlyForDraftedOnFire(t *testing.T) {
	w := game.WorldView{Fires: []game.FireView{{X: 4, Y: 5, Size: 30}}}
	if fireOrders(game.PawnView{ID: 1}, w, 4, 5) != nil {
		t.Error("undrafted pawn got the fire menu")
	}
	if fireOrders(game.PawnView{ID: 1, Drafted: true}, w, 0, 0) != nil {
		t.Error("menu off the fire")
	}
	opts := fireOrders(game.PawnView{ID: 7, Drafted: true}, w, 4, 5)
	if len(opts) != 2 {
		t.Fatalf("options %d", len(opts))
	}
	c := opts[1].Cmd
	if c.Kind != command.Prioritize || c.Order != command.OrderExtinguish || c.Pawn != 7 || c.X != 4 || c.Y != 5 {
		t.Errorf("cmd %+v", c)
	}
	if floatIcon(opts[1]) != "extinguish" || floatIcon(opts[0]) != "" {
		t.Error("icons")
	}
}

func TestAshKey(t *testing.T) {
	if ashKey(0) != "overlay/ash_0" || ashKey(4) != "overlay/ash_1" {
		t.Errorf("%s %s", ashKey(0), ashKey(4))
	}
}

func TestBlazeLightScalesWithSizeAndRebuilds(t *testing.T) {
	t.Parallel()
	const w, h = 12, 12
	small := blazeLight(nil, []game.FireView{{X: 6, Y: 6, Size: 10}}, w, h)
	large := blazeLight(nil, []game.FireView{{X: 6, Y: 6, Size: 90}}, w, h)
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
	again := blazeLight(large, []game.FireView{{X: 0, Y: 0, Size: 90}}, w, h)
	if again[6*w+6] != 0 || again[0] == 0 {
		t.Error("stale light kept or edge fire lost")
	}
	if len(blazeLight(again, nil, w, h)) != 0 {
		t.Error("no fire, no grid")
	}
}

func TestLightGridPicksUpBlazeByVersion(t *testing.T) {
	r := &Renderer{}
	v := roomWorld()
	v.Fires, v.FireVer = []game.FireView{{X: 4, Y: 1, Size: 90}}, 1
	r.view.W = v
	if _, lamp := r.lightTile(4, 1); lamp <= 0 {
		t.Fatalf("a burning cell lights its tile, lamp %v", lamp)
	}
	v.Fires, v.FireVer = nil, 2
	r.view.W = v
	if _, lamp := r.lightTile(4, 1); lamp != 0 {
		t.Fatalf("light stays after the fire is out, lamp %v", lamp)
	}
}

func TestBlazeAmbiencePerCluster(t *testing.T) {
	t.Parallel()
	a, _ := soundApp(&stubSim{w: 48, h: 40})
	r := a.renderer
	r.cam.Zoom = 1.5
	cx, cy := centreTile(a)
	x, y := int(cx), int(cy)
	r.heard = game.WorldView{Hour: 12, FireVer: 1, Fires: []game.FireView{
		{X: x, Y: y, Size: 50}, {X: x + 1, Y: y, Size: 50}, {X: x, Y: y + 1, Size: 50},
	}}
	want := map[string]loopWant{}
	a.colonyAmbience(want)
	n := 0
	for k, l := range want {
		if l.id == "amb.fire" {
			n++
			_ = k
		}
	}
	if n != 1 {
		t.Fatalf("one blaze, %d amb.fire loops (%v)", n, want)
	}
	r.heard.Fires, r.heard.FireVer = nil, 2
	want = map[string]loopWant{}
	a.colonyAmbience(want)
	for _, l := range want {
		if l.id == "amb.fire" {
			t.Fatal("silence after the fire is out")
		}
	}
}

func TestDrawWorldFiresAndParticles(t *testing.T) {
	t.Parallel()
	a, _ := soundApp(&stubSim{w: 48, h: 40})
	r := a.renderer
	w := game.WorldView{Fires: []game.FireView{{X: 3, Y: 3, Size: 90}, {X: 4, Y: 3, Size: 5}}}
	for p := uint64(0); p < 200; p++ {
		r.present = p
		r.emitBlazeParticles(w)
	}
	if len(r.parts) == 0 {
		t.Error("a blaze emits no embers or smoke")
	}
	screen := ebiten.NewImage(a.sw, a.sh)
	r.drawWorldFires(screen, w, 32) // no art for the effect: the procedural flame, no panic
	r.drawWorldFires(screen, game.WorldView{}, 32)
}

func TestDevStartFireEmitsCommand(t *testing.T) {
	t.Parallel()
	s := bioSim()
	a, _ := colonyApp(t, s)
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	for _, x := range a.devActions(7, 8) {
		if x.label == "Start fire here" {
			x.run(nil)
			if x.icon != "start_fire" {
				t.Errorf("icon %q", x.icon)
			}
			c := s.cmds[len(s.cmds)-1]
			if c.Kind != command.DevStartFire || c.X != 7 || c.Y != 8 {
				t.Errorf("cmd %+v", c)
			}
			return
		}
	}
	t.Fatal("no Start fire action")
}

func TestFirefightColumnLive(t *testing.T) {
	t.Parallel()
	w, ok := workTypeForCol(0)
	if !ok || w != command.WorkFirefight {
		t.Fatalf("firefight column: %v %v", w, ok)
	}
	if strings.Contains(workHelp[0], "Not available") {
		t.Error("help still says not available")
	}
	if workPriority(game.PawnView{Priorities: []int{2}}, 0) != 2 {
		t.Error("priority not read")
	}
}

func TestShowStrikesOncePerStrikeAndNotOldOnes(t *testing.T) {
	t.Parallel()
	sim := &stubSim{w: 20, h: 20}
	sim.tick = 5000
	r := New(sim)
	w := game.WorldView{Strikes: []game.StrikeView{{X: 1, Y: 1, Tick: 100}, {X: 2, Y: 3, Tick: 4990}}}
	r.showStrikes(w)
	r.showStrikes(w) // the same view again: no second crack
	cues := r.takeCues()
	if len(cues) != 1 || cues[0].x != 2.5 || cues[0].y != 3.5 {
		t.Fatalf("cues %+v: want one crack, at the fresh strike", cues)
	}
}

func TestDrawAshStainsSkipsBlood(t *testing.T) {
	t.Parallel()
	a, _ := soundApp(&stubSim{w: 48, h: 40})
	w := game.WorldView{Stains: []game.StainView{{X: 2, Y: 2, Kind: "ash", Amount: 0.8, Variant: 4}, {X: 3, Y: 3, Kind: "blood", Amount: 1}}}
	a.renderer.drawAshStains(ebiten.NewImage(a.sw, a.sh), w, a.sw, a.sh, 32) // no art yet: no panic
}
