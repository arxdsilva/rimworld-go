// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package inspector

import (
	"fmt"
	"image/color"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// The pawn inspector's Bio tab (NG-1 §2.12): the colonist's identity from
// the read model Simulation.Bio, read-only: portrait, names, sex/age/build,
// backstories and traits with hover cards, what they can't do, the eleven
// skills with level, passion, experience bar and "This week: +N", and
// conditions. The tab's content is declared and painted with the widget
// kit every Update, into the expanded panel the colonist pane opens above
// itself (cmds_pawn.go's Bio paneTab calls bioTab with that rectangle).

const (
	bioPortW  = 84
	bioPortH  = 108
	bioHdrH   = 20 // the skill table's header row
	bioRowMax = 18
	bioRowMin = 14
	// bioScrollRows is the fewest skill rows the table keeps in view (it
	// scrolls when it cannot show them all) before the relations give way.
	bioScrollRows = 6
	bioSkillsID   = "bio.skills"
)

// hasBio reports the colonist carries an identity (colonies started from a
// setup do; the legacy start's colonists don't).
func HasBio(p game.PawnView) bool { return p.Sex != "" }

// bioTabOn reports the Bio tab is the one showing.
func (a Tabs) BioTabOn() bool {
	p, ok := a.SelectedPawn()
	return ok && a.Tab == "bio" && HasBio(p)
}

// bioName is `First "Nick" Last`; the nickname is left out when it repeats
// a name or is empty.
func BioName(b game.BioView) string {
	nick := ""
	if b.Nick != "" && b.Nick != b.First && b.Nick != b.Last {
		nick = ` "` + b.Nick + `" `
	} else {
		nick = " "
	}
	return strings.TrimSpace(b.First + nick + b.Last)
}

// bioInfoLine is "Female, age 31, Average build".
func BioInfoLine(b game.BioView) string { return bioInfoLine(lang.English{}, b) }

func bioInfoLine(tr lang.Tr, b game.BioView) string {
	return lang.Tf(tr, "%s, age %d, %s build", b.Sex, b.AgeYears, strings.ToLower(b.Build))
}

// bioIncapable is the short "Violence, Hauling" text of the incapable list
// and the full reasons for its hover card.
func BioIncapable(b game.BioView) (short, full string) {
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
func WeekText(xp int) string {
	if xp <= 0 {
		return "·"
	}
	return fmt.Sprintf("+%d", xp)
}

// bioSkillTip is a skill row's hover card: the character screen's card
// plus this week's learning and progress to the next level.
func BioSkillTip(base string, s game.SkillView, week int) string {
	return bioSkillTip(lang.English{}, base, s, week)
}

func bioSkillTip(tr lang.Tr, base string, s game.SkillView, week int) string {
	t := base
	if s.Incapable {
		return t
	}
	t += lang.Tf(tr, " %d%% of the way to the next level.", s.XPPct)
	if week > 0 {
		t += lang.Tf(tr, " Learned %d experience in the last 7 days.", week)
	} else {
		t += tr.T(" Nothing learned in the last 7 days.")
	}
	return t
}

// bioTab declares the Bio tab of colonist id inside r, the expanded panel's
// inner rectangle.
// The skills table takes what the panel has left under the identity block
// and above the relations and conditions; when that is less than the full
// table it scrolls, and the relations give way once it would show fewer
// than bioScrollRows rows.
func (a Tabs) bioTab(id entity.EntityID, r gfx.Rect) {
	p, ok := a.pawnView(id)
	if !ok || a.sim() == nil {
		return
	}
	in := r
	b, ok := a.sim().Bio(p.ID)
	if !ok {
		a.kit().Label(in.X, in.Y, a.shell.T("No biography for this colonist."), uikit.DimText())
		return
	}
	card, _ := a.sim().Dossier(p.ID)
	week := card.LearnedWeek
	u := a.kit()

	// Portrait and names.
	pr := gfx.Rect{X: in.X, Y: in.Y, W: bioPortW, H: bioPortH}
	name := BioName(b)
	u.Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, pr, uikit.Field)
		uikit.StrokeRect(dst, pr, uikit.PanelBorder)
		a.shell.Portrait(dst, b.Look, p.Disfigured, pr, name)
	})
	tx := pr.Right() + 12
	tw := in.Right() - tx
	nm := gfx.Title // UIS-012: a heading
	nm.Color = uikit.Heading
	y := in.Y
	for i, l := range gfx.WrapPx(name, nm, float64(tw)) {
		if i == 2 {
			break
		}
		a.kit().Label(tx, y, l, nm)
		y += float32(gfx.LineH(nm))
	}
	a.kit().Label(tx, y+2, uikit.FitText(bioInfoLine(a.shell, b), uikit.DimText(), in.Right()-tx), uikit.DimText())
	y += 28
	a.shell.Backstory(tx, y, in.Right()-tx, "Childhood", b.Childhood, a.sources().withSource(b.ChildhoodDesc, "backstory", b.ChildhoodKey), "bio.child")
	y += 20
	a.shell.Backstory(tx, y, in.Right()-tx, "Adulthood", b.Adulthood, a.sources().withSource(b.AdulthoodDesc, "backstory", b.AdulthoodKey), "bio.adult")

	// Traits.
	y = max(pr.Bottom()+6, y+22)
	y = a.shell.Traits(in.X, y, in.W, game.CandidateView{BioView: a.sources().traitsWithSource(b)})

	// Incapable of.
	short, full := BioIncapable(b)
	col := uikit.TextDim
	if full != "" {
		col = uikit.Danger
	}
	iy := y
	a.kit().Label(in.X, iy, a.shell.T("Incapable of:"), gfx.Label)
	u.Queue(func(dst *ebiten.Image) {
		st := gfx.Body
		st.Color = col
		gfx.Text(dst, uikit.FitText(short, st, in.W-104), float64(in.X+104), float64(iy), st)
	})
	if full != "" {
		a.kit().Tip("bio.incap", gfx.Rect{X: in.X, Y: iy, W: in.W, H: 18}, a.shell.T("Incapable of\n")+full)
	}
	y += 22

	// Relations and conditions at the bottom, skills above them.
	rel := a.bioRelations(p)
	relH := bioRelationsH(len(rel))
	// the Health line: its label's glyphs, then a Small line per further condition
	_, labelH := gfx.Measure(a.shell.T("Health:"), gfx.Label)
	condH := float32(math.Ceil(labelH)) + 2
	if len(b.Conditions) > 1 {
		condH += float32(len(b.Conditions)-1) * gfx.LineStep(gfx.Small)
	}
	top := y
	if in.Bottom()-condH-relH-top < bioHdrH+bioScrollRows*bioRowMin {
		rel, relH = nil, 0 // too tight for a useful table: the relations go first (the Social tab has them)
	}
	r, rowH := bioSkillsRect(in, top, condH, relH, len(b.Skills))
	a.bioSkills(r, b, week, rowH)

	if len(rel) > 0 {
		ry := in.Bottom() - condH - relH + 2
		a.kit().Label(in.X, ry, a.shell.T("Relations:"), gfx.Label)
		rs := gfx.Small
		rs.Color = uikit.TextDim
		for i, ln := range rel {
			a.kit().Label(in.X+74, ry+1+float32(i)*14, uikit.FitText(ln, rs, in.W-74), rs)
		}
		a.kit().Tip("bio.relations", gfx.Rect{X: in.X, Y: ry, W: in.W, H: relH}, a.shell.T("Relations\n")+strings.Join(rel, "\n")+a.shell.T("\nSee the Social tab for opinions."))
	}
	cy := in.Bottom() - condH + 2
	a.kit().Label(in.X, cy, a.shell.T("Health:"), gfx.Label)
	text, st := a.shell.T("No conditions"), gfx.Small
	st.Color = uikit.TextDim
	if len(b.Conditions) > 0 {
		text, st.Color = strings.Join(b.Conditions, "\n"), uikit.Warning
	}
	u.Queue(func(dst *ebiten.Image) { gfx.Text(dst, text, float64(in.X+60), float64(cy)+1, st) })
}

