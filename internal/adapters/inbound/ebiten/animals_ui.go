package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Animals tab: the colony's tame animals, with slaughter and release.

const animalRowH = 30

func (a *App) tameAnimals() []game.AnimalView {
	var out []game.AnimalView
	for _, an := range a.renderer.world().Animals {
		if an.Tame {
			out = append(out, an)
		}
	}
	return out
}

// Column x offsets (px from the panel's left) of the Colony list.
const (
	colName    = 52
	colSpecies = 140
	colSex     = 262
	colAge     = 292
	colFood    = 346
	colProduct = 432
	colPreg    = 530
	colHealth  = 600
	colMaster  = 652
	colTrain   = 736
	colDoing   = 856
	colBtn     = 1040 // Slaughter, then Release
)

// Column x offsets of the Wild list.
const (
	wildLabel     = 52
	wildCount     = 190
	wildDiet      = 240
	wildPred      = 340
	wildTame      = 392
	wildMeat      = 490
	wildMarks     = 576
	wildBtn       = 700 // Hunt all, Tame one, Cancel
	animalsRowTop = 84
)

// wildlife is the game's wild species list, read again only when the
// simulation tick moves.
func (a *App) wildlife() []game.WildlifeRow {
	if a.renderer == nil {
		return nil
	}
	t := a.renderer.game.Tick()
	if !a.wildRead || t != a.wildTick {
		a.wildRows, a.wildTick, a.wildRead = a.renderer.game.Wildlife(), t, true
	}
	return a.wildRows
}

// wildRow is the Wild list's row of a species.
func (a *App) wildRow(species string) (game.WildlifeRow, bool) {
	for _, r := range a.wildlife() {
		if r.Species == species {
			return r, true
		}
	}
	return game.WildlifeRow{}, false
}

func (a *App) animalsTabButtons() []button {
	r := workRect(a.sw, a.sh)
	out := []button{
		{id: "animals.view.colony", label: "Colony", x: r.x + r.w - 190, y: r.y + 6, w: 84, h: 24, selected: !a.animalsWild},
		{id: "animals.view.wild", label: "Wild", x: r.x + r.w - 100, y: r.y + 6, w: 84, h: 24, selected: a.animalsWild},
	}
	if a.animalsWild {
		for i, row := range a.wildlife() {
			y := r.y + animalsRowTop + float32(i*animalRowH)
			if y+animalRowH > r.y+r.h {
				break
			}
			out = append(out,
				button{id: "animals.focus." + row.Species, x: r.x + 12, y: y, w: wildBtn - 24, h: animalRowH - 2},
				button{id: "animals.wild.hunt." + row.Species, label: "Hunt all", x: r.x + wildBtn, y: y, w: 90, h: 24, selected: row.Hunted > 0},
				button{id: "animals.wild.tame." + row.Species, label: "Tame one", x: r.x + wildBtn + 96, y: y, w: 90, h: 24, selected: row.TameMarked > 0},
				button{id: "animals.wild.cancel." + row.Species, label: "Cancel", x: r.x + wildBtn + 192, y: y, w: 80, h: 24},
			)
		}
		return out
	}
	for i, an := range a.tameAnimals() {
		y := r.y + animalsRowTop + float32(i*animalRowH)
		if y+animalRowH > r.y+r.h {
			break
		}
		id := strconv.FormatUint(uint64(an.ID), 10)
		slaughter := "Slaughter"
		if an.Designated {
			slaughter = "Spare"
		}
		out = append(out,
			button{id: "animals.slaughter." + id, label: slaughter, x: r.x + colBtn, y: y, w: 100, h: 24, selected: an.Designated},
			button{id: "animals.release." + id, label: "Release", x: r.x + colBtn + 106, y: y, w: 90, h: 24},
		)
	}
	return out
}

