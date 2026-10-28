package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Animals tab: the colony's tame animals, with slaughter and release.

const animalRowH = 30

func (a *App) tameAnimals() []game.AnimalView {
	var out []game.AnimalView
	for _, an := range a.shell.World().Animals {
		if an.Tame {
			out = append(out, an)
		}
	}
	return out
}

// Column x offsets (px from the panel's left) of the Colony list.
const (
	colName    = 52
	colSpecies = 134
	colSex     = 232
	colAge     = 258
	colFood    = 322
	colProduct = 396
	colPreg    = 468
	colHealth  = 512
	colMaster  = 558 // a dropdown button (PLAY-015)
	colFollow  = 650 // follows its drafted master
	colArea    = 706 // area dropdown, or the pen's name
	colTrain   = 828
	colDoing   = 932
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

// animalCol is a column of the Colony or Wild list that is left out when
// the window is narrow: its x in the full layout (the constants above) and
// its width.
type animalCol struct{ x, w int }

// The columns left out first when the window is narrower than the full
// layout, in that order, and the full layouts' widths (right padding
// included). The ones kept are the ones the buttons need.
var (
	colonyDrops = []animalCol{{colDoing, colBtn - colDoing}, {colTrain, colDoing - colTrain}, {colPreg, colHealth - colPreg},
		{colProduct, colPreg - colProduct}, {colAge, colFood - colAge}, {colSex, colAge - colSex}}
	wildDrops = []animalCol{{wildMarks, wildBtn - wildMarks}, {wildMeat, wildMarks - wildMeat}, {wildDiet, wildPred - wildDiet}}
)

const (
	colonyFullW = colBtn + 196 + 12
	wildFullW   = wildBtn + 272 + 12
)

// animalCols are the columns of a list left out to fit a window.
type animalCols []animalCol

// animalColsFor is the columns the Colony (or Wild) list leaves out in a
// window w px wide.
func animalColsFor(w float32, wild bool) animalCols {
	drops, full := colonyDrops, colonyFullW
	if wild {
		drops, full = wildDrops, wildFullW
	}
	var out animalCols
	for _, d := range drops {
		if float32(full) <= w {
			break
		}
		out = append(out, d)
		full -= d.w
	}
	return out
}

// shown reports the column at full-layout x is kept.
func (c animalCols) shown(x int) bool {
	for _, h := range c {
		if h.x == x {
			return false
		}
	}
	return true
}

// at is the x of the column at full-layout x once the left-out columns
// before it are gone.
func (c animalCols) at(x int) int {
	out := x
	for _, h := range c {
		if h.x < x {
			out -= h.w
		}
	}
	return out
}

// animalsWant is the Animals (or Wildlife) window's wanted size: the full
// layout, and a row per tame animal or wild species.
func (a *App) animalsWant() (float32, float32) {
	w, rows := colonyFullW, 0
	if a.animalsWild {
		w, rows = wildFullW, len(a.wildlife())
	} else if a.renderer != nil {
		rows = len(a.tameAnimals())
	}
	return float32(w), float32(max(160, animalsRowTop+rows*animalRowH+8))
}

// wildlife is the game's wild species list, read again only when the
// simulation tick moves.
func (a *App) wildlife() []game.WildlifeRow {
	if a.renderer == nil {
		return nil
	}
	t := a.shell.Tick()
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
	r := a.windowRect()
	cols := animalColsFor(r.w, a.animalsWild)
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
				button{id: "animals.focus." + row.Species, x: r.x + 12, y: y, w: float32(cols.at(wildBtn) - 24), h: animalRowH - 2},
				button{id: "animals.wild.hunt." + row.Species, label: "Hunt all", x: r.x + float32(cols.at(wildBtn)), y: y, w: 90, h: 24, selected: row.Hunted > 0},
				button{id: "animals.wild.tame." + row.Species, label: "Tame one", x: r.x + float32(cols.at(wildBtn)) + 96, y: y, w: 90, h: 24, selected: row.TameMarked > 0},
				button{id: "animals.wild.cancel." + row.Species, label: "Cancel", x: r.x + float32(cols.at(wildBtn)) + 192, y: y, w: 80, h: 24},
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
			button{id: "animals.master." + id, label: animalMasterCell(an), x: r.x + float32(cols.at(colMaster)) - 2, y: y, w: 88, h: 24,
				tip: "Master\nPick the colonist who handles this animal."},
			button{id: "animals.slaughter." + id, label: slaughter, x: r.x + float32(cols.at(colBtn)), y: y, w: 100, h: 24, selected: an.Designated},
			button{id: "animals.release." + id, label: "Release", x: r.x + float32(cols.at(colBtn)) + 106, y: y, w: 90, h: 24},
		)
		if an.CanFollow { // PLAY-015: follow the drafted master, on or off
			out = append(out, button{id: "animals.follow." + id, label: followCell(an), x: r.x + float32(cols.at(colFollow)) - 2, y: y,
				w: colArea - colFollow - 8, h: 24, selected: an.Follows,
				tip: "Follow drafted\nWhether it follows its master while the master is drafted."})
		}
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
		case "master":
			a.openAnimalMasterMenu(an)
		case "slaughter":
			c := command.Command{Kind: command.DesignateHunt, X: an.X, Y: an.Y, X2: an.X, Y2: an.Y}
			if an.Designated { // only the mark: the pen stays
				c.Kind, c.Order = command.Undesignate, command.OrderHunt
			}
			a.shell.Enqueue(c)
		case "release":
			a.shell.Enqueue(command.Command{Kind: command.ReleaseAnimal, Pawn: n})
		case "follow":
			a.shell.Enqueue(command.Command{Kind: command.SetAnimalFollow, Target: n, Enabled: !an.Follows})
		}
	}
}

