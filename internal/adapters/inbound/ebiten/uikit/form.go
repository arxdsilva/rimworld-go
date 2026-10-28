package uikit

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// The composites the setup screens and the Options story tab share: a
// vertical form inside a scroll view with its labelled rows, and the
// buttons, steppers, dropdowns and segmented rows built from the kit's
// widgets. They use only the Kit's exported API, so they are free
// functions (and Form methods) over it rather than more Kit methods.

// Owner hands out its kit (the ports.Widgets role; the root App). Form
// rows take one so a screen passes its shell.
type Owner interface {
	Kit() *Kit
}

// Form lays a vertical form out inside a scroll view: Row hands out the
// next row of the content, shifted by the scroll offset; a row that is
// not fully inside the view reports false and the caller declares no
// widget for it.
type Form struct {
	View gfx.Rect
	X, W float32
	y    float32 // content height so far
	Off  float32
}

// NewForm starts a form in the view of a scroll area (padding 4 px).
func NewForm(area ScrollArea) *Form {
	return &Form{View: area.View, X: area.View.X + 4, W: max(0, area.View.W-8), Off: area.Off}
}

// Row is the next row of height h.
func (f *Form) Row(h float32) (gfx.Rect, bool) {
	r := gfx.Rect{X: f.X, Y: f.View.Y + f.y - f.Off, W: f.W, H: h}
	f.y += h
	return r, r.Y >= f.View.Y-0.5 && r.Bottom() <= f.View.Bottom()+0.5
}

// Gap leaves h px empty.
func (f *Form) Gap(h float32) { f.y += h }

// Height is the content laid out so far (unpadded).
func (f *Form) Height() float32 { return f.y }

// Content is the content height so far (for the scroll area next frame).
func (f *Form) Content() float32 { return f.y + 4 }

// SectionTitle is a heading row of the form.
func (f *Form) SectionTitle(o Owner, s string) {
	if row, ok := f.Row(26); ok {
		k := o.Kit()
		hs := gfx.Label
		hs.Color = Heading
		k.Label(row.X, row.Y+4, s, hs)
		k.Queue(func(dst *ebiten.Image) {
			FillRect(dst, gfx.Rect{X: row.X, Y: row.Bottom() - 2, W: row.W, H: 1}, PanelBorder)
		})
	}
}

// Form row heights.
const (
	formSliderH = 40
	formCheckH  = 28
)

// Slider is a labelled slider row over *v: the label at the left, the
// value at the right, the track under them. sp gives the range, the key
// step and the host's step sound (Tick); the row sets the title, the
// format and the disabled state. A reason disables it (dimmed, with the
// reason as its tooltip).
func (f *Form) Slider(o Owner, id, label string, v *int, sp SliderSpec, format func(int) string, disabled, tip string) {
	row, ok := f.Row(formSliderH)
	if !ok {
		return
	}
	k := o.Kit()
	ls := gfx.Body
	vs := gfx.Label
	vs.Align = gfx.AlignRight
	if disabled != "" {
		ls.Color, vs.Color = TextDisabled, TextDisabled
	}
	k.Label(row.X, row.Y+1, label, ls)
	k.Label(row.Right(), row.Y+1, format(*v), vs)
	sr := gfx.Rect{X: row.X + 2, Y: row.Y + 20, W: row.W - 4, H: 18}
	sp.Title = label
	sp.Format = func(x float64) string { return format(int(x)) }
	sp.Disabled, sp.Reason = disabled != "", disabled
	k.SliderInt("diff.f."+id, sr, v, sp)
	if tip != "" && disabled == "" {
		k.Tip("diff.t."+id, gfx.Rect{X: row.X, Y: row.Y, W: row.W, H: 18}, tip)
	}
}

// Check is a toggle row; a reason disables it.
func (f *Form) Check(o Owner, id, label string, v *bool, disabled, tip string) {
	row, ok := f.Row(formCheckH)
	if !ok {
		return
	}
	o.Kit().Check("diff.f."+id, row, label, v, CheckOpts{Disabled: disabled != "", Reason: disabled, Tip: tip})
}

// Stepper is a labelled -/+ row over *v within [lo, hi].
func (f *Form) Stepper(o Owner, id, label string, v *int, lo, hi int, tip string) {
	row, ok := f.Row(formCheckH + 4)
	if !ok {
		return
	}
	k := o.Kit()
	ls := gfx.Body
	k.Label(row.X, row.Y+(row.H-float32(gfx.LineH(ls)))/2, label, ls)
	Stepper(k, "diff.f."+id, gfx.Rect{X: row.Right() - 112, Y: row.Y + 2, W: 112, H: row.H - 4}, v, lo, hi)
	if tip != "" {
		k.Tip("diff.t."+id, gfx.Rect{X: row.X, Y: row.Y, W: row.W - 116, H: row.H}, tip)
	}
}

// DimText is the secondary body style.
func DimText() gfx.TextStyle {
	st := gfx.Body
	st.Color = TextDim
	return st
}

// TextLines draws s wrapped to w px from (x, y) in st and returns the
// height used.
func TextLines(k *Kit, x, y, w float32, s string, st gfx.TextStyle) float32 {
	h := float32(0)
	for _, l := range gfx.WrapPx(s, st, float64(w)) {
		k.Label(x, y+h, l, st)
		h += float32(gfx.LineH(st))
	}
	return h
}

