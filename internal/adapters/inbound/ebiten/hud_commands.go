package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
// A producer must not call a.hud() or the rect accessors (the layout
// calls the producers to size itself).

// cmdOption is one entry of a command's options menu (right-click, or the
// corner arrow): "now" by a colonist, a material, an owner.
type cmdOption struct {
	Label    string
	Disabled bool
	Reason   string
	Cmd      *command.Command // enqueued when picked, else Run
	Run      func(a *App)
}

// cmdButton is one command button.
type cmdButton struct {
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

	Run     func(a *App)      // run on click, else Cmds, else applyClick(ID)
	Cmds    []command.Command // enqueued on click
	Options func(a *App) []cmdOption
}

// mergeKey is the key a command merges under.
func (c cmdButton) mergeKey() string {
	if c.Key != "" {
		return c.Key
	}
	return c.ID
}

// cmdsState is this region's presentation state (App.hst).
type cmdsState struct {
	memo groupMemo

	// testProducer replaces the per-kind command producers (tests only);
	// selNone stands for the Architect's tools.
	testProducer func(a *App, k selKind, id entity.EntityID) []cmdButton
}

// Command cell metrics (logical px).
const (
	cmdIconMax  = 32
	cmdArrowW   = 16 // the options arrow's hit area (top right)
	cmdMaxKeys  = 12
	cmdLabelPad = 3
)

// selectionCommands are the current selection's (or the Architect's)
// commands, in declaration order.
func (a *App) selectionCommands() []cmdButton {
	if a.renderer == nil || a.workOpen {
		return nil
	}
	switch {
	case a.archOpen:
		return a.commandsFor(selNone, 0)
	case a.grouped():
		return a.groupCommands()
	case a.renderer.HasSelect():
		return a.commandsFor(selPawn, a.renderer.Selected())
	case a.hasAnimal:
		return a.commandsFor(selAnimal, a.selAnimal)
	case a.hasItem:
		return a.commandsFor(selItem, a.selItem)
	case a.hasPlant:
		return a.commandsFor(selPlant, a.selPlant)
	case a.hasBuilding:
		return a.commandsFor(selBuilding, a.selBuilding)
	case a.hasFrame:
		return a.commandsFor(selFrame, a.selFrame)
	}
	return nil
}

// commandsFor dispatches to the kind's producer (selNone: the Architect).
func (a *App) commandsFor(k selKind, id entity.EntityID) []cmdButton {
	if p := a.hst.cmds.testProducer; p != nil {
		return p(a, k, id)
	}
	switch k {
	case selNone:
		return a.archToolCommands()
	case selPawn:
		return a.pawnCommands(id)
	case selAnimal:
		return a.animalCommands(id)
	case selItem:
		return a.itemCommands(id)
	case selPlant:
		return a.plantCommands(id)
	case selBuilding:
		return a.buildingCommands(id)
	case selFrame:
		return a.frameCommands(id)
	}
	return nil
}

// commandCount is the number of command buttons the layout reserves.
func (a *App) commandCount() int { return len(a.selectionCommands()) }

// cmdSlot is a command and the layout cell it sits in.
type cmdSlot struct {
	cmd cmdButton
	r   rect
	idx int // declaration index (the hotkey slot)
}

