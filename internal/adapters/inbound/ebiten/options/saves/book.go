package saves

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
	// (SAV-013: Leave asks before unsaved progress is lost).
	//lint:ignore U1000 SAV-013 reads it
	progress progress
	// pending is what runs once the open save dialog or the unsaved-progress
	// question is answered (SAV-001, SAV-013).
	pending func()
	// sort and filter are the Load screen's order and colony filter
	// (ids.LoadSort, ids.LoadColony).
	//lint:ignore U1000 the load list's sort fills it
	sort int
	//lint:ignore U1000 the load list's colony filter fills it
	filter string
	// dialog is the Save dialog (SAV-001).
	//lint:ignore U1000 SAV-001 fills it
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

// progress is when the colony last changed and was last saved, in ticks;
// known is false until a save or load sets both.
type progress struct {
	//lint:ignore U1000 SAV-013 sets and reads them
	dirtyAt, savedAt int64
	//lint:ignore U1000 SAV-013 sets and reads it
	known bool
}

// dialog is the Save dialog's state: open, and the name typed.
type dialog struct {
	//lint:ignore U1000 SAV-001 fills it
	open bool
	//lint:ignore U1000 SAV-001 fills it
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
	b.progress, b.pending = progress{}, nil
}

// OpenSaveDialog is the pause menu's Save. SAV-001 opens the Save dialog
// (name the save, see the colony's saves, overwrite one or write a new
// one) and calls done when it closes. Until then it saves the colony's
// slot at once and calls done.
func (b *Book) OpenSaveDialog(done func()) {
	b.SaveNow(b.SaveSlot)
	done()
}

// Leave runs proceed once the colony may be left (Return to Main Menu,
// Quit). SAV-013 asks first when there is unsaved progress (b.progress)
// and keeps proceed in b.pending. Today it saves a commitment colony
// (SaveBeforeLeaving) and proceeds unless that save failed.
func (b *Book) Leave(proceed func()) {
	if b.SaveBeforeLeaving() {
		proceed()
	}
}

// SaveAndLeave is Save & Quit (to the menu or the desktop). SAV-005 saves
// the colony's slot, waits for it and then proceeds; proceed is the
// leaving intent, which goes through Leave, so SAV-005 must leave Leave
// nothing to save (b.progress) or a commitment colony saves twice. Today
// it proceeds at once and Leave saves a commitment colony, as before.
func (b *Book) SaveAndLeave(proceed func()) { proceed() }

// Click runs a Load screen id (ids.PrefixLoad) and reports it handled:
// false for load.back, which the root's screen intent takes.
func (b *Book) Click(id string) bool {
	switch {
	case strings.HasPrefix(id, ids.PrefixLoadSlot):
		if i, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixLoadSlot)); err == nil {
			b.LoadSlot(i)
		}
		return true
	case strings.HasPrefix(id, ids.PrefixLoadDelete), id == ids.LoadSort, id == ids.LoadColony:
		return true // the load list's delete, sort and colony filter fill these
	}
	return false
}

// DrawPauseStatus draws the colony's save status under the pause menu at
// x, y ("Saved 2 hours ago", "Unsaved progress"; SAV-013). It draws nothing
// yet.
func (b *Book) DrawPauseStatus(dst *ebiten.Image, x, y float32) {}
