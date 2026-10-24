package ebiten

import (
	"fmt"
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/skill"
)

// uiStripe is the faint stripe of every other table row (non-premultiplied,
// so the 16 % alpha scales the colour).
var uiStripe = color.NRGBA{R: uiPanelBorder.R, G: uiPanelBorder.G, B: uiPanelBorder.B, A: 0x28}

// The pawn inspector's Bio tab (NG-1 §2.12): the colonist's identity from
// the read model Simulation.Bio, read-only: portrait, names, sex/age/build,
// backstories and traits with hover cards, what they can't do, the eleven
// skills with level, passion, experience bar and "This week: +N", and
// conditions. The tab's content is declared and painted with the widget
// kit every Update (colony_ui.go calls bioUI); inspector.go only draws the
// panel and the legacy tab row around it.

const (
	bioPortW    = 84
	bioPortH    = 108
	bioHdrH     = 20 // the skill table's header row
	bioRowMax   = 18
	bioRowMin   = 14
	bioActionH  = 68 // the Draft / Clear orders row and the tab row under the content
	bioCloseGap = 26 // room kept for the inspector's X button
)

// hasBio reports the colonist carries an identity (colonies started from a
// setup do; the legacy start's colonists don't).
func hasBio(p game.PawnView) bool { return p.Sex != "" }

// bioTabOn reports the Bio tab is the one showing.
func (a *App) bioTabOn() bool {
	p, ok := a.selectedPawn()
	return ok && a.inspectTab == "bio" && hasBio(p)
}

// bioUI declares the Bio tab's tab-row click and, while it is open, its
// content.
func (a *App) bioUI() {
	if a.renderer == nil || !a.renderer.HasSelect() || a.workOpen || a.archOpen {
		return
	}
	p, ok := a.selectedPawn()
	if !ok {
		return
	}
	u := a.uiSt()
	for _, b := range a.inspectorButtons() {
		if b.id == "inspector.bio" && !b.disabled {
			u.add(uiWidget{id: b.id, label: b.label, r: rect{b.x, b.y, b.w, b.h}, click: func() { a.inspectTab = "bio" }})
		}
	}
	if a.bioTabOn() {
		a.bioTab(p)
	}
}

// bioName is `First "Nick" Last`; the nickname is left out when it repeats
// a name or is empty.
func bioName(b game.BioView) string {
	nick := ""
	if b.Nick != "" && b.Nick != b.First && b.Nick != b.Last {
		nick = ` "` + b.Nick + `" `
	} else {
		nick = " "
	}
	return strings.TrimSpace(b.First + nick + b.Last)
}

// bioInfoLine is "Female, age 31, Average build".
func bioInfoLine(b game.BioView) string {
	return fmt.Sprintf("%s, age %d, %s build", b.Sex, b.AgeYears, strings.ToLower(b.Build))
}

// bioIncapable is the short "Violence, Hauling" text of the incapable list
// and the full reasons for its hover card.
func bioIncapable(b game.BioView) (short, full string) {
	if len(b.Incapable) == 0 {
		return "None", ""
	}
	names := make([]string, len(b.Incapable))
	for i, s := range b.Incapable {
		names[i] = s
		if k := strings.Index(s, " ("); k > 0 {
			names[i] = s[:k]
		}
	}
	return strings.Join(names, ", "), strings.Join(b.Incapable, "\n")
}

// weekText is a skill row's "This week" cell: "+N" or a dot.
func weekText(xp int) string {
	if xp <= 0 {
		return "·"
	}
	return fmt.Sprintf("+%d", xp)
}

// bioSkillTip is a skill row's hover card: the character screen's card
// plus this week's learning and progress to the next level.
func bioSkillTip(s game.SkillView, week int) string {
	t := skillTip(s)
	if s.Incapable {
		return t
	}
	t += fmt.Sprintf(" %d%% of the way to the next level.", s.XPPct)
	if week > 0 {
		t += fmt.Sprintf(" Learned %d experience in the last 7 days.", week)
	} else {
		t += " Nothing learned in the last 7 days."
	}
	return t
}

