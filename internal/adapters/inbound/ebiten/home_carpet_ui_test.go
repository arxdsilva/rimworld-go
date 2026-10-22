package ebiten

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

func stubColony(t *testing.T) (*App, *stubSim) {
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

func lastCmd(t *testing.T, s *stubSim) command.Command {
	t.Helper()
	if len(s.cmds) == 0 {
		t.Fatal("no command enqueued")
	}
	return s.cmds[len(s.cmds)-1]
}

func hasButton(bs []button, id string) (button, bool) {
	for _, b := range bs {
		if b.id == id {
			return b, true
		}
	}
	return button{}, false
}

// Work column 7 (Cln) is live, shows the pawn's priority and emits WorkClean.
func TestWorkColumnCleanLive(t *testing.T) {
	t.Parallel()
	if workColumns[7] != "Cln" {
		t.Fatalf("column 7 = %q", workColumns[7])
	}
	wt, ok := workTypeForCol(7)
	if !ok || wt != command.WorkClean {
		t.Fatalf("col 7 = %v/%v, want WorkClean", wt, ok)
	}
	if got := workCellValue(game.PawnView{WorkClean: true}, 7); got != "3" {
		t.Fatalf("cell on = %q", got)
	}
	if got := workCellValue(game.PawnView{}, 7); got != "-" {
		t.Fatalf("cell off = %q", got)
	}
	if got := workPriority(game.PawnView{Priorities: []int{0, 0, 0, 0, 0, 0, 0, 2}}, 7); got != 2 {
		t.Fatalf("priority = %d", got)
	}
}

func TestWorkCleanCellEmitsWorkClean(t *testing.T) {
	t.Parallel()
	a2, s := stubColony(t)
	s.pawns = []game.PawnView{{ID: 7, Name: "Ada", WorkClean: true, Priorities: []int{0, 0, 0, 0, 0, 0, 0, 3}}}
	click(t, a2, "tab.work")
	click(t, a2, "work.cell.7.7")
	c := lastCmd(t, s)
	if c.Kind != command.SetWorkPriority || c.Work != command.WorkClean || c.Pawn != 7 || c.Count != 4 {
		t.Fatalf("cmd = %+v", c)
	}
}

func TestHomeToolsEmitDesignateHome(t *testing.T) {
	t.Parallel()
	for id, enabled := range map[string]bool{"home": true, "unhome": false} {
		a, s := stubColony(t)
		click(t, a, "tab.architect")
		click(t, a, "arch.cat.zones")
		click(t, a, "arch.tool."+id)
		a.archDrag = true
		a.archX0, a.archY0, a.archX1, a.archY1 = 2, 3, 5, 6
		a.submitArchitect()
		c := lastCmd(t, s)
		if c.Kind != command.DesignateHome || c.Enabled != enabled ||
			c.X != 2 || c.Y != 3 || c.X2 != 5 || c.Y2 != 6 {
			t.Fatalf("%s cmd = %+v, want one DesignateHome rectangle (Enabled=%v)", id, c, enabled)
		}
		if len(s.cmds) != 1 {
			t.Fatalf("%s sent %d commands, want 1", id, len(s.cmds))
		}
	}
}

func TestHomeToolCatalog(t *testing.T) {
	t.Parallel()
	for id, icon := range map[string]string{"home": "home", "unhome": "home_clear"} {
		tool, ok := toolByID(id)
		if !ok || tool.category != "zones" || tool.mode != placeArea || tool.icon != icon || tool.cmd != command.DesignateHome {
			t.Fatalf("tool %q = %+v ok=%v", id, tool, ok)
		}
	}
	if tool, _ := toolByID("home"); tool.label != "Home" {
		t.Fatalf("label = %q", tool.label)
	}
	if tool, _ := toolByID("unhome"); tool.label != "Clear home" {
		t.Fatalf("label = %q", tool.label)
	}
}

func TestHomeOverlayForcedWhileToolHeld(t *testing.T) {
	t.Parallel()
	a, _ := stubColony(t)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.zones")
	click(t, a, "arch.tool.home")
	if !a.renderer.homeOverlayForced {
		t.Fatal("Home tool should force the overlay")
	}
	click(t, a, "arch.tool.unhome")
	if !a.renderer.homeOverlayForced {
		t.Fatal("Clear home tool should force the overlay")
	}
	click(t, a, "arch.tool.unhome") // deselect
	if a.renderer.homeOverlayForced || a.renderer.homeShown() {
		t.Fatal("deselecting should clear the forced overlay")
	}
	click(t, a, "arch.tool.stockpile")
	if a.renderer.homeOverlayForced {
		t.Fatal("another tool must not force the Home overlay")
	}
	click(t, a, "arch.tool.home")
	click(t, a, "tab.architect") // close
	a.syncRoofForce()
	if a.renderer.homeOverlayForced {
		t.Fatal("closing the Architect should clear the forced overlay")
	}
	if a.renderer.roofOverlayForced {
		t.Fatal("Home tool must not force the Roof overlay")
	}
}

func TestHomeButtonToggle(t *testing.T) {
	t.Parallel()
	a, _ := stubColony(t)
	sw, sh := a.sw, a.sh
	find := func() button {
		b, ok := hasButton(a.colonyButtons(sw, sh), homeButtonID)
		if !ok {
			t.Fatal("hud.home button missing")
		}
		return b
	}
	b := find()
	if b.selected {
		t.Fatal("active while overlay off")
	}
	if rb, _ := hasButton(a.colonyButtons(sw, sh), roofButtonID); b.x+b.w > rb.x {
		t.Fatalf("Home button %+v overlaps Roof %+v", b, rb)
	}
	if my := int(b.y) + 1; my >= hudTopH || !a.colonyChromeHit(int(b.x)+1, my, sw, sh) {
		t.Fatal("button should sit in the top strip and consume the pointer")
	}
	a.renderer.toggleHomeOverlayIfAny()
	if !a.renderer.homeOverlay || !find().selected {
		t.Fatal("toggle should turn the overlay on and show active")
	}
	a.renderer.toggleHomeOverlayIfAny()
	if a.renderer.homeOverlay {
		t.Fatal("second toggle should turn it off")
	}
	var nilR *Renderer
	nilR.toggleHomeOverlayIfAny() // menus: no renderer, no panic
}

func TestCarpetToolAndMaterialButtons(t *testing.T) {
	t.Parallel()
	a, _ := stubColony(t)
	tool, ok := toolByID("carpet")
	if !ok || tool.category != "furniture" || tool.mode != placeSingle || tool.build != "carpet" || tool.icon != "carpet" {
		t.Fatalf("carpet tool = %+v ok=%v", tool, ok)
	}
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.furniture")
	click(t, a, "arch.tool.carpet")
	var stuffs []button
	for _, b := range a.architectButtons() {
		if strings.HasPrefix(b.id, "arch.stuff.") {
			stuffs = append(stuffs, b)
		}
	}
	// wool carpet joined with animal products (SIM-026)
	want := []string{"straw carpet", "wood carpet", "leather carpet", "cloth carpet", "wool carpet"}
	bv, _ := buildable("carpet")
	if len(bv.Stuffs) != len(want) {
		t.Fatalf("carpet stuffs = %v", bv.Stuffs)
	}
	if len(stuffs) != len(want) {
		t.Fatalf("%d material buttons, want %d", len(stuffs), len(want))
	}
	for _, w := range want {
		if _, ok := hasButton(stuffs, "arch.stuff."+w); !ok {
			t.Fatalf("no button for %q in %v", w, stuffs)
		}
	}
	if !stuffs[0].selected || a.toolStuff(tool) != bv.Stuffs[0] {
		t.Fatalf("default material should be selected first: %q", a.toolStuff(tool))
	}
	if stuffs[0].x+stuffs[0].w > stuffs[1].x || stuffs[len(stuffs)-1].x+stuffs[len(stuffs)-1].w > archW+hudPad {
		t.Fatalf("buttons overlap or overflow the panel: %+v", stuffs)
	}
	click(t, a, "arch.stuff.cloth carpet")
	if got := a.toolStuff(tool); got != "cloth carpet" {
		t.Fatalf("toolStuff = %q", got)
	}
}

func TestWallMaterialButtonsStillWoodStone(t *testing.T) {
	t.Parallel()
	a, _ := stubColony(t)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.structure")
	click(t, a, "arch.tool.wall")
	_, w := hasButton(a.architectButtons(), "arch.stuff.wood")
	_, s := hasButton(a.architectButtons(), "arch.stuff.stone")
	if !w || !s {
		t.Fatal("wall should offer Wood and Stone")
	}
	click(t, a, "arch.stuff.stone")
	click(t, a, "arch.cat.furniture")
	click(t, a, "arch.tool.carpet")
	tool, _ := toolByID("carpet")
	if got := a.toolStuff(tool); got != "straw carpet" {
		t.Fatalf("stone pick leaked into carpet: %q", got)
	}
	click(t, a, "arch.tool.carpet") // off
	click(t, a, "arch.cat.structure")
	click(t, a, "arch.tool.door")
	for _, b := range a.architectButtons() {
		if strings.HasPrefix(b.id, "arch.stuff.") {
			t.Fatalf("door should have no material buttons, got %q", b.id)
		}
	}
}

func TestCarpetTooltipUsesSelectedMaterial(t *testing.T) {
	t.Parallel()
	tool, _ := toolByID("carpet")
	bv, _ := buildable("carpet")
	for i, s := range bv.Stuffs {
		lines := archTooltipLines(tool, s, 0, 0)
		want := fmt.Sprintf("Cost: 1 %s · Work %d · catches %d%% of tracked dirt", s, bv.Work, bv.Catches[i])
		if !contains(lines, want) {
			t.Fatalf("tooltip for %s = %q, want line %q", s, lines, want)
		}
		for _, l := range lines {
			if strings.Contains(l, "wood") && s != "wood carpet" && !strings.Contains(l, "inside a door") && l != tool.desc {
				t.Fatalf("hard-coded wood in %q", l)
			}
		}
		if !contains(lines, "Tip: lay it on the tile just inside a door") {
			t.Fatalf("no door tip in %q", lines)
		}
	}
	if bv.Catches[3] <= bv.Catches[0] {
		t.Fatalf("cloth should catch more than straw: %v", bv.Catches)
	}
}

func TestStoneWallTooltipNamesStone(t *testing.T) {
	t.Parallel()
	tool, _ := toolByID("wall")
	bv, _ := buildable("wall")
	lines := archTooltipLines(tool, "stone", 0, 0)
	want := fmt.Sprintf("Cost: %d stone   Work: %d", bv.Wood, bv.Work)
	if !contains(lines, want) {
		t.Fatalf("lines = %q, want %q", lines, want)
	}
	for _, l := range lines {
		if strings.HasPrefix(l, "Only ") {
			t.Fatalf("wood warning on a stone build: %q", l)
		}
	}
	lines = archTooltipLines(tool, "wood", 0, 0)
	if !contains(lines, "Only 0 wood in the colony") {
		t.Fatalf("wood warning missing: %q", lines)
	}
}

func contains(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

func TestCarpetPlacementSendsStuff(t *testing.T) {
	t.Parallel()
	a, s := stubColony(t)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.furniture")
	click(t, a, "arch.tool.carpet")
	click(t, a, "arch.stuff.cloth carpet")
	a.archDrag = true
	a.archX0, a.archY0, a.archX1, a.archY1 = 4, 4, 4, 4
	a.submitArchitect()
	c := lastCmd(t, s)
	want, _ := game.StuffID("cloth carpet")
	kind, _ := kindID("carpet")
	if c.Kind != command.DesignateBuild || c.Build != kind || c.Stuff != want || want == 0 {
		t.Fatalf("cmd = %+v, want carpet stuff %d", c, want)
	}
}

func TestCarpetSpriteKeys(t *testing.T) {
	t.Parallel()
	for stuff, want := range map[string]string{
		"straw carpet": "furniture/carpet_straw", "wood carpet": "furniture/carpet_wood",
		"leather carpet": "furniture/carpet_leather", "cloth carpet": "furniture/carpet_cloth",
		"": "furniture/carpet_straw", "bogus": "furniture/carpet_straw",
	} {
		if got := buildSpriteStuff("carpet", stuff, "south", 0, false, false); got != want {
			t.Errorf("sprite(%q) = %q, want %q", stuff, got, want)
		}
	}
	if got := buildSprite("carpet", "south", 0, false, false); got != "furniture/carpet_straw" {
		t.Errorf("default = %q", got)
	}
}

func TestPresetArchToolCarpetMaterial(t *testing.T) {
	t.Parallel()
	a, _ := stubColony(t)
	if !a.presetArchTool("carpet", "leather carpet", "south") {
		t.Fatal("no carpet tool")
	}
	tool, _ := toolByID("carpet")
	if got := a.toolStuff(tool); got != "leather carpet" {
		t.Fatalf("stuff = %q", got)
	}
}

func TestCropLinesCottonAndWheat(t *testing.T) {
	t.Parallel()
	var cotton, wheat, potato game.CropView
	for _, c := range game.ZoneSpecies() {
		switch c.Name {
		case "cotton":
			cotton = c
		case "wheat":
			wheat = c
		case "potato":
			potato = c
		}
	}
	if cotton.Name == "" || wheat.Name == "" {
		t.Skip("catalog lacks cotton or wheat")
	}
	if got := strings.Join(cropLines(cotton), "|"); !strings.Contains(got, "cloth, not food") || strings.Contains(got, "food 0 each") {
		t.Fatalf("cotton lines = %q", got)
	}
	if got := strings.Join(cropLines(wheat), "|"); !strings.Contains(got, "+4 straw") {
		t.Fatalf("wheat lines = %q", got)
	}
	if got := strings.Join(cropLines(potato), "|"); strings.Contains(got, "not food") || strings.Contains(got, "straw") {
		t.Fatalf("potato lines = %q", got)
	}
	if lines := cropLines(game.CropView{Name: "tree", Days: 5, Yield: 10}); len(lines) != 2 {
		t.Fatalf("tree lines = %q", lines)
	}
}

func TestPawnDirtyBootsLine(t *testing.T) {
	t.Parallel()
	if got := dirtyBootsLine(game.PawnView{}); got != "" {
		t.Fatalf("clean boots = %q", got)
	}
	if got := dirtyBootsLine(game.PawnView{Dirt: 42}); got != "Dirty boots 42%" {
		t.Fatalf("line = %q", got)
	}
}

func TestCarpetBuildingInspector(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{Kind: "carpet", Stuff: "straw carpet", Catch: 35}
	if got := buildingTitle(b); got != "Straw carpet" {
		t.Fatalf("title = %q", got)
	}
	if got := carpetLine(b); got != "Catches 35% of tracked dirt" {
		t.Fatalf("line = %q", got)
	}
	if got := carpetLine(game.BuildingView{Kind: "wall"}); got != "" {
		t.Fatalf("wall line = %q", got)
	}
	if got := buildingTitle(game.BuildingView{Kind: "wall", Stuff: "stone"}); got != "Stone wall" {
		t.Fatalf("wall title = %q", got)
	}
}

func TestCarpetAndMaterialItemInspector(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"cloth", "straw"} {
		if got := itemCategory(game.ItemView{Kind: k}); got != "material" {
			t.Fatalf("%s category = %q", k, got)
		}
	}
	it := game.ItemView{Kind: "cloth carpet", Qty: 1, Catch: 70, FreshDays: -1}
	if got := itemCategory(it); got != "furnishing" {
		t.Fatalf("category = %q", got)
	}
	lines := itemLines(it, true)
	if !contains(lines, "Catches 70% of tracked dirt") || !contains(lines, "Install from Architect > Furniture > Carpet") {
		t.Fatalf("lines = %q", lines)
	}
	if lines := itemLines(game.ItemView{Kind: "cloth", FreshDays: -1}, false); len(lines) != 1 {
		t.Fatalf("cloth lines = %q", lines)
	}
}

func TestHoverHomeAndCleanliness(t *testing.T) {
	t.Parallel()
	w := game.WorldView{
		TerrainW: 3,
		Home:     []uint8{0, 1, 0},
		RoomGrid: []int{-1, 0, 1},
		Rooms: []game.RoomView{
			{Cleanliness: "dirty", FilthPenalty: 3},
			{Cleanliness: "clean"},
		},
	}
	if got := homeLabelAt(w, 1, 0); got != "Home area" {
		t.Fatalf("home = %q", got)
	}
	if got := homeLabelAt(w, 0, 0); got != "" {
		t.Fatalf("not home = %q", got)
	}
	if got := homeLabelAt(w, 9, 9); got != "" {
		t.Fatalf("oob = %q", got)
	}
	if got := roomLabelAt(w, 1, 0); !strings.Contains(got, "dirty (-3)") {
		t.Fatalf("hover room label = %q, want the cleanliness once", got)
	}
}

func TestCaptureHomeAndCarpetStates(t *testing.T) {
	t.Parallel()
	for _, ui := range []string{"home", "carpet"} {
		a, g, _ := liveColony(t, 0)
		_ = g
		a.captureUI(ui)
		switch ui {
		case "home":
			if a.archTool != "home" || !a.renderer.homeShown() {
				t.Fatalf("home capture: tool=%q shown=%v", a.archTool, a.renderer.homeShown())
			}
		case "carpet":
			if a.archTool != "carpet" || a.toolStuff(archTool{build: "carpet"}) != "cloth carpet" {
				t.Fatalf("carpet capture: tool=%q", a.archTool)
			}
		}
	}
}
