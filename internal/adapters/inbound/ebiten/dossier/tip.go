package dossier

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// tipper is the hover card of the panel.
type tipper struct{ p *Panel }

const (
	tipMaxW = 280 // the card's widest line, narrower than the kit's
	tipPad  = 6
)

// tipCard is the hover card, laid out when the cursor comes to rest and placed
// by the cursor each frame.
type tipCard struct {
	on   bool
	rows []tipRow
	w, h float32
	box  gfx.Rect
}

type tipRow struct {
	text string
	st   gfx.TextStyle
}

// hover tracks the item under the cursor and shows its card after a rest.
func (t tipper) hover(l *layout, hov, mx, my int) {
	if hov != t.p.hov {
		t.p.hov, t.p.hovSince, t.p.tip.on = hov, t.p.now(), false
	}
	if hov < 0 || !t.p.live() || t.p.now()-t.p.hovSince < tipDelay {
		return
	}
	if !t.p.tip.on {
		t.buildTip(&l.items[hov])
	}
	if !t.p.tip.on {
		return
	}
	sw, sh := t.p.sz.Size()
	x, y := float32(mx+14), float32(my+20)
	if x+t.p.tip.w > float32(sw-uikit.TipMargin) {
		x = float32(sw-uikit.TipMargin) - t.p.tip.w
	}
	if y+t.p.tip.h > float32(sh-uikit.TipMargin) {
		y = float32(my) - t.p.tip.h - 8
	}
	t.p.tip.box = gfx.Rect{X: max(uikit.TipMargin, x), Y: max(uikit.TipMargin, y), W: t.p.tip.w, H: t.p.tip.h}
	t.p.k.Queue(t.p.tipPaint)
}

// buildTip wraps the item's tip lines, each in its tone.
func (t tipper) buildTip(it *item) {
	lines := t.tipLines(it)
	if len(lines) == 0 {
		return
	}
	body := gfx.TextStyle{Size: float64(t.p.bodySize)}
	inner := float64(tipMaxW - 2*tipPad)
	t.p.tip.rows = t.p.tip.rows[:0]
	var w, h float64
	for i, ln := range lines {
		st := body
		st.Bold = i == 0
		st.Color = t.p.pal[sheet.ToneKey(ln.Tone)]
		for _, s := range gfx.WrapPx(ln.Text, st, inner) {
			t.p.tip.rows = append(t.p.tip.rows, tipRow{text: s, st: st})
			tw, _ := gfx.Measure(s, st)
			w = max(w, tw)
			h += gfx.LineH(st)
		}
	}
	t.p.tip.w, t.p.tip.h = float32(w)+2*tipPad, float32(h)+2*tipPad
	t.p.tip.on = len(t.p.tip.rows) > 0
}

// paintTip draws the card; the kit calls it from its paint list.
func (t tipper) paintTip(dst *ebiten.Image) {
	if !t.p.tip.on {
		return
	}
	b := t.p.tip.box
	uikit.DrawPanel(dst, b.X, b.Y, b.W, b.H)
	y := float64(b.Y + tipPad)
	for i := range t.p.tip.rows {
		r := &t.p.tip.rows[i]
		gfx.Text(dst, r.text, float64(b.X+tipPad), y, r.st)
		y += gfx.LineH(r.st)
	}
}

// tipLines are the lines of an item's tooltip.
func (t tipper) tipLines(it *item) []sheet.TipLine {
	d := &t.p.d
	switch it.tip {
	case tipPlain:
		return []sheet.TipLine{{Text: it.plain}}
	case tipBio:
		if d.animal {
			return sheet.AnimalTip(t.p.a, d.a)
		}
		return sheet.BioTip(t.p.a, d.p, d.bio, d.card)
	case tipHealth:
		return sheet.HealthTip(t.p.a, d.health)
	case tipMind:
		return sheet.MindTip(t.p.a, d.p, d.card)
	case tipFood:
		return sheet.FoodTip(t.p.a, "", d.p.FoodPolicy, d.card)
	case tipRest:
		return sheet.RestTip(t.p.a, d.card)
	case tipFun:
		return sheet.FunTip(t.p.a, d.p)
	case tipTraits:
		return sheet.TraitsTip(t.p.a, d.bio)
	case tipActivity:
		return sheet.ActivityTip(t.p.a, d.card)
	case tipSkill:
		if it.arg < len(d.bio.Skills) {
			s := d.bio.Skills[it.arg]
			weapon := ""
			if s.Key == "melee" {
				weapon = sheet.WeaponLine(t.p.a, d.p)
			}
			return sheet.SkillTip(t.p.a, s, d.card, weapon)
		}
	case tipTrain:
		if it.arg < len(d.a.Training) {
			return sheet.TrainTip(t.p.a, d.a, d.a.Training[it.arg])
		}
	}
	return nil
}
