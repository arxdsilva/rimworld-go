package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/input"
)

// The bottom tab strip (PLAY-018: WINDOWS owns it).

var hudTabSpecs = []struct {
	id, label string
	enabled   bool
	key       input.Action // its shortcut ("" none)
}{
	{"tab.architect", "Architect", true, input.TabArchitect},
	{"tab.work", "Work", true, input.TabWork},
	{"tab.schedule", "Schedule", true, input.TabSchedule},
	{"tab.assign", "Assign", true, input.TabAssign},
	{"tab.animals", "Animals", true, input.TabAnimals},
	{"tab.research", "Research", true, input.TabResearch},
	{"tab.world", "World", true, input.TabWorld},
	{"tab.history", "History", true, input.TabHistory},
	{"tab.factions", "Factions", true, ""},
	{"tab.menu", "Menu", true, input.Cancel},
}

func (a *App) hudTabButtons(sw, sh int) []button {
	n := len(hudTabSpecs)
	if n < 1 || sw < 1 {
		return nil
	}
	w := float32(sw) / float32(n)
	y := hudTabY(sh)
	out := make([]button, 0, n)
	for i, t := range hudTabSpecs {
		b := button{
			id:    t.id,
			label: t.label,
			x:     float32(i) * w,
			y:     y,
			w:     w,
			h:     hudTabH,
		}
		if !t.enabled {
			b.disabled = true
			b.reason = "not implemented"
		}
		if t.key != "" {
			b.tip = t.label + a.keyHint(t.key)
		}
		if t.id == "tab.assign" && a.renderer != nil {
			b.selected = a.uiModalTop() == assignModalID
		}
		if t.id == "tab.history" && a.renderer != nil {
			b.selected = a.uiModalTop() == historyModalID
		}
		if t.id == "tab.factions" && a.renderer != nil {
			if len(a.renderer.game.Factions()) == 0 {
				b.disabled, b.reason = true, "no other peoples in this colony"
			} else {
				b.selected = a.uiModalTop() == colFacModalID
			}
		}
		if t.id == "tab.work" && a.workOpen && (a.panelMode == "work" || a.panelMode == "") {
			b.selected = true
		}
		if t.id == "tab.animals" && a.workOpen && a.panelMode == "animals" {
			b.selected = true
		}
		if t.id == "tab.research" && a.workOpen && a.panelMode == "research" {
			b.selected = true
		}
		if t.id == "tab.schedule" && a.workOpen && a.panelMode == "schedule" {
			b.selected = true
		}
		if t.id == "tab.architect" && a.archOpen {
			b.selected = true
		}
		out = append(out, b)
	}
	return out
}

// drawTabs draws the bottom tab strip.
func (a *App) drawTabs(screen *ebiten.Image, sw, sh int) {
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.hudTabButtons(sw, sh), mx, my)
}

// tabsChrome is the tab strip's area while it is drawn the legacy way
// (the layout's Tabs rect covers it once migrated).
func (a *App) tabsChrome(sw, sh int) []rect {
	return []rect{{0, hudTabY(sh), float32(sw), hudTabH}}
}
