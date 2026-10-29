package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func TestPlanToolsEmitDesignatePlan(t *testing.T) {
	t.Parallel()
	for id, enabled := range map[string]bool{"plan": true, "unplan": false} {
		a, s := stubColony(t)
		click(t, a, "tab.architect")
		click(t, a, "arch.cat.orders")
		click(t, a, "arch.tool."+id)
		a.arch.Drag = true
		a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 2, 3, 5, 6
		a.arch.Submit()
		c := lastCmd(t, s)
		if c.Kind != command.DesignatePlan || c.Enabled != enabled ||
			c.X != 2 || c.Y != 3 || c.X2 != 5 || c.Y2 != 6 {
			t.Fatalf("%s cmd = %+v, want one DesignatePlan rectangle (Enabled=%v)", id, c, enabled)
		}
		if len(s.cmds) != 1 {
			t.Fatalf("%s sent %d commands, want 1", id, len(s.cmds))
		}
	}
}

func TestPlanToolCatalog(t *testing.T) {
	t.Parallel()
	for id, icon := range map[string]string{"plan": "plan", "unplan": "plan_clear"} {
		tool, ok := toolByID(id)
		if !ok || tool.category != "orders" || tool.mode != architect.PlaceArea || tool.icon != icon || tool.cmd != command.DesignatePlan {
			t.Fatalf("tool %q = %+v ok=%v", id, tool, ok)
		}
	}
	if tool, _ := toolByID("plan"); tool.label != "Plan" {
		t.Fatalf("label = %q", tool.label)
	}
	if tool, _ := toolByID("unplan"); tool.label != "Remove plan" {
		t.Fatalf("label = %q", tool.label)
	}
}

func TestPlanOverlayForcedWhileToolHeld(t *testing.T) {
	t.Parallel()
	a, _ := stubColony(t)
	a.renderer.overlays().Plan.On = false
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.orders")
	click(t, a, "arch.tool.plan")
	if !a.renderer.overlays().Plan.Forced || !a.renderer.overlays().Plan.Shown() {
		t.Fatal("Plan tool should force the overlay on")
	}
	click(t, a, "arch.tool.unplan")
	if !a.renderer.overlays().Plan.Forced {
		t.Fatal("Remove plan should force the overlay")
	}
	click(t, a, "arch.tool.unplan") // deselect
	if a.renderer.overlays().Plan.Forced || a.renderer.overlays().Plan.Shown() {
		t.Fatal("deselecting releases the forced overlay")
	}
	click(t, a, "arch.tool.mine")
	if a.renderer.overlays().Plan.Forced {
		t.Fatal("another tool must not force the Plan overlay")
	}
	if a.renderer.overlays().Home.Forced {
		t.Fatal("Plan tool must not force Home")
	}
}

func TestPlanButtonToggle(t *testing.T) {
	t.Parallel()
	a, _ := stubColony(t)
	sw, sh := a.sw, a.sh
	find := func() uikit.Button {
		b, ok := hasButton(a.colonyButtons(sw, sh), hud.PlanButtonID)
		if !ok {
			t.Fatal("hud.plan button missing")
		}
		return b
	}
	b := find()
	if !b.Selected {
		t.Fatal("overlay is on by default")
	}
	// left to right in Global: Roof, Home, Auto home, Plans, Rooms; none shares a rect
	for _, o := range []string{hud.HomeButtonID, hud.RoomsButtonID} {
		ob, _ := hasButton(a.colonyButtons(sw, sh), o)
		if i := (gfx.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}).Intersect(gfx.Rect{X: ob.X, Y: ob.Y, W: ob.W, H: ob.H}); i.W > 0 && i.H > 0 {
			t.Fatalf("Plans button %+v overlaps %s %+v", b, o, ob)
		}
	}
	if g := a.hud().Global; b.X < g.X || b.X+b.W > g.Right() || b.Y < g.Y || b.Y+b.H > g.Bottom() {
		t.Fatalf("Plans button %+v is outside Global %+v", b, g)
	}
	click(t, a, hud.PlanButtonID)
	if a.renderer.overlays().Plan.On || find().Selected {
		t.Fatal("click hides the overlay")
	}
	click(t, a, hud.PlanButtonID)
	if !a.renderer.overlays().Plan.On || !find().Selected {
		t.Fatal("second click shows it again")
	}
	var nilR *Renderer
	nilR.togglePlanOverlayIfAny() // menus: no renderer, no panic
}

func TestPlanLabelAt(t *testing.T) {
	t.Parallel()
	w := planWorld(1)
	if got := hud.PlanLabelAt(w, 2, 3); got != "Planned" {
		t.Fatalf("marked tile: %q", got)
	}
	for _, c := range [][2]int{{0, 0}, {-1, 3}, {10, 3}, {2, 50}} {
		if got := hud.PlanLabelAt(w, c[0], c[1]); got != "" {
			t.Fatalf("%v: %q", c, got)
		}
	}
	if hud.PlanLabelAt(game.WorldView{}, 0, 0) != "" {
		t.Fatal("no grid, no label")
	}
}
