package dossier

import (
	"maps"
	"reflect"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
)

// The setup modal (RHUD-054): one dialog with the tabs Look, Sections, Presets
// and Colours. It holds no copy of the settings: every control reads them
// through Prefs and every change goes through Edit, which applies it to a copy,
// stores it and has the card read it again. Only the modal's own state (the
// tab, scroll offsets, the colour being edited, the text being typed) lives
// here. Escape and the X close it through the kit's modal stack.

// Setup tabs, in order.
const (
	tabLook = iota
	tabSections
	tabPresets
	tabColours
)

// Setup modal geometry (logical pixels).
const (
	setupW      = 640
	setupH      = 560
	setupTabsH  = 28
	setupTabGap = 8
)

const setupID = "dossier.setup"

var setupTabLabels = []string{"Look", "Sections", "Presets", "Colours"}

// Setup is the setup modal. DOS-5 owns this file, setup_look.go and
// setup_colours.go; DOS-6 owns the Sections and Presets tabs and reads the
// fields p, k, pr, a, store, open, folder and sz, and the helpers HUD and Edit.
type Setup struct {
	p      *Panel
	k      *uikit.Kit
	pr     Prefs
	a      Actions
	store  PresetStore
	open   FolderOpener
	folder string
	sz     ports.Sizer

	sections *Sections
	presets  *Presets

	tab    int
	bufs   map[string]*string // text being typed, by field id
	typing map[string]bool    // fields that were being edited last frame
	lookH  float32            // the Look tab's content height, as of the last frame
	col    colourTab
}

// NewSetup builds the modal over the card p. store and open may be nil (the
// Presets tab then says why it cannot save or open the folder); folder is the
// presets folder shown to the player.
func NewSetup(p *Panel, k *uikit.Kit, pr Prefs, a Actions, store PresetStore, open FolderOpener, folder string, s ports.Sizer) *Setup {
	su := &Setup{p: p, k: k, pr: pr, a: a, store: store, open: open, folder: folder, sz: s, bufs: map[string]*string{}, typing: map[string]bool{}}
	su.sections, su.presets = newSections(su), newPresets(su)
	return su
}

// Kit is the widget kit the modal declares into (uikit.Owner, for the form rows).
func (s *Setup) Kit() *uikit.Kit { return s.k }

// HUD is the card's settings now.
func (s *Setup) HUD() config.HUD { return s.pr.Settings().HUD }

// Edit applies f to a copy of the settings, stores the result and has the card
// read it again. The colour map is copied first, so f may change it. When f
// leaves HUD.Preset as it was, Edit clears it: an edited card is no longer the
// preset it was loaded from (a preset load sets the name).
func (s *Setup) Edit(f func(h *config.HUD)) {
	st := s.pr.Settings()
	h := st.HUD
	h.Colors = maps.Clone(h.Colors)
	was := h.Preset
	f(&h)
	if h.Preset == was {
		h.Preset = ""
	}
	if len(h.Colors) == 0 {
		h.Colors = nil
	}
	st.HUD = h
	s.pr.SetSettings(st)
	s.p.Invalidate()
}

// Open shows the modal (the card's gear).
func (s *Setup) Open() {
	s.k.Push(uikit.Modal{
		ID: setupID, Title: "Colonist card setup", W: setupW, H: setupH,
		Draw: s.draw,
		Buttons: []uikit.ModalButton{
			{ID: "close", Label: "Close", Style: uikit.BtnPrimary, Closes: true},
			{ID: "defaults", Label: "Back to defaults", Disabled: s.defaultsWhy, OnClick: s.askDefaults},
		},
	})
}

// OpenAt shows the modal on a tab (0 Look, 1 Sections, 2 Presets, 3 Colours).
func (s *Setup) OpenAt(tab int) {
	s.tab = max(0, min(len(setupTabLabels)-1, tab))
	s.Open()
}

// Showing reports the modal is up and whether it shows the Look tab or one of
// the two that edit the layout (the learning helper's lessons read it).
func (s *Setup) Showing() (open, look, layouts bool) {
	for i := range s.k.Modals {
		if s.k.Modals[i].ID == setupID {
			return true, s.tab == tabLook, s.tab == tabSections || s.tab == tabPresets
		}
	}
	return false, false, false
}

// Reset forgets the modal's own state (a new session).
func (s *Setup) Reset() {
	s.tab, s.lookH = tabLook, 0
	s.col = colourTab{}
	clear(s.bufs)
	clear(s.typing)
	s.k.CloseModal(setupID)
}

// defaults is the card's settings as a new game has them. The card stays on or
// off as the player left it: "back to defaults" must not make it vanish.
func (s *Setup) defaults() config.HUD {
	d := config.Defaults().HUD
	d.Enabled = s.HUD().Enabled
	return d
}

// Changed reports that the settings differ from the defaults (RHUD-056).
func (s *Setup) Changed() bool {
	h, d := s.HUD(), s.defaults()
	if len(h.Colors) == 0 {
		h.Colors = nil
	}
	h.Layouts.Dropped = false // a note about the file, not a setting
	return !reflect.DeepEqual(h, d)
}

// defaultsWhy is why "Back to defaults" is off ("" when it works).
func (s *Setup) defaultsWhy() string {
	if s.Changed() {
		return ""
	}
	return "Nothing differs from the defaults."
}

// askDefaults confirms, then puts every setting back.
func (s *Setup) askDefaults() {
	s.k.Confirm("dossier.setup.confirm", "Back to defaults",
		"Every setting of the colonist card goes back to how a new game starts: the look, the layouts, the colours and the chosen preset. Presets you saved stay.",
		"Back to defaults", true, func() {
			d := s.defaults()
			s.Edit(func(h *config.HUD) { *h = d })
			s.col = colourTab{}
			clear(s.bufs)
			clear(s.typing)
		})
}

// draw declares the modal's body: the tab strip, the "Changed" tag and the
// tab's content.
func (s *Setup) draw(body gfx.Rect) {
	bar := gfx.Rect{X: body.X, Y: body.Y, W: body.W, H: setupTabsH}
	s.k.Tabs(setupID+".tabs", bar, setupTabLabels, &s.tab, uikit.TabsOpts{Tips: []string{
		"Look\nSizes, places and text of the card.",
		"Sections\nWhat the card shows and where.",
		"Presets\nSave a card and load it again.",
		"Colours\nThe card's colours.",
	}})
	s.changedTag(bar)
	c := gfx.Rect{X: body.X, Y: bar.Bottom() + setupTabGap, W: body.W, H: body.H - setupTabsH - setupTabGap}
	switch s.tab {
	case tabLook:
		s.drawLook(c)
	case tabSections:
		s.sections.Declare(c)
		s.k.Queue(func(dst *ebiten.Image) { s.sections.Draw(dst, c) })
	case tabPresets:
		s.presets.Declare(c)
		s.k.Queue(func(dst *ebiten.Image) { s.presets.Draw(dst, c) })
	default:
		s.drawColours(c)
	}
}

// changedTag says whether the settings differ from the defaults, at the right
// of the tab strip.
func (s *Setup) changedTag(bar gfx.Rect) {
	text, col := "Same as the defaults", uikit.TextDim
	if s.Changed() {
		text, col = "Changed", uikit.Warning
	}
	st := gfx.WithColor(gfx.Label, col)
	st.Align = gfx.AlignRight
	s.k.Label(bar.Right(), bar.Y+(bar.H-float32(gfx.LineH(st)))/2, text, st)
}
