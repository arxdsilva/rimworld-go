package ebiten

import (
	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/learn"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// learn.Helper owns the learning helper (ARCH-005, PLAY-018); learnHost is
// the root's side of learn.Host and newLearn builds the feature for
// NewApp. The aliases and one-line wrappers keep the root's other call
// sites unchanged until the sweep (SOLID_GO.md §11).

type learnHost struct{ a *App }

var _ learn.Host = learnHost{}

// learnShell is the feature's view of the App: the ports roles plus its Host.
type learnShell struct {
	shell
	learnHost
}

func newLearn(a *App) *learn.Helper {
	return learn.New(learnShell{a.shell, learnHost{a}})
}

type (
	learnLayout = learn.Layout
)

const (
	learnHeaderH     = learn.HeaderH
	learnRowW        = learn.RowW
	learnRowPad      = learn.RowPad
	learnPanFrames   = learn.PanFrames
	learnFlashFrames = learn.FlashFrames
)

var lessonCatalog = learn.Catalog

func lessonByID(id string) *learn.Lesson { return learn.ByID(id) }

// Colony is the running colony (nil on a menu screen) and whether a
// scripted capture runs it.
func (h learnHost) Colony() (any, bool) {
	if h.a.renderer == nil {
		return nil, h.a.cfg.Capture != nil
	}
	return h.a.renderer, h.a.cfg.Capture != nil
}

// Chain installs observe after the observer already on the colony.
func (h learnHost) Chain(observe func(command.Command)) {
	r := h.a.renderer
	if r == nil {
		return
	}
	prev := r.onEnqueue
	r.onEnqueue = func(c command.Command) {
		if prev != nil {
			prev(c)
		}
		observe(c)
	}
}

// Frame is this Update's input.
func (h learnHost) Frame() *inputFrame { return &h.a.in }

// Sense reads the colony state the UI-event lessons compare.
func (h learnHost) Sense() learn.Sensed {
	a, r := h.a, h.a.renderer
	if r == nil {
		return learn.Sensed{}
	}
	s := learn.Sensed{Selection: a.learnSelection(), Speed: int(r.speed), ThreatPaused: r.threatPaused}
	if r.edgeScroll && a.in.Raw.Focused {
		mx, my := a.mapCursor()
		if ex, ey := edgePan(mx, my, a.mapViewport()); ex != 0 || ey != 0 {
			s.EdgePan = true
		}
	}
	return s
}

// Layout is this frame's HUD layout.
func (h learnHost) Layout() hudlayout.Layout { return h.a.hud() }

// Updates counts the Updates run.
func (h learnHost) Updates() uint64 { return h.a.updates }

// Save stores settings as setSettings does, without re-applying them:
// progress changes nothing of that, and it happens in the middle of play.
func (h learnHost) Save(s config.Settings) error {
	a := h.a
	a.settings = s.Normalize()
	if a.cfg.Settings.Save != nil {
		return a.cfg.Settings.Save(a.settings)
	}
	return nil
}

// keyName is an action's first key for lesson text ("its key" when unbound).
func (a *App) keyName(act input.Action) string {
	if l := a.bindings().Label(act); l != "" {
		return l
	}
	return "its key"
}

// learnSelection names what is selected ("" for nothing) so a change of
// selection can be told from a stale one.
func (a *App) learnSelection() string {
	r := a.renderer
	switch {
	case a.grouped():
		return fmt.Sprintf("g%d:%v", a.sel.Group.Kind, a.sel.Group.Ids)
	case r.HasSelect():
		return fmt.Sprintf("p%d", r.Selected())
	case a.sel.HasBuilding:
		return fmt.Sprintf("b%d", a.sel.SelBuilding)
	case a.sel.HasAnimal:
		return fmt.Sprintf("a%d", a.sel.SelAnimal)
	case a.sel.HasPlant:
		return fmt.Sprintf("l%d", a.sel.SelPlant)
	case a.sel.HasItem:
		return fmt.Sprintf("i%d", a.sel.SelItem)
	case a.sel.HasFrame:
		return fmt.Sprintf("f%d", a.sel.SelFrame)
	case a.sel.HasZone:
		return fmt.Sprintf("z%d", a.sel.ZoneSel)
	}
	return ""
}

func (a *App) learnButtons() []button              { return a.lrn.Buttons() }
func (a *App) learnChrome() []rect                 { return a.lrn.Chrome() }
func (a *App) learnHeight() float32                { return a.lrn.Height() }
func (a *App) learnLayout() learn.Layout           { return a.lrn.Layout() }
func (a *App) learnFlash() (string, bool)          { return a.lrn.Flash() }
func (a *App) applyLearnClick(id string)           { a.lrn.ApplyClick(id) }
func (a *App) closeLearnCard() bool                { return a.lrn.CloseCard() }
func (a *App) drawLearn(screen *ebiten.Image)      { a.lrn.Draw(screen) }
func (a *App) learnObserve(c command.Command)      { a.lrn.Observe(c) }
func (a *App) learnUI(sw, sh int)                  { a.lrn.UI(sw, sh) }
func (a *App) learnComplete(id string, flash bool) { a.lrn.Complete(id, flash) }
func (a *App) learnFolded(p rect) bool             { return a.lrn.Folded(p) }
func (a *App) learnTracking() bool                 { return a.lrn.Tracking() }
func (a *App) learnCardLayout(l *learn.Layout, h hudLayout, row rect) {
	a.lrn.CardLayout(l, h, row)
}
