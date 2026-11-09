package saves

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
)

// AutosaveHour is when each in-game day's autosave happens.
const AutosaveHour = 6

// Commitment mode (NG-1 §2.13): the colony has one save, its slot, which
// the game writes itself: every morning at AutosaveHour, and whenever the
// player leaves (Save & Quit to Menu, Return to Main Menu, Quit, closing
// the window). There is no -autosave copy, Load is off in the pause menu
// and developer mode is locked (options). Reload-anytime colonies keep
// manual saves and the Options autosave interval to <slot>-autosave.
const CommitLoadReason = "Commitment: one timeline"

// NewSlotName names a new colony's save slot.
func NewSlotName() string { return "colony-" + time.Now().Format("2006-01-02-1504") }

// Book is the running colony's saves and the Load screen's list.
type Book struct {
	sh Shell

	// The state the root reads (the main menu's Continue, the shell's Saves
	// role).
	Slots       []ports.SaveSlot // the Load screen's list
	SlotsRead   bool             // the main menu read the list once
	SaveSlot    string           // the current colony's save slot
	AutosaveDay int              // in-game day of the last autosave
	SlotPage    int              // the Load screen's page of saves

	// Declared for the named-saves work (SAV-001…025); each says who fills
	// it. Until then only NewColony writes them.

	// session is the running colony as its saves name it (SAV-003).
	session session
	// progress is whether the colony changed since its last save
	// (SAV-013: Leave asks before unsaved progress is lost; progress.go).
	progress progress
	// pending is what runs once the open save dialog or the unsaved-progress
	// question is answered (SAV-001, SAV-013).
	pending func()
	// sort and filter are the Load screen's order and colony filter
	// (ids.LoadSort, ids.LoadColony); view is the list they show
	// (load_screen.go).
	sort   int
	filter string
	view   loadView
	// dialog is the Save dialog (SAV-001; dialog.go).
	dialog dialog
	// newID makes a colony's stable id (crypto/rand, 16 hex digits; tests
	// inject their own).
	newID func() string
}

// session names the running colony's saves: its stable id and name, the
// slot of the last save and the slot its autosaves are named after.
type session struct {
	Colony, ColonyName, Name, Base string
}

// progress is how far the colony's saves have caught up with it, in ticks
// (SAV-013; its methods are in progress.go).
type progress struct {
	// dirtyAt is the latest tick seen; savedAt the encode tick of the last
	// save written (or the tick the colony began or was loaded at).
	dirtyAt, savedAt int64
	// known is set once a save was written or loaded: "Saved N hours ago";
	// until then the colony is "Not saved yet" and counts as unsaved.
	known bool
	// inflight is the background saves started and not yet reported,
	// oldest first.
	inflight []encoded
	// due is a leave the player confirmed; it runs at the next PollSaves,
	// never inside the modal's callback.
	due func()
}

// dialog is the Save dialog's state: open, and the name typed.
type dialog struct {
	open bool
	name string
}

// New builds the book over its shell.
func New(sh Shell) *Book { return &Book{sh: sh, newID: randomID} }

// randomID is 8 random bytes in hex: a colony id no two colonies share.
func randomID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // never fails (crypto/rand.Read)
	return hex.EncodeToString(b[:])
}

// NewColony starts the book of a colony just built at tick: a new slot,
// no autosave yet, a new colony id (SAV-003 writes the session into each
// save's header; tick is the colony's first, for SAV-013's progress).
func (b *Book) NewColony(tick int64) {
	name := NewSlotName()
	b.SaveSlot, b.AutosaveDay = name, 0
	b.session = session{Colony: b.newID(), ColonyName: name, Name: name, Base: name}
	b.progress, b.pending = progressBegun(tick), nil
}

// OpenSaveDialog is the pause menu's Save: the Save dialog (SAV-001,
// dialog.go) names the save, lists the saves, and asks before replacing
// one; done runs once the colony is saved from it (Cancel and Esc close
// it without done).
func (b *Book) OpenSaveDialog(done func()) { openDialog(b, done) }

// Click runs a Load screen id (ids.PrefixLoad) and reports it handled:
// false for load.back, which the root's screen intent takes.
func (b *Book) Click(id string) bool {
	switch {
	case strings.HasPrefix(id, ids.PrefixLoadSlot):
		if i, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixLoadSlot)); err == nil {
			b.LoadSlot(i)
		}
		return true
	case id == ids.LoadSort:
		b.cycleSort()
		return true
	case id == ids.LoadColony:
		b.cycleColony()
		return true
	case strings.HasPrefix(id, ids.PrefixLoadDelete):
		if i, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixLoadDelete)); err == nil {
			askDelete(b, i) // delete.go
		}
		return true
	}
	return false
}
