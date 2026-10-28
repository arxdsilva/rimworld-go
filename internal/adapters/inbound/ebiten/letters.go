package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Letters (PLAY-018, MAIN_UI §32): the game keeps every letter until the
// player dismisses it (WorldView.Letters, oldest first). They stack in the
// right column, in the layout's Letters rect: oldest at the top, the
// newest just above the global controls, at most maxLetters rows and a
// "+N older" row. A click opens the letter window (title, text, Continue
// and, when the letter has a place or a creature, "Jump to"); a
// right-click dismisses it (command.DismissLetter). Letters never open by
// themselves, except the scenario's intro, once, at the start. Whether a
// letter was read, and the highlight of a new one, are presentation only.

const (
	letterW = 560
	// letterFresh is how old (game ticks) an intro letter may be and still
	// open by itself: a loaded save's intro is not a letter to read again.
	letterFresh = 600
	// letterFlashFrames is how long a new letter's row is highlighted
	// (presentation frames).
	letterFlashFrames = 90

	letterBarW  = 4  // the kind-coloured edge on a row's left
	letterIconW = 20 // the kind icon
	letterPadX  = 8
)

// letterKinds are the kinds a letter can have: the icon, the colour.
var letterKinds = []struct {
	kind, icon string
	clr        color.RGBA
}{
	{"threat", "letter_threat", uiLetterThreat},
	{"caution", "letter_caution", uiLetterCaution},
	{"info", "letter_info", uiLetterInfo},
	{"good", "letter_good", uiLetterGood},
}

// letterKind is a letter's kind in the table; an unknown or empty one
// reads as info.
func letterKind(kind string) int {
	for i, k := range letterKinds {
		if k.kind == kind {
			return i
		}
	}
	return 2
}

// letterColor is the colour of a letter kind.
func letterColor(kind string) color.RGBA { return letterKinds[letterKind(kind)].clr }

// letterKey identifies a letter's text within the running session. The
// intro uses it to open once; the capture pre-marks it.
func (a *App) letterKey(m game.MessageView) string {
	return fmt.Sprintf("letter|%d|%s", a.simsStarted, messageKey(m))
}

// letters are the game's letters, oldest first.
func (a *App) letters() []game.LetterView {
	if a.renderer == nil {
		return nil
	}
	return a.shell.World().Letters
}

// letterCount is the number of letters the layout reserves rows for.
func (a *App) letterCount() int { return len(a.letters()) }

// letterRow is one row of the stack: a letter, or the "+N older" row that
// stands for the letters that did not fit.
type letterRow struct {
	r      rect
	letter game.LetterView // zero for the "+N older" row
	older  int             // > 0: the "+N older" row
}

// letterRowLabel is the text a row shows.
func letterRowLabel(r letterRow) string {
	if r.older > 0 {
		return fmt.Sprintf("+%d older", r.older)
	}
	if r.letter.Title == "" {
		return "Letter"
	}
	return r.letter.Title
}

// letterRows lays the stack out in l's Letters rect: the "+N older" row
// first when letters were folded, then the newest letters, oldest on top,
// the newest ending at the rect's bottom.
func (a *App) letterRows(l hudLayout) []letterRow {
	ls := a.letters()
	if len(ls) == 0 || l.LetterRows <= 0 || l.Letters.empty() {
		return nil
	}
	shown, older := stackSplit(len(ls), l.LetterRows, maxLetters)
	n := shown
	if older > 0 {
		n++
	}
	rows := make([]letterRow, 0, n)
	top := l.Letters.bottom() - float32(n)*(letterRowH+hudGap) + hudGap
	at := func(i int) rect {
		return rect{l.Letters.X, top + float32(i)*(letterRowH+hudGap), l.Letters.W, letterRowH}
	}
	i := 0
	if older > 0 {
		rows = append(rows, letterRow{r: at(i), older: older})
		i++
	}
	for _, lt := range ls[len(ls)-shown:] {
		rows = append(rows, letterRow{r: at(i), letter: lt})
		i++
	}
	return rows
}

