package ebiten

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Animal selection, its inspector (hunt toggle) and the hostile alert.

var alertClr = color.RGBA{R: 0xD8, G: 0x4A, B: 0x3C, A: 0xFF}

func animalAt(w game.WorldView, x, y int) (game.AnimalView, bool) {
	for _, an := range w.Animals {
		if an.X == x && an.Y == y {
			return an, true
		}
	}
	return game.AnimalView{}, false
}

func (a *App) selectAnimalAt(tx, ty int) bool {
	an, ok := animalAt(a.renderer.world(), tx, ty)
	if !ok {
		a.hasAnimal = false
		return false
	}
	a.selectCand(selCand{selAnimal, an.ID})
	return true
}

func (a *App) clearAnimal() { a.hasAnimal = false }

func (a *App) selectedAnimal() (game.AnimalView, bool) {
	if !a.hasAnimal || a.renderer == nil {
		return game.AnimalView{}, false
	}
	for _, an := range a.renderer.world().Animals {
		if an.ID == a.selAnimal {
			return an, true
		}
	}
	a.hasAnimal = false // it died or left
	return game.AnimalView{}, false
}

const (
	animalPanelH     = 222 // with the Recent list
	animalPanelFullH = 340 // with the full scrollable log open
)

// animalPanel is the inspector rect; it grows while the full log is open.
func (a *App) animalPanel() button {
	h := float32(animalPanelH)
	if a.animalLogOpen {
		h = animalPanelFullH
	}
	return button{x: hudPad, y: hudTopH + hudPad, w: 300, h: h}
}

func (a *App) animalButtons() []button {
	an, ok := a.selectedAnimal()
	if !ok || a.workOpen || a.archOpen {
		return nil
	}
	a.syncAnimalLog(an.ID)
	r := a.animalPanel()
	label := "Hunt"
	switch {
	case an.Designated:
		label = "Cancel hunt"
	case an.Tame:
		label = "Slaughter"
	}
	out := []button{
		{id: "animal.close", label: "X", x: r.x + r.w - 28, y: r.y + 6, w: 22, h: 22},
		{id: "animal.hunt", label: label, x: r.x + 12, y: r.y + r.h - 34, w: 120, h: 24, selected: an.Designated},
	}
	logLabel := "Log"
	if a.animalLogOpen {
		logLabel = "Less"
	}
	out = append(out, button{id: "animal.log", label: logLabel, x: r.x + 248, y: r.y + r.h - 34, w: 40, h: 24, selected: a.animalLogOpen})
	switch {
	case an.Tame:
		out = append(out, button{id: "animal.release", label: "Release", x: r.x + 140, y: r.y + r.h - 34, w: 100, h: 24})
	case !an.Manhunter:
		tl := "Tame"
		if an.TameMark {
			tl = "Cancel tame"
		}
		out = append(out, button{id: "animal.tame", label: tl, x: r.x + 140, y: r.y + r.h - 34, w: 100, h: 24, selected: an.TameMark})
	}
	return out
}

func animalTitle(an game.AnimalView) string {
	s := strings.ToUpper(an.Species[:1]) + an.Species[1:]
	if an.Tame && an.Name != "" {
		return an.Name + " (tame " + an.Species + ")"
	}
	return s
}

func animalBlurb(species string) string {
	switch species {
	case "dustbuck":
		return "Skittish grazer. Flees people. Hunt for meat."
	case "ridgewolf":
		return "Predator. Hunts dustbucks; bites back."
	}
	return ""
}

func (a *App) drawAnimalInspector(screen *ebiten.Image) {
	an, ok := a.selectedAnimal()
	if !ok || a.workOpen || a.archOpen {
		return
	}
	a.syncAnimalLog(an.ID)
	r := a.animalPanel()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	if a.renderer.art != nil {
		if img := a.renderer.art.image("ui_icon/"+an.Species, color.NRGBA{}); img != nil {
			drawIcon(screen, img, float64(r.x)+12, float64(r.y)+10, 40)
		}
	}
	drawLabel(screen, int(r.x)+60, int(r.y)+12, animalTitle(an))
	state := an.Mode
	switch {
	case an.Manhunter:
		state = "MANHUNTER: hunting colonists!"
	case an.Hostile:
		state = "attacking a colonist!"
	case an.TameMark:
		state += ", marked for taming"
	}
	drawLabel(screen, int(r.x)+60, int(r.y)+30, state)
	drawLabel(screen, int(r.x)+12, int(r.y)+58, animalBlurb(an.Species))
	drawNeedBar(screen, int(r.x)+12, int(r.y)+84, 270, 14, an.Health, "health")
	a.drawAnimalLog(screen, an, r)
	if an.Designated {
		drawLabel(screen, int(r.x)+150, int(r.y+r.h)-28, "marked for hunting")
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.animalButtons(), mx, my)
	if a.renderer.art != nil {
		wx, wy := a.renderer.interpAnimal(an)
		a.renderer.drawSprite(screen, "overlay/selection", color.NRGBA{}, wx, wy, a.renderer.tilePx(), a.sw, a.sh)
	}
}

func (a *App) applyAnimalClick(id string) {
	an, ok := a.selectedAnimal()
	if !ok {
		return
	}
	switch id {
	case "animal.close":
		a.clearAnimal()
	case "animal.log":
		a.animalLogOpen = !a.animalLogOpen
		a.animalLogScroll = 0
	case "animal.hunt":
		c := command.Command{Kind: command.DesignateHunt, X: an.X, Y: an.Y, X2: an.X, Y2: an.Y}
		if an.Designated { // only the mark: a pen or zone underneath stays
			c.Kind, c.Order = command.Undesignate, command.OrderHunt
		}
		a.renderer.enqueue(c)
	case "animal.tame":
		c := command.Command{Kind: command.DesignateTame, X: an.X, Y: an.Y, X2: an.X, Y2: an.Y}
		if an.TameMark {
			c.Kind, c.Order = command.Undesignate, command.OrderTame
		}
		a.renderer.enqueue(c)
	case "animal.release":
		a.renderer.enqueue(command.Command{Kind: command.ReleaseAnimal, Pawn: uint64(an.ID)})
	}
}
