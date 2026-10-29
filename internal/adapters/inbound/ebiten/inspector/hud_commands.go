package inspector

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Command buttons (PLAY-018: CORE-PANE owns them): every action on the
// selection is a square button (icon, label, hotkey, disabled reason) in a
// grid right of the inspect pane, filling the bottom row first and
// wrapping upward (the layout's GridCells). Each selection kind declares
// its commands (cmds_*.go); a group selection merges its members' commands
// by key (multiselect.go). Hotkeys follow declaration order.
//
// Ids. A command keeps its legacy id ("pawn.draft", "item.forbid",
// "bldg.move", ...) and carries no Run or Cmds: its click reaches
// app.go's applyClick, which acts on the selection. A new command uses
// "cmd.<kind>.<name>": app.go sends it to applyCommandClick, which runs
// Run, else enqueues Cmds, else runs the first enabled option.
//
// A producer must not call a.shell.Layout() or the rect accessors (the layout
// calls the producers to size itself).

// cmdOption is one entry of a command's options menu (right-click, or the
// corner arrow): "now" by a colonist, a material, an owner.
type Option struct {
	Label    string
	Disabled bool
	Reason   string
	Cmd      *command.Command // enqueued when picked, else Run
	Run      func()
}

// cmdButton is one command button.
type Button struct {
	ID     string // click id; legacy ids are kept ("pawn.draft", "item.forbid", ...)
	Key    string // merge key across a group ("" = ID)
	Label  string
	Icon   string // ui_icon name
	Desc   string // tooltip body
	Hotkey input.Action

	Disabled bool
	Reason   string
	Toggle   bool // a toggle (Draft, Forbid): On says which way
	On       bool
	Mixed    bool // a merged toggle that is on for some members only

	Run     func()            // run on click, else Cmds, else applyClick(ID)
	Cmds    []command.Command // enqueued on click
	Options func() []Option
}

// mergeKey is the key a command merges under.
func (c Button) mergeKey() string {
	if c.Key != "" {
		return c.Key
	}
	return c.ID
}

// Command cell metrics (logical px).
const (
	cmdIconMax  = 32
	cmdArrowW   = 16 // the options arrow's hit area (top right)
	CmdMaxKeys  = 12
	CmdLabelPad = 3
)

// selectionCommands are the current selection's (or the Architect's)
// commands, in declaration order.
func (a Commands) SelectionCommands() []Button {
	if a.sim() == nil || a.env().WinOpen {
		return nil
	}
	selID, hasSel := a.shell.Selected()
	switch {
	case a.env().ArchOpen:
		return a.commandsFor(selection.None, 0)
	case a.sel.Grouped():
		return a.GroupCommands()
	case hasSel:
		return a.commandsFor(selection.Pawn, selID)
	case a.sel.HasAnimal:
		return a.commandsFor(selection.Animal, a.sel.SelAnimal)
	case a.sel.HasItem:
		return a.commandsFor(selection.Item, a.sel.SelItem)
	case a.sel.HasPlant:
		return a.commandsFor(selection.Plant, a.sel.SelPlant)
	case a.sel.HasBuilding:
		return a.commandsFor(selection.Building, a.sel.SelBuilding)
	case a.sel.HasFrame:
		return a.commandsFor(selection.Frame, a.sel.SelFrame)
	case a.sel.HasZone:
		return a.commandsFor(selection.Zone, entity.EntityID(a.sel.ZoneSel))
	}
	return nil
}

// commandsFor dispatches to the kind's producer (selection.None: the
// Architect's tools).
func (a Commands) commandsFor(k selection.Kind, id entity.EntityID) []Button {
	if p := a.TestCommands; p != nil {
		return p(k, id)
	}
	if p := a.Producers[k].Commands; p != nil {
		return p(id)
	}
	return nil
}

// commandCount is the number of command buttons the layout reserves.
func (a Commands) CommandCount() int { return len(a.SelectionCommands()) }