// letterButtons are the letters as click targets, ids "letter.<ID>".
func (a *App) letterButtons() []button { return a.letterButtonsIn(a.hud()) }

func (a *App) letterButtonsIn(l hudLayout) []button {
	var out []button
	for _, row := range a.letterRows(l) {
		if row.older > 0 {
			continue
		}
		out = append(out, button{id: ids.PrefixLetter + strconv.FormatUint(row.letter.ID, 10), label: letterRowLabel(row),
			x: row.r.X, y: row.r.Y, w: row.r.W, h: row.r.H})
	}
	return out
}

// letterByID is the game's letter with id.
func (a *App) letterByID(id uint64) (game.LetterView, bool) {
	for _, l := range a.letters() {
		if l.ID == id {
			return l, true
		}
	}
	return game.LetterView{}, false
}

// rightSession resets the column's presentation state when a new colony
// has started (letter ids restart with it).
func (a *App) rightSession() *rightState {
	st := &a.hst.right
	if st.session != a.simsStarted || st.known == nil {
		*st = rightState{session: a.simsStarted, known: map[uint64]bool{}, fresh: map[uint64]uint64{}, read: map[uint64]bool{}}
	}
	return st
}

// letterRead reports the player opened letter id.
func (a *App) letterRead(id uint64) bool { return a.rightSession().read[id] }

// letterFlash is the highlight of a new letter's row, 1 when it arrives
// down to 0 after letterFlashFrames (presentation clock).
func (a *App) letterFlash(id uint64) float64 {
	if a.renderer == nil {
		return 0
	}
	at, ok := a.rightSession().fresh[id]
	if !ok || a.shell.Present() < at {
		return 0
	}
	age := a.shell.Present() - at
	if age >= letterFlashFrames {
		return 0
	}
	return 1 - float64(age)/letterFlashFrames
}

// lettersUI runs once per Update: it notices the letters that arrived
// since the last one (their rows flash) and opens the scenario's intro
// letter, once. No other letter opens by itself.
func (a *App) lettersUI() {
	if a.renderer == nil {
		return
	}
	st := a.rightSession()
	now := uint64(a.shell.Tick())
	for _, l := range a.letters() {
		if !st.known[l.ID] {
			st.known[l.ID] = true
			if st.primed {
				st.fresh[l.ID] = a.shell.Present()
			}
		}
		if !l.Intro {
			continue
		}
		key := a.letterKey(game.MessageView{Tick: l.Tick, Text: l.Text})
		if a.msgSeen == nil {
			a.msgSeen = map[string]uint64{}
		}
		if _, seen := a.msgSeen[key]; seen {
			continue
		}
		if l.Tick+letterFresh < now {
			continue // a loaded save's intro stays in the stack
		}
		a.msgSeen[key] = a.shell.Present()
		a.openLetterView(l)
	}
	st.primed = true
}

// applyLetterClick handles a "letter.<ID>" click: it opens the letter.
func (a *App) applyLetterClick(id string) {
	n, err := strconv.ParseUint(strings.TrimPrefix(id, ids.PrefixLetter), 10, 64)
	if err != nil {
		return
	}
	if l, ok := a.letterByID(n); ok {
		a.openLetterView(l)
	}
}

// dismissLetter asks the game to drop letter id from the stack.
func (a *App) dismissLetter(id uint64) {
	a.shell.Enqueue(command.Command{Kind: command.DismissLetter, Target: id})
}

// letterHasPlace reports the letter can take the camera somewhere.
func letterHasPlace(l game.LetterView) bool {
	return l.Focus != 0 || (l.X >= 0 && l.Y >= 0)
}

// jumpToLetter takes the camera to the letter's creature, or else its tile.
func (a *App) jumpToLetter(l game.LetterView) {
	if a.focusEntity(l.Focus) {
		return
	}
	if l.X >= 0 && l.Y >= 0 {
		a.centerOnMap(float64(l.X)+0.5, float64(l.Y)+0.5)
	}
}

