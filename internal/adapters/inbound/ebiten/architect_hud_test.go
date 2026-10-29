package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// PLAY-018 (ARCHITECT): the Architect on the HUD grid. Two category
// columns and a description box sit at the lower-left (ArchCats, ArchDesc);
// the open category's tools are command buttons in the grid.

// archColony is a stub colony with the Architect open on Structure.
func archColony(t *testing.T) (*App, *stubSim) {
	t.Helper()
	s := &stubSim{}
	a, _ := colonyApp(t, s)
	a.arch.OpenPanel()
	return a, s
}

// archCmd finds the tool command with the legacy click id.
func archCmd(a *App, id string) (inspector.Button, bool) {
	for _, c := range a.archToolCommands() {
		if c.ID == id {
			return c, true
		}
	}
	return inspector.Button{}, false
}

// archPick clicks a tool the way the command grid does: the command's
// legacy id goes to applyClick (a disabled one only says why).
func archPick(t *testing.T, a *App, tool string) {
	t.Helper()
	c, ok := archCmd(a, "arch.tool."+tool)
	if !ok {
		t.Fatalf("no command for tool %q in category %q", tool, a.arch.Cat)
	}
	if c.Disabled {
		a.hint = c.Reason
		return
	}
	if c.Run != nil || len(c.Cmds) > 0 {
		t.Fatalf("tool command %q must go through applyClick: Run=%v Cmds=%v", c.ID, c.Run != nil, c.Cmds)
	}
	a.applyClick(c.ID)
}

// archButtonsWith are the Architect's hit targets whose id starts with prefix.
func archButtonsWith(a *App, prefix string) []uikit.Button {
	var out []uikit.Button
	for _, b := range a.arch.Buttons() {
		if strings.HasPrefix(b.ID, prefix) {
			out = append(out, b)
		}
	}
	return out
}

// archDescText is the description box's text lines, top to bottom.
func archDescText(a *App) []string {
	var out []string
	for _, l := range a.arch.Plan().Lines {
		out = append(out, l.Text)
	}
	return out
}

func within(b uikit.Button, r gfx.Rect) bool {
	return b.X >= r.X && b.Y >= r.Y && b.X+b.W <= r.Right() && b.Y+b.H <= r.Bottom()
}

func overlap(a, b uikit.Button) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

func TestArchitectCategoriesInTwoColumns(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	l := a.hud()
	if l.ArchCats.Empty() {
		t.Fatal("the layout has no ArchCats while the Architect is open")
	}
	cats := archButtonsWith(a, "arch.cat.")
	if len(cats) != len(architect.Categories) {
		t.Fatalf("%d category buttons, want %d", len(cats), len(architect.Categories))
	}
	cols := map[float32]bool{}
	for i, b := range cats {
		if want := "arch.cat." + architect.Categories[i].ID; b.ID != want {
			t.Errorf("category %d id = %q, want %q (catalog order)", i, b.ID, want)
		}
		if !within(b, l.ArchCats) {
			t.Errorf("%s %+v is outside ArchCats %+v", b.ID, b, l.ArchCats)
		}
		if b.W < 92 || b.W > 104 || b.H != 28 {
			t.Errorf("%s is %vx%v, want about 100x28", b.ID, b.W, b.H)
		}
		if b.Selected != (architect.Categories[i].ID == a.arch.Cat) {
			t.Errorf("%s selected = %v with archCat %q", b.ID, b.Selected, a.arch.Cat)
		}
		if b.Label != architect.Categories[i].Label {
			t.Errorf("%s label = %q", b.ID, b.Label)
		}
		cols[b.X] = true
		for _, o := range cats[:i] {
			if overlap(b, o) {
				t.Errorf("%s overlaps %s", b.ID, o.ID)
			}
		}
	}
	if len(cols) != 2 {
		t.Errorf("categories sit in %d columns, want 2: %v", len(cols), cols)
	}
}

