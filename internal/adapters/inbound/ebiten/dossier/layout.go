package dossier

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/platform/config"
)

// cardPad is the margin inside the floating card's background.
const cardPad = 6

// cardGutter is the space between children placed side by side: a right-aligned
// value must not touch the text of the next column.
const cardGutter = 8

// layout is the items of one mode (docked or floating) at one size.
type layout struct {
	ok     bool
	w, h   float32
	items  []item
	height float32
}

func modeIdx(docked bool) int {
	if docked {
		return 0
	}
	return 1
}

// tree is the player's layout for the mode, or the built-in one, cut to this
// target.
func (p *Panel) tree(hud config.HUD, docked bool) config.HUDNode {
	root := p.defs[modeIdx(docked)]
	if l := hud.Layouts.Floating; !docked && l != nil {
		root = *l
		p.warnBadLayout(root)
	}
	if l := hud.Layouts.Docked; docked && l != nil {
		root = *l
		p.warnBadLayout(root)
	}
	if hud.Layouts.Dropped && !p.warned[droppedLayoutKey] {
		p.warned[droppedLayoutKey] = true
		p.a.Hint("A saved card layout was too big to use, so the built-in one is back.")
	}
	return sheet.Prune(root, p.d.who)
}

// droppedLayoutKey marks the too-big warning as given (no element has this key).
const droppedLayoutKey = "\x00dropped"

// warnBadLayout says once what is wrong with a stored tree (RHUD-057). The card
// still draws it: what it can't read shows as a marker. An unknown element has
// its own warning, warnUnknown, with its name.
func (p *Panel) warnBadLayout(root config.HUDNode) {
	for _, bad := range sheet.Validate(root) {
		if strings.HasPrefix(bad.Text, "unknown element") || p.warned[bad.Text] {
			continue
		}
		p.warned[bad.Text] = true
		p.a.Hint("The saved card layout has a problem: " + bad.Text + ". The Sections tab or Back to defaults can fix it.")
		return
	}
}

// layoutFor is the mode's items at size w x h, built again only when the size
// or the facts changed.
func (p *Panel) layoutFor(hud config.HUD, docked bool, w, h float32) *layout {
	l := &p.lay[modeIdx(docked)]
	if l.ok && l.w == w && l.h == h {
		return l
	}
	b := p.builder(hud)
	root := p.tree(hud, docked)
	l.items = l.items[:0]
	b.items = l.items
	b.place(root, 0, 0, w, h)
	l.items = b.items
	l.height = sheet.MeasureWith(root, b.m, 0, b.empty)
	prefix := "dossier.f."
	if docked {
		prefix = "dossier.d."
	}
	for i := range l.items {
		it := &l.items[i]
		if it.tab != "" || it.act != actNone || it.tip != tipNone {
			it.id = prefix + strconv.Itoa(i)
		}
	}
	l.ok, l.w, l.h = true, w, h
	return l
}

// measure is the tree's height at width w.
func (p *Panel) measure(hud config.HUD, docked bool, w float32) float32 {
	if !p.sync(hud) {
		return 0
	}
	b := p.builder(hud)
	pad := float32(0)
	if !docked {
		pad = 2 * cardPad
	}
	return sheet.MeasureWith(p.tree(hud, docked), b.m, 0, b.empty) + pad
}

// empty reports a widget with nothing to show.
func (b *builder) empty(n config.HUDNode) bool {
	_, ok := b.widget(n, 0)
	return !ok
}

// place lays n out in the box (x, y, w, h), appending its items.
func (b *builder) place(n config.HUDNode, x, y, w, h float32) {
	if !sheet.IsContainer(n.Kind) {
		b.placeWidget(n, x, y, w)
		return
	}
	switch n.Kind {
	case sheet.KindStrip:
		b.placeAcross(n, x, y, w, h, false)
	case sheet.KindLine:
		b.placeAcross(n, x, y, w, h, true)
	default:
		hs := sheet.Heights(n, b.m, h, b.empty)
		for i, k := range n.Kids {
			if hs[i] > 0 {
				b.place(k, x, y, w, hs[i])
				y += hs[i]
			}
		}
	}
}

// placeAcross shares the width evenly. A strip skips children with nothing to
// show; a line keeps the slot of a blank so its columns hold still.
func (b *builder) placeAcross(n config.HUDNode, x, y, w, h float32, line bool) {
	var kids []config.HUDNode
	for _, k := range n.Kids {
		switch {
		case line && k.Kind == sheet.KeyGap && k.Def != "":
			kids = append(kids, k)
		case sheet.MeasureWith(k, b.m, 0, b.empty) > 0:
			kids = append(kids, k)
		}
	}
	if len(kids) == 0 {
		return
	}
	hs := sheet.Heights(n, b.m, h, b.empty)
	rowH := hs[0]
	cw := (w - cardGutter*float32(len(kids)-1)) / float32(len(kids))
	for i, k := range kids {
		b.place(k, x+float32(i)*(cw+cardGutter), y, cw, rowH)
	}
}

func (b *builder) placeWidget(n config.HUDNode, x, y, w float32) {
	if n.Kind == sheet.KeyGap {
		return
	}
	it, ok := b.widget(n, w)
	if !ok {
		return
	}
	it.x, it.y, it.w = x, y, w
	it.h = sheet.WidgetHeight(n.Kind, n.Def, b.m)
	if it.kind != kRule {
		it.ty = (it.h - float32(gfx.LineH(it.st))) / 2
	}
	b.items = append(b.items, it)
}

// icon is an art badge (nil without art).
func (p *Panel) icon(key string) *ebiten.Image {
	if p.ic == nil {
		return nil
	}
	return p.ic.Image(key, color.NRGBA{})
}
