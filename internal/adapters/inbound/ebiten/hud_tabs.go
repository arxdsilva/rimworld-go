package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/input"
)

// The bottom tab strip (PLAY-018: WINDOWS owns it): 28 logical px tall, the
// full width, divided into equal tabs. A tab's label ellipsizes when the
// tab is narrow (drawButtonLabel); its tooltip names the live key.

var hudTabSpecs = []struct {
	id, label string
	key       input.Action // its shortcut ("" none)
}{
	{"tab.architect", "Architect", input.TabArchitect},
	{"tab.work", "Work", input.TabWork},
	{"tab.schedule", "Schedule", input.TabSchedule},
	{"tab.assign", "Assign", input.TabAssign},
	{"tab.animals", "Animals", input.TabAnimals},
	{"tab.wildlife", "Wildlife", input.TabWildlife},
	{"tab.research", "Research", input.TabResearch},
	{"tab.world", "World", input.TabWorld},
	{"tab.history", "History", input.TabHistory},
	{"tab.factions", "Factions", ""},
	{"tab.menu", "Menu", input.Cancel},
}

// tabsRect is the strip, the layout's Tabs rect.
func tabsRect(sw, sh int) rect { return rect{0, float32(sh - tabsH), float32(sw), tabsH} }

// tabOn reports tab id's window (or the Architect) is the one open.
func (a *App) tabOn(id string) bool {
	switch id {
	case "tab.architect":
		return a.archOpen
	case "tab.work":
		return a.workOpen && (a.panelMode == "work" || a.panelMode == "")
	case "tab.schedule":
		return a.workOpen && a.panelMode == "schedule"
	case "tab.research":
		return a.workOpen && a.panelMode == "research"
	case "tab.animals":
		return a.workOpen && a.panelMode == "animals" && !a.animalsWild
	case "tab.wildlife":
		return a.workOpen && a.panelMode == "animals" && a.animalsWild
	case "tab.assign":
		return a.kitWindowOn(assignModalID)
	case "tab.world":
		return a.kitWindowOn(worldModalID)
	case "tab.history":
		return a.kitWindowOn(historyModalID)
	case "tab.factions":
		return a.kitWindowOn(colFacModalID)
	}
	return false
}

// kitWindowOn reports the kit window id is the open one.
func (a *App) kitWindowOn(id string) bool {
	return a.windowKitOpen() && a.hst.win.kit.id == id
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
			id:       t.id,
			label:    t.label,
			x:        tr.X + float32(i)*w,
			y:        tr.Y,
			w:        w,
			h:        tr.H,
			selected: a.tabOn(t.id),
		}
		if t.key != "" {
			b.tip = t.label + a.keyHint(t.key)
		}
		switch t.id {
		case "tab.research":
			if name, pct, ok := a.researchUnderWay(); ok {
				b.tip = fmt.Sprintf("%s: %s %d%%", b.tip, name, pct)
			}
		case "tab.factions":
			if a.renderer != nil && len(a.renderer.game.Factions()) == 0 {
				b.disabled, b.reason = true, "no other peoples in this colony"
				b.selected = false
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
	for _, p := range a.renderer.world().Research {
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
		if b.id != "tab.research" {
			continue
		}
		track := rect{b.x + 4, b.y + b.h - 6, b.w - 8, 3}
		uiFillRect(screen, track.X, track.Y, track.W, track.H, uiSliderTrack, false)
		uiFillRect(screen, track.X, track.Y, track.W*float32(pct)/100, track.H, uiAccent, false)
	}
}