func TestArchitectNoLegacyToolCards(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	if got := archButtonsWith(a, "arch.tool."); len(got) != 0 {
		t.Fatalf("tool buttons come from the command grid now, found %d card(s) in the panel", len(got))
	}
	assertChromeIsTheLayout(t, a) // the layout covers the Architect's regions: no legacy area
	// The panel rect the older code asked for is the Architect's area: the
	// description box and the categories, from the layout.
	l := a.hud()
	r := a.arch.Rect()
	if r.X > l.ArchCats.X || r.X+r.W < l.ArchCats.Right() || r.Y+r.H != l.ArchCats.Bottom() {
		t.Fatalf("architectRect %+v does not cover ArchCats %+v", r, l.ArchCats)
	}
}

// The category the Architect opens on is not yet a choice: clicking it
// confirms it, and only then does a click collapse it.
func TestArchitectOpeningCategoryIsConfirmedBeforeItCollapses(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	if a.arch.Cat != "structure" {
		t.Fatalf("opened on %q, want structure", a.arch.Cat)
	}
	a.applyClick("arch.cat.structure")
	if a.arch.Cat != "structure" || len(a.archToolCommands()) == 0 {
		t.Fatalf("the first click on the opening category collapsed it: %q", a.arch.Cat)
	}
	a.applyClick("arch.cat.structure")
	if a.arch.Cat != "" {
		t.Fatalf("the second click left %q open", a.arch.Cat)
	}

	// Arming a tool in the opening category counts as working in it.
	b, _ := archColony(t)
	archPick(t, b, "wall")
	b.applyClick("arch.cat.structure")
	if b.arch.Cat != "" || b.arch.ToolID != "" {
		t.Fatalf("after arming a tool, a category click left archCat=%q archTool=%q", b.arch.Cat, b.arch.ToolID)
	}
}

func TestArchitectCategoryClickSelectsAndCollapses(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	a.applyClick("arch.cat.zones")
	if a.arch.Cat != "zones" {
		t.Fatalf("archCat = %q, want zones", a.arch.Cat)
	}
	cmds := a.archToolCommands()
	if len(cmds) == 0 || cmds[0].ID != "arch.tool.stockpile" {
		t.Fatalf("zones tools = %+v, want stockpile first", cmds)
	}
	archPick(t, a, "stockpile")
	if a.arch.ToolID != "stockpile" {
		t.Fatalf("archTool = %q", a.arch.ToolID)
	}
	// Clicking the open category again collapses it: no tools, none armed.
	a.applyClick("arch.cat.zones")
	if a.arch.Cat != "" || a.arch.ToolID != "" {
		t.Fatalf("collapse left archCat=%q archTool=%q", a.arch.Cat, a.arch.ToolID)
	}
	if n := len(a.archToolCommands()); n != 0 {
		t.Fatalf("%d tool commands with no category open", n)
	}
	if n := len(a.insp.Commands().SelectionCommands()); n != 0 {
		t.Fatalf("%d selection commands with no category open", n)
	}
	// The categories stay, none selected.
	for _, b := range archButtonsWith(a, "arch.cat.") {
		if b.Selected {
			t.Errorf("%s selected with no category open", b.ID)
		}
	}
	a.applyClick("arch.cat.orders")
	if a.arch.Cat != "orders" || len(a.archToolCommands()) == 0 {
		t.Fatalf("category did not reopen: %q", a.arch.Cat)
	}
	// Reopening the Architect after a collapse lands on a category again.
	a.applyClick("arch.cat.orders")
	a.arch.ClosePanel()
	a.arch.OpenPanel()
	if a.arch.Cat == "" {
		t.Fatal("openArchitect must leave a category open")
	}
}

func TestArchitectCategoryChangeDropsOtherCategoryTool(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	a.applyClick("arch.cat.orders") // another category than the open Structure: opens it
	archPick(t, a, "harvest")
	a.applyClick("arch.cat.zones")
	if a.arch.ToolID != "" {
		t.Fatalf("archTool = %q after leaving its category", a.arch.ToolID)
	}
}

