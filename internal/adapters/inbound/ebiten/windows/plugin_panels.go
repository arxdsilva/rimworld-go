// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// The Plugins window in the colony: the panels the loaded plugins' scripts
// return each in-game hour (WorldView.PluginPanels), each with its title,
// its text lines and its action buttons. A button press only enqueues
// command.ScriptAction ("plugin\naction"); the window changes no state.
// Opened from the HUD's Plugins tab. Panels past the view are neither
// declared nor drawn.

// PluginPanelsModalID is the window's kit id.
const PluginPanelsModalID = "colony.plugins"

const (
	ppScrollID = "plugins.panels"
	ppW        = 720
	ppH        = 520
	ppPad      = 10
	ppTitleH   = 28
	ppLineH    = 18
	ppActH     = 30
	ppGap      = 14
	ppBtnW     = 150
)

// Toggle opens the window, or closes it when it is the open one.
func (w PluginPanels) Toggle() { w.toggleKit(PluginPanelsModalID, w.Launch) }

// Launch opens the window: a main-tab window in the Window rect.
func (w PluginPanels) Launch() {
	if !w.live() {
		return
	}
	w.OpenKit(uikit.Modal{
		ID: PluginPanelsModalID, Title: w.sh.T("Plugins"), W: ppW, H: ppH, Anchor: w.Anchor, Frameless: true,
		Draw: func(body gfx.Rect) { w.body(body) },
	})
}

// panelHeight is one panel's height in px, its trailing gap included.
func panelHeight(p game.PluginPanelView) float32 {
	h := float32(ppTitleH + len(p.Lines)*ppLineH + ppGap)
	if len(p.Actions) > 0 {
		h += ppActH + 6
	}
	return h
}

// panelText is a drawn string at a content-space position.
type panelText struct {
	s    string
	x, y float32
	st   gfx.TextStyle
}

// body declares the visible panels' buttons and queues their text.
func (w PluginPanels) body(body gfx.Rect) {
	if !w.live() {
		return
	}
	k := w.kit()
	panels := w.sh.World().PluginPanels
	if len(panels) == 0 {
		ds := uikit.DimText()
		k.Label(body.X+ppPad, body.Y+ppPad, uikit.FitText(w.sh.T("No plugin panels yet (panels refresh each in-game hour)"), ds, body.W-2*ppPad), ds)
		return
	}
	total := float32(0)
	for _, p := range panels {
		total += panelHeight(p)
	}
	area := k.ScrollArea(ppScrollID, body, total, uikit.ScrollOpts{Step: 48, Focusable: true})
	textW := area.View.W - 2*ppPad
	hs := gfx.Title
	hs.Color = uikit.Heading
	ls := gfx.Label
	var texts []panelText
	y := area.View.Y - area.Off
	for _, p := range panels {
		h := panelHeight(p)
		if y+h < area.View.Y || y > area.View.Bottom() { // virtualised
			y += h
			continue
		}
		x := area.View.X + ppPad
		texts = append(texts, panelText{uikit.FitText(p.Title, hs, textW), x, y, hs})
		ty := y + ppTitleH
		for _, l := range p.Lines {
			texts = append(texts, panelText{uikit.FitText(l, ls, textW), x, ty, ls})
			ty += ppLineH
		}
		for i, a := range p.Actions {
			r := gfx.Rect{X: x + float32(i)*(ppBtnW+8), Y: ty + 4, W: ppBtnW, H: ppActH}
			if r.Intersect(area.View) != r { // only a whole button takes clicks
				continue
			}
			if k.Button(ids.PluginAction+p.Plugin+"."+a.ID, r, uikit.FitText(a.Label, gfx.Body, ppBtnW-12), uikit.BtnOpts{}) {
				w.sh.Enqueue(command.Command{Kind: command.ScriptAction, Name: p.Plugin + "\n" + a.ID})
			}
		}
		y += h
	}
	view := area.View
	k.Queue(func(dst *ebiten.Image) {
		clip := gfx.ClipTo(dst, view)
		for _, t := range texts {
			gfx.Text(clip, t.s, float64(t.x), float64(t.y), t.st)
		}
	})
}
