package uikit

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Tabs (NG-1 §1.4), generalised from the Options tab strip.

const (
	tabMinW = 90
	tabPadX = 28
	tabGap  = 4
)

// TabsOpts are a tab strip's optional parts.
type TabsOpts struct {
	Disabled []string // per tab: a reason disables it
	Tips     []string // per tab hover cards
}

// TabRects lays the tabs of labels out left to right inside r.
func TabRects(r gfx.Rect, labels []string) []gfx.Rect {
	out := make([]gfx.Rect, len(labels))
	x := r.X
	for i, l := range labels {
		w := max(tabMinW, gfx.TextW(l, gfx.Label)+tabPadX)
		out[i] = gfx.Rect{X: x, Y: r.Y, W: w, H: r.H}
		x += w + tabGap
	}
	return out
}

// Tabs is a strip of tabs bound to *sel. Clicking selects; while the
// strip has focus Left and Right move to the neighbouring enabled tab.
// Tab ids are id+".N". It reports the selection changed.
func (k *Kit) Tabs(id string, r gfx.Rect, labels []string, sel *int, o TabsOpts) bool {
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
		if *sel != i {
			*sel = i
			k.fire(id, evChanged)
		}
		k.OK()
	}
	rects := TabRects(r, labels)
	changed := false
	if k.Add(Widget{ID: id, R: r, Focusable: true}) {
		for i := range labels {
			i := i
			w := Widget{ID: OptID(id, i), Label: labels[i], R: rects[i], Disabled: reason(i) != "", Reason: reason(i), Click: func() { k.Focus = id; choose(i) }}
			w.Tip = reason(i)
			if w.Tip == "" && i < len(o.Tips) {
				w.Tip = o.Tips[i]
			}
			k.Add(w)
			if k.Press(rects[i]) {
				k.Focus = id
				choose(i)
			}
		}
		if k.Focus == id {
			for _, d := range []struct {
				key string
				dir int
			}{{"ArrowLeft", -1}, {"ArrowRight", 1}} {
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
			tr := rects[i]
			dis := reason(i) != ""
			fill, border, text := BtnNormal.Colors(k.Hot == OptID(id, i) && !dis, false, *sel == i)
			if dis {
				fill, border, text = BtnDisabled, BtnBorder, TextDisabled
			}
			FillRect(dst, tr, fill)
			StrokeRect(dst, tr, border)
			st := gfx.Label
			st.Color, st.Align = text, gfx.AlignCenter
			cx, cy := tr.Center()
			gfx.Text(dst, FitText(l, gfx.Label, tr.W-8), float64(cx), float64(cy)-gfx.LineH(st)/2, st)
		}
	})
	return changed
}
