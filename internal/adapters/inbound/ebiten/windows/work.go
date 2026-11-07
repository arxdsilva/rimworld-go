package windows

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
)

var WorkColumns = []string{"FF", "Doc", "Grow", "Cook", "Bld", "Hunt", "Haul", "Cln", "Rsh", "Mine", "Crft", "Hndl", "Ward", "Cut"}

// WorkLive maps live work columns (a column is a work.Type's index, which is
// also its index in WorkColumns, WorkHelp and WorkColSkill) to their command
// and pawn flag. Where a column is drawn is workSlot's business.
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
	11: {command.WorkHandle, func(p game.PawnView) bool { return false }},   // priorities carry it
	12: {command.WorkWarden, func(p game.PawnView) bool { return false }},   // ENG-003: off by default
	13: {command.WorkPlantCut, func(p game.PawnView) bool { return false }}, // FOOD-2: priorities carry it
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

// PrevPriority is NextPriority's inverse: off -> 4 -> 3 -> 2 -> 1 -> off.
func PrevPriority(v int) int {
	if v <= 0 {
		return 4
	}
	return v - 1
}

// NextPriority cycles a cell: off -> 1 -> 2 -> 3 -> 4 -> off.
func NextPriority(v int) int {
	if v >= 4 {
		return 0
	}
	return v + 1
}

// The Work window's measures at UI step 1 (UIS-061): row pitch 30, cell
// stride 39, name column 240, first row 97 below the top, 60 under the last
// row. The cell box is hudlayout.WorkBox (25) square, left-aligned in its
// 39 px slot; the slot's 14 px gutter right of it holds the passion glyph.
const (
	workRowH   = 30
	WorkNameW  = 240
	workCellW  = 39
	workCellH  = hudlayout.WorkBox
	workGlyph  = 10 // the passion glyph's size
	workTop    = 97
	workBottom = 60
	workHeadY  = 60 // the header buttons' top; two staggered label rows
	workHeadH  = 37
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
	13: {skill.Plants, true},
}

// WorkHelp is the header hover card of each work column.
var WorkHelp = [...]string{
	0:  "Firefighting\nBeat out fires inside the Home area and your allowed area. No skill.",
	1:  "Doctor\nTend the wounded and the sick, and carry the downed to bed. Skill: Medicine.",
	2:  "Growing\nSow and harvest crops in growing zones, and fell zone trees. Skill: Plants.",
	3:  "Cooking\nCook meals at a stove or campfire and butcher animals. Skill: Cooking.",
	4:  "Building\nBuild, deconstruct, repair, roof and smooth, and deliver materials to sites. Skill: Construction.",
	5:  "Hunting\nHunt wild animals with a rifle, or a melee weapon. Skill: Shooting (Melee).",
	6:  "Hauling\nCarry items to stockpiles and frames, fill hoppers, refuel. No skill.",
	7:  "Cleaning\nSweep dirt and filth off the Home area. No skill.",
	8:  "Research\nStudy at a research bench. Skill: Intellectual.",
	9:  "Mining\nDig out rock and ore. Skill: Mining.",
	10: "Crafting\nWork at benches: workbench, tailoring, stonecutting. Skill: Crafting.",
	11: "Handling\nTame, milk, shear and collect from animals. Skill: Animals.",
	12: "Warden\nChat with prisoners, recruit them and bring them food. Off unless you turn it on. Skill: Social.",
	13: "Plant cutting\nCut marked plants and grass, harvest wild plants, fell marked trees, clear blight. Skill: Plants.",
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
		why = w.sh.T("Incapable")
	}
	return why, true
}

// workSlot is the screen position (0 = leftmost) of column col, a work.Type's
// index: its place in work.Order, the order a colonist breaks ties in, so
// left to right on screen is what the colonist does first. A type missing
// from work.Order keeps its own index (TestWorkSlotsCoverEveryType fails).
func workSlot(col int) int {
	for i, t := range work.Order {
		if int(t) == col {
			return i
		}
	}
	return col
}

