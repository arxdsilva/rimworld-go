package dossier

import (
	"image/color"
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/platform/config"
)

// builder turns one pruned tree into items for the current facts.
type builder struct {
	items              []item
	p                  *Panel
	hud                config.HUD
	m                  sheet.Metrics
	body, large, small gfx.TextStyle
	dec                bool
}

func (p *Panel) builder(hud config.HUD) *builder {
	t := hud.Text
	b := &builder{p: p, hud: hud, m: sheet.MetricsFor(t), dec: t.Decimals}
	b.body = gfx.TextStyle{Size: float64(t.BodySize)}
	b.large = gfx.TextStyle{Size: float64(t.BodySize + t.LargeDelta), Bold: true}
	b.small = gfx.TextStyle{Size: float64(t.BodySize + t.SmallDelta)}
	return b
}

func (b *builder) col(key string) (c color.RGBA) { return b.p.pal[key] }

func (b *builder) styled(st gfx.TextStyle, key string) gfx.TextStyle {
	st.Color = b.col(key)
	return st
}

func (b *builder) toned(st gfx.TextStyle, t sheet.Tone) gfx.TextStyle {
	return b.styled(st, sheet.ToneKey(t))
}

// text is a one-line item; empty text gives no item.
func (b *builder) text(l sheet.Line, st gfx.TextStyle, tip tipKind, w float32) (item, bool) {
	if l.Text == "" {
		return item{}, false
	}
	it := item{kind: kText, tab: l.Tab, tip: tip}
	if tip == tipNone && l.Tip != "" {
		it.tip, it.plain = tipPlain, l.Tip
	}
	it.st = b.toned(st, l.Tone)
	it.text = fit(l.Text, it.st, w)
	return it, true
}

func fit(s string, st gfx.TextStyle, w float32) string {
	if w <= 0 {
		return s
	}
	return gfx.Ellipsize(s, st, float64(w))
}

// widget builds one node's item (geometry apart); ok false: it has nothing to
// show and takes no room.
func (b *builder) widget(n config.HUDNode, w float32) (item, bool) {
	d := &b.p.d
	switch n.Kind {
	case sheet.KeyGap:
		return item{}, false
	case sheet.KeyRule:
		return item{kind: kRule, key: n.Kind, track: b.col("rule")}, true
	}
	if !sheet.IsWidget(n.Kind) {
		b.p.warnUnknown(n.Kind)
		return item{kind: kMarker, key: n.Kind, text: n.Kind,
			st: b.styled(b.small, "tree_bad"), back: b.col("tree_bad")}, true
	}
	var (
		it item
		ok bool
	)
	if d.animal {
		it, ok = b.animalWidget(n, w)
	} else {
		it, ok = b.pawnWidget(n, w)
	}
	it.key = n.Kind
	return it, ok
}

