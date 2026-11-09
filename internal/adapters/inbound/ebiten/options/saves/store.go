package saves

import (
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
)

// Saved games. The book only sees saves through the Saver, so it never
// touches files or game internals.

// SaveNow writes the current colony to slot and reports success (the hint
// says what happened). With background saves (Env.AsyncSaves) it reports
// that the save started: PollSaves says how the write went. The header
// names the running colony (metaFor, session.go).
func (b *Book) SaveNow(slot string) bool { return write(b, metaFor(b, slot, false)) }

// write saves the running colony with header meta (SaveNow, the autosave,
// the Save dialog).
func write(b *Book, meta ports.SaveMeta) bool {
	slot := meta.Name
	env := b.sh.SaveEnv()
	if !env.CanSave {
		return false
	}
	at := int64(b.sh.Tick()) // the save holds the colony as of this tick (SAV-013)
	if err := b.sh.Save(meta); err != nil {
		b.sh.Hint(b.sh.Tf("save failed: %s", err.Error()))
		return false
	}
	if env.AsyncSaves {
		b.progress.started(slot, at)
	} else {
		b.progress.saved(at)
		b.sh.Hint(b.sh.Tf("saved %s", slot))
	}
	return true
}

// PollSaves reports the background saves written since the last call: the
// hint for each, and a Danger toast for a failure. Then it runs a leave
// the player confirmed (SAV-013): the App calls it once per Update, a safe
// point outside the widget kit's callbacks.
func (b *Book) PollSaves() {
	b.bg().poll()
	b.runDue()
}

// background is the book's side of the background saves (Env.AsyncSaves);
// p hears each result (SAV-013).
type background struct {
	sh Shell
	p  *progress
}

// bg is the book's background saves.
func (b *Book) bg() background { return background{b.sh, &b.progress} }

func (g background) poll() {
	if g.sh.SaveEnv().AsyncSaves {
		g.report(g.sh.Finished(), "")
	}
}

// wait blocks until the background saves are written and reports them;
// false when the last save of slot failed (any slot: "").
func (g background) wait(slot string) bool {
	if !g.sh.SaveEnv().AsyncSaves {
		return true
	}
	g.sh.Wait()
	return g.report(g.sh.Finished(), slot)
}

// report hints each finished save and raises a failure; false when the
// last one of slot ("": any) failed.
func (g background) report(done []SaveResult, slot string) bool {
	ok := true
	for _, r := range done {
		g.p.finished(r)
		if r.Err == nil {
			g.sh.Hint(g.sh.Tf("saved %s", r.Slot))
		} else {
			msg := g.sh.Tf("save failed: %s", r.Err.Error())
			g.sh.Hint(msg)
			g.sh.SaveFailed(msg)
		}
		if slot == "" || r.Slot == slot {
			ok = r.Err == nil
		}
	}
	return ok
}

// SaveBeforeLeaving saves a commitment colony as the player leaves it
// (Save & Quit to Menu, Return to Main Menu, Quit, closing the window). It
// reports whether leaving may go ahead: false only when that save failed,
// so the player stays (the hint says why) instead of losing the day since
// the last save. A reload-anytime colony leaves without saving, as before.
func (b *Book) SaveBeforeLeaving() bool {
	env := b.sh.SaveEnv()
	if !env.Commitment || env.Capture {
		return true
	}
	return b.SaveNow(b.SaveSlot) && b.bg().wait(b.SaveSlot) // a background save is checked before leaving
}

// RefreshSlots re-reads the save list for the load screen.
func (b *Book) RefreshSlots() {
	b.Slots, b.SlotPage = nil, 0
	if !b.sh.SaveEnv().CanLoad {
		return
	}
	b.bg().wait("") // the list shows a save still being written
	slots, err := b.sh.List()
	if err != nil {
		b.sh.Hint(b.sh.Tf("cannot read saves: %s", err.Error()))
		return
	}
	b.Slots = slots
}