// commandCells pairs the selection's commands with the layout's cells (a
// command beyond the last cell is not shown).
func (a *App) commandCells() []cmdSlot {
	cmds := a.selectionCommands()
	if len(cmds) == 0 {
		return nil
	}
	cells := a.hud().GridCells
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
func cmdArrowRect(r rect) rect { return rect{r.right() - cmdArrowW, r.Y, cmdArrowW, cmdArrowW} }

// cellButtons are the grid's hit targets: each options arrow, then the
// cells (the arrow sits on its cell, so it comes first).
func (a *App) cellButtons() []button {
	cells := a.commandCells()
	var out []button
	for _, c := range cells {
		if c.cmd.Options != nil {
			b := cmdArrowRect(c.r).asButton(cmdOptionsID(c.idx), "")
			b.disabled, b.reason, b.quiet = c.cmd.Disabled, c.cmd.Reason, true
			out = append(out, b)
		}
	}
	for _, c := range cells {
		b := c.r.asButton(c.cmd.ID, c.cmd.Label)
		b.disabled, b.reason, b.selected, b.quiet = c.cmd.Disabled, c.cmd.Reason, c.cmd.On, true
		out = append(out, b)
	}
	return out
}

// commandGridButtons are the command buttons as hit targets (a group's are
// groupButtons, which colonyButtons also lists).
func (a *App) commandGridButtons() []button {
	if a.grouped() {
		return nil
	}
	return a.cellButtons()
}

// commandKey is the hotkey of the command at declaration index i: its own
// dedicated one, else the i-th command key (up to twelve).
func commandKey(i int, c cmdButton) input.Action {
	switch {
	case c.Hotkey != "":
		return c.Hotkey
	case i >= 0 && i < len(input.Gizmos):
		return input.Gizmos[i]
	}
	return ""
}

// commandKeyLabel is the hotkey hint of the command at index i ("" none).
func (a *App) commandKeyLabel(i int, c cmdButton) string {
	if k := commandKey(i, c); k != "" {
		return a.bindings().Label(k)
	}
	return ""
}

// cmdTip is a command's hover card: label and key, then its description,
// its state and, when it cannot be used, why.
func cmdTip(c cmdButton, key string) string {
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
func (a *App) commandGridUI(sw, sh int) {
	for _, c := range a.commandCells() {
		a.uiTip("cmd.tip."+c.cmd.ID, c.r, cmdTip(c.cmd, a.commandKeyLabel(c.idx, c.cmd)))
	}
}

// drawCommandGrid draws the command buttons.
func (a *App) drawCommandGrid(screen *ebiten.Image) {
	cells := a.commandCells()
	if len(cells) == 0 {
		return
	}
	mx, my := a.currentCursor()
	for _, c := range cells {
		drawCmdCell(screen, c.cmd, c.r, c.r.hit(mx, my), a.commandKeyLabel(c.idx, c.cmd), a.icon(c.cmd.Icon))
	}
}

// drawCmdCell draws one 64 x 64 command button inside r.
func drawCmdCell(screen *ebiten.Image, c cmdButton, r rect, hover bool, key string, icon *ebiten.Image) {
	fill, border := uiBtn, uiBtnBorder
	switch {
	case c.Disabled:
		fill = uiBtnDisabled
	case c.On:
		fill, border = uiBtnSelected, uiAccent
	case hover:
		fill = uiBtnHover
	}
	fillRect(screen, r, fill)
	if c.Mixed && !c.Disabled {
		fillRect(screen, rect{r.X, r.Y + r.H/2, r.W, r.H / 2}, uiBtnSelected)
	}
	if hover && !c.Disabled && !c.On {
		border = uiAccent
	}
	strokeRect(screen, r, border)

	// the label: at most two lines under the icon
	maxW := float64(r.W - 2*cmdLabelPad)
	if c.Options != nil {
		maxW -= 6 // clear of the corner arrow
	}
	ts, lines := cmdLabelLines(c.Label, maxW, c.Disabled)
	step := lineStep(ts)
	labelTop := r.bottom() - 2 - float32(len(lines))*step
	for i, ln := range lines {
		drawCentered(screen, ellipsize(ln, ts, maxW), float64(r.X+r.W/2), float64(labelTop+float32(i)*step)+lineH(ts)/2-1, ts)
	}

	// the icon: centred between the top edge and the label
	area := labelTop - 1 - (r.Y + 3)
	size := min(cmdIconMax, area)
	ix, iy := r.X+(r.W-size)/2, r.Y+3+(area-size)/2
	switch {
	case icon != nil && c.Disabled:
		drawIconCS(screen, icon, float64(ix), float64(iy), float64(size), colorScale{0.45, 0.45, 0.45, 1})
	case icon != nil:
		drawIcon(screen, icon, float64(ix), float64(iy), float64(size))
	default:
		drawCmdFallback(screen, c.Label, rect{ix, iy, size, size}, c.Disabled)
	}

	if key != "" { // hotkey hint, top left
		w := textW(key, tsSmall) + 4
		fillRect(screen, rect{r.X + 2, r.Y + 2, w, step - 2}, uiSpace)
		drawText(screen, key, float64(r.X+4), float64(r.Y+1), withColor(tsSmall, uiAccent))
	}
	if c.Options != nil { // options arrow, top right
		ar := cmdArrowRect(r)
		cx, cy := ar.center()
		clr := uiTextDim
		if hover {
			clr = uiAccent
		}
		uiFillPoly(screen, [][2]float32{{cx - 4, cy - 2}, {cx + 4, cy - 2}, {cx, cy + 3}}, clr, false)
	}
}

// drawCmdFallback is a plain tile with the label's initial, for a command
// whose icon is missing from the art catalog.
func drawCmdFallback(screen *ebiten.Image, label string, r rect, disabled bool) {
	fillRect(screen, r, uiBtnDisabled)
	strokeRect(screen, r, uiBtnBorder)
	if label == "" {
		return
	}
	ts := tsLarge
	if disabled {
		ts = withColor(ts, uiTextDisabled)
	}
	cx, cy := r.center()
	drawCentered(screen, strings.ToUpper(string([]rune(label)[:1])), float64(cx), float64(cy), ts)
}

// applyCommandClick handles a "cmd." click: a corner arrow opens its
// command's options menu, any other id runs its command.
func (a *App) applyCommandClick(id string) {
	if a.renderer == nil {
		return
	}
	if n, ok := strings.CutPrefix(id, "cmd.options."); ok {
		i, err := strconv.Atoi(n)
		cells := a.commandCells()
		if err == nil && i >= 0 && i < len(cells) {
			a.openCommandOptions(cells[i])
		}
		return
	}
	for _, c := range a.selectionCommands() {
		if c.ID != id {
			continue
		}
		if c.Disabled {
			a.hint = c.Reason
			return
		}
		a.hint = ""
		a.runCommand(c)
		return
	}
}

// runCommand runs a command: Run, else its Cmds, else the first enabled
// option, else (a legacy id) the click handler of its id.
func (a *App) runCommand(c cmdButton) {
	switch {
	case c.Run != nil:
		c.Run(a)
	case len(c.Cmds) > 0:
		for _, cm := range c.Cmds {
			a.renderer.enqueue(cm)
		}
	case c.Options != nil:
		for _, o := range c.Options(a) {
			if !o.Disabled {
				a.runOption(o)
				return
			}
		}
	case !strings.HasPrefix(c.ID, "cmd."):
		a.applyClick(c.ID)
	}
}

// runOption runs a picked option.
func (a *App) runOption(o cmdOption) {
	switch {
	case o.Disabled:
		a.hint = o.Reason
	case o.Cmd != nil:
		a.renderer.enqueue(*o.Cmd)
	case o.Run != nil:
		o.Run(a)
	}
}

// pressCommandKey presses the selection's command at declaration index i:
// the i-th command key (input.Gizmos, B H Y N J U M K I L O P), whatever
// the layout does with the buttons.
func (a *App) pressCommandKey(i int) {
	if a.renderer == nil || a.hideUI || i < 0 || i >= len(input.Gizmos) {
		return
	}
	if cmds := a.selectionCommands(); i < len(cmds) {
		a.pressCommand(cmds[i])
	}
}

// pressCommand presses command c of the selection (a hotkey): a disabled
// one shows its reason.
func (a *App) pressCommand(c cmdButton) {
	if c.Disabled {
		a.hint = c.Reason
		return
	}
	a.hint = ""
	a.runCommand(c)
}

// openCommandOptions opens a cell's options menu above it.
func (a *App) openCommandOptions(c cmdSlot) {
	if c.cmd.Options == nil || a.renderer == nil {
		return
	}
	if c.cmd.Disabled {
		a.hint = c.cmd.Reason
		return
	}
	opts := c.cmd.Options(a)
	if len(opts) == 0 {
		return
	}
	entries := make([]game.OrderOption, len(opts))
	for i, o := range opts {
		entries[i] = game.OrderOption{Label: o.Label, Disabled: o.Disabled, Reason: o.Reason}
	}
	a.openPick(c.r.X, c.r.Y, true, entries, func(i int) { a.runOption(opts[i]) })
}

// rightClickCommand opens a command's options under a right-click. A
// click anywhere on a cell is the HUD's, so it never reaches the map.
func (a *App) rightClickCommand(mx, my int) bool {
	for _, c := range a.commandCells() {
		if c.r.hit(mx, my) {
			a.openCommandOptions(c)
			return true
		}
	}
	return false
}

// hintAnchor is where disabled-reason hints and toasts sit: their left
// edge and the bottom they rest on (the layout's: just above the command
// buttons or the pane, else just above the tab strip).
func (a *App) hintAnchor() (x, bottom float32) {
	l := a.hud()
	x, bottom = l.HintX, l.HintBottom
	if lines := a.cellInfoLines(); len(lines) > 0 { // nothing selected: rest above the cell info
		bottom = min(bottom, cellInfoBox(lines, l.Tabs.Y).Y-hudMargin)
	}
	return x, bottom
}

// groupMemo caches a group's merged commands for one Update: merging
// asks every member's producer, and the layout asks for the commands many
// times a frame.
type groupMemo struct {
	updates uint64
	kind    selKind
	ids     []entity.EntityID
	out     []cmdButton
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
func mergeCommands(sets [][]cmdButton, run func(a *App, member int, c cmdButton)) []cmdButton {
	type slot struct {
		first   cmdButton
		members []int // member index per command
		cmds    []cmdButton
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
	out := make([]cmdButton, 0, len(order))
	for _, k := range order {
		s := byKey[k]
		m := s.first
		m.ID = "cmd.group." + strings.TrimPrefix(k, "cmd.")
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
		m.Run = func(a *App) {
			for i, c := range cmds {
				if c.Disabled || (c.Toggle && c.On != toggleOn) {
					continue
				}
				if run != nil {
					run(a, members[i], c)
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
func cmdLabelLines(label string, maxW float64, disabled bool) (textStyle, []string) {
	ts := tsSmall
	for ts.Size > cmdLabelMinSize && widestWord(label, ts) > maxW {
		ts.Size -= 0.5
	}
	if disabled {
		ts = withColor(ts, uiTextDisabled)
	}
	lines := wrapPx(label, ts, maxW)
	if len(lines) > 2 {
		lines = []string{lines[0], strings.Join(lines[1:], " ")}
	}
	return ts, lines
}

// widestWord is the width of label's widest word in st.
func widestWord(label string, st textStyle) float64 {
	w := 0.0
	for _, word := range strings.Fields(label) {
		if ww, _ := measure(word, st); ww > w {
			w = ww
		}
	}
	return w
}
