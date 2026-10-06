package ebiten

import (
	"fmt"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

const (
	hudTopH = 32
	hudTabH = 32
	hudBarH = 56
	hudPad  = 8
	barSlot = 52
)

var hudTabSpecs = []struct {
	id, label string
	enabled   bool
}{
	{"tab.architect", "Architect", true},
	{"tab.work", "Work", true},
	{"tab.schedule", "Schedule", false},
	{"tab.assign", "Assign", false},
	{"tab.animals", "Animals", false},
	{"tab.research", "Research", false},
	{"tab.world", "World", false},
	{"tab.history", "History", false},
	{"tab.factions", "Factions", false},
	{"tab.menu", "Menu", true},
}

func hudTabY(sh int) float32 { return float32(sh - hudTabH) }

func hudBarY(sh int) float32 { return float32(sh - hudTabH - hudBarH) }

func pawnLabel(p game.PawnView) string {
	if p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("Colonist %d", p.ID)
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
		if t.id == "tab.work" && a.workOpen {
			b.selected = true
		}
		if t.id == "tab.architect" && a.archOpen {
			b.selected = true
		}
		out = append(out, b)
	}
	return out
}

func (a *App) colonistBarButtons(sw, sh int) []button {
	if a.renderer == nil {
		return nil
	}
	pawns := a.renderer.snapshot()
	y := hudBarY(sh)
	out := make([]button, 0, len(pawns))
	for i, p := range pawns {
		sel := a.renderer.HasSelect() && a.renderer.Selected() == p.ID
		out = append(out, button{
			id:       "bar.pawn." + strconv.FormatUint(uint64(p.ID), 10),
			label:    "",
			x:        float32(hudPad + i*barSlot),
			y:        y + 4,
			w:        barSlot - 4,
			h:        hudBarH - 8,
			selected: sel,
		})
	}
	return out
}

func (a *App) colonyButtons(sw, sh int) []button {
	out := a.floatButtons() // topmost first: hit-testing takes the first match
	out = append(out, a.hudTabButtons(sw, sh)...)
	out = append(out, a.colonistBarButtons(sw, sh)...)
	out = append(out, a.inspectorButtons()...)
	out = append(out, a.buildingButtons()...)
	out = append(out, a.animalButtons()...)
	out = append(out, a.alertButtons(sw)...)
	out = append(out, a.architectButtons()...)
	out = append(out, a.workButtons()...)
	return out
}

func (a *App) drawColonyHUD(screen *ebiten.Image, sw, sh int) {
	if a.renderer == nil {
		return
	}
	a.drawArchitectGhost(screen)
	drawPanel(screen, 0, 0, float32(sw), hudTopH)
	x := hudPad
	wld := a.renderer.world()
	drawLabel(screen, x, 10, fmt.Sprintf("Food %d", wld.Food))
	x += 90
	drawLabel(screen, x, 10, fmt.Sprintf("Wood %d", wld.Wood))
	x += 90
	drawLabel(screen, x, 10, fmt.Sprintf("Stone %d", wld.Stone))
	time := fmt.Sprintf("%s  %s  %s", a.renderer.date(), a.renderer.speedRate(), a.renderer.activity())
	drawLabel(screen, sw-len(time)*7-hudPad, 10, time)

	drawPanel(screen, 0, hudBarY(sh), float32(sw), hudBarH)
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.hudTabButtons(sw, sh), mx, my)
	a.drawButtons(screen, a.colonistBarButtons(sw, sh), mx, my)
	a.drawColonistPortraits(screen, sh)
	a.drawInspector(screen)
	a.drawBuildingInspector(screen)
	a.drawAnimalInspector(screen)
	a.drawAlerts(screen, sw)
	a.drawArchitect(screen)
	if a.workOpen {
		a.drawWork(screen, sw, sh)
	}
	a.drawFloat(screen)
	a.drawHoverRoom(screen, sh)
}

func (a *App) drawColonistPortraits(screen *ebiten.Image, sh int) {
	pawns := a.renderer.snapshot()
	y := float64(hudBarY(sh) + 6)
	for i, p := range pawns {
		x := float64(hudPad + i*barSlot + 6)
		a.drawPawnPortrait(screen, p, x, y, 40, 48)
		if p.Drafted {
			drawIcon(screen, a.icon("draft"), x+24, y-2, 18)
		}
		vector.FillRect(screen, float32(x), float32(y)+46, 40*float32(p.Mood)/100, 3, moodColor(p.Mood), false)
		drawLabel(screen, int(x), int(hudBarY(sh))+hudBarH-14, pawnLabel(p))
	}
}

func (a *App) drawPawnPortrait(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) {
	if a.renderer.art != nil && a.renderer.drawPawnPortraitArt(screen, p, x, y, w, h) {
		return
	}
	if a.renderer.pack == nil {
		return
	}
	ss := a.renderer.pack.colonist(p.ID)
	if ss == nil {
		return
	}
	img := animFrame(ss.idle, a.renderer.present, uint64(idleAnimPeriod))
	drawImage(screen, img, x, y, w, h, false)
}

func (a *App) colonyChromeHit(mx, my, sw, sh int) bool {
	if my < hudTopH {
		return true
	}
	if a.float.open && a.floatRect().contains(mx, my) {
		return true
	}
	if my >= int(hudBarY(sh)) {
		return true
	}
	if a.workOpen && workRect(sw, sh).contains(mx, my) {
		return true
	}
	if a.archOpen && architectRect().contains(mx, my) {
		return true
	}
	if a.renderer != nil && a.renderer.HasSelect() && !a.workOpen && inspectorRect().contains(mx, my) {
		return true
	}
	if a.hasBuilding && !a.workOpen && !a.archOpen && buildingRect().contains(mx, my) {
		return true
	}
	return false
}

// drawHoverRoom shows the room under the cursor above the colonist bar.
func (a *App) drawHoverRoom(screen *ebiten.Image, sh int) {
	if a.renderer == nil {
		return
	}
	mx, my := a.currentCursor()
	sw, _ := a.size()
	if a.colonyChromeHit(mx, my, sw, sh) {
		return
	}
	tx, ty := a.renderer.TileAt(mx, my)
	if lbl := roomLabelAt(a.renderer.world(), tx, ty); lbl != "" {
		drawLabel(screen, sw-len(lbl)*6-hudPad-4, int(hudBarY(sh))-18, lbl)
	}
}