func TestArchitectToolsAreCommands(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	for _, c := range architect.Categories {
		a.arch.Cat = c.ID
		a.arch.ToolID = ""
		cmds := a.archToolCommands()
		sel := a.insp.Commands().SelectionCommands()
		tools := toolsIn(c.ID)
		if len(cmds) != len(tools) || len(sel) != len(tools) {
			t.Fatalf("%s: %d commands (%d via selection), want %d", c.ID, len(cmds), len(sel), len(tools))
		}
		for i, tool := range tools {
			cb := cmds[i]
			if cb.ID != "arch.tool."+tool.ID || cb.Label != tool.Label || cb.Icon != tool.Icon {
				t.Errorf("%s[%d] = {%q %q %q}, want {arch.tool.%s %q %q}", c.ID, i, cb.ID, cb.Label, cb.Icon, tool.ID, tool.Label, tool.Icon)
			}
			if !strings.Contains(cb.Desc, tool.Desc) {
				t.Errorf("%s: Desc %q lacks the tool description", tool.ID, cb.Desc)
			}
			if bv, ok := architect.Buildable(tool.Build); ok && !strings.Contains(cb.Desc, "Cost: ") {
				t.Errorf("%s: Desc %q lacks the cost of %s", tool.ID, cb.Desc, bv.Kind)
			}
			if cb.Run != nil || len(cb.Cmds) > 0 || cb.Options != nil {
				t.Errorf("%s: legacy ids go through applyClick, got Run/Cmds/Options", cb.ID)
			}
			if cb.On || cb.Disabled {
				t.Errorf("%s: On=%v Disabled=%v with nothing selected", cb.ID, cb.On, cb.Disabled)
			}
		}
	}
}

func TestArchitectSelectedToolIsOn(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t) // opens on Structure
	archPick(t, a, "door")
	on := ""
	for _, c := range a.archToolCommands() {
		if c.On {
			if on != "" {
				t.Fatalf("two tools On: %s and %s", on, c.ID)
			}
			on = c.ID
		}
	}
	if on != "arch.tool.door" {
		t.Fatalf("On tool = %q, want arch.tool.door", on)
	}
	archPick(t, a, "door") // the selected button again: off
	for _, c := range a.archToolCommands() {
		if c.On {
			t.Fatalf("%s still On after toggling off", c.ID)
		}
	}
}

func TestArchitectLockedToolsAreDisabledWithReason(t *testing.T) {
	t.Parallel()
	s := &stubSim{buildables: lockedCatalog("campfire")}
	s.world.Locked = map[string]string{"build:lamp": "Electricity"}
	a, _ := colonyApp(t, s)
	a.arch.OpenPanel()
	a.applyClick("arch.cat.furniture")

	fire, ok1 := archCmd(a, "arch.tool.campfire")
	lamp, ok2 := archCmd(a, "arch.tool.lamp")
	table, ok3 := archCmd(a, "arch.tool.table")
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("missing furniture commands: campfire=%v lamp=%v table=%v", ok1, ok2, ok3)
	}
	if !fire.Disabled || fire.Reason != game.ReasonScenario {
		t.Errorf("campfire = %+v, want disabled: %s", fire, game.ReasonScenario)
	}
	if !lamp.Disabled || lamp.Reason != "Needs research: Electricity" {
		t.Errorf("lamp = %+v, want disabled: Needs research: Electricity", lamp)
	}
	if table.Disabled || table.Reason != "" {
		t.Errorf("table = %+v, want enabled", table)
	}
	// A locked tool never selects, and says why.
	archPick(t, a, "campfire")
	if a.arch.ToolID != "" || a.hint != game.ReasonScenario {
		t.Fatalf("tool=%q hint=%q: a locked tool must not select and says why", a.arch.ToolID, a.hint)
	}
	// An area tool has no build to lock.
	a.applyClick("arch.cat.orders")
	for _, c := range a.archToolCommands() {
		if c.Disabled {
			t.Errorf("%s disabled: %q", c.ID, c.Reason)
		}
	}
}

