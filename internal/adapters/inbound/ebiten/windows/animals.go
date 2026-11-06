package windows

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Animals tab: the colony's tame animals, with slaughter and release.

const animalRowH = 30

func (w Animals) tame() []game.AnimalView {
	var out []game.AnimalView
	for _, an := range w.sh.World().Animals {
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
	colBtn     = 1040         // Slaughter, then Release
	colDiet    = colBtn + 202 // FOOD-5: the food policy button
	dietW      = 120
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
		{colProduct, colPreg - colProduct}, {colAge, colFood - colAge}, {colSex, colAge - colSex}, {colDiet, dietW + 6}}
	wildDrops = []animalCol{{wildMarks, wildBtn - wildMarks}, {wildMeat, wildMarks - wildMeat}, {wildDiet, wildPred - wildDiet}}
)

const (
	colonyFullW = colDiet + dietW + 12
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

// want is the Animals (or Wildlife) window's wanted size: the full
// layout, and a row per tame animal or wild species.
func (w Animals) want() (float32, float32) {
	width, rows := colonyFullW, 0
	if w.Wild {
		width, rows = wildFullW, len(w.wildlife())
	} else if w.live() {
		rows = len(w.tame())
	}
	return float32(width), float32(max(160, animalsRowTop+rows*animalRowH+8))
}

// wildlife is the game's wild species list, read again only when the
// simulation tick moves.
func (w Animals) wildlife() []game.WildlifeRow {
	if !w.live() {
		return nil
	}
	t := w.sh.Tick()
	if !w.wildRead || t != w.wildTick {
		w.wildRows, w.wildTick, w.wildRead = w.sh.Env().Game.Wildlife(), t, true
	}
	return w.wildRows
}

// WildRow is the Wild list's row of a species.
func (w Animals) WildRow(species string) (game.WildlifeRow, bool) {
	for _, r := range w.wildlife() {
		if r.Species == species {
			return r, true
		}
	}
	return game.WildlifeRow{}, false
}

func (w Animals) Buttons() []uikit.Button {
	r := w.Rect()
	cols := animalColsFor(r.W, w.Wild)
	out := []uikit.Button{
		{ID: "animals.view.colony", Label: w.sh.T("Colony"), X: r.X + r.W - 190, Y: r.Y + 6, W: 84, H: 24, Selected: !w.Wild},
		{ID: "animals.view.wild", Label: w.sh.T("Wild"), X: r.X + r.W - 100, Y: r.Y + 6, W: 84, H: 24, Selected: w.Wild},
	}
	if !w.Wild {
		out = append(out, uikit.Button{ID: AutoSlaughterBtnID, Label: w.sh.T("Auto-slaughter"), X: r.X + r.W - 330, Y: r.Y + 6, W: 130, H: 24,
			Tip: w.sh.T("Auto-slaughter\nKeep each tame species at a herd size: handlers slaughter the surplus once a day.")})
	}
	if w.Wild {
		for i, row := range w.wildlife() {
			y := r.Y + animalsRowTop + float32(i*animalRowH)
			if y+animalRowH > r.Y+r.H {
				break
			}
			out = append(out,
				uikit.Button{ID: "animals.focus." + row.Species, X: r.X + 12, Y: y, W: float32(cols.at(wildBtn) - 24), H: animalRowH - 2},
				uikit.Button{ID: "animals.wild.hunt." + row.Species, Label: w.sh.T("Hunt all"), X: r.X + float32(cols.at(wildBtn)), Y: y, W: 90, H: 24, Selected: row.Hunted > 0},
				uikit.Button{ID: "animals.wild.tame." + row.Species, Label: w.sh.T("Tame one"), X: r.X + float32(cols.at(wildBtn)) + 96, Y: y, W: 90, H: 24, Selected: row.TameMarked > 0},
				uikit.Button{ID: "animals.wild.cancel." + row.Species, Label: w.sh.T("Cancel"), X: r.X + float32(cols.at(wildBtn)) + 192, Y: y, W: 80, H: 24},
			)
		}
		return out
	}
	for i, an := range w.tame() {
		y := r.Y + animalsRowTop + float32(i*animalRowH)
		if y+animalRowH > r.Y+r.H {
			break
		}
		id := strconv.FormatUint(uint64(an.ID), 10)
		slaughter := w.sh.T("Slaughter")
		if an.Designated {
			slaughter = w.sh.T("Spare")
		}
		out = append(out,
			uikit.Button{ID: "animals.master." + id, Label: AnimalMasterCell(an), X: r.X + float32(cols.at(colMaster)) - 2, Y: y, W: 88, H: 24,
				Tip: w.sh.T("Master\nPick the colonist who handles this animal.")},
			uikit.Button{ID: "animals.slaughter." + id, Label: slaughter, X: r.X + float32(cols.at(colBtn)), Y: y, W: 100, H: 24, Selected: an.Designated},
			uikit.Button{ID: "animals.release." + id, Label: w.sh.T("Release"), X: r.X + float32(cols.at(colBtn)) + 106, Y: y, W: 90, H: 24},
		)
		if cols.shown(colDiet) {
			label := an.FoodPolicy
			if label == "" {
				label = "-"
			}
			out = append(out, uikit.Button{ID: AnimalDietID(an.ID), Label: label, X: r.X + float32(cols.at(colDiet)), Y: y, W: dietW, H: 24,
				Tip: w.sh.T("Food policy\nWhat this animal may eat. A click picks the next policy.")})
		}
		if an.CanFollow { // PLAY-015: follow the drafted master, on or off
			out = append(out, uikit.Button{ID: "animals.follow." + id, Label: animalFollowCell(w.sh, an), X: r.X + float32(cols.at(colFollow)) - 2, Y: y,
				W: colArea - colFollow - 8, H: 24, Selected: an.Follows,
				Tip: w.sh.T("Follow drafted\nWhether it follows its master while the master is drafted.")})
		}
	}
	return out
}

func (w Animals) Click(id string) {
	if id == AutoSlaughterBtnID {
		w.OpenAutoSlaughter()
		return
	}
	parts := strings.Split(id, ".")
	if len(parts) < 3 || !w.live() {
		return
	}
	switch parts[1] {
	case "view":
		w.Wild = parts[2] == "wild"
		return
	case "focus":
		w.focusWild(parts[2])
		return
	case "wild":
		if len(parts) == 4 {
			w.wildAction(parts[2], parts[3])
		}
		return
	}
	n, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return
	}
	for _, an := range w.tame() {
		if uint64(an.ID) != n {
			continue
		}
		switch parts[1] {
		case "master":
			w.masterMenu(an)
		case "slaughter":
			c := command.Command{Kind: command.DesignateHunt, X: an.X, Y: an.Y, X2: an.X, Y2: an.Y}
			if an.Designated { // only the mark: the pen stays
				c.Kind, c.Order = command.Undesignate, command.OrderHunt
			}
			w.sh.Enqueue(c)
		case "release":
			uikit.ConfirmIf(w.sh.Kit(), "animals.release.confirm", w.sh.T("Release this animal?"),
				w.sh.T("It goes wild again and is no longer yours. A bonded colonist will mind."),
				w.sh.T("Release"), true, func() { w.sh.Enqueue(command.Command{Kind: command.ReleaseAnimal, Pawn: n}) })
		case "diet":
			if next, ok := NextFoodPolicy(w.sh.World().FoodPolicies, an.FoodPolicy); ok {
				w.sh.Enqueue(command.Command{Kind: command.SetFoodPolicy, Pawn: n, Index: next})
			}
		case "follow":
			w.sh.Enqueue(command.Command{Kind: command.SetAnimalFollow, Target: n, Enabled: !an.Follows})
		}
	}
}

