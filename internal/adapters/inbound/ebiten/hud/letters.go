package hud

import (
	"fmt"
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strconv"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"

	ebiten "github.com/hajimehoshi/ebiten/v2"
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
	letterW    = 620 // UIS-073: the reference letter window's width
	letterMinH = 480
	// letterFresh is how old (game ticks) an intro letter may be and still
	// open by itself: a loaded save's intro is not a letter to read again.
	LetterFresh = 600
	// letterFlashFrames is how long a new letter's row is highlighted
	// (presentation frames).
	LetterFlashFrames = 90

	letterIconW = 20 // the kind icon on its tile
	letterPadX  = 8
)

// letterKinds are the kinds a letter can have: the icon, the colour.
var LetterKinds = []struct {
	Kind, Icon string
	Clr        color.RGBA
}{
	{"threat", "letter_threat", uikit.LetterThreat},
	{"caution", "letter_caution", uikit.LetterCaution},
	{"info", "letter_info", uikit.LetterInfo},
	{"good", "letter_good", uikit.LetterGood},
}

// LetterKind is a letter's kind in the table; an unknown or empty one
// reads as info.
func LetterKind(kind string) int {
	for i, k := range LetterKinds {
		if k.Kind == kind {
			return i
		}
	}
	return 2
}

// LetterColor is the colour of a letter kind.
func LetterColor(kind string) color.RGBA { return LetterKinds[LetterKind(kind)].Clr }

// LetterKey identifies a letter's text within the running session. The
// intro uses it to open once; the capture pre-marks it.
func (c Right) LetterKey(m game.MessageView) string {
	return fmt.Sprintf("letter|%d|%s", c.sh.Env().Session, messageKey(m))
}

// letters are the game's letters, oldest first.
func (c Right) letters() []game.LetterView {
	if !c.live() {
		return nil
	}
	return c.sh.World().Letters
}

// LetterCount is the number of letters the layout reserves rows for.
func (c Right) LetterCount() int { return len(c.letters()) }

// LetterRow is one row of the stack: a letter, or the "+N older" row that
// stands for the letters that did not fit.
type LetterRow struct {
	R      gfx.Rect
	Letter game.LetterView // zero for the "+N older" row
	Older  int             // > 0: the "+N older" row
	Opens  uint64          // the "+N older" row: the newest folded letter, which a click opens
}

// LetterRowLabel is the text a row shows.
func LetterRowLabel(r LetterRow) string { return LetterRowLabelIn(en, r) }

// LetterRowLabelIn is LetterRowLabel in tr's language.
func LetterRowLabelIn(tr lang.Tr, r LetterRow) string {
	if r.Older > 0 {
		return lang.Tf(tr, "+%d older", r.Older)
	}
	if r.Letter.Title == "" {
		return tr.T("Letter")
	}
	return tr.T(r.Letter.Title)
}

// LetterTextIn is a letter's body in tr's language (English: l.Text).
func LetterTextIn(tr lang.Tr, l game.LetterView) string { return lang.Msg(tr, l.Msg, l.Text) }

// LetterRows lays the stack out in l's Letters rect: the "+N older" row
// first when letters were folded, then the newest letters, oldest on top,
// the newest ending at the rect's bottom.
func (c Right) LetterRows(l hudlayout.Layout) []LetterRow {
	ls := c.letters()
	if len(ls) == 0 || l.LetterRows <= 0 || l.Letters.Empty() {
		return nil
	}
	shown, older := StackSplit(len(ls), l.LetterRows, hudlayout.MaxLetters)
	n := shown
	if older > 0 {
		n++
	}
	rows := make([]LetterRow, 0, n)
	gap := l.LetterGap // RHUD-475; zero reads as the usual gap
	if gap <= 0 {
		gap = hudlayout.LetterGap
	}
	top := l.Letters.Bottom() - float32(n)*(hudlayout.LetterRowH+gap) + gap
	at := func(i int) gfx.Rect {
		return gfx.Rect{X: l.Letters.X, Y: top + float32(i)*(hudlayout.LetterRowH+gap), W: l.Letters.W, H: hudlayout.LetterRowH}
	}
	i := 0
	if older > 0 {
		rows = append(rows, LetterRow{R: at(i), Older: older, Opens: ls[len(ls)-shown-1].ID})
		i++
	}
	for _, lt := range ls[len(ls)-shown:] {
		rows = append(rows, LetterRow{R: at(i), Letter: lt})
		i++
	}
	return rows
}