func TestArchitectDescriptionBoxSitsAboveCategories(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	for _, tool := range []string{"", "wall", "bed", "carpet"} {
		a.arch.ToolID = ""
		if tool != "" {
			a.arch.Cat = toolCategory(t, tool)
			archPick(t, a, tool)
		}
		h := a.arch.DescHeight()
		if h <= 0 {
			t.Fatalf("tool %q: archDescHeight = %v, want > 0", tool, h)
		}
		l := a.hud()
		if l.ArchDesc.Empty() || l.ArchDesc.H != h || l.ArchDesc.W != hudlayout.ArchDescW {
			t.Fatalf("tool %q: ArchDesc %+v, want %vx%v", tool, l.ArchDesc, hudlayout.ArchDescW, h)
		}
		if l.ArchDesc.Bottom() != l.ArchCats.Y-hudlayout.HudGap {
			t.Errorf("tool %q: ArchDesc bottom %v, want ArchCats.Y-gap = %v", tool, l.ArchDesc.Bottom(), l.ArchCats.Y-hudlayout.HudGap)
		}
	}
}

func toolCategory(t *testing.T, id string) string {
	t.Helper()
	tool, ok := toolByID(id)
	if !ok {
		t.Fatalf("no tool %q", id)
	}
	return tool.Category
}

func TestArchitectDescriptionHeightGrowsWithContent(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	none := a.arch.DescHeight()
	a.arch.Cat = "structure"
	archPick(t, a, "wall") // seven materials
	wall := a.arch.DescHeight()
	a.applyClick("arch.cat.zones")
	archPick(t, a, "grow") // the crop picker
	grow := a.arch.DescHeight()
	if !(none < wall) || !(none < grow) {
		t.Fatalf("heights none=%v wall=%v grow=%v: a selected tool shows more than the hint", none, wall, grow)
	}
	// Repeated asks are stable (the height feeds the layout every frame).
	if again := a.arch.DescHeight(); again != grow {
		t.Fatalf("archDescHeight changed between calls: %v then %v", grow, again)
	}
	a.arch.Open = false
	if h := a.arch.DescHeight(); h != 0 {
		t.Fatalf("archDescHeight = %v with the Architect closed", h)
	}
}

func TestArchitectDescriptionShowsNameCostDescriptionStock(t *testing.T) {
	t.Parallel()
	s := &stubSim{world: game.WorldView{Wood: 7, Stone: 3}}
	a, _ := colonyApp(t, s)
	a.arch.OpenPanel()
	text := strings.Join(archDescText(a), "\n")
	if !strings.Contains(text, "Wood 7   Stone 3") || !strings.Contains(text, "Pick something to place") {
		t.Fatalf("empty box = %q, want the stock line and the hint", text)
	}

	a.applyClick("arch.cat.furniture")
	archPick(t, a, "hospital_bed")
	bv, _ := architect.Buildable("hospital bed")
	tool, _ := toolByID("hospital_bed")
	text = strings.Join(archDescText(a), "\n")
	for _, want := range []string{
		tool.Label,
		"Cost: 45 wood + 2 components",
		"Only 7 wood in the colony", // 45 wood needed, 7 in stock
		"Wood 7   Stone 3",
		"Click to place",
		"rotate (south)",
		"Right-click: stop placing",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("hospital bed box lacks %q\n%s", want, text)
		}
	}
	// The description wraps: its words are all there, in order.
	if got := strings.Join(strings.Fields(text), " "); !strings.Contains(got, strings.Join(strings.Fields(tool.Desc)[:6], " ")) {
		t.Errorf("box lacks the start of the description %q\n%s", tool.Desc, text)
	}
	if bv.Wood != 45 {
		t.Fatalf("catalog changed: hospital bed costs %d wood", bv.Wood)
	}

	// Another material changes the cost line.
	a.applyClick("arch.cat.structure")
	archPick(t, a, "wall")
	a.applyClick("arch.stuff.steel")
	text = strings.Join(archDescText(a), "\n")
	if !strings.Contains(text, "steel") || strings.Contains(text, "Only ") {
		t.Errorf("wall in steel box = %q", text)
	}
}

