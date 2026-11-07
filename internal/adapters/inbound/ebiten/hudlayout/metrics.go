package hudlayout

// The colony HUD's sizes in logical pixels at UI step 1. A size with a row
// id is the reference game's measured size (`.agents/gaps/UI_SCALE_GAP_ANALYSIS.md`);
// one marked "ours" has no reference measure. TestMetricsMatchReference pins
// each reference size to its row.

// Spacing with no reference distance (ours).
const (
	HudMargin = 8
	HudGap    = 4
)

// The main tab bar (UIS-040, UIS-042, UIS-043).
const (
	TabsH = 34
	// IconTabShare is an icon tab's width as a share of a text tab's.
	IconTabShare = 0.5
	// TabSep is the separator between two tabs.
	TabSep = 2
	// TabLabelX is the label's left inset as a share of the tab's width,
	// TabBaseline its baseline below the bar's top.
	TabLabelX   = 0.148
	TabBaseline = 20
)

// The inspect pane, its tab row and its contents (UIS-044 to UIS-049).
const (
	PaneW = 432
	PaneH = 165
	// PaneTabsH is the tab row above the pane; a pawn pane has PawnPaneTabs
	// equal tabs across its width, other kinds one PaneTabW tab on the right.
	PaneTabsH    = 30
	PawnPaneTabs = 6
	PaneTabW     = 70
	PaneTabGap   = 2
	// PaneInset is the text inset; the name's baseline is PaneNameBaseline
	// below the pane's top (UIS-046).
	PaneInset        = 12
	PaneNameBaseline = 28
	// The status bars under the name (UIS-047): four of PaneBarW x PaneBarH,
	// PaneBarGap apart, their top PaneBarTop below the pane's top.
	PaneBarW   = 93
	PaneBarH   = 16
	PaneBarGap = 6
	PaneBarTop = 36
	// PaneLineH is a plain body line in the pane (UIS-048).
	PaneLineH = 16

	CardMinH = 100 // ours: the floating card never shrinks below this
	// CardPaneMinH is the docked card's shortest pane (config.HUDPaneMinH).
	CardPaneMinH = 120

	// An open tab's panel, on the tab row: ours bound the reference sizes
	// (PaneTabSize), the widest 630.
	ExpandMinW = 400
	ExpandMaxW = 640
)

// PaneTabSize is an open pawn tab's panel size (UIS-049), by tab: "log",
// "gear", "social", "bio", "needs" or "health". ok is false for any other.
func PaneTabSize(tab string) (w, h float32, ok bool) {
	switch tab {
	case "log":
		return 630, 510, true
	case "gear":
		return 460, 450, true
	case "social":
		return 540, 510, true
	case "bio":
		return 514, 489, true
	case "needs":
		return 580, 520, true
	case "health":
		return 630, 430, true
	}
	return 0, 0, false
}

// Command buttons and the Architect's designators (UIS-050 to UIS-055).
const (
	CmdCell   = 75
	CmdStride = 80
	// CmdGridGap is the pane's right edge to the first cell, CmdBottom the
	// bottom row's clearance above the tab bar.
	CmdGridGap = 14
	CmdBottom  = 16
	// CmdReserveCols are the command columns a wide card pane leaves free (ours).
	CmdReserveCols = 5
	// CmdInset is the hotkey letter's inset; CmdIconMax the icon art's size
	// (UIS-051). The label is drawn inside the cell: its block's bottom
	// CmdLabelPadB above the cell's bottom (ours: text never leaves its box);
	// a two-line label shrinks the icon, never below CmdIconMin (ours).
	CmdInset     = 6
	CmdIconMax   = 40
	CmdLabelPadB = 2
	CmdIconMin   = 24

	// The Architect: category buttons ArchCatW x ArchCatH, two per row,
	// flush, a search row of ArchSearchH under them, the description box
	// ArchDescW x ArchDescH flush on top, the designators ArchGridGap right of
	// the categories.
	ArchCatW    = 100
	ArchCatH    = 32
	ArchCatCols = 2
	ArchCatRows = 6
	ArchSearchH = 28
	ArchCatsW   = ArchCatCols * ArchCatW
	ArchDescW   = 200
	ArchDescH   = 270
	ArchGridGap = 10
	// ArchDescInset is the description's text inset.
	ArchDescInset = 8
)

// ArchCatsH is the category panel's height for n categories (ArchCatRows
// rows when n is 0).
func ArchCatsH(n int) float32 {
	rows := ArchCatRows
	if n > 0 {
		rows = (n + ArchCatCols - 1) / ArchCatCols
	}
	return float32(rows*ArchCatH + ArchSearchH)
}

