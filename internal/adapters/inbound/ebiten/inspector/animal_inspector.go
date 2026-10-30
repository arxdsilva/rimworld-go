package inspector

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/command"
)

// Animal selection, its inspector (hunt toggle) and the hostile alert.

// alertClr is the hostile-animal marker and alert colour: the danger token.
var alertClr = uikit.Danger

func animalAt(w game.WorldView, x, y int) (game.AnimalView, bool) {
	for _, an := range w.Animals {
		if an.X == x && an.Y == y {
			return an, true
		}
	}
	return game.AnimalView{}, false
}

func (a Animal) SelectAnimalAt(tx, ty int) bool {
	an, ok := animalAt(a.shell.World(), tx, ty)
	if !ok {
		a.sel.HasAnimal = false
		return false
	}
	a.sel.Select(selection.Cand{Kind: selection.Animal, ID: an.ID})
	return true
}

func (a Animal) clearAnimal() { a.sel.HasAnimal = false }

func (a *Pane) SelectedAnimal() (game.AnimalView, bool) {
	if !a.sel.HasAnimal || a.sim() == nil {
		return game.AnimalView{}, false
	}
	for _, an := range a.shell.World().Animals {
		if an.ID == a.sel.SelAnimal {
			return an, true
		}
	}
	a.sel.HasAnimal = false // it died or left
	return game.AnimalView{}, false
}

// animalLabel is the species' display name ("arctic fox"): the view's
// Label, else the key with its underscores spaced out.
func AnimalLabel(an game.AnimalView) string {
	switch {
	case an.Label != "":
		return an.Label
	case an.Species != "":
		return strings.ReplaceAll(an.Species, "_", " ")
	}
	return "animal"
}

func AnimalTitle(an game.AnimalView) string {
	l := AnimalLabel(an)
	if an.Tame && an.Name != "" {
		return an.Name + " (tame " + l + ")"
	}
	return strings.ToUpper(l[:1]) + l[1:]
}

// animalInitial is the letter shown on the fallback icon disc.
func AnimalInitial(label string) string {
	for _, r := range label {
		if r != ' ' && r != '_' {
			return strings.ToUpper(string(r))
		}
	}
	return "?"
}

// drawAnimalIcon draws the species' ui_icon at (x, y), size px square; a
// species without one gets its initial on a disc.
func DrawAnimalIcon(dst *ebiten.Image, art *art.Catalog, an game.AnimalView, x, y, size float64) {
	if img := art.Image("ui_icon/"+an.Species, color.NRGBA{}); img != nil {
		gfx.Icon(dst, img, x, y, size)
		return
	}
	r := float32(size) / 2
	gfx.FillCircle(dst, float32(x)+r, float32(y)+r, r, uikit.Btn, true)
	gfx.StrokeCircle(dst, float32(x)+r, float32(y)+r, r, 1.5, uikit.BtnBorder, true)
	uikit.DrawCentered(dst, AnimalInitial(AnimalLabel(an)), x+float64(r), y+float64(r), gfx.Label)
}

func (a Animal) ApplyAnimalClick(id string) {
	an, ok := a.SelectedAnimal()
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
			a.shell.AreaPicker("Allowed area", an.AreaRef, true, func(ref int) { a.shell.AssignArea(an.ID, ref) })
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
func (a Animal) AnimalStateLine(an game.AnimalView) string {
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
		return "Wild | " + Capitalize(st)
	}
	return Capitalize(st)
}

