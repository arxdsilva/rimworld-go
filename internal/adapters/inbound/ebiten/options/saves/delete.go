package saves

import (
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game/savefmt"
)

// Deleting a save (SAV-010): every save row of the Save dialog and of the
// Load screen has a delete control, ids.PrefixLoadDelete + the save's index
// in Slots, which Click routes to askDelete. It asks first (a danger
// confirm), then the Saver deletes the file and the list is read again.
// The running commitment colony's one save cannot be deleted.

// deleteID is the delete question; its OK is modal.save.delete.ok.
const deleteID = "save.delete"

// deleteColW is the width of the Save dialog's delete column, right of
// its list.
const deleteColW = 84

// DeleteReason is why save i cannot be deleted ("" when it can): it is
// not in the list, or it is the running commitment colony's one save.
func (b *Book) DeleteReason(i int) string {
	if i < 0 || i >= len(b.Slots) {
		return b.sh.T("No such save")
	}
	env := b.sh.SaveEnv()
	if env.Commitment && env.CanSave && runningColony(b, b.Slots[i]) {
		return b.sh.T("Commitment: this colony's one save cannot be deleted while it runs")
	}
	return ""
}

// runningColony reports sl is a save of the running colony: its header
// names the colony, else (a list that does not read headers) its name is
// the colony's slot or one of its autosaves.
func runningColony(b *Book, sl ports.SaveSlot) bool {
	if sl.Colony != "" {
		return sl.Colony == b.session.Colony
	}
	return savefmt.BaseSlot(sl.Name) == b.SaveSlot
}

// deleteButton is save i's delete control in r, for a list of
// uikit.Buttons (the Load screen).
func deleteButton(b *Book, i int, r gfx.Rect) uikit.Button {
	reason := b.DeleteReason(i)
	return uikit.Button{ID: ids.PrefixLoadDelete + strconv.Itoa(i), Label: b.sh.T("Delete"),
		X: r.X, Y: r.Y, W: r.W, H: r.H, Disabled: reason != "", Reason: reason}
}

// deleteButtons declares the Save dialog's delete controls, one beside
// each row of list fully in view; order is the rows' data indices in
// display order (the table's sort).
func deleteButtons(b *Book, k *uikit.Kit, list gfx.Rect, order []int) {
	body := gfx.Rect{X: list.X, Y: list.Y + uikit.TableHeaderH, W: list.W, H: list.H - uikit.TableHeaderH}
	off := k.Scroll[listID+".body"]
	first, last := uikit.ListVisible(off, body.H, listRowH, len(order))
	for di := first; di <= last; di++ {
		r := gfx.Rect{X: list.Right() + 6, Y: body.Y + float32(di)*listRowH - off + 1, W: deleteColW - 6, H: listRowH - 2}
		if r.Y < body.Y || r.Bottom() > body.Bottom() {
			continue
		}
		i := order[di]
		reason := b.DeleteReason(i)
		id := ids.PrefixLoadDelete + strconv.Itoa(i)
		if k.Button(id, r, b.sh.T("Delete"), uikit.BtnOpts{Style: uikit.BtnDanger, Disabled: reason != "", Reason: reason}) {
			b.Click(id)
		}
	}
}

// askDelete asks before deleting save i (Click on a delete control); a
// save that cannot be deleted says why instead.
func askDelete(b *Book, i int) {
	if r := b.DeleteReason(i); r != "" {
		b.sh.Hint(r)
		return
	}
	sl := b.Slots[i]
	when := sl.Date
	if when == "" {
		when = savedAt(sl)
	}
	b.sh.Kit().Confirm(deleteID, b.sh.T("Delete save?"),
		b.sh.Tf("Delete %s (%s)? It cannot be brought back.", slotTitle(sl), when), b.sh.T("Delete"), true,
		func() { deleteSlot(b, sl.Name) })
}

// deleteSlot deletes the save in slot name and reads the list again, on
// the same page. Deleting the save the session last wrote leaves the
// session as it is, but the colony counts as unsaved again (progress.known
// false: progress.go).
func deleteSlot(b *Book, name string) {
	b.bg().wait("") // never delete a file the queue is still writing
	if err := b.sh.Delete(name); err != nil {
		b.sh.Hint(b.sh.Tf("cannot delete %s: %s", name, err.Error()))
		return
	}
	if name == b.session.Name {
		b.progress.known = false
	}
	page := b.SlotPage
	b.RefreshSlots()
	b.SlotPage = page // slotWindow keeps it in range
	b.sh.Hint(b.sh.Tf("deleted %s", name))
}
