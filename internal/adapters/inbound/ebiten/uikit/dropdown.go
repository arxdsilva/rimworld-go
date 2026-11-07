package uikit

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
)

// DropOpt is one entry of a DropList: a Disabled entry is greyed, cannot
// be picked and shows Tip (its reason) on hover.
type DropOpt struct {
	Label, Tip string
	Disabled   bool
}

// DropList opens a dropdown under anchor (UIS-008): a Popup modal of one
// column of rows MenuRowH apart, at least as wide as anchor and
// ScaleListW and as its widest label, kept on the screen. A row's id is
// id+".N"; picking a row closes the list and calls pick. A click outside
// it or Esc closes it.
func DropList(k *Kit, id string, anchor gfx.Rect, opts []DropOpt, cur int, pick func(int)) {
	w, h := DropListSize(anchor, opts)
	k.Push(Modal{
		ID: id, W: w, H: h, Popup: true,
		Anchor: func() gfx.Rect {
			sw, sh := k.host.Size()
			return DropListRect(anchor, w, h, sw, sh)
		},
		Draw: func(body gfx.Rect) {
			for i, o := range opts {
				r := gfx.Rect{X: body.X, Y: body.Y + float32(i)*hudlayout.ScaleListRowH, W: body.W, H: hudlayout.ScaleListRowH}
				if r.Bottom() > body.Bottom()+0.5 {
					break
				}
				StyledButton(k, OptID(id, i), r, o.Label,
					BtnOpts{Plain: true, Selected: i == cur, Disabled: o.Disabled, Reason: o.Tip, Tip: o.Tip},
					func() { k.CloseModal(id); pick(i) })
			}
		},
	})
}

// DropListSize is the list's frame size: rows plus the 1 px frame.
func DropListSize(anchor gfx.Rect, opts []DropOpt) (w, h float32) {
	w = max(anchor.W, hudlayout.ScaleListW)
	for _, o := range opts {
		w = max(w, gfx.TextW(o.Label, gfx.Body)+dropTextPad)
	}
	return w + 2, float32(len(opts))*hudlayout.ScaleListRowH + 2
}

// dropTextPad is what a row adds to its text: DrawListRow's 8 px each side.
const dropTextPad = 16

// DropListRect places a w x h list under anchor, flush with its left edge,
// moved left and up as needed to stay on an sw x sh screen (above the
// anchor when there is no room under it).
func DropListRect(anchor gfx.Rect, w, h float32, sw, sh int) gfx.Rect {
	W, H := float32(sw), float32(sh)
	w, h = min(w, W), min(h, H)
	x := min(max(anchor.X, 0), W-w)
	y := anchor.Bottom()
	if y+h > H {
		y = max(anchor.Y-h, 0)
	}
	return gfx.Rect{X: x, Y: y, W: w, H: h}
}

// RowStripeColor is the faint fill of every other table row
// (non-premultiplied).
var RowStripeColor = color.NRGBA{R: PanelBorder.R, G: PanelBorder.G, B: PanelBorder.B, A: 0x28}

// RowStripe fills row r when i is odd: the stripe of every other table row.
func RowStripe(dst *ebiten.Image, r gfx.Rect, i int) {
	if i%2 == 1 {
		FillRect(dst, r, RowStripeColor)
	}
}
