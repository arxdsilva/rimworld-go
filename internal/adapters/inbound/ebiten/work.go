package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
)

var workColumns = []string{"FF", "Doc", "Grow", "Cook", "Bld", "Hunt", "Haul", "Cln", "Rsh", "Mine", "Crft", "Hndl", "Ward"}

// workLive maps live work columns (indexes into workColumns, which follow
// work.Type order) to their command and pawn flag.
var workLive = map[int]struct {
	cmd command.WorkType
	on  func(game.PawnView) bool
}{
	0:  {command.WorkFirefight, func(p game.PawnView) bool { return p.WorkFirefight }}, // PLAY-015
	1:  {command.WorkDoctor, func(p game.PawnView) bool { return p.WorkDoctor }},
	2:  {command.WorkGrow, func(p game.PawnView) bool { return p.WorkGrow }},
	3:  {command.WorkCook, func(p game.PawnView) bool { return p.WorkCook }},
	4:  {command.WorkConstruct, func(p game.PawnView) bool { return p.WorkConstruct }},
	5:  {command.WorkHunt, func(p game.PawnView) bool { return p.WorkHunt }},
	6:  {command.WorkHaul, func(p game.PawnView) bool { return p.WorkHaul }},
	7:  {command.WorkClean, func(p game.PawnView) bool { return p.WorkClean }},
	8:  {command.WorkResearch, func(p game.PawnView) bool { return p.WorkResearch }},
	9:  {command.WorkMine, func(p game.PawnView) bool { return p.WorkMine }},
	10: {command.WorkCraft, func(p game.PawnView) bool { return p.WorkCraft }},
	11: {command.WorkHandle, func(p game.PawnView) bool { return false }}, // priorities carry it
	12: {command.WorkWarden, func(p game.PawnView) bool { return false }}, // ENG-003: off by default
}

// workPriority is the colonist's priority for a work column (0 = off).
func workPriority(p game.PawnView, col int) int {
	if col < len(p.Priorities) {
		return p.Priorities[col]
	}
	if w, ok := workLive[col]; ok && w.on(p) {
		return 3
	}
	return 0
}

// nextPriority cycles a cell: off -> 1 -> 2 -> 3 -> 4 -> off.
func nextPriority(v int) int {
	if v >= 4 {
		return 0
	}
	return v + 1
}

const (
	workRowH  = 22
	workNameW = 110
	workCellW = 46 // wide enough for the skill level, the priority and the passion glyph
)

// workColSkill is the skill each work column trains and is paced by
// (NG-1 §2.12); Firefighting, Hauling and Cleaning have none.
var workColSkill = [...]struct {
	sk skill.ID
	ok bool
}{
	1:  {skill.Medicine, true},
	2:  {skill.Plants, true},
	3:  {skill.Cooking, true},
	4:  {skill.Construction, true},
	5:  {skill.Shooting, true},
	8:  {skill.Intellectual, true},
	9:  {skill.Mining, true},
	10: {skill.Crafting, true},
	11: {skill.Animals, true},
	12: {skill.Social, true},
}

// workHelp is the header hover card of each work column.
var workHelp = [...]string{
	0:  "Firefighting\nBeat out fires inside the Home area and your allowed area. No skill.",
	1:  "Doctor\nTend the wounded and the sick, and carry the downed to bed. Skill: Medicine.",
	2:  "Growing\nSow, harvest and cut plants, and cut grass. Skill: Plants.",
	3:  "Cooking\nCook meals at a stove or campfire and butcher animals. Skill: Cooking.",
	4:  "Building\nBuild, deconstruct, repair, roof and smooth. Skill: Construction.",
	5:  "Hunting\nHunt wild animals with a rifle, or a melee weapon. Skill: Shooting (Melee).",
	6:  "Hauling\nCarry items to stockpiles, benches and frames. No skill.",
	7:  "Cleaning\nSweep dirt and filth off the Home area. No skill.",
	8:  "Research\nStudy at a research bench. Skill: Intellectual.",
	9:  "Mining\nDig out rock and ore. Skill: Mining.",
	10: "Crafting\nWork at benches: workbench, tailoring, stonecutting. Skill: Crafting.",
	11: "Handling\nTame, milk, shear and collect from animals. Skill: Animals.",
	12: "Warden\nChat with prisoners, recruit them and bring them food. Off unless you turn it on. Skill: Social.",
}