// wildOf lists the wild (not tame) animals of a species.
func (w Animals) wildOf(species string) []game.AnimalView {
	var out []game.AnimalView
	for _, an := range w.sh.World().Animals {
		if !an.Tame && an.Species == species {
			out = append(out, an)
		}
	}
	return out
}

// wildAction emits the existing designation commands for a species
// row: hunt every animal of it, tame one, or cancel its marks.
func (w Animals) wildAction(verb, species string) {
	at := func(kind command.Kind, an game.AnimalView) command.Command {
		return command.Command{Kind: kind, X: an.X, Y: an.Y, X2: an.X, Y2: an.Y}
	}
	switch verb {
	case "hunt":
		for _, an := range w.wildOf(species) {
			if !an.Designated {
				w.sh.Enqueue(at(command.DesignateHunt, an))
			}
		}
	case "tame":
		for _, an := range w.wildOf(species) {
			if !an.TameMark && !an.Manhunter {
				w.sh.Enqueue(at(command.DesignateTame, an))
				return
			}
		}
	case "cancel":
		for _, an := range w.wildOf(species) {
			if an.Designated {
				c := at(command.Undesignate, an)
				c.Order = command.OrderHunt
				w.sh.Enqueue(c)
			}
			if an.TameMark {
				c := at(command.Undesignate, an)
				c.Order = command.OrderTame
				w.sh.Enqueue(c)
			}
		}
	}
}

