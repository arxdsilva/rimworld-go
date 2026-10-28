package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/platform/input"
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
func tabsRect(sw, sh int) rect { return rect{X: 0, Y: float32(sh - tabsH), W: float32(sw), H: tabsH} }

// tabOn reports tab id's window (or the Architect) is the one open.
func (a *App) tabOn(id string) bool {
	switch id {
	case ids.TabArchitect:
		return a.arch.Open
	case ids.TabWork:
		return a.win.Open && (a.win.Mode == "work" || a.win.Mode == "")
	case ids.TabSchedule:
		return a.win.Open && a.win.Mode == "schedule"
	case ids.TabResearch:
		return a.win.Open && a.win.Mode == "research"
	case ids.TabAnimals:
		return a.win.Open && a.win.Mode == "animals" && !a.win.Wild
	case ids.TabWildlife:
		return a.win.Open && a.win.Mode == "animals" && a.win.Wild
	case ids.TabAssign:
		return a.kitWindowOn(assignModalID)
	case ids.TabWorld:
		return a.kitWindowOn(worldModalID)
	case ids.TabHistory:
		return a.kitWindowOn(historyModalID)
	case ids.TabFactions:
		return a.kitWindowOn(colFacModalID)
	}
	return false
}

// kitWindowOn reports the kit window id is the open one.
func (a *App) kitWindowOn(id string) bool {
	return a.win.KitID() == id
}

func (a *App) hudTabButtons(sw, sh int) []button {
	n := len(hudTabSpecs)
	if n < 1 || sw < 1 {
		return nil
	}
	tr := tabsRect(sw, sh)
	w := tr.W / float32(n)
	out := make([]button, 0, n)
	for i, t := range hudTabSpecs {
		b := button{
			ID:       t.id,
			Label:    t.label,
			X:        tr.X + float32(i)*w,
			Y:        tr.Y,
			W:        w,
			H:        tr.H,
			Selected: a.tabOn(t.id),
		}
		if t.key != "" {
			b.Tip = t.label + a.keyHint(t.key)
		}
		switch t.id {
		case ids.TabResearch:
			if name, pct, ok := a.researchUnderWay(); ok {
				b.Tip = fmt.Sprintf("%s: %s %d%%", b.Tip, name, pct)
			}
		case ids.TabFactions:
			if a.renderer != nil && len(a.renderer.game.Factions()) == 0 {
				b.Disabled, b.Reason = true, "no other peoples in this colony"
				b.Selected = false
			}
		}
		out = append(out, b)
	}
	return out
}

// researchUnderWay is the project being researched: its name and percent.
func (a *App) researchUnderWay() (name string, pct int, ok bool) {
	if a.renderer == nil {
		return "", 0, false
	}
	for _, p := range a.shell.World().Research {
		if p.Current && !p.Done {
			if p.Cost > 0 {
				pct = min(100, 100*p.Progress/p.Cost)
			}
			return p.Label, pct, true
		}
	}
	return "", 0, false
}

// drawTabs draws the bottom tab strip; Research shows a thin fill for the
// project under way.
func (a *App) drawTabs(screen *ebiten.Image, sw, sh int) {
	mx, my := a.currentCursor()
	bs := a.hudTabButtons(sw, sh)
	a.drawButtons(screen, bs, mx, my)
	_, pct, ok := a.researchUnderWay()
	if !ok {
		return
	}
	for _, b := range bs {
		if b.ID != ids.TabResearch {
			continue
		}
		track := rect{X: b.X + 4, Y: b.Y + b.H - 6, W: b.W - 8, H: 3}
		uiFillRect(screen, track.X, track.Y, track.W, track.H, uiSliderTrack, false)
		uiFillRect(screen, track.X, track.Y, track.W*float32(pct)/100, track.H, uiAccent, false)
	}
}
