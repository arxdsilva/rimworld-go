package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier"
	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
)

// The colonist card's place in the root: dossier.Panel is built here, once per
// session, over a host that fills its reader, actions, settings and scene. The
// inspector asks for it through inspector.CardHost. With HUD.Enabled off the
// panel exists and nothing asks it anything.

// floatFrameID is the floating card's backing widget: it keeps the wheel and
// the hover from the map behind the card.
const floatFrameID = "dossier.frame"

// CardFiles is where the card's presets live (nil Store and Open: none; the
// Presets tab then says why it cannot keep a look or show the folder).
type CardFiles struct {
	Store  dossier.PresetStore
	Open   dossier.FolderOpener
	Folder string
}

// dossierState is the App's hold on the card.
type dossierState struct {
	panel  *dossier.Panel
	setup  *dossier.Setup
	viewed game.MapID
	badge  cardBadge
}

// cardBadge is the header's emblem as of the last look, at the card's refresh
// rate: the faction list is not read every frame.
type cardBadge struct {
	id          entity.EntityID
	at          uint64
	emblem, tip string
}

// reset drops the card's caches; a state without a panel has none.
func (d dossierState) reset() {
	if d.panel != nil {
		d.panel.Reset()
	}
	if d.setup != nil {
		d.setup.Reset()
	}
}

// newDossier builds the card over this session's art.
func newDossier(a *App) dossierState {
	h := dossierHost{a}
	st := dossierState{panel: dossier.New(h, h, h, shellSize{a}, a.uiSt(), a.art, h)}
	cf := a.cfg.CardFiles
	st.setup = dossier.NewSetup(st.panel, a.uiSt(), h, h, cf.Store, cf.Open, cf.Folder, shellSize{a})
	st.panel.SetOnSetup(st.setup.Open)
	if r := a.renderer; r != nil && r.game != nil {
		st.viewed = r.game.ViewedMap()
	}
	return st
}

// dossierHost is the App in the shapes the card asks for: dossier.Reader,
// Actions, Prefs and Scene, and the floating card's frame hooks.
type dossierHost struct{ a *App }

var (
	_ dossier.Reader  = dossierHost{}
	_ dossier.Actions = dossierHost{}
	_ dossier.Prefs   = dossierHost{}
	_ dossier.Scene   = dossierHost{}
)

// ---- dossier.Reader

// Pawn looks a pawn up in the snapshot.
func (h dossierHost) Pawn(id entity.EntityID) (game.PawnView, bool) {
	for _, p := range h.a.shell.Pawns() {
		if p.ID == id {
			return p, true
		}
	}
	return game.PawnView{}, false
}

// Animal looks an animal up in the snapshot.
func (h dossierHost) Animal(id entity.EntityID) (game.AnimalView, bool) {
	for _, an := range h.a.shell.World().Animals {
		if an.ID == id {
			return an, true
		}
	}
	return game.AnimalView{}, false
}

// Card is the selection's dossier.
func (h dossierHost) Card(id entity.EntityID) (dossierview.Card, bool) {
	if g := h.sim(); g != nil {
		return g.Dossier(id)
	}
	return dossierview.Card{}, false
}

// Bio is the selection's background and skills.
func (h dossierHost) Bio(id entity.EntityID) (game.BioView, bool) {
	if g := h.sim(); g != nil {
		return g.Bio(id)
	}
	return game.BioView{}, false
}

// Health is the selection's health.
func (h dossierHost) Health(id entity.EntityID) (game.HealthView, bool) {
	if g := h.sim(); g != nil {
		return g.Health(id)
	}
	return game.HealthView{}, false
}

// Prisoner is the selection's prison record.
func (h dossierHost) Prisoner(id entity.EntityID) (game.PrisonerView, bool) {
	if g := h.sim(); g != nil {
		return g.Prisoner(id)
	}
	return game.PrisonerView{}, false
}

// Factions lists the factions.
func (h dossierHost) Factions() []game.FactionView {
	if g := h.sim(); g != nil {
		return g.Factions()
	}
	return nil
}

