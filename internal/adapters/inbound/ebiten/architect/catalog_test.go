package architect

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// TestArchCategoriesHaveTools: every category in Categories gets a
// button, so each must hold tools; the PLAY-021 Power and Temperature
// categories join the list with their first tools.
func TestArchCategoriesHaveTools(t *testing.T) {
	t.Parallel()
	listed := map[string]bool{}
	for _, c := range Categories {
		listed[c.ID] = true
		if len(ToolsIn(c.ID)) == 0 {
			t.Errorf("category %s has no tools", c.ID)
		}
	}
	for _, id := range []string{CatPower, CatTemperature} {
		if len(ToolsIn(id)) > 0 && !listed[id] {
			t.Errorf("%s has tools but no category button", id)
		}
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
		tool, ok := ToolByID(id)
		if !ok || tool.Category != "zones" || tool.Label != w.label || tool.Icon != w.icon {
			t.Errorf("tool %q = %+v", id, tool)
		}
	}
}

func TestSnowClearToolsInZones(t *testing.T) {
	t.Parallel()
	found := 0
	for _, tl := range Tools {
		if !SnowClearToolID(tl.ID) {
			continue
		}
		found++
		if tl.Category != "zones" || tl.Cmd != command.DesignateSnowClear || tl.Mode != PlaceArea || tl.Icon != "snow_clear" {
			t.Fatalf("tool %+v", tl)
		}
		if (tl.ID == "snow_clear") != tl.Enable {
			t.Fatalf("%s paints %v", tl.ID, tl.Enable)
		}
	}
	if found != 2 {
		t.Fatalf("%d snow clearing tools, want 2", found)
	}
}

// TestToolCatalogIsConsistent: ids are unique, every tool sits in a known
// category, a build tool names a buildable with a kind id, an area tool
// names a command.
func TestToolCatalogIsConsistent(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	cats := map[string]bool{CatTemperature: true}
	for _, c := range Categories {
		cats[c.ID] = true
	}
	for _, d := range AllTools() {
		if seen[d.ID] {
			t.Errorf("tool id %s twice", d.ID)
		}
		seen[d.ID] = true
		if !cats[d.Category] {
			t.Errorf("%s in unknown category %q", d.ID, d.Category)
		}
		if d.Mode == PlaceArea {
			if d.Cmd == 0 {
				t.Errorf("area tool %s sends no command", d.ID)
			}
			continue
		}
		if _, ok := KindID(d.Build); !ok {
			t.Errorf("build tool %s: %q is not buildable", d.ID, d.Build)
		}
	}
	if _, ok := ToolByID("no such tool"); ok {
		t.Error("an unknown id finds a tool")
	}
}

func TestPlacementCells(t *testing.T) {
	t.Parallel()
	if got := PlacementCells(PlaceSingle, 0, 0, 3, 4); len(got) != 1 || got[0] != [2]int{3, 4} {
		t.Errorf("single = %v", got)
	}
	if got := PlacementCells(PlaceLine, 5, 2, 1, 3); len(got) != 5 || got[0] != [2]int{1, 2} || got[4] != [2]int{5, 2} {
		t.Errorf("line along x = %v", got)
	}
	if got := PlacementCells(PlaceLine, 2, 0, 3, 3); len(got) != 4 || got[3] != [2]int{2, 3} {
		t.Errorf("line along y = %v", got)
	}
	if got := PlacementCells(PlaceFill, 2, 2, 0, 0); len(got) != 9 || got[0] != [2]int{0, 0} {
		t.Errorf("fill = %v", got)
	}
}

func TestFacings(t *testing.T) {
	t.Parallel()
	if RotateFacing(0, -1) != 3 || RotateFacing(3, 1) != 0 || RotateFacing(1, 2) != 3 {
		t.Error("rotation wraps")
	}
	if FacingIndex("west") != 3 || FacingIndex("bogus") != 0 || world.FacingOr("") != "south" || world.FacingOr("east") != "east" {
		t.Error("facing names")
	}
	if Abs(-4) != 4 || Abs(3) != 3 {
		t.Error("Abs")
	}
}

// TestBuildSpriteKeys pins the build keys the Architect's previews draw
// (render/world owns them).
func TestBuildSpriteKeys(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ got, want string }{
		{world.BuildSprite("wall", "", 5, false, false), "structure/wall_wood_05"},
		{world.BuildSprite("door", "", 0, true, true), "structure/door_wood_ew_open"},
		{world.BuildSprite("bed", "", 0, false, false), "furniture/bed_basic_south"},
		{world.BuildSprite("grave", "west", 0, false, false), "furniture/grave_dug_east"},
		{world.BuildSprite("fence gate", "east", 0, false, true), "structure/fence_gate_open_east"},
		{world.BuildSprite("power switch", "", 0, false, true), "power/power_switch_off"},
		{world.BuildSprite("battery", "north", 0, false, false), "power/battery_north"},
		{world.BuildSpriteStuff("door", "steel", "", 0, false, false), "structure/door_steel_ns_closed"},
		{world.BuildSpriteStuff("floor", "granite blocks", "", 0, false, false), "structure/floor_stone"},
		{world.BuildSpriteStuff("carpet", "cloth carpet", "", 0, false, false), "furniture/carpet_cloth"},
		{world.BuildSpriteStuff("carpet", "silk", "", 0, false, false), "furniture/carpet_straw"},
		{world.BuildSprite("no such kind", "", 0, false, false), ""},
	} {
		if c.got != c.want {
			t.Errorf("key %q, want %q", c.got, c.want)
		}
	}
}

func TestCardLines(t *testing.T) {
	t.Parallel()
	wall, _ := ToolByID("wall")
	lines := TooltipLines(wall, "wood", 0, 0)
	if lines[0] != "Wall" || !strings.HasPrefix(lines[2], "Cost: ") || !strings.HasPrefix(lines[len(lines)-1], "Only 0 wood") {
		t.Errorf("wall card %q", lines)
	}
	carpet, _ := ToolByID("carpet")
	if lines := TooltipLines(carpet, "straw carpet", 0, 0); !strings.Contains(strings.Join(lines, "\n"), "In stock: 0 straw carpet") {
		t.Errorf("carpet card %q", lines)
	}
	if got := LockWord("Needs research: Stonecutting"); got != "needs research" || LockWord("scenario") != "locked" {
		t.Errorf("lock words %q", got)
	}
	w := game.WorldView{Stone: 7, Items: []game.ItemView{{Kind: "steel", Qty: 5}, {Kind: "steel", Qty: 9, Carried: true}}}
	if got := StockLine(w, 3, ""); got != "Wood 3   Stone 7" {
		t.Errorf("stock line %q", got)
	}
	if got := StockLine(w, 3, "steel"); got != "Wood 3   Steel 5" {
		t.Errorf("steel stock line %q", got)
	}
	tree := game.CropView{Name: "tree", Days: 10, Yield: 25}
	if got := CropLines(tree); len(got) != 2 || !strings.HasPrefix(got[0], "Tree: 10.0 days") {
		t.Errorf("tree lines %q", got)
	}
	if got := CropLines(game.CropView{Name: "rice", Label: "rice", Food: true, FrostKill: -25}); !strings.Contains(got[2], "below -3C") {
		t.Errorf("crop lines %q", got)
	}
}
