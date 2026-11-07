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
	"rimworld-go/internal/domain/sim/command"
)

// The Architect on the colony HUD (PLAY-018: ARCHITECT owns it). At the
// lower-left, two flush category columns over a search row (ArchCats,
// UIS-053) carry a fixed description box flush above them (ArchDesc,
// UIS-054); the open category's tools, or the search's matches, are command
// buttons in the grid to their right (Cards). Placement input and the ghost
// stay in tool.go; the search row and the box's scroll are Panel's
// (desc_box.go).
//
// The box is ArchDescW x min(ArchDescH, room): Plan lays its content out
// once per change of its inputs (wrapping text is the costly part); content
// taller than the box scrolls inside it. The pickers (materials, colours,
// crops) come right under the name so they sit in view first. Nothing here
// may ask for the layout while planning; the box does not follow the
// hovered tool: the grid's tooltip shows the hovered tool.

const (
	archPad = hudlayout.ArchDescInset // the description box's text inset

	archBtnH     = 22 // a material or crop button
	archBtnGap   = 4
	archBlockGap = 4 // between blocks of the description box
	archCropCols = 4 // crop buttons per row, fewer when a name does not fit

	// InnerW is the description box's text width: the inset on the left,
	// the scrollbar's width on the right.
	InnerW = hudlayout.ArchDescW - archPad - uikit.ScrollBarW
)

