package architect

import (
	"fmt"
	"strings"
	"sync"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// The Architect on the colony HUD (PLAY-018: ARCHITECT owns it). At the
// lower-left, two category columns (ArchCats) carry a description box
// above them (ArchDesc); the open category's tools are command buttons in
// the grid to their right (Cards). Placement input and the ghost stay in
// tool.go.
//
// The description box is content-sized: Plan measures it once per change
// of its inputs (the layout asks for its height every frame, and wrapping
// text is the costly part), and DescHeight reports that height to the
// layout. Nothing here may ask for the layout; the box does not follow the
// hovered tool (its height would then depend on the grid, which depends on
// the layout): the grid's tooltip shows the hovered tool.

const (
	archPad    = 8 // inside the category panel and the description box
	archCatW   = 96
	archCatH   = 28
	archCatGap = 4

	archBtnH     = 22 // a material or crop button
	archBtnGap   = 4
	archBlockGap = 4 // between blocks of the description box
	archCropCols = 4 // crop buttons per row, fewer when a name does not fit

	// InnerW is the description box's text width.
	InnerW = hudlayout.ArchDescW - 2*archPad
)

// Cards are the open category's tools as command buttons, in catalog
// order. They keep the legacy click ids (arch.tool.<id>), so a click
// reaches SelectTool through the shell's click routes; the selected tool
// is On and a tool the scenario or the research forbids is disabled with
// the reason.
func (t *Tool) Cards() []Card {
	if !t.Open {
		return nil
	}
	tools := ToolsIn(t.Cat)
	if len(tools) == 0 {
		return nil
	}
	var w game.WorldView
	if t.sim() != nil {
		w = t.sh.World()
	}
	out := make([]Card, 0, len(tools))
	for _, d := range tools {
		b := Card{ID: ids.PrefixArchTool + d.ID, Label: d.Label, Icon: d.Icon, Toggle: true}
		stuff := t.ToolStuff(d)
		stock := 0
		if bv, ok := Buildable(d.Build); ok && bv.Carpet {
			stock = StockOf(w, stuff)
		}
		// everything after the name: the description, then the cost lines
		b.Desc = strings.Join(TooltipLines(d, stuff, w.Wood, stock)[1:], "\n")
		if why := t.ToolLock(d); why != "" {
			b.Disabled, b.Reason = true, why
		} else {
			b.On = t.ToolID == d.ID
		}
		out = append(out, b)
	}
	return out
}

// ToolCell is the command-grid cell of tool id in the open category.
func (t *Tool) ToolCell(id string) (gfx.Rect, bool) {
	cells := t.sh.Layout().GridCells
	for i, c := range t.Cards() {
		if c.ID == ids.PrefixArchTool+id && i < len(cells) {
			return cells[i], true
		}
	}
	return gfx.Rect{}, false
}

// DescHeight is the description box's height (0 while the Architect is
// closed).
func (t *Tool) DescHeight() float32 {
	if !t.Open {
		return 0
	}
	return t.Plan().H
}

// descState is the description box measured for the inputs in key.
type descState struct {
	key  planKey
	plan DescPlan
	ok   bool

	// dflt is the category OpenPanel opened on its own, until the player
	// picks a category or a tool (clicking it then is a first choice, not a
	// collapse).
	dflt string
}

// DescLine is one wrapped line of the description box, Y from its top.
type DescLine struct {
	Y     float32
	Text  string
	Style gfx.TextStyle
}

// DescPlan is the description box laid out: its height, its text, and
// where the material and crop buttons start (0: none).
type DescPlan struct {
	H     float32
	Lines []DescLine

	stuffTop    float32
	stuffPerRow int
	stuffW      float32

	cropTop  float32
	cropCols int
	cropW    float32
}

// planKey is everything the plan depends on.
type planKey struct {
	tool, stuff, crop, rot string
	facing                 uint8
	style                  DrawStyle
	scale                  float64
	hasWorld               bool
	wood, stone, stock     int
}

func (t *Tool) key() planKey {
	k := planKey{tool: t.ToolID, facing: t.Facing, scale: gfx.HUDScale(), rot: t.rotateKeys()}
	var w game.WorldView
	if t.sim() != nil {
		w = t.sh.World()
		k.hasWorld, k.wood, k.stone = true, w.Wood, w.Stone
	}
	d, ok := ToolByID(t.ToolID)
	if !ok {
		return k
	}
	k.style = t.CurStyle(d)
	if d.Crop != "" {
		k.crop = t.selectedCrop().Name
	}
	if _, ok := Buildable(d.Build); ok {
		k.stuff = t.ToolStuff(d)
		if k.stuff != "wood" { // wood is the world's own count
			k.stock = StockOf(w, k.stuff)
		}
	}
	return k
}

// Plan is the description box for the current state, measured again only
// when its inputs change. The result is read-only.
func (t *Tool) Plan() *DescPlan {
	k := t.key()
	s := &t.desc
	if !s.ok || s.key != k {
		s.plan, s.key, s.ok = t.buildPlan(k), k, true
	}
	return &s.plan
}

// buildPlan lays out the box: the armed tool's name, description, cost,
// the stock line, its material row or crop picker, and the placement
// hints; or the stock line and a hint when nothing is armed.
func (t *Tool) buildPlan(k planKey) DescPlan {
	var p DescPlan
	head := gfx.WithColor(gfx.Label, uikit.Heading)
	dim := gfx.WithColor(gfx.Small, uikit.TextDim)
	warn := gfx.WithColor(gfx.Body, uikit.Warning)
	y := float32(archPad)
	add := func(s string, st gfx.TextStyle) {
		lines := []string{s}
		if w, _ := gfx.Measure(s, st); w > InnerW { // a line that fits keeps its spacing
			lines = gfx.WrapPx(s, st, InnerW)
		}
		for _, ln := range lines {
			p.Lines = append(p.Lines, DescLine{Y: y, Text: ln, Style: st})
			y += gfx.LineStep(st)
		}
	}
	var w game.WorldView
	if t.sim() != nil {
		w = t.sh.World()
	}
	d, ok := ToolByID(t.ToolID)
	if !ok {
		if k.hasWorld {
			add(StockLine(w, w.Wood, ""), gfx.Body)
		}
		add("Pick something to place", dim)
		p.H = y + archPad
		return p
	}

	add(d.Label, head)
	add(d.Desc, dim)
	bv, built := Buildable(d.Build)
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
		for _, ln := range TooltipLines(d, k.stuff, w.Wood, stock)[2:] {
			st := gfx.Body
			if strings.HasPrefix(ln, "Only ") {
				st = warn
			}
			add(ln, st)
		}
	}
	if k.hasWorld {
		y += archBlockGap
		add(StockLine(w, w.Wood, stuff), gfx.Body)
	}
	if built && len(bv.Stuffs) > 1 {
		y += archBlockGap
		perRow, rows, bw := archStuffGrid(bv)
		p.stuffTop, p.stuffPerRow, p.stuffW = y, perRow, bw
		y += float32(rows)*(archBtnH+archBtnGap) - archBtnGap
	}
	if d.Crop != "" {
		y += archBlockGap
		cols, bw := archCropGrid()
		p.cropTop, p.cropCols, p.cropW = y, cols, bw
		rows := (len(ZoneSpecies()) + cols - 1) / cols
		y += float32(rows)*(archBtnH+archBtnGap) - archBtnGap + archBlockGap
		for _, ln := range CropLines(t.selectedCrop()) {
			add(ln, dim)
		}
	}
	y += archBlockGap
	add(t.hint(d), dim)
	add("Right-click: stop placing", dim)
	p.H = y + archPad
	return p
}

