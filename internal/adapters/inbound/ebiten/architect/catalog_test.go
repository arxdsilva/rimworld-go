package architect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/pkg/mathx"
)

// TestArchCategoriesHaveTools: every category in Categories gets a
// button, so each must hold cards, and every vanilla category is listed
// (STK-AG: all twelve, in the vanilla order) with Storage (NST) after Furniture.
func TestArchCategoriesHaveTools(t *testing.T) {
	t.Parallel()
	var ids []string
	for _, c := range Categories {
		ids = append(ids, c.ID)
		if len(ToolsIn(c.ID)) == 0 {
			t.Errorf("category %s has no tools", c.ID)
		}
	}
	want := []string{"orders", "zones", "structure", "production", "furniture", CatStorage, CatPower, "security", "misc",
		CatFloors, CatRecreation, CatShip, CatTemperature}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Errorf("categories %v, want %v", ids, want)
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
	cats := map[string]bool{}
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
		if d.Planned != strings.HasPrefix(d.ID, PlannedPrefix) {
			t.Errorf("%s: placeholder %v, id prefix disagrees", d.ID, d.Planned)
		}
		if d.Planned {
			if d.Build != "" || d.Cmd != 0 || d.Crop != "" || d.Variant != 0 || d.Icon != PlannedIcon || d.Desc == "" {
				t.Errorf("placeholder %s must carry no build or command, the planned icon and a sentence: %+v", d.ID, d)
			}
			continue
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
	if mathx.Abs(-4) != 4 || mathx.Abs(3) != 3 {
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
		{world.BuildSpriteStuff("floor", "granite blocks", "", 0, false, false), "structure/floor_stone_tile_granite"},
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
	if got := CropLines(tree, units.Celsius); len(got) != 2 || !strings.HasPrefix(got[0], "Tree: 10.0 days") {
		t.Errorf("tree lines %q", got)
	}
	if got := CropLines(game.CropView{Name: "rice", Label: "rice", Food: true, FrostKill: -25}, units.Celsius); !strings.Contains(got[2], "below -3C") {
		t.Errorf("crop lines %q", got)
	}
}

// TestPlannedCardsAreGreyAndInert: a placeholder card is disabled with
// "Not in this game yet" and its sentence; selecting it arms nothing and
// says why, and a forced Submit on it sends no command.
func TestPlannedCardsAreGreyAndInert(t *testing.T) {
	t.Parallel()
	tl, f := newFake(&fakeSim{})
	tl.Open = true
	n := 0
	for _, c := range Categories {
		tl.Cat = c.ID
		sawPlanned := false
		for _, card := range tl.Cards() {
			d, _ := ToolByID(strings.TrimPrefix(card.ID, "arch.tool."))
			if !d.Planned {
				if sawPlanned {
					t.Errorf("%s: working card %s after a placeholder", c.ID, card.ID)
				}
				continue
			}
			sawPlanned = true
			n++
			if !card.Disabled || card.Reason != PlannedReason || card.Desc != d.Desc || card.Toggle || card.On || card.Icon != PlannedIcon {
				t.Errorf("placeholder card %+v", card)
			}
			tl.SelectTool(d.ID)
			if tl.ToolID != "" || f.hint != PlannedReason {
				t.Fatalf("%s armed %q (hint %q)", d.ID, tl.ToolID, f.hint)
			}
			tl.ToolID, tl.Drag = d.ID, true // forced, as no input path can
			tl.X0, tl.Y0, tl.X1, tl.Y1 = 0, 0, 3, 3
			tl.Submit()
			tl.ToolID, tl.Drag = "", false
			if len(f.cmds) != 0 {
				t.Fatalf("%s sent %+v", d.ID, f.cmds)
			}
		}
	}
	if n != len(Planned) {
		t.Fatalf("%d placeholder cards shown, %d planned", n, len(Planned))
	}
	if tl.Preset("", "", ""); strings.HasPrefix(tl.ToolID, PlannedPrefix) {
		t.Fatalf("Preset armed placeholder %s", tl.ToolID)
	}
}

// TestEveryGapRowHasACard: .cursor/ARCHITECT_GAP_ANALYSIS.md lists every
// core vanilla Architect item with the card that stands for it. Each
// row's card exists in that category, a missing row's card is a
// placeholder and a done or partial row's card works, every placeholder
// has a row, and the counts table agrees.
func TestEveryGapRowHasACard(t *testing.T) {
	t.Parallel()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for ; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		if filepath.Dir(dir) == dir {
			t.Fatal("no go.mod above the package")
		}
	}
	doc, err := os.ReadFile(filepath.Join(dir, ".cursor", "ARCHITECT_GAP_ANALYSIS.md"))
	if err != nil {
		t.Fatal(err)
	}
	rows, missing, cats := 0, 0, map[string]bool{}
	covered := map[string]bool{}
	tick := func(s string) (string, bool) {
		s = strings.TrimSpace(s)
		if len(s) < 3 || s[0] != '`' || s[len(s)-1] != '`' {
			return "", false
		}
		return s[1 : len(s)-1], true
	}
	for _, ln := range strings.Split(string(doc), "\n") {
		cols := strings.Split(ln, "|")
		if len(cols) < 6 {
			continue
		}
		cat, ok1 := tick(cols[1])
		card, ok2 := tick(cols[3])
		if !ok1 || !ok2 {
			continue
		}
		item, status := strings.TrimSpace(cols[2]), strings.TrimSpace(cols[4])
		rows++
		cats[cat] = true
		d, ok := ToolByID(card)
		switch {
		case !ok:
			t.Errorf("%s (%s): no card %q", item, cat, card)
			continue
		case d.Category != cat:
			t.Errorf("%s: card %s sits in %q, the doc says %q", item, card, d.Category, cat)
		}
		switch status {
		case "missing":
			missing++
			if !d.Planned {
				t.Errorf("%s is missing but %s is a working card", item, card)
			}
		case "done", "partial":
			if d.Planned {
				t.Errorf("%s is %s but %s is a placeholder", item, status, card)
			}
		default:
			t.Errorf("%s: status %q", item, status)
		}
		covered[card] = true
	}
	for _, d := range Planned {
		if !covered[d.ID] {
			t.Errorf("placeholder %s has no row in the gap analysis", d.ID)
		}
	}
	for _, c := range Categories {
		if !cats[c.ID] {
			t.Errorf("category %s has no rows", c.ID)
		}
	}
	for _, want := range []string{
		fmt.Sprintf("| Vanilla core items (rows below) | %d |", rows),
		fmt.Sprintf("| Done or partial (a working card) | %d |", rows-missing),
		fmt.Sprintf("| Missing (a placeholder card) | %d rows on %d placeholder cards", missing, len(Planned)),
	} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("counts table lacks %q", want)
		}
	}
}
