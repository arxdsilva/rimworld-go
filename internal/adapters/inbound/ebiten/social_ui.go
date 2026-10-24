package ebiten

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// The pawn inspector's Social tab (MAIN_UI_SCREENS §15, ENG-003 C4): the
// colonist's relation rows with both opinions, a hover card that breaks
// each opinion down, and the social log under them. Read-only: it reads
// Simulation.Social and has no editing controls (§16.9).

const (
	socialRowH      = 24
	socialLogRowH   = 18
	socialEmptyText = "No relationships yet"
	socialLogEmpty  = "Nothing has happened yet."
	socialRowsID    = "social.rows"
	socialLogID     = "social.log"
)

// opinionBand is the colour band of an opinion: 0 (-100..-60) to 4 (60..100).
func opinionBand(v int) int {
	switch {
	case v <= -60:
		return 0
	case v < -20:
		return 1
	case v <= 20:
		return 2
	case v < 60:
		return 3
	}
	return 4
}

// opinionColor tints an opinion from the palette's danger through warning
// and neutral to good.
func opinionColor(v int) color.RGBA {
	switch opinionBand(v) {
	case 0:
		return uiDanger
	case 1:
		return uiWarning
	case 2:
		return uiTextDim
	case 3:
		return uiGood
	}
	return lerpRGBA(uiGood, uiHeading, 0.45)
}

// statusTag is the short tag of a relation whose person is not around.
func statusTag(s string) string {
	if s == "caravan" {
		return "in caravan"
	}
	return s
}

// termLines turns opinion terms into "Label: +n" lines.
func termLines(ts []game.OpinionTermView) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = fmt.Sprintf("%s: %+d", t.Label, t.Value)
	}
	return out
}

// relationTip is a row's hover card: this colonist's opinion and its
// terms, the romance note, then the other person's opinion and its terms.
func relationTip(r game.RelationRowView) string {
	lines := []string{fmt.Sprintf("Opinion of %s: %d", r.Name, r.OpinionTo)}
	lines = append(lines, termLines(r.TermsTo)...)
	lines = append(lines, "")
	if r.RomanceNote != "" {
		lines = append(lines, r.RomanceNote, "")
	}
	lines = append(lines, fmt.Sprintf("%s's opinion of me: %d", r.Name, r.OpinionFrom))
	lines = append(lines, termLines(r.TermsFrom)...)
	return strings.Join(lines, "\n")
}

// logNewestFirst copies the log sorted newest first.
func logNewestFirst(log []game.SocialLogLineView) []game.SocialLogLineView {
	out := append([]game.SocialLogLineView(nil), log...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Tick > out[j].Tick })
	return out
}

// socialTab declares and paints the Social tab of colonist p in the
// inspector's content rectangle in, below the colonist's name.
func (a *App) socialTab(p game.PawnView, in rect) {
	s, ok := a.renderer.game.Social(p.ID)
	top := in.Y + float32(lineStep(tsLabel)) + 8
	if !ok {
		a.uiLabel(in.X, top, noDataYet, dimText())
		return
	}
	rowsH := (in.bottom() - top) * 0.55
	rows := rect{in.X, top, in.W, rowsH}
	a.socialRows(rows, s.Relations)

	ly := rows.bottom() + 8
	a.uiLabel(in.X, ly, "Social log", tsLabel)
	ly += float32(lineStep(tsLabel)) + 2
	a.socialLog(rect{in.X, ly, in.W, in.bottom() - ly}, s.Log)
}