// bioRelationsH is the height of the Relations block for n summary lines.
func bioRelationsH(n int) float32 {
	if n == 0 {
		return 0
	}
	return 21 + float32(n-1)*14 // the 14 px label's glyphs, then a 14 px pitch of Small lines
}

// bioSkillsRect is the skill table's rect in the tab's inner rect in, from
// top, above a Health block of condH and a Relations block of relH, and the
// height of one of its n rows. The table scrolls when its rows are taller
// than the room, and never reaches into the bottom blocks (QA-002).
func bioSkillsRect(in gfx.Rect, top, condH, relH float32, n int) (r gfx.Rect, rowH float32) {
	avail := in.Bottom() - condH - relH - top - bioHdrH
	rowH = min(float32(bioRowMax), max(float32(bioRowMin), avail/float32(max(n, 1))))
	tableH := min(bioHdrH+rowH*float32(n), max(in.Bottom()-condH-relH-top, bioHdrH))
	return gfx.Rect{X: in.X, Y: top, W: in.W, H: tableH}, rowH
}

// The skill table's columns (QA-002) and their x offsets in a row.
const (
	bioPassX = 108 // the passion glyph
	bioLvlX  = 150 // the level's right edge is bioLvlX+10
	bioBarX  = 164 // the experience bar
)

