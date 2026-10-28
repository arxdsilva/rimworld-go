package wizard

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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

// PrettyKey turns "industrial_medicine" into "Industrial medicine".
func PrettyKey(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func itemLabel(k item.Kind) string { return PrettyKey(k.String()) }

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

// ScenarioLines is the key-value body of a scenario's details panel.
func ScenarioLines(d scenario.Def) []game.DetailLine {
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
	Rows  []detailRow
	Total float32
}

// BuildDetail lays out the title, summary, narrative and lines of a
// scenario in w px.
func BuildDetail(d scenario.Def, lines []game.DetailLine, w float32) detailLayout {
	var L detailLayout
	add := func(h float32, tip string, draw func(dst *ebiten.Image, x, y, w float32)) {
		L.Rows = append(L.Rows, detailRow{y: L.Total, h: h, draw: draw, tip: tip})
		L.Total += h
	}
	text := func(s string, st gfx.TextStyle) {
		for _, l := range gfx.WrapPx(s, st, float64(w)) {
			l := l
			add(float32(gfx.LineH(st)), "", func(dst *ebiten.Image, x, y, _ float32) { gfx.Text(dst, l, float64(x), float64(y), st) })
		}
	}
	title := gfx.Title
	title.Color = uikit.Heading
	name := d.Name
	if strings.TrimSpace(name) == "" {
		name = "Unnamed scenario"
	}
	text(name, title)
	if d.Summary != "" {
		L.Total += 2
		text(d.Summary, uikit.DimText())
	}
	if d.Desc != "" {
		L.Total += detailGap
		text(d.Desc, gfx.Body)
	}
	L.Total += detailGap
	lh := float32(gfx.LineH(gfx.Body))
	for _, ln := range lines {
		ln := ln
		switch {
		case ln.Heading:
			L.Total += 6
			hs := gfx.Label
			hs.Color = uikit.Heading
			add(float32(gfx.LineH(hs))+2, "", func(dst *ebiten.Image, x, y, w float32) {
				gfx.Text(dst, ln.Text, float64(x), float64(y), hs)
				uikit.FillRect(dst, gfx.Rect{X: x, Y: y + float32(gfx.LineH(hs)) + 1, W: w, H: 1}, uikit.PanelBorder)
			})
		case strings.Contains(ln.Text, "\t"):
			k, v, _ := strings.Cut(ln.Text, "\t")
			vals := gfx.WrapPx(v, gfx.Body, float64(w-detailKeyW))
			h := max(lh, float32(len(vals))*lh)
			add(h, ln.Tip, func(dst *ebiten.Image, x, y, _ float32) {
				ks := uikit.DimText()
				gfx.Text(dst, k, float64(x), float64(y), ks)
				for i, l := range vals {
					gfx.Text(dst, l, float64(x+detailKeyW), float64(y)+float64(i)*float64(lh), gfx.Body)
				}
			})
		default:
			rows := gfx.WrapPx(ln.Text, gfx.Body, float64(w-14))
			h := float32(len(rows)) * lh
			add(h, ln.Tip, func(dst *ebiten.Image, x, y, _ float32) {
				gfx.Text(dst, "•", float64(x+2), float64(y), uikit.DimText())
				for i, l := range rows {
					gfx.Text(dst, l, float64(x+14), float64(y)+float64(i)*float64(lh), gfx.Body)
				}
			})
		}
	}
	return L
}

// detailView declares a scenario's details in r (a scroll view with
// tooltips on the item lines) and queues its drawing.
func (w *Flow) detailView(id string, r gfx.Rect, d scenario.Def, lines []game.DetailLine) {
	pane := r.Inset(10)
	L := BuildDetail(d, lines, max(40, pane.W-uikit.ScrollBarW-8))
	area := w.kit().ScrollArea(id, pane, L.Total, uikit.ScrollOpts{Step: 48})
	u := w.kit()
	for i, row := range L.Rows {
		if row.tip == "" {
			continue
		}
		rr := gfx.Rect{X: area.View.X, Y: area.View.Y + row.y - area.Off, W: area.View.W, H: row.h}.Intersect(area.View)
		if !rr.Empty() {
			w.kit().Tip(id+".tip."+uikit.Num(i), rr, row.tip)
		}
	}
	u.Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		for _, row := range L.Rows {
			y := area.View.Y + row.y - area.Off
			if y+row.h < area.View.Y || y > area.View.Bottom() {
				continue
			}
			row.draw(sub, area.View.X, y, area.View.W-8)
		}
	})
}
