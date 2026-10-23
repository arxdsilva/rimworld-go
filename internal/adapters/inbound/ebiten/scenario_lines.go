package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/scenario"
)

// The read-only details of a scenario (NG-1 §2.2), built straight from the
// definition: `game.ScenarioViewOf` is a stub until SETUP fills it. The
// same renderer draws Choose Scenario's right column and the Scenario
// Editor's live preview.

// prettyKey turns "industrial_medicine" into "Industrial medicine".
func prettyKey(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func itemLabel(k item.Kind) string { return prettyKey(k.String()) }

func techLabel(t faction.Tech) string {
	switch t {
	case faction.Neolithic:
		return "Neolithic"
	case faction.Spacer:
		return "Spacer"
	}
	return "Industrial"
}

func arrivalLabel(a scenario.Arrival) string {
	switch a {
	case scenario.Standing:
		return "Standing"
	case scenario.WalkIn:
		return "Walk in"
	}
	return "Drop pods"
}

func factionLabel(t faction.Type) string {
	if t == faction.Clans {
		return "Clan"
	}
	return "Settlers"
}

// kv is a key-value detail line: the key, a tab, the value.
func kv(k, v string) game.DetailLine { return game.DetailLine{Text: k + "\t" + v} }

func heading(s string) game.DetailLine { return game.DetailLine{Heading: true, Text: s} }

// stackLine is "Rifle x2" for a starting or scattered stack.
func stackLine(s scenario.Stack, scattered bool) game.DetailLine {
	text := fmt.Sprintf("%s ×%d", itemLabel(s.Kind), s.Qty)
	tip := itemLabel(s.Kind)
	if scattered {
		where := "anywhere on the map"
		if s.Near {
			where = "near the start"
		}
		text += fmt.Sprintf(" in %d piles, %s", max(s.Piles, 1), where)
		tip += "\nScattered around the landing site."
	} else {
		tip += "\nIn the starting kit."
	}
	if s.Kind.IsFood() {
		tip += " Food."
	}
	return game.DetailLine{Text: text, Tip: tip}
}

// mergeStacks adds up stacks of the same item (starting kit lines).
func mergeStacks(in []scenario.Stack) []scenario.Stack {
	var out []scenario.Stack
	for _, s := range in {
		merged := false
		for i := range out {
			if out[i].Kind == s.Kind {
				out[i].Qty += s.Qty
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, s)
		}
	}
	return out
}

// scenarioLines is the key-value body of a scenario's details panel.
func scenarioLines(d scenario.Def) []game.DetailLine {
	e := d.Apply()
	var out []game.DetailLine
	out = append(out, kv("Player faction", factionLabel(d.Faction)))
	people := fmt.Sprintf("%d", d.People)
	if d.ChooseFrom > d.People {
		people += fmt.Sprintf(", chosen from %d candidates", d.ChooseFrom)
	}
	out = append(out, kv("Starting people", people))
	out = append(out, kv("Arrival", arrivalLabel(d.Arrival)))
	out = append(out, kv("Tech level", techLabel(d.Tech)))
	out = append(out, kv("Research cost", fmt.Sprintf("%d%% of normal", e.ResearchCostPct)))
	if len(e.Research) > 0 {
		out = append(out, heading("Research granted"))
		for _, p := range e.Research {
			def := research.DefOf(p)
			out = append(out, game.DetailLine{Text: def.Label, Tip: def.Label + "\n" + def.Desc})
		}
	}
	if items := mergeStacks(e.StartItems); len(items) > 0 {
		out = append(out, heading("Starting equipment and supplies"))
		for _, s := range items {
			out = append(out, stackLine(s, false))
		}
	}
	var animals []string
	for _, p := range d.Parts {
		if p.Kind == scenario.PartStartAnimal {
			animals = append(animals, p.Summary())
		}
	}
	if len(animals) > 0 {
		out = append(out, heading("Starting animals"))
		for _, a := range animals {
			out = append(out, game.DetailLine{Text: a})
		}
	}
	if len(e.Scatter) > 0 {
		out = append(out, heading("Scattered on the map"))
		for _, s := range e.Scatter {
			out = append(out, stackLine(s, true))
		}
	}
	var rules []game.DetailLine
	for _, p := range d.Parts {
		switch p.Kind {
		case scenario.PartForcedTrait:
			rules = append(rules, game.DetailLine{Text: p.Kind.Label() + ": " + p.Summary(), Tip: trait.DefOf(p.Trait).Label + "\n" + trait.DefOf(p.Trait).Desc})
		case scenario.PartCondition:
			rules = append(rules, game.DetailLine{Text: p.Kind.Label() + ": " + p.Summary(), Tip: p.Cond.Label() + "\n" + p.Cond.Desc()})
		case scenario.PartAgeRange, scenario.PartForcedCondition, scenario.PartNoClothes,
			scenario.PartScheduledIncident, scenario.PartDisabledIncident, scenario.PartDisallowBuilding:
			rules = append(rules, game.DetailLine{Text: p.Kind.Label() + ": " + p.Summary(), Tip: p.Kind.Label() + "\n" + p.Kind.Desc()})
		}
	}
	if len(rules) > 0 {
		out = append(out, heading("Special rules"))
		out = append(out, rules...)
	}
	return out
}

// ---- rendering ---------------------------------------------------------

const (
	detailKeyW = 150
	detailGap  = 10
)

// detailRow is one laid-out row of a details panel.
type detailRow struct {
	y, h float32
	draw func(dst *ebiten.Image, x, y, w float32)
	tip  string
}

// detailLayout is a scenario's details panel laid out for a width.
type detailLayout struct {
	rows  []detailRow
	total float32
}

// buildDetail lays out the title, summary, narrative and lines of a
// scenario in w px.
func buildDetail(d scenario.Def, lines []game.DetailLine, w float32) detailLayout {
	var L detailLayout
	add := func(h float32, tip string, draw func(dst *ebiten.Image, x, y, w float32)) {
		L.rows = append(L.rows, detailRow{y: L.total, h: h, draw: draw, tip: tip})
		L.total += h
	}
	text := func(s string, st textStyle) {
		for _, l := range wrapPx(s, st, float64(w)) {
			l := l
			add(float32(lineH(st)), "", func(dst *ebiten.Image, x, y, _ float32) { drawText(dst, l, float64(x), float64(y), st) })
		}
	}
	title := tsTitle
	title.Color = uiHeading
	name := d.Name
	if strings.TrimSpace(name) == "" {
		name = "Unnamed scenario"
	}
	text(name, title)
	if d.Summary != "" {
		L.total += 2
		text(d.Summary, dimText())
	}
	if d.Desc != "" {
		L.total += detailGap
		text(d.Desc, tsBody)
	}
	L.total += detailGap
	lh := float32(lineH(tsBody))
	for _, ln := range lines {
		ln := ln
		switch {
		case ln.Heading:
			L.total += 6
			hs := tsLabel
			hs.Color = uiHeading
			add(float32(lineH(hs))+2, "", func(dst *ebiten.Image, x, y, w float32) {
				drawText(dst, ln.Text, float64(x), float64(y), hs)
				fillRect(dst, rect{x, y + float32(lineH(hs)) + 1, w, 1}, uiPanelBorder)
			})
		case strings.Contains(ln.Text, "\t"):
			k, v, _ := strings.Cut(ln.Text, "\t")
			vals := wrapPx(v, tsBody, float64(w-detailKeyW))
			h := max(lh, float32(len(vals))*lh)
			add(h, ln.Tip, func(dst *ebiten.Image, x, y, _ float32) {
				ks := dimText()
				drawText(dst, k, float64(x), float64(y), ks)
				for i, l := range vals {
					drawText(dst, l, float64(x+detailKeyW), float64(y)+float64(i)*float64(lh), tsBody)
				}
			})
		default:
			rows := wrapPx(ln.Text, tsBody, float64(w-14))
			h := float32(len(rows)) * lh
			add(h, ln.Tip, func(dst *ebiten.Image, x, y, _ float32) {
				drawText(dst, "•", float64(x+2), float64(y), dimText())
				for i, l := range rows {
					drawText(dst, l, float64(x+14), float64(y)+float64(i)*float64(lh), tsBody)
				}
			})
		}
	}
	return L
}

// detailView declares a scenario's details in r (a scroll view with
// tooltips on the item lines) and queues its drawing.
func (a *App) detailView(id string, r rect, d scenario.Def, lines []game.DetailLine) {
	pane := r.inset(10)
	L := buildDetail(d, lines, max(40, pane.W-scrollBarW-8))
	area := a.uiScrollArea(id, pane, L.total, scrollOpts{Step: 48})
	u := a.uiSt()
	for i, row := range L.rows {
		if row.tip == "" {
			continue
		}
		rr := rect{area.View.X, area.View.Y + row.y - area.Off, area.View.W, row.h}.intersect(area.View)
		if !rr.empty() {
			a.uiTip(id+".tip."+uiNum(i), rr, row.tip)
		}
	}
	u.queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, area.View)
		for _, row := range L.rows {
			y := area.View.Y + row.y - area.Off
			if y+row.h < area.View.Y || y > area.View.bottom() {
				continue
			}
			row.draw(sub, area.View.X, y, area.View.W-8)
		}
	})
}
