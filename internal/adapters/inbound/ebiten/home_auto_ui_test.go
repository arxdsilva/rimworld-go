package ebiten

import (
	"testing"

	"rimworld-go/internal/domain/sim/command"
)

func TestHomeAutoButtonEmitsAndShowsState(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	sw, sh := a.sw, a.sh
	find := func() button {
		b, ok := hasButton(a.colonyButtons(sw, sh), homeAutoButtonID)
		if !ok {
			t.Fatal("hud.homeauto button missing")
		}
		return b
	}
	if find().selected {
		t.Fatal("button selected while the view says auto is off")
	}
	// left to right: Roof, Home, Auto home, Plans, Rooms (each its own rect)
	hb, _ := hasButton(a.colonyButtons(sw, sh), homeButtonID)
	if b := find(); b.x < hb.x+hb.w {
		t.Fatalf("Home auto %+v overlaps Home %+v", b, hb)
	}
	for _, o := range []string{planButtonID, roomsButtonID} {
		ob, _ := hasButton(a.colonyButtons(sw, sh), o)
		if b := find(); ob.x < b.x+b.w {
			t.Fatalf("Home auto %+v overlaps %s %+v", b, o, ob)
		}
	}
	if g := a.hud().Global; find().x < g.X || find().x+find().w > g.Right() {
		t.Fatalf("Home auto %+v is outside Global %+v", find(), g)
	}
	s.world.HomeAuto = true
	if !find().selected {
		t.Fatal("button shows auto on")
	}
	click(t, a, homeAutoButtonID)
	if c := lastCmd(t, s); c.Kind != command.SetHomeAuto || c.Enabled {
		t.Fatalf("click with auto on emits %+v, want off", c)
	}
	s.world.HomeAuto = false
	click(t, a, homeAutoButtonID)
	if c := lastCmd(t, s); c.Kind != command.SetHomeAuto || !c.Enabled {
		t.Fatalf("click with auto off emits %+v, want on", c)
	}
}

func TestZoneToolsRenamedAndRoofMoved(t *testing.T) {
	t.Parallel()
	for id, w := range map[string]struct{ label, icon string }{
		"home":   {"Expand home area", "home"},
		"unhome": {"Clear home area", "home_clear"},
		"roof":   {"Build roof area", "roof_area"},
		"noroof": {"Remove roof area", "roof_remove"},
	} {
		tool, ok := toolByID(id)
		if !ok || tool.category != "zones" || tool.label != w.label || tool.icon != w.icon {
			t.Errorf("tool %q = %+v", id, tool)
		}
	}
}