// workIncapable reports colonist p can never do work column col, and why
// ("Incapable: Violence (backstory: Convoy mechanic)").
func (a *App) workIncapable(p game.PawnView, col int) (string, bool) {
	if !p.Incapable.Enabled(work.Type(col)) {
		return "", false
	}
	why := ""
	if a.renderer != nil {
		why = a.renderer.game.IncapableReason(p.ID, work.Type(col))
	}
	if why == "" {
		why = "Incapable"
	}
	return why, true
}

// workCellRect is the rectangle of colonist row's cell in column col.
func workCellRect(r button, row, col int) rect {
	hx := r.x + 12 + workNameW
	y := r.y + 48 + float32(row*workRowH)
	return rect{hx + float32(col*workCellW), y, workCellW - 4, workRowH - 2}
}

// workRowShown reports the row fits the panel.
func workRowShown(r button, row int) bool { return int(r.y)+48+row*workRowH <= int(r.y+r.h)-20 }

// workCellTip is the hover card of a live cell: priority, skill, passion.
func workCellTip(p game.PawnView, col int) string {
	name := workHelp[col]
	name, _, _ = strings.Cut(name, "\n")
	tip := pawnLabel(p) + ": " + name + "\n"
	if pr := workPriority(p, col); pr > 0 {
		tip += fmt.Sprintf("Priority %d (1 first, 4 last). ", pr)
	} else {
		tip += "Never. "
	}
	if col < len(workColSkill) && workColSkill[col].ok {
		sk := workColSkill[col].sk
		tip += fmt.Sprintf("%s level %d, %s.", sk.Label(), p.SkillLv[sk], passionName(int(p.Passion[sk])))
	}
	return tip
}

// passionName is "no passion", "minor passion" or "major passion".
func passionName(p int) string {
	switch p {
	case 1:
		return "minor passion"
	case 2:
		return "major passion"
	}
	return "no passion"
}

// workTipsUI declares the Work tab's hover cards: each column header names
// its work and skill, each incapable cell says why.
func (a *App) workTipsUI(sw, sh int) {
	if !a.workOpen || (a.panelMode != "work" && a.panelMode != "") {
		return
	}
	r := a.windowRect()
	for col := range workColumns {
		cr := workCellRect(r, 0, col)
		a.uiTip("work.hdr."+uiNum(col), rect{cr.X, r.y + 22, cr.W, 22}, workHelp[col])
	}
	for row, p := range a.renderer.snapshot() {
		if !workRowShown(r, row) {
			break
		}
		for col := range workColumns {
			if _, live := workLive[col]; !live {
				continue
			}
			tip := workCellTip(p, col)
			if why, inc := a.workIncapable(p, col); inc {
				tip = why
			}
			a.uiTip(fmt.Sprintf("work.tip.%d.%d", p.ID, col), workCellRect(r, row, col), tip)
		}
	}
}

// workWant is the Work window's wanted size: the name column, every work
// column, and a row per colonist.
func (a *App) workWant() (float32, float32) {
	n := 0
	if a.renderer != nil {
		n = len(a.renderer.snapshot())
	}
	return float32(12 + workNameW + len(workColumns)*workCellW + 8), float32(max(160, 48+n*workRowH+20))
}

func workCellValue(p game.PawnView, col int) string {
	if _, live := workLive[col]; !live {
		return ""
	}
	if v := workPriority(p, col); v > 0 {
		return strconv.Itoa(v)
	}
	return "-"
}

// workButtons are the open hand-drawn window's buttons (Work, Schedule,
// Research, Animals); a kit window (History, Assign, World, Factions)
// declares its own widgets and has none here.
func (a *App) workButtons() []button {
	if a.renderer == nil || !a.workOpen || !a.legacyWindow() {
		return nil
	}
	if a.panelMode == "schedule" {
		return a.scheduleButtons()
	}
	if a.panelMode == "research" {
		return a.researchButtons()
	}
	if a.panelMode == "animals" {
		return a.animalsTabButtons()
	}
	r := a.windowRect()
	hx := int(r.x) + 12 + workNameW
	pawns := a.renderer.snapshot()
	out := make([]button, 0)
	for row, p := range pawns {
		y := int(r.y) + 48 + row*workRowH
		if y > int(r.y+r.h)-20 {
			break
		}
		for col := range workColumns {
			if _, ok := workTypeForCol(col); !ok {
				continue
			}
			b := button{
				id:    "work.cell." + strconv.FormatUint(uint64(p.ID), 10) + "." + strconv.Itoa(col),
				label: "",
				x:     float32(hx + col*workCellW),
				y:     float32(y),
				w:     float32(workCellW - 4),
				h:     float32(workRowH - 2),
			}
			if why, inc := a.workIncapable(p, col); inc {
				// An incapable cell is hatched and can't be clicked: the
				// reason shows as its tooltip (workTipsUI) and as the hint.
				b.disabled, b.reason, b.quiet = true, why, true
			}
			out = append(out, b)
		}
	}
	return out
}