// focusWild centres the camera on the next animal of a species, cycling.
func (w Animals) focusWild(species string) {
	list := w.wildOf(species)
	if len(list) == 0 {
		return
	}
	if w.focus == nil {
		w.focus = map[string]int{}
	}
	key := "wild." + species
	an := list[w.focus[key]%len(list)]
	w.focus[key]++
	w.sh.CenterOnMap(float64(an.X)+0.5, float64(an.Y)+0.5)
}

func (w Animals) Draw(screen *ebiten.Image, sw, sh int) {
	r := w.Rect()
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	wv := w.sh.World()
	marked := 0
	for _, an := range wv.Animals {
		if an.TameMark {
			marked++
		}
	}
	title := lang.Tf(w.sh, "Animals  (%d tame, %d marked for taming, %d pens)", len(w.tame()), marked, len(wv.Pens)+CountZones(wv, game.ZonePen))
	if w.Wild {
		title = lang.Tf(w.sh, "Wildlife  (%d species)", len(w.wildlife()))
	}
	gfx.Text(screen, gfx.Ellipsize(title, gfx.Label, float64(r.W)-12-200), float64(r.X)+12, float64(r.Y)+8, gfx.Label)
	mx, my := w.sh.Cursor()
	if w.Wild {
		w.drawWild(screen, r)
	} else {
		w.drawColony(screen, r)
	}
	for _, b := range w.Buttons() {
		if !strings.HasPrefix(b.ID, "animals.focus.") {
			uikit.DrawButton(screen, b, b.Contains(mx, my))
		}
	}
	if w.Wild {
		w.drawWildArrow(screen)
	}
}