// cmdSlot is a command and the layout cell it sits in.
type cmdSlot struct {
	cmd Button
	r   gfx.Rect
	idx int // declaration index (the hotkey slot)
}

// commandCells pairs the selection's commands with the layout's cells (a
// command beyond the last cell is not shown).
func (a Commands) CommandCells() []cmdSlot {
	cmds := a.SelectionCommands()
	if len(cmds) == 0 {
		return nil
	}
	cells := a.shell.Layout().GridCells
	n := min(len(cmds), len(cells))
	out := make([]cmdSlot, n)
	for i := range n {
		out[i] = cmdSlot{cmds[i], cells[i], i}
	}
	return out
}

// cmdOptionsID is the id of command i's corner arrow.
func cmdOptionsID(i int) string { return "cmd.options." + strconv.Itoa(i) }

// cmdArrowRect is the options arrow's area in a cell (top right).
func cmdArrowRect(r gfx.Rect) gfx.Rect {
	return gfx.Rect{X: r.Right() - cmdArrowW, Y: r.Y, W: cmdArrowW, H: cmdArrowW}
}

// cellButtons are the grid's hit targets: each options arrow, then the
// cells (the arrow sits on its cell, so it comes first).
func (a Commands) cellButtons() []uikit.Button {
	cells := a.CommandCells()
	var out []uikit.Button
	for _, c := range cells {
		if c.cmd.Options != nil {
			b := uikit.AsButton(cmdArrowRect(c.r), cmdOptionsID(c.idx), "")
			b.Disabled, b.Reason, b.Quiet = c.cmd.Disabled, c.cmd.Reason, true
			out = append(out, b)
		}
	}
	for _, c := range cells {
		b := uikit.AsButton(c.r, c.cmd.ID, c.cmd.Label)
		b.Disabled, b.Reason, b.Selected, b.Quiet = c.cmd.Disabled, c.cmd.Reason, c.cmd.On, true
		out = append(out, b)
	}
	return out
}

// commandGridButtons are the command buttons as hit targets (a group's are
// groupButtons, which colonyButtons also lists).
func (a Commands) CommandGridButtons() []uikit.Button {
	if a.sel.Grouped() {
		return nil
	}
	return a.cellButtons()
}

// commandKey is the hotkey of the command at declaration index i: its own
// dedicated one, else the i-th command key (up to twelve).
func commandKey(i int, c Button) input.Action {
	switch {
	case c.Hotkey != "":
		return c.Hotkey
	case i >= 0 && i < len(input.Gizmos):
		return input.Gizmos[i]
	}
	return ""
}

// commandKeyLabel is the hotkey hint of the command at index i ("" none).
func (a Commands) CommandKeyLabel(i int, c Button) string {
	if k := commandKey(i, c); k != "" {
		return a.env().Keys.Label(k)
	}
	return ""
}

// cmdTip is a command's hover card: label and key, then its description,
// its state and, when it cannot be used, why.
func cmdTip(c Button, key string) string {
	title := c.Label
	if key != "" {
		title += "  [" + key + "]"
	}
	var body []string
	if c.Desc != "" {
		body = append(body, c.Desc)
	}
	if c.Mixed {
		body = append(body, "On for some of the selection.")
	}
	if c.Disabled && c.Reason != "" {
		body = append(body, "Unavailable: "+c.Reason)
	}
	return title + "\n" + strings.Join(body, "\n")
}

// commandGridUI declares the grid's widget-kit parts: each cell's tooltip.
func (a Commands) CommandGridUI(sw, sh int) {
	for _, c := range a.CommandCells() {
		a.kit().Tip("cmd.tip."+c.cmd.ID, c.r, cmdTip(c.cmd, a.CommandKeyLabel(c.idx, c.cmd)))
	}
}