func workTypeForCol(col int) (command.WorkType, bool) {
	w, ok := workLive[col]
	return w.cmd, ok
}

func (a *App) drawWork(screen *ebiten.Image, sw, sh int) {
	if a.renderer == nil {
		return
	}
	if a.panelMode == "schedule" {
		a.drawSchedule(screen, sw, sh)
		return
	}
	if a.panelMode == "research" {
		a.drawResearch(screen, sw, sh)
		return
	}
	if a.panelMode == "animals" {
		a.drawAnimalsTab(screen, sw, sh)
		return
	}
	r := a.windowRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawText(screen, "Work priorities", float64(r.x)+12, float64(r.y)+8, withColor(tsLabel, uiHeading))
	hintX := float64(r.x) + 12 + float64(textW("Work priorities", tsLabel)) + 16
	drawText(screen, ellipsize("1 = do first ... 4 = do last, - = never; click a cell to change", tsSmall, float64(r.x+r.w)-12-hintX),
		hintX, float64(r.y)+8+(lineH(tsLabel)-lineH(tsSmall))/2, withColor(tsSmall, uiTextDim))
	hx := int(r.x) + 12 + workNameW
	for i, col := range workColumns {
		drawText(screen, col, float64(hx+i*workCellW)+float64(workCellW-4)/2, float64(r.y)+28, textStyle{Size: tsSmall.Size, Color: uiTextDim, Align: alignCenter})
	}
	pawns := a.renderer.snapshot()
	for row, p := range pawns {
		y := int(r.y) + 48 + row*workRowH
		if y > int(r.y+r.h)-20 {
			break
		}
		drawText(screen, ellipsize(pawnLabel(p), tsBody, workNameW-8), float64(r.x)+12, float64(y)+(workRowH-2-lineH(tsBody))/2, tsBody)
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.workButtons(), mx, my)
	for row, p := range pawns {
		y := int(r.y) + 48 + row*workRowH
		if y > int(r.y+r.h)-20 {
			break
		}
		for col := range workColumns {
			v := workCellValue(p, col)
			if v == "" {
				continue
			}
			cell := workCellRect(r, row, col)
			if p.Incapable.Enabled(work.Type(col)) {
				drawHatch(screen, cell)
				continue
			}
			if pr := workPriority(p, col); pr > 0 {
				// brighter for more urgent work
				uiFillRect(screen, cell.X, cell.Y, cell.W, cell.H,
					lerpRGBA(uiSliderFill, uiBtnSelected, float64(pr-1)/3), false)
			}
			a.drawWorkSkill(screen, p, col, cell)
			drawText(screen, v, float64(cell.X+cell.W/2), float64(cell.Y)+(float64(cell.H)-lineH(tsBody))/2, textStyle{Size: tsBody.Size, Align: alignCenter})
		}
	}
}

// drawWorkSkill adds the related skill's level (left) and the passion glyph
// (right) to a live Work cell. Work with no skill shows neither.
func (a *App) drawWorkSkill(screen *ebiten.Image, p game.PawnView, col int, cell rect) {
	if col >= len(workColSkill) || !workColSkill[col].ok {
		return
	}
	sk := workColSkill[col].sk
	ls := tsSmall
	ls.Color = uiTextDim
	drawText(screen, strconv.Itoa(int(p.SkillLv[sk])), float64(cell.X+3), float64(cell.Y)+(float64(cell.H)-lineH(ls))/2, ls)
	if ps := int(p.Passion[sk]); ps > 0 {
		a.drawPassion(screen, ps, float64(cell.right()-13), float64(cell.Y)+(float64(cell.H)-10)/2, 10, ls, float64(cell.Y)+3)
	}
}

// drawHatch fills r with the disabled colour and diagonal stripes: work the
// colonist can never do.
func drawHatch(dst *ebiten.Image, r rect) {
	uiFillRect(dst, r.X, r.Y, r.W, r.H, uiBtnDisabled, false)
	sub := clipTo(dst, r)
	c := uiTextDisabled
	for d := float32(-r.H); d < r.W; d += 6 {
		uiLine(sub, r.X+d, r.bottom(), r.X+d+r.H, r.Y, 1, c, false)
	}
	strokeRect(dst, r, uiBtnBorder)
}
