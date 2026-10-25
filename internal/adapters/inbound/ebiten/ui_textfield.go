package ebiten

import (
	"sort"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Text field (NG-1 §1.4), single and multi-line. A click focuses it and
// puts the caret under the cursor; typing goes through the CtxText input
// context (handleTextInput -> uiTextInput), which swallows every binding,
// so Esc and Enter are read as raw keys. The field edits the bound string
// live; Esc restores the value it had at focus time, Enter (single line),
// Tab or a click elsewhere commits.

const (
	fieldPad    = 6
	fieldPadTop = 4
	blinkFrames = 30
)

// textFieldSpec describes a text field.
type textFieldSpec struct {
	Multiline bool
	MaxLen    int               // in runes; 0 = unlimited
	Valid     func(r rune) bool // accepted characters; nil = any printable
	// Placeholder shows dim while the field is empty and not edited.
	Placeholder string
	// Normalize runs on commit (trim, default a blank value, ...).
	Normalize func(string) string
	Disabled  bool
	Reason    string // tooltip while disabled
	Tip       string // hover card: "Title\nbody"
}

// digitsOnly is a Valid function for numeric fields.
func digitsOnly(r rune) bool { return r >= '0' && r <= '9' }

// textEdit is the field being edited. Its caret and selection also mirror
// into uiState.caret, selStart and selEnd.
type textEdit struct {
	id      string
	val     *string
	spec    textFieldSpec
	orig    string // the value at focus time (Esc restores it)
	anchor  int    // the selection's fixed end; caret is the moving end
	r       rect
	seen    uint64 // the frame the widget last declared itself
	blinkAt uint64 // App.updates of the last edit (the caret shows solid)
}

// textBuf is the editable text with a caret and a selection anchor, in
// runes. Selection is [min(caret, anchor), max(caret, anchor)).
type textBuf struct {
	rs            []rune
	caret, anchor int
}

func (b *textBuf) sel() (lo, hi int) {
	lo, hi = b.caret, b.anchor
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi
}

func (b *textBuf) hasSel() bool { return b.caret != b.anchor }

// move puts the caret at to, keeping the anchor when extending.
func (b *textBuf) move(to int, extend bool) {
	b.caret = max(0, min(len(b.rs), to))
	if !extend {
		b.anchor = b.caret
	}
}

// insert replaces the selection with s, honouring maxLen (0: unlimited).
// It reports whether anything changed.
func (b *textBuf) insert(s []rune, maxLen int) bool {
	lo, hi := b.sel()
	if maxLen > 0 {
		room := maxLen - (len(b.rs) - (hi - lo))
		if room < len(s) {
			s = s[:max(0, room)]
		}
	}
	if len(s) == 0 && lo == hi {
		return false
	}
	out := make([]rune, 0, len(b.rs)-(hi-lo)+len(s))
	out = append(append(append(out, b.rs[:lo]...), s...), b.rs[hi:]...)
	b.rs = out
	b.caret = lo + len(s)
	b.anchor = b.caret
	return true
}

// backspace deletes the selection, or the rune before the caret.
func (b *textBuf) backspace() bool {
	if !b.hasSel() {
		if b.caret == 0 {
			return false
		}
		b.anchor = b.caret - 1
	}
	return b.insert(nil, 0)
}

// del deletes the selection, or the rune after the caret.
func (b *textBuf) del() bool {
	if !b.hasSel() {
		if b.caret >= len(b.rs) {
			return false
		}
		b.anchor = b.caret + 1
	}
	return b.insert(nil, 0)
}

// printable reports r is a character a field accepts from typing.
func printable(r rune) bool {
	return r >= 32 && r != 127 && !(r >= 0x80 && r < 0xA0)
}

// ---- layout of a multi-line field ---------------------------------------

// textSpan is one visual line: runes [start, end) (the newline that ends
// a hard line is not part of it).
type textSpan struct{ start, end int }

type lineCache struct {
	text  string
	w     float32
	scale float64
	spans []textSpan
}

// layoutLines breaks rs into visual lines no wider than maxW: at newlines,
// then after the last space that fits, else mid-word.
func layoutLines(rs []rune, st textStyle, maxW float64) []textSpan {
	width := func(a []rune) float64 { w, _ := measure(string(a), st); return w }
	var out []textSpan
	i := 0
	for {
		e := i
		for e < len(rs) && rs[e] != '\n' {
			e++
		}
		s := i
		for {
			if e-s <= 1 || width(rs[s:e]) <= maxW {
				out = append(out, textSpan{s, e})
				break
			}
			lo, hi := s+1, e
			for lo < hi {
				mid := (lo + hi + 1) / 2
				if width(rs[s:mid]) <= maxW {
					lo = mid
				} else {
					hi = mid - 1
				}
			}
			k := lo
			for j := k; j > s; j-- {
				if rs[j-1] == ' ' {
					k = j
					break
				}
			}
			out = append(out, textSpan{s, k})
			s = k
		}
		if e >= len(rs) {
			return out
		}
		i = e + 1
	}
}

// fieldLines is layoutLines cached per field (text, width and UI scale).
func (u *uiState) fieldLines(id string, rs []rune, w float32) []textSpan {
	if u.lines == nil {
		u.lines = map[string]lineCache{}
	}
	s := string(rs)
	if c, ok := u.lines[id]; ok && c.text == s && c.w == w && c.scale == uiScale() {
		return c.spans
	}
	spans := layoutLines(rs, tsBody, float64(w))
	u.lines[id] = lineCache{s, w, uiScale(), spans}
	return spans
}

// lineOf is the visual line the caret c sits on.
func lineOf(spans []textSpan, c int) int {
	k := 0
	for i, sp := range spans {
		if sp.start <= c {
			k = i
		}
	}
	return k
}

// nearestIndex is the rune boundary of rs closest to pixel x from its start.
func nearestIndex(rs []rune, x float64) int {
	width := func(n int) float64 { w, _ := measure(string(rs[:n]), tsBody); return w }
	i := sort.Search(len(rs)+1, func(n int) bool { return width(n) >= x })
	if i > len(rs) {
		return len(rs)
	}
	if i > 0 && x-width(i-1) < width(i)-x {
		return i - 1
	}
	return i
}

func (r rect) fieldInner() rect { return r.insetXY(fieldPad, fieldPadTop) }

// caretAt is the rune index under a cursor position in field r.
func (a *App) textCaretAt(id string, rs []rune, r rect, sp textFieldSpec, mx, my int) int {
	u := a.uiSt()
	in := r.fieldInner()
	if !sp.Multiline {
		return nearestIndex(rs, float64(float32(mx)-in.X+u.scroll[id+".x"]))
	}
	spans := u.fieldLines(id, rs, in.W)
	lh := float32(lineH(tsBody))
	k := int((float32(my) - in.Y + u.scroll[id+".y"]) / lh)
	k = max(0, min(len(spans)-1, k))
	sp2 := spans[k]
	return sp2.start + nearestIndex(rs[sp2.start:sp2.end], float64(float32(mx)-in.X))
}

// ---- editing --------------------------------------------------------------

func (u *uiState) buf() textBuf {
	e := &u.edit
	return textBuf{rs: []rune(*e.val), caret: u.caret, anchor: e.anchor}
}

// store writes b back to the field's value and the mirrored caret state.
func (a *App) textStore(b textBuf, changed bool) {
	u := a.uiSt()
	e := &u.edit
	if changed {
		*e.val = string(b.rs)
		u.fire(e.id, evChanged)
	}
	e.anchor = b.anchor
	u.caret = b.caret
	u.selStart, u.selEnd = b.sel()
	e.blinkAt = a.updates
	a.textReveal()
}

// textStart begins editing: caret at pos (<0: the end), everything
// selected when all is set. Another field being edited is committed.
func (a *App) textStart(id string, val *string, sp textFieldSpec, r rect, pos int, all bool) {
	u := a.uiSt()
	if u.edit.id != "" && u.edit.id != id {
		a.textEnd(true)
	}
	n := len([]rune(*val))
	if pos < 0 || pos > n {
		pos = n
	}
	u.edit = textEdit{id: id, val: val, spec: sp, orig: *val, r: r, seen: u.frame, blinkAt: a.updates}
	u.focus = id
	b := textBuf{rs: []rune(*val), caret: pos, anchor: pos}
	if all {
		b.caret, b.anchor = n, 0
	}
	a.textStore(b, false)
}

// textEnd stops editing: commit keeps the value (after Normalize), else it
// is restored to what it was at focus time.
func (a *App) textEnd(commit bool) {
	u := a.uiSt()
	e := u.edit
	if e.id == "" {
		return
	}
	u.edit = textEdit{}
	u.active = ""
	u.caret, u.selStart, u.selEnd = 0, 0, 0
	if commit {
		if e.spec.Normalize != nil {
			if n := e.spec.Normalize(*e.val); n != *e.val {
				*e.val = n
				u.fire(e.id, evChanged)
			}
		}
		u.fire(e.id, evCommit)
		return
	}
	if *e.val != e.orig {
		*e.val = e.orig
		u.fire(e.id, evChanged)
	}
	u.fire(e.id, evCancel)
}

// textReveal scrolls the field so the caret shows.
func (a *App) textReveal() {
	u := a.uiSt()
	e := &u.edit
	if e.id == "" || e.val == nil {
		return
	}
	if u.scroll == nil {
		u.scroll = map[string]float32{}
	}
	in := e.r.fieldInner()
	rs := []rune(*e.val)
	if !e.spec.Multiline {
		cx, _ := measure(string(rs[:min(u.caret, len(rs))]), tsBody)
		tw, _ := measure(*e.val, tsBody)
		sx := u.scroll[e.id+".x"]
		switch {
		case tw <= float64(in.W):
			sx = 0
		case float32(cx)-sx > in.W-2:
			sx = float32(cx) - in.W + 2
		case float32(cx) < sx:
			sx = float32(cx)
		}
		u.scroll[e.id+".x"] = max(0, sx)
		return
	}
	spans := u.fieldLines(e.id, rs, in.W)
	lh := float32(lineH(tsBody))
	k := float32(lineOf(spans, u.caret))
	sy := u.scroll[e.id+".y"]
	switch {
	case k*lh < sy:
		sy = k * lh
	case (k+1)*lh > sy+in.H:
		sy = (k+1)*lh - in.H
	}
	u.scroll[e.id+".y"] = max(0, sy)
}

func (a *App) metaHeld() bool {
	for _, k := range a.in.raw.held {
		if k == "Meta" {
			return true
		}
	}
	return false
}

// uiTextInput feeds the field being edited (called from handleTextInput
// while CtxText is on top).
func (a *App) uiTextInput() {
	u := a.uiSt()
	e := &u.edit
	if e.id == "" {
		return
	}
	if e.seen != u.frame { // its widget was not declared: the field is gone
		a.textEnd(true)
		return
	}
	mx, my := a.currentCursor()
	if a.in.raw.press[mbLeft] && !e.r.hit(mx, my) { // click elsewhere commits
		a.textEnd(true)
		return
	}
	shift := a.in.shift()
	cmd := a.in.ctrl() || a.metaHeld()
	b := u.buf()
	changed := false
	if !cmd {
		for _, r := range a.in.raw.chars {
			if printable(r) && (e.spec.Valid == nil || e.spec.Valid(r)) {
				changed = b.insert([]rune{r}, e.spec.MaxLen) || changed
			}
		}
	}
	switch {
	case a.in.key("Escape"):
		a.textEnd(false)
		return
	case a.in.key("Enter") || a.in.key("NumpadEnter"):
		if !e.spec.Multiline {
			a.textStore(b, changed)
			a.textEnd(true)
			return
		}
		changed = b.insert([]rune{'\n'}, e.spec.MaxLen) || changed
	case a.in.key("Tab"):
		a.textStore(b, changed)
		a.textEnd(true)
		u.cycleFocus(map[bool]int{false: 1, true: -1}[shift])
		return
	case cmd && a.in.key("A"):
		b.anchor, b.caret = 0, len(b.rs)
	case a.keyRepeat("Backspace"):
		changed = b.backspace() || changed
	case a.keyRepeat("Delete"):
		changed = b.del() || changed
	case a.keyRepeat("ArrowLeft"):
		a.textArrow(&b, -1, shift)
	case a.keyRepeat("ArrowRight"):
		a.textArrow(&b, 1, shift)
	case e.spec.Multiline && a.keyRepeat("ArrowUp"):
		a.textVertical(&b, -1, shift)
	case e.spec.Multiline && a.keyRepeat("ArrowDown"):
		a.textVertical(&b, 1, shift)
	case a.in.key("Home"):
		b.move(a.textLineEdge(b, false), shift)
	case a.in.key("End"):
		b.move(a.textLineEdge(b, true), shift)
	}
	a.textStore(b, changed)
}

// textArrow moves the caret one rune; without Shift a selection first
// collapses to its near edge.
func (a *App) textArrow(b *textBuf, dir int, extend bool) {
	if b.hasSel() && !extend {
		lo, hi := b.sel()
		if dir < 0 {
			b.move(lo, false)
		} else {
			b.move(hi, false)
		}
		return
	}
	b.move(b.caret+dir, extend)
}

// textLineEdge is the start or end of the caret's visual line.
func (a *App) textLineEdge(b textBuf, end bool) int {
	e := &a.uiSt().edit
	if !e.spec.Multiline {
		if end {
			return len(b.rs)
		}
		return 0
	}
	spans := a.uiSt().fieldLines(e.id, b.rs, e.r.fieldInner().W)
	sp := spans[lineOf(spans, b.caret)]
	if end {
		return sp.end
	}
	return sp.start
}

// textVertical moves the caret to the same x on the line above or below.
func (a *App) textVertical(b *textBuf, dir int, extend bool) {
	u := a.uiSt()
	e := &u.edit
	spans := u.fieldLines(e.id, b.rs, e.r.fieldInner().W)
	k := lineOf(spans, b.caret)
	switch {
	case k+dir < 0:
		b.move(0, extend)
		return
	case k+dir >= len(spans):
		b.move(len(b.rs), extend)
		return
	}
	cur, tgt := spans[k], spans[k+dir]
	x, _ := measure(string(b.rs[cur.start:min(b.caret, cur.end)]), tsBody)
	b.move(tgt.start+nearestIndex(b.rs[tgt.start:tgt.end], x), extend)
}

// uiTextField is a text field bound to *val. It reports the value changed
// since its last call and that an edit was committed (Enter, Tab or a
// click elsewhere; Esc restores the value and reports changed).
func (a *App) uiTextField(id string, r rect, val *string, sp textFieldSpec) (changed, committed bool) {
	u := a.uiSt()
	tip := sp.Tip
	if sp.Disabled && sp.Reason != "" {
		tip = sp.Reason
	}
	w := uiWidget{id: id, r: r, focusable: true, disabled: sp.Disabled, reason: sp.Reason, tip: tip}
	w.click = func() {
		if !sp.Disabled {
			a.textStart(id, val, sp, r, -1, false)
		}
	}
	selID := id + ".sel"
	if u.add(w) && !sp.Disabled {
		editing := u.edit.id == id
		if editing {
			u.edit.val, u.edit.spec, u.edit.r, u.edit.seen = val, sp, r, u.frame
		}
		mx, my := a.currentCursor()
		switch {
		case a.uiPress(r):
			pos := a.textCaretAt(id, []rune(*val), r, sp, mx, my)
			if !editing {
				a.textStart(id, val, sp, r, pos, false)
			} else {
				b := u.buf()
				b.move(pos, a.in.shift())
				a.textStore(b, false)
			}
			u.active = selID
		case u.tabTo == id && !editing:
			u.tabTo = ""
			a.textStart(id, val, sp, r, -1, true)
		}
		if u.active == selID && u.edit.id == id {
			b := u.buf()
			b.move(a.textCaretAt(id, b.rs, r, sp, mx, my), true)
			a.textStore(b, false)
			if !a.in.down(mbLeft) {
				u.active = ""
				a.uiRelease()
			}
		}
	}
	ev := u.take(id)
	u.queue(func(dst *ebiten.Image) { a.drawTextField(dst, id, r, *val, sp) })
	return ev&evChanged != 0, ev&evCommit != 0
}

func (a *App) drawTextField(dst *ebiten.Image, id string, r rect, s string, sp textFieldSpec) {
	u := a.uiSt()
	editing := u.edit.id == id
	border := uiFieldBorder
	switch {
	case editing || u.focus == id && u.kbd:
		border = uiFieldFocus
	case u.hot == id && !sp.Disabled:
		border = uiBtnBorder
	}
	fillRect(dst, r, uiField)
	strokeRect(dst, r, border)
	in := r.fieldInner()
	clip := clipTo(dst, in)
	st := tsBody
	if sp.Disabled {
		st.Color = uiTextDisabled
	}
	lh := float32(lineH(tsBody))
	if s == "" && !editing {
		ph := tsBody
		ph.Color = uiTextDisabled
		drawText(clip, sp.Placeholder, float64(in.X), float64(in.Y+max(0, (in.H-lh)/2)), ph)
		return
	}
	rs := []rune(s)
	sx, sy := u.scroll[id+".x"], u.scroll[id+".y"]
	var spans []textSpan
	switch {
	case sp.Multiline:
		spans = u.fieldLines(id, rs, in.W)
	default:
		spans = []textSpan{{0, len(rs)}}
		sy = 0
	}
	ty := in.Y
	if !sp.Multiline {
		ty = r.Y + (r.H-lh)/2
	}
	if !sp.Multiline {
		sy = 0
	} else {
		sx = 0
	}
	lo, hi := u.selStart, u.selEnd
	for i, ln := range spans {
		y := ty + float32(i)*lh - sy
		if y+lh < in.Y || y > in.bottom() {
			continue
		}
		x0 := in.X - sx
		if editing && hi > lo && hi > ln.start && lo < ln.end+1 {
			a0, a1 := max(lo, ln.start), min(hi, ln.end)
			xa, _ := measure(string(rs[ln.start:a0]), tsBody)
			xb, _ := measure(string(rs[ln.start:max(a0, a1)]), tsBody)
			if hi > ln.end && ln.end < len(rs) { // the selected newline: a nub
				xb += 4
			}
			sel := uiBtnSelected
			sel.A = 0xC0
			fillRect(clip, rect{x0 + float32(xa), y, float32(xb - xa), lh}, sel)
		}
		drawText(clip, string(rs[ln.start:ln.end]), float64(x0), float64(y), st)
	}
	if !editing {
		return
	}
	blink := a.settings.Accessibility.ReducedMotion || ((a.updates-u.edit.blinkAt)/blinkFrames)%2 == 0
	if !blink {
		return
	}
	k := lineOf(spans, u.caret)
	ln := spans[k]
	cx, _ := measure(string(rs[ln.start:max(ln.start, min(u.caret, ln.end))]), tsBody)
	cy := ty + float32(k)*lh - sy
	x := in.X - sx + float32(cx)
	uiLine(clip, x, cy+1, x, cy+lh-1, 1, uiAccent, false)
}
