package ebiten

import (
	"fmt"

	"rimworld-go/internal/application/game"
)

// Letters (NG-1 §2.11): a message with Letter set (the scenario's intro
// after the landing, a heat wave or a cold snap) opens a letter window once,
// with a Continue button. The game keeps running behind it. There is no
// colony message history to add the letter to: the messages the HUD lists
// are the last few, and fade after ten seconds.

const (
	letterW = 560
	// letterFresh is how old (game ticks) a letter message may be and still
	// open: a loaded save's saved messages are not letters to read again.
	letterFresh = 600
)

// letterKey identifies a letter message within the running session.
func (a *App) letterKey(m game.MessageView) string {
	return fmt.Sprintf("letter|%d|%s", a.simsStarted, messageKey(m))
}

// lettersUI opens a window for every fresh letter not shown yet.
func (a *App) lettersUI() {
	if a.renderer == nil {
		return
	}
	now := uint64(a.renderer.game.Tick())
	for _, m := range a.renderer.world().Messages {
		if !m.Letter || m.Tick+letterFresh < now {
			continue
		}
		key := a.letterKey(m)
		if a.msgSeen == nil {
			a.msgSeen = map[string]uint64{}
		}
		if _, seen := a.msgSeen[key]; seen {
			continue
		}
		a.msgSeen[key] = a.renderer.present
		a.openLetter(m)
	}
}

// openLetter pushes the letter window for m.
func (a *App) openLetter(m game.MessageView) {
	title := m.Title
	if title == "" {
		title = "Letter"
	}
	lines := wrapPx(m.Text, tsBody, letterW-2*modalPad)
	h := float32(modalTitleH+modalFooterH+2*modalPad) + float32(len(lines))*float32(lineH(tsBody)) + 8
	a.uiPushModal(modal{
		id: fmt.Sprintf("letter.%d.%s", m.Tick, title), title: title, w: letterW, h: h,
		draw: func(a *App, body rect) {
			y := body.Y
			for _, l := range lines {
				a.uiLabel(body.X, y, l, tsBody)
				y += float32(lineH(tsBody))
			}
		},
		buttons: []modalButton{{id: "continue", label: "Continue", style: btnPrimary, closes: true}},
	})
}
