package ebiten

import (
	"fmt"
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The right column (PLAY-018: HUD-RIGHT owns it): the alerts below the
// learning helper (alerts.go), the letter stack above the global controls
// (letters.go) and the messages under the colonist bar (climate_ui.go),
// each in its rect of the HUD layout. This file draws and routes them
// together, and owns the hover cards that explain a row.

// drawRightColumn draws the messages, the gathering banner, the alerts and
// the letters, then the hover card over them.
func (a *App) drawRightColumn(screen *ebiten.Image, sw, sh int) {
	if a.renderer == nil {
		return
	}
	l := a.hud()
	a.drawMessagesIn(screen, l)
	a.drawGatheringBannerIn(screen, sw, l) // gathering_ui.go (ENG-003)
	a.drawAlertsIn(screen, l)
	a.drawLettersIn(screen, l)
	if card, anchor, ok := a.rightHover(l); ok {
		drawRightCard(screen, card, anchor, sw, sh)
	}
}

// rightColumnButtons are the alerts', letters' and messages' buttons.
func (a *App) rightColumnButtons(sw int) []button {
	if a.renderer == nil || a.hideUI {
		return nil
	}
	l := a.hud()
	out := a.alertButtonsIn(l)
	out = append(out, a.letterButtonsIn(l)...)
	return append(out, a.messageButtonsIn(l)...)
}

// alertCount, letterCount and messageCount are the rows the layout
// reserves room for. They run while the layout is being computed, so they
// must never ask for it.
func (a *App) alertCount() int {
	if a.renderer == nil {
		return 0
	}
	return len(a.currentAlerts())
}

func (a *App) messageCount() int { return len(a.liveMessageList()) }

// rightClickRightColumn answers a right-click over the column: on a
// letter it dismisses it, on an alert it centres the previous target. A
// right-click anywhere else in the column's rects is taken and does
// nothing, so it never reaches the map beneath.
func (a *App) rightClickRightColumn(mx, my int) bool {
	if a.renderer == nil {
		return false
	}
	l := a.hud()
	for _, row := range a.alertRows(l) {
		if row.r.hit(mx, my) {
			if row.more == 0 {
				a.cycleAlert(row.al.key, -1)
			}
			return true
		}
	}
	for _, row := range a.letterRows(l) {
		if row.r.hit(mx, my) {
			if row.older == 0 {
				a.dismissLetter(row.letter.ID)
			}
			return true
		}
	}
	for _, r := range []rect{l.Alerts, l.Letters, l.Messages} {
		if !r.empty() && r.hit(mx, my) {
			return true
		}
	}
	return false
}

// rightColumnUI declares the column's widget-kit parts: none, its rows
// are plain hit buttons and its cards are drawn by drawRightColumn.
func (a *App) rightColumnUI(sw, sh int) {}

// rightState is this region's presentation state (App.hst): which letters
// the stack has seen (a new one flashes) and which the player opened. It
// restarts with each colony (rightSession).
type rightState struct {
	session int
	primed  bool              // the first Update of the session has seen the stack
	known   map[uint64]bool   // letters the stack has seen
	fresh   map[uint64]uint64 // new letter -> presentation frame it arrived
	read    map[uint64]bool   // letters the player opened
}

// ---- hover cards ------------------------------------------------------

const (
	rightCardMaxW = 296
	rightCardMinW = 120
	rightCardPad  = 8
	rightCardGap  = 2 // between title and body
	rightCardBar  = 3
)

// rightCard is the text of a card: a title, a body, and the colour of its
// left edge.
type rightCard struct {
	title, body string
	clr         color.RGBA
}

// rightHover is the card for the row under the cursor and the row it
// explains (false: none). Alerts say what is wrong and what to do; the
// "+N" rows list what they hide; a letter previews its text.
func (a *App) rightHover(l hudLayout) (rightCard, rect, bool) {
	if a.renderer == nil || a.hideUI || a.uiModalOpen() {
		return rightCard{}, rect{}, false
	}
	mx, my := a.currentCursor()
	for _, row := range a.alertRows(l) {
		if !row.r.hit(mx, my) {
			continue
		}
		if row.more > 0 {
			lines := make([]string, 0, len(row.hidden))
			for i, h := range row.hidden {
				if i == 8 {
					lines = append(lines, "...")
					break
				}
				lines = append(lines, h.label)
			}
			return rightCard{title: fmt.Sprintf("%d more alerts", row.more), body: strings.Join(lines, "\n"), clr: uiPanelBorder}, row.r, true
		}
		al := row.al
		body := al.explain
		if n := len(al.targets); n > 1 {
			body += fmt.Sprintf("\n\nClick to see the next of %d places, right-click for the previous one.", n)
		} else if n == 1 {
			body += "\n\nClick to look at it."
		}
		clr := uiWarning
		if al.sev == sevDanger {
			clr = uiDanger
		}
		return rightCard{title: al.label, body: body, clr: clr}, row.r, true
	}
	for _, row := range a.letterRows(l) {
		if !row.r.hit(mx, my) {
			continue
		}
		if row.older > 0 {
			return rightCard{title: fmt.Sprintf("%d older letters", row.older),
				body: "Right-click a letter to dismiss it and bring an older one back.", clr: uiPanelBorder}, row.r, true
		}
		text := row.letter.Text
		if r := []rune(text); len(r) > 240 {
			text = string(r[:240]) + "..."
		}
		return rightCard{title: letterRowLabel(row), body: text + "\n\nClick to read it, right-click to dismiss it.",
			clr: letterColor(row.letter.Kind)}, row.r, true
	}
	return rightCard{}, rect{}, false
}

// layoutRightCard wraps the card's text and places it to the left of the
// row it explains, level with the row and clamped inside the sw x sh
// screen. It returns the box and the wrapped title and body lines.
func layoutRightCard(c rightCard, anchor rect, sw, sh int) (box rect, title, body []string) {
	avail := anchor.X - 2*hudMargin
	maxW := min(float32(rightCardMaxW), max(avail, rightCardMinW))
	inner := float64(maxW - 2*rightCardPad - rightCardBar)
	title = wrapPx(c.title, tsLabel, inner)
	if strings.TrimSpace(c.body) != "" {
		body = wrapPx(c.body, tsSmall, inner)
	}
	var w, h float64
	for _, ln := range title {
		lw, _ := measure(ln, tsLabel)
		w, h = max(w, lw), h+lineH(tsLabel)
	}
	for _, ln := range body {
		lw, _ := measure(ln, tsSmall)
		w, h = max(w, lw), h+lineH(tsSmall)
	}
	if len(body) > 0 {
		h += rightCardGap
	}
	bw, bh := float32(w)+2*rightCardPad+rightCardBar, float32(h)+2*rightCardPad
	x := min(anchor.X-hudMargin-bw, float32(sw)-hudMargin-bw)
	x = max(x, hudMargin)
	y := min(anchor.Y, float32(sh)-hudMargin-bh)
	y = max(y, hudMargin)
	return rect{x, y, bw, bh}, title, body
}

// drawRightCard paints a card beside anchor.
func drawRightCard(screen *ebiten.Image, c rightCard, anchor rect, sw, sh int) {
	box, title, body := layoutRightCard(c, anchor, sw, sh)
	fillRect(screen, box, uiTooltip)
	strokeRect(screen, box, uiTooltipEdge)
	fillRect(screen, rect{box.X, box.Y, rightCardBar, box.H}, c.clr)
	x, y := float64(box.X+rightCardBar+rightCardPad), float64(box.Y+rightCardPad)
	for _, ln := range title {
		drawText(screen, ln, x, y, withColor(tsLabel, uiHeading))
		y += lineH(tsLabel)
	}
	if len(body) > 0 {
		y += rightCardGap
		for _, ln := range body {
			drawText(screen, ln, x, y, withColor(tsSmall, uiText))
			y += lineH(tsSmall)
		}
	}
}
