package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/command"
)

func TestHomeAutoButtonEmitsAndShowsState(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	sw, sh := a.sw, a.sh
	find := func() uikit.Button {
		b, ok := hasButton(a.colonyButtons(sw, sh), architect.HomeAutoButtonID)
		if !ok {
			t.Fatal("hud.homeauto button missing")
		}
		return b
	}
	if find().Selected {
		t.Fatal("button selected while the view says auto is off")
	}
	// left to right: Roof, Home, Auto home, Plans, Rooms (each its own rect)
	hb, _ := hasButton(a.colonyButtons(sw, sh), hud.HomeButtonID)
	if b := find(); b.X < hb.X+hb.W {
		t.Fatalf("Home auto %+v overlaps Home %+v", b, hb)
	}
	for _, o := range []string{hud.PlanButtonID, hud.RoomsButtonID} {
		ob, _ := hasButton(a.colonyButtons(sw, sh), o)
		if b := find(); ob.X < b.X+b.W {
			t.Fatalf("Home auto %+v overlaps %s %+v", b, o, ob)
		}
	}
	if g := a.hud().Global; find().X < g.X || find().X+find().W > g.Right() {
		t.Fatalf("Home auto %+v is outside Global %+v", find(), g)
	}
	s.world.HomeAuto = true
	if !find().Selected {
		t.Fatal("button shows auto on")
	}
	click(t, a, architect.HomeAutoButtonID)
	if c := lastCmd(t, s); c.Kind != command.SetHomeAuto || c.Enabled {
		t.Fatalf("click with auto on emits %+v, want off", c)
	}
	s.world.HomeAuto = false
	click(t, a, architect.HomeAutoButtonID)
	if c := lastCmd(t, s); c.Kind != command.SetHomeAuto || !c.Enabled {
		t.Fatalf("click with auto off emits %+v, want on", c)
	}
}