// bioColumns declares the skill table's columns and their header titles.
func bioColumns(sh interface{ T(string) string }) [5]uikit.TableCol {
	return [5]uikit.TableCol{
		{ID: "skill", Title: sh.T("Skill")}, {ID: "passion", Title: sh.T("Passion")},
		{ID: "level", Title: sh.T("Lvl")}, {ID: "xp", Title: sh.T("Experience")},
		{ID: "week", Title: sh.T("This week")},
	}
}

// bioHeaderRow is the header's row for cells: the table's x from the view the
// rows are laid in, so a scrollbar narrows both alike.
func bioHeaderRow(r, view gfx.Rect) gfx.Rect {
	return gfx.Rect{X: view.X, Y: r.Y, W: view.W, H: bioHdrH}
}

// bioSkillCells appends the five cells of a skill row (or the header) row, in
// bioColumns order: the name up to the passion header, the 12 px passion
// glyph, the level right-anchored at bioLvlX+10, the experience bar and
// This week from the bar's end to 6 px short of the row's right edge.
func bioSkillCells(dst []uikit.TableCell, row gfx.Rect) []uikit.TableCell {
	const lvlW, weekPad = 24, 6
	barR := row.X + bioBarX + max(0, row.W-bioBarX-52)
	at := func(id string, x, w float32) uikit.TableCell {
		return uikit.TableCell{Col: id, R: gfx.Rect{X: x, Y: row.Y, W: w, H: row.H}}
	}
	return append(dst,
		at("skill", row.X+22, bioPassX-14-22),
		at("passion", row.X+bioPassX, 12),
		at("level", row.X+bioLvlX+10-lvlW, lvlW),
		at("xp", row.X+bioBarX, barR-row.X-bioBarX),
		at("week", barR, row.Right()-weekPad-barR))
}

// bioSkills draws the skill table of the Bio tab in r: a header, then the
// rows (icon and name, passion, level, experience bar, this week's
// learning). When the rows are taller than the room under the header the
// body scrolls (wheel, thumb) and is clipped to it.
func (a Tabs) bioSkills(r gfx.Rect, b game.BioView, week [skill.NumSkills]int, rowH float32) {
	u := a.kit()
	view := gfx.Rect{X: r.X, Y: r.Y + bioHdrH, W: r.W, H: max(0, r.H-bioHdrH)}
	var off float32
	if contentH := rowH * float32(len(b.Skills)); contentH > view.H+0.5 {
		area := a.kit().ScrollArea(bioSkillsID, view, contentH, uikit.ScrollOpts{Step: rowH * 2})
		view, off = area.View, area.Off
	}
	rowAt := func(i int) gfx.Rect {
		return gfx.Rect{X: view.X, Y: view.Y + float32(i)*rowH - off, W: view.W, H: rowH}
	}
	for i := range b.Skills {
		if row := rowAt(i).Intersect(view); !row.Empty() {
			a.kit().Tip(fmt.Sprintf("bio.skill.%d", i), row, bioSkillTip(a.shell, a.skillTip(b.Skills[i]), b.Skills[i], week[i]))
		}
	}
	u.Queue(func(dst *ebiten.Image) {
		var buf [5]uikit.TableCell
		hs := gfx.Small
		hs.Color = uikit.Heading
		hr := gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: bioHdrH}
		hc := bioSkillCells(buf[:0], bioHeaderRow(r, view))
		uikit.FillRect(dst, hr, uikit.Btn)
		uikit.StrokeRect(dst, hr, uikit.BtnBorder)
		hy := gfx.CenterTop(float64(r.Y), bioHdrH, hs)
		cols := bioColumns(a.shell)
		lh := hs
		lh.Align = gfx.AlignRight
		gfx.Text(dst, cols[0].Title, float64(hc[0].R.X-14), hy, hs)
		gfx.Text(dst, cols[1].Title, float64(hc[1].R.X-14), hy, hs)
		gfx.Text(dst, cols[2].Title, float64(hc[2].R.Right()), hy, lh)
		gfx.Text(dst, cols[3].Title, float64(hc[3].R.X), hy, hs)
		gfx.Text(dst, cols[4].Title, float64(hc[4].R.Right()), hy, lh)
		body := gfx.ClipTo(dst, view)
		for i, s := range b.Skills {
			row := rowAt(i)
			if row.Bottom() < view.Y || row.Y > view.Bottom() {
				continue
			}
			a.bioRow(body, row, i, s, week[i])
		}
	})
}