// RowLabel puts a dim label at the left of a form row and returns the
// rectangle left for the control.
func RowLabel(k *Kit, r gfx.Rect, label string, labelW float32) gfx.Rect {
	st := DimText()
	k.Label(r.X, r.Y+(r.H-float32(gfx.LineH(st)))/2, FitText(label, st, labelW-8), st)
	return gfx.Rect{X: r.X + labelW, Y: r.Y, W: max(0, r.W-labelW), H: r.H}
}

// StyledButton is a kit button whose click runs fn at once (also when the
// click comes from the id-driven click path of tests and captures), not on
// the next frame.
func StyledButton(k *Kit, id string, r gfx.Rect, label string, o BtnOpts, fn func()) {
	pressed := k.Button(id, r, label, o)
	if n := len(k.Cur); n > 0 && k.Cur[n-1].ID == id {
		k.Cur[n-1].Click = func() {
			if o.Disabled {
				k.Deny(o.Reason)
				return
			}
			k.OK()
			fn()
		}
	}
	if pressed {
		fn()
	}
}

// Stepper is "- value +" in r over *v, within [lo, hi].
func Stepper(k *Kit, id string, r gfx.Rect, v *int, lo, hi int) {
	*v = max(lo, min(hi, *v))
	bw := float32(28)
	StyledButton(k, id+".dec", gfx.Rect{X: r.X, Y: r.Y, W: bw, H: r.H}, "-", BtnOpts{Disabled: *v <= lo, Reason: fmt.Sprintf("The minimum is %d.", lo)}, func() { *v = max(lo, *v-1) })
	StyledButton(k, id+".inc", gfx.Rect{X: r.X + bw + 56, Y: r.Y, W: bw, H: r.H}, "+", BtnOpts{Disabled: *v >= hi, Reason: fmt.Sprintf("The maximum is %d.", hi)}, func() { *v = min(hi, *v+1) })
	val := *v
	k.Queue(func(dst *ebiten.Image) {
		st := gfx.Label
		st.Align = gfx.AlignCenter
		gfx.Text(dst, strconv.Itoa(val), float64(r.X+bw+28), float64(r.Y)+(float64(r.H)-gfx.LineH(st))/2, st)
	})
}

// Choice is a dropdown: a button showing the current option that opens a
// modal list. pick receives the chosen option. A reason disables it.
func Choice(k *Kit, id string, r gfx.Rect, title string, opts []string, cur int, disabled string, pick func(i int)) {
	label := "-"
	if cur >= 0 && cur < len(opts) {
		label = opts[cur]
	}
	StyledButton(k, id, r, label+"  v", BtnOpts{Disabled: disabled != "", Reason: disabled}, func() {
		Picker(k, id, title, opts, cur, pick)
	})
}

// Picker opens the option list of a dropdown.
func Picker(k *Kit, id, title string, opts []string, cur int, pick func(i int)) {
	h := float32(ModalTitleH+2*ModalPad) + min(float32(len(opts)), 12)*ListRowH + 4
	mid := "pick." + id
	k.Push(Modal{
		ID: mid, Title: title, W: 380, H: h,
		Draw: func(body gfx.Rect) {
			choice := -1
			changed := k.List("modal."+mid+".list", body, len(opts), &choice, ListSpec{
				Draw: func(dst *ebiten.Image, i int, row gfx.Rect, hover, selected bool) {
					DrawListRow(dst, row, opts[i], hover, i == cur, false)
				},
			})
			if changed && choice >= 0 {
				k.CloseModal(mid)
				pick(choice)
			}
		},
	})
}

// Segmented is a row of mutually exclusive buttons over *sel.
func Segmented(k *Kit, id string, r gfx.Rect, labels []string, sel *int, tips []string) {
	n := len(labels)
	if n == 0 {
		return
	}
	w := (r.W - float32(n-1)*4) / float32(n)
	for i, l := range labels {
		o := BtnOpts{Selected: *sel == i}
		if i < len(tips) {
			o.Tip = tips[i]
		}
		StyledButton(k, OptID(id, i), gfx.Rect{X: r.X + float32(i)*(w+4), Y: r.Y, W: w, H: r.H}, l, o, func() { *sel = i })
	}
}

// OnScreenLayer makes the widgets a screen hook declares belong to the
// screen's own layer (0), so they stay dead and painted under an open
// modal. The frame hands hooks the top layer, which would keep them live
// and draw them over the dialog.
func OnScreenLayer(k *Kit) { k.Layer = 0 }

// WrapCols breaks s into rows of at most cols characters at spaces (a
// longer word is cut); cols is at least 4.
func WrapCols(s string, cols int) []string {
	if cols < 4 {
		cols = 4
	}
	var rows []string
	cur := ""
	for _, w := range strings.Fields(s) {
		for len(w) > cols {
			if cur != "" {
				rows = append(rows, cur)
				cur = ""
			}
			rows = append(rows, w[:cols])
			w = w[cols:]
		}
		switch {
		case cur == "":
			cur = w
		case len(cur)+1+len(w) <= cols:
			cur += " " + w
		default:
			rows = append(rows, cur)
			cur = w
		}
	}
	if cur != "" {
		rows = append(rows, cur)
	}
	return rows
}