// The right column: alerts, letters, the lower-right controls (UIS-030,
// UIS-031, UIS-034, UIS-056 to UIS-058).
const (
	// RightW is the column's layout and hit width (ours); rows draw as wide
	// as their text, to the left of it if they must.
	RightW      = 232
	MinimapSize = 180 // ours

	AlertRowH = 22
	// AlertRight is the alert text's right inset.
	AlertRight = 7
	// AlertLetterGap is the last alert row to the first letter;
	// RightStackGap the lowest alert or letter to the controls under them.
	AlertLetterGap = 13
	RightStackGap  = 16

	// Letters: LetterW x LetterRowH tiles, LetterGap apart, LetterRight from
	// the window's right edge; the label's right end LetterLabelRight from it.
	LetterW          = 38
	LetterRowH       = 30
	LetterGap        = 12
	LetterRight      = 12
	LetterLabelRight = 3
	MaxLetters       = 8 // ours (UIS-033): then a "+N older" row

	// MinAlertRows are kept for alerts before the letters take the rest of
	// the column, MinLetterRows for the newest letters before the learning
	// helper unfolds (ours).
	MinAlertRows  = 3
	MinLetterRows = 2

	// The lower-right controls: icons GlobalIcon at GlobalIconPitch,
	// GlobalPerRow to a row, GlobalRight from the window's right edge and
	// GlobalBottom above the tab bar; SpeedN speed buttons SpeedW x SpeedH,
	// flush, SpeedRight from the edge, SpeedAbove over the icon rows; the
	// text rows GlobalRowPitch apart, GlobalTextRight from the edge.
	GlobalIcon      = 24
	GlobalIconPitch = 28
	GlobalPerRow    = 5
	GlobalRight     = 3
	GlobalBottom    = 6
	SpeedW          = 32
	SpeedH          = 24
	SpeedRight      = 6
	SpeedAbove      = 4
	GlobalRowPitch  = 26
	GlobalTextRight = 8
)

// The colonist bar (UIS-020 to UIS-025).
const (
	// BarSlot is the full-size portrait box, BarPitch its pitch (the gap is
	// half the slot), BarTop its top, BarNameH the name plate under it.
	BarSlot  = 48
	BarPitch = 72
	BarTop   = 21
	BarNameH = 11
	// BarReserveW is the width the bar leaves free: a row is at most the
	// screen's width minus it.
	BarReserveW = 520

	// MaxBarGroups caps the colonist bar's map groups (MAP-1): the most
	// open maps. Inputs keeps them in an array so it stays comparable.
	MaxBarGroups = 8
	barGroupPad  = 3  // ours: a group frame's inset around its slots, at full size
	barChipH     = 14 // ours: a group's label chip above its slots
	barGroupGap  = 8  // ours: between two group frames
)

// barSteps are the colonist bar's sizes: slot, pitch, the rows each allows
// and the row pitch (UIS-022). Past two rows of the smallest, the last slot
// reads "+N".
var barSteps = [...]struct {
	slot, pitch float32
	rows        int
	rowPitch    float32
}{{48, 72, 1, 0}, {37, 56, 1, 0}, {27, 41, 2, 46}}

// The resource list, top-left (ours, UIS-028).
const (
	ResourcesW   = 168
	ResourceRowH = 20
	ResourcesMin = 40 // less room than this hides the list
)

// Messages, top-left (UIS-029).
const (
	MsgX      = 140
	MsgY      = 17
	MsgRowH   = 22
	MsgStride = MsgRowH
	MsgMaxW   = 480 // ours
	MsgMinW   = 160 // ours: narrower than this, the messages hide
	MaxMsgs   = 5   // ours
)

// The cell and room readout, lower-left (UIS-059): text CellX from the
// left, lines CellPitch apart, the last baseline CellBottom above the tab bar.
const (
	CellX      = 15
	CellPitch  = 19
	CellBottom = 17
)

// Windows, menus, rows and dialogs (UIS-016, UIS-017, UIS-060, UIS-069,
// UIS-070).
const (
	// WindowPad is a main-tab window's inner margin, WindowEdge its frame.
	WindowPad  = 18
	WindowEdge = 1
	// MenuRowH is a menu or list row, TableRowH a table row.
	MenuRowH  = 29
	TableRowH = 30
	// OptionRowH is an option row's pitch, OptionCtrlH its control.
	OptionRowH  = 35
	OptionCtrlH = 30
	// FloatRowH is a float-menu row; FloatIcon its icon, FloatPadW the width
	// the menu adds to its widest text.
	FloatRowH = 29
	FloatIcon = 19
	FloatPadW = 48
	// The in-game menu, bottom-right over the tab bar: PauseW x PauseH,
	// buttons PauseBtnW x PauseBtnH at PauseBtnPitch, PauseInset inside.
	PauseW        = 450
	PauseH        = 391
	PauseBtnW     = 168
	PauseBtnH     = 44
	PauseBtnPitch = 52
	PauseInset    = 19
)

// LearnHeaderH is the learning helper's collapsed height (its header alone; ours).
const LearnHeaderH = 28
