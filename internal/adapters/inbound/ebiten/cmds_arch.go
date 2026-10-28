package ebiten

import (
	"fmt"
	"strings"
	"sync"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/application/game"
)

// The Architect on the colony HUD (PLAY-018: ARCHITECT owns it). At the
// lower-left, two category columns (ArchCats) carry a description box
// above them (ArchDesc); the open category's tools are command buttons in
// the grid to their right (archToolCommands). Placement input and the
// ghost stay in architect.go.
//
// The description box is content-sized: archPlan measures it once per
// change of its inputs (the layout asks for its height every frame, and
// wrapping text is the costly part), and archDescHeight reports that
// height to the layout. Nothing here may call a.hud(); the box does not
// follow the hovered tool (its height would then depend on the grid, which
// depends on the layout): the grid's tooltip shows the hovered tool.

const (
	archPad    = 8 // inside the category panel and the description box
	archCatW   = 96
	archCatH   = 28
	archCatGap = 4

	archBtnH     = 22 // a material or crop button
	archBtnGap   = 4
	archBlockGap = 4 // between blocks of the description box
	archCropCols = 4 // crop buttons per row, fewer when a name does not fit

	archInnerW = archDescW - 2*archPad
)

// archToolCommands are the open category's tools as command buttons, in
// catalog order. They keep the legacy click ids (arch.tool.<id>), so a
// click reaches selectArchTool through applyClick; the selected tool is On
// and a tool the scenario or the research forbids is disabled with the
// reason.
func (a *App) archToolCommands() []cmdButton {
	if !a.archOpen {
		return nil
	}
	tools := toolsIn(a.archCat)
	if len(tools) == 0 {
		return nil
	}
	var w game.WorldView
	if a.renderer != nil {
		w = a.shell.World()
	}
	out := make([]cmdButton, 0, len(tools))
	for _, t := range tools {
		b := cmdButton{ID: ids.PrefixArchTool + t.id, Label: t.label, Icon: t.icon, Toggle: true}
		stuff := a.toolStuff(t)
		stock := 0
		if bv, ok := buildable(t.build); ok && bv.Carpet {
			stock = stockOf(w, stuff)
		}
		// everything after the name: the description, then the cost lines
		b.Desc = strings.Join(archTooltipLines(t, stuff, w.Wood, stock)[1:], "\n")
		if why := a.toolLock(t); why != "" {
			b.Disabled, b.Reason = true, why
		} else {
			b.On = a.archTool == t.id
		}
		out = append(out, b)
	}
	return out
}

// archToolCell is the command-grid cell of tool id in the open category.
func (a *App) archToolCell(id string) (rect, bool) {
	cells := a.hud().GridCells
	for i, c := range a.archToolCommands() {
		if c.ID == ids.PrefixArchTool+id && i < len(cells) {
			return cells[i], true
		}
	}
	return rect{}, false
}

// archDescHeight is the description box's height (0 while the Architect
// is closed).
func (a *App) archDescHeight() float32 {
	if !a.archOpen {
		return 0
	}
	return a.archPlan().h
}

// archState is this region's presentation state (App.hst): the
// description box measured for the inputs in key.
type archState struct {
	key  archPlanKey
	plan archDescPlan
	ok   bool

	// dflt is the category openArchitect opened on its own, until the
	// player picks a category or a tool (clicking it then is a first
	// choice, not a collapse).
	dflt string
}

// archTextLine is one wrapped line of the description box, y from its top.
type archTextLine struct {
	y     float32
	text  string
	style textStyle
}

// archDescPlan is the description box laid out: its height, its text, and
// where the material and crop buttons start (0: none).
type archDescPlan struct {
	h     float32
	lines []archTextLine

	stuffTop    float32
	stuffPerRow int
	stuffW      float32

	cropTop  float32
	cropCols int
	cropW    float32
}