// Areas lists the allowed areas.
func (h dossierHost) Areas() []game.AreaView {
	if g := h.sim(); g != nil {
		return g.Areas()
	}
	return nil
}

// sim is the session's game, nil at the menu.
func (h dossierHost) sim() Simulation {
	if r := h.a.renderer; r != nil {
		return r.game
	}
	return nil
}

// ---- dossier.Actions

// Enqueue sends a command to the simulation.
func (h dossierHost) Enqueue(c command.Command) { h.a.shell.Enqueue(c) }

// OpenTab opens an inspector tab by its bare name.
func (h dossierHost) OpenTab(name string) { h.a.insp.OpenPaneTab(name) }

// ShowWindow opens a main tab's window.
func (h dossierHost) ShowWindow(tab string) { h.a.applyClick(tab) }

// AreaPicker opens the allowed area menu.
func (h dossierHost) AreaPicker(title string, cur int, base bool, pick func(ref int)) {
	h.a.arch.Areas.OpenPicker(title, cur, base, pick)
}

// Hint shows why a click did nothing.
func (h dossierHost) Hint(s string) { h.a.hint = s }

// T and Tf are the language catalog.
func (h dossierHost) T(src string) string { return h.a.shell.T(src) }

func (h dossierHost) Tf(src string, args ...any) string { return h.a.shell.Tf(src, args...) }

// ---- dossier.Prefs

// Settings are the player's settings.
func (h dossierHost) Settings() config.Settings { return h.a.settings }

// SetSettings stores them and has the card read them again.
func (h dossierHost) SetSettings(s config.Settings) {
	h.a.setSettings(s)
	if p := h.a.dos.panel; p != nil {
		p.Invalidate()
	}
}

// ---- dossier.Scene

// areaReader is the game's allowed area lookup; architect reads it the same way.
type areaReader interface {
	AreaOf(id entity.EntityID) int
}

// AreaOf is a colonist's or a tame animal's allowed area.
func (h dossierHost) AreaOf(id entity.EntityID) int {
	if rd, ok := h.sim().(areaReader); ok {
		if _, isPawn := h.Pawn(id); isPawn {
			return rd.AreaOf(id)
		}
	}
	if an, ok := h.Animal(id); ok {
		return an.AreaRef
	}
	return 0
}

// FoodPolicies are the colony's food policies.
func (h dossierHost) FoodPolicies() []game.FoodPolicyView { return h.a.shell.World().FoodPolicies }

// Hour is the hour of the colony's day.
func (h dossierHost) Hour() int { return int(h.a.shell.World().Hour) }

// ---- frame hooks

// followMap drops the card's caches when the viewed map changes (RHUD-503).
func (h dossierHost) followMap() {
	g := h.sim()
	if g == nil || h.a.dos.panel == nil {
		return
	}
	if m := g.ViewedMap(); m != h.a.dos.viewed {
		h.a.dos.viewed = m
		h.a.dos.reset()
	}
}

// pointerOnCard reports the pointer is over the card as shown now, in the pane
// or floating.
func (h dossierHost) pointerOnCard() bool {
	a := h.a
	if a.dos.panel == nil || !a.settings.HUD.Enabled || a.chrome.Hidden || a.world.Open() {
		return false
	}
	if _, _, ok := a.insp.Card().Target(); !ok {
		return false
	}
	l := a.hud()
	mx, my := a.currentCursor()
	return l.Card.Hit(mx, my) || (a.settings.HUD.Docked && l.Pane.Hit(mx, my))
}

// declareFloating declares the floating card: the backing widget first, then
// what the card holds over it, then a press on the blank parts of the card,
// which stops there and does not reach the map (RHUD-502).
func (h dossierHost) declareFloating() {
	a := h.a
	p := a.dos.panel
	if p == nil || a.world.Open() {
		return
	}
	l := a.hud()
	k, id, ok := a.insp.Card().Target()
	if !ok || l.Card.Empty() {
		return
	}
	u := a.uiSt()
	p.Select(k, id)
	live := u.Add(uikit.Widget{ID: floatFrameID, R: l.Card, Wheel: true})
	p.DeclareFloating(l.Card)
	if live {
		u.Press(l.Card)
	}
}