func TestArchitectDescriptionMaterialRow(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	archPick(t, a, "wall")
	l := a.hud()
	stuffs := archButtonsWith(a, "arch.stuff.")
	bv, _ := architect.Buildable("wall")
	if len(stuffs) != len(bv.Stuffs) || len(stuffs) < 2 {
		t.Fatalf("%d material buttons, want %d", len(stuffs), len(bv.Stuffs))
	}
	for i, b := range stuffs {
		if !within(b, l.ArchDesc) {
			t.Errorf("%s %+v outside ArchDesc %+v", b.ID, b, l.ArchDesc)
		}
		if b.Selected != (bv.Stuffs[i] == a.toolStuff(architect.ToolDef{Build: "wall"})) {
			t.Errorf("%s selected = %v", b.ID, b.Selected)
		}
		for _, o := range stuffs[:i] {
			if overlap(b, o) {
				t.Errorf("%s overlaps %s", b.ID, o.ID)
			}
		}
	}
	// A tool with one material shows no row.
	for _, tool := range allArchTools() {
		if bv, ok := architect.Buildable(tool.Build); !ok || len(bv.Stuffs) != 1 {
			continue
		}
		a.arch.Cat = tool.Category
		archPick(t, a, tool.ID)
		if got := archButtonsWith(a, "arch.stuff."); len(got) != 0 {
			t.Fatalf("%s builds from one material but offers %d buttons", tool.ID, len(got))
		}
		return
	}
	t.Fatal("the catalog has no single-material tool to check")
}

func TestArchitectDescriptionCropPicker(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	a.applyClick("arch.cat.zones")
	if got := archButtonsWith(a, "arch.crop."); len(got) != 0 {
		t.Fatalf("%d crop buttons before the growing zone is armed", len(got))
	}
	archPick(t, a, "grow")
	l := a.hud()
	crops := archButtonsWith(a, "arch.crop.")
	if len(crops) != len(game.ZoneSpecies()) {
		t.Fatalf("%d crop buttons, want %d", len(crops), len(game.ZoneSpecies()))
	}
	sel := 0
	for i, b := range crops {
		if !within(b, l.ArchDesc) {
			t.Errorf("%s %+v outside ArchDesc %+v", b.ID, b, l.ArchDesc)
		}
		if b.Selected {
			sel++
		}
		for _, o := range crops[:i] {
			if overlap(b, o) {
				t.Errorf("%s overlaps %s", b.ID, o.ID)
			}
		}
	}
	if sel != 1 {
		t.Errorf("%d crops selected, want 1", sel)
	}
	a.applyClick("arch.crop.tree")
	if a.arch.Crop != "tree" {
		t.Fatalf("archCrop = %q", a.arch.Crop)
	}
	text := strings.Join(archDescText(a), "\n")
	if !strings.Contains(text, "Tree: ") || !strings.Contains(text, "wood per tree") {
		t.Errorf("box lacks the tree stats:\n%s", text)
	}
	if !strings.Contains(text, architect.StyleNames[architect.StyleRect]) || !strings.Contains(text, "shape, Ctrl square") {
		t.Errorf("box lacks the area tool's draw style:\n%s", text)
	}
}

func TestArchitectDescriptionDragStyleHints(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	archPick(t, a, "wall")
	text := strings.Join(archDescText(a), "\n")
	if !strings.Contains(text, architect.StyleNames[architect.StyleLine]) || !strings.Contains(text, "shape, Ctrl square") {
		t.Errorf("wall hint lacks its draw style:\n%s", text)
	}
	a.arch.Style = architect.StyleOval
	text = strings.Join(archDescText(a), "\n")
	if !strings.Contains(text, architect.StyleNames[architect.StyleOval]) {
		t.Errorf("the box did not follow the draw style:\n%s", text)
	}
}

