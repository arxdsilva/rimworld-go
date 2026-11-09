package saves

import (
	"fmt"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game/savefmt"
)

// Saved games. The book only sees saves through the Saver, so it never
// touches files or game internals.

// SaveNow writes the current colony to slot and reports success (the hint
// says what happened). With background saves (Env.AsyncSaves) it reports
// that the save started: PollSaves says how the write went.
func (b *Book) SaveNow(slot string) bool {
	env := b.sh.SaveEnv()
	if !env.CanSave {
		return false
	}
	if err := b.sh.Save(ports.SaveMeta{Name: slot}); err != nil {
		b.sh.Hint(b.sh.Tf("save failed: %s", err.Error()))
		return false
	}
	if !env.AsyncSaves {
		b.sh.Hint(b.sh.Tf("saved %s", slot))
	}
	return true
}

// PollSaves reports the background saves written since the last call: the
// hint for each, and a Danger toast for a failure. The App calls it at
// the start of every Update.
func (b *Book) PollSaves() { background{b.sh}.poll() }

// background is the book's side of the background saves (Env.AsyncSaves).
type background struct{ sh Shell }

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

// TickAutosave saves at AutosaveHour every Options-chosen number of
// in-game days (0: never) to <slot>-autosave, or with General.AutosavesKept
// above one to the next of <slot>-autosave-1…N (autosaveSlot); a
// commitment colony saves its own slot every day, whatever Options say.
func (b *Book) TickAutosave() {
	env := b.sh.SaveEnv()
	if !env.CanSave || env.Capture {
		return
	}
	n := b.sh.Settings().Gameplay.AutosaveDays
	if env.Commitment {
		n = 1 // min(Options, daily), never off
	}
	if n <= 0 {
		return
	}
	d := b.sh.Date()
	day := int(b.sh.Tick()) / 60000
	if d.Hour == AutosaveHour && day-b.AutosaveDay >= n {
		b.AutosaveDay = day
		slot := b.SaveSlot
		if !env.Commitment {
			slot = autosaveSlot(b, day)
		}
		b.SaveNow(slot)
	}
}

// autosaveSlot is the slot the next autosave of this colony is written
// to. One kept autosave is <slot>-autosave, as it always was. With more it
// is the first of <slot>-autosave-1…N that does not exist, else the one
// written longest ago, so the oldest is overwritten; when the save list
// cannot be read it goes round by the in-game day.
func autosaveSlot(b *Book, day int) string {
	base := b.SaveSlot + "-autosave"
	kept := b.sh.Settings().General.AutosavesKept
	if kept <= 1 {
		return base
	}
	slots, err := b.sh.List()
	if err != nil || !b.sh.SaveEnv().CanLoad {
		return fmt.Sprintf("%s-%d", base, day%kept+1)
	}
	var oldest time.Time
	pick := ""
	for i := 1; i <= kept; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		at, found := time.Time{}, false
		for _, sl := range slots {
			if sl.Name == name {
				at, found = sl.Modified, true
				break
			}
		}
		if !found {
			return name
		}
		if pick == "" || at.Before(oldest) {
			pick, oldest = name, at
		}
	}
	return pick
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
	return b.SaveNow(b.SaveSlot) && background{b.sh}.wait(b.SaveSlot) // a background save is checked before leaving
}

// RefreshSlots re-reads the save list for the load screen.
func (b *Book) RefreshSlots() {
	b.Slots, b.SlotPage = nil, 0
	if !b.sh.SaveEnv().CanLoad {
		return
	}
	background{b.sh}.wait("") // the list shows a save still being written
	slots, err := b.sh.List()
	if err != nil {
		b.sh.Hint(b.sh.Tf("cannot read saves: %s", err.Error()))
		return
	}
	b.Slots = slots
}

// LoadSlot replaces the session with saved game i.
func (b *Book) LoadSlot(i int) {
	if i == slotNextPage { // the "more saves" row of a long list
		_, _, pages := b.slotWindow()
		b.SlotPage = (b.SlotPage + 1) % max(1, pages)
		return
	}
	if i < 0 || i >= len(b.Slots) || !b.sh.SaveEnv().CanLoad {
		return
	}
	background{b.sh}.wait("") // never read a file the queue is replacing
	sl := b.Slots[i]
	if sl.Err != "" {
		b.sh.Hint(b.sh.Tf("cannot load: %s", sl.Err))
		return
	}
	meta, tick, err := b.sh.Load(sl.Path)
	if err != nil {
		b.sh.Hint(b.sh.Tf("load failed: %s", err.Error()))
		return
	}
	b.SaveSlot = savefmt.BaseSlot(meta.Name)
	b.AutosaveDay = int(tick) / 60000
	b.sh.Hint(b.sh.Tf("loaded %s", sl.Name))
}
