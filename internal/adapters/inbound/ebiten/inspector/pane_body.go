package inspector

import (
	"cmp"
	"slices"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
)

// The colonist card in the pane (RHUD): one pawn or one animal gets the card;
// groups, buildings, plants, items, zones and cell info keep today's pane in
// every mode. With HUD.Enabled off nothing here changes a Spec, so the pane
// behaves and looks as it did.

// CardHost is what the root adds to the Shell for the card, asserted on it as
// bedPrisonReader is on Sim: a shell without it shows today's pane.
type CardHost interface {
	// CardHUD is the card's settings as of now.
	CardHUD() config.HUD
	// CardBody is the card (nil off the colony).
	CardBody() PaneBody
	// CardBadge is the header's emblem and its tooltip ("": none).
	CardBadge(k selection.Kind, id entity.EntityID) (emblem, tip string)
	// CardStats opens the selection's stat sheet.
	CardStats()
}

// CardPane is the pane's side of the card; a view of the same Pane.
type CardPane struct{ *Pane }

// Card is the pane's side of the colonist card.
func (a *Pane) Card() CardPane { return CardPane{a} }

// host is the card's host when the card is on.
func (c CardPane) host() (CardHost, config.HUD, bool) {
	h, ok := c.shell.(CardHost)
	if !ok {
		return nil, config.HUD{}, false
	}
	hud := h.CardHUD()
	return h, hud, hud.Enabled
}

// cardKind reports the card can show a selection of kind k.
func cardKind(k selection.Kind) bool { return k == selection.Pawn || k == selection.Animal }

// dress turns the producer's spec into the restyled pane's: the tinted name
// and its tooltip and click, the emblem, the fixed tabs, and the card or the
// log in the body. Other selections pass through.
func (c CardPane) dress(k selection.Kind, id entity.EntityID, spec Spec) Spec {
	if spec.Title == "" || !cardKind(k) {
		return spec
	}
	h, hud, on := c.host()
	if !on || !hud.Pane.Restyle {
		return spec
	}
	body := h.CardBody()
	if body == nil {
		return spec
	}
	spec.Restyled, spec.Kind, spec.ID = true, k, id
	spec.TabW = float32(hud.Pane.TabW)
	spec.TitleColor, spec.TitleTip, spec.TitleTab = body.Title(k, id)
	if hud.Pane.FactionIcon {
		spec.Emblem, spec.EmblemTip = h.CardBadge(k, id)
	}
	switch {
	case hud.Docked && body.Claims(k, id):
		spec.Card = true
		spec.Subtitle, spec.Bars, spec.Chips, spec.Lines, spec.Rows = "", nil, nil, nil, nil
	case !hud.Docked && hud.Pane.LogBody && k == selection.Pawn:
		spec.LogBody = true
		spec.Lines, spec.Rows = nil, nil
	}
	return spec
}

// Target is the one pawn or animal the shown pane is about (a stale id has no
// spec, so no target).
func (c CardPane) Target() (selection.Kind, entity.EntityID, bool) {
	spec, ok := c.shownPane()
	if !ok || c.sel.Grouped() {
		return selection.None, 0, false
	}
	if spec.Restyled {
		return spec.Kind, spec.ID, true
	}
	if id, has := c.shell.Selected(); has {
		return selection.Pawn, id, true
	}
	if c.sel.HasAnimal {
		return selection.Animal, c.sel.SelAnimal, true
	}
	return selection.None, 0, false
}

// Size is the pane's size for the layout (0, 0: today's): as wide as its tabs
// on the restyled pane (RHUD-004, 006), and as tall as the setting while the
// card is its body (RHUD-003).
func (c CardPane) Size() (w, h float32) {
	_, hud, on := c.host()
	if !on {
		return 0, 0
	}
	spec, ok := c.shownPane()
	if !ok || !spec.Restyled {
		return 0, 0
	}
	w = float32(hud.Pane.TabW * max(hud.Pane.MinTabs, len(spec.Tabs)))
	if spec.Card {
		h = float32(hud.Pane.Height)
	}
	return w, h
}

// FloatRect is where the floating card sits, empty while docked, off, or for
// a selection the card does not show.
func (c CardPane) FloatRect(sw, sh int) gfx.Rect {
	_, hud, on := c.host()
	if !on || hud.Docked {
		return gfx.Rect{}
	}
	if _, _, ok := c.Target(); !ok {
		return gfx.Rect{}
	}
	cd := hud.Card
	return hudlayout.CardRect(sw, sh, cd.Anchor, cd.OffsetX, cd.OffsetY, cd.W, cd.H)
}

// drawBody draws what the restyled pane holds in place of the plain body, and
// reports it drew.
func (c CardPane) drawBody(dst *ebiten.Image, spec Spec, g Geom) bool {
	switch {
	case spec.Card:
		if h, _, on := c.host(); on {
			if b := h.CardBody(); b != nil {
				b.Draw(dst, g.Card)
			}
		}
		return true
	case spec.LogBody:
		if p, ok := c.pawnView(spec.ID); ok && !g.Body.Empty() {
			p.Log = c.Pawn().mergedLog(p)
			c.Pawn().DrawPawnLogRows(dst, p, g.Body)
		}
		return true
	}
	return false
}

// declareBody declares the card's widgets in the pane's body.
func (c CardPane) declareBody(spec Spec, g Geom) {
	if !spec.Card {
		return
	}
	if h, _, on := c.host(); on {
		if b := h.CardBody(); b != nil {
			b.Declare(g.Card)
		}
	}
}

// logBodyRect is the pane body the log fills, when it does.
func (c CardPane) logBodyRect() (gfx.Rect, bool) {
	spec, ok := c.shownPane()
	if !ok || !spec.LogBody {
		return gfx.Rect{}, false
	}
	l := c.shell.Layout()
	if l.Pane.Empty() {
		return gfx.Rect{}, false
	}
	return PaneLayout(l.Pane, spec).Body, true
}

// mergedLogCap is how many lines the log body keeps, newest first.
const mergedLogCap = 60

// mergedLog is p's own lines and its social ones by tick, newest first, cut
// to mergedLogCap (RHUD-027). Both views are cached per tick.
func (a Pawn) mergedLog(p game.PawnView) []game.LogLineView {
	var social []game.SocialLogLineView
	if sim := a.sim(); sim != nil {
		if sv, ok := sim.Social(p.ID); ok {
			social = sv.Log
		}
	}
	return mergeLog(p.Log, social)
}

// mergeLog merges the two logs, newest first; on a tie the pawn's own line
// comes first.
func mergeLog(own []game.LogLineView, social []game.SocialLogLineView) []game.LogLineView {
	out := slices.Clone(own)
	for _, l := range social {
		out = append(out, socialLogLine(l))
	}
	slices.SortStableFunc(out, func(x, y game.LogLineView) int { return cmp.Compare(y.Tick, x.Tick) })
	return out[:min(len(out), mergedLogCap)]
}

// socialLogLine is a social line in the log's form.
func socialLogLine(l game.SocialLogLineView) game.LogLineView {
	d := clock.DateAt(clock.Tick(l.Tick))
	return game.LogLineView{Tick: l.Tick, Day: d.Day + 1, Hour: d.Hour, Text: l.Text}
}
