package saves

import (
	"image/color"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game/savefmt"
)

// The Save dialog (SAV-001): the pause menu's Save opens it over the
// pause menu. A name field, prefilled with the colony's last save and
// selected so typing replaces it; under it why the name cannot be used
// (SAV-004), or the save it would replace; the saves on disk (clicking one
// takes its name; each has a delete control, delete.go); Save and Cancel.
// Saving over an existing file asks first (SAV-002).

// The dialog's ids. Its footer buttons are modal.save.ok and
// modal.save.cancel; the overwrite question's OK is modal.save.overwrite.ok.
const (
	dialogID    = "save"
	nameFieldID = "save.name"
	listID      = "save.list"
	overwriteID = "save.overwrite"
)

const (
	dialogW, dialogH = 680, 520
	fieldH           = 28
	listRowH         = 26
	savedCol         = 3 // the Saved column: the list opens newest first
)

// openDialog opens the Save dialog; done runs once the colony is saved
// from it (Cancel and Esc close it without done).
func openDialog(b *Book, done func()) {
	k := b.sh.Kit()
	if b.dialog.open && k.ModalOpen() && dialogShown(k) {
		return
	}
	b.RefreshSlots()
	name := b.session.Base
	if name == "" {
		name = b.SaveSlot
	}
	b.dialog = dialog{open: true, name: name}
	focused, picked := false, ""
	k.TableSetSort(listID, savedCol, true)
	k.Push(uikit.Modal{ID: dialogID, Title: b.sh.T("Save Game"), W: dialogW, H: dialogH,
		Draw: func(body gfx.Rect) {
			if !focused { // the field takes the keyboard on its first frame, all selected
				focused = true
				k.TabTo = nameFieldID
			}
			drawDialog(b, k, body, &picked)
		},
		Buttons: []uikit.ModalButton{
			{ID: "ok", Label: b.sh.T("Save"), Style: uikit.BtnPrimary,
				Disabled: func() string { return nameReason(b, b.dialog.name) },
				OnClick:  func() { submit(b, done) }},
			{ID: "cancel", Label: b.sh.T("Cancel"), Closes: true, OnClick: func() { b.dialog = dialog{} }},
		},
		OnClose: func() { b.dialog = dialog{} },
	})
}

// DialogName is the name in the open Save dialog ("" when it is closed).
func (b *Book) DialogName() string { return b.dialog.name }

// dialogShown reports the Save dialog is on the kit's stack.
func dialogShown(k *uikit.Kit) bool {
	for _, m := range k.Modals {
		if m.ID == dialogID {
			return true
		}
	}
	return false
}

// drawDialog declares the dialog's body: the name, its note, the saves.
func drawDialog(b *Book, k *uikit.Kit, body gfx.Rect, picked *string) {
	if k.Top() == dialogID && k.Editing() && k.Host().Frame().Key("Escape") {
		k.CloseTop() // Esc in the name field cancels the dialog, not only the edit
		return
	}
	k.Label(body.X, body.Y, b.sh.T("Save name"), uikit.DimText())
	field := gfx.Rect{X: body.X, Y: body.Y + 22, W: body.W, H: fieldH}
	k.TextField(nameFieldID, field, &b.dialog.name, uikit.TextFieldSpec{MaxLen: savefmt.NameMax})
	lh := float32(gfx.LineH(gfx.Small))
	if note, c := nameNote(b, b.dialog.name); note != "" {
		k.Label(field.X, field.Bottom()+3, note, gfx.WithColor(gfx.Small, c))
	}
	top := field.Bottom() + 3 + lh + 8
	list := gfx.Rect{X: body.X, Y: top, W: body.W - deleteColW, H: body.Bottom() - top}
	if len(b.Slots) == 0 {
		k.Label(list.X, list.Y, b.sh.T("No saves yet."), uikit.DimText())
		return
	}
	cols := dialogCols(b)
	deleteButtons(b, k, list, k.TableOrder(listID, cols, len(b.Slots))) // before the table: beside its rows
	sel := slotIndex(b.Slots, *picked)
	if k.Table(listID, list, cols, len(b.Slots), &sel, listRowH) && sel >= 0 {
		*picked = b.Slots[sel].Name
		b.dialog.name = slotTitle(b.Slots[sel])
	}
}