// drawCommandGrid draws the command buttons.
func (a Commands) DrawCommandGrid(screen *ebiten.Image) {
	cells := a.CommandCells()
	if len(cells) == 0 {
		return
	}
	mx, my := a.shell.Cursor()
	for _, c := range cells {
		drawCmdCell(screen, c.cmd, c.r, c.r.Hit(mx, my), a.CommandKeyLabel(c.idx, c.cmd), a.icon(c.cmd.Icon))
	}
}

// drawCmdCell draws one 64 x 64 command button inside r.
func drawCmdCell(screen *ebiten.Image, c Button, r gfx.Rect, hover bool, key string, icon *ebiten.Image) {
	fill, border := uikit.Btn, uikit.BtnBorder
	switch {
	case c.Disabled:
		fill = uikit.BtnDisabled
	case c.On:
		fill, border = uikit.BtnSelected, uikit.Accent
	case hover:
		fill = uikit.BtnHover
	}
	uikit.FillRect(screen, r, fill)
	if c.Mixed && !c.Disabled {
		uikit.FillRect(screen, gfx.Rect{X: r.X, Y: r.Y + r.H/2, W: r.W, H: r.H / 2}, uikit.BtnSelected)
	}
	if hover && !c.Disabled && !c.On {
		border = uikit.Accent
	}
	uikit.StrokeRect(screen, r, border)

	// the label: at most two lines under the icon
	maxW := float64(r.W - 2*CmdLabelPad)
	if c.Options != nil {
		maxW -= 6 // clear of the corner arrow
	}
	ts, lines := CmdLabelLines(c.Label, maxW, c.Disabled)
	step := gfx.LineStep(ts)
	labelTop := r.Bottom() - 2 - float32(len(lines))*step
	for i, ln := range lines {
		uikit.DrawCentered(screen, gfx.Ellipsize(ln, ts, maxW), float64(r.X+r.W/2), float64(labelTop+float32(i)*step)+gfx.LineH(ts)/2-1, ts)
	}

	// the icon: centred between the top edge and the label
	area := labelTop - 1 - (r.Y + 3)
	size := min(cmdIconMax, area)
	ix, iy := r.X+(r.W-size)/2, r.Y+3+(area-size)/2
	switch {
	case icon != nil && c.Disabled:
		drawIconCS(screen, icon, float64(ix), float64(iy), float64(size), sprite.ColorScale{0.45, 0.45, 0.45, 1})
	case icon != nil:
		gfx.Icon(screen, icon, float64(ix), float64(iy), float64(size))
	default:
		drawCmdFallback(screen, c.Label, gfx.Rect{X: ix, Y: iy, W: size, H: size}, c.Disabled)
	}

	if key != "" { // hotkey hint, top left
		w := gfx.TextW(key, gfx.Small) + 4
		uikit.FillRect(screen, gfx.Rect{X: r.X + 2, Y: r.Y + 2, W: w, H: step - 2}, uikit.Space)
		gfx.Text(screen, key, float64(r.X+4), float64(r.Y+1), gfx.WithColor(gfx.Small, uikit.Accent))
	}
	if c.Options != nil { // options arrow, top right
		ar := cmdArrowRect(r)
		cx, cy := ar.Center()
		clr := uikit.TextDim
		if hover {
			clr = uikit.Accent
		}
		gfx.FillPoly(screen, [][2]float32{{cx - 4, cy - 2}, {cx + 4, cy - 2}, {cx, cy + 3}}, clr, false)
	}
}

// drawCmdFallback is a plain tile with the label's initial, for a command
// whose icon is missing from the art catalog.
func drawCmdFallback(screen *ebiten.Image, label string, r gfx.Rect, disabled bool) {
	uikit.FillRect(screen, r, uikit.BtnDisabled)
	uikit.StrokeRect(screen, r, uikit.BtnBorder)
	if label == "" {
		return
	}
	ts := gfx.Large
	if disabled {
		ts = gfx.WithColor(ts, uikit.TextDisabled)
	}
	cx, cy := r.Center()
	uikit.DrawCentered(screen, strings.ToUpper(string([]rune(label)[:1])), float64(cx), float64(cy), ts)
}