// archPlanKey is everything the plan depends on.
type archPlanKey struct {
	tool, stuff, crop, rot string
	facing                 uint8
	style                  drawStyle
	scale                  float64
	hasWorld               bool
	wood, stone, stock     int
}

func (a *App) archKey() archPlanKey {
	k := archPlanKey{tool: a.archTool, facing: a.archFacing, scale: hudScale(), rot: a.rotateKeys()}
	var w game.WorldView
	if a.renderer != nil {
		w = a.shell.World()
		k.hasWorld, k.wood, k.stone = true, w.Wood, w.Stone
	}
	t, ok := toolByID(a.archTool)
	if !ok {
		return k
	}
	k.style = a.curStyle(t)
	if t.crop != "" {
		k.crop = a.selectedCrop().Name
	}
	if _, ok := buildable(t.build); ok {
		k.stuff = a.toolStuff(t)
		if k.stuff != "wood" { // wood is the world's own count
			k.stock = stockOf(w, k.stuff)
		}
	}
	return k
}

// archPlan is the description box for the current state, measured again
// only when its inputs change. The result is read-only.
func (a *App) archPlan() *archDescPlan {
	k := a.archKey()
	s := &a.hst.arch
	if !s.ok || s.key != k {
		s.plan, s.key, s.ok = a.buildArchPlan(k), k, true
	}
	return &s.plan
}

// buildArchPlan lays out the box: the armed tool's name, description,
// cost, the stock line, its material row or crop picker, and the
// placement hints; or the stock line and a hint when nothing is armed.
func (a *App) buildArchPlan(k archPlanKey) archDescPlan {
	var p archDescPlan
	head := withColor(tsLabel, uiHeading)
	dim := withColor(tsSmall, uiTextDim)
	warn := withColor(tsBody, uiWarning)
	y := float32(archPad)
	add := func(s string, st textStyle) {
		lines := []string{s}
		if w, _ := measure(s, st); w > archInnerW { // a line that fits keeps its spacing
			lines = wrapPx(s, st, archInnerW)
		}
		for _, ln := range lines {
			p.lines = append(p.lines, archTextLine{y: y, text: ln, style: st})
			y += lineStep(st)
		}
	}
	var w game.WorldView
	if a.renderer != nil {
		w = a.shell.World()
	}
	t, ok := toolByID(a.archTool)
	if !ok {
		if k.hasWorld {
			add(archStockLine(w, w.Wood, ""), tsBody)
		}
		add("Pick something to place", dim)
		p.h = y + archPad
		return p
	}

	add(t.label, head)
	add(t.desc, dim)
	bv, built := buildable(t.build)
	stuff := ""
	if built && !bv.Carpet {
		stuff = k.stuff
	}
	if built {
		y += archBlockGap
		stock := 0
		if bv.Carpet {
			stock = k.stock
		}
		for _, ln := range archTooltipLines(t, k.stuff, w.Wood, stock)[2:] {
			st := tsBody
			if strings.HasPrefix(ln, "Only ") {
				st = warn
			}
			add(ln, st)
		}
	}
	if k.hasWorld {
		y += archBlockGap
		add(archStockLine(w, w.Wood, stuff), tsBody)
	}
	if built && len(bv.Stuffs) > 1 {
		y += archBlockGap
		perRow, rows, bw := archStuffGrid(bv)
		p.stuffTop, p.stuffPerRow, p.stuffW = y, perRow, bw
		y += float32(rows)*(archBtnH+archBtnGap) - archBtnGap
	}
	if t.crop != "" {
		y += archBlockGap
		cols, bw := archCropGrid()
		p.cropTop, p.cropCols, p.cropW = y, cols, bw
		rows := (len(zoneSpecies()) + cols - 1) / cols
		y += float32(rows)*(archBtnH+archBtnGap) - archBtnGap + archBlockGap
		for _, ln := range cropLines(a.selectedCrop()) {
			add(ln, dim)
		}
	}
	y += archBlockGap
	add(a.archHint(t), dim)
	add("Right-click: stop placing", dim)
	p.h = y + archPad
	return p
}

