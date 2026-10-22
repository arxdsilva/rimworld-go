package ebiten

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

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

// animalLabel is the species' display name ("arctic fox"): the view's
// Label, else the key with its underscores spaced out.
func animalLabel(an game.AnimalView) string {
	switch {
	case an.Label != "":
		return an.Label
	case an.Species != "":
		return strings.ReplaceAll(an.Species, "_", " ")
	}
	return "animal"
}

func animalTitle(an game.AnimalView) string {
	l := animalLabel(an)
	if an.Tame && an.Name != "" {
		return an.Name + " (tame " + l + ")"
	}
	return strings.ToUpper(l[:1]) + l[1:]
}

// animalInitial is the letter shown on the fallback icon disc.
func animalInitial(label string) string {
	for _, r := range label {
		if r != ' ' && r != '_' {
			return strings.ToUpper(string(r))
		}
	}
	return "?"
}

var iconDiscClr = color.RGBA{R: 0x4A, G: 0x3C, B: 0x2C, A: 0xFF}

// drawAnimalIcon draws the species' ui_icon at (x, y), size px square; a
// species without one gets its initial on a disc.
func drawAnimalIcon(dst *ebiten.Image, art *artCatalog, an game.AnimalView, x, y, size float64) {
	if img := art.image("ui_icon/"+an.Species, color.NRGBA{}); img != nil {
		drawIcon(dst, img, x, y, size)
		return
	}
	r := float32(size) / 2
	vector.FillCircle(dst, float32(x)+r, float32(y)+r, r, iconDiscClr, true)
	vector.StrokeCircle(dst, float32(x)+r, float32(y)+r, r, 1.5, color.RGBA{R: 0xC8, G: 0xB0, B: 0x82, A: 0xFF}, true)
	drawLabel(dst, int(x+float64(r))-3, int(y+float64(r))-7, animalInitial(animalLabel(an)))
}

func (a *App) drawAnimalInspector(screen *ebiten.Image) {
	an, ok := a.selectedAnimal()
	if !ok || a.workOpen || a.archOpen {
		return
	}
	a.syncAnimalLog(an.ID)
	r := a.animalPanel()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawAnimalIcon(screen, a.renderer.art, an, float64(r.x)+12, float64(r.y)+10, 40)
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
