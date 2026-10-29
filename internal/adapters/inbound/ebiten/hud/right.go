package hud

import (
	"fmt"
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The right column (PLAY-018: HUD-RIGHT owns it): the alerts below the
// learning helper (alerts.go), the letter stack above the global controls
// (letters.go) and the messages under the colonist bar (messages.go),
// each in its rect of the HUD layout. This file draws and routes them
// together, and owns the hover cards that explain a row.

// Draw draws the messages, the gathering banner, the alerts and
// the letters, then the hover card over them.
func (c Right) Draw(screen *ebiten.Image, sw, sh int) {
	if !c.live() {
		return
	}
	l := c.sh.Layout()
	c.DrawMessagesIn(screen, l)
	c.Banner().DrawIn(screen, sw, l) // gathering.go (ENG-003)
	c.Alerts().DrawIn(screen, l)
	c.drawLetters(screen, l)
	if card, anchor, ok := c.Hover(l); ok {
		drawRightCard(screen, card, anchor, sw, sh)
	}
}

// Buttons are the alerts', letters' and messages' buttons.
func (c Right) Buttons(sw int) []uikit.Button {
	if !c.live() || c.Hidden {
		return nil
	}
	l := c.sh.Layout()
	out := c.Alerts().ButtonsIn(l)
	out = append(out, c.letterButtonsIn(l)...)
	return append(out, c.messageButtonsIn(l)...)
}

// alertCount, letterCount and messageCount are the rows the layout
// reserves room for. They run while the layout is being computed, so they
// must never ask for it.
func (c Alerts) Count() int {
	if !c.live() {
		return 0
	}
	return len(c.Current())
}

func (c Right) MessageCount() int { return len(c.liveMessageList()) }

// RightClick answers a right-click over the column: on a
// letter it dismisses it, on an alert it centres the previous target. A
// right-click anywhere else in the column's rects is taken and does
// nothing, so it never reaches the map beneath.
func (c Right) RightClick(mx, my int) bool {
	if !c.live() {
		return false
	}
	l := c.sh.Layout()
	for _, row := range c.Alerts().Rows(l) {
		if row.R.Hit(mx, my) {
			if row.More == 0 {
				c.Alerts().cycle(row.Al.Key, -1)
			}
			return true
		}
	}
	for _, row := range c.LetterRows(l) {
		if row.R.Hit(mx, my) {
			if row.Older == 0 {
				c.dismissLetter(row.Letter.ID)
			}
			return true
		}
	}
	for _, r := range []gfx.Rect{l.Alerts, l.Letters, l.Messages} {
		if !r.Empty() && r.Hit(mx, my) {
			return true
		}
	}
	return false
}

// Declare declares the column's widget-kit parts: none, its rows
// are plain hit buttons and its cards are drawn by drawRightColumn.
func (c Right) Declare(sw, sh int) {}

// rightState is this region's presentation state (Chrome): which letters
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

// RightCard is the text of a card: a title, a body, and the colour of its
// left edge.
type RightCard struct {
	Title, Body string
	Clr         color.RGBA
}

// Hover is the card for the row under the cursor and the row it
// explains (false: none). Alerts say what is wrong and what to do; the
// "+N" rows list what they hide; a letter previews its text.
func (c Right) Hover(l hudlayout.Layout) (RightCard, gfx.Rect, bool) {
	if !c.live() || c.Hidden || c.kit().ModalOpen() {
		return RightCard{}, gfx.Rect{}, false
	}
	mx, my := c.sh.Cursor()
	for _, row := range c.Alerts().Rows(l) {
		if !row.R.Hit(mx, my) {
			continue
		}
		if row.More > 0 {
			lines := make([]string, 0, len(row.Hidden))
			for i, h := range row.Hidden {
				if i == 8 {
					lines = append(lines, "...")
					break
				}
				lines = append(lines, h.Label)
			}
			return RightCard{Title: fmt.Sprintf("%d more alerts", row.More), Body: strings.Join(lines, "\n"), Clr: uikit.PanelBorder}, row.R, true
		}
		al := row.Al
		body := al.Explain
		if n := len(al.Targets); n > 1 {
			body += fmt.Sprintf("\n\nClick to see the next of %d places, right-click for the previous one.", n)
		} else if n == 1 {
			body += "\n\nClick to look at it."
		}
		clr := uikit.Warning
		if al.Sev == SevDanger {
			clr = uikit.Danger
		}
		return RightCard{Title: al.Label, Body: body, Clr: clr}, row.R, true
	}
	for _, row := range c.LetterRows(l) {
		if !row.R.Hit(mx, my) {
			continue
		}
		if row.Older > 0 {
			return RightCard{Title: fmt.Sprintf("%d older letters", row.Older),
				Body: "Right-click a letter to dismiss it and bring an older one back.", Clr: uikit.PanelBorder}, row.R, true
		}
		text := row.Letter.Text
		if r := []rune(text); len(r) > 240 {
			text = string(r[:240]) + "..."
		}
		return RightCard{Title: LetterRowLabel(row), Body: text + "\n\nClick to read it, right-click to dismiss it.",
			Clr: LetterColor(row.Letter.Kind)}, row.R, true
	}
	return RightCard{}, gfx.Rect{}, false
}

// LayoutRightCard wraps the card's text and places it to the left of the
// row it explains, level with the row and clamped inside the sw x sh
// screen. It returns the box and the wrapped title and body lines.
func LayoutRightCard(c RightCard, anchor gfx.Rect, sw, sh int) (box gfx.Rect, title, body []string) {
	avail := anchor.X - 2*hudlayout.HudMargin
	maxW := min(float32(rightCardMaxW), max(avail, rightCardMinW))
	inner := float64(maxW - 2*rightCardPad - rightCardBar)
	title = gfx.WrapPx(c.Title, gfx.Label, inner)
	if strings.TrimSpace(c.Body) != "" {
		body = gfx.WrapPx(c.Body, gfx.Small, inner)
	}
	var w, h float64
	for _, ln := range title {
		lw, _ := gfx.Measure(ln, gfx.Label)
		w, h = max(w, lw), h+gfx.LineH(gfx.Label)
	}
	for _, ln := range body {
		lw, _ := gfx.Measure(ln, gfx.Small)
		w, h = max(w, lw), h+gfx.LineH(gfx.Small)
	}
	if len(body) > 0 {
		h += rightCardGap
	}
	bw, bh := float32(w)+2*rightCardPad+rightCardBar, float32(h)+2*rightCardPad
	x := min(anchor.X-hudlayout.HudMargin-bw, float32(sw)-hudlayout.HudMargin-bw)
	x = max(x, hudlayout.HudMargin)
	y := min(anchor.Y, float32(sh)-hudlayout.HudMargin-bh)
	y = max(y, hudlayout.HudMargin)
	return gfx.Rect{X: x, Y: y, W: bw, H: bh}, title, body
}

// drawRightCard paints a card beside anchor.
func drawRightCard(screen *ebiten.Image, c RightCard, anchor gfx.Rect, sw, sh int) {
	box, title, body := LayoutRightCard(c, anchor, sw, sh)
	uikit.FillRect(screen, box, uikit.Tooltip)
	uikit.StrokeRect(screen, box, uikit.TooltipEdge)
	uikit.FillRect(screen, gfx.Rect{X: box.X, Y: box.Y, W: rightCardBar, H: box.H}, c.Clr)
	x, y := float64(box.X+rightCardBar+rightCardPad), float64(box.Y+rightCardPad)
	for _, ln := range title {
		gfx.Text(screen, ln, x, y, gfx.WithColor(gfx.Label, uikit.Heading))
		y += gfx.LineH(gfx.Label)
	}
	if len(body) > 0 {
		y += rightCardGap
		for _, ln := range body {
			gfx.Text(screen, ln, x, y, gfx.WithColor(gfx.Small, uikit.Text))
			y += gfx.LineH(gfx.Small)
		}
	}
}
