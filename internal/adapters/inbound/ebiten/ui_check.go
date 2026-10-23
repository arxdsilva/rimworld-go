package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	checkBox    = 16 // the box's side
	checkGap    = 8  // between the box and its label
	radioRowH   = 24
	radioRadius = 7
)

// checkOpts are a checkbox's optional parts.
type checkOpts struct {
	Disabled bool
	Reason   string // tooltip while disabled
	Tip      string // hover card: "Title\nbody"
}

// uiCheck is a 16x16 checkbox with a label to its right, bound to *val.
// A click anywhere on r, or Space while focused, toggles it. It reports
// the value changed.
func (a *App) uiCheck(id string, r rect, label string, val *bool, o checkOpts) bool {
	u := a.uiSt()
	tip := o.Tip
	if o.Disabled && o.Reason != "" {
		tip = o.Reason
	}
	w := uiWidget{id: id, label: label, r: r, focusable: true, disabled: o.Disabled, reason: o.Reason, tip: tip}
	toggle := func() {
		if o.Disabled {
			a.uiDeny(o.Reason)
			return
		}
		*val = !*val
		a.uiOK()
		u.fire(id, evChanged)
	}
	w.click = toggle
	changed := false
	if u.add(w) {
		if a.uiPress(r) || (!o.Disabled && a.uiKeyAct(id)) {
			toggle()
		}
		changed = u.take(id)&evChanged != 0
	}
	u.queue(func(dst *ebiten.Image) {
		box := rect{r.X, r.Y + (r.H-checkBox)/2, checkBox, checkBox}
		border := uiFieldBorder
		if u.hot == id && !o.Disabled {
			border = uiAccent
		}
		fillRect(dst, box, uiField)
		strokeRect(dst, box, border)
		tc := color.Color(uiText)
		if o.Disabled {
			tc = uiTextDisabled
		}
		if *val {
			c := uiAccent
			if o.Disabled {
				c = uiTextDisabled
			}
			vector.StrokeLine(dst, box.X+3, box.Y+8, box.X+6.5, box.Y+12, 2, c, true)
			vector.StrokeLine(dst, box.X+6, box.Y+12, box.X+13, box.Y+3.5, 2, c, true)
		}
		st := tsBody
		st.Color = tc
		drawText(dst, fitText(label, st, r.W-checkBox-checkGap), float64(box.right()+checkGap), float64(r.Y)+(float64(r.H)-lineH(st))/2, st)
	})
	return changed
}

// radioOpts are a radio group's optional parts.
type radioOpts struct {
	RowH     float32  // option height (default 24)
	Disabled []string // per option: a reason disables it ("" enables)
	Tips     []string // per option hover cards
}

// uiRadio is a vertical radio group over labels, bound to *sel. Clicking
// an option selects it; while the group has focus Up and Down move the
// selection to the neighbouring enabled option. Option ids are id+".N".
// It reports the selection changed.
func (a *App) uiRadio(id string, r rect, labels []string, sel *int, o radioOpts) bool {
	u := a.uiSt()
	rowH := o.RowH
	if rowH <= 0 {
		rowH = radioRowH
	}
	reason := func(i int) string {
		if i < len(o.Disabled) {
			return o.Disabled[i]
		}
		return ""
	}
	choose := func(i int) {
		if reason(i) != "" {
			a.uiDeny(reason(i))
			return
		}
		u.focus = id
		if *sel != i {
			*sel = i
			u.fire(id, evChanged)
		}
		a.uiOK()
	}
	rows := r.rowsN(len(labels), rowH, 0)
	changed := false
	if u.add(uiWidget{id: id, r: r, focusable: true}) {
		for i := range labels {
			i := i
			w := uiWidget{id: optID(id, i), label: labels[i], r: rows[i].intersect(r), disabled: reason(i) != "", reason: reason(i), click: func() { choose(i) }}
			w.tip = reason(i)
			if w.tip == "" && i < len(o.Tips) {
				w.tip = o.Tips[i]
			}
			u.add(w)
			if a.uiPress(w.r) {
				choose(i)
			}
		}
		if u.focus == id {
			for _, d := range []struct {
				key string
				dir int
			}{{"ArrowUp", -1}, {"ArrowDown", 1}} {
				if !a.keyRepeat(d.key) {
					continue
				}
				for i := *sel + d.dir; i >= 0 && i < len(labels); i += d.dir {
					if reason(i) == "" {
						choose(i)
						break
					}
				}
			}
		}
		changed = u.take(id)&evChanged != 0
	}
	u.queue(func(dst *ebiten.Image) {
		for i, l := range labels {
			row := rows[i]
			cx, cy := float32(row.X)+radioRadius+1, row.Y+row.H/2
			dis := reason(i) != ""
			border := uiFieldBorder
			if u.hot == optID(id, i) && !dis {
				border = uiAccent
			}
			vector.FillCircle(dst, cx, cy, radioRadius, uiField, true)
			vector.StrokeCircle(dst, cx, cy, radioRadius, 1, border, true)
			if *sel == i {
				c := uiAccent
				if dis {
					c = uiTextDisabled
				}
				vector.FillCircle(dst, cx, cy, radioRadius-3, c, true)
			}
			st := tsBody
			if dis {
				st.Color = uiTextDisabled
			}
			drawText(dst, fitText(l, st, row.W-2*radioRadius-checkGap-2), float64(cx+radioRadius+checkGap), float64(row.Y)+(float64(row.H)-lineH(st))/2, st)
		}
	})
	return changed
}

// optID is the id of option i of a group (radio options, tabs, rows).
func optID(id string, i int) string { return id + "." + uiNum(i) }