func (a *App) applyAnimalsTab(id string) {
	parts := strings.Split(id, ".")
	if len(parts) < 3 || a.renderer == nil {
		return
	}
	switch parts[1] {
	case "view":
		a.animalsWild = parts[2] == "wild"
		return
	case "focus":
		a.focusWild(parts[2])
		return
	case "wild":
		if len(parts) == 4 {
			a.applyWildAction(parts[2], parts[3])
		}
		return
	}
	n, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return
	}
	for _, an := range a.tameAnimals() {
		if uint64(an.ID) != n {
			continue
		}
		switch parts[1] {
		case "slaughter":
			c := command.Command{Kind: command.DesignateHunt, X: an.X, Y: an.Y, X2: an.X, Y2: an.Y}
			if an.Designated { // only the mark: the pen stays
				c.Kind, c.Order = command.Undesignate, command.OrderHunt
			}
			a.renderer.enqueue(c)
		case "release":
			a.renderer.enqueue(command.Command{Kind: command.ReleaseAnimal, Pawn: n})
		}
	}
}

// wildOf lists the wild (not tame) animals of a species.
func (a *App) wildOf(species string) []game.AnimalView {
	var out []game.AnimalView
	for _, an := range a.renderer.world().Animals {
		if !an.Tame && an.Species == species {
			out = append(out, an)
		}
	}
	return out
}

// applyWildAction emits the existing designation commands for a species
// row: hunt every animal of it, tame one, or cancel its marks.
func (a *App) applyWildAction(verb, species string) {
	at := func(kind command.Kind, an game.AnimalView) command.Command {
		return command.Command{Kind: kind, X: an.X, Y: an.Y, X2: an.X, Y2: an.Y}
	}
	switch verb {
	case "hunt":
		for _, an := range a.wildOf(species) {
			if !an.Designated {
				a.renderer.enqueue(at(command.DesignateHunt, an))
			}
		}
	case "tame":
		for _, an := range a.wildOf(species) {
			if !an.TameMark && !an.Manhunter {
				a.renderer.enqueue(at(command.DesignateTame, an))
				return
			}
		}
	case "cancel":
		for _, an := range a.wildOf(species) {
			if an.Designated {
				c := at(command.Undesignate, an)
				c.Order = command.OrderHunt
				a.renderer.enqueue(c)
			}
			if an.TameMark {
				c := at(command.Undesignate, an)
				c.Order = command.OrderTame
				a.renderer.enqueue(c)
			}
		}
	}
}

// focusWild centres the camera on the next animal of a species, cycling.
func (a *App) focusWild(species string) {
	list := a.wildOf(species)
	if len(list) == 0 {
		return
	}
	if a.alertCycle == nil {
		a.alertCycle = map[string]int{}
	}
	key := "wild." + species
	an := list[a.alertCycle[key]%len(list)]
	a.alertCycle[key]++
	a.renderer.centerOn(float64(an.X)+0.5, float64(an.Y)+0.5, a.sw, a.sh)
}

func (a *App) drawAnimalsTab(screen *ebiten.Image, sw, sh int) {
	r := workRect(sw, sh)
	drawPanel(screen, r.x, r.y, r.w, r.h)
	w := a.renderer.world()
	marked := 0
	for _, an := range w.Animals {
		if an.TameMark {
			marked++
		}
	}
	drawText(screen, fmt.Sprintf("Animals  (%d tame, %d marked for taming, %d pens)", len(a.tameAnimals()), marked, countZones(w, game.ZonePen)), float64(r.x)+12, float64(r.y)+8, tsLabel)
	mx, my := a.currentCursor()
	if a.animalsWild {
		a.drawWildList(screen, r)
	} else {
		a.drawColonyList(screen, r)
	}
	for _, b := range a.animalsTabButtons() {
		if !strings.HasPrefix(b.id, "animals.focus.") {
			drawButton(screen, b, b.contains(mx, my))
		}
	}
}

