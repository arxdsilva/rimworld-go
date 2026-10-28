package learn

import (
	"math"
	"slices"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// The learning helper (PLAY-018): a collapsible top-right panel in the
// layout's Learn rect, in the warm tutor colours. Expanded it lists the
// next three unfinished lessons (lessons.go); clicking one opens its text
// in a detail card left of the panel, with a "Got it" button. Progress
// (Gameplay.LearnedLessons), the collapse and the on/off switch live in the
// settings and are saved through Host.Save.
//
// Lessons are done by what the player does: commands arrive through
// Host.Chain (hooked on the first Update of a colony), UI events are read
// each Update (UI): the selection and the speed from Host.Sense, the camera
// from the player's pan and zoom input. The panel hides itself while the
// setting is off and once every lesson is done; Options > Gameplay turns it
// back on and resets the lessons.

const (
	learnHeaderH  = hudlayout.LearnHeaderH
	learnRowH     = 44
	learnRowGap   = 4
	learnFooterH  = 20
	learnPanelPad = 4
	learnShown    = 3 // lessons listed at once

	learnRowW   = hudlayout.RightW - 2*learnPanelPad // a lesson row
	learnRowPad = 10                                 // text inset in a row (after its accent bar)

	learnCardW   = 280 // the detail card, at most
	learnCardPad = 12
	learnGotItW  = 96
	learnGotItH  = 28

	// HeaderH is the folded panel's height.
	HeaderH = learnHeaderH
	// RowW is a lesson row's width.
	RowW = learnRowW
	// RowPad is the text inset in a row (after its accent bar).
	RowPad = learnRowPad
	// PanFrames is the frames of pan input (0.4 s held) that finish the camera lesson.
	PanFrames = 24
	// ZoomSteps is the wheel notches or zoom keys that finish it too.
	ZoomSteps = 2
	// FlashFrames is how long a lesson done on its own is highlighted.
	FlashFrames = 150
)

// State is the helper's presentation state.
type State struct {
	hooked any  // the colony the command observer is installed on
	memo   Memo // earlier commands, for two-step lessons

	// what the UI-event lessons compare against, taken when the helper
	// starts looking (a new colony, the helper turned on, a reset)
	based      bool
	lastSel    string
	lastSpeed  int
	panFrames  float64 // camera: frames of pan input so far
	zoomSteps  float64 // camera: wheel notches and zoom keys so far
	nLearned   int     // lessons done at the last Update: a drop is a reset
	open       string  // the lesson whose card is up
	flashID    string  // the lesson just done on its own
	flashUntil uint64  // Host.Updates when its highlight ends
}

// Helper is the feature's one type (ARCH-005).
type Helper struct {
	sh Shell
	st State
}

// New builds the feature over its shell.
func New(sh Shell) *Helper { return &Helper{sh: sh} }

// Open is the lesson whose card is up ("" none).
func (h *Helper) Open() string { return h.st.open }

// SetBaseSpeed re-bases the speed lesson on tier once the helper looks (a
// pause the root forced is not the player's doing).
func (h *Helper) SetBaseSpeed(tier int) {
	if h.st.based {
		h.st.lastSpeed = tier
	}
}

// ------------------------------------------------------------ progress

func (h *Helper) learned(id string) bool {
	return slices.Contains(h.sh.Settings().Gameplay.LearnedLessons, id)
}

// count is the number of lessons not done yet.
func (h *Helper) count() int {
	n := 0
	for i := range Catalog {
		if !h.learned(Catalog[i].ID) {
			n++
		}
	}
	return n
}

// left is the lessons not done yet, in order.
func (h *Helper) left() []*Lesson {
	out := make([]*Lesson, 0, len(Catalog))
	for i := range Catalog {
		if !h.learned(Catalog[i].ID) {
			out = append(out, &Catalog[i])
		}
	}
	return out
}

// visible reports the helper is on screen: a colony runs, the setting is
// on and a lesson is left.
func (h *Helper) visible() bool {
	colony, _ := h.sh.Colony()
	return colony != nil && h.sh.Settings().Gameplay.ShowLearningHelper && h.count() > 0
}

// Tracking reports the player's actions count: the helper is visible and
// this is not a scripted capture.
func (h *Helper) Tracking() bool {
	_, scripted := h.sh.Colony()
	return !scripted && h.visible()
}

// save stores the helper's progress or fold through the settings save
// hook. It does not re-apply the settings: progress changes nothing of
// that, and it happens in the middle of play.
func (h *Helper) save(s config.Settings) {
	if err := h.sh.Save(s); err != nil {
		h.sh.Hint("settings not saved: " + err.Error())
	}
}

// Complete marks a lesson done and saves it. flash highlights it briefly
// (a lesson done on its own; "Got it" is the player's own word).
func (h *Helper) Complete(id string, flash bool) {
	if ByID(id) == nil || h.learned(id) {
		return
	}
	s := h.sh.Settings()
	s.Gameplay.LearnedLessons = append(slices.Clone(s.Gameplay.LearnedLessons), id)
	h.save(s)
	st := &h.st
	st.nLearned = len(h.sh.Settings().Gameplay.LearnedLessons)
	if st.open == id {
		st.open = ""
	}
	if flash {
		st.flashID, st.flashUntil = id, h.sh.Updates()+FlashFrames
	}
}

// Flash is the title of the lesson just done on its own, while it is
// highlighted.
func (h *Helper) Flash() (string, bool) {
	st := &h.st
	if st.flashID == "" || h.sh.Updates() >= st.flashUntil {
		return "", false
	}
	if l := ByID(st.flashID); l != nil {
		return l.Title, true
	}
	return "", false
}

// Observe sees every command the player sends.
func (h *Helper) Observe(c command.Command) {
	if !h.Tracking() {
		return
	}
	for _, l := range h.left() {
		if l.OnCmd != nil && l.OnCmd(&h.st.memo, c) {
			h.Complete(l.ID, true)
		}
	}
}

// hook installs Observe on the colony, once per colony.
func (h *Helper) hook() {
	colony, _ := h.sh.Colony()
	if colony == nil || h.st.hooked == colony {
		return
	}
	h.st.hooked = colony
	h.sh.Chain(h.Observe)
	h.st.rebase()
}

// rebase makes the helper start counting from now.
func (st *State) rebase() {
	st.based, st.memo, st.panFrames, st.zoomSteps = false, Memo{}, 0, 0
}

// UI runs once per Update on the colony (the widget-kit hook): it hooks the
// colony, notices a reset and reads the UI events the lessons finish on.
func (h *Helper) UI(sw, sh int) {
	st := &h.st
	if colony, _ := h.sh.Colony(); colony == nil {
		return
	}
	h.hook()
	if n := len(h.sh.Settings().Gameplay.LearnedLessons); n < st.nLearned {
		st.rebase() // lessons were reset: count from now
	}
	st.nLearned = len(h.sh.Settings().Gameplay.LearnedLessons)
	if !h.Tracking() {
		st.rebase()
		st.open = ""
		return
	}
	h.watch()
}

// cameraInput is this Update's camera input: whether a pan control is in
// use (a pan key held, a middle-button drag, edge scrolling) and how many
// zoom steps were taken (wheel notches, zoom keys). The lesson reads the
// player's input, not the camera, so a window resize, a UI scale change or
// a jump to an alert never counts as a pan.
func (h *Helper) cameraInput(sensed Sensed) (pan, zoom float64) {
	f := h.sh.Frame()
	if f.Top() == input.CtxText {
		return 0, 0
	}
	if f.Held(input.CamLeft) || f.Held(input.CamRight) || f.Held(input.CamUp) || f.Held(input.CamDown) || f.Down(inputframe.Middle) {
		pan = 1
	}
	if sensed.EdgePan {
		pan = 1
	}
	if f.Fired(input.ZoomIn) || f.Fired(input.ZoomOut) {
		zoom++
	}
	if lx, ly := h.sh.Cursor(); f.Raw.WheelY != 0 && !h.sh.Kit().WheelOver(lx, ly) { // a scroll view under the cursor owns the wheel
		zoom += math.Abs(f.Raw.WheelY)
	}
	return pan, zoom
}

// watch compares the camera input, the selection and the speed with what
// the helper saw before and finishes the lessons that ask for a change.
func (h *Helper) watch() {
	st := &h.st
	sensed := h.sh.Sense()
	if !st.based {
		st.lastSel, st.lastSpeed, st.based = sensed.Selection, sensed.Speed, true
		return
	}
	pan, zoom := h.cameraInput(sensed)
	st.panFrames += pan
	st.zoomSteps += zoom
	var f Frame
	f.Panned = st.panFrames >= PanFrames || st.zoomSteps >= ZoomSteps
	f.Selected = sensed.Selection != "" && sensed.Selection != st.lastSel
	st.lastSel = sensed.Selection
	// the game pausing itself on a threat is not the player's doing
	f.SpeedSet = sensed.Speed != st.lastSpeed && !(sensed.Speed == 0 && sensed.ThreatPaused)
	st.lastSpeed = sensed.Speed
	for _, l := range h.left() {
		if l.OnUI != nil && l.OnUI(f) {
			h.Complete(l.ID, true)
		}
	}
}

// ApplyClick handles a "learn." click.
func (h *Helper) ApplyClick(id string) {
	st := &h.st
	switch {
	case id == "learn.toggle":
		s := h.sh.Settings()
		s.Gameplay.LearnCollapsed = !s.Gameplay.LearnCollapsed
		st.open = ""
		h.save(s)
	case strings.HasPrefix(id, "learn.lesson."):
		lid := strings.TrimPrefix(id, "learn.lesson.")
		switch {
		case ByID(lid) == nil || h.learned(lid):
		case st.open == lid:
			st.open = "" // the open lesson again: close its card
		default:
			st.open = lid
		}
	case strings.HasPrefix(id, "learn.done."):
		h.Complete(strings.TrimPrefix(id, "learn.done."), false)
	}
}

// CloseCard closes the detail card and reports there was one up (the
// colony's Escape handling can take it before anything else).
func (h *Helper) CloseCard() bool {
	shown := len(h.Chrome()) > 0
	h.st.open = ""
	return shown
}
