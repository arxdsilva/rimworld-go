package windows

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
)

var WorkColumns = []string{"FF", "Doc", "Grow", "Cook", "Bld", "Hunt", "Haul", "Cln", "Rsh", "Mine", "Crft", "Hndl", "Ward"}

// WorkLive maps live work columns (indexes into workColumns, which follow
// work.Type order) to their command and pawn flag.
var WorkLive = map[int]struct {
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

// WorkPriority is the colonist's priority for a work column (0 = off).
func WorkPriority(p game.PawnView, col int) int {
	if col < len(p.Priorities) {
		return p.Priorities[col]
	}
	if w, ok := WorkLive[col]; ok && w.on(p) {
		return 3
	}
	return 0
}

// NextPriority cycles a cell: off -> 1 -> 2 -> 3 -> 4 -> off.
func NextPriority(v int) int {
	if v >= 4 {
		return 0
	}
	return v + 1
}

const (
	workRowH  = 22
	WorkNameW = 110
	workCellW = 46 // wide enough for the skill level, the priority and the passion glyph
)

// WorkColSkill is the skill each work column trains and is paced by
// (NG-1 §2.12); Firefighting, Hauling and Cleaning have none.
var WorkColSkill = [...]struct {
	Skill skill.ID
	OK    bool
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

// WorkHelp is the header hover card of each work column.
var WorkHelp = [...]string{
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

// incapable reports colonist p can never do work column col, and why
// ("Incapable: Violence (backstory: Convoy mechanic)").
func (w Work) incapable(p game.PawnView, col int) (string, bool) {
	if !p.Incapable.Enabled(work.Type(col)) {
		return "", false
	}
	why := ""
	if w.live() {
		why = w.sh.Env().Game.IncapableReason(p.ID, work.Type(col))
	}
	if why == "" {
		why = "Incapable"
	}
	return why, true
}

// WorkCellRect is the rectangle of colonist row's cell in column col.
func WorkCellRect(r uikit.Button, row, col int) gfx.Rect {
	hx := r.X + 12 + WorkNameW
	y := r.Y + 48 + float32(row*workRowH)
	return gfx.Rect{X: hx + float32(col*workCellW), Y: y, W: workCellW - 4, H: workRowH - 2}
}

// workRowShown reports the row fits the panel.
func workRowShown(r uikit.Button, row int) bool { return int(r.Y)+48+row*workRowH <= int(r.Y+r.H)-20 }

// workCellTip is the hover card of a live cell: priority, skill, passion.
func (w Work) cellTip(p game.PawnView, col int) string {
	name := WorkHelp[col]
	name, _, _ = strings.Cut(name, "\n")
	tip := w.sh.PawnLabel(p) + ": " + name + "\n"
	if pr := WorkPriority(p, col); pr > 0 {
		tip += fmt.Sprintf("Priority %d (1 first, 4 last). ", pr)
	} else {
		tip += "Never. "
	}
	if col < len(WorkColSkill) && WorkColSkill[col].OK {
		sk := WorkColSkill[col].Skill
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

// Tips declares the Work tab's hover cards: each column header names
// its work and skill, each incapable cell says why.
func (w Work) Tips(sw, sh int) {
	if !w.Open || (w.Mode != "work" && w.Mode != "") {
		return
	}
	r := w.Rect()
	for col := range WorkColumns {
		cr := WorkCellRect(r, 0, col)
		w.kit().Tip("work.hdr."+uikit.Num(col), gfx.Rect{X: cr.X, Y: r.Y + 22, W: cr.W, H: 22}, WorkHelp[col])
	}
	for row, p := range w.sh.Pawns() {
		if !workRowShown(r, row) {
			break
		}
		for col := range WorkColumns {
			if _, live := WorkLive[col]; !live {
				continue
			}
			tip := w.cellTip(p, col)
			if why, inc := w.incapable(p, col); inc {
				tip = why
			}
			w.kit().Tip(fmt.Sprintf("work.tip.%d.%d", p.ID, col), WorkCellRect(r, row, col), tip)
		}
	}
}

// want is the Work window's wanted size: the name column, every work
// column, and a row per colonist.
func (w Work) want() (float32, float32) {
	n := 0
	if w.live() {
		n = len(w.sh.Pawns())
	}
	return float32(12 + WorkNameW + len(WorkColumns)*workCellW + 8), float32(max(160, 48+n*workRowH+20))
}

func WorkCellValue(p game.PawnView, col int) string {
	if _, live := WorkLive[col]; !live {
		return ""
	}
	if v := WorkPriority(p, col); v > 0 {
		return strconv.Itoa(v)
	}
	return "-"
}

// buttons are the Work window's priority cells (an incapable one disabled).
func (w Work) buttons() []uikit.Button {
	r := w.Rect()
	hx := int(r.X) + 12 + WorkNameW
	pawns := w.sh.Pawns()
	out := make([]uikit.Button, 0)
	for row, p := range pawns {
		y := int(r.Y) + 48 + row*workRowH
		if y > int(r.Y+r.H)-20 {
			break
		}
		for col := range WorkColumns {
			if _, ok := WorkTypeForCol(col); !ok {
				continue
			}
			b := uikit.Button{
				ID:    ids.PrefixWorkCell + strconv.FormatUint(uint64(p.ID), 10) + "." + strconv.Itoa(col),
				Label: "",
				X:     float32(hx + col*workCellW),
				Y:     float32(y),
				W:     float32(workCellW - 4),
				H:     float32(workRowH - 2),
			}
			if why, inc := w.incapable(p, col); inc {
				// An incapable cell is hatched and can't be clicked: the
				// reason shows as its tooltip (workTipsUI) and as the hint.
				b.Disabled, b.Reason, b.Quiet = true, why, true
			}
			out = append(out, b)
		}
	}
	return out
}

func WorkTypeForCol(col int) (command.WorkType, bool) {
	w, ok := WorkLive[col]
	return w.cmd, ok
}

// draw draws the Work window.
func (w Work) draw(screen *ebiten.Image, sw, sh int) {
	r := w.Rect()
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	gfx.Text(screen, "Work priorities", float64(r.X)+12, float64(r.Y)+8, gfx.WithColor(gfx.Label, uikit.Heading))
	hintX := float64(r.X) + 12 + float64(gfx.TextW("Work priorities", gfx.Label)) + 16
	gfx.Text(screen, gfx.Ellipsize("1 = do first ... 4 = do last, - = never; click a cell to change", gfx.Small, float64(r.X+r.W)-12-hintX),
		hintX, float64(r.Y)+8+(gfx.LineH(gfx.Label)-gfx.LineH(gfx.Small))/2, gfx.WithColor(gfx.Small, uikit.TextDim))
	hx := int(r.X) + 12 + WorkNameW
	for i, col := range WorkColumns {
		gfx.Text(screen, col, float64(hx+i*workCellW)+float64(workCellW-4)/2, float64(r.Y)+28, gfx.TextStyle{Size: gfx.Small.Size, Color: uikit.TextDim, Align: gfx.AlignCenter})
	}
	pawns := w.sh.Pawns()
	for row, p := range pawns {
		y := int(r.Y) + 48 + row*workRowH
		if y > int(r.Y+r.H)-20 {
			break
		}
		gfx.Text(screen, gfx.Ellipsize(w.sh.PawnLabel(p), gfx.Body, WorkNameW-8), float64(r.X)+12, float64(y)+(workRowH-2-gfx.LineH(gfx.Body))/2, gfx.Body)
	}
	mx, my := w.sh.Cursor()
	drawButtons(screen, w.buttons(), mx, my)
	for row, p := range pawns {
		y := int(r.Y) + 48 + row*workRowH
		if y > int(r.Y+r.H)-20 {
			break
		}
		for col := range WorkColumns {
			v := WorkCellValue(p, col)
			if v == "" {
				continue
			}
			cell := WorkCellRect(r, row, col)
			if p.Incapable.Enabled(work.Type(col)) {
				drawHatch(screen, cell)
				continue
			}
			if pr := WorkPriority(p, col); pr > 0 {
				// brighter for more urgent work
				gfx.FillRect(screen, cell.X, cell.Y, cell.W, cell.H,
					uikit.LerpRGBA(uikit.SliderFill, uikit.BtnSelected, float64(pr-1)/3), false)
			}
			w.drawSkill(screen, p, col, cell)
			gfx.Text(screen, v, float64(cell.X+cell.W/2), float64(cell.Y)+(float64(cell.H)-gfx.LineH(gfx.Body))/2, gfx.TextStyle{Size: gfx.Body.Size, Align: gfx.AlignCenter})
		}
	}
}

// drawSkill adds the related skill's level (left) and the passion glyph
// (right) to a live Work cell. Work with no skill shows neither.
func (w Work) drawSkill(screen *ebiten.Image, p game.PawnView, col int, cell gfx.Rect) {
	if col >= len(WorkColSkill) || !WorkColSkill[col].OK {
		return
	}
	sk := WorkColSkill[col].Skill
	ls := gfx.Small
	ls.Color = uikit.TextDim
	gfx.Text(screen, strconv.Itoa(int(p.SkillLv[sk])), float64(cell.X+3), float64(cell.Y)+(float64(cell.H)-gfx.LineH(ls))/2, ls)
	if ps := int(p.Passion[sk]); ps > 0 {
		w.sh.DrawPassion(screen, ps, float64(cell.Right()-13), float64(cell.Y)+(float64(cell.H)-10)/2, 10, ls, float64(cell.Y)+3)
	}
}

// drawHatch fills r with the disabled colour and diagonal stripes: work the
// colonist can never do.
func drawHatch(dst *ebiten.Image, r gfx.Rect) {
	gfx.FillRect(dst, r.X, r.Y, r.W, r.H, uikit.BtnDisabled, false)
	sub := gfx.ClipTo(dst, r)
	c := uikit.TextDisabled
	for d := float32(-r.H); d < r.W; d += 6 {
		gfx.Line(sub, r.X+d, r.Bottom(), r.X+d+r.H, r.Y, 1, c, false)
	}
	uikit.StrokeRect(dst, r, uikit.BtnBorder)
}