// Every tool, with every material and crop, fits its box at the sizes the
// game runs at, and the regions do not overlap the command grid.
func TestArchitectBoxFitsEveryToolAtEverySize(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	for _, sz := range [][2]int{{1280, 720}, {1920, 1080}, {1113, 626}} {
		a.sw, a.sh = sz[0], sz[1]
		for _, tool := range allArchTools() {
			a.arch.Cat = tool.Category
			a.arch.ToolID = ""
			a.arch.Stuff = ""
			archPick(t, a, tool.ID)
			if a.arch.ToolID != tool.ID {
				continue // an allowed-area tool opens its area menu first
			}
			stuffs := []string{""}
			if bv, ok := architect.Buildable(tool.Build); ok && len(bv.Stuffs) > 1 {
				stuffs = bv.Stuffs
			}
			for _, s := range stuffs {
				a.arch.Stuff = s
				l := a.hud()
				if l.ArchDesc.Empty() || l.ArchDesc.H != a.arch.DescHeight() {
					t.Errorf("%dx%d %s/%s: box %+v is clamped (wants %v)", sz[0], sz[1], tool.ID, s, l.ArchDesc, a.arch.DescHeight())
				}
				if l.ArchDesc.Y < 0 {
					t.Errorf("%dx%d %s/%s: box above the screen: %+v", sz[0], sz[1], tool.ID, s, l.ArchDesc)
				}
				for _, ln := range a.arch.Plan().Lines {
					if w, _ := gfx.Measure(ln.Text, ln.Style); w > architect.InnerW+0.5 {
						t.Errorf("%dx%d %s/%s: line %q is %.1f wide, the box holds %v", sz[0], sz[1], tool.ID, s, ln.Text, w, float32(architect.InnerW))
					}
					if ln.Y+gfx.LineStep(ln.Style) > l.ArchDesc.H {
						t.Errorf("%dx%d %s/%s: line %q at y=%v runs past the box (%v)", sz[0], sz[1], tool.ID, s, ln.Text, ln.Y, l.ArchDesc.H)
					}
				}
				for _, b := range archButtonsWith(a, "arch.stuff.") {
					if !within(b, l.ArchDesc) {
						t.Errorf("%dx%d %s: %s %+v outside the box %+v", sz[0], sz[1], tool.ID, b.ID, b, l.ArchDesc)
					}
				}
				for _, b := range archButtonsWith(a, "arch.crop.") {
					if !within(b, l.ArchDesc) {
						t.Errorf("%dx%d %s: %s %+v outside the box %+v", sz[0], sz[1], tool.ID, b.ID, b, l.ArchDesc)
					}
				}
			}
		}
	}
}

func TestArchitectLayoutKeepsRegionsApart(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t)
	for _, sz := range [][2]int{{1280, 720}, {1920, 1080}, {1113, 626}} {
		a.sw, a.sh = sz[0], sz[1]
		for _, c := range architect.Categories {
			a.arch.Cat = c.ID
			a.arch.ToolID = ""
			cmds := a.archToolCommands()
			l := hudlayout.Compute(sz[0], sz[1], hudlayout.Inputs{ArchOpen: true, Commands: len(cmds), ArchDescH: a.arch.DescHeight(), Colonists: 6})
			regions := []gfx.Rect{l.Tabs, l.ArchCats, l.ArchDesc}
			regions = append(regions, l.GridCells...)
			for i := range regions {
				for j := range regions[:i] {
					p, q := regions[i], regions[j]
					if p.Empty() || q.Empty() {
						continue
					}
					if p.X < q.Right() && q.X < p.Right() && p.Y < q.Bottom() && q.Y < p.Bottom() {
						t.Errorf("%dx%d %s: region %d %+v overlaps region %d %+v", sz[0], sz[1], c.ID, i, p, j, q)
					}
				}
			}
			if len(l.GridCells) != len(cmds) {
				t.Errorf("%dx%d %s: %d grid cells for %d tools", sz[0], sz[1], c.ID, len(l.GridCells), len(cmds))
			}
			if len(l.GridCells) > 0 && l.GridCells[0].X != l.ArchCats.Right()+hudlayout.HudMargin {
				t.Errorf("%dx%d %s: grid starts at %v, want right of ArchCats (%v)", sz[0], sz[1], c.ID, l.GridCells[0].X, l.ArchCats.Right()+hudlayout.HudMargin)
			}
		}
	}
}