// LetterButtons are the letters as click targets, ids "letter.<ID>".
func (c Right) LetterButtons() []uikit.Button { return c.letterButtonsIn(c.sh.Layout()) }

func (c Right) letterButtonsIn(l hudlayout.Layout) []uikit.Button {
	var out []uikit.Button
	for _, row := range c.LetterRows(l) {
		id := row.Letter.ID
		if row.Older > 0 {
			id = row.Opens // RHUD-474: the bundle opens the newest folded letter
		}
		out = append(out, uikit.Button{ID: ids.PrefixLetter + strconv.FormatUint(id, 10), Label: LetterRowLabelIn(c.sh, row),
			X: row.R.X, Y: row.R.Y, W: row.R.W, H: row.R.H})
	}
	return out
}

// letterByID is the game's letter with id.
func (c Right) letterByID(id uint64) (game.LetterView, bool) {
	for _, l := range c.letters() {
		if l.ID == id {
			return l, true
		}
	}
	return game.LetterView{}, false
}

// session resets the column's presentation state when a new colony
// has started (letter ids restart with it).
func (c Right) session() *rightState {
	st := &c.right
	if st.session != c.sh.Env().Session || st.known == nil {
		*st = rightState{session: c.sh.Env().Session, known: map[uint64]bool{}, fresh: map[uint64]uint64{}, read: map[uint64]bool{}}
	}
	return st
}

// LetterRead reports the player opened letter id.
func (c Right) LetterRead(id uint64) bool { return c.session().read[id] }

// LetterFlash is the highlight of a new letter's row, 1 when it arrives
// down to 0 after letterFlashFrames (presentation clock).
func (c Right) LetterFlash(id uint64) float64 {
	if !c.live() {
		return 0
	}
	at, ok := c.session().fresh[id]
	if !ok || c.sh.Present() < at {
		return 0
	}
	age := c.sh.Present() - at
	if age >= LetterFlashFrames {
		return 0
	}
	return 1 - float64(age)/LetterFlashFrames
}

// DeclareLetters runs once per Update: it notices the letters that arrived
// since the last one (their rows flash) and opens the scenario's intro
// letter, once. No other letter opens by itself.
func (c Right) DeclareLetters() {
	if !c.live() {
		return
	}
	st := c.session()
	now := uint64(c.sh.Tick())
	for _, l := range c.letters() {
		if !st.known[l.ID] {
			st.known[l.ID] = true
			if st.primed {
				st.fresh[l.ID] = c.sh.Present()
			}
		}
		if !l.Intro {
			continue
		}
		key := c.LetterKey(game.MessageView{Tick: l.Tick, Text: l.Text})
		if c.MsgSeen == nil {
			c.MsgSeen = map[string]uint64{}
		}
		if _, seen := c.MsgSeen[key]; seen {
			continue
		}
		if l.Tick+LetterFresh < now {
			continue // a loaded save's intro stays in the stack
		}
		c.MsgSeen[key] = c.sh.Present()
		c.openLetterView(l)
	}
	st.primed = true
	c.declareEnding() // ending.go (AX-8): once the ship has left
}

// LetterClick handles a "letter.<ID>" click: it opens the letter.
func (c Right) LetterClick(id string) {
	n, err := strconv.ParseUint(strings.TrimPrefix(id, ids.PrefixLetter), 10, 64)
	if err != nil {
		return
	}
	if l, ok := c.letterByID(n); ok {
		c.openLetterView(l)
	}
}