// WorkCellRect is the slot (the hit rectangle) of colonist row's cell in
// column col (a work.Type's index), placed at the column's workSlot: the
// box on its left and the passion gutter on its right.
func WorkCellRect(r uikit.Button, row, col int) gfx.Rect {
	hx := r.X + hudlayout.WorkPadL + WorkNameW
	y := r.Y + workTop + float32(row*workRowH)
	return gfx.Rect{X: hx + float32(workSlot(col)*workCellW), Y: y, W: workCellW, H: workCellH}
}

// workHeadRect is the header button of column col: one slot wide, centred on
// the box so the column name (drawn centred on the box) stays inside it.
func workHeadRect(r uikit.Button, col int) gfx.Rect {
	c := WorkCellRect(r, 0, col)
	return gfx.Rect{X: c.X - (workCellW-hudlayout.WorkBox)/2, Y: r.Y + workHeadY, W: workCellW, H: workHeadH}
}

// workBoxRect is the box inside a slot.
func workBoxRect(slot gfx.Rect) gfx.Rect {
	return gfx.Rect{X: slot.X, Y: slot.Y, W: hudlayout.WorkBox, H: workCellH}
}

// workGlyphRect is the passion glyph's square: centred in the gutter right
// of the box, vertically centred, inside the slot.
func workGlyphRect(slot gfx.Rect) gfx.Rect {
	gx := slot.X + hudlayout.WorkBox + (workCellW-hudlayout.WorkBox-workGlyph)/2
	return gfx.Rect{X: gx, Y: slot.Y + (slot.H-workGlyph)/2, W: workGlyph, H: workGlyph}
}

// workTint is the box fill of a skill level 0 to 20: dark at 0, the accent
// at 20, brighter with every level.
func workTint(level int) color.RGBA {
	t := float64(max(0, min(20, level))) / 20
	return uikit.LerpRGBA(uikit.Btn, uikit.Accent, 0.1+0.6*t)
}

// workNumColor is the priority number's colour: the more urgent, the
// brighter (1 first, 4 last).
func workNumColor(pr int) color.RGBA {
	return uikit.LerpRGBA(uikit.Heading, uikit.TextDim, float64(max(1, min(4, pr))-1)/3)
}

// workRowShown reports the row fits the panel.
func workRowShown(r uikit.Button, row int) bool {
	return int(r.Y)+workTop+row*workRowH+workRowH <= int(r.Y+r.H)-hudlayout.WindowPad
}

// hint is the header's one-line legend.
func (w Work) hint() string {
	return w.sh.T("1 = do first ... 4 = do last, - = never; ties: left first; click a cell to change")
}

// help is the header hover card of work column col, in the player's
// language (WorkHelp holds the English for tests and captures).
func (w Work) help(col int) string {
	switch col {
	case 0:
		return w.sh.T("Firefighting\nBeat out fires inside the Home area and your allowed area. No skill.")
	case 1:
		return w.sh.T("Doctor\nTend the wounded and the sick, and carry the downed to bed. Skill: Medicine.")
	case 2:
		return w.sh.T("Growing\nSow and harvest crops in growing zones, and fell zone trees. Skill: Plants.")
	case 3:
		return w.sh.T("Cooking\nCook meals at a stove or campfire and butcher animals. Skill: Cooking.")
	case 4:
		return w.sh.T("Building\nBuild, deconstruct, repair, roof and smooth, and deliver materials to sites. Skill: Construction.")
	case 5:
		return w.sh.T("Hunting\nHunt wild animals with a rifle, or a melee weapon. Skill: Shooting (Melee).")
	case 6:
		return w.sh.T("Hauling\nCarry items to stockpiles and frames, fill hoppers, refuel. No skill.")
	case 7:
		return w.sh.T("Cleaning\nSweep dirt and filth off the Home area. No skill.")
	case 8:
		return w.sh.T("Research\nStudy at a research bench. Skill: Intellectual.")
	case 9:
		return w.sh.T("Mining\nDig out rock and ore. Skill: Mining.")
	case 10:
		return w.sh.T("Crafting\nWork at benches: workbench, tailoring, stonecutting. Skill: Crafting.")
	case 11:
		return w.sh.T("Handling\nTame, milk, shear and collect from animals. Skill: Animals.")
	case 12:
		return w.sh.T("Warden\nChat with prisoners, recruit them and bring them food. Off unless you turn it on. Skill: Social.")
	case 13:
		return w.sh.T("Plant cutting\nCut marked plants and grass, harvest wild plants, fell marked trees, clear blight. Skill: Plants.")
	}
	return ""
}

