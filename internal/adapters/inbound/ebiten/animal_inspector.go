package ebiten

import (
	"fmt"
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
	if an, ok := a.selectedAnimal(); ok {
		h += float32(animalExtraH(an))
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
	drawCentered(dst, animalInitial(animalLabel(an)), x+float64(r), y+float64(r), tsLabel)
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
	drawFitSmall(screen, animalTitle(an), float64(r.x)+60, float64(r.y)+8, float64(r.w)-60-36, tsLabel)
	state := an.Mode
	switch {
	case an.Manhunter:
		state = "MANHUNTER: hunting colonists!"
	case an.Hostile:
		state = "attacking a colonist!"
	case an.TameMark:
		state += ", marked for taming"
	}
	if st := a.animalStateLine(an); st != "" && !an.Manhunter && !an.Hostile {
		state = st
		if an.TameMark {
			state += ", marked for taming"
		}
	}
	drawFitSmall(screen, state, float64(r.x)+60, float64(r.y)+27, float64(r.w)-72, tsSmall)
	if l := animalLifeLine(an); l != "" {
		drawFitSmall(screen, l, float64(r.x)+60, float64(r.y)+41, float64(r.w)-72, tsSmall)
	}
	for i, ln := range wrapPx(animalBlurb(an.Species), withColor(tsSmall, uiTextDim), float64(r.w)-24) {
		if i > 1 {
			break
		}
		drawText(screen, ln, float64(r.x)+12, float64(r.y)+55+float64(i*13), withColor(tsSmall, uiTextDim))
	}
	drawNeedBar(screen, int(r.x)+12, int(r.y)+84, 270, 14, an.Health, "health")
	a.drawAnimalNeeds(screen, an, r)
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

// Animal needs, life and products (plan 6.2): the inspector's block
// under the health bar.

// animalStateLine is what the animal is doing: "Feeding", or "Wild |
// Wandering" for a wild one (the Mode when the sim gives no State).
func (a *App) animalStateLine(an game.AnimalView) string {
	st := an.State
	if st == "" {
		st = an.Mode
	}
	if st == "" {
		return ""
	}
	if !an.Tame {
		return "Wild | " + capitalize(st)
	}
	return capitalize(st)
}

// animalLifeLine is "Female | adult | 34 days old" (the sex, stage and
// age), "" when the view carries none of it.
func animalLifeLine(an game.AnimalView) string {
	if an.AgeDays == 0 && an.Stage == "" && an.Scale == 0 {
		return ""
	}
	sex := "Male"
	if an.Female {
		sex = "Female"
	}
	stage := an.Stage
	if stage == "" {
		stage = "adult"
		if !an.Adult {
			stage = "young"
		}
	}
	return fmt.Sprintf("%s | %s | %d days old", sex, stage, an.AgeDays)
}

// animalHasNeeds reports whether the view carries needs to show.
func animalHasNeeds(an game.AnimalView) bool { return an.State != "" }

func animalPregLine(an game.AnimalView) string {
	switch {
	case an.Pregnant:
		return fmt.Sprintf("Pregnant %d%%", an.PregPct)
	case an.EggsFertile:
		return "Eggs are fertile"
	}
	return ""
}

func animalDietLine(an game.AnimalView) string {
	if an.Diet == "" {
		return ""
	}
	return "Diet: " + an.Diet
}

// animalExtraH is how tall the needs block is: Food and Rest bars, the
// product bar, a pregnancy line, the diet line and a wild animal's tame
// chance.
func animalExtraH(an game.AnimalView) int {
	if !animalHasNeeds(an) {
		return 0
	}
	h := 40 // food, rest
	if an.Product != "" {
		h += 20
	}
	if animalPregLine(an) != "" {
		h += 16
	}
	if animalDietLine(an) != "" {
		h += 16
	}
	if !an.Tame {
		h += 16 // tame chance
	}
	return h
}

func (a *App) drawAnimalNeeds(screen *ebiten.Image, an game.AnimalView, r button) {
	if !animalHasNeeds(an) {
		return
	}
	x, y := int(r.x)+12, int(r.y)+104
	drawNeedBar(screen, x, y, 270, 14, an.Food, "food")
	y += 20
	drawNeedBar(screen, x, y, 270, 14, an.Rest, "rest")
	y += 20
	if an.Product != "" {
		drawNeedBar(screen, x, y, 270, 14, an.ProductPct, an.Product)
		y += 20
	}
	if l := animalPregLine(an); l != "" {
		drawLabel(screen, x, y, l)
		y += 16
	}
	if l := animalDietLine(an); l != "" {
		drawLabel(screen, x, y, l)
		y += 16
	}
	if !an.Tame {
		if row, ok := a.wildRow(an.Species); ok {
			drawLabel(screen, x, y, fmt.Sprintf("Tame chance %d%%", row.TameChance))
		}
	}
}
