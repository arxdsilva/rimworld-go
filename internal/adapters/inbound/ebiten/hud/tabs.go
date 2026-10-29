package hud

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"

	"rimworld-go/internal/platform/input"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The bottom tab strip (PLAY-018: WINDOWS owns it): 28 logical px tall, the
// full width, divided into equal tabs. A tab's label ellipsizes when the
// tab is narrow (drawButtonLabel); its tooltip names the live key.

var hudTabSpecs = []struct {
	id, label string
	key       input.Action // its shortcut ("" none)
}{
	{ids.TabArchitect, "Architect", input.TabArchitect},
	{ids.TabWork, "Work", input.TabWork},
	{ids.TabSchedule, "Schedule", input.TabSchedule},
	{ids.TabAssign, "Assign", input.TabAssign},
	{ids.TabAnimals, "Animals", input.TabAnimals},
	{ids.TabWildlife, "Wildlife", input.TabWildlife},
	{ids.TabResearch, "Research", input.TabResearch},
	{ids.TabWorld, "World", input.TabWorld},
	{ids.TabHistory, "History", input.TabHistory},
	{ids.TabFactions, "Factions", ""},
	{ids.TabMenu, "Menu", input.Cancel},
}

// tabsRect is the strip, the layout's Tabs rect.
func tabsRect(sw, sh int) gfx.Rect {
	return gfx.Rect{X: 0, Y: float32(sh - hudlayout.TabsH), W: float32(sw), H: hudlayout.TabsH}
}

// Buttons are the strip's tabs, equal widths across the screen; a
// tab is selected while its window (or the Architect) is the open one.
func (c Tabs) Buttons(sw, sh int) []uikit.Button {
	n := len(hudTabSpecs)
	if n < 1 || sw < 1 {
		return nil
	}
	tr := tabsRect(sw, sh)
	w := tr.W / float32(n)
	out := make([]uikit.Button, 0, n)
	for i, t := range hudTabSpecs {
		b := uikit.Button{
			ID:       t.id,
			Label:    t.label,
			X:        tr.X + float32(i)*w,
			Y:        tr.Y,
			W:        w,
			H:        tr.H,
			Selected: c.sh.TabOn(t.id),
		}
		if t.key != "" {
			b.Tip = t.label + c.sh.KeyHint(t.key)
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
	drawButtons(screen, bs, mx, my)
	_, pct, ok := c.researchUnderWay()
	if !ok {
		return
	}
	for _, b := range bs {
		if b.ID != ids.TabResearch {
			continue
		}
		track := gfx.Rect{X: b.X + 4, Y: b.Y + b.H - 6, W: b.W - 8, H: 3}
		gfx.FillRect(screen, track.X, track.Y, track.W, track.H, uikit.SliderTrack, false)
		gfx.FillRect(screen, track.X, track.Y, track.W*float32(pct)/100, track.H, uikit.Accent, false)
	}
}