// dialogCols are the list's columns: name, colony, in-game date, when saved.
func dialogCols(b *Book) []uikit.TableCol {
	sl := func(i int) ports.SaveSlot { return b.Slots[i] }
	return []uikit.TableCol{
		{ID: "name", Title: b.sh.T("Name"), Cell: func(i int) string { return slotTitle(sl(i)) },
			Less: func(i, j int) bool { return strings.ToLower(slotTitle(sl(i))) < strings.ToLower(slotTitle(sl(j))) }},
		{ID: "colony", Title: b.sh.T("Colony"), W: 150, Cell: func(i int) string { return sl(i).ColonyName },
			Less: func(i, j int) bool { return sl(i).ColonyName < sl(j).ColonyName }},
		{ID: "date", Title: b.sh.T("Date"), W: 130, Cell: func(i int) string { return sl(i).Date }},
		{ID: "saved", Title: b.sh.T("Saved"), W: 110, Cell: func(i int) string { return savedAt(sl(i)) },
			Less: func(i, j int) bool { return sl(i).Modified.Before(sl(j).Modified) }},
	}
}

// savedAt is when the save was written, as the list shows it.
func savedAt(sl ports.SaveSlot) string {
	if sl.Modified.IsZero() {
		return ""
	}
	return sl.Modified.Format("Jan 2 15:04")
}

// slotIndex is the index of the save named name in slots, or -1.
func slotIndex(slots []ports.SaveSlot, name string) int {
	for i, sl := range slots {
		if name != "" && sl.Name == name {
			return i
		}
	}
	return -1
}

// nameReason is why the dialog's name cannot be saved ("" when it can):
// savefmt.CheckName's reason in the player's language.
func nameReason(b *Book, name string) string {
	switch savefmt.CheckName(name) {
	case "":
		return ""
	case savefmt.NameEmpty:
		return b.sh.T("Name the save")
	case savefmt.NameTooLong:
		return b.sh.T("Use at most 60 characters")
	case savefmt.NameChars:
		return b.sh.T("Use only letters, digits, spaces, - and _")
	case savefmt.NameAutosave:
		return b.sh.T("Names ending in -autosave are kept for autosaves")
	case savefmt.NameDevice:
		return b.sh.T("Windows keeps this name for a device")
	}
	return b.sh.T("This name cannot be used")
}

// nameNote is the line under the name field and its colour: why the name
// cannot be used (danger), or the save of another spelling whose file it
// would replace (warning).
func nameNote(b *Book, name string) (string, color.Color) {
	if r := nameReason(b, name); r != "" {
		return r, uikit.Danger
	}
	name = strings.TrimSpace(name)
	if sl, ok := slotKeyed(b.Slots, name); ok && slotTitle(sl) != name && sl.Name != name {
		return b.sh.Tf("Same file as %s", slotTitle(sl)), uikit.Warning
	}
	return "", nil
}

// submit is Save: the name is checked again, a save whose file it would
// replace is asked about first (Cancel goes back to the dialog), then the
// colony is written.
func submit(b *Book, done func()) {
	name := strings.TrimSpace(b.dialog.name)
	if nameReason(b, name) != "" {
		return
	}
	sl, ok := slotKeyed(b.Slots, name)
	if !ok {
		commitSave(b, name, done)
		return
	}
	b.sh.Kit().Confirm(overwriteID, b.sh.T("Replace save?"), overwriteText(b, sl), b.sh.T("Replace"), true,
		func() { commitSave(b, name, done) })
}

// overwriteText names the save Save would replace: its in-game date, and
// its colony when that is another colony.
func overwriteText(b *Book, sl ports.SaveSlot) string {
	when := sl.Date
	if when == "" {
		when = savedAt(sl)
	}
	if sl.Colony != "" && sl.Colony != b.session.Colony {
		colony := sl.ColonyName
		if colony == "" {
			colony = sl.Base
		}
		return b.sh.Tf("%s (%s) belongs to another colony, %s. Replace it with this colony?", slotTitle(sl), when, colony)
	}
	return b.sh.Tf("%s (%s) is replaced by this save.", slotTitle(sl), when)
}

// commitSave writes the colony to name, which becomes the session's save
// and the Base its autosaves carry, closes the dialog and runs done. A
// save that fails keeps the dialog open (the hint says why).
func commitSave(b *Book, name string, done func()) {
	meta := metaFor(b, name, false)
	meta.Base = name
	if !write(b, meta) {
		return
	}
	b.session.Name, b.session.Base = name, name
	b.sh.Kit().CloseModal(dialogID)
	b.dialog = dialog{}
	if done != nil {
		done()
	}
}