// bioTab declares the Bio tab of colonist p inside the inspector panel.
func (a *App) bioTab(p game.PawnView) {
	r := inspectorRect()
	in := rect{r.x + 12, r.y + 10, r.w - 24, r.h - 10 - bioActionH}
	b, ok := a.renderer.game.Bio(p.ID)
	if !ok {
		a.uiLabel(in.X, in.Y, "No biography for this colonist.", dimText())
		return
	}
	week := a.renderer.game.LearnedThisWeek(p.ID)
	u := a.uiSt()

	// Portrait and names.
	pr := rect{in.X, in.Y, bioPortW, bioPortH}
	name := bioName(b)
	u.queue(func(dst *ebiten.Image) {
		fillRect(dst, pr, uiField)
		strokeRect(dst, pr, uiPanelBorder)
		if !drawPortrait(dst, a.art, b.Look, p.Disfigured, float64(pr.X), float64(pr.Y), float64(pr.W), float64(pr.H)) {
			drawSilhouette(dst, pr, name)
		}
	})
	tx := pr.right() + 12
	tw := in.right() - tx - bioCloseGap
	nm := tsLarge
	nm.Color = uiHeading
	y := in.Y
	for i, l := range wrapPx(name, nm, float64(tw)) {
		if i == 2 {
			break
		}
		a.uiLabel(tx, y, l, nm)
		y += float32(lineH(nm))
	}
	a.uiLabel(tx, y+2, fitText(bioInfoLine(b), dimText(), in.right()-tx), dimText())
	y += 28
	a.backstoryLine(tx, y, in.right()-tx, "Childhood", b.Childhood, b.ChildhoodDesc, "bio.child")
	y += 20
	a.backstoryLine(tx, y, in.right()-tx, "Adulthood", b.Adulthood, b.AdulthoodDesc, "bio.adult")

	// Traits.
	y = max(pr.bottom()+10, y+26)
	y = a.traitChips(in.X, y, in.W, game.CandidateView{BioView: b})

	// Incapable of.
	short, full := bioIncapable(b)
	col := uiTextDim
	if full != "" {
		col = uiDanger
	}
	iy := y
	a.uiLabel(in.X, iy, "Incapable of:", tsLabel)
	u.queue(func(dst *ebiten.Image) {
		st := tsBody
		st.Color = col
		drawText(dst, fitText(short, st, in.W-104), float64(in.X+104), float64(iy), st)
	})
	if full != "" {
		a.uiTip("bio.incap", rect{in.X, iy, in.W, 18}, "Incapable of\n"+full)
	}
	y += 26

	// Relations and conditions at the bottom, skills above them.
	rel := a.bioRelations(p)
	relH := float32(0)
	if len(rel) > 0 {
		relH = 16 + float32(len(rel)-1)*14
	}
	condH := float32(18)
	if len(b.Conditions) > 1 {
		condH += float32(len(b.Conditions)-1) * 14
	}
	top := y
	avail := in.bottom() - condH - relH - top - bioHdrH
	rowH := min(float32(bioRowMax), max(float32(bioRowMin), avail/float32(max(len(b.Skills), 1))))
	a.bioSkills(rect{in.X, top, in.W, bioHdrH + rowH*float32(len(b.Skills))}, b, week, rowH)

	if len(rel) > 0 {
		ry := in.bottom() - condH - relH + 2
		a.uiLabel(in.X, ry, "Relations:", tsLabel)
		rs := tsSmall
		rs.Color = uiTextDim
		for i, ln := range rel {
			a.uiLabel(in.X+74, ry+1+float32(i)*14, fitText(ln, rs, in.W-74), rs)
		}
		a.uiTip("bio.relations", rect{in.X, ry, in.W, relH}, "Relations\n"+strings.Join(rel, "\n")+"\nSee the Social tab for opinions.")
	}
	cy := in.bottom() - condH + 2
	a.uiLabel(in.X, cy, "Health:", tsLabel)
	text, st := "No conditions", tsSmall
	st.Color = uiTextDim
	if len(b.Conditions) > 0 {
		text, st.Color = strings.Join(b.Conditions, "\n"), uiWarning
	}
	u.queue(func(dst *ebiten.Image) { drawText(dst, text, float64(in.X+60), float64(cy)+1, st) })
}