// A click by id still arms the tool, and the drag still sends the same
// commands as before the re-layout.
func TestArchitectToolClickEmitsTheSameCommands(t *testing.T) {
	t.Parallel()
	a, s := archColony(t)

	// A line of walls: one DesignateBuild per cell, wood by default.
	archPick(t, a, "wall") // Structure is open
	if a.arch.ToolID != "wall" {
		t.Fatalf("archTool = %q", a.arch.ToolID)
	}
	a.arch.Drag = true
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 3, 4, 6, 4
	a.arch.Submit()
	wall, _ := architect.KindID("wall")
	wood, _ := game.StuffID("wood")
	if len(s.cmds) != 4 {
		t.Fatalf("%d commands for a 4-cell wall line: %+v", len(s.cmds), s.cmds)
	}
	for i, c := range s.cmds {
		if c.Kind != command.DesignateBuild || c.Build != wall || c.Stuff != wood || c.X != 3+i || c.Y != 4 {
			t.Errorf("wall command %d = %+v", i, c)
		}
	}

	// A material pick changes the stuff of the next command.
	s.cmds = nil
	a.applyClick("arch.stuff.steel")
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 8, 8, 8, 8
	a.arch.Submit()
	steel, _ := game.StuffID("steel")
	if len(s.cmds) != 1 || s.cmds[0].Stuff != steel {
		t.Fatalf("steel wall commands = %+v", s.cmds)
	}

	// An area tool sends one rectangle.
	s.cmds = nil
	a.applyClick("arch.cat.zones")
	archPick(t, a, "stockpile")
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 2, 2, 5, 3
	a.arch.Submit()
	if len(s.cmds) != 1 || s.cmds[0].Kind != command.DesignateStockpile || s.cmds[0].X != 2 || s.cmds[0].Y2 != 3 {
		t.Fatalf("stockpile commands = %+v", s.cmds)
	}

	// A growing zone carries the picked crop.
	s.cmds = nil
	archPick(t, a, "grow")
	a.applyClick("arch.crop.corn")
	a.arch.X0, a.arch.Y0, a.arch.X1, a.arch.Y1 = 1, 1, 2, 2
	a.arch.Submit()
	if len(s.cmds) != 1 || s.cmds[0].Kind != command.DesignateGrowZone {
		t.Fatalf("grow commands = %+v", s.cmds)
	}
	if id, _ := game.ZoneSpeciesID("corn"); s.cmds[0].Build != id {
		t.Fatalf("grow zone crop = %d, want %d (corn)", s.cmds[0].Build, id)
	}
}

// The tool buttons reach applyClick through the command grid itself.
func TestArchitectToolClickThroughTheGrid(t *testing.T) {
	t.Parallel()
	a, _ := archColony(t) // opens on Structure
	l := a.hud()
	if len(l.GridCells) == 0 {
		t.Fatalf("no command grid cells for %d Structure tools: commandCount must count selectionCommands (CORE-PANE)", len(a.archToolCommands()))
	}
	var cell uikit.Button
	for _, b := range a.insp.Commands().CommandGridButtons() {
		if b.ID == "arch.tool.wall" {
			cell = b
		}
	}
	if cell.ID == "" {
		t.Fatalf("no arch.tool.wall hit target in the command grid (%d cells)", len(l.GridCells))
	}
	inGrid := false
	for _, r := range l.GridCells {
		inGrid = inGrid || r.Contains(cell.X+cell.W/2, cell.Y+cell.H/2)
	}
	if !inGrid {
		t.Fatalf("wall button %+v is not in a grid cell", cell)
	}
	click(t, a, "arch.tool.wall")
	if a.arch.ToolID != "wall" {
		t.Fatalf("archTool = %q after a grid click", a.arch.ToolID)
	}
}

// The panel and the box paint for every kind of tool, with the cursor on
// a category and on a material button.
func TestArchitectDraws(t *testing.T) {
	t.Parallel()
	a, f := colonyApp(t, &stubSim{world: game.WorldView{Wood: 50, Stone: 20}})
	a.arch.OpenPanel()
	for _, tool := range []string{"", "wall", "carpet", "grow", "hospital_bed", "stockpile", "door"} {
		a.arch.ToolID = ""
		if tool != "" {
			a.arch.Cat = toolCategory(t, tool)
			archPick(t, a, tool)
		}
		for _, id := range []string{"arch.cat.zones", "arch.stuff.steel", "arch.crop.potato"} {
			f.Cur.Mx, f.Cur.My = -50, -50
			for _, b := range a.arch.Buttons() {
				if b.ID == id {
					f.Cur.Mx, f.Cur.My = int(b.X+b.W/2), int(b.Y+b.H/2)
				}
			}
			drawFrame(t, a, f)
		}
	}
}
