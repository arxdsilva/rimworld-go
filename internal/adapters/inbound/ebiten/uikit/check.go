package uikit

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

const (
	checkBox    = 16 // the box's side
	CheckGap    = 8  // between the box and its label
	RadioRowH   = 24
	RadioRadius = 7
)

// CheckOpts are a checkbox's optional parts.
type CheckOpts struct {
	Disabled bool
	Reason   string // tooltip while disabled
	Tip      string // hover card: "Title\nbody"
}

// Check is a 16x16 checkbox with a label to its right, bound to *val.
// A click anywhere on r, or Space while focused, toggles it. It reports
// the value changed.
func (k *Kit) Check(id string, r gfx.Rect, label string, val *bool, o CheckOpts) bool {
	tip := o.Tip
	if o.Disabled && o.Reason != "" {
		tip = o.Reason
	}
	w := Widget{ID: id, Label: label, R: r, Focusable: true, Disabled: o.Disabled, Reason: o.Reason, Tip: tip}
	toggle := func() {
		if o.Disabled {
			k.Deny(o.Reason)
			return
		}
		*val = !*val
		k.OK()
		k.fire(id, evChanged)
	}
	w.Click = toggle
	changed := false
	if k.Add(w) {
		if k.Press(r) || (!o.Disabled && k.KeyAct(id)) {
			toggle()
		}
		changed = k.take(id)&evChanged != 0
	}
	k.Queue(func(dst *ebiten.Image) {
		box := gfx.Rect{X: r.X, Y: r.Y + (r.H-checkBox)/2, W: checkBox, H: checkBox}
		border := FieldBorder
		if k.Hot == id && !o.Disabled {
			border = Accent
		}
		FillRect(dst, box, Field)
		StrokeRect(dst, box, border)
		tc := color.Color(Text)
		if o.Disabled {
			tc = TextDisabled
		}
		if *val {
			c := Accent
			if o.Disabled {
				c = TextDisabled
			}
			gfx.Line(dst, box.X+3, box.Y+8, box.X+6.5, box.Y+12, 2, c, true)
			gfx.Line(dst, box.X+6, box.Y+12, box.X+13, box.Y+3.5, 2, c, true)
		}
		st := gfx.Body
		st.Color = tc
		gfx.Text(dst, FitText(label, st, r.W-checkBox-CheckGap), float64(box.Right()+CheckGap), float64(r.Y)+(float64(r.H)-gfx.LineH(st))/2, st)
	})
	return changed
}

// RadioOpts are a radio group's optional parts.
type RadioOpts struct {
	RowH     float32  // option height (default 24)
	Disabled []string // per option: a reason disables it ("" enables)
	Tips     []string // per option hover cards
}

// Radio is a vertical radio group over labels, bound to *sel. Clicking
// an option selects it; while the group has focus Up and Down move the
// selection to the neighbouring enabled option. Option ids are id+".N".
// It reports the selection changed.
func (k *Kit) Radio(id string, r gfx.Rect, labels []string, sel *int, o RadioOpts) bool {
	rowH := o.RowH
	if rowH <= 0 {
		rowH = RadioRowH
	}
	reason := func(i int) string {
		if i < len(o.Disabled) {
			return o.Disabled[i]
		}
		return ""
	}
	choose := func(i int) {
		if reason(i) != "" {
			k.Deny(reason(i))
			return
		}
		k.Focus = id
		if *sel != i {
			*sel = i
			k.fire(id, evChanged)
		}
		k.OK()
	}
	rows := r.RowsN(len(labels), rowH, 0)
	changed := false
	if k.Add(Widget{ID: id, R: r, Focusable: true}) {
		for i := range labels {
			i := i
			w := Widget{ID: OptID(id, i), Label: labels[i], R: rows[i].Intersect(r), Disabled: reason(i) != "", Reason: reason(i), Click: func() { choose(i) }}
			w.Tip = reason(i)
			if w.Tip == "" && i < len(o.Tips) {
				w.Tip = o.Tips[i]
			}
			k.Add(w)
			if k.Press(w.R) {
				choose(i)
			}
		}
		if k.Focus == id {
			for _, d := range []struct {
				key string
				dir int
			}{{"ArrowUp", -1}, {"ArrowDown", 1}} {
				if !k.host.KeyRepeat(d.key) {
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
		changed = k.take(id)&evChanged != 0
	}
	k.Queue(func(dst *ebiten.Image) {
		for i, l := range labels {
			row := rows[i]
			cx, cy := float32(row.X)+RadioRadius+1, row.Y+row.H/2
			dis := reason(i) != ""
			border := FieldBorder
			if k.Hot == OptID(id, i) && !dis {
				border = Accent
			}
			gfx.FillCircle(dst, cx, cy, RadioRadius, Field, true)
			gfx.StrokeCircle(dst, cx, cy, RadioRadius, 1, border, true)
			if *sel == i {
				c := Accent
				if dis {
					c = TextDisabled
				}
				gfx.FillCircle(dst, cx, cy, RadioRadius-3, c, true)
			}
			st := gfx.Body
			if dis {
				st.Color = TextDisabled
			}
			gfx.Text(dst, FitText(l, st, row.W-2*RadioRadius-CheckGap-2), float64(cx+RadioRadius+CheckGap), float64(row.Y)+(float64(row.H)-gfx.LineH(st))/2, st)
		}
	})
	return changed
}

// OptID is the id of option i of a group (radio options, tabs, rows).
func OptID(id string, i int) string { return id + "." + Num(i) }