// bioRow draws skill s's row i in row on body: the stripe, the icon and
// name, the passion, the level, then the experience bar and this week's
// learning, or "incapable" for an incapable skill. Every x is its cell's.
func (a Tabs) bioRow(body *ebiten.Image, row gfx.Rect, i int, s game.SkillView, week int) {
	uikit.RowStripe(body, row, i)
	var buf [5]uikit.TableCell
	rc := bioSkillCells(buf[:0], row)
	st := gfx.Small
	if s.Incapable {
		st.Color = uikit.TextDisabled
	}
	ty := gfx.CenterTop(float64(row.Y), float64(row.H), st)
	if img := a.env().Art.Image("ui_icon/skill_"+s.Key, color.NRGBA{}); img != nil {
		gfx.Icon(body, img, float64(row.X+6), float64(row.Y)+(float64(row.H)-12)/2, 12)
	}
	gfx.Text(body, s.Label, float64(rc[0].R.X), ty, st)
	if !s.Incapable && s.Passion > 0 {
		a.DrawPassion(body, s.Passion, float64(rc[1].R.X), float64(row.Y)+(float64(row.H)-12)/2, 12, st, ty)
	}
	lt := st
	lt.Align = gfx.AlignRight
	gfx.Text(body, a.skillLevel(s), float64(rc[2].R.Right()), ty, lt)
	if s.Incapable {
		gfx.Text(body, a.shell.T("incapable"), float64(rc[3].R.X), ty, st)
		return
	}
	bar := gfx.Rect{X: rc[3].R.X, Y: row.Y + (row.H-6)/2, W: rc[3].R.W, H: 6}
	uikit.FillRect(body, bar, uikit.SliderTrack)
	uikit.FillRect(body, gfx.Rect{X: bar.X, Y: bar.Y, W: bar.W * float32(s.XPPct) / 100, H: bar.H}, uikit.SliderFill)
	uikit.StrokeRect(body, bar, uikit.PanelBorder)
	wt := st
	wt.Align = gfx.AlignRight
	if week <= 0 {
		wt.Color = uikit.TextDim
	} else {
		wt.Color = uikit.Good
	}
	gfx.Text(body, WeekText(week), float64(rc[4].R.Right()), ty, wt)
}

// drawPassion draws the passion glyph of level p (1 minor, 2 major) at
// (x, y), or stars while the icon is missing from the art.
func (a Tabs) DrawPassion(dst *ebiten.Image, p int, x, y, size float64, st gfx.TextStyle, ty float64) {
	key := "ui_icon/passion_minor"
	if p >= 2 {
		key = "ui_icon/passion_major"
	}
	if img := a.env().Art.Image(key, color.NRGBA{}); img != nil {
		gfx.Icon(dst, img, x, y, size)
		return
	}
	gfx.Text(dst, strings.Repeat("*", min(p, 2)), x, ty, st)
}

// bioRelationMax is how many relations the Bio tab summarises.
const bioRelationMax = 3

// bioRelationLines is the Bio tab's relations summary: the first rows of
// the Social read model as "Wife: Bo".
func BioRelationLines(rows []game.RelationRowView) []string {
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
func (a Tabs) bioRelations(p game.PawnView) []string {
	s, ok := a.sim().Social(p.ID)
	if !ok {
		return nil
	}
	return BioRelationLines(s.Relations)
}

// skillLevel is a skill's level text; skillTip its hover card.
func (a Tabs) skillLevel(s game.SkillView) string { l, _ := a.shell.Skill(s); return l }
func (a Tabs) skillTip(s game.SkillView) string   { _, t := a.shell.Skill(s); return t }
