package hud

import (
	"fmt"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/input"
)

// The bottom tab strip (PLAY-018: WINDOWS owns it; UIS-040 to UIS-043):
// hudlayout.TabsH tall, the full width. Eight text tabs share a unit width
// and the three icon tabs (History, Factions, Menu) half of it, the last
// tab taking what is left; TabSep separators part them. A text tab's label
// is left-aligned at TabLabelX of its width on the TabBaseline; an icon
// tab draws its icon and keeps the label in its tooltip, which also names
// the live key.

var hudTabSpecs = []struct {
	id, label string
	key       input.Action // its shortcut ("" none)
	icon      string       // ui_icon name of an icon tab ("" a text tab)
}{
	{ids.TabArchitect, "Architect", input.TabArchitect, ""},
	{ids.TabWork, "Work", input.TabWork, ""},
	{ids.TabSchedule, "Schedule", input.TabSchedule, ""},
	{ids.TabAssign, "Assign", input.TabAssign, ""},
	{ids.TabAnimals, "Animals", input.TabAnimals, ""},
	{ids.TabWildlife, "Wildlife", input.TabWildlife, ""},
	{ids.TabResearch, "Research", input.TabResearch, ""},
	{ids.TabWorld, "World", input.TabWorld, ""},
	{ids.TabHistory, "History", input.TabHistory, "tab_history"},
	{ids.TabFactions, "Factions", "", "tab_factions"},
	{ids.TabMenu, "Menu", input.Cancel, "tab_menu"},
}

// tabIconSize is an icon tab's icon (logical px).
const tabIconSize = 24

// tabsRect is the strip, the layout's Tabs rect.
func tabsRect(sw, sh int) gfx.Rect {
	return gfx.Rect{X: 0, Y: float32(sh - hudlayout.TabsH), W: float32(sw), H: hudlayout.TabsH}
}

// TabWidths are the strip's tab widths for a strip w wide (UIS-042): a
// text tab is unit = floor(w / (texts + icons x IconTabShare)), an icon
// tab floor(unit x IconTabShare), and the last tab absorbs the remainder.
func TabWidths(w float32) []float32 {
	n := len(hudTabSpecs)
	icons := 0
	for _, t := range hudTabSpecs {
		if t.icon != "" {
			icons++
		}
	}
	unit := float32(math.Floor(float64(w) / (float64(n-icons) + float64(icons)*hudlayout.IconTabShare)))
	half := float32(math.Floor(float64(unit) * hudlayout.IconTabShare))
	out := make([]float32, n)
	used := float32(0)
	for i, t := range hudTabSpecs {
		out[i] = unit
		if t.icon != "" {
			out[i] = half
		}
		if i == n-1 {
			out[i] = max(0, w-used)
		}
		used += out[i]
	}
	return out
}

// Buttons are the strip's tabs (TabWidths); a tab is selected while its
// window (or the Architect) is the open one.
func (c Tabs) Buttons(sw, sh int) []uikit.Button {
	n := len(hudTabSpecs)
	if n < 1 || sw < 1 {
		return nil
	}
	tr := tabsRect(sw, sh)
	ws := TabWidths(tr.W)
	out := make([]uikit.Button, 0, n)
	x := tr.X
	for i, t := range hudTabSpecs {
		b := uikit.Button{
			ID:       t.id,
			Label:    t.label,
			X:        x,
			Y:        tr.Y,
			W:        ws[i],
			H:        tr.H,
			Selected: c.sh.TabOn(t.id),
		}
		x += ws[i]
		switch {
		case t.key != "":
			b.Tip = t.label + c.sh.KeyHint(t.key)
		case t.icon != "":
			b.Tip = t.label // an icon tab's name lives in its tooltip
		}
		switch t.id {
		case ids.TabResearch:
			if name, pct, ok := c.researchUnderWay(); ok {
				b.Tip = fmt.Sprintf("%s: %s %d%%", b.Tip, name, pct)
			}
		case ids.TabFactions:
			if g := c.sh.Env().Game; g != nil && len(g.Factions()) == 0 {
				b.Disabled, b.Reason = true, "no other peoples in this colony"
				b.Selected = false
			}
		}
		out = append(out, b)
	}
	return out
}

// researchUnderWay is the project being researched: its name and percent.
func (c Tabs) researchUnderWay() (name string, pct int, ok bool) {
	if !c.live() {
		return "", 0, false
	}
	for _, p := range c.sh.World().Research {
		if p.Current && !p.Done {
			if p.Cost > 0 {
				pct = min(100, 100*p.Progress/p.Cost)
			}
			return p.Label, pct, true
		}
	}
	return "", 0, false
}

// Draw draws the bottom tab strip; Research shows a thin fill for the
// project under way.
func (c Tabs) Draw(screen *ebiten.Image, sw, sh int) {
	mx, my := c.sh.Cursor()
	bs := c.Buttons(sw, sh)
	for i, b := range bs {
		c.drawTab(screen, b, hudTabSpecs[i].icon, b.Contains(mx, my), i == len(bs)-1)
	}
	_, pct, ok := c.researchUnderWay()
	if !ok {
		return
	}
	for _, b := range bs {
		if b.ID != ids.TabResearch {
			continue
		}
		track := gfx.Rect{X: b.X + 4, Y: b.Y + b.H - 6, W: b.W - 8 - hudlayout.TabSep, H: 3}
		gfx.FillRect(screen, track.X, track.Y, track.W, track.H, uikit.SliderTrack, false)
		gfx.FillRect(screen, track.X, track.Y, track.W*float32(pct)/100, track.H, uikit.Accent, false)
	}
}

// drawTab draws one tab: its fill, its separator on the right (not after
// the last), and its label (a text tab) or its icon (an icon tab; the
// label stands in only while the art is missing).
func (c Tabs) drawTab(screen *ebiten.Image, b uikit.Button, icon string, hover, last bool) {
	fill, _, text := uikit.ButtonColors(b, hover)
	gfx.FillRect(screen, b.X, b.Y, b.W, b.H, fill, false)
	inner := b.W
	if !last {
		inner -= hudlayout.TabSep
		gfx.FillRect(screen, b.X+inner, b.Y, hudlayout.TabSep, b.H, uikit.Space, false)
	}
	if icon != "" {
		if img := c.icon(icon); img != nil {
			size := float32(min(tabIconSize, inner-4, b.H-4))
			x, y := b.X+(inner-size)/2, b.Y+(b.H-size)/2
			if b.Disabled {
				drawIconCS(screen, img, float64(x), float64(y), float64(size), sprite.ColorScale{0.45, 0.45, 0.45, 1})
			} else {
				gfx.Icon(screen, img, float64(x), float64(y), float64(size))
			}
			return
		}
	}
	st := gfx.WithColor(gfx.Body, text)
	x := b.X + float32(math.Floor(float64(inner)*hudlayout.TabLabelX))
	label := gfx.Ellipsize(b.Label, st, float64(b.X+inner-x-2))
	gfx.Text(screen, label, float64(x), float64(TabLabelTop(b.Y, st)), st)
}

// notoAscent is Noto Sans's ascent in em: a line's top sits that far above
// its baseline.
const notoAscent = 1.069

// TabLabelTop is the top of a tab label's line whose baseline sits
// hudlayout.TabBaseline below the strip's top y (UIS-043).
func TabLabelTop(y float32, st gfx.TextStyle) float32 {
	return y + hudlayout.TabBaseline - float32(st.Size*notoAscent)
}