func (w Animals) drawColony(screen *ebiten.Image, r uikit.Button) {
	x0, y0 := int(r.X), int(r.Y)
	cols := animalColsFor(r.W, false)
	uikit.DrawFitSmall(screen, w.sh.T("Tame with Architect > Orders > Tame (hunters do it). Handle work tames, trains, milks and shears; pens keep animals in place. Pick a master and training in the inspector."), float64(x0+12), float64(y0+34), float64(r.W)-24, gfx.Body)
	head := y0 + animalsRowTop - 22
	for _, h := range []struct {
		x int
		s string
	}{{colName, w.sh.T("Name")}, {colSpecies, w.sh.T("Species")}, {colSex, w.sh.T("Sex")}, {colAge, w.sh.T("Age")}, {colFood, w.sh.T("Food")},
		{colProduct, w.sh.T("Product")}, {colPreg, w.sh.T("Preg.")}, {colHealth, w.sh.T("Health")}, {colMaster, w.sh.T("Master")}, {colFollow, w.sh.T("Follow")}, {colArea, w.sh.T("Area / Pen")}, {colTrain, w.sh.T("Training")}, {colDoing, w.sh.T("Doing")}, {colDiet, w.sh.T("Diet")}} {
		if cols.shown(h.x) {
			uikit.DrawLabel(screen, x0+cols.at(h.x), head, h.s)
		}
	}
	list := w.tame()
	for i, an := range list {
		y := y0 + animalsRowTop + i*animalRowH
		if y+animalRowH > int(r.Y+r.H) {
			break
		}
		w.sh.DrawAnimalIcon(screen, an, float64(r.X)+16, float64(y), 26)
		ty := y + 6
		uikit.DrawLabelClip(screen, x0+cols.at(colName), ty, colSpecies-colName-6, an.Name)
		uikit.DrawLabelClip(screen, x0+cols.at(colSpecies), ty, colSex-colSpecies-6, w.sh.AnimalLabel(an))
		if cols.shown(colSex) {
			uikit.DrawLabelClip(screen, x0+cols.at(colSex), ty, colAge-colSex-4, animalSexLetter(w.sh, an))
		}
		if cols.shown(colAge) {
			uikit.DrawLabelClip(screen, x0+cols.at(colAge), ty, colFood-colAge-6, animalAgeText(w.sh, an))
		}
		if hasNeeds(an) {
			drawMiniBar(screen, x0+cols.at(colFood), y+8, 70, 10, an.Food, an.Food < hungryBelow)
		}
		if cols.shown(colProduct) {
			uikit.DrawLabelClip(screen, x0+cols.at(colProduct), ty, colPreg-colProduct-6, AnimalProductText(an))
		}
		if cols.shown(colPreg) {
			uikit.DrawLabelClip(screen, x0+cols.at(colPreg), ty, colHealth-colPreg-6, animalPregCell(w.sh, an))
		}
		uikit.DrawLabelClip(screen, x0+cols.at(colHealth), ty, colMaster-colHealth-6, w.sh.AnimalHealth(an))
		if !an.CanFollow { // trained animals get the on/off toggle button
			uikit.DrawLabelClip(screen, x0+cols.at(colFollow), ty, colArea-colFollow-6, animalFollowCell(w.sh, an))
		}
		if an.Penned { // area animals have the dropdown widget (animalsTabUI)
			uikit.DrawLabelClip(screen, x0+cols.at(colArea), ty, colTrain-colArea-6, animalAreaCell(w.sh, an, ""))
		}
		if cols.shown(colTrain) {
			uikit.DrawLabelClip(screen, x0+cols.at(colTrain), ty, colDoing-colTrain-6, AnimalTrainingCell(an))
		}
		if cols.shown(colDoing) {
			uikit.DrawLabelClip(screen, x0+cols.at(colDoing), ty, colBtn-colDoing-8, animalDoing(w.sh, an))
		}
	}
	if len(list) == 0 {
		uikit.DrawLabel(screen, int(r.X)+52, int(r.Y)+animalsRowTop, w.sh.T("No tame animals yet."))
	}
}

