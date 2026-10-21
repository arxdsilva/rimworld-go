package ebiten

import (
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
		"roof":   {"Build roof", "Colonists roof these tiles. A roof needs a wall within 6 tiles; closed rooms are roofed automatically.", "roof_build", command.DesignateRoof},
		"noroof": {"Remove roof", "Take roofs down and keep them off, even in closed rooms.", "roof_remove", command.DesignateRemoveRoof},
	}
	for id, w := range want {
		tool, ok := toolByID(id)
		if !ok {
			t.Fatalf("tool %q missing", id)
		}
		if tool.category != "structure" || tool.mode != placeArea || tool.label != w.label ||
			tool.desc != w.desc || tool.icon != w.icon || tool.cmd != w.cmd {
			t.Fatalf("tool %q = %+v", id, tool)
		}
	}
}

func roofToolEmits(t *testing.T, id string, kind command.Kind) {
	t.Helper()
	a, s := roofStubApp(t)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.structure")
	click(t, a, "arch.tool."+id)
	a.archDrag = true
	a.archX0, a.archY0, a.archX1, a.archY1 = 2, 3, 6, 7
	a.submitArchitect()
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
		{"supported tile", roofSupported, false, false},
		{"supported and planned", roofSupported | roofPlanBuild, false, false},
		{"wall is skipped", 0, true, false},
		{"already roofed but unsupported", roofBuilt, false, true},
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
	click(t, a, "arch.cat.structure")
	click(t, a, "arch.tool.roof")
	if !a.renderer.roofOverlayForced || !a.renderer.roofOverlayOn() {
		t.Fatal("roof tool should force the overlay")
	}
	click(t, a, "arch.tool.noroof")
	if !a.renderer.roofOverlayForced {
		t.Fatal("remove-roof tool should force the overlay")
	}
	click(t, a, "arch.tool.noroof") // deselect
	if a.renderer.roofOverlayForced || a.renderer.roofOverlayOn() {
		t.Fatal("deselect should clear the forced overlay")
	}
	click(t, a, "arch.tool.roof")
	click(t, a, "arch.tool.wall")
	if a.renderer.roofOverlayForced {
		t.Fatal("another tool should not force the overlay")
	}
	click(t, a, "arch.tool.roof")
	click(t, a, "tab.architect") // close Architect
	a.syncRoofForce()
	if a.renderer.roofOverlayForced {
		t.Fatal("closing the Architect should clear the forced overlay")
	}
}

func TestHUDRoofButtonToggles(t *testing.T) {
	t.Parallel()
	a, _ := roofStubApp(t)
	sw, sh := a.sw, a.sh
	find := func() button {
		for _, b := range a.colonyButtons(sw, sh) {
			if b.id == roofButtonID {
				return b
			}
		}
		t.Fatal("hud.roof button missing")
		return button{}
	}
	if find().selected {
		t.Fatal("button active while overlay off")
	}
	click(t, a, roofButtonID)
	if !a.renderer.roofOverlay || !find().selected {
		t.Fatal("click should turn the overlay on and show active")
	}
	click(t, a, roofButtonID)
	if a.renderer.roofOverlay || find().selected {
		t.Fatal("second click should turn it off")
	}
	b := find()
	if my := int(b.y) + 1; my >= hudTopH || !a.colonyChromeHit(int(b.x)+1, my, sw, sh) {
		t.Fatal("button should sit in the top strip and consume the pointer")
	}
}

func TestRKeyToggles(t *testing.T) {
	t.Parallel()
	a, _ := roofStubApp(t)
	a.renderer.toggleRoofOverlay() // what the R key calls
	if !a.renderer.roofOverlay {
		t.Fatal("toggle should turn the overlay on")
	}
	a.renderer.toggleRoofOverlay()
	if a.renderer.roofOverlay {
		t.Fatal("toggle should turn the overlay off")
	}
}

func TestLegendShownWithOverlay(t *testing.T) {
	t.Parallel()
	a, _ := roofStubApp(t)
	if got := a.roofLegend(); got != "" {
		t.Fatalf("legend = %q with overlay off", got)
	}
	a.renderer.roofOverlay = true
	want := "Roof: blue built · slate natural · dashed planned · hatched removal"
	if got := a.roofLegend(); got != want {
		t.Fatalf("legend = %q", got)
	}
	a.renderer.roofOverlay = false
	a.renderer.roofOverlayForced = true
	if !strings.HasPrefix(a.roofLegend(), "Roof:") {
		t.Fatal("legend should show when forced")
	}
}