// archHint is how to place tool t: the drag shape, the rotate keys, the
// draw style.
func (a *App) archHint(t archTool) string {
	hint := map[placeMode]string{
		placeSingle: "Click to place",
		placeLine:   "Drag for a line",
		placeFill:   "Drag to fill",
		placeArea:   "Drag an area",
	}[t.mode]
	if t.build == "door" || t.build == "fence gate" {
		hint += "  faces along the wall or fence; " + a.rotateKeys() + " sets a free-standing one (" + facingNames[a.archFacing] + ")"
	} else if bv, ok := buildable(t.build); ok && bv.Rotatable {
		hint += "  " + a.rotateKeys() + " rotate (" + facingNames[a.archFacing] + ")"
	}
	if len(toolStyles(t)) > 1 {
		hint = styleNames[a.curStyle(t)] + "  " + a.rotateKeys() + " shape, Ctrl square"
	}
	return hint
}

// cropLabel is a crop button's text.
func cropLabel(name string) string { return strings.ToUpper(name[:1]) + name[1:] }

// archCropGrid is the crop picker's columns and button width: four across
// unless a name does not fit its button.
func archCropGrid() (cols int, bw float32) {
	for cols = archCropCols; cols > 1; cols-- {
		bw = (archInnerW - archBtnGap*float32(cols-1)) / float32(cols)
		fits := true
		for _, c := range zoneSpecies() {
			fits = fits && textW(cropLabel(c.Name), tsBody)+8 <= bw
		}
		if fits {
			return cols, bw
		}
	}
	return cols, (archInnerW - archBtnGap*float32(cols-1)) / float32(cols)
}

// archDescButtons are the armed tool's material buttons and, for a
// growing zone, the crop picker, inside the description box r. Buttons the
// box was too small to hold are left out.
func (a *App) archDescButtons(r rect) []button {
	t, ok := toolByID(a.archTool)
	if !ok {
		return nil
	}
	plan := a.archPlan()
	var out []button
	add := func(b button) {
		if b.y+b.h <= r.bottom() {
			out = append(out, b)
		}
	}
	if bv, ok := buildable(t.build); ok && plan.stuffTop > 0 {
		sel := a.toolStuff(t)
		for i, s := range bv.Stuffs {
			row, col := i/plan.stuffPerRow, i%plan.stuffPerRow
			label := stuffLabel(s)
			if bv.Carpet && a.renderer != nil {
				label = fmt.Sprintf("%s (%d)", label, stockOf(a.shell.World(), s))
			}
			add(button{
				id: ids.PrefixArchStuff + s, label: label,
				x: r.X + archPad + float32(col)*(plan.stuffW+archBtnGap),
				y: r.Y + plan.stuffTop + float32(row)*(archBtnH+archBtnGap),
				w: plan.stuffW, h: archBtnH, selected: sel == s,
			})
		}
	}
	if plan.cropTop > 0 {
		sel := a.selectedCrop().Name
		for i, c := range zoneSpecies() {
			col, row := i%plan.cropCols, i/plan.cropCols
			b := button{
				id: ids.PrefixArchCrop + c.Name, label: cropLabel(c.Name),
				x: r.X + archPad + float32(col)*(plan.cropW+archBtnGap),
				y: r.Y + plan.cropTop + float32(row)*(archBtnH+archBtnGap),
				w: plan.cropW, h: archBtnH, selected: c.Name == sel,
			}
			if p := a.lockedBy("crop:" + c.Name); p != "" {
				b.disabled, b.reason, b.selected, b.quiet = true, "Needs research: "+p, false, true
			}
			add(b)
		}
	}
	return out
}

// zoneSpecies is what a growing zone can sow (game.ZoneSpecies), read
// once: it is static and rebuilt on every call.
var zoneSpecies = sync.OnceValue(game.ZoneSpecies)
