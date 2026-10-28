package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Tabs (NG-1 §1.4), generalised from the Options tab strip.

const (
	tabMinW = 90
	tabPadX = 28
	tabGap  = 4
)

// tabsOpts are a tab strip's optional parts.
type tabsOpts struct {
	Disabled []string // per tab: a reason disables it
	Tips     []string // per tab hover cards
}

// tabRects lays the tabs of labels out left to right inside r.
func tabRects(r rect, labels []string) []rect {
	out := make([]rect, len(labels))
	x := r.X
	for i, l := range labels {
		w := max(tabMinW, textW(l, tsLabel)+tabPadX)
		out[i] = rect{X: x, Y: r.Y, W: w, H: r.H}
		x += w + tabGap
	}
	return out
}

// uiTabs is a strip of tabs bound to *sel. Clicking selects; while the
// strip has focus Left and Right move to the neighbouring enabled tab.
// Tab ids are id+".N". It reports the selection changed.
func (a *App) uiTabs(id string, r rect, labels []string, sel *int, o tabsOpts) bool {
	u := a.uiSt()
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
		if *sel != i {
			*sel = i
			u.fire(id, evChanged)
		}
		a.uiOK()
	}
	rects := tabRects(r, labels)
	changed := false
	if u.add(uiWidget{id: id, r: r, focusable: true}) {
		for i := range labels {
			i := i
			w := uiWidget{id: optID(id, i), label: labels[i], r: rects[i], disabled: reason(i) != "", reason: reason(i), click: func() { u.focus = id; choose(i) }}
			w.tip = reason(i)
			if w.tip == "" && i < len(o.Tips) {
				w.tip = o.Tips[i]
			}
			u.add(w)
			if a.uiPress(rects[i]) {
				u.focus = id
				choose(i)
			}
		}
		if u.focus == id {
			for _, d := range []struct {
				key string
				dir int
			}{{"ArrowLeft", -1}, {"ArrowRight", 1}} {
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
			tr := rects[i]
			dis := reason(i) != ""
			fill, border, text := btnNormal.colors(u.hot == optID(id, i) && !dis, false, *sel == i)
			if dis {
				fill, border, text = uiBtnDisabled, uiBtnBorder, uiTextDisabled
			}
			fillRect(dst, tr, fill)
			strokeRect(dst, tr, border)
			st := tsLabel
			st.Color, st.Align = text, alignCenter
			cx, cy := tr.Center()
			drawText(dst, fitText(l, tsLabel, tr.W-8), float64(cx), float64(cy)-lineH(st)/2, st)
		}
	})
	return changed
}