// Cards are the open category's tools as command buttons, in catalog
// order. They keep the legacy click ids (arch.tool.<id>), so a click
// reaches SelectTool through the shell's click routes; the selected tool
// is On, a tool the scenario or the research forbids is disabled with
// the reason, and a placeholder (planned.go) is disabled as not in this
// game yet.
func (t *Tool) Cards() []Card {
	if !t.Open {
		return nil
	}
	tools := ToolsIn(t.Cat)
	if q := t.Panel.Query(); q != "" { // the search row: matches from every category
		tools = t.Panel.matches(q)
	}
	if len(tools) == 0 {
		return nil
	}
	var w game.WorldView
	if t.sim() != nil {
		w = t.sh.World()
	}
	out := make([]Card, 0, len(tools))
	for _, d := range tools {
		head := d.ID
		if m, ok := ToolByID(t.ToolID); ok && d.Family != "" && m.Family == d.Family {
			d = m // a family's card stands for its armed member (A2)
		}
		d = t.tx.localize(t.sh, d)
		if d.Planned { // a greyed placeholder: its sentence, and why it cannot be used
			out = append(out, Card{ID: ids.PrefixArchTool + d.ID, Label: d.Label, Icon: d.Icon, Desc: d.Desc,
				Disabled: true, Reason: t.sh.T("Not in this game yet")})
			continue
		}
		b := Card{ID: ids.PrefixArchTool + head, Label: d.Label, Icon: d.Icon, Toggle: true, Options: t.orderOptions(d.ID)}
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

// DescHeight is the description box's content height (0 while the
// Architect is closed); the box itself is the layout's ArchDesc and scrolls
// when this is taller.
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

	colourTop float32 // the carpet floor's swatches (colour.go)
}

// planKey is everything the plan depends on.
type planKey struct {
	tool, stuff, crop, rot string
	colour                 string
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
	if ColourTool(d) {
		k.colour = t.ToolColour()
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
		add(t.sh.T("Pick something to place"), dim)
		p.H = y + archPad
		return p
	}

	d = t.tx.localize(t.sh, d)
	add(d.Label, head)
	bv, built := Buildable(d.Build)
	// the pickers first, so they show without scrolling
	if built && len(bv.Stuffs) > 1 {
		y += archBlockGap
		perRow, rows, bw := archStuffGrid(bv)
		p.stuffTop, p.stuffPerRow, p.stuffW = y, perRow, bw
		y += float32(rows)*(archBtnH+archBtnGap) - archBtnGap
	}
	if ColourTool(d) { // AX-3: the colour picker, then the picked colour's name
		y += archBlockGap
		p.colourTop = y
		y += float32(colourRows())*(archSwatch+archSwatchGap) - archSwatchGap + archBlockGap
		add(ColourLabel(k.colour), gfx.Body)
	}
	if d.Crop != "" {
		y += archBlockGap
		cols, bw := archCropGrid()
		p.cropTop, p.cropCols, p.cropW = y, cols, bw
		rows := (len(ZoneSpecies()) + cols - 1) / cols
		y += float32(rows)*(archBtnH+archBtnGap) - archBtnGap
	}
	y += archBlockGap
	add(d.Desc, dim)
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
	if d.Crop != "" {
		y += archBlockGap
		for _, ln := range CropLines(t.selectedCrop(), t.sh.Env().Units) {
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
	if d.Build == "wall torch" || d.Build == "wall lamp" { // A2
		hint += "  in front of a wall: it faces away from it"
	} else if d.Build == "door" || d.Build == "powered door" || d.Build == "fence gate" || d.Build == "ornate door" || d.Build == "animal flap" {
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

// descButtons are the armed tool's material buttons, colour swatches and,
// for a growing zone, the crop picker, inside the description box r
// scrolled by off. Only the buttons wholly in view are there; the rest are
// a scroll away.
func (t *Tool) descButtons(r gfx.Rect, off float32) []uikit.Button {
	d, ok := ToolByID(t.ToolID)
	if !ok {
		return nil
	}
	plan := t.Plan()
	var out []uikit.Button
	add := func(b uikit.Button) {
		b.Y -= off
		if b.Y >= r.Y && b.Y+b.H <= r.Bottom() {
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
	if plan.colourTop > 0 {
		for _, b := range t.colourButtons(r, plan.colourTop) {
			add(b)
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
				W: plan.cropW, H: archBtnH, Selected: c.Name == sel, Tip: CropTip(c),
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

// CropTip is a crop button's hover card with the published numbers: days
// to ripen on plain, poor and rich soil, the yield per plant, the Plants
// skill needed, whether a hydroponics basin grows it, the frost that kills
// it and how long the produce keeps. Trees and other zero rows give just
// what they have.
func CropTip(c game.CropView) string {
	var b strings.Builder
	b.WriteString(CropLabel(c.Name))
	b.WriteString("\n")
	if c.Days > 0 {
		fmt.Fprintf(&b, "Ripe in %.1f days on soil (%.1f on gravel, %.1f on rich soil).", c.Days, c.GravelDay, c.RichDays)
	}
	if c.Yield > 0 {
		fmt.Fprintf(&b, " Yields %d %s per plant.", c.Yield, c.Label)
	}
	if c.MinSkill > 0 {
		fmt.Fprintf(&b, " Needs Plants %d to sow.", c.MinSkill)
	}
	if c.Hydro {
		b.WriteString(" Grows in a hydroponics basin.")
	}
	if c.FrostKill != 0 {
		fmt.Fprintf(&b, " Killed by frost below %.0f C.", float64(c.FrostKill)/10)
	}
	if c.ShelfDays > 0 {
		fmt.Fprintf(&b, " Keeps %.0f days.", c.ShelfDays)
	}
	if c.Lifespan > 0 {
		fmt.Fprintf(&b, " Lives %.0f days.", c.Lifespan)
	}
	return b.String()
}

// orderOptions is the right-click menu of an order tool (CTL-016): the
// whole map at once. Forbid and Allow flip every stack; Hunt and Tame drop
// every mark of their own (Undesignate clears just the named order).
func (t *Tool) orderOptions(id string) []CardOption {
	var label string
	c := command.Command{}
	switch id {
	case "forbid", "allow":
		label, c = "Forbid all on the map", command.Command{Kind: command.DesignateForbid, Enabled: true}
		if id == "allow" {
			label, c = "Allow all on the map", command.Command{Kind: command.DesignateForbid}
		}
	case "hunt":
		label, c = "Cancel all hunting", command.Command{Kind: command.Undesignate, Order: command.OrderHunt}
	case "tame":
		label, c = "Cancel all taming", command.Command{Kind: command.Undesignate, Order: command.OrderTame}
	default:
		return nil
	}
	w, h := t.sh.MapSize()
	if w <= 0 || h <= 0 {
		return nil
	}
	c.X2, c.Y2 = w-1, h-1
	return []CardOption{{Label: label, Cmd: c}}
}