func (a *App) drawColonyList(screen *ebiten.Image, r button) {
	x0, y0 := int(r.x), int(r.y)
	drawFitSmall(screen, "Tame with Architect > Orders > Tame (hunters do it). Handle work tames, trains, milks and shears; pens keep animals in place. Pick a master and training in the inspector.", float64(x0+12), float64(y0+34), float64(colBtn-24), tsBody)
	head := y0 + animalsRowTop - 22
	for _, h := range []struct {
		x int
		s string
	}{{colName, "Name"}, {colSpecies, "Species"}, {colSex, "Sex"}, {colAge, "Age"}, {colFood, "Food"},
		{colProduct, "Product"}, {colPreg, "Preg."}, {colHealth, "Health"}, {colMaster, "Master"}, {colTrain, "Training"}, {colDoing, "Doing"}} {
		drawLabel(screen, x0+h.x, head, h.s)
	}
	list := a.tameAnimals()
	for i, an := range list {
		y := y0 + animalsRowTop + i*animalRowH
		if y+animalRowH > int(r.y+r.h) {
			break
		}
		drawAnimalIcon(screen, a.renderer.art, an, float64(r.x)+16, float64(y), 26)
		ty := y + 6
		drawLabelClip(screen, x0+colName, ty, colSpecies-colName-6, an.Name)
		drawLabelClip(screen, x0+colSpecies, ty, colSex-colSpecies-6, animalLabel(an))
		drawLabelClip(screen, x0+colSex, ty, colAge-colSex-4, animalSexLetter(an))
		drawLabelClip(screen, x0+colAge, ty, colFood-colAge-6, animalAgeText(an))
		if animalHasNeeds(an) {
			drawMiniBar(screen, x0+colFood, y+8, 70, 10, an.Food, an.Food < hungryBelow)
		}
		drawLabelClip(screen, x0+colProduct, ty, colPreg-colProduct-6, animalProductText(an))
		drawLabelClip(screen, x0+colPreg, ty, colHealth-colPreg-6, animalPregCell(an))
		drawLabelClip(screen, x0+colHealth, ty, colMaster-colHealth-6, fmt.Sprintf("%d%%", an.Health))
		drawLabelClip(screen, x0+colMaster, ty, colTrain-colMaster-6, animalMasterCell(an))
		drawLabelClip(screen, x0+colTrain, ty, colDoing-colTrain-6, animalTrainingCell(an))
		drawLabelClip(screen, x0+colDoing, ty, colBtn-colDoing-8, animalDoing(an))
	}
	if len(list) == 0 {
		drawLabel(screen, int(r.x)+52, int(r.y)+animalsRowTop, "No tame animals yet.")
	}
}

func (a *App) drawWildList(screen *ebiten.Image, r button) {
	x0, y0 := int(r.x), int(r.y)
	drawFitSmall(screen, "Wild animals on the map. Click a row to look at one; Hunt all marks every animal of it, Tame one marks a single animal.", float64(x0+12), float64(y0+34), float64(wildBtn-24), tsBody)
	head := y0 + animalsRowTop - 22
	for _, h := range []struct {
		x int
		s string
	}{{wildLabel, "Species"}, {wildCount, "Count"}, {wildDiet, "Diet"}, {wildPred, "Pred."}, {wildTame, "Tame chance"}, {wildMeat, "Meat each"}, {wildMarks, "Marked"}} {
		drawLabel(screen, x0+h.x, head, h.s)
	}
	rows := a.wildlife()
	for i, row := range rows {
		y := y0 + animalsRowTop + i*animalRowH
		if y+animalRowH > int(r.y+r.h) {
			break
		}
		drawAnimalIcon(screen, a.renderer.art, game.AnimalView{Species: row.Species, Label: row.Label}, float64(r.x)+16, float64(y), 26)
		ty := y + 6
		drawLabelClip(screen, x0+wildLabel, ty, wildCount-wildLabel-6, row.Label)
		drawLabelClip(screen, x0+wildCount, ty, wildDiet-wildCount-6, fmt.Sprintf("x%d", row.Count))
		drawLabelClip(screen, x0+wildDiet, ty, wildPred-wildDiet-6, row.Diet)
		if row.Predator {
			drawWarning(screen, float32(x0+wildPred+6), float32(y+13))
		}
		drawLabelClip(screen, x0+wildTame, ty, wildMeat-wildTame-6, fmt.Sprintf("%d%%", row.TameChance))
		drawLabelClip(screen, x0+wildMeat, ty, wildMarks-wildMeat-6, strconv.Itoa(row.Meat))
		drawLabelClip(screen, x0+wildMarks, ty, wildBtn-wildMarks-8, fmt.Sprintf("hunt %d, tame %d", row.Hunted, row.TameMarked))
	}
	if len(rows) == 0 {
		drawLabel(screen, x0+52, y0+animalsRowTop, "No wild animals in sight.")
	}
}