// colName is the header abbreviation of work column col (WorkColumns holds
// the English).
func (w Work) colName(col int) string {
	switch col {
	case 0:
		return w.sh.T("FF")
	case 1:
		return w.sh.T("Doc")
	case 2:
		return w.sh.T("Grow")
	case 3:
		return w.sh.T("Cook")
	case 4:
		return w.sh.T("Bld")
	case 5:
		return w.sh.T("Hunt")
	case 6:
		return w.sh.T("Haul")
	case 7:
		return w.sh.T("Cln")
	case 8:
		return w.sh.T("Rsh")
	case 9:
		return w.sh.T("Mine")
	case 10:
		return w.sh.T("Crft")
	case 11:
		return w.sh.T("Hndl")
	case 12:
		return w.sh.T("Ward")
	case 13:
		return w.sh.T("Cut")
	}
	return ""
}

// workCellTip is the hover card of a live cell: priority, skill, passion.
func (w Work) cellTip(p game.PawnView, col int) string {
	name := w.help(col)
	name, _, _ = strings.Cut(name, "\n")
	tip := w.sh.PawnLabel(p) + ": " + name + "\n"
	if pr := WorkPriority(p, col); pr > 0 {
		tip += lang.Tf(w.sh, "Priority %d (1 first, 4 last). ", pr)
	} else {
		tip += w.sh.T("Never. ")
	}
	if col < len(WorkColSkill) && WorkColSkill[col].OK {
		sk := WorkColSkill[col].Skill
		tip += lang.Tf(w.sh, "%s level %d, %s.", sk.Label(), p.SkillLv[sk], passionName(w.sh, int(p.Passion[sk])))
	}
	return tip
}

// passionName is "no passion", "minor passion" or "major passion".
func passionName(tr lang.Tr, p int) string {
	switch p {
	case 1:
		return tr.T("minor passion")
	case 2:
		return tr.T("major passion")
	}
	return tr.T("no passion")
}