// socialRows is the scrollable relation list.
func (a *App) socialRows(r rect, rels []game.RelationRowView) {
	u := a.uiSt()
	if len(rels) == 0 {
		a.uiLabel(r.X, r.Y, socialEmptyText, dimText())
		return
	}
	area := a.uiScrollArea(socialRowsID, r, float32(len(rels))*socialRowH, scrollOpts{Step: socialRowH * 2})
	type vis struct {
		i int
		y float32
	}
	var shown []vis
	for i := range rels {
		y := area.View.Y + float32(i)*socialRowH - area.Off
		if y+socialRowH < area.View.Y || y > area.View.bottom() {
			continue
		}
		shown = append(shown, vis{i, y})
	}
	u.queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, area.View)
		for _, v := range shown {
			paintRelationRow(sub, rect{area.View.X, v.y, area.View.W, socialRowH}, rels[v.i], v.i%2 == 1)
		}
	})
	for _, v := range shown {
		hit := rect{area.View.X, v.y, area.View.W, socialRowH}.intersect(area.View)
		a.uiSt().add(uiWidget{id: fmt.Sprintf("social.row.%d", v.i), r: hit, label: relationLine(rels[v.i]), tip: relationTip(rels[v.i])})
	}
}

// relationLine is a row's plain text (its accessible label).
func relationLine(r game.RelationRowView) string {
	ln := fmt.Sprintf("%s  %+d / %+d", r.Name, r.OpinionTo, r.OpinionFrom)
	if r.Label != "" {
		ln = r.Label + " " + ln
	}
	if t := statusTag(r.Status); t != "" {
		ln += " (" + t + ")"
	}
	return ln
}

// paintRelationRow draws one relation row: perspective label, name, status
// tag, the big opinion of them and the small (their opinion of us).
func paintRelationRow(dst *ebiten.Image, row rect, r game.RelationRowView, stripe bool) {
	if stripe {
		fillRect(dst, row, uiStripe)
	}
	const (
		labelW = 74
		tagW   = 64
		valW   = 92
	)
	small := tsSmall
	small.Color = uiTextDim
	ty := float64(row.Y) + (float64(row.H)-lineH(tsBody))/2
	sy := float64(row.Y) + (float64(row.H)-lineH(small))/2
	drawText(dst, fitText(r.Label, small, labelW-6), float64(row.X+4), sy, small)
	nameW := row.W - labelW - tagW - valW
	nm := tsBody
	if r.Status != "" {
		nm.Color = uiTextDim
	}
	drawText(dst, fitText(r.Name, nm, nameW), float64(row.X+labelW), ty, nm)
	if t := statusTag(r.Status); t != "" {
		drawText(dst, fitText(t, small, tagW-4), float64(row.X+labelW+nameW), sy, small)
	}
	big := tsLabel
	big.Color = opinionColor(r.OpinionTo)
	big.Align = alignRight
	drawText(dst, fmt.Sprintf("%+d", r.OpinionTo), float64(row.right()-48), float64(row.Y)+(float64(row.H)-lineH(big))/2, big)
	sm := small
	sm.Color = opinionColor(r.OpinionFrom)
	sm.Align = alignRight
	drawText(dst, fmt.Sprintf("(%+d)", r.OpinionFrom), float64(row.right()-6), sy, sm)
}

// socialLog is the scrollable social log, newest first.
func (a *App) socialLog(r rect, log []game.SocialLogLineView) {
	if len(log) == 0 {
		a.uiLabel(r.X, r.Y, socialLogEmpty, dimText())
		return
	}
	lines := logNewestFirst(log)
	area := a.uiScrollArea(socialLogID, r, float32(len(lines))*socialLogRowH, scrollOpts{Step: socialLogRowH * 2})
	art := a.art
	a.uiSt().queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, area.View)
		for i, l := range lines {
			y := area.View.Y + float32(i)*socialLogRowH - area.Off
			if y+socialLogRowH < area.View.Y || y > area.View.bottom() {
				continue
			}
			st := tsSmall
			st.Color = uiTextDim
			if l.Weighty {
				st.Color = uiWarning
			}
			x := float64(area.View.X)
			if l.Icon != "" {
				if img := art.image("ui_icon/"+l.Icon, noTint); img != nil {
					drawIcon(sub, img, x, float64(y)+1, 14)
				}
			}
			drawText(sub, fitText(l.Text, st, area.View.W-20), x+18, float64(y)+(socialLogRowH-lineH(st))/2, st)
		}
	})
}
