package saves

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/clock"
)

// Leaving the colony (SAV-005, SAV-013): Save & Quit, the unsaved-progress
// question, and the pause menu's save status.

// unsavedModal is the id of the unsaved-progress question; its buttons
// are modal.saves.unsaved.{save,leave,cancel}.
const unsavedModal = "saves.unsaved"

// Leave runs proceed once the colony may be left (Return to Main Menu,
// Quit, an in-game Load). With no colony to save, a capture, or nothing
// unsaved it proceeds at once. A commitment colony saves itself first and
// stays when that fails. A reload-anytime colony with unsaved progress
// asks: Save & leave, Leave, or Cancel; the answer's proceed runs at the
// next PollSaves.
func (b *Book) Leave(proceed func()) {
	env := b.sh.SaveEnv()
	b.progress.seen(int64(b.sh.Tick()))
	switch {
	case !env.CanSave || env.Capture || !b.progress.dirty():
		proceed()
	case env.Commitment:
		if b.SaveBeforeLeaving() {
			proceed()
		}
	default:
		b.askUnsaved(proceed)
	}
}

// SaveAndLeave is Save & Quit (to the menu or the desktop): it saves the
// colony's slot, waits for the write and only then proceeds; a failed save
// keeps the player in the colony with the hint. proceed is the leaving
// intent, which passes Leave again: the save just written leaves it
// nothing to save or ask.
func (b *Book) SaveAndLeave(proceed func()) {
	if b.saveAndWait() {
		proceed()
	}
}

// saveAndWait saves the colony's slot and waits for a background write;
// false when it failed (the hint says why) or saving is unavailable.
func (b *Book) saveAndWait() bool {
	if !b.sh.SaveEnv().CanSave {
		b.sh.Hint(b.sh.T("saving unavailable"))
		return false
	}
	return b.SaveNow(b.SaveSlot) && b.bg().wait(b.SaveSlot)
}

// askUnsaved opens the unsaved-progress question for proceed.
func (b *Book) askUnsaved(proceed func()) {
	const w = 460
	body := b.sh.Tf("Unsaved progress since %s.", b.dateAt(b.progress.savedAt)) + " " + b.sh.T("Leaving now loses it.")
	lines := gfx.WrapPx(body, gfx.Body, w-2*uikit.ModalPad)
	k := b.sh.Kit()
	h := float32(uikit.ModalTitleH+uikit.ModalFooterH+2*uikit.ModalPad) + float32(len(lines))*float32(gfx.LineH(gfx.Body))
	k.Push(uikit.Modal{
		ID: unsavedModal, Title: b.sh.T("Unsaved progress"), W: w, H: h,
		Draw: func(r gfx.Rect) {
			y := r.Y
			for _, l := range lines {
				k.Label(r.X, y, l, gfx.Body)
				y += float32(gfx.LineH(gfx.Body))
			}
		},
		Buttons: []uikit.ModalButton{ // right to left
			{ID: "save", Label: b.sh.T("Save & leave"), Style: uikit.BtnPrimary, Closes: true, OnClick: func() {
				if b.saveAndWait() {
					b.progress.due = proceed
				}
			}},
			{ID: "leave", Label: b.sh.T("Leave"), Style: uikit.BtnDanger, Closes: true, OnClick: func() { b.progress.due = proceed }},
			{ID: "cancel", Label: b.sh.T("Cancel"), Closes: true},
		},
	})
}

// runDue runs the leave the player confirmed, once (PollSaves).
func (b *Book) runDue() {
	if p := b.progress.due; p != nil {
		b.progress.due = nil
		p()
	}
}

// dateAt is tick as the question names it: "Day N, HH:00".
func (b *Book) dateAt(tick int64) string {
	return b.sh.Tf("Day %d, %02d:00", tick/clock.TicksPerDay+1, clock.DateAt(clock.Tick(tick)).Hour)
}

// DrawPauseStatus draws the colony's save status under the pause menu at
// x, y: "Saved N in-game hours ago", "Not saved yet", or "Saving…" while
// a background save is written. Nothing without a colony to save.
func (b *Book) DrawPauseStatus(dst *ebiten.Image, x, y float32) {
	if s := b.pauseStatus(); s != "" {
		gfx.Text(dst, s, float64(x), float64(y), gfx.WithColor(gfx.Small, uikit.TextDim))
	}
}

// pauseStatus is DrawPauseStatus's text.
func (b *Book) pauseStatus() string {
	if !b.sh.SaveEnv().CanSave {
		return ""
	}
	switch {
	case b.progress.saving():
		return b.sh.T("Saving…")
	case !b.progress.known:
		return b.sh.T("Not saved yet")
	}
	switch h := (int64(b.sh.Tick()) - b.progress.savedAt) / clock.TicksPerHour; {
	case h < 1:
		return b.sh.T("Saved less than an in-game hour ago")
	case h == 1:
		return b.sh.T("Saved 1 in-game hour ago")
	default:
		return b.sh.Tf("Saved %d in-game hours ago", h)
	}
}
