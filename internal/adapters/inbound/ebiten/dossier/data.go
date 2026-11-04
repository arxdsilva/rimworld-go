package dossier

import (
	"image/color"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/platform/config"
)

// data is the selection's facts as of the last refresh.
type data struct {
	animal bool
	p      game.PawnView
	a      game.AnimalView
	card   dossierview.Card
	bio    game.BioView
	health game.HealthView
	pris   game.PrisonerView
	who    sheet.Who

	tint     color.RGBA
	titleTip string
	titleTab string
}

// settingsSig is what the layouts depend on in the settings; a change drops
// them. The colours are compared by count here and in full at each refresh.
type settingsSig struct {
	enabled, restyle bool
	text             config.HUDText
	dock, flt        *config.HUDNode
	ncolors          int
	inv              uint64
}

func sigOf(h config.HUD, inv uint64) settingsSig {
	return settingsSig{enabled: h.Enabled, restyle: h.Pane.Restyle, text: h.Text,
		dock: h.Layouts.Docked, flt: h.Layouts.Floating, ncolors: len(h.Colors), inv: inv}
}

// now is the Update counter.
func (p *Panel) now() uint64 {
	if p.k == nil || !p.k.Bound() {
		return 0
	}
	return p.k.Host().Updates()
}

// sync reads the facts again when the refresh gate is due, the selection or the
// settings changed; it reports the card has something to show.
func (p *Panel) sync(hud config.HUD) bool {
	gate := uint64(max(hud.RefreshTenths, 1)) * updatesPerTenth
	if p.have && sigOf(hud, p.inv) == p.sig && p.now()-p.last < gate {
		return true
	}
	p.refresh(hud)
	return p.have
}

// refresh reads the selection, the palette and the title; it drops the
// layouts, which are built again from the new facts on demand.
func (p *Panel) refresh(hud config.HUD) {
	p.sig, p.last = sigOf(hud, p.inv), p.now()
	p.lay = [2]layout{}
	p.palette(hud)
	p.have = p.fetch()
	if p.have {
		p.title()
	}
}

func (p *Panel) fetch() bool {
	p.d = data{}
	d := &p.d
	switch p.kind {
	case selection.Pawn:
		pv, ok := p.r.Pawn(p.id)
		if !ok {
			return false
		}
		d.p = pv
		d.who = sheet.WhoOf(pv)
		d.bio, _ = p.r.Bio(p.id)
		d.health, _ = p.r.Health(p.id)
		if pv.Prisoner {
			d.pris, _ = p.r.Prisoner(p.id)
		}
	case selection.Animal:
		av, ok := p.r.Animal(p.id)
		if !ok {
			return false
		}
		d.animal, d.a = true, av
		d.who = sheet.WhoOfAnimal(av)
	default:
		return false
	}
	var ok bool
	if d.card, ok = p.r.Card(p.id); !ok {
		d.card.ActiveSkill, d.card.WorkType = -1, -1
	}
	return true
}

func (p *Panel) title() {
	d := &p.d
	if d.animal {
		d.tint = p.pal[sheet.RelationKey(sheet.AnimalRelation(d.a))]
		d.titleTip = sheet.JoinTip(sheet.AnimalTip(d.a))
		if d.a.Tame {
			d.titleTab = sheet.TabTraining
		}
		return
	}
	d.tint = p.pal[sheet.RelationKey(sheet.RelationOf(d.p, d.card))]
	d.titleTip = sheet.JoinTip(sheet.BioTip(d.p, d.bio, d.card))
	d.titleTab = sheet.TabSocial
}

// palette resolves every card colour: the player's override, else the theme.
func (p *Panel) palette(hud config.HUD) {
	clear(p.pal)
	defer p.finishPalette(hud)
	for _, key := range uikit.DossierColorKeys() {
		c, _ := uikit.DossierColor(key)
		if s, ok := hud.Colors[key]; ok {
			if o, ok := sheet.ParseColor(s); ok {
				c = o
			}
		}
		p.pal[key] = c
	}
}

// finishPalette derives the colours the draw loop reads boxed.
func (p *Panel) finishPalette(hud config.HUD) {
	h := uikit.Fade(p.pal["rule"], 70)
	p.hoverCol = h
	p.gearBack = p.pal["picker_back"]
	p.gearImg = p.icon("ui_icon/dossier_gear")
	p.bodySize = hud.Text.BodySize
}