// drawFloating paints the floating card.
func (h dossierHost) drawFloating(dst *ebiten.Image) {
	a := h.a
	p := a.dos.panel
	if p == nil || a.chrome.Hidden || a.world.Open() {
		return
	}
	if _, _, ok := a.insp.Card().Target(); !ok {
		return
	}
	if l := a.hud(); !l.Card.Empty() {
		p.DrawFloating(dst, l.Card)
	}
}

// ---- inspector.CardHost, on the host the inspector already holds

var _ inspector.CardHost = inspectorHost{}

// CardHUD is the card's settings; docked, its sizes fitted to the logical
// screen's height (sheet.DockedFor), so a resize lays the pane out again.
func (h inspectorHost) CardHUD() config.HUD {
	hud := h.a.settings.HUD
	if hud.Docked {
		_, sh := h.a.size()
		hud = sheet.DockedFor(hud, sh)
	}
	return hud
}

// CardBody is the card, nil outside a session.
func (h inspectorHost) CardBody() inspector.PaneBody {
	if p := h.a.dos.panel; p != nil {
		return p
	}
	return nil
}

// CardBadge is the selection's faction emblem and its tooltip, looked up at
// the card's refresh rate.
func (h inspectorHost) CardBadge(_ selection.Kind, id entity.EntityID) (emblem, tip string) {
	a := h.a
	b := &a.dos.badge
	gate := uint64(max(a.settings.HUD.RefreshTenths, 1) * 6)
	if b.id == id && b.at != 0 && a.updates-b.at < gate {
		return b.emblem, b.tip
	}
	b.id, b.at, b.emblem, b.tip = id, max(a.updates, 1), "", ""
	host := dossierHost{a}
	c, ok := host.Card(id)
	if !ok {
		return "", ""
	}
	b.emblem, b.tip = c.FactionEmblem, badgeTip(c, host.Factions())
	return b.emblem, b.tip
}

// badgeTip words the emblem's tooltip: who, and how they feel about the colony.
func badgeTip(c dossierview.Card, factions []game.FactionView) string {
	if c.Faction == "" {
		return "Your colony"
	}
	var b strings.Builder
	b.WriteString(c.Faction)
	for i := range factions {
		if f := &factions[i]; f.Name == c.Faction {
			fmt.Fprintf(&b, "\n%s, goodwill %d", f.Relation, f.Goodwill)
			break
		}
	}
	return b.String()
}

// CardStats opens the selection's stat sheet.
func (h inspectorHost) CardStats() {
	if p := h.a.dos.panel; p != nil {
		p.OpenStats()
	}
}

// capture stages the card for -capture-ui: it turns the card on for this run
// (never saved) and selects a colonist, or an animal for dossier-animal.
func (h dossierHost) capture(ui string, pawns []game.PawnView, sw, sh int) {
	a := h.a
	hud := &a.settings.HUD
	hud.Enabled, hud.Pane.Restyle = true, true
	hud.Docked = ui == "dossier" || ui == "dossier-animal"
	hud.Pane.LogBody = ui == "dossier-log"
	a.cursor = &[2]int{-100, -100}
	if ui == "dossier-animal" {
		for _, an := range a.renderer.world().Animals {
			if an.Tame || !a.sel.HasAnimal {
				a.sel.Select(selection.Cand{Kind: selection.Animal, ID: an.ID})
				a.renderer.centerOn(float64(an.X)+4, float64(an.Y), sw, sh)
			}
			if an.Tame {
				break
			}
		}
		return
	}
	if len(pawns) > 0 {
		p := pawns[0]
		a.renderer.Select(p.ID)
		a.renderer.centerOn(float64(p.X)+4, float64(p.Y), sw, sh)
	}
	if ui == "dossier-log" {
		a.insp.OpenPaneTab("bio")
	}
}
