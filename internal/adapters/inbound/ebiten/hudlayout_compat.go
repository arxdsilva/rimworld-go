package ebiten

import "rimworld-go/internal/adapters/inbound/ebiten/hudlayout"

// The colony HUD layout lives in hudlayout (ARCH-003); aliases keep root call sites unchanged until the last mover deletes this file.

type (
	hudLayout = hudlayout.Layout
	hudState  = hudlayout.Inputs
)

const (
	hudMargin     = hudlayout.HudMargin
	hudGap        = hudlayout.HudGap
	tabsH         = hudlayout.TabsH
	paneW         = hudlayout.PaneW
	paneH         = hudlayout.PaneH
	paneTabsH     = hudlayout.PaneTabsH
	expandMinW    = hudlayout.ExpandMinW
	expandMaxW    = hudlayout.ExpandMaxW
	cmdCell       = hudlayout.CmdCell
	cmdStride     = hudlayout.CmdStride
	cmdMaxRows    = hudlayout.CmdMaxRows
	rightW        = hudlayout.RightW
	minimapSize   = hudlayout.MinimapSize
	letterRowH    = hudlayout.LetterRowH
	maxLetters    = hudlayout.MaxLetters
	alertRowH     = hudlayout.AlertRowH
	minAlertRows  = hudlayout.MinAlertRows
	minLetterRows = hudlayout.MinLetterRows
	barSlotW      = hudlayout.BarSlotW
	barSlotH      = hudlayout.BarSlotH
	barGap        = hudlayout.BarGap
	resourcesW    = hudlayout.ResourcesW
	resourceRowH  = hudlayout.ResourceRowH
	resourcesMin  = hudlayout.ResourcesMin
	msgRowH       = hudlayout.MsgRowH
	msgStride     = hudlayout.MsgStride
	msgMaxW       = hudlayout.MsgMaxW
	msgMinW       = hudlayout.MsgMinW
	maxMsgs       = hudlayout.MaxMsgs
	archCatsW     = hudlayout.ArchCatsW
	archCatsH     = hudlayout.ArchCatsH
	archDescW     = hudlayout.ArchDescW
)

func computeHUDLayout(sw, sh int, st hudState) hudLayout { return hudlayout.Compute(sw, sh, st) }
func boundsOf(rs []rect) rect                            { return hudlayout.BoundsOf(rs) }