// hungryBelow is the food percent under which a tame animal is hungry.
const hungryBelow = 25

// animalSexLetter is "F" or "M".
func animalSexLetter(an game.AnimalView) string {
	if an.Female {
		return "F"
	}
	return "M"
}

// animalAgeText is the age in days, "young" ones marked.
func animalAgeText(an game.AnimalView) string {
	if !animalHasNeeds(an) && an.AgeDays == 0 {
		return "-"
	}
	if !an.Adult {
		return fmt.Sprintf("%dd (young)", an.AgeDays)
	}
	return fmt.Sprintf("%dd", an.AgeDays)
}

// animalProductText is "milk 70%" or "-".
func animalProductText(an game.AnimalView) string {
	if an.Product == "" {
		return "-"
	}
	return fmt.Sprintf("%s %d%%", an.Product, an.ProductPct)
}

// animalPregCell is the Colony list's pregnancy column.
func animalPregCell(an game.AnimalView) string {
	if an.Pregnant {
		return fmt.Sprintf("%d%%", an.PregPct)
	}
	if an.EggsFertile {
		return "eggs"
	}
	return "-"
}

// animalMasterCell is the Colony list's Master column ("-": none).
func animalMasterCell(an game.AnimalView) string {
	if an.Master == "" {
		return "-"
	}
	return an.Master
}

// animalTrainingCell is the Colony list's Training column: each skill
// handlers train, with its steps ("Obedience 2/3, Guard 0/3"), or "-".
func animalTrainingCell(an game.AnimalView) string {
	var parts []string
	for _, t := range an.Training {
		if t.Allowed && (t.On || t.Steps > 0) {
			parts = append(parts, fmt.Sprintf("%s %d/%d", t.Label, t.Steps, t.Needed))
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// animalDoing is what the Colony list's last column says.
func animalDoing(an game.AnimalView) string {
	switch {
	case an.Designated:
		return "marked for slaughter"
	case an.State != "":
		return an.State
	}
	return an.Mode
}

// drawMiniBar is a thin percent bar; low ones are red.
func drawMiniBar(screen *ebiten.Image, x, y, w, h, pct int, low bool) {
	pct = max(0, min(100, pct))
	vector.FillRect(screen, float32(x), float32(y), float32(w), float32(h), color.RGBA{R: 40, G: 30, B: 24, A: 255}, false)
	fill := color.RGBA{R: 120, G: 170, B: 70, A: 255}
	if low {
		fill = alertClr
	}
	vector.FillRect(screen, float32(x), float32(y), float32(w*pct/100), float32(h), fill, false)
}

// drawWarning is a red "!" disc: the predator flag.
func drawWarning(screen *ebiten.Image, cx, cy float32) {
	vector.FillCircle(screen, cx, cy, 8, alertClr, true)
	drawCentered(screen, "!", float64(cx), float64(cy), tsLabel)
}

// animalRow is one line of the Animals tab: name, species label (long
// ones like "yorkshire terrier" fit), health and what it is doing.
func animalRow(an game.AnimalView) string {
	doing := an.Mode
	if an.Designated {
		doing = "marked for slaughter"
	}
	return fmt.Sprintf("%-13s %-18s %3d%%    %s", an.Name, animalLabel(an), an.Health, doing)
}

// countZones is how many zones of kind the world has.
func countZones(w game.WorldView, kind string) int {
	n := 0
	for _, z := range w.Zones {
		if z.Kind == kind {
			n++
		}
	}
	return n
}