// applyCommandClick handles a "cmd." click: a corner arrow opens its
// command's options menu, any other id runs its command.
func (a Commands) ApplyCommandClick(id string) {
	if a.sim() == nil {
		return
	}
	if n, ok := strings.CutPrefix(id, "cmd.options."); ok {
		i, err := strconv.Atoi(n)
		cells := a.CommandCells()
		if err == nil && i >= 0 && i < len(cells) {
			a.openCommandOptions(cells[i])
		}
		return
	}
	for _, c := range a.SelectionCommands() {
		if c.ID != id {
			continue
		}
		if c.Disabled {
			a.shell.Hint(c.Reason)
			return
		}
		a.shell.Hint("")
		a.RunCommand(c)
		return
	}
}

// runCommand runs a command: Run, else its Cmds, else the first enabled
// option, else (a legacy id) the click handler of its id.
func (a Commands) RunCommand(c Button) {
	switch {
	case c.Run != nil:
		c.Run()
	case len(c.Cmds) > 0:
		for _, cm := range c.Cmds {
			a.shell.Enqueue(cm)
		}
	case c.Options != nil:
		for _, o := range c.Options() {
			if !o.Disabled {
				a.runOption(o)
				return
			}
		}
	case !strings.HasPrefix(c.ID, ids.PrefixCmd):
		a.shell.Click(c.ID)
	}
}

// runOption runs a picked option.
func (a Commands) runOption(o Option) {
	switch {
	case o.Disabled:
		a.shell.Hint(o.Reason)
	case o.Cmd != nil:
		a.shell.Enqueue(*o.Cmd)
	case o.Run != nil:
		o.Run()
	}
}

// pressCommandKey presses the selection's command at declaration index i:
// the i-th command key (input.Gizmos, B H Y N J U M K I L O P), whatever
// the layout does with the buttons.
func (a Commands) PressCommandKey(i int) {
	if a.sim() == nil || a.env().UIHidden || i < 0 || i >= len(input.Gizmos) {
		return
	}
	if cmds := a.SelectionCommands(); i < len(cmds) {
		a.pressCommand(cmds[i])
	}
}

// pressCommand presses command c of the selection (a hotkey): a disabled
// one shows its reason.
func (a Commands) pressCommand(c Button) {
	if c.Disabled {
		a.shell.Hint(c.Reason)
		return
	}
	a.shell.Hint("")
	a.RunCommand(c)
}

// openCommandOptions opens a cell's options menu above it.
func (a Commands) openCommandOptions(c cmdSlot) {
	if c.cmd.Options == nil || a.sim() == nil {
		return
	}
	if c.cmd.Disabled {
		a.shell.Hint(c.cmd.Reason)
		return
	}
	opts := c.cmd.Options()
	if len(opts) == 0 {
		return
	}
	entries := make([]game.OrderOption, len(opts))
	for i, o := range opts {
		entries[i] = game.OrderOption{Label: o.Label, Disabled: o.Disabled, Reason: o.Reason}
	}
	a.shell.Pick(c.r.X, c.r.Y, true, entries, func(i int) { a.runOption(opts[i]) })
}

// rightClickCommand opens a command's options under a right-click. A
// click anywhere on a cell is the HUD's, so it never reaches the map.
func (a Commands) RightClickCommand(mx, my int) bool {
	for _, c := range a.CommandCells() {
		if c.r.Hit(mx, my) {
			a.openCommandOptions(c)
			return true
		}
	}
	return false
}

// groupMemo caches a group's merged commands for one Update: merging
// asks every member's producer, and the layout asks for the commands many
// times a frame.
type groupMemo struct {
	updates uint64
	kind    selection.Kind
	ids     []entity.EntityID
	out     []Button
	ok      bool
}

