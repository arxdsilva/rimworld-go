package dossier

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
)

// updatesPerTenth is how many Updates make a tenth of a second (60 a second).
const updatesPerTenth = 6

// hoverState is the item under the cursor and its card.
type hoverState struct {
	hov      int // the hovered item, -1 none
	hovSince uint64
	tip      tipCard
	tipPaint func(*ebiten.Image)
}

// look is the resolved colours and sizes the draw loop reads.
type look struct {
	pal      map[string]color.RGBA
	hoverCol color.Color // the cursor's row
	gearBack color.Color
	gearImg  *ebiten.Image
	bodySize int
}

// Panel is the colonist card: it draws and declares the player's layout tree
// for one selected pawn or animal, docked in the inspector pane or floating.
// It satisfies inspector.PaneBody (Claims, Declare, Draw, Title) and adds the
// floating card's DeclareFloating and DrawFloating.
type Panel struct {
	r  Reader
	a  Actions
	pf Prefs
	sz ports.Sizer
	k  *uikit.Kit
	ic *art.Catalog
	sc Scene

	kind selection.Kind
	id   entity.EntityID

	d      data        // the selection's facts, as of the last refresh
	have   bool        // d holds the selection
	last   uint64      // the Update of the last refresh
	sig    settingsSig // the settings the layouts were built from
	inv    uint64      // Invalidate calls
	lay    [2]layout   // docked, floating
	warned map[string]bool
	defs   [2]config.HUDNode // the built-in trees: docked, floating

	hoverState
	look
	setupFn func()
	onSetup func()
}

// New builds the card. r reads the selection, a acts, p holds the settings
// (HUD, the refresh rate and colours), s is the logical screen and pointer, k
// the widget kit the card declares into, ic the art for the badges (nil draws
// none) and sc the allowed area, food policies and hour for the selectors.
func New(r Reader, a Actions, p Prefs, s ports.Sizer, k *uikit.Kit, ic *art.Catalog, sc Scene) *Panel {
	pn := &Panel{r: r, a: a, pf: p, sz: s, k: k, ic: ic, sc: sc, hoverState: hoverState{hov: -1}, look: look{pal: map[string]color.RGBA{}}, warned: map[string]bool{},
		defs: [2]config.HUDNode{sheet.DefaultDocked(), sheet.DefaultFloating()}}
	pn.tipPaint, pn.setupFn = tipper{pn}.paintTip, pn.openSetup
	return pn
}

// SetOnSetup sets what the gear opens: the setup modal (DOS-5).
func (p *Panel) SetOnSetup(f func()) { p.onSetup = f }

// Select tells the card what is selected. DOS-4 calls it when the selection
// changes, before Declare. A different id drops the cache at once.
func (p *Panel) Select(k selection.Kind, id entity.EntityID) {
	if k == p.kind && id == p.id {
		return
	}
	p.kind, p.id = k, id
	p.have = false
	p.dropHover()
}

// Reset forgets the selection and every cache (a map switch, the return to
// the menu).
func (p *Panel) Reset() {
	p.kind, p.id, p.have = selection.None, 0, false
	p.d = data{}
	p.lay = [2]layout{}
	p.dropHover()
}

// Invalidate makes the next Declare read the facts and the settings again
// (the setup modal calls it after an edit).
func (p *Panel) Invalidate() { p.inv++ }

// Claims reports the docked pane body is the card for this selection: the
// card is enabled, restyles the pane and the selection is a pawn or animal
// the reader knows.
func (p *Panel) Claims(k selection.Kind, id entity.EntityID) bool {
	hud := p.pf.Settings().HUD
	if !hud.Enabled || !hud.Pane.Restyle || id == 0 {
		return false
	}
	p.Select(k, id)
	return p.ready(hud)
}

// ready reports the selection exists, reading it when it is new.
func (p *Panel) ready(hud config.HUD) bool {
	switch p.kind {
	case selection.Pawn, selection.Animal:
	default:
		return false
	}
	if !p.have {
		p.refresh(hud)
	}
	return p.have
}

// Title is the pane's title tint, its tooltip and the inspector tab the name
// opens.
func (p *Panel) Title(k selection.Kind, id entity.EntityID) (tint color.RGBA, tip, tab string) {
	p.Select(k, id)
	hud := p.pf.Settings().HUD
	if !p.ready(hud) {
		return color.RGBA{}, "", ""
	}
	return p.d.tint, p.d.titleTip, p.d.titleTab
}

// Declare registers the docked card's widgets in r (once per Update).
func (p *Panel) Declare(r gfx.Rect) { p.declare(r, true) }

// Draw paints the docked card in r.
func (p *Panel) Draw(dst *ebiten.Image, r gfx.Rect) { p.draw(dst, r, true) }

// DeclareFloating registers the floating card's widgets in r.
func (p *Panel) DeclareFloating(r gfx.Rect) { p.declare(r, false) }

// DrawFloating paints the floating card (background, margin and tree) in r.
func (p *Panel) DrawFloating(dst *ebiten.Image, r gfx.Rect) { p.draw(dst, r, false) }

// Height is the tree's height in logical pixels at width w for the current
// selection, docked or floating; DOS-4 sizes the pane and the card with it.
func (p *Panel) Height(docked bool, w float32) float32 {
	hud := p.pf.Settings().HUD
	if !p.ready(hud) {
		return 0
	}
	return p.measure(hud, docked, w)
}

// OpenStats opens the stat sheet of the selection (the header's info button).
func (p *Panel) OpenStats() {
	if p.have {
		statSheet{p}.open()
	}
}

func (p *Panel) openSetup() {
	if p.onSetup != nil {
		p.onSetup()
	}
}

func (p *Panel) dropHover() {
	p.hov, p.tip.on = -1, false
}