// dismissLetter asks the game to drop letter id from the stack.
func (c Right) dismissLetter(id uint64) {
	c.sh.Enqueue(command.Command{Kind: command.DismissLetter, Target: id})
}

// letterHasPlace reports the letter can take the camera somewhere.
func letterHasPlace(l game.LetterView) bool {
	return l.Focus != 0 || (l.X >= 0 && l.Y >= 0)
}

// jumpToLetter takes the camera to the letter's creature, or else its tile.
func (c Right) jumpToLetter(l game.LetterView) {
	if c.focusEntity(l.Focus, l.Map, l.X, l.Y) {
		return
	}
	if l.X >= 0 && l.Y >= 0 {
		c.sh.Jump(ports.Spot{Map: l.Map, X: l.X, Y: l.Y})
	}
}

// OpenLetter pushes the letter window for a message that is a letter.
func (c Right) OpenLetter(m game.MessageView) {
	l := game.LetterView{Tick: m.Tick, Title: m.Title, Text: m.Text, Msg: m.Msg, Focus: m.Focus, X: -1, Y: -1}
	if m.X != 0 || m.Y != 0 {
		l.X, l.Y = m.X, m.Y
	}
	c.openLetterView(l)
}

// openLetterView pushes the letter window for l and marks it read.
func (c Right) openLetterView(l game.LetterView) {
	key := l.Title // the window id keeps the English title in every language
	if key == "" {
		key = "Letter"
	}
	title := c.sh.T(key)
	if l.ID != 0 {
		c.session().read[l.ID] = true
	}
	lines := gfx.WrapPx(LetterTextIn(c.sh, l), gfx.Body, letterW-2*uikit.ModalPad)
	h := max(letterMinH, float32(uikit.ModalTitleH+uikit.ModalFooterH+2*uikit.ModalPad)+float32(len(lines))*float32(gfx.LineH(gfx.Body))+8)
	id := fmt.Sprintf("letter.%d.%s", l.Tick, key)
	if l.ID != 0 {
		id = fmt.Sprintf("letter.%d", l.ID)
	}
	buttons := c.letterFooter(l)
	c.kit().Push(uikit.Modal{
		ID: id, Title: title, W: letterW, H: h,
		Draw: func(body gfx.Rect) {
			y := body.Y
			for _, line := range lines {
				c.kit().Label(body.X, y, line, gfx.Body)
				y += float32(gfx.LineH(gfx.Body))
			}
		},
		Buttons: buttons,
	})
}

// letterFooter is the letter window's buttons: Continue, or, for a
// rescued stranger's offer to join (SIM-063), Accept (command.AcceptLetter)
// and Decline (command.DismissLetter), or, for a demand on a caravan
// (MAP-08), Give and Refuse (command.EncounterAnswer); then "Jump to" when
// the letter has a place.
func (c Right) letterFooter(l game.LetterView) []uikit.ModalButton {
	buttons := []uikit.ModalButton{{ID: "continue", Label: c.sh.T("Continue"), Style: uikit.BtnPrimary, Closes: true}}
	if l.JoinOffer && l.ID != 0 {
		buttons = []uikit.ModalButton{
			{ID: "accept", Label: c.sh.T("Accept"), Style: uikit.BtnPrimary, Closes: true, OnClick: func() {
				c.sh.Enqueue(command.Command{Kind: command.AcceptLetter, Target: l.ID})
			}},
			{ID: "decline", Label: c.sh.T("Decline"), Closes: true, OnClick: func() { c.dismissLetter(l.ID) }},
		}
	}
	if l.Demand && l.ID != 0 {
		answer := func(mode uint8) {
			c.sh.Enqueue(command.Command{Kind: command.EncounterAnswer, Target: l.ID, Mode: mode})
		}
		buttons = []uikit.ModalButton{
			{ID: "give", Label: c.sh.T("Give"), Style: uikit.BtnPrimary, Closes: true, OnClick: func() { answer(0) }},
			{ID: "refuse", Label: c.sh.T("Refuse"), Closes: true, OnClick: func() { answer(1) }},
		}
	}
	if letterHasPlace(l) {
		buttons = append(buttons, uikit.ModalButton{ID: "jump", Label: c.sh.T("Jump to"), Closes: true, OnClick: func() { c.jumpToLetter(l) }})
	}
	return buttons
}