// mergeCommands merges the group members' command lists (one per member)
// by key, in the order each key was first declared:
//   - Disabled only when every member that has the command is; the reason
//     is the first one, suffixed "(k of n)" (k members share it, of the n
//     that have the command);
//   - a toggle is On when all members that have it are on, Mixed when only
//     some are;
//   - Options are per member and are dropped.
//
// The merged id is "cmd.group.<key>". A click runs run(member, command)
// for each member whose command is enabled and, for a toggle, whose On
// equals the merged On (so a mixed toggle switches everyone on).
func MergeCommands(sets [][]Button, run func(member int, c Button)) []Button {
	type slot struct {
		first   Button
		members []int // member index per command
		cmds    []Button
	}
	var order []string
	byKey := map[string]*slot{}
	for m, set := range sets {
		for _, c := range set {
			k := c.mergeKey()
			s := byKey[k]
			if s == nil {
				s = &slot{first: c}
				byKey[k] = s
				order = append(order, k)
			}
			s.members = append(s.members, m)
			s.cmds = append(s.cmds, c)
		}
	}
	out := make([]Button, 0, len(order))
	for _, k := range order {
		s := byKey[k]
		m := s.first
		m.ID = "cmd.group." + strings.TrimPrefix(k, ids.PrefixCmd)
		m.Key = k
		m.Options, m.Cmds = nil, nil
		n := len(s.cmds)
		off, on := 0, 0
		reasons := map[string]int{}
		firstReason := ""
		for _, c := range s.cmds {
			if c.Disabled {
				off++
				if firstReason == "" {
					firstReason = c.Reason
				}
				reasons[c.Reason]++
			}
			if c.On {
				on++
			}
			m.Toggle = m.Toggle || c.Toggle
		}
		m.Disabled = off == n
		m.Reason = ""
		if m.Disabled {
			m.Reason = fmt.Sprintf("%s (%d of %d)", firstReason, reasons[firstReason], n)
			if firstReason == "" {
				m.Reason = fmt.Sprintf("(%d of %d)", reasons[firstReason], n)
			}
		}
		m.On = m.Toggle && on == n
		m.Mixed = m.Toggle && on > 0 && on < n
		members, cmds, toggleOn := s.members, s.cmds, m.On
		m.Run = func() {
			for i, c := range cmds {
				if c.Disabled || (c.Toggle && c.On != toggleOn) {
					continue
				}
				if run != nil {
					run(members[i], c)
				}
			}
		}
		out = append(out, m)
	}
	return out
}

// cmdLabelMinSize is the smallest a command label shrinks to before a
// word has to break.
const cmdLabelMinSize = 9

// cmdLabelLines lays out a command button's label in at most two lines of
// maxW: a single word too wide for the cell ("Deconstruct") steps the type
// down in half points rather than breaking in two.
func CmdLabelLines(label string, maxW float64, disabled bool) (gfx.TextStyle, []string) {
	ts := gfx.Small
	for ts.Size > cmdLabelMinSize && widestWord(label, ts) > maxW {
		ts.Size -= 0.5
	}
	if disabled {
		ts = gfx.WithColor(ts, uikit.TextDisabled)
	}
	lines := gfx.WrapPx(label, ts, maxW)
	if len(lines) > 2 {
		lines = []string{lines[0], strings.Join(lines[1:], " ")}
	}
	return ts, lines
}

// widestWord is the width of label's widest word in st.
func widestWord(label string, st gfx.TextStyle) float64 {
	w := 0.0
	for _, word := range strings.Fields(label) {
		if ww, _ := gfx.Measure(word, st); ww > w {
			w = ww
		}
	}
	return w
}

// Anchor is where command id's menu opens: above its button, else above
// the pointer.
func (a Commands) Anchor(id string) (x, y float32) {
	for _, c := range a.CommandCells() {
		if c.cmd.ID == id {
			return c.r.X, c.r.Y
		}
	}
	mx, my := a.shell.Cursor()
	return float32(mx), float32(my)
}