func (b *builder) pawnWidget(n config.HUDNode, w float32) (item, bool) {
	d := &b.p.d
	pv, c := d.p, d.card
	switch n.Kind {
	case "title":
		it := item{kind: kText, tab: sheet.TabSocial, tip: tipBio}
		it.st = b.styled(b.large, sheet.RelationKey(sheet.RelationOf(pv, c)))
		it.text = fit(pv.Name, it.st, w)
		return it, pv.Name != ""
	case "standing":
		return b.text(sheet.StandingLine(pv, d.bio, c), b.body, tipBio, w)
	case "identity":
		return b.text(sheet.IdentityLine(pv, d.bio, c), b.body, tipBio, w)
	case "body_state":
		return b.text(sheet.BodyLine(pv, d.health), b.body, tipHealth, w)
	case "mind_state":
		return b.text(sheet.MindLine(pv, c), b.body, tipMind, w)
	case "doing":
		return b.text(sheet.ActivityLine(pv, c), b.bold(), tipActivity, w)
	case "next_up":
		return b.text(sheet.QueuedLine(pv), b.body, tipNone, w)
	case "in_hand":
		return b.text(sheet.EquippedLine(pv), b.body, tipNone, w)
	case "hauling":
		return b.text(sheet.CarryingLine(pv), b.body, tipNone, w)
	case "traits":
		return b.text(sheet.TraitsLine(d.bio), b.body, tipTraits, w)
	case "captive":
		return b.text(sheet.CaptiveLine(pv, d.pris, c), b.body, tipNone, w)
	case "tally":
		return b.text(sheet.TallyLine(pv, n.Def), b.body, tipNone, w)
	case "stat":
		return b.text(sheet.StatLine(c, n.Def, b.dec), b.body, tipNone, w)
	case "notes":
		return b.text(b.pawnNotes(), b.body, tipNone, w)
	case "vigour_bar":
		return b.bar("Health", pv.Health, n, w, tipHealth, sheet.TabHealth), true
	case "mood_bar":
		it := b.bar("Mood", pv.Mood, n, w, tipMind, sheet.TabNeeds)
		at := [3]int{c.BreakAt.Minor, c.BreakAt.Major, c.BreakAt.Extreme}
		for _, v := range at {
			if v > 0 && v < 100 {
				it.ticks[it.nticks] = float32(v) / 100
				it.nticks++
			}
		}
		return it, true
	case "hunger_bar":
		return b.bar("Food", pv.Hunger, n, w, tipFood, sheet.TabNeeds), true
	case "rest_bar":
		return b.bar("Rest", pv.Rest, n, w, tipRest, sheet.TabNeeds), true
	case "fun_bar":
		return b.bar("Recreation", pv.Recreation, n, w, tipFun, sheet.TabNeeds), true
	case "need_bar":
		label, pct, ok := sheet.NeedValue(pv, n.Def)
		if !ok {
			return item{}, false
		}
		return b.bar(label, pct, n, w, tipNone, sheet.TabNeeds), true
	case "skill":
		return b.skill(n.Def, w)
	case "diet_pick":
		if pv.Hostile || pv.Visitor || pv.Ally || pv.FoodPolicy == "" {
			return item{}, false
		}
		return b.pick("Food: "+pv.FoodPolicy, actDiet, b.col("picker_back"), w), true
	case "area_pick":
		if d.who != sheet.WhoColonist || pv.Prisoner || b.p.sc == nil {
			return item{}, false
		}
		return b.areaPick(w), true
	case "hour_pick":
		if d.who != sheet.WhoColonist || pv.Prisoner || b.p.sc == nil {
			return item{}, false
		}
		return b.hourPick(w), true
	}
	return item{}, false
}

func (b *builder) animalWidget(n config.HUDNode, w float32) (item, bool) {
	d := &b.p.d
	av, c := d.a, d.card
	name := av.Name
	if name == "" {
		name = av.Label
	}
	switch n.Kind {
	case "title":
		it := item{kind: kText, tip: tipBio}
		if av.Tame {
			it.tab = sheet.TabTraining
		}
		it.st = b.styled(b.large, sheet.RelationKey(sheet.AnimalRelation(av)))
		it.text = fit(name, it.st, w)
		return it, name != ""
	case "standing":
		return b.text(sheet.AnimalStandingLine(av, c), b.body, tipBio, w)
	case "identity":
		return b.text(sheet.AnimalIdentityLine(av, c), b.body, tipBio, w)
	case "body_state":
		return b.text(sheet.AnimalBodyLine(av), b.body, tipBio, w)
	case "mind_state":
		return b.text(sheet.AnimalMindLine(av, c), b.body, tipNone, w)
	case "doing":
		return b.text(sheet.AnimalActivityLine(av, c), b.bold(), tipActivity, w)
	case "keeper":
		l := sheet.KeeperLine(av)
		if l.Tab == "" && av.Tame {
			l.Tab = sheet.TabTraining
		}
		return b.text(l, b.body, tipNone, w)
	case "stat":
		return b.text(sheet.StatLine(c, n.Def, b.dec), b.body, tipNone, w)
	case "notes":
		return b.text(b.animalNotes(), b.body, tipNone, w)
	case "vigour_bar":
		return b.bar("Health", av.Health, n, w, tipBio, ""), true
	case "hunger_bar":
		return b.bar("Food", av.Food, n, w, tipBio, ""), true
	case "rest_bar":
		return b.bar("Rest", av.Rest, n, w, tipBio, ""), true
	case "train":
		return b.train(n.Def, w)
	case "diet_pick":
		if !av.Tame || av.FoodPolicy == "" {
			return item{}, false
		}
		return b.pick("Food: "+av.FoodPolicy, actDiet, b.col("picker_back"), w), true
	case "area_pick":
		if !av.Tame || b.p.sc == nil {
			return item{}, false
		}
		return b.areaPick(w), true
	}
	return item{}, false
}