func (w Animals) drawWild(screen *ebiten.Image, r uikit.Button) {
	x0, y0 := int(r.X), int(r.Y)
	cols := animalColsFor(r.W, true)
	uikit.DrawFitSmall(screen, w.sh.T("Wild animals on the map. Click a row to look at one; Hunt all marks every animal of it, Tame one marks a single animal."), float64(x0+12), float64(y0+34), float64(r.W)-24, gfx.Body)
	head := y0 + animalsRowTop - 22
	for _, h := range []struct {
		x int
		s string
	}{{wildLabel, w.sh.T("Species")}, {wildCount, w.sh.T("Count")}, {wildDiet, w.sh.T("Diet")}, {wildPred, w.sh.T("Pred.")}, {wildTame, w.sh.T("Tame chance")}, {wildMeat, w.sh.T("Meat each")}, {wildMarks, w.sh.T("Marked")}} {
		if cols.shown(h.x) {
			uikit.DrawLabel(screen, x0+cols.at(h.x), head, h.s)
		}
	}
	rows := w.wildlife()
	for i, row := range rows {
		y := y0 + animalsRowTop + i*animalRowH
		if y+animalRowH > int(r.Y+r.H) {
			break
		}
		w.sh.DrawAnimalIcon(screen, game.AnimalView{Species: row.Species, Label: row.Label}, float64(r.X)+16, float64(y), 26)
		ty := y + 6
		uikit.DrawLabelClip(screen, x0+wildLabel, ty, wildCount-wildLabel-6, row.Label)
		uikit.DrawLabelClip(screen, x0+cols.at(wildCount), ty, wildDiet-wildCount-6, lang.Tf(w.sh, "x%d", row.Count))
		if cols.shown(wildDiet) {
			uikit.DrawLabelClip(screen, x0+cols.at(wildDiet), ty, wildPred-wildDiet-6, row.Diet)
		}
		if row.Predator {
			drawWarning(screen, float32(x0+cols.at(wildPred)+6), float32(y+13))
		}
		uikit.DrawLabelClip(screen, x0+cols.at(wildTame), ty, wildMeat-wildTame-6, fmt.Sprintf("%d%%", row.TameChance))
		if cols.shown(wildMeat) {
			uikit.DrawLabelClip(screen, x0+cols.at(wildMeat), ty, wildMarks-wildMeat-6, strconv.Itoa(row.Meat))
		}
		if cols.shown(wildMarks) {
			uikit.DrawLabelClip(screen, x0+cols.at(wildMarks), ty, wildBtn-wildMarks-8, lang.Tf(w.sh, "hunt %d, tame %d", row.Hunted, row.TameMarked))
		}
	}
	if len(rows) == 0 {
		uikit.DrawLabel(screen, x0+52, y0+animalsRowTop, w.sh.T("No wild animals in sight."))
	}
}

// hasNeeds reports the animal has needs to show (a living one with a
// state; the inspector's test too).
func hasNeeds(an game.AnimalView) bool { return an.State != "" }

// hungryBelow is the food percent under which a tame animal is hungry.
const hungryBelow = 25

// AnimalSexLetter is "F" or "M".
func AnimalSexLetter(an game.AnimalView) string { return animalSexLetter(lang.English{}, an) }

func animalSexLetter(tr lang.Tr, an game.AnimalView) string {
	if an.Female {
		return tr.T("F")
	}
	return tr.T("M")
}

// AnimalAgeText is the age in days, "young" ones marked.
func AnimalAgeText(an game.AnimalView) string { return animalAgeText(lang.English{}, an) }

func animalAgeText(tr lang.Tr, an game.AnimalView) string {
	if !hasNeeds(an) && an.AgeDays == 0 {
		return "-"
	}
	if !an.Adult {
		return lang.Tf(tr, "%dd (young)", an.AgeDays)
	}
	return lang.Tf(tr, "%dd", an.AgeDays)
}

// AnimalProductText is "milk 70%" or "-".
func AnimalProductText(an game.AnimalView) string {
	if an.Product == "" {
		return "-"
	}
	return fmt.Sprintf("%s %d%%", an.Product, an.ProductPct)
}

// AnimalPregCell is the Colony list's pregnancy column.
func AnimalPregCell(an game.AnimalView) string { return animalPregCell(lang.English{}, an) }

func animalPregCell(tr lang.Tr, an game.AnimalView) string {
	if an.Pregnant {
		return fmt.Sprintf("%d%%", an.PregPct)
	}
	if an.EggsFertile {
		return tr.T("eggs")
	}
	return "-"
}

// AnimalMasterCell is the Colony list's Master column ("-": none).
func AnimalMasterCell(an game.AnimalView) string {
	if an.Master == "" {
		return "-"
	}
	return an.Master
}