// animalLifeLine is "Female | adult | 34 days old" (the sex, stage and
// age), "" when the view carries none of it.
func AnimalLifeLine(an game.AnimalView) string {
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
func AnimalHasNeeds(an game.AnimalView) bool { return an.State != "" }

func AnimalPregLine(an game.AnimalView) string {
	switch {
	case an.Pregnant:
		return fmt.Sprintf("Pregnant %d%%", an.PregPct)
	case an.EggsFertile:
		return "Eggs are fertile"
	}
	return ""
}

func AnimalDietLine(an game.AnimalView) string {
	if an.Diet == "" {
		return ""
	}
	return "Diet: " + an.Diet
}

// AnimalFeedLine is the prepared food a tame animal eats besides its
// diet (FEED-001): "Also eats kibble, pemmican and meals" (meals only
// when no feed is in reach), "Also eats kibble and pemmican" (tree
// eaters), or "Eats no kibble or meals" (strict carnivores).
func AnimalFeedLine(an game.AnimalView) string {
	s, ok := animal.ParseSpecies(an.Species)
	if !an.Tame || !ok || an.Diet == "" {
		return ""
	}
	if made := animal.EcologyOf(s).Diet.Made(); made != "" {
		return "Also eats " + made
	}
	return "Eats no kibble or meals"
}

// animalHuntLine is the note under a tame predator's diet (PLAY-017).
func AnimalHuntLine(an game.AnimalView) string {
	if !an.Hunts || an.Diet == "" {
		return ""
	}
	return "Hunts wild animals when hungry"
}

// Master, bond, training and family (ENG-003, M10): the pane's Master,
// Bond, Mother and Young lines; Master and each learnable skill are
// command buttons (cmds_thing.go).

// animalBondRows are the Master, Bond, Mother and Young lines.
func AnimalBondRows(an game.AnimalView) []string {
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
func (a Animal) toggleTraining(an game.AnimalView, s string) {
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
	AnimalLogID     = "pane.animal.log" // the Log tab's scroll area
)

const trainingIntro = "Handlers train an animal in these skills while they are on. " +
	"Turn each on or off with its command button."

// animalLogRows are the Log tab's rows, wrapped for a body w px wide.
func animalLogRows(an game.AnimalView, w float32) []string {
	return WrapLogRows(an.Log, float64(w-uikit.ScrollBarW)-4)
}

// declareAnimalLog declares the Log tab's scroll area over inner (the wheel
// and the scrollbar).
func (a Animal) declareAnimalLog(an game.AnimalView, inner gfx.Rect) {
	rows := animalLogRows(an, inner.W)
	a.kit().ScrollArea(AnimalLogID, inner, float32(len(rows)*logLineH()), uikit.ScrollOpts{Step: float32(3 * logLineH())})
}

// drawAnimalLogTab is the Log tab body: the animal's full log, newest
// first, scrolled by the area declareAnimalLog declared.
func (a Animal) drawAnimalLogTab(screen *ebiten.Image, an game.AnimalView, inner gfx.Rect) {
	rows := animalLogRows(an, inner.W)
	if len(rows) == 0 {
		gfx.Text(screen, logEmptyText, float64(inner.X), float64(inner.Y), gfx.WithColor(LogStyle, uikit.TextDim))
		return
	}
	lh := logLineH()
	off := a.kit().Scroll[AnimalLogID]
	for i, row := range rows {
		y := inner.Y + float32(i*lh) - off
		if y+float32(lh) < inner.Y || y > inner.Bottom() {
			continue
		}
		gfx.Text(screen, row, float64(inner.X), float64(y), LogStyle)
	}
}

// animalTrainingTabH is the Training tab's wanted panel height for the
// skills an can learn: the intro, then a row each, inside the panel inset.
func animalTrainingTabH(an game.AnimalView) float32 {
	return 2*PaneInset + 2*gfx.LineStep(gfx.Small) + 8 + float32(len(animalTrainable(an))*animalTrainRowH)
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
func (a Animal) drawAnimalTrainingTab(screen *ebiten.Image, an game.AnimalView, inner gfx.Rect) {
	x, y := float64(inner.X), float64(inner.Y)
	dim := gfx.WithColor(gfx.Small, uikit.TextDim)
	for i, ln := range gfx.WrapPx(trainingIntro, dim, float64(inner.W)) {
		if i > 1 {
			break
		}
		gfx.Text(screen, ln, x, y+float64(i)*float64(gfx.LineStep(gfx.Small)), dim)
	}
	y += float64(2*gfx.LineStep(gfx.Small)) + 8
	for _, t := range animalTrainable(an) {
		gfx.Text(screen, t.Label, x, y+(animalTrainRowH-gfx.LineH(gfx.Body))/2, gfx.Body)
		bx, by := float32(x)+animalTrainLblW, float32(y)+(animalTrainRowH-animalTrainBarH)/2
		uikit.FillRect(screen, gfx.Rect{X: bx, Y: by, W: animalTrainBarW, H: animalTrainBarH}, uikit.ScrollTrack)
		if t.Needed > 0 {
			fill := uikit.SliderFill
			if t.Learned {
				fill = uikit.Good
			}
			uikit.FillRect(screen, gfx.Rect{X: bx, Y: by, W: animalTrainBarW * float32(min(t.Steps, t.Needed)) / float32(t.Needed), H: animalTrainBarH}, fill)
		}
		gfx.Text(screen, fmt.Sprintf("%d/%d  %s", t.Steps, t.Needed, trainState(t)), float64(bx)+animalTrainBarW+10, y+(animalTrainRowH-gfx.LineH(gfx.Small))/2, dim)
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
func AnimalHealthRows(an game.AnimalView) []string {
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
	return append(rows, AnimalPartRows(an)...)
}

// animalPartRowsMax caps the part lines; the rest fold into "+N more".
const AnimalPartRowsMax = 6

// animalPartRows are the pane's anatomy lines (MED-13): the moving
// capacity while it is down, then each hurt or lost part ("Front left
// leg: missing", "Torso: 12/28 HP, bleeding"), at most animalPartRowsMax.
func AnimalPartRows(an game.AnimalView) []string {
	if len(an.Parts) == 0 {
		return nil
	}
	var rows []string
	if an.MovingPct < 100 {
		rows = append(rows, fmt.Sprintf("Moving %d%%", an.MovingPct))
	}
	for i, p := range an.Parts {
		if i == AnimalPartRowsMax {
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
func AnimalHealthCell(an game.AnimalView) string {
	if animalHealthAlarm(an) {
		return fmt.Sprintf("%d%%!", an.Health)
	}
	return fmt.Sprintf("%d%%", an.Health)
}

// animalCareCmd is a tame animal's "Care" command: its care level now,
// and the five levels as options (the current one marked); a plain click
// lists them, like the master picker.
func (a Animal) animalCareCmd(an game.AnimalView) Button {
	cur := medcare.Policy(an.CareLevel)
	orders := make([]game.OrderOption, 0, medcare.NumPolicies)
	opts := make([]Option, 0, medcare.NumPolicies)
	for p := medcare.Policy(0); p < medcare.NumPolicies; p++ {
		cmd := command.Command{Kind: command.SetMedicalCare, Pawn: uint64(an.ID), Mode: uint8(p)}
		label := p.Label()
		if p == cur {
			label += " (now)"
		}
		orders = append(orders, game.OrderOption{Label: label, Cmd: cmd})
		opts = append(opts, Option{Label: label, Cmd: &cmd})
	}
	return Button{ID: "cmd.animal.care", Label: "Care: " + CareShort(cur),
		Icon:    strings.TrimPrefix(careIcon(cur), "ui_icon/"),
		Desc:    "Pick how a doctor treats it. " + careTip(cur),
		Options: func() []Option { return opts },
		Run: func() {
			mx, my := a.shell.Cursor()
			a.shell.Pick(float32(mx), float32(my), true, orders, func(i int) { a.Commands().runOption(opts[i]) })
		}}
}