// wildOf lists the wild (not tame) animals of a species.
func (a *App) wildOf(species string) []game.AnimalView {
	var out []game.AnimalView
	for _, an := range a.shell.World().Animals {
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
				a.shell.Enqueue(at(command.DesignateHunt, an))
			}
		}
	case "tame":
		for _, an := range a.wildOf(species) {
			if !an.TameMark && !an.Manhunter {
				a.shell.Enqueue(at(command.DesignateTame, an))
				return
			}
		}
	case "cancel":
		for _, an := range a.wildOf(species) {
			if an.Designated {
				c := at(command.Undesignate, an)
				c.Order = command.OrderHunt
				a.shell.Enqueue(c)
			}
			if an.TameMark {
				c := at(command.Undesignate, an)
				c.Order = command.OrderTame
				a.shell.Enqueue(c)
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
	a.centerOnMap(float64(an.X)+0.5, float64(an.Y)+0.5)
}

func (a *App) drawAnimalsTab(screen *ebiten.Image, sw, sh int) {
	r := a.windowRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	w := a.shell.World()
	marked := 0
	for _, an := range w.Animals {
		if an.TameMark {
			marked++
		}
	}
	title := fmt.Sprintf("Animals  (%d tame, %d marked for taming, %d pens)", len(a.tameAnimals()), marked, len(w.Pens)+countZones(w, game.ZonePen))
	if a.animalsWild {
		title = fmt.Sprintf("Wildlife  (%d species)", len(a.wildlife()))
	}
	drawText(screen, ellipsize(title, tsLabel, float64(r.w)-12-200), float64(r.x)+12, float64(r.y)+8, tsLabel)
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
	cols := animalColsFor(r.w, false)
	drawFitSmall(screen, "Tame with Architect > Orders > Tame (hunters do it). Handle work tames, trains, milks and shears; pens keep animals in place. Pick a master and training in the inspector.", float64(x0+12), float64(y0+34), float64(r.w)-24, tsBody)
	head := y0 + animalsRowTop - 22
	for _, h := range []struct {
		x int
		s string
	}{{colName, "Name"}, {colSpecies, "Species"}, {colSex, "Sex"}, {colAge, "Age"}, {colFood, "Food"},
		{colProduct, "Product"}, {colPreg, "Preg."}, {colHealth, "Health"}, {colMaster, "Master"}, {colFollow, "Follow"}, {colArea, "Area / Pen"}, {colTrain, "Training"}, {colDoing, "Doing"}} {
		if cols.shown(h.x) {
			drawLabel(screen, x0+cols.at(h.x), head, h.s)
		}
	}
	list := a.tameAnimals()
	for i, an := range list {
		y := y0 + animalsRowTop + i*animalRowH
		if y+animalRowH > int(r.y+r.h) {
			break
		}
		drawAnimalIcon(screen, a.renderer.art, an, float64(r.x)+16, float64(y), 26)
		ty := y + 6
		drawLabelClip(screen, x0+cols.at(colName), ty, colSpecies-colName-6, an.Name)
		drawLabelClip(screen, x0+cols.at(colSpecies), ty, colSex-colSpecies-6, animalLabel(an))
		if cols.shown(colSex) {
			drawLabelClip(screen, x0+cols.at(colSex), ty, colAge-colSex-4, animalSexLetter(an))
		}
		if cols.shown(colAge) {
			drawLabelClip(screen, x0+cols.at(colAge), ty, colFood-colAge-6, animalAgeText(an))
		}
		if animalHasNeeds(an) {
			drawMiniBar(screen, x0+cols.at(colFood), y+8, 70, 10, an.Food, an.Food < hungryBelow)
		}
		if cols.shown(colProduct) {
			drawLabelClip(screen, x0+cols.at(colProduct), ty, colPreg-colProduct-6, animalProductText(an))
		}
		if cols.shown(colPreg) {
			drawLabelClip(screen, x0+cols.at(colPreg), ty, colHealth-colPreg-6, animalPregCell(an))
		}
		drawLabelClip(screen, x0+cols.at(colHealth), ty, colMaster-colHealth-6, animalHealthCell(an))
		if !an.CanFollow { // trained animals get the on/off toggle button
			drawLabelClip(screen, x0+cols.at(colFollow), ty, colArea-colFollow-6, followCell(an))
		}
		if an.Penned { // area animals have the dropdown widget (animalsTabUI)
			drawLabelClip(screen, x0+cols.at(colArea), ty, colTrain-colArea-6, animalAreaCell(an, nil))
		}
		if cols.shown(colTrain) {
			drawLabelClip(screen, x0+cols.at(colTrain), ty, colDoing-colTrain-6, animalTrainingCell(an))
		}
		if cols.shown(colDoing) {
			drawLabelClip(screen, x0+cols.at(colDoing), ty, colBtn-colDoing-8, animalDoing(an))
		}
	}
	if len(list) == 0 {
		drawLabel(screen, int(r.x)+52, int(r.y)+animalsRowTop, "No tame animals yet.")
	}
}

func (a *App) drawWildList(screen *ebiten.Image, r button) {
	x0, y0 := int(r.x), int(r.y)
	cols := animalColsFor(r.w, true)
	drawFitSmall(screen, "Wild animals on the map. Click a row to look at one; Hunt all marks every animal of it, Tame one marks a single animal.", float64(x0+12), float64(y0+34), float64(r.w)-24, tsBody)
	head := y0 + animalsRowTop - 22
	for _, h := range []struct {
		x int
		s string
	}{{wildLabel, "Species"}, {wildCount, "Count"}, {wildDiet, "Diet"}, {wildPred, "Pred."}, {wildTame, "Tame chance"}, {wildMeat, "Meat each"}, {wildMarks, "Marked"}} {
		if cols.shown(h.x) {
			drawLabel(screen, x0+cols.at(h.x), head, h.s)
		}
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
		drawLabelClip(screen, x0+cols.at(wildCount), ty, wildDiet-wildCount-6, fmt.Sprintf("x%d", row.Count))
		if cols.shown(wildDiet) {
			drawLabelClip(screen, x0+cols.at(wildDiet), ty, wildPred-wildDiet-6, row.Diet)
		}
		if row.Predator {
			drawWarning(screen, float32(x0+cols.at(wildPred)+6), float32(y+13))
		}
		drawLabelClip(screen, x0+cols.at(wildTame), ty, wildMeat-wildTame-6, fmt.Sprintf("%d%%", row.TameChance))
		if cols.shown(wildMeat) {
			drawLabelClip(screen, x0+cols.at(wildMeat), ty, wildMarks-wildMeat-6, strconv.Itoa(row.Meat))
		}
		if cols.shown(wildMarks) {
			drawLabelClip(screen, x0+cols.at(wildMarks), ty, wildBtn-wildMarks-8, fmt.Sprintf("hunt %d, tame %d", row.Hunted, row.TameMarked))
		}
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
	case an.Downed: // too hurt to stand (it may also be marked)
		return "Downed"
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
	uiFillRect(screen, float32(x), float32(y), float32(w), float32(h), uiField, false)
	fill := uiGood
	if low {
		fill = alertClr
	}
	uiFillRect(screen, float32(x), float32(y), float32(w*pct/100), float32(h), fill, false)
}

// drawWarning is a red "!" disc: the predator flag.
func drawWarning(screen *ebiten.Image, cx, cy float32) {
	uiFillCircle(screen, cx, cy, 8, alertClr, true)
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

// followCell is the Follow column: "on"/"off" for an animal trained to
// follow its drafted master (the player's setting), else "-".
func followCell(an game.AnimalView) string {
	switch {
	case !an.CanFollow:
		return "-"
	case an.Follows:
		return "on"
	}
	return "off"
}

// animalAreaCell is the Area / Pen column's text: a penned animal's pen
// ("Pen 1", "Roaming", "Being roped"), else its allowed area.
func animalAreaCell(an game.AnimalView, areas []game.AreaView) string {
	if an.Penned {
		if an.Pen == "" {
			return "Roaming"
		}
		return an.Pen
	}
	return areaRefLabel(areas, an.AreaRef)
}

// animalsTabUI declares the Area dropdown of every visible non-penned
// animal row (the AREA-UI selector, keyed by the animal's id).
func (a *App) animalsTabUI(sw, sh int) {
	if a.renderer == nil || !a.workOpen || a.panelMode != "animals" || a.animalsWild {
		return
	}
	r := a.windowRect()
	cols := animalColsFor(r.w, false)
	for i, an := range a.tameAnimals() {
		y := r.y + animalsRowTop + float32(i*animalRowH)
		if y+animalRowH > r.y+r.h {
			break
		}
		if an.Penned {
			continue
		}
		an := an
		a.areaDropdown("animals.area."+strconv.FormatUint(uint64(an.ID), 10),
			rect{r.x + float32(cols.at(colArea)) - 2, y, colTrain - colArea - 4, 24}, an.AreaRef,
			func(ref int) { a.assignArea(an.ID, ref) })
	}
}

// openAnimalMasterMenu lists the colonists (and "No master") as
// SetAnimalMaster choices under the animal's Master button.
func (a *App) openAnimalMasterMenu(an game.AnimalView) {
	var opts []game.OrderOption
	for _, p := range a.shell.Pawns() {
		o := game.OrderOption{Label: p.Name, Cmd: command.Command{Kind: command.SetAnimalMaster, Target: uint64(an.ID), Pawn: uint64(p.ID)}}
		if p.ID == an.MasterID {
			o.Label += " (master)"
		}
		opts = append(opts, o)
	}
	opts = append(opts, game.OrderOption{Label: "No master", Cmd: command.Command{Kind: command.SetAnimalMaster, Target: uint64(an.ID)}})
	r := a.windowRect()
	cols := animalColsFor(r.w, false)
	for i, t := range a.tameAnimals() {
		if t.ID == an.ID {
			a.float = floatMenu{open: true, x: r.x + float32(cols.at(colMaster)) - 2, y: r.y + animalsRowTop + float32(i*animalRowH) + 26, opts: opts}
			return
		}
	}
}