func (b *builder) bold() gfx.TextStyle {
	s := b.body
	s.Bold = true
	return s
}

// bar builds a bar row: label, bar, value.
func (b *builder) bar(label string, pct int, n config.HUDNode, w float32, tip tipKind, tab string) item {
	pct = max(0, min(100, pct))
	frac := float64(pct) / 100
	style := n.BarStyle
	if style == "" {
		style = sheet.BarFadeUp
	}
	it := item{kind: kBar, tip: tip, tab: tab, frac: float32(frac)}
	it.st = b.styled(b.body, "text")
	it.vst = it.st
	it.vst.Align = gfx.AlignRight
	it.text, it.value = label, strconv.Itoa(pct)+"%"
	it.fill = sheet.BarColor(style, frac, b.col("bar_low"), b.col("bar_main"))
	it.track, it.tick = b.col("bar_back"), b.col("bar_tick")
	h := sheet.WidgetHeight(n.Kind, n.Def, b.m)
	lw := min(b.m.LabelW, w*0.45)
	vw := min(b.m.ValueW, w*0.3)
	it.bx, it.bw = lw+4, max(0, w-lw-vw-8)
	it.bh = max(4, h*0.5)
	it.by = (h - it.bh) / 2
	return it
}

func (b *builder) skill(def string, w float32) (item, bool) {
	d := &b.p.d
	for i, s := range d.bio.Skills {
		if s.Key != def {
			continue
		}
		cell := sheet.SkillCell(s, d.card, b.dec)
		key := sheet.SkillStateOf(s, d.card).Key()
		it := b.cell(cell, key, w)
		it.tip, it.arg, it.tab = tipSkill, i, sheet.TabBio
		if s.Passion > 0 {
			ic := "ui_icon/passion_minor"
			if s.Passion >= 2 {
				ic = "ui_icon/passion_major"
			}
			it.passIc = b.p.icon(ic)
			if it.passIc != nil {
				it.lblX = b.m.BodyH
			}
		}
		return it, true
	}
	return item{}, false
}

func (b *builder) train(def string, w float32) (item, bool) {
	d := &b.p.d
	sk, ok := sheet.TrainByKey(def)
	if !ok {
		return item{}, false
	}
	for i, t := range d.a.Training {
		if t.Skill != int(sk) {
			continue
		}
		cell := sheet.TrainCell(t)
		if cell.Label == "" {
			return item{}, false
		}
		it := b.cell(cell, sheet.ToneKey(cell.Tone), w)
		it.tip, it.arg, it.tab = tipTrain, i, sheet.TabTraining
		return it, true
	}
	return item{}, false
}

// cell builds a skill or training entry from its cell.
func (b *builder) cell(c sheet.Cell, colKey string, w float32) item {
	if c.Tone != sheet.ToneText && colKey == "text" {
		colKey = sheet.ToneKey(c.Tone)
	}
	it := item{kind: kCell, text: c.Label, value: c.Value, frac: float32(c.Pct) / 100}
	st := b.styled(b.body, colKey)
	st.Bold = c.Active
	it.st = st
	it.vst = st
	it.vst.Align = gfx.AlignRight
	f := uikit.Fade(b.col(colKey), 90)
	it.fill, it.track = f, b.col("bar_back")
	it.text = fit(c.Label, st, max(0, w-b.m.ValueW-4))
	return it
}

