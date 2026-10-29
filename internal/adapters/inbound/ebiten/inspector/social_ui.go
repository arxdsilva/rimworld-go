package inspector

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// The pawn inspector's Social tab (MAIN_UI_SCREENS §15, ENG-003 C4): the
// colonist's relation rows with both opinions, a hover card that breaks
// each opinion down, and the social log under them. Read-only: it reads
// Simulation.Social and has no editing controls (§16.9).

const (
	socialRowH      = 24
	socialLogRowH   = 18
	SocialEmptyText = "No relationships yet"
	socialLogEmpty  = "Nothing has happened yet."
	socialRowsID    = "social.rows"
	socialLogID     = "social.log"
)

// opinionBand is the colour band of an opinion: 0 (-100..-60) to 4 (60..100).
func OpinionBand(v int) int {
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
func OpinionColor(v int) color.RGBA {
	switch OpinionBand(v) {
	case 0:
		return uikit.Danger
	case 1:
		return uikit.Warning
	case 2:
		return uikit.TextDim
	case 3:
		return uikit.Good
	}
	return uikit.LerpRGBA(uikit.Good, uikit.Heading, 0.45)
}

// statusTag is the short tag of a relation whose person is not around.
func StatusTag(s string) string {
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
func RelationTip(r game.RelationRowView) string {
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
func LogNewestFirst(log []game.SocialLogLineView) []game.SocialLogLineView {
	out := append([]game.SocialLogLineView(nil), log...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Tick > out[j].Tick })
	return out
}

// socialTab declares and paints the Social tab of colonist p in the
// inspector's content rectangle in, below the colonist's name.
func (a Tabs) socialTab(p game.PawnView, in gfx.Rect) {
	s, ok := a.sim().Social(p.ID)
	top := in.Y + float32(gfx.LineStep(gfx.Label)) + 8
	if !ok {
		a.kit().Label(in.X, top, noDataYet, uikit.DimText())
		return
	}
	rowsH := (in.Bottom() - top) * 0.55
	rows := gfx.Rect{X: in.X, Y: top, W: in.W, H: rowsH}
	a.socialRows(rows, s.Relations)

	ly := rows.Bottom() + 8
	a.kit().Label(in.X, ly, "Social log", gfx.Label)
	ly += float32(gfx.LineStep(gfx.Label)) + 2
	a.socialLog(gfx.Rect{X: in.X, Y: ly, W: in.W, H: in.Bottom() - ly}, s.Log)
}

// socialRows is the scrollable relation list.
func (a Tabs) socialRows(r gfx.Rect, rels []game.RelationRowView) {
	u := a.kit()
	if len(rels) == 0 {
		a.kit().Label(r.X, r.Y, SocialEmptyText, uikit.DimText())
		return
	}
	area := a.kit().ScrollArea(socialRowsID, r, float32(len(rels))*socialRowH, uikit.ScrollOpts{Step: socialRowH * 2})
	type vis struct {
		i int
		y float32
	}
	var shown []vis
	for i := range rels {
		y := area.View.Y + float32(i)*socialRowH - area.Off
		if y+socialRowH < area.View.Y || y > area.View.Bottom() {
			continue
		}
		shown = append(shown, vis{i, y})
	}
	u.Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		for _, v := range shown {
			paintRelationRow(sub, gfx.Rect{X: area.View.X, Y: v.y, W: area.View.W, H: socialRowH}, rels[v.i], v.i%2 == 1)
		}
	})
	for _, v := range shown {
		hit := gfx.Rect{X: area.View.X, Y: v.y, W: area.View.W, H: socialRowH}.Intersect(area.View)
		a.kit().Add(uikit.Widget{ID: fmt.Sprintf("social.row.%d", v.i), R: hit, Label: relationLine(rels[v.i]), Tip: RelationTip(rels[v.i])})
	}
}

// relationLine is a row's plain text (its accessible label).
func relationLine(r game.RelationRowView) string {
	ln := fmt.Sprintf("%s  %+d / %+d", r.Name, r.OpinionTo, r.OpinionFrom)
	if r.Label != "" {
		ln = r.Label + " " + ln
	}
	if t := StatusTag(r.Status); t != "" {
		ln += " (" + t + ")"
	}
	return ln
}

// paintRelationRow draws one relation row: perspective label, name, status
// tag, the big opinion of them and the small (their opinion of us).
func paintRelationRow(dst *ebiten.Image, row gfx.Rect, r game.RelationRowView, stripe bool) {
	if stripe {
		uikit.FillRect(dst, row, uiStripe)
	}
	const (
		labelW = 92
		tagW   = 64
		valW   = 92
	)
	small := gfx.Small
	small.Color = uikit.TextDim
	ty := float64(row.Y) + (float64(row.H)-gfx.LineH(gfx.Body))/2
	sy := float64(row.Y) + (float64(row.H)-gfx.LineH(small))/2
	gfx.Text(dst, uikit.FitText(r.Label, small, labelW-6), float64(row.X+4), sy, small)
	nameW := row.W - labelW - tagW - valW
	nm := gfx.Body
	if r.Status != "" {
		nm.Color = uikit.TextDim
	}
	gfx.Text(dst, uikit.FitText(r.Name, nm, nameW), float64(row.X+labelW), ty, nm)
	if t := StatusTag(r.Status); t != "" {
		gfx.Text(dst, uikit.FitText(t, small, tagW-4), float64(row.X+labelW+nameW), sy, small)
	}
	big := gfx.Label
	big.Color = OpinionColor(r.OpinionTo)
	big.Align = gfx.AlignRight
	gfx.Text(dst, fmt.Sprintf("%+d", r.OpinionTo), float64(row.Right()-48), float64(row.Y)+(float64(row.H)-gfx.LineH(big))/2, big)
	sm := small
	sm.Color = OpinionColor(r.OpinionFrom)
	sm.Align = gfx.AlignRight
	gfx.Text(dst, fmt.Sprintf("(%+d)", r.OpinionFrom), float64(row.Right()-6), sy, sm)
}

// socialLog is the scrollable social log, newest first.
func (a Tabs) socialLog(r gfx.Rect, log []game.SocialLogLineView) {
	if len(log) == 0 {
		a.kit().Label(r.X, r.Y, socialLogEmpty, uikit.DimText())
		return
	}
	lines := LogNewestFirst(log)
	area := a.kit().ScrollArea(socialLogID, r, float32(len(lines))*socialLogRowH, uikit.ScrollOpts{Step: socialLogRowH * 2})
	art := a.env().Art
	a.kit().Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		for i, l := range lines {
			y := area.View.Y + float32(i)*socialLogRowH - area.Off
			if y+socialLogRowH < area.View.Y || y > area.View.Bottom() {
				continue
			}
			st := gfx.Small
			st.Color = uikit.TextDim
			if l.Weighty {
				st.Color = uikit.Warning
			}
			x := float64(area.View.X)
			if l.Icon != "" {
				if img := art.Image("ui_icon/"+l.Icon, color.NRGBA{}); img != nil {
					gfx.Icon(sub, img, x, float64(y)+1, 14)
				}
			}
			gfx.Text(sub, uikit.FitText(l.Text, st, area.View.W-20), x+18, float64(y)+(socialLogRowH-gfx.LineH(st))/2, st)
		}
	})
}