// Tips declares the Work tab's hover cards: each column header names
// its work and skill, each incapable cell says why.
func (w Work) Tips(sw, sh int) {
	if !w.Open || (w.Mode != "work" && w.Mode != "") {
		return
	}
	r := w.Rect()
	for _, t := range work.Order {
		col := int(t)
		cr := WorkCellRect(r, 0, col)
		w.kit().Tip("work.hdr."+uikit.Num(col), gfx.Rect{X: cr.X, Y: r.Y + workHeadY, W: cr.W, H: workHeadH}, w.help(col))
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
	pad := float32(hudlayout.WindowPad)
	hint := pad + gfx.TextW(w.hint(), gfx.Small) + pad
	return max(hudlayout.WorkPadL+WorkNameW+float32(len(WorkColumns)*workCellW)+hudlayout.WorkPadL, hint), float32(max(160, workTop+n*workRowH+workBottom))
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

// buttons are the Work window's priority cells (an incapable one disabled)
// and, after them, one header button per column (CTL-014).
func (w Work) buttons() []uikit.Button {
	out := w.cellButtons()
	r := w.Rect()
	for _, t := range work.Order {
		col := int(t)
		if _, ok := WorkTypeForCol(col); !ok {
			continue
		}
		hr := workHeadRect(r, col)
		out = append(out, uikit.Button{ID: ids.PrefixWorkHead + strconv.Itoa(col), X: hr.X, Y: hr.Y, W: hr.W, H: hr.H, Quiet: true})
	}
	return out
}

// cellButtons are the priority cells alone (an incapable one disabled).
func (w Work) cellButtons() []uikit.Button {
	r := w.Rect()
	pawns := w.sh.Pawns()
	out := make([]uikit.Button, 0)
	for row, p := range pawns {
		if !workRowShown(r, row) {
			break
		}
		for col := range WorkColumns {
			if _, ok := WorkTypeForCol(col); !ok {
				continue
			}
			cr := WorkCellRect(r, row, col)
			b := uikit.Button{
				ID:    ids.PrefixWorkCell + strconv.FormatUint(uint64(p.ID), 10) + "." + strconv.Itoa(col),
				Label: "",
				X:     cr.X,
				Y:     cr.Y,
				W:     cr.W,
				H:     cr.H,
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

// draw draws the Work window: the panel, a title line with the legend, the
// column names on two staggered rows, then a row per colonist.
func (w Work) draw(screen *ebiten.Image, sw, sh int) {
	r := w.Rect()
	pad := float64(hudlayout.WindowPad)
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	title := w.sh.T("Work priorities")
	gfx.Text(screen, title, float64(r.X)+pad, float64(r.Y)+pad, gfx.WithColor(gfx.Label, uikit.Heading))
	hintX := float64(r.X) + pad + float64(gfx.TextW(title, gfx.Label)) + 16
	gfx.Text(screen, gfx.Ellipsize(w.hint(), gfx.Small, float64(r.X+r.W)-pad-hintX),
		hintX, gfx.CenterTop(float64(r.Y)+pad, gfx.LineH(gfx.Label), gfx.Small), gfx.WithColor(gfx.Small, uikit.TextDim))
	for i := range WorkColumns {
		slot := workSlot(i)
		st := gfx.TextStyle{Size: gfx.Small.Size, Color: uikit.TextDim, Align: gfx.AlignCenter}
		band := float64(workHeadH) / 2 // staggered rows, each centred in half the header
		y := gfx.CenterTop(float64(r.Y)+workHeadY+float64(slot%2)*band, band, st)
		gfx.Text(screen, w.colName(i), float64(workBoxRect(WorkCellRect(r, 0, i)).X)+float64(hudlayout.WorkBox)/2, y, st)
	}
	pawns := w.sh.Pawns()
	for row, p := range pawns {
		if !workRowShown(w.Rect(), row) {
			break
		}
		y := int(r.Y) + workTop + row*workRowH
		gfx.Text(screen, gfx.Ellipsize(w.sh.PawnLabel(p), gfx.Body, WorkNameW-8), float64(r.X)+pad, gfx.CenterTop(float64(y), workCellH, gfx.Body), gfx.Body)
	}
	mx, my := w.sh.Cursor()
	for row, p := range pawns {
		if !workRowShown(w.Rect(), row) {
			break
		}
		for col := range WorkColumns {
			if _, live := WorkLive[col]; live {
				w.drawCell(screen, p, WorkCellRect(w.Rect(), row, col), col, mx, my)
			}
		}
	}
}

// drawCell draws one cell: the box tinted by the colonist's skill level, the
// priority number centred in it (its colour is the urgency), the passion
// glyph in the gutter; incapable work is hatched.
func (w Work) drawCell(screen *ebiten.Image, p game.PawnView, slot gfx.Rect, col, mx, my int) {
	box := workBoxRect(slot)
	if p.Incapable.Enabled(work.Type(col)) {
		drawHatch(screen, box)
		return
	}
	level, passion := 0, 0
	if col < len(WorkColSkill) && WorkColSkill[col].OK {
		sk := WorkColSkill[col].Skill
		level, passion = int(p.SkillLv[sk]), int(p.Passion[sk])
	}
	fill, border := workTint(level), uikit.BtnBorder
	if slot.Hit(mx, my) {
		border = uikit.Accent
	}
	uikit.FillRect(screen, box, fill)
	uikit.StrokeRect(screen, box, border)
	st := gfx.TextStyle{Size: gfx.Body.Size, Align: gfx.AlignCenter, Color: uikit.TextDim}
	if pr := WorkPriority(p, col); pr > 0 {
		st.Color = workNumColor(pr)
	}
	gfx.Text(screen, WorkCellValue(p, col), float64(box.X+box.W/2), gfx.CenterTop(float64(box.Y), float64(box.H), st), st)
	if passion > 0 {
		ls := gfx.Small
		ls.Color = uikit.TextDim
		g := workGlyphRect(slot)
		w.sh.DrawPassion(screen, passion, float64(g.X), float64(g.Y), workGlyph, ls, gfx.CenterTop(float64(g.Y), workGlyph, ls))
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

// parseWorkCell splits "work.cell.<pawn>.<col>".
func parseWorkCell(id string) (pawn uint64, col int, ok bool) {
	a, b, found := strings.Cut(strings.TrimPrefix(id, ids.PrefixWorkCell), ".")
	if !found {
		return 0, 0, false
	}
	pn, err1 := strconv.ParseUint(a, 10, 64)
	c, err2 := strconv.Atoi(b)
	return pn, c, err1 == nil && err2 == nil
}

// pawnByID is the colonist's read model.
func (w Work) pawnByID(id uint64) (game.PawnView, bool) {
	for _, p := range w.sh.Pawns() {
		if uint64(p.ID) == id {
			return p, true
		}
	}
	return game.PawnView{}, false
}

// set sends the priority of p's column col unless it already has it or the
// colonist can never do that work.
func (w Work) set(p game.PawnView, col, value int) {
	wt, ok := WorkTypeForCol(col)
	if !ok || p.Incapable.Enabled(work.Type(col)) || WorkPriority(p, col) == value {
		return
	}
	w.sh.Enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: uint64(p.ID), Work: wt, Count: value})
}

// Click is a left click on cell id: the next priority, which a drag then
// paints over the cells it passes (CTL-015).
func (w Work) Click(id string) {
	pn, col, ok := parseWorkCell(id)
	if !ok || !w.live() {
		return
	}
	p, ok := w.pawnByID(pn)
	if !ok || p.Incapable.Enabled(work.Type(col)) {
		return
	}
	next := NextPriority(WorkPriority(p, col))
	w.set(p, col, next)
	w.paint.Arm(id, next)
}

// RightClick is a right click at (mx, my): a cell takes the previous
// priority and arms the drag with it. It reports whether the click was on a
// cell; an incapable one is consumed with no effect (CTL-013).
func (w Work) RightClick(mx, my int) bool {
	if !w.Open || (w.Mode != "work" && w.Mode != "") || !w.live() {
		return false
	}
	b, ok := uikit.HitButton(w.cellButtons(), mx, my)
	if !ok {
		if h, on := uikit.HitButton(w.Buttons(), mx, my); on {
			return w.HeadClick(h.ID, w.kit().Host().Frame().Shift(), true)
		}
		return false
	}
	if b.Disabled {
		return true
	}
	pn, col, ok := parseWorkCell(b.ID)
	if !ok {
		return true
	}
	if p, ok := w.pawnByID(pn); ok {
		prev := PrevPriority(WorkPriority(p, col))
		w.set(p, col, prev)
		w.paint.Arm(b.ID, prev)
	}
	return true
}

// HeadClick is a click on a column header, id ids.PrefixWorkHead+<col>. With
// shift, the first capable colonist's next priority (previous when inverse,
// the right button) goes to every capable colonist; without it nothing
// changes (CTL-014). It reports whether id was a header.
func (w Work) HeadClick(id string, shift, inverse bool) bool {
	if !strings.HasPrefix(id, ids.PrefixWorkHead) {
		return false
	}
	col, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixWorkHead))
	if err != nil || !shift || !w.live() {
		return err == nil
	}
	target, have := 0, false
	for _, p := range w.sh.Pawns() {
		if p.Incapable.Enabled(work.Type(col)) {
			continue
		}
		if !have {
			target, have = NextPriority(WorkPriority(p, col)), true
			if inverse {
				target = PrevPriority(WorkPriority(p, col))
			}
		}
		w.set(p, col, target)
	}
	return true
}

// Drag keeps painting the first click's priority over the cells the held
// mouse passes.
func (w Work) Drag() {
	if !w.Open || (w.Mode != "work" && w.Mode != "") || !w.live() {
		return
	}
	down := w.kit().Host().Frame().Down(inputframe.Left) || w.kit().Host().Frame().Down(inputframe.Right)
	cell := ""
	if w.paint.Armed {
		mx, my := w.sh.Cursor()
		if b, ok := uikit.HitButton(w.cellButtons(), mx, my); ok && !b.Disabled {
			cell = b.ID
		}
	}
	id, ok := w.paint.Entered(down, cell)
	if !ok || !w.paint.Armed {
		return
	}
	if pn, col, ok := parseWorkCell(id); ok {
		if p, ok := w.pawnByID(pn); ok {
			w.set(p, col, w.paint.Value)
		}
	}
}