// LetterTile is the tile of a row: LetterW x LetterRowH, LetterRight from
// the row's right edge (UIS-031). The row itself is the click target.
func LetterTile(r gfx.Rect) gfx.Rect {
	return gfx.Rect{X: r.Right() - hudlayout.LetterRight - hudlayout.LetterW, Y: r.Y, W: hudlayout.LetterW, H: r.H}
}

// drawLetters paints the stack: an icon tile in the kind's colour with the
// label right-aligned on a dark strip left of it (UIS-031).
func (c Right) drawLetters(screen *ebiten.Image, l hudlayout.Layout) {
	rows := c.LetterRows(l)
	if len(rows) == 0 {
		return
	}
	mx, my := c.sh.Cursor()
	for _, row := range rows {
		r := row.R
		tile := LetterTile(r)
		hover := r.Hit(mx, my)
		label, text := LetterRowLabelIn(c.sh, row), uikit.Text
		clr := uikit.PanelBorder
		var k struct {
			Icon string
			Clr  color.RGBA
		}
		if row.Older > 0 {
			text = uikit.TextDim
		} else {
			kk := LetterKinds[LetterKind(row.Letter.Kind)]
			k.Icon, k.Clr, clr = kk.Icon, kk.Clr, kk.Clr
			if c.LetterRead(row.Letter.ID) {
				text = uikit.TextDim
			}
		}
		bg := uikit.Panel
		if hover {
			bg = uikit.BtnHover
		}
		uikit.FillRect(screen, tile, bg)
		if row.Older == 0 {
			uikit.FillRect(screen, tile, creature.Fade(k.Clr, 0.25))
			if f := c.LetterFlash(row.Letter.ID); f > 0 {
				uikit.FillRect(screen, tile, creature.Fade(k.Clr, 0.45*f))
			}
		}
		border := creature.Fade(clr, 0.7)
		if hover || (row.Older == 0 && c.LetterFlash(row.Letter.ID) > 0) {
			border = clr
		}
		uikit.StrokeRect(screen, tile, border)
		switch img := c.icon(k.Icon); {
		case row.Older > 0:
			gfx.Text(screen, "+", float64(tile.X)+float64(tile.W)/2-3, float64(tile.Y)+(float64(tile.H)-gfx.LineH(gfx.Body))/2, gfx.WithColor(gfx.Body, uikit.TextDim))
		case img != nil:
			gfx.Icon(screen, img, float64(tile.X+(tile.W-letterIconW)/2), float64(tile.Y+(tile.H-letterIconW)/2), letterIconW)
		default:
			gfx.FillCircle(screen, tile.X+tile.W/2, tile.Y+tile.H/2, 5, k.Clr, true) // no art: a dot in the kind's colour
		}
		label = uikit.FitText(label, gfx.Small, tile.X-r.X-hudlayout.LetterLabelRight-2*letterPadX)
		w, _ := gfx.Measure(label, gfx.Small)
		lx := float64(tile.X) - hudlayout.LetterLabelRight - w
		uikit.FillRect(screen, gfx.Rect{X: float32(lx) - 4, Y: r.Y + 4, W: float32(w) + 8, H: r.H - 8}, creature.Fade(uikit.Panel, 0.7))
		gfx.Text(screen, label, lx, textY(r, label, gfx.Small), gfx.WithColor(gfx.Small, text))
	}
}