// openLetter pushes the letter window for a message that is a letter.
func (a *App) openLetter(m game.MessageView) {
	l := game.LetterView{Tick: m.Tick, Title: m.Title, Text: m.Text, Focus: m.Focus, X: -1, Y: -1}
	if m.X != 0 || m.Y != 0 {
		l.X, l.Y = m.X, m.Y
	}
	a.openLetterView(l)
}

// openLetterView pushes the letter window for l and marks it read.
func (a *App) openLetterView(l game.LetterView) {
	title := l.Title
	if title == "" {
		title = "Letter"
	}
	if l.ID != 0 {
		a.rightSession().read[l.ID] = true
	}
	lines := wrapPx(l.Text, tsBody, letterW-2*modalPad)
	h := float32(modalTitleH+modalFooterH+2*modalPad) + float32(len(lines))*float32(lineH(tsBody)) + 8
	id := fmt.Sprintf("letter.%d.%s", l.Tick, title)
	if l.ID != 0 {
		id = fmt.Sprintf("letter.%d", l.ID)
	}
	buttons := []modalButton{{id: "continue", label: "Continue", style: btnPrimary, closes: true}}
	if letterHasPlace(l) {
		buttons = append(buttons, modalButton{id: "jump", label: "Jump to", closes: true, onClick: func() { a.jumpToLetter(l) }})
	}
	a.uiPushModal(modal{
		id: id, title: title, w: letterW, h: h,
		draw: func(a *App, body rect) {
			y := body.Y
			for _, line := range lines {
				a.uiLabel(body.X, y, line, tsBody)
				y += float32(lineH(tsBody))
			}
		},
		buttons: buttons,
	})
}

// drawLettersIn paints the stack.
func (a *App) drawLettersIn(screen *ebiten.Image, l hudLayout) {
	rows := a.letterRows(l)
	if len(rows) == 0 {
		return
	}
	mx, my := a.currentCursor()
	for _, row := range rows {
		r := row.r
		if row.older > 0 {
			fillRect(screen, r, uiPanel)
			strokeRect(screen, r, fade(uiPanelBorder, 0.7))
			drawText(screen, letterRowLabel(row), float64(r.X)+letterPadX+letterBarW, float64(r.Y)+(float64(r.H)-lineH(tsBody))/2, withColor(tsBody, uiTextDim))
			continue
		}
		k := letterKinds[letterKind(row.letter.Kind)]
		hover := r.hit(mx, my)
		bg := uiPanel
		if hover {
			bg = uiBtnHover
		}
		fillRect(screen, r, bg)
		if f := a.letterFlash(row.letter.ID); f > 0 {
			fillRect(screen, r, fade(k.clr, 0.45*f))
		}
		fillRect(screen, rect{r.X, r.Y, letterBarW, r.H}, k.clr)
		border := uiPanelBorder
		if hover || a.letterFlash(row.letter.ID) > 0 {
			border = k.clr
		}
		strokeRect(screen, r, border)
		ix, iy := r.X+letterBarW+letterPadX, r.Y+(r.H-letterIconW)/2
		if img := a.icon(k.icon); img != nil {
			drawIcon(screen, img, float64(ix), float64(iy), letterIconW)
		} else {
			uiFillCircle(screen, ix+letterIconW/2, iy+letterIconW/2, 5, k.clr, true) // no art: a dot in the kind's colour
		}
		tx := ix + letterIconW + letterPadX
		text := uiText
		if a.letterRead(row.letter.ID) {
			text = uiTextDim
		}
		drawText(screen, fitText(letterRowLabel(row), tsBody, r.right()-tx-letterPadX), float64(tx), float64(r.Y)+(float64(r.H)-lineH(tsBody))/2, withColor(tsBody, text))
	}
}