// hint is how to place tool d: the drag shape, the rotate keys, the draw
// style.
func (t *Tool) hint(d ToolDef) string {
	hint := map[PlaceMode]string{
		PlaceSingle: "Click to place",
		PlaceLine:   "Drag for a line",
		PlaceFill:   "Drag to fill",
		PlaceArea:   "Drag an area",
	}[d.Mode]
	if d.Build == "door" || d.Build == "fence gate" {
		hint += "  faces along the wall or fence; " + t.rotateKeys() + " sets a free-standing one (" + FacingNames[t.Facing] + ")"
	} else if bv, ok := Buildable(d.Build); ok && bv.Rotatable {
		hint += "  " + t.rotateKeys() + " rotate (" + FacingNames[t.Facing] + ")"
	}
	if len(ToolStyles(d)) > 1 {
		hint = StyleNames[t.CurStyle(d)] + "  " + t.rotateKeys() + " shape, Ctrl square"
	}
	return hint
}

// CropLabel is a crop button's text.
func CropLabel(name string) string { return strings.ToUpper(name[:1]) + name[1:] }

// archCropGrid is the crop picker's columns and button width: four across
// unless a name does not fit its button.
func archCropGrid() (cols int, bw float32) {
	for cols = archCropCols; cols > 1; cols-- {
		bw = (InnerW - archBtnGap*float32(cols-1)) / float32(cols)
		fits := true
		for _, c := range ZoneSpecies() {
			fits = fits && gfx.TextW(CropLabel(c.Name), gfx.Body)+8 <= bw
		}
		if fits {
			return cols, bw
		}
	}
	return cols, (InnerW - archBtnGap*float32(cols-1)) / float32(cols)
}