// AnimalTrainingCell is the Colony list's Training column: each skill
// handlers train, with its steps ("Obedience 2/3, Guard 0/3"), or "-".
func AnimalTrainingCell(an game.AnimalView) string {
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

// AnimalDoing is what the Colony list's last column says.
func AnimalDoing(an game.AnimalView) string { return animalDoing(lang.English{}, an) }

func animalDoing(tr lang.Tr, an game.AnimalView) string {
	switch {
	case an.Downed: // too hurt to stand (it may also be marked)
		return tr.T("Downed")
	case an.Designated:
		return tr.T("marked for slaughter")
	case an.State != "":
		return an.State
	}
	return an.Mode
}

// drawMiniBar is a thin percent bar; low ones are red.
func drawMiniBar(screen *ebiten.Image, x, y, w, h, pct int, low bool) {
	pct = max(0, min(100, pct))
	gfx.FillRect(screen, float32(x), float32(y), float32(w), float32(h), uikit.Field, false)
	fill := uikit.Good
	if low {
		fill = uikit.Danger
	}
	gfx.FillRect(screen, float32(x), float32(y), float32(w*pct/100), float32(h), fill, false)
}

// drawWarning is a red "!" disc: the predator flag.
func drawWarning(screen *ebiten.Image, cx, cy float32) {
	gfx.FillCircle(screen, cx, cy, 8, uikit.Danger, true)
	uikit.DrawCentered(screen, "!", float64(cx), float64(cy), gfx.Label)
}

// AnimalRow is one line of the Animals tab: name, species label (long
// ones like "yorkshire terrier" fit), health and what it is doing.
func AnimalRow(an game.AnimalView, label string) string {
	doing := an.Mode
	if an.Designated {
		doing = "marked for slaughter"
	}
	return fmt.Sprintf("%-13s %-18s %3d%%    %s", an.Name, label, an.Health, doing)
}

// CountZones is how many zones of kind the world has.
func CountZones(w game.WorldView, kind string) int {
	n := 0
	for _, z := range w.Zones {
		if z.Kind == kind {
			n++
		}
	}
	return n
}

// FollowCell is the Follow column: "on"/"off" for an animal trained to
// follow its drafted master (the player's setting), else "-".
func FollowCell(an game.AnimalView) string { return animalFollowCell(lang.English{}, an) }

func animalFollowCell(tr lang.Tr, an game.AnimalView) string {
	switch {
	case !an.CanFollow:
		return "-"
	case an.Follows:
		return tr.T("on")
	}
	return tr.T("off")
}

// AnimalAreaCell is the Area / Pen column's text: a penned animal's pen
// ("Pen 1", "Roaming", "Being roped"), else its allowed area.
// area is the allowed area's name (the shell knows the areas).
func AnimalAreaCell(an game.AnimalView, area string) string {
	return animalAreaCell(lang.English{}, an, area)
}

func animalAreaCell(tr lang.Tr, an game.AnimalView, area string) string {
	if an.Penned {
		if an.Pen == "" {
			return tr.T("Roaming")
		}
		return an.Pen
	}
	return area
}

// Declare declares the Area dropdown of every visible non-penned
// animal row (the AREA-UI selector, keyed by the animal's id).
func (w Animals) Declare(sw, sh int) {
	if !w.live() || !w.Open || w.Mode != "animals" || w.Wild {
		return
	}
	r := w.Rect()
	cols := animalColsFor(r.W, false)
	for i, an := range w.tame() {
		y := r.Y + animalsRowTop + float32(i*animalRowH)
		if y+animalRowH > r.Y+r.H {
			break
		}
		if an.Penned {
			continue
		}
		an := an
		w.sh.AreaDropdown("animals.area."+strconv.FormatUint(uint64(an.ID), 10),
			gfx.Rect{X: r.X + float32(cols.at(colArea)) - 2, Y: y, W: colTrain - colArea - 4, H: 24}, an.AreaRef,
			func(ref int) {
				w.sh.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(an.ID), Index: ref})
			})
	}
}

// masterMenu lists the colonists (and "No master") as
// SetAnimalMaster choices under the animal's Master button.
func (w Animals) masterMenu(an game.AnimalView) {
	var opts []game.OrderOption
	for _, p := range w.sh.Pawns() {
		o := game.OrderOption{Label: p.Name, Cmd: command.Command{Kind: command.SetAnimalMaster, Target: uint64(an.ID), Pawn: uint64(p.ID)}}
		if p.ID == an.MasterID {
			o.Label += " (master)"
		}
		opts = append(opts, o)
	}
	opts = append(opts, game.OrderOption{Label: w.sh.T("No master"), Cmd: command.Command{Kind: command.SetAnimalMaster, Target: uint64(an.ID)}})
	r := w.Rect()
	cols := animalColsFor(r.W, false)
	for i, t := range w.tame() {
		if t.ID == an.ID {
			w.sh.Float(r.X+float32(cols.at(colMaster))-2, r.Y+animalsRowTop+float32(i*animalRowH)+26, opts)
			return
		}
	}
}

