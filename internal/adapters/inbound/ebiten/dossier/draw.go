package dossier

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
)

const (
	gearSize  = 18
	gearInset = 3
	tipDelay  = 20 // Updates the cursor rests before the card shows
)

// gearRect is the settings button: the lower right corner docked, the upper
// right floating.
func gearRect(r gfx.Rect, docked bool) gfx.Rect {
	g := gfx.Rect{X: r.Right() - gearSize - gearInset, Y: r.Y + gearInset, W: gearSize, H: gearSize}
	if docked {
		g.Y = r.Bottom() - gearSize - gearInset
	}
	return g
}

// live reports the kit takes input for the layer the card is declared in.
func (p *Panel) live() bool {
	return p.k != nil && !p.k.Suppress && p.k.Layer == len(p.k.Modals)
}

// body is the box the tree fills inside the card's rect.
func body(r gfx.Rect, docked bool) gfx.Rect {
	if docked {
		return r
	}
	return r.Inset(cardPad)
}

// prepare reads what the frame needs and returns the layout, or nil when the
// card has nothing to show.
func (p *Panel) prepare(r gfx.Rect, docked bool) (*layout, config.HUD) {
	hud := p.pf.Settings().HUD
	if !p.sync(hud) {
		return nil, hud
	}
	b := body(r, docked)
	return p.layoutFor(hud, docked, b.W, b.H), hud
}

// declare registers what can be clicked or hovered, once per Update.
func (p *Panel) declare(r gfx.Rect, docked bool) {
	l, _ := p.prepare(r, docked)
	if l == nil {
		return
	}
	k := p.k
	live := p.live()
	mx, my := p.sz.Cursor()
	over := live && r.Hit(mx, my)
	gear := gearRect(r, docked)
	// The gear takes its press before any widget under it can see it.
	showGear := over && gear.Hit(mx, my) && k.Press(gear)
	if showGear {
		p.openSetup()
	}
	bx := body(r, docked)
	hov := -1
	for i := range l.items {
		it := &l.items[i]
		if it.id == "" || it.y+it.h > bx.H+0.5 {
			continue
		}
		ir := gfx.Rect{X: bx.X + it.x, Y: bx.Y + it.y, W: it.w, H: it.h}
		if !k.Add(uikit.Widget{ID: it.id, R: ir}) {
			continue
		}
		if !ir.Hit(mx, my) || (over && gear.Hit(mx, my)) {
			continue
		}
		hov = i
		if k.Press(ir) {
			menus{p}.activate(it)
		}
	}
	tipper{p}.hover(l, hov, mx, my)
	if over {
		k.Add(uikit.Widget{ID: ids.DossierGear, R: gear, Label: "Card settings",
			Tip: "Card settings\nChange what the card shows, its text and its colours.", Click: p.setupFn})
	}
}

// draw paints the card.
func (p *Panel) draw(dst *ebiten.Image, r gfx.Rect, docked bool) {
	if !p.have {
		return
	}
	l := &p.lay[modeIdx(docked)]
	if !l.ok {
		return
	}
	if !docked {
		uikit.DrawPanel(dst, r.X, r.Y, r.W, r.H)
	}
	bx := body(r, docked)
	for i := range l.items {
		it := &l.items[i]
		if it.y+it.h > bx.H+0.5 {
			continue
		}
		p.drawItem(dst, it, bx.X+it.x, bx.Y+it.y)
	}
	mx, my := p.sz.Cursor()
	if r.Hit(mx, my) && p.live() {
		p.drawGear(dst, gearRect(r, docked))
	}
}

func (p *Panel) drawItem(dst *ebiten.Image, it *item, x, y float32) {
	if it.id != "" && p.k.IsHot(it.id) {
		gfx.FillRect(dst, x, y, it.w, it.h, p.hoverCol, false)
	}
	switch it.kind {
	case kText:
		gfx.Text(dst, it.text, float64(x), float64(y+it.ty), it.st)
	case kBar:
		gfx.Text(dst, it.text, float64(x), float64(y+it.ty), it.st)
		gfx.FillRect(dst, x+it.bx, y+it.by, it.bw, it.bh, it.track, false)
		gfx.FillRect(dst, x+it.bx, y+it.by, it.bw*it.frac, it.bh, it.fill, false)
		for j := 0; j < it.nticks; j++ {
			gfx.FillRect(dst, x+it.bx+it.bw*it.ticks[j]-1, y+it.by-1, 2, it.bh+2, it.tick, false)
		}
		gfx.Text(dst, it.value, float64(x+it.w), float64(y+it.ty), it.vst)
	case kCell:
		gfx.FillRect(dst, x, y+1, it.w, it.h-2, it.track, false)
		gfx.FillRect(dst, x, y+1, it.w*it.frac, it.h-2, it.fill, false)
		if it.passIc != nil {
			gfx.Icon(dst, it.passIc, float64(x+1), float64(y+1), float64(it.h-2))
		}
		gfx.Text(dst, it.text, float64(x+it.lblX+2), float64(y+it.ty), it.st)
		gfx.Text(dst, it.value, float64(x+it.w-2), float64(y+it.ty), it.vst)
	case kPick:
		gfx.FillRect(dst, x, y, it.w, it.h, it.back, false)
		gfx.Text(dst, it.text, float64(x+4), float64(y+it.ty), it.st)
	case kRule:
		gfx.FillRect(dst, x, y+it.h/2, it.w, 1, it.track, false)
	case kMarker:
		gfx.StrokeRect(dst, x, y, it.w, it.h, 1, it.back, false)
		gfx.Text(dst, it.text, float64(x+3), float64(y+it.ty), it.st)
	}
}

func (p *Panel) drawGear(dst *ebiten.Image, g gfx.Rect) {
	gfx.FillRect(dst, g.X, g.Y, g.W, g.H, p.gearBack, false)
	if p.gearImg != nil {
		gfx.Icon(dst, p.gearImg, float64(g.X+1), float64(g.Y+1), gearSize-2)
	}
}