// bioSkills draws the skill table of the Bio tab: icon and name, passion,
// level, experience bar, and this week's learning.
func (a *App) bioSkills(r rect, b game.BioView, week [skill.NumSkills]int, rowH float32) {
	u := a.uiSt()
	for i, s := range b.Skills {
		row := rect{r.X, r.Y + bioHdrH + float32(i)*rowH, r.W, rowH}
		a.uiTip(fmt.Sprintf("bio.skill.%d", i), row, bioSkillTip(s, week[i]))
	}
	const (
		passX = 108
		lvlX  = 150
		barX  = 164
	)
	barW := r.W - barX - 52
	u.queue(func(dst *ebiten.Image) {
		hs := tsSmall
		hs.Color = uiHeading
		hr := rect{r.X, r.Y, r.W, bioHdrH}
		fillRect(dst, hr, uiBtn)
		strokeRect(dst, hr, uiBtnBorder)
		hy := float64(r.Y) + (bioHdrH-lineH(hs))/2
		drawText(dst, "Skill", float64(r.X+8), hy, hs)
		drawText(dst, "Passion", float64(r.X+passX-14), hy, hs)
		lh := hs
		lh.Align = alignRight
		drawText(dst, "Lvl", float64(r.X+lvlX+10), hy, lh)
		drawText(dst, "Experience", float64(r.X+barX), hy, hs)
		drawText(dst, "This week", float64(r.right()-6), hy, lh)
		for i, s := range b.Skills {
			row := rect{r.X, r.Y + bioHdrH + float32(i)*rowH, r.W, rowH}
			if i%2 == 1 {
				fillRect(dst, row, uiStripe)
			}
			st := tsSmall
			if s.Incapable {
				st.Color = uiTextDisabled
			}
			ty := float64(row.Y) + (float64(row.H)-lineH(st))/2
			if img := a.art.image("ui_icon/skill_"+s.Key, noTint); img != nil {
				drawIcon(dst, img, float64(row.X+6), float64(row.Y)+(float64(row.H)-12)/2, 12)
			}
			drawText(dst, s.Label, float64(row.X+22), ty, st)
			if !s.Incapable && s.Passion > 0 {
				a.drawPassion(dst, s.Passion, float64(row.X+passX), float64(row.Y)+(float64(row.H)-12)/2, 12, st, ty)
			}
			lt := st
			lt.Align = alignRight
			drawText(dst, skillLevelText(s), float64(row.X+lvlX+10), ty, lt)
			if s.Incapable {
				drawText(dst, "incapable", float64(row.X+barX), ty, st)
				continue
			}
			bar := rect{row.X + barX, row.Y + (row.H-6)/2, barW, 6}
			fillRect(dst, bar, uiSliderTrack)
			fillRect(dst, rect{bar.X, bar.Y, bar.W * float32(s.XPPct) / 100, bar.H}, uiSliderFill)
			strokeRect(dst, bar, uiPanelBorder)
			wt := st
			wt.Align = alignRight
			if week[i] <= 0 {
				wt.Color = uiTextDim
			} else {
				wt.Color = uiGood
			}
			drawText(dst, weekText(week[i]), float64(row.right()-6), ty, wt)
		}
	})
}

// drawPassion draws the passion glyph of level p (1 minor, 2 major) at
// (x, y), or stars while the icon is missing from the art.
func (a *App) drawPassion(dst *ebiten.Image, p int, x, y, size float64, st textStyle, ty float64) {
	key := "ui_icon/passion_minor"
	if p >= 2 {
		key = "ui_icon/passion_major"
	}
	if img := a.art.image(key, noTint); img != nil {
		drawIcon(dst, img, x, y, size)
		return
	}
	drawText(dst, strings.Repeat("*", min(p, 2)), x, ty, st)
}

// bioRelationMax is how many relations the Bio tab summarises.
const bioRelationMax = 3

// bioRelationLines is the Bio tab's relations summary: the first rows of
// the Social read model as "Wife: Bo".
func bioRelationLines(rows []game.RelationRowView) []string {
	var out []string
	for _, r := range rows {
		if len(out) == bioRelationMax {
			break
		}
		if r.Label == "" {
			out = append(out, r.Name)
		} else {
			out = append(out, r.Label+": "+r.Name)
		}
	}
	return out
}

// bioRelations reads colonist p's relations summary.
func (a *App) bioRelations(p game.PawnView) []string {
	s, ok := a.renderer.game.Social(p.ID)
	if !ok {
		return nil
	}
	return bioRelationLines(s.Relations)
}
