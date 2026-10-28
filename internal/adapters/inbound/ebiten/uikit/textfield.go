package uikit

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"sort"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Text field (NG-1 §1.4), single and multi-line. A click focuses it and
// puts the caret under the cursor; typing goes through the CtxText input
// context (the host feeds TextInput), which swallows every binding,
// so Esc and Enter are read as raw keys. The field edits the bound string
// live; Esc restores the value it had at focus time, Enter (single line),
// Tab or a click elsewhere commits.

const (
	FieldPad    = 6
	fieldPadTop = 4
	blinkFrames = 30
)

// TextFieldSpec describes a text field.
type TextFieldSpec struct {
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

// DigitsOnly is a Valid function for numeric fields.
func DigitsOnly(r rune) bool { return r >= '0' && r <= '9' }

// textEdit is the field being edited. Its caret and selection also mirror
// into Kit.Caret, SelStart and SelEnd.
type textEdit struct {
	ID      string
	val     *string
	spec    TextFieldSpec
	orig    string // the value at focus time (Esc restores it)
	anchor  int    // the selection's fixed end; caret is the moving end
	r       gfx.Rect
	seen    uint64 // the frame the widget last declared itself
	blinkAt uint64 // Host.Updates of the last edit (the caret shows solid)
}

// TextBuf is the editable text with a caret and a selection anchor, in
// runes. Selection is [min(caret, anchor), max(caret, anchor)).
type TextBuf struct {
	Rs            []rune
	Caret, anchor int
}

func (b *TextBuf) Sel() (lo, hi int) {
	lo, hi = b.Caret, b.anchor
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi
}

func (b *TextBuf) hasSel() bool { return b.Caret != b.anchor }

// Move puts the caret at to, keeping the anchor when extending.
func (b *TextBuf) Move(to int, extend bool) {
	b.Caret = max(0, min(len(b.Rs), to))
	if !extend {
		b.anchor = b.Caret
	}
}

// Insert replaces the selection with s, honouring maxLen (0: unlimited).
// It reports whether anything changed.
func (b *TextBuf) Insert(s []rune, maxLen int) bool {
	lo, hi := b.Sel()
	if maxLen > 0 {
		room := maxLen - (len(b.Rs) - (hi - lo))
		if room < len(s) {
			s = s[:max(0, room)]
		}
	}
	if len(s) == 0 && lo == hi {
		return false
	}
	out := make([]rune, 0, len(b.Rs)-(hi-lo)+len(s))
	out = append(append(append(out, b.Rs[:lo]...), s...), b.Rs[hi:]...)
	b.Rs = out
	b.Caret = lo + len(s)
	b.anchor = b.Caret
	return true
}

// Backspace deletes the selection, or the rune before the caret.
func (b *TextBuf) Backspace() bool {
	if !b.hasSel() {
		if b.Caret == 0 {
			return false
		}
		b.anchor = b.Caret - 1
	}
	return b.Insert(nil, 0)
}

// Del deletes the selection, or the rune after the caret.
func (b *TextBuf) Del() bool {
	if !b.hasSel() {
		if b.Caret >= len(b.Rs) {
			return false
		}
		b.anchor = b.Caret + 1
	}
	return b.Insert(nil, 0)
}

// printable reports r is a character a field accepts from typing.
func printable(r rune) bool {
	return r >= 32 && r != 127 && !(r >= 0x80 && r < 0xA0)
}

// ---- layout of a multi-line field ---------------------------------------

// TextSpan is one visual line: runes [start, end) (the newline that ends
// a hard line is not part of it).
type TextSpan struct{ Start, End int }

type lineCache struct {
	text  string
	w     float32
	scale float64
	spans []TextSpan
}

// LayoutLines breaks rs into visual lines no wider than maxW: at newlines,
// then after the last space that fits, else mid-word.
func LayoutLines(rs []rune, st gfx.TextStyle, maxW float64) []TextSpan {
	width := func(a []rune) float64 { w, _ := gfx.Measure(string(a), st); return w }
	var out []TextSpan
	i := 0
	for {
		e := i
		for e < len(rs) && rs[e] != '\n' {
			e++
		}
		s := i
		for {
			if e-s <= 1 || width(rs[s:e]) <= maxW {
				out = append(out, TextSpan{s, e})
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
			out = append(out, TextSpan{s, k})
			s = k
		}
		if e >= len(rs) {
			return out
		}
		i = e + 1
	}
}

// fieldLines is layoutLines cached per field (text, width and UI scale).
func (k *Kit) fieldLines(id string, rs []rune, w float32) []TextSpan {
	if k.lines == nil {
		k.lines = map[string]lineCache{}
	}
	s := string(rs)
	if c, ok := k.lines[id]; ok && c.text == s && c.w == w && c.scale == gfx.UIScale() {
		return c.spans
	}
	spans := LayoutLines(rs, gfx.Body, float64(w))
	k.lines[id] = lineCache{s, w, gfx.UIScale(), spans}
	return spans
}

// lineOf is the visual line the caret c sits on.
func lineOf(spans []TextSpan, c int) int {
	k := 0
	for i, sp := range spans {
		if sp.Start <= c {
			k = i
		}
	}
	return k
}

// NearestIndex is the rune boundary of rs closest to pixel x from its start.
func NearestIndex(rs []rune, x float64) int {
	width := func(n int) float64 { w, _ := gfx.Measure(string(rs[:n]), gfx.Body); return w }
	i := sort.Search(len(rs)+1, func(n int) bool { return width(n) >= x })
	if i > len(rs) {
		return len(rs)
	}
	if i > 0 && x-width(i-1) < width(i)-x {
		return i - 1
	}
	return i
}

func FieldInner(r gfx.Rect) gfx.Rect { return r.InsetXY(FieldPad, fieldPadTop) }

// caretAt is the rune index under a cursor position in field r.
func (k *Kit) caretAt(id string, rs []rune, r gfx.Rect, sp TextFieldSpec, mx, my int) int {
	in := FieldInner(r)
	if !sp.Multiline {
		return NearestIndex(rs, float64(float32(mx)-in.X+k.Scroll[id+".x"]))
	}
	spans := k.fieldLines(id, rs, in.W)
	lh := float32(gfx.LineH(gfx.Body))
	li := int((float32(my) - in.Y + k.Scroll[id+".y"]) / lh)
	li = max(0, min(len(spans)-1, li))
	sp2 := spans[li]
	return sp2.Start + NearestIndex(rs[sp2.Start:sp2.End], float64(float32(mx)-in.X))
}

// ---- editing --------------------------------------------------------------

func (k *Kit) buf() TextBuf {
	e := &k.Edit
	return TextBuf{Rs: []rune(*e.val), Caret: k.Caret, anchor: e.anchor}
}

// store writes b back to the field's value and the mirrored caret state.
func (k *Kit) store(b TextBuf, changed bool) {
	e := &k.Edit
	if changed {
		*e.val = string(b.Rs)
		k.fire(e.ID, evChanged)
	}
	e.anchor = b.anchor
	k.Caret = b.Caret
	k.SelStart, k.SelEnd = b.Sel()
	e.blinkAt = k.host.Updates()
	k.reveal()
}

// start begins editing: caret at pos (<0: the end), everything
// selected when all is set. Another field being edited is committed.
func (k *Kit) start(id string, val *string, sp TextFieldSpec, r gfx.Rect, pos int, all bool) {
	if k.Edit.ID != "" && k.Edit.ID != id {
		k.TextEnd(true)
	}
	n := len([]rune(*val))
	if pos < 0 || pos > n {
		pos = n
	}
	k.Edit = textEdit{ID: id, val: val, spec: sp, orig: *val, r: r, seen: k.frame, blinkAt: k.host.Updates()}
	k.Focus = id
	b := TextBuf{Rs: []rune(*val), Caret: pos, anchor: pos}
	if all {
		b.Caret, b.anchor = n, 0
	}
	k.store(b, false)
}

// TextEnd stops editing: commit keeps the value (after Normalize), else it
// is restored to what it was at focus time.
func (k *Kit) TextEnd(commit bool) {
	e := k.Edit
	if e.ID == "" {
		return
	}
	k.Edit = textEdit{}
	k.Active = ""
	k.Caret, k.SelStart, k.SelEnd = 0, 0, 0
	if commit {
		if e.spec.Normalize != nil {
			if n := e.spec.Normalize(*e.val); n != *e.val {
				*e.val = n
				k.fire(e.ID, evChanged)
			}
		}
		k.fire(e.ID, evCommit)
		return
	}
	if *e.val != e.orig {
		*e.val = e.orig
		k.fire(e.ID, evChanged)
	}
	k.fire(e.ID, evCancel)
}

// reveal scrolls the field so the caret shows.
func (k *Kit) reveal() {
	e := &k.Edit
	if e.ID == "" || e.val == nil {
		return
	}
	if k.Scroll == nil {
		k.Scroll = map[string]float32{}
	}
	in := FieldInner(e.r)
	rs := []rune(*e.val)
	if !e.spec.Multiline {
		cx, _ := gfx.Measure(string(rs[:min(k.Caret, len(rs))]), gfx.Body)
		tw, _ := gfx.Measure(*e.val, gfx.Body)
		sx := k.Scroll[e.ID+".x"]
		switch {
		case tw <= float64(in.W):
			sx = 0
		case float32(cx)-sx > in.W-2:
			sx = float32(cx) - in.W + 2
		case float32(cx) < sx:
			sx = float32(cx)
		}
		k.Scroll[e.ID+".x"] = max(0, sx)
		return
	}
	spans := k.fieldLines(e.ID, rs, in.W)
	lh := float32(gfx.LineH(gfx.Body))
	li := float32(lineOf(spans, k.Caret))
	sy := k.Scroll[e.ID+".y"]
	switch {
	case li*lh < sy:
		sy = li * lh
	case (li+1)*lh > sy+in.H:
		sy = (li+1)*lh - in.H
	}
	k.Scroll[e.ID+".y"] = max(0, sy)
}

func (k *Kit) metaHeld() bool {
	for _, key := range k.in().Raw.Held {
		if key == "Meta" {
			return true
		}
	}
	return false
}

// TextInput feeds the field being edited (the host calls it
// while CtxText is on top).
func (k *Kit) TextInput() {
	e := &k.Edit
	if e.ID == "" {
		return
	}
	if e.seen != k.frame { // its widget was not declared: the field is gone
		k.TextEnd(true)
		return
	}
	mx, my := k.host.Cursor()
	if k.in().Raw.Press[inputframe.Left] && !e.r.Hit(mx, my) { // click elsewhere commits
		k.TextEnd(true)
		return
	}
	shift := k.in().Shift()
	cmd := k.in().Ctrl() || k.metaHeld()
	b := k.buf()
	changed := false
	if !cmd {
		for _, r := range k.in().Raw.Chars {
			if printable(r) && (e.spec.Valid == nil || e.spec.Valid(r)) {
				changed = b.Insert([]rune{r}, e.spec.MaxLen) || changed
			}
		}
	}
	switch {
	case k.in().Key("Escape"):
		k.TextEnd(false)
		return
	case k.in().Key("Enter") || k.in().Key("NumpadEnter"):
		if !e.spec.Multiline {
			k.store(b, changed)
			k.TextEnd(true)
			return
		}
		changed = b.Insert([]rune{'\n'}, e.spec.MaxLen) || changed
	case k.in().Key("Tab"):
		k.store(b, changed)
		k.TextEnd(true)
		k.cycleFocus(map[bool]int{false: 1, true: -1}[shift])
		return
	case cmd && k.in().Key("A"):
		b.anchor, b.Caret = 0, len(b.Rs)
	case k.host.KeyRepeat("Backspace"):
		changed = b.Backspace() || changed
	case k.host.KeyRepeat("Delete"):
		changed = b.Del() || changed
	case k.host.KeyRepeat("ArrowLeft"):
		k.arrow(&b, -1, shift)
	case k.host.KeyRepeat("ArrowRight"):
		k.arrow(&b, 1, shift)
	case e.spec.Multiline && k.host.KeyRepeat("ArrowUp"):
		k.vertical(&b, -1, shift)
	case e.spec.Multiline && k.host.KeyRepeat("ArrowDown"):
		k.vertical(&b, 1, shift)
	case k.in().Key("Home"):
		b.Move(k.lineEdge(b, false), shift)
	case k.in().Key("End"):
		b.Move(k.lineEdge(b, true), shift)
	}
	k.store(b, changed)
}

// arrow moves the caret one rune; without Shift a selection first
// collapses to its near edge.
func (k *Kit) arrow(b *TextBuf, dir int, extend bool) {
	if b.hasSel() && !extend {
		lo, hi := b.Sel()
		if dir < 0 {
			b.Move(lo, false)
		} else {
			b.Move(hi, false)
		}
		return
	}
	b.Move(b.Caret+dir, extend)
}

// lineEdge is the start or end of the caret's visual line.
func (k *Kit) lineEdge(b TextBuf, end bool) int {
	e := &k.Edit
	if !e.spec.Multiline {
		if end {
			return len(b.Rs)
		}
		return 0
	}
	spans := k.fieldLines(e.ID, b.Rs, FieldInner(e.r).W)
	sp := spans[lineOf(spans, b.Caret)]
	if end {
		return sp.End
	}
	return sp.Start
}

// vertical moves the caret to the same x on the line above or below.
func (k *Kit) vertical(b *TextBuf, dir int, extend bool) {
	e := &k.Edit
	spans := k.fieldLines(e.ID, b.Rs, FieldInner(e.r).W)
	li := lineOf(spans, b.Caret)
	switch {
	case li+dir < 0:
		b.Move(0, extend)
		return
	case li+dir >= len(spans):
		b.Move(len(b.Rs), extend)
		return
	}
	cur, tgt := spans[li], spans[li+dir]
	x, _ := gfx.Measure(string(b.Rs[cur.Start:min(b.Caret, cur.End)]), gfx.Body)
	b.Move(tgt.Start+NearestIndex(b.Rs[tgt.Start:tgt.End], x), extend)
}

// TextField is a text field bound to *val. It reports the value changed
// since its last call and that an edit was committed (Enter, Tab or a
// click elsewhere; Esc restores the value and reports changed).
func (k *Kit) TextField(id string, r gfx.Rect, val *string, sp TextFieldSpec) (changed, committed bool) {
	tip := sp.Tip
	if sp.Disabled && sp.Reason != "" {
		tip = sp.Reason
	}
	w := Widget{ID: id, R: r, Focusable: true, Disabled: sp.Disabled, Reason: sp.Reason, Tip: tip}
	w.Click = func() {
		if !sp.Disabled {
			k.start(id, val, sp, r, -1, false)
		}
	}
	selID := id + ".sel"
	if k.Add(w) && !sp.Disabled {
		editing := k.Edit.ID == id
		if editing {
			k.Edit.val, k.Edit.spec, k.Edit.r, k.Edit.seen = val, sp, r, k.frame
		}
		mx, my := k.host.Cursor()
		switch {
		case k.Press(r):
			pos := k.caretAt(id, []rune(*val), r, sp, mx, my)
			if !editing {
				k.start(id, val, sp, r, pos, false)
			} else {
				b := k.buf()
				b.Move(pos, k.in().Shift())
				k.store(b, false)
			}
			k.Active = selID
		case k.TabTo == id && !editing:
			k.TabTo = ""
			k.start(id, val, sp, r, -1, true)
		}
		if k.Active == selID && k.Edit.ID == id {
			b := k.buf()
			b.Move(k.caretAt(id, b.Rs, r, sp, mx, my), true)
			k.store(b, false)
			if !k.in().Down(inputframe.Left) {
				k.Active = ""
				k.release()
			}
		}
	}
	ev := k.take(id)
	k.Queue(func(dst *ebiten.Image) { k.drawTextField(dst, id, r, *val, sp) })
	return ev&evChanged != 0, ev&evCommit != 0
}

func (k *Kit) drawTextField(dst *ebiten.Image, id string, r gfx.Rect, s string, sp TextFieldSpec) {
	editing := k.Edit.ID == id
	border := FieldBorder
	switch {
	case editing || k.Focus == id && k.kbd:
		border = FieldFocus
	case k.Hot == id && !sp.Disabled:
		border = BtnBorder
	}
	FillRect(dst, r, Field)
	StrokeRect(dst, r, border)
	in := FieldInner(r)
	clip := gfx.ClipTo(dst, in)
	st := gfx.Body
	if sp.Disabled {
		st.Color = TextDisabled
	}
	lh := float32(gfx.LineH(gfx.Body))
	if s == "" && !editing {
		ph := gfx.Body
		ph.Color = TextDisabled
		gfx.Text(clip, sp.Placeholder, float64(in.X), float64(in.Y+max(0, (in.H-lh)/2)), ph)
		return
	}
	rs := []rune(s)
	sx, sy := k.Scroll[id+".x"], k.Scroll[id+".y"]
	var spans []TextSpan
	switch {
	case sp.Multiline:
		spans = k.fieldLines(id, rs, in.W)
	default:
		spans = []TextSpan{{0, len(rs)}}
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
	lo, hi := k.SelStart, k.SelEnd
	for i, ln := range spans {
		y := ty + float32(i)*lh - sy
		if y+lh < in.Y || y > in.Bottom() {
			continue
		}
		x0 := in.X - sx
		if editing && hi > lo && hi > ln.Start && lo < ln.End+1 {
			a0, a1 := max(lo, ln.Start), min(hi, ln.End)
			xa, _ := gfx.Measure(string(rs[ln.Start:a0]), gfx.Body)
			xb, _ := gfx.Measure(string(rs[ln.Start:max(a0, a1)]), gfx.Body)
			if hi > ln.End && ln.End < len(rs) { // the selected newline: a nub
				xb += 4
			}
			sel := BtnSelected
			sel.A = 0xC0
			FillRect(clip, gfx.Rect{X: x0 + float32(xa), Y: y, W: float32(xb - xa), H: lh}, sel)
		}
		gfx.Text(clip, string(rs[ln.Start:ln.End]), float64(x0), float64(y), st)
	}
	if !editing {
		return
	}
	blink := k.host.ReducedMotion() || ((k.host.Updates()-k.Edit.blinkAt)/blinkFrames)%2 == 0
	if !blink {
		return
	}
	li := lineOf(spans, k.Caret)
	ln := spans[li]
	cx, _ := gfx.Measure(string(rs[ln.Start:max(ln.Start, min(k.Caret, ln.End))]), gfx.Body)
	cy := ty + float32(li)*lh - sy
	x := in.X - sx + float32(cx)
	gfx.Line(clip, x, cy+1, x, cy+lh-1, 1, Accent, false)
}
