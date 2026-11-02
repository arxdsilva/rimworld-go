package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// journey_arch_test.go: the Architect's vanilla categories and placeholder
// cards (STK-AG, .cursor/ARCHITECT_GAP_ANALYSIS.md).
var _ = owe("journey_arch_test.go",
	[]string{},
	[]string{})

// archHit is the hit target id on the colony HUD this frame.
func (j *journey) archHit(id string) (uikit.Button, bool) {
	j.t.Helper()
	j.update(1)
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		if b.ID == id {
			return b, true
		}
	}
	return uikit.Button{}, false
}

// TestJourneyArchitectVanillaCategoriesAndPlaceholders: Tab opens the
// Architect; each of the twelve vanilla categories opens from its button
// and shows every card it holds in the grid. Resting the pointer on a
// placeholder shows "Not in this game yet" and its sentence; clicking it
// arms nothing, and clicking the map afterwards places nothing.
func TestJourneyArchitectVanillaCategoriesAndPlaceholders(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	if len(architect.Categories) != 12 {
		t.Fatalf("%d categories, want the 12 vanilla ones", len(architect.Categories))
	}
	tried := 0
	for _, c := range architect.Categories {
		j.openArch(c.ID, "")
		if j.a.arch.Cat != c.ID {
			t.Fatalf("category %s did not open (open: %q)", c.ID, j.a.arch.Cat)
		}
		var first architect.ToolDef
		for _, d := range architect.ToolsIn(c.ID) {
			if _, ok := j.archHit("arch.tool." + d.ID); !ok {
				t.Fatalf("%s: card %s is not on the grid", c.ID, d.ID)
			}
			if d.Planned && first.ID == "" {
				first = d
			}
		}
		if first.ID == "" {
			continue // Temperature: every vanilla item works
		}
		tried++
		id := "arch.tool." + first.ID
		card, _ := j.archHit(id)
		if !card.Disabled || card.Reason != architect.PlannedReason {
			t.Fatalf("%s: placeholder %s is not greyed: %+v", c.ID, first.ID, card)
		}

		// rest the pointer on the card: its tooltip shows
		s := float32(j.a.scale.S)
		j.f.Cur.Mx, j.f.Cur.My = int((card.X+card.W/2)*s), int((card.Y+card.H/2)*s)
		j.update(uikit.TipDelay + 2)
		k := j.a.uiSt()
		w, ok := k.Find(k.Hot)
		if !k.TipVisible() || !ok || !strings.Contains(w.Tip, architect.PlannedReason) || !strings.Contains(w.Tip, first.Desc) ||
			!strings.HasPrefix(w.Tip, first.Label) {
			t.Fatalf("%s: tooltip over %s (hot %q, visible %v) = %q", c.ID, first.ID, k.Hot, k.TipVisible(), w.Tip)
		}

		// click it with the mouse anyway, then click the map
		j.reset()
		j.f.clickAt(t, j.a, int((card.X+card.W/2)*s), int((card.Y+card.H/2)*s))
		j.f.Cur.Mx, j.f.Cur.My = -50, -50
		if j.a.arch.ToolID != "" {
			t.Fatalf("%s: clicking %s armed %q", c.ID, first.ID, j.a.arch.ToolID)
		}
		frames := len(j.world().Frames)
		j.clickTile(12, 6, inputframe.Left, 0)
		j.dragTiles(inputframe.Left, 12, 6, 14, 8, 0)
		if len(j.cmds) != 0 || len(j.world().Frames) != frames {
			t.Fatalf("%s: a placeholder sent %+v", c.ID, j.cmds)
		}
		if !j.a.arch.Open || j.a.arch.Cat != c.ID {
			t.Fatalf("%s: the Architect left the category (open %v, %q)", c.ID, j.a.arch.Open, j.a.arch.Cat)
		}
	}
	if tried != 11 {
		t.Fatalf("placeholders tried in %d categories, want 11", tried)
	}
}

// TestJourneyArchitectMovedTools: tools sit in their vanilla category
// now: the heater under Temperature, the horseshoe pin under Recreation,
// the wood floor under Floors, and each still arms from there.
func TestJourneyArchitectMovedTools(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	for _, c := range []struct{ cat, tool string }{
		{architect.CatTemperature, "heater"}, {architect.CatTemperature, "campfire"},
		{architect.CatRecreation, "horseshoes"}, {architect.CatFloors, "floor"},
		{architect.CatFloors, "smooth"}, {"misc", "grave"}, {"production", "hydroponics"}, {"furniture", "sun_lamp"},
	} {
		j.openArch(c.cat, c.tool)
		j.key("Escape", 0) // drop the tool, keep the panel
	}
}
