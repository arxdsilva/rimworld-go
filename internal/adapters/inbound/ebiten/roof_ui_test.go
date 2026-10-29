package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	overlaypkg "rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strings"
	"testing"

	"rimworld-go/internal/domain/sim/command"
)

func roofStubApp(t *testing.T) (*App, *stubSim) {
	t.Helper()
	s := &stubSim{w: 16, h: 16}
	a := testApp(s)
	a.finishBoot()
	a.startGame()
	if a.renderer == nil {
		t.Fatal("no renderer")
	}
	return a, s
}

func TestArchitectHasRoofTools(t *testing.T) {
	t.Parallel()
	want := map[string]struct {
		label, desc, icon string
		cmd               command.Kind
	}{
		"roof":   {"Build roof area", "Colonists roof these tiles and rebuild the roof if it falls. A roof needs a wall within 6 tiles; closed rooms are roofed automatically.", "roof_area", command.DesignateRoof},
		"noroof": {"Remove roof area", "Take roofs down and keep them off, even in closed rooms.", "roof_remove", command.DesignateRemoveRoof},
	}
	for id, w := range want {
		tool, ok := toolByID(id)
		if !ok {
			t.Fatalf("tool %q missing", id)
		}
		if tool.category != "zones" || tool.mode != architect.PlaceArea || tool.label != w.label ||
			tool.desc != w.desc || tool.icon != w.icon || tool.cmd != w.cmd {
			t.Fatalf("tool %q = %+v", id, tool)
		}
	}
}

func roofToolEmits(t *testing.T, id string, kind command.Kind) {
	t.Helper()
	a, s := roofStubApp(t)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.zones")
	click(t, a, "arch.tool."+id)
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 2, 3, 6, 7
	a.arch.Submit()
	if len(s.cmds) != 1 {
		t.Fatalf("cmds = %+v", s.cmds)
	}
	c := s.cmds[0]
	if c.Kind != kind || c.X != 2 || c.Y != 3 || c.X2 != 6 || c.Y2 != 7 {
		t.Fatalf("cmd = %+v", c)
	}
}

func TestRoofToolEmitsDesignateRoof(t *testing.T) {
	t.Parallel()
	roofToolEmits(t, "roof", command.DesignateRoof)
}

func TestRemoveRoofToolEmits(t *testing.T) {
	t.Parallel()
	roofToolEmits(t, "noroof", command.DesignateRemoveRoof)
}

func TestRoofGhostMarksUnsupported(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		flags uint8
		wall  bool
		want  bool
	}{
		{"unsupported open tile", 0, false, true},
		{"supported tile", overlaypkg.RoofSupported, false, false},
		{"supported and planned", overlaypkg.RoofSupported | overlaypkg.RoofPlanBuild, false, false},
		{"wall is skipped", 0, true, false},
		{"already roofed but unsupported", overlaypkg.RoofBuilt, false, true},
	}
	for _, c := range cases {
		if got := roofGhostUnsupported(c.flags, c.wall); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestRoofToolForcesOverlay(t *testing.T) {
	t.Parallel()
	a, _ := roofStubApp(t)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.zones")
	click(t, a, "arch.tool.roof")
	if !a.renderer.overlays().Roof.Forced || !a.renderer.overlays().Roof.Shown() {
		t.Fatal("roof tool should force the overlay")
	}
	click(t, a, "arch.tool.noroof")
	if !a.renderer.overlays().Roof.Forced {
		t.Fatal("remove-roof tool should force the overlay")
	}
	click(t, a, "arch.tool.noroof") // deselect
	if a.renderer.overlays().Roof.Forced || a.renderer.overlays().Roof.Shown() {
		t.Fatal("deselect should clear the forced overlay")
	}
	click(t, a, "arch.tool.roof")
	click(t, a, "arch.tool.stockpile") // another zone tool
	if a.renderer.overlays().Roof.Forced {
		t.Fatal("another tool should not force the overlay")
	}
	click(t, a, "arch.tool.roof")
	click(t, a, "tab.architect") // close Architect
	a.syncRoofForce()
	if a.renderer.overlays().Roof.Forced {
		t.Fatal("closing the Architect should clear the forced overlay")
	}
}

func TestHUDRoofButtonToggles(t *testing.T) {
	t.Parallel()
	a, _ := roofStubApp(t)
	sw, sh := a.sw, a.sh
	find := func() uikit.Button {
		for _, b := range a.colonyButtons(sw, sh) {
			if b.ID == hud.RoofButtonID {
				return b
			}
		}
		t.Fatal("hud.roof button missing")
		return uikit.Button{}
	}
	if find().Selected {
		t.Fatal("button active while overlay off")
	}
	click(t, a, hud.RoofButtonID)
	if !a.renderer.overlays().Roof.On || !find().Selected {
		t.Fatal("click should turn the overlay on and show active")
	}
	click(t, a, hud.RoofButtonID)
	if a.renderer.overlays().Roof.On || find().Selected {
		t.Fatal("second click should turn it off")
	}
	b := find()
	if g := a.hud().Global; b.X < g.X || b.X+b.W > g.Right() || b.Y < g.Y || b.Y+b.H > g.Bottom() {
		t.Fatalf("button %+v should sit inside Global %+v", b, g)
	}
	if !a.colonyChromeHit(int(b.X)+1, int(b.Y)+1, sw, sh) {
		t.Fatal("button should consume the pointer")
	}
}

// UI-016 moved the roof overlay from R (now Draft) to Shift+R; the key
// itself is covered by TestRDraftsAndShiftRTogglesRoof.
func TestRKeyToggles(t *testing.T) {
	t.Parallel()
	a, _ := roofStubApp(t)
	a.renderer.overlays().Roof.Toggle() // what Shift+R calls
	if !a.renderer.overlays().Roof.On {
		t.Fatal("toggle should turn the overlay on")
	}
	a.renderer.overlays().Roof.Toggle()
	if a.renderer.overlays().Roof.On {
		t.Fatal("toggle should turn the overlay off")
	}
}

func TestLegendShownWithOverlay(t *testing.T) {
	t.Parallel()
	a, _ := roofStubApp(t)
	if got := a.chrome.Global().RoofLegend(); got != "" {
		t.Fatalf("legend = %q with overlay off", got)
	}
	a.renderer.overlays().Roof.On = true
	want := "Roof: blue built · slate natural · dashed planned · hatched removal"
	if got := a.chrome.Global().RoofLegend(); got != want {
		t.Fatalf("legend = %q", got)
	}
	a.renderer.overlays().Roof.On = false
	a.renderer.overlays().Roof.Forced = true
	if !strings.HasPrefix(a.chrome.Global().RoofLegend(), "Roof:") {
		t.Fatal("legend should show when forced")
	}
}