// mapProjector is what the wild-row arrow needs of the camera: a world
// point in map pixels (consumer-owned; the root's view implements it).
type mapProjector interface {
	WorldToScreenF(wx, wy float64) (float32, float32)
}

// hoveredWildRow is the Wild list row under the cursor.
func (w Animals) hoveredWildRow() (game.WildlifeRow, float32, bool) {
	if !w.Open || !w.Wild || !w.live() {
		return game.WildlifeRow{}, 0, false
	}
	r := w.Rect()
	mx, my := w.sh.Cursor()
	fy := float32(my) - (r.Y + animalsRowTop)
	if float32(mx) < r.X || float32(mx) >= r.X+r.W || fy < 0 {
		return game.WildlifeRow{}, 0, false
	}
	i := int(fy) / animalRowH
	rows := w.wildlife()
	y := r.Y + animalsRowTop + float32(i*animalRowH)
	if i >= len(rows) || y+animalRowH > r.Y+r.H {
		return game.WildlifeRow{}, 0, false
	}
	return rows[i], y + animalRowH/2, true
}

// WildArrow is the arrow of the hovered Wild row (CTL-019): from the row's
// right edge to the screen spot of one of its animals, the one nearest the
// screen centre. ok is false without a hovered row, a camera, or an animal
// that is on screen and out of the fog.
func (w Animals) WildArrow() (x0, y0, x1, y1 float32, ok bool) {
	row, ry, hov := w.hoveredWildRow()
	if !hov {
		return
	}
	cam, has := w.sh.(mapProjector)
	if !has {
		return
	}
	sw, sh := w.sh.Size()
	cx, cy := float32(sw)/2, float32(sh)/2
	mw, _ := w.sh.MapSize()
	fog := w.sh.World().Fog
	scale := float32(gfx.UIScale())
	best := float32(-1)
	for _, an := range w.sh.World().Animals {
		if an.Tame || an.Species != row.Species {
			continue
		}
		if i := an.Y*mw + an.X; mw > 0 && i >= 0 && i < len(fog) && fog[i] != 0 {
			continue // under fog
		}
		px, py := cam.WorldToScreenF(float64(an.X)+0.5, float64(an.Y)+0.5)
		px, py = px/scale, py/scale
		if px < 0 || py < 0 || px >= float32(sw) || py >= float32(sh) {
			continue
		}
		if d := (px-cx)*(px-cx) + (py-cy)*(py-cy); best < 0 || d < best {
			best, x1, y1 = d, px, py
		}
	}
	if best < 0 {
		return
	}
	r := w.Rect()
	return r.X + r.W, ry, x1, y1, true
}

// drawWildArrow draws the hovered row's arrow: a line with a head at the
// animal. It allocates nothing.
func (w Animals) drawWildArrow(screen *ebiten.Image) {
	x0, y0, x1, y1, ok := w.WildArrow()
	if !ok {
		return
	}
	gfx.Line(screen, x0, y0, x1, y1, 3, uikit.Accent, true)
	dx, dy := x0-x1, y0-y1
	n := float32(math.Hypot(float64(dx), float64(dy)))
	if n < 1 {
		return
	}
	dx, dy = dx/n*14, dy/n*14                   // back along the line, 14 px
	for _, s := range [2]float32{0.45, -0.45} { // a head's two sides, turned by about 26 degrees
		hx := dx*float32(math.Cos(float64(s))) - dy*float32(math.Sin(float64(s)))
		hy := dx*float32(math.Sin(float64(s))) + dy*float32(math.Cos(float64(s)))
		gfx.Line(screen, x1, y1, x1+hx, y1+hy, 3, uikit.Accent, true)
	}
}