// descButtons are the armed tool's material buttons and, for a growing
// zone, the crop picker, inside the description box r. Buttons the box
// was too small to hold are left out.
func (t *Tool) descButtons(r gfx.Rect) []uikit.Button {
	d, ok := ToolByID(t.ToolID)
	if !ok {
		return nil
	}
	plan := t.Plan()
	var out []uikit.Button
	add := func(b uikit.Button) {
		if b.Y+b.H <= r.Bottom() {
			out = append(out, b)
		}
	}
	if bv, ok := Buildable(d.Build); ok && plan.stuffTop > 0 {
		sel := t.ToolStuff(d)
		live := t.sim() != nil
		for i, s := range bv.Stuffs {
			row, col := i/plan.stuffPerRow, i%plan.stuffPerRow
			label := StuffLabel(s)
			if bv.Carpet && live {
				label = fmt.Sprintf("%s (%d)", label, StockOf(t.sh.World(), s))
			}
			add(uikit.Button{
				ID: ids.PrefixArchStuff + s, Label: label,
				X: r.X + archPad + float32(col)*(plan.stuffW+archBtnGap),
				Y: r.Y + plan.stuffTop + float32(row)*(archBtnH+archBtnGap),
				W: plan.stuffW, H: archBtnH, Selected: sel == s,
			})
		}
	}
	if plan.cropTop > 0 {
		sel := t.selectedCrop().Name
		for i, c := range ZoneSpecies() {
			col, row := i%plan.cropCols, i/plan.cropCols
			b := uikit.Button{
				ID: ids.PrefixArchCrop + c.Name, Label: CropLabel(c.Name),
				X: r.X + archPad + float32(col)*(plan.cropW+archBtnGap),
				Y: r.Y + plan.cropTop + float32(row)*(archBtnH+archBtnGap),
				W: plan.cropW, H: archBtnH, Selected: c.Name == sel,
			}
			if p := t.sh.LockedBy("crop:" + c.Name); p != "" {
				b.Disabled, b.Reason, b.Selected, b.Quiet = true, "Needs research: "+p, false, true
			}
			add(b)
		}
	}
	return out
}

// ZoneSpecies is what a growing zone can sow (game.ZoneSpecies), read
// once: it is static and rebuilt on every call.
var ZoneSpecies = sync.OnceValue(game.ZoneSpecies)
