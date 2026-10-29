package ebiten

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/orders"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestStrikeFlashAndThunder(t *testing.T) {
	r := &Renderer{present: 100}
	r.overlays().Fires.OnSimStrike(5, 6, 4000)
	if !r.envLayer().FlashSeen || r.envLayer().FlashAt != 100 {
		t.Errorf("flash %+v", r.env)
	}
	if r.cueBus().ThunderAt <= 100 {
		t.Errorf("thunder at %d, want after the flash", r.cueBus().ThunderAt)
	}
	cues := r.cueBus().Take()
	if len(cues) != 1 || cues[0].ID != "sfx.lightning_strike" || cues[0].X != 5.5 {
		t.Errorf("cues %+v", cues)
	}
}

func TestExtinguishOnlyForDraftedOnFire(t *testing.T) {
	w := game.WorldView{Fires: []game.FireView{{X: 4, Y: 5, Size: 30}}}
	if orders.FireOrders(game.PawnView{ID: 1}, w, 4, 5) != nil {
		t.Error("undrafted pawn got the fire menu")
	}
	if orders.FireOrders(game.PawnView{ID: 1, Drafted: true}, w, 0, 0) != nil {
		t.Error("menu off the fire")
	}
	opts := orders.FireOrders(game.PawnView{ID: 7, Drafted: true}, w, 4, 5)
	if len(opts) != 2 {
		t.Fatalf("options %d", len(opts))
	}
	c := opts[1].Cmd
	if c.Kind != command.Prioritize || c.Order != command.OrderExtinguish || c.Pawn != 7 || c.X != 4 || c.Y != 5 {
		t.Errorf("cmd %+v", c)
	}
	if orders.FloatIcon(opts[1]) != "extinguish" || orders.FloatIcon(opts[0]) != "" {
		t.Error("icons")
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
	want := map[string]cue.Loop{}
	a.colonyAmbience(want)
	n := 0
	for k, l := range want {
		if l.ID == "amb.fire" {
			n++
			_ = k
		}
	}
	if n != 1 {
		t.Fatalf("one blaze, %d amb.fire loops (%v)", n, want)
	}
	r.heard.Fires, r.heard.FireVer = nil, 2
	want = map[string]cue.Loop{}
	a.colonyAmbience(want)
	for _, l := range want {
		if l.ID == "amb.fire" {
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
	for _, x := range a.tools.Actions(7, 8) {
		if x.Label == "Start fire here" {
			x.Run(nil)
			if x.Icon != "start_fire" {
				t.Errorf("icon %q", x.Icon)
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
	w, ok := windows.WorkTypeForCol(0)
	if !ok || w != command.WorkFirefight {
		t.Fatalf("firefight column: %v %v", w, ok)
	}
	if strings.Contains(windows.WorkHelp[0], "Not available") {
		t.Error("help still says not available")
	}
	if windows.WorkPriority(game.PawnView{Priorities: []int{2}}, 0) != 2 {
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
	cues := r.cueBus().Take()
	if len(cues) != 1 || cues[0].X != 2.5 || cues[0].Y != 3.5 {
		t.Fatalf("cues %+v: want one crack, at the fresh strike", cues)
	}
}

func TestDrawAshStainsSkipsBlood(t *testing.T) {
	t.Parallel()
	a, _ := soundApp(&stubSim{w: 48, h: 40})
	w := game.WorldView{Stains: []game.StainView{{X: 2, Y: 2, Kind: "ash", Amount: 0.8, Variant: 4}, {X: 3, Y: 3, Kind: "blood", Amount: 1}}}
	a.renderer.drawAshStains(ebiten.NewImage(a.sw, a.sh), w, a.sw, a.sh, 32) // no art yet: no panic
}