// pick builds a selector.
func (b *builder) pick(label string, a action, back color.RGBA, w float32) item {
	it := item{kind: kPick, act: a, back: back}
	it.st = b.styled(b.small, "picker_text")
	it.text = fit(label+" v", it.st, max(0, w-8))
	return it
}

func (b *builder) areaPick(w float32) item {
	d := &b.p.d
	ref := b.p.sc.AreaOf(d.p.ID)
	if d.animal {
		ref = b.p.sc.AreaOf(d.a.ID)
	}
	name, back := b.p.areaLook(ref)
	back = sheet.Lerp(b.col("picker_back"), back, 0.5)
	if name == "" {
		back = b.col("picker_back")
		name = "Unrestricted"
	}
	return b.pick("Area: "+name, actArea, back, w)
}

func (b *builder) hourPick(w float32) item {
	act := b.p.d.p.Activity
	back := b.col("picker_back")
	switch act {
	case schedule.Work:
		back = sheet.Lerp(back, b.col("tone_warning"), 0.35)
	case schedule.Recreation:
		back = sheet.Lerp(back, b.col("tone_good"), 0.35)
	case schedule.Sleep:
		back = sheet.Lerp(back, b.col("side_ally"), 0.35)
	}
	return b.pick("Hour "+strconv.Itoa(b.p.sc.Hour())+": "+act.String(), actHour, back, w)
}

// areaLook is an area ref's name ("" unrestricted) and colour.
func (p *Panel) areaLook(ref int) (string, color.RGBA) {
	switch {
	case ref == 0 || ref == -1:
		return "", color.RGBA{}
	case ref == -2:
		return "Home", p.pal["picker_back"]
	}
	for _, a := range p.r.Areas() {
		if int(a.ID) == ref {
			return a.Name, color.RGBA{R: a.Color[0], G: a.Color[1], B: a.Color[2], A: 0xFF}
		}
	}
	return "Area", p.pal["picker_back"]
}

// pawnNotes are the short status notes of a person.
func (b *builder) pawnNotes() sheet.Line {
	pv := b.p.d.p
	var parts []string
	if pv.Drafted {
		parts = append(parts, "drafted")
	}
	if pv.Infection > 0 {
		parts = append(parts, "infected "+strconv.Itoa(pv.Infection)+"%")
	}
	if pv.Malnutrition > 0 {
		parts = append(parts, "malnourished")
	}
	if pv.ToxicBuildup > 0 {
		parts = append(parts, "toxic buildup "+strconv.Itoa(pv.ToxicBuildup)+"%")
	}
	if pv.Frostbite > 0 {
		parts = append(parts, "frostbite")
	}
	return notes(parts)
}

func (b *builder) animalNotes() sheet.Line {
	av := b.p.d.a
	var parts []string
	if av.Pregnant {
		parts = append(parts, "pregnant")
	}
	if av.Hunts && av.Tame {
		parts = append(parts, "hunts when hungry")
	}
	if av.Hypothermia > 0 {
		parts = append(parts, "chilled")
	}
	if av.Heatstroke > 0 {
		parts = append(parts, "overheated")
	}
	if av.ToxicBuildup > 0 {
		parts = append(parts, "toxic buildup "+strconv.Itoa(av.ToxicBuildup)+"%")
	}
	return notes(parts)
}

func notes(parts []string) sheet.Line {
	if len(parts) == 0 {
		return sheet.Line{}
	}
	s := strings.Join(parts, ", ")
	return sheet.Line{Text: strings.ToUpper(s[:1]) + s[1:], Tone: sheet.ToneWarning}
}

// warnUnknown tells once per key that the layout holds an element the card
// does not know; the card draws a marker box in its place.
func (p *Panel) warnUnknown(key string) {
	if p.warned[key] {
		return
	}
	p.warned[key] = true
	p.a.Hint("The card layout has an element it does not know: " + key)
}
