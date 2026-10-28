package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/command"
)

// Animal selection, its inspector (hunt toggle) and the hostile alert.

// alertClr is the hostile-animal marker and alert colour: the danger token.
var alertClr = uiDanger

func animalAt(w game.WorldView, x, y int) (game.AnimalView, bool) {
	for _, an := range w.Animals {
		if an.X == x && an.Y == y {
			return an, true
		}
	}
	return game.AnimalView{}, false
}

func (a *App) selectAnimalAt(tx, ty int) bool {
	an, ok := animalAt(a.shell.World(), tx, ty)
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
	for _, an := range a.shell.World().Animals {
		if an.ID == a.selAnimal {
			return an, true
		}
	}
	a.hasAnimal = false // it died or left
	return game.AnimalView{}, false
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

// drawAnimalIcon draws the species' ui_icon at (x, y), size px square; a
// species without one gets its initial on a disc.
func drawAnimalIcon(dst *ebiten.Image, art *artCatalog, an game.AnimalView, x, y, size float64) {
	if img := art.image("ui_icon/"+an.Species, color.NRGBA{}); img != nil {
		drawIcon(dst, img, x, y, size)
		return
	}
	r := float32(size) / 2
	uiFillCircle(dst, float32(x)+r, float32(y)+r, r, uiBtn, true)
	uiStrokeCircle(dst, float32(x)+r, float32(y)+r, r, 1.5, uiBtnBorder, true)
	drawCentered(dst, animalInitial(animalLabel(an)), x+float64(r), y+float64(r), tsLabel)
}

// drawAnimalMark draws the selection mark on the animal's tile; the
// panel itself is the inspect pane (animalPane).
func (a *App) drawAnimalMark(screen *ebiten.Image) {
	an, ok := a.selectedAnimal()
	if !ok || a.workOpen || a.archOpen {
		return
	}
	if a.renderer.art != nil {
		wx, wy := a.renderer.interpAnimal(an)
		a.renderer.drawSprite(screen, "overlay/selection", color.NRGBA{}, wx, wy, a.shell.TilePx(), a.pw, a.ph) // physical, like the map
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
	case "animal.hunt":
		c := command.Command{Kind: command.DesignateHunt, X: an.X, Y: an.Y, X2: an.X, Y2: an.Y}
		if an.Designated { // only the mark: a pen or zone underneath stays
			c.Kind, c.Order = command.Undesignate, command.OrderHunt
		}
		a.shell.Enqueue(c)
	case "animal.tame":
		c := command.Command{Kind: command.DesignateTame, X: an.X, Y: an.Y, X2: an.X, Y2: an.Y}
		if an.TameMark {
			c.Kind, c.Order = command.Undesignate, command.OrderTame
		}
		a.shell.Enqueue(c)
	case "animal.release":
		a.shell.Enqueue(command.Command{Kind: command.ReleaseAnimal, Pawn: uint64(an.ID)})
	case "animal.area": // the pane's allowed-area chip (a tame animal that is not penned)
		if an.Tame && !an.Penned {
			a.openAreaPicker("Allowed area", an.AreaRef, true, func(ref int) { a.assignArea(an.ID, ref) })
		}
	default:
		if s, ok := strings.CutPrefix(id, "animal.train."); ok {
			a.toggleTraining(an, s)
		}
	}
}

// Animal needs, life and products (plan 6.2): the inspector's block
// under the health bar.

// animalStateLine is what the animal is doing: "Feeding", or "Wild |
// Wandering" for a wild one (the Mode when the sim gives no State).
func (a *App) animalStateLine(an game.AnimalView) string {
	if an.Dormant { // a sealed vault's sleeping defender
		return "Dormant"
	}
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

// animalHuntLine is the note under a tame predator's diet (PLAY-017).
func animalHuntLine(an game.AnimalView) string {
	if !an.Hunts || an.Diet == "" {
		return ""
	}
	return "Hunts wild animals when hungry"
}

// Master, bond, training and family (ENG-003, M10): the pane's Master,
// Bond, Mother and Young lines; Master and each learnable skill are
// command buttons (cmds_thing.go).

// animalBondRows are the Master, Bond, Mother and Young lines.
func animalBondRows(an game.AnimalView) []string {
	var rows []string
	if an.Tame {
		rows = append(rows, "Master: "+orNone(an.Master), "Bond: "+orNone(an.Bond))
	}
	if an.Mother != "" {
		rows = append(rows, "Mother: "+an.Mother)
	}
	if len(an.Offspring) > 0 {
		rows = append(rows, "Young: "+strings.Join(an.Offspring, ", "))
	}
	return rows
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// animalTrainable lists the skills an's species can learn.
func animalTrainable(an game.AnimalView) []game.AnimalTrainingView {
	var out []game.AnimalTrainingView
	for _, t := range an.Training {
		if t.Allowed {
			out = append(out, t)
		}
	}
	return out
}

// trainLabel is a training checkbox's label: "Obedience 2/3".
func trainLabel(t game.AnimalTrainingView) string {
	return fmt.Sprintf("%s %d/%d", t.Label, t.Steps, t.Needed)
}

// toggleTraining flips one training checkbox (SetTraining).
func (a *App) toggleTraining(an game.AnimalView, s string) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return
	}
	for _, t := range an.Training {
		if t.Skill == n && t.Allowed {
			a.shell.Enqueue(command.Command{Kind: command.SetTraining, Target: uint64(an.ID), Index: n, Enabled: !t.On})
		}
	}
}

// The pane's tabs (PLAY-018): the Log, and for a tame animal that can
// learn, Training. A tab body draws into the inner rectangle of the
// expanded panel (the panel inset by paneInset), clipped to it.

const (
	animalTabW      = 480 // the expanded panel's wanted width
	animalLogTabH   = 240
	animalTrainRowH = 24
	animalTrainBarW = 120
	animalTrainBarH = 8
	animalTrainLblW = 140
	animalLogID     = "pane.animal.log" // the Log tab's scroll area
)

const trainingIntro = "Handlers train an animal in these skills while they are on. " +
	"Turn each on or off with its command button."

// animalLogRows are the Log tab's rows, wrapped for a body w px wide.
func animalLogRows(an game.AnimalView, w float32) []string {
	return wrapLogRows(an.Log, float64(w-scrollBarW)-4)
}

// declareAnimalLog declares the Log tab's scroll area over inner (the wheel
// and the scrollbar).
func (a *App) declareAnimalLog(an game.AnimalView, inner rect) {
	rows := animalLogRows(an, inner.W)
	a.uiScrollArea(animalLogID, inner, float32(len(rows)*logLineH()), scrollOpts{Step: float32(3 * logLineH())})
}

// drawAnimalLogTab is the Log tab body: the animal's full log, newest
// first, scrolled by the area declareAnimalLog declared.
func (a *App) drawAnimalLogTab(screen *ebiten.Image, an game.AnimalView, inner rect) {
	rows := animalLogRows(an, inner.W)
	if len(rows) == 0 {
		drawText(screen, logEmptyText, float64(inner.X), float64(inner.Y), withColor(logStyle, uiTextDim))
		return
	}
	lh := logLineH()
	off := a.uiSt().scroll[animalLogID]
	for i, row := range rows {
		y := inner.Y + float32(i*lh) - off
		if y+float32(lh) < inner.Y || y > inner.bottom() {
			continue
		}
		drawText(screen, row, float64(inner.X), float64(y), logStyle)
	}
}

// animalTrainingTabH is the Training tab's wanted panel height for the
// skills an can learn: the intro, then a row each, inside the panel inset.
func animalTrainingTabH(an game.AnimalView) float32 {
	return 2*paneInset + 2*lineStep(tsSmall) + 8 + float32(len(animalTrainable(an))*animalTrainRowH)
}

// trainState is a skill row's state word.
func trainState(t game.AnimalTrainingView) string {
	switch {
	case t.Learned:
		return "learned"
	case t.On:
		return "training"
	}
	return "off"
}

// drawAnimalTrainingTab is the Training tab body: one row per learnable
// skill with its steps.
func (a *App) drawAnimalTrainingTab(screen *ebiten.Image, an game.AnimalView, inner rect) {
	x, y := float64(inner.X), float64(inner.Y)
	dim := withColor(tsSmall, uiTextDim)
	for i, ln := range wrapPx(trainingIntro, dim, float64(inner.W)) {
		if i > 1 {
			break
		}
		drawText(screen, ln, x, y+float64(i)*float64(lineStep(tsSmall)), dim)
	}
	y += float64(2*lineStep(tsSmall)) + 8
	for _, t := range animalTrainable(an) {
		drawText(screen, t.Label, x, y+(animalTrainRowH-lineH(tsBody))/2, tsBody)
		bx, by := float32(x)+animalTrainLblW, float32(y)+(animalTrainRowH-animalTrainBarH)/2
		fillRect(screen, rect{bx, by, animalTrainBarW, animalTrainBarH}, uiScrollTrack)
		if t.Needed > 0 {
			fill := uiSliderFill
			if t.Learned {
				fill = uiGood
			}
			fillRect(screen, rect{bx, by, animalTrainBarW * float32(min(t.Steps, t.Needed)) / float32(t.Needed), animalTrainBarH}, fill)
		}
		drawText(screen, fmt.Sprintf("%d/%d  %s", t.Steps, t.Needed, trainState(t)), float64(bx)+animalTrainBarW+10, y+(animalTrainRowH-lineH(tsSmall))/2, dim)
		y += animalTrainRowH
	}
}

// Animal health (MED-09): the pane's health lines under the state line,
// read from the view (AnimalView's health block), and for a tame animal a
// "Care" command whose options are the five care levels (SetMedicalCare
// on the animal). The Animals table's Health cell marks a bleeding or
// infected animal the same way.

// animalHealthRows are the pane's health lines: the headline when it is
// not plain health, blood loss, the worst infection, a due tend, then the
// hurt and lost parts (animalPartRows).
func animalHealthRows(an game.AnimalView) []string {
	var rows []string
	if an.HealthState != "" && an.HealthState != "Healthy" && an.HealthState != "Injured" {
		rows = append(rows, an.HealthState) // "Bleeding 25%/day · death in 31h"
	}
	if an.BloodLossPct > 0 {
		s := fmt.Sprintf("Blood loss %d%%", an.BloodLossPct)
		if an.BloodStage != "" {
			s += " (" + an.BloodStage + ")"
		}
		rows = append(rows, s)
	}
	if an.InfectionPct > 0 {
		s := fmt.Sprintf("Infection %d%% · immunity %d%%", an.InfectionPct, an.ImmunityPct)
		if an.InfectionLosing {
			s += " (losing)"
		}
		rows = append(rows, s)
	}
	if an.NeedsTend {
		rows = append(rows, "Needs tending")
	}
	return append(rows, animalPartRows(an)...)
}

// animalPartRowsMax caps the part lines; the rest fold into "+N more".
const animalPartRowsMax = 6

// animalPartRows are the pane's anatomy lines (MED-13): the moving
// capacity while it is down, then each hurt or lost part ("Front left
// leg: missing", "Torso: 12/28 HP, bleeding"), at most animalPartRowsMax.
func animalPartRows(an game.AnimalView) []string {
	if len(an.Parts) == 0 {
		return nil
	}
	var rows []string
	if an.MovingPct < 100 {
		rows = append(rows, fmt.Sprintf("Moving %d%%", an.MovingPct))
	}
	for i, p := range an.Parts {
		if i == animalPartRowsMax {
			rows = append(rows, fmt.Sprintf("+%d more", len(an.Parts)-i))
			break
		}
		s := p.Label + ": "
		if s != ": " {
			s = strings.ToUpper(s[:1]) + s[1:]
		}
		if p.Missing {
			s += "missing"
		} else {
			s += fmt.Sprintf("%d/%d HP", p.HP, p.MaxHP)
		}
		if p.Bleeding {
			s += ", bleeding"
		}
		rows = append(rows, s)
	}
	return rows
}

// animalHealthAlarm reports a bleeding or infected animal (the Animals
// table marks its Health cell).
func animalHealthAlarm(an game.AnimalView) bool { return an.BleedPerDay > 0 || an.InfectionPct > 0 }

// animalHealthCell is the Animals table's Health cell: "62%", with a "!"
// while the animal bleeds or fights an infection.
func animalHealthCell(an game.AnimalView) string {
	if animalHealthAlarm(an) {
		return fmt.Sprintf("%d%%!", an.Health)
	}
	return fmt.Sprintf("%d%%", an.Health)
}

// animalCareCmd is a tame animal's "Care" command: its care level now,
// and the five levels as options (the current one marked); a plain click
// lists them, like the master picker.
func (a *App) animalCareCmd(an game.AnimalView) cmdButton {
	cur := medcare.Policy(an.CareLevel)
	orders := make([]game.OrderOption, 0, medcare.NumPolicies)
	opts := make([]cmdOption, 0, medcare.NumPolicies)
	for p := medcare.Policy(0); p < medcare.NumPolicies; p++ {
		cmd := command.Command{Kind: command.SetMedicalCare, Pawn: uint64(an.ID), Mode: uint8(p)}
		label := p.Label()
		if p == cur {
			label += " (now)"
		}
		orders = append(orders, game.OrderOption{Label: label, Cmd: cmd})
		opts = append(opts, cmdOption{Label: label, Cmd: &cmd})
	}
	return cmdButton{ID: "cmd.animal.care", Label: "Care: " + careShort(cur),
		Icon:    strings.TrimPrefix(careIcon(cur), "ui_icon/"),
		Desc:    "Pick how a doctor treats it. " + careTip(cur),
		Options: func(*App) []cmdOption { return opts },
		Run: func(a *App) {
			mx, my := a.currentCursor()
			a.openPick(float32(mx), float32(my), true, orders, func(i int) { a.runOption(opts[i]) })
		}}
}
