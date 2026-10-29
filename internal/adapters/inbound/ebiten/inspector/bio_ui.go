package inspector

import (
	"fmt"
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// uiStripe is the faint stripe of every other table row (non-premultiplied,
// so the 16 % alpha scales the colour).
var uiStripe = color.NRGBA{R: uikit.PanelBorder.R, G: uikit.PanelBorder.G, B: uikit.PanelBorder.B, A: 0x28}

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
func BioInfoLine(b game.BioView) string {
	return fmt.Sprintf("%s, age %d, %s build", b.Sex, b.AgeYears, strings.ToLower(b.Build))
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
	t := base
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
		a.kit().Label(in.X, in.Y, "No biography for this colonist.", uikit.DimText())
		return
	}
	week := a.sim().LearnedThisWeek(p.ID)
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
	nm := gfx.Large
	nm.Color = uikit.Heading
	y := in.Y
	for i, l := range gfx.WrapPx(name, nm, float64(tw)) {
		if i == 2 {
			break
		}
		a.kit().Label(tx, y, l, nm)
		y += float32(gfx.LineH(nm))
	}
	a.kit().Label(tx, y+2, uikit.FitText(BioInfoLine(b), uikit.DimText(), in.Right()-tx), uikit.DimText())
	y += 28
	a.shell.Backstory(tx, y, in.Right()-tx, "Childhood", b.Childhood, b.ChildhoodDesc, "bio.child")
	y += 20
	a.shell.Backstory(tx, y, in.Right()-tx, "Adulthood", b.Adulthood, b.AdulthoodDesc, "bio.adult")

	// Traits.
	y = max(pr.Bottom()+6, y+22)
	y = a.shell.Traits(in.X, y, in.W, game.CandidateView{BioView: b})

	// Incapable of.
	short, full := BioIncapable(b)
	col := uikit.TextDim
	if full != "" {
		col = uikit.Danger
	}
	iy := y
	a.kit().Label(in.X, iy, "Incapable of:", gfx.Label)
	u.Queue(func(dst *ebiten.Image) {
		st := gfx.Body
		st.Color = col
		gfx.Text(dst, uikit.FitText(short, st, in.W-104), float64(in.X+104), float64(iy), st)
	})
	if full != "" {
		a.kit().Tip("bio.incap", gfx.Rect{X: in.X, Y: iy, W: in.W, H: 18}, "Incapable of\n"+full)
	}
	y += 22

	// Relations and conditions at the bottom, skills above them.
	rel := a.bioRelations(p)
	relH := bioRelationsH(len(rel))
	condH := float32(18)
	if len(b.Conditions) > 1 {
		condH += float32(len(b.Conditions)-1) * 14
	}
	top := y
	if in.Bottom()-condH-relH-top < bioHdrH+bioScrollRows*bioRowMin {
		rel, relH = nil, 0 // too tight for a useful table: the relations go first (the Social tab has them)
	}
	avail := in.Bottom() - condH - relH - top - bioHdrH
	rowH := min(float32(bioRowMax), max(float32(bioRowMin), avail/float32(max(len(b.Skills), 1))))
	// the table scrolls when the room is less than its rows; never beyond the bottom blocks
	tableH := min(bioHdrH+rowH*float32(len(b.Skills)), max(in.Bottom()-condH-relH-top, bioHdrH))
	a.bioSkills(gfx.Rect{X: in.X, Y: top, W: in.W, H: tableH}, b, week, rowH)

	if len(rel) > 0 {
		ry := in.Bottom() - condH - relH + 2
		a.kit().Label(in.X, ry, "Relations:", gfx.Label)
		rs := gfx.Small
		rs.Color = uikit.TextDim
		for i, ln := range rel {
			a.kit().Label(in.X+74, ry+1+float32(i)*14, uikit.FitText(ln, rs, in.W-74), rs)
		}
		a.kit().Tip("bio.relations", gfx.Rect{X: in.X, Y: ry, W: in.W, H: relH}, "Relations\n"+strings.Join(rel, "\n")+"\nSee the Social tab for opinions.")
	}
	cy := in.Bottom() - condH + 2
	a.kit().Label(in.X, cy, "Health:", gfx.Label)
	text, st := "No conditions", gfx.Small
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
	return 16 + float32(n-1)*14
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
			a.kit().Tip(fmt.Sprintf("bio.skill.%d", i), row, BioSkillTip(a.skillTip(b.Skills[i]), b.Skills[i], week[i]))
		}
	}
	const (
		passX = 108
		lvlX  = 150
		barX  = 164
	)
	barW := view.W - barX - 52
	u.Queue(func(dst *ebiten.Image) {
		hs := gfx.Small
		hs.Color = uikit.Heading
		hr := gfx.Rect{X: r.X, Y: r.Y, W: r.W, H: bioHdrH}
		uikit.FillRect(dst, hr, uikit.Btn)
		uikit.StrokeRect(dst, hr, uikit.BtnBorder)
		hy := float64(r.Y) + (bioHdrH-gfx.LineH(hs))/2
		gfx.Text(dst, "Skill", float64(r.X+8), hy, hs)
		gfx.Text(dst, "Passion", float64(r.X+passX-14), hy, hs)
		lh := hs
		lh.Align = gfx.AlignRight
		gfx.Text(dst, "Lvl", float64(r.X+lvlX+10), hy, lh)
		gfx.Text(dst, "Experience", float64(r.X+barX), hy, hs)
		gfx.Text(dst, "This week", float64(view.Right()-6), hy, lh)
		body := gfx.ClipTo(dst, view)
		for i, s := range b.Skills {
			row := rowAt(i)
			if row.Bottom() < view.Y || row.Y > view.Bottom() {
				continue
			}
			if i%2 == 1 {
				uikit.FillRect(body, row, uiStripe)
			}
			st := gfx.Small
			if s.Incapable {
				st.Color = uikit.TextDisabled
			}
			ty := float64(row.Y) + (float64(row.H)-gfx.LineH(st))/2
			if img := a.env().Art.Image("ui_icon/skill_"+s.Key, color.NRGBA{}); img != nil {
				gfx.Icon(body, img, float64(row.X+6), float64(row.Y)+(float64(row.H)-12)/2, 12)
			}
			gfx.Text(body, s.Label, float64(row.X+22), ty, st)
			if !s.Incapable && s.Passion > 0 {
				a.DrawPassion(body, s.Passion, float64(row.X+passX), float64(row.Y)+(float64(row.H)-12)/2, 12, st, ty)
			}
			lt := st
			lt.Align = gfx.AlignRight
			gfx.Text(body, a.skillLevel(s), float64(row.X+lvlX+10), ty, lt)
			if s.Incapable {
				gfx.Text(body, "incapable", float64(row.X+barX), ty, st)
				continue
			}
			bar := gfx.Rect{X: row.X + barX, Y: row.Y + (row.H-6)/2, W: barW, H: 6}
			uikit.FillRect(body, bar, uikit.SliderTrack)
			uikit.FillRect(body, gfx.Rect{X: bar.X, Y: bar.Y, W: bar.W * float32(s.XPPct) / 100, H: bar.H}, uikit.SliderFill)
			uikit.StrokeRect(body, bar, uikit.PanelBorder)
			wt := st
			wt.Align = gfx.AlignRight
			if week[i] <= 0 {
				wt.Color = uikit.TextDim
			} else {
				wt.Color = uikit.Good
			}
			gfx.Text(body, WeekText(week[i]), float64(row.Right()-6), ty, wt)
		}
	})
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
