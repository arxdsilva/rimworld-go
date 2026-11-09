package saves

import (
	"strings"
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/platform/input"
)

// testUI is the fake shell's widget kit with the input it reads: one frame
// at a time, as the root's Update gives it (Begin, the modals, then the
// text field's typing).
type testUI struct {
	kit   *uikit.Kit
	frame inputframe.Frame
}

type kitHost struct{ u *testUI }

func (h kitHost) Frame() *inputframe.Frame { return &h.u.frame }
func (h kitHost) Cursor() (int, int)       { return h.u.frame.Raw.Mx, h.u.frame.Raw.My }
func (h kitHost) Size() (int, int)         { return 1280, 720 }
func (h kitHost) Updates() uint64          { return 0 }
func (h kitHost) KeyRepeat(k string) bool  { return h.u.frame.Key(k) }
func (h kitHost) Cue(string)               {}
func (h kitHost) Hint(string)              {}
func (h kitHost) ReducedMotion() bool      { return true }

// Kit is the fake shell's kit (ports.Widgets), made on first use.
func (f *fakeShell) Kit() *uikit.Kit {
	if f.ui == nil {
		f.ui = &testUI{frame: inputframe.NewFrame(inputframe.Raw{Mx: -50, My: -50})}
		f.ui.kit = uikit.New(kitHost{f.ui})
	}
	return f.ui.kit
}

// step runs one Update of the kit over raw.
func (f *fakeShell) step(raw inputframe.Raw) {
	k := f.Kit()
	f.ui.frame = inputframe.NewFrame(raw)
	k.Begin()
	k.RunModals(nil)
	if k.Editing() {
		k.TextInput()
	}
}

// idle runs one Update with the mouse off the window.
func (f *fakeShell) idle() { f.step(inputframe.Raw{Mx: -50, My: -50}) }

// widget is this frame's widget id, or fails.
func (f *fakeShell) widget(t *testing.T, id string) uikit.Widget {
	t.Helper()
	f.idle()
	w, ok := f.Kit().Find(id)
	if !ok {
		t.Fatalf("no widget %q (top modal %q)", id, f.Kit().Top())
	}
	return w
}

// click presses and releases the left button on widget id.
func (f *fakeShell) click(t *testing.T, id string) {
	t.Helper()
	x, y := f.widget(t, id).R.Center()
	raw := inputframe.Raw{Mx: int(x), My: int(y)}
	raw.Press[inputframe.Left], raw.Down[inputframe.Left] = true, true
	f.step(raw)
	raw.Press[inputframe.Left], raw.Down[inputframe.Left], raw.Release[inputframe.Left] = false, false, true
	f.step(raw)
	f.idle()
}

// typeName replaces the name field's text with s (Ctrl+A, then the
// characters).
func (f *fakeShell) typeName(s string) {
	f.step(inputframe.Raw{Mx: -50, My: -50, Pressed: []string{"A"}, Mods: input.ModCtrl})
	f.step(inputframe.Raw{Mx: -50, My: -50, Chars: []rune(s)})
}

// dialogFake is a running colony with saves listed, its session from
// NewColony with a fixed id.
func dialogFake() (*fakeShell, *Book) {
	f, b := newFake()
	f.env = Env{CanSave: true, CanLoad: true}
	f.season = "spring"
	b.newID = func() string { return "c0ffee" }
	b.NewColony(0)
	return f, b
}

func TestSaveDialogSavesUnderTheTypedName(t *testing.T) {
	t.Parallel()
	f, b := dialogFake()
	colony := b.session.ColonyName
	done := 0
	b.OpenSaveDialog(func() { done++ })
	f.idle()
	if k := f.Kit(); k.Top() != dialogID || !k.Editing() || b.dialog.name != colony {
		t.Fatalf("dialog %q editing %v name %q, want the open dialog editing %q", k.Top(), k.Editing(), b.dialog.name, colony)
	}
	if len(f.saved) != 0 || done != 0 {
		t.Fatalf("opening the dialog saved %v (done %d)", f.saved, done)
	}
	f.step(inputframe.Raw{Mx: -50, My: -50, Chars: []rune("Second Base")}) // the prefill is selected: typing replaces it
	if b.dialog.name != "Second Base" {
		t.Fatalf("name %q after typing over the selected prefill", b.dialog.name)
	}
	f.click(t, "modal.save.ok")
	want := ports.SaveMeta{Name: "Second Base", Colony: "c0ffee", ColonyName: colony, Base: "Second Base", Season: "spring"}
	if len(f.metas) != 1 || f.metas[0] != want {
		t.Fatalf("saved %+v, want %+v", f.metas, want)
	}
	if done != 1 || f.Kit().ModalOpen() || b.dialog.open {
		t.Fatalf("after Save: done %d, modal %q, dialog open %v", done, f.Kit().Top(), b.dialog.open)
	}
	if b.session.Name != "Second Base" || b.session.Base != "Second Base" || b.SaveSlot != colony {
		t.Fatalf("session %+v slot %q: the save names the session, the slot stays the colony's", b.session, b.SaveSlot)
	}
}

func TestSaveDialogCancelAndEscapeSaveNothing(t *testing.T) {
	t.Parallel()
	f, b := dialogFake()
	done := 0
	b.OpenSaveDialog(func() { done++ })
	f.click(t, "modal.save.cancel")
	if f.Kit().ModalOpen() || b.dialog.open {
		t.Fatalf("Cancel left %q open", f.Kit().Top())
	}
	b.OpenSaveDialog(func() { done++ })
	f.idle()
	if !f.Kit().Editing() {
		t.Fatal("the name field does not have the keyboard")
	}
	f.step(inputframe.Raw{Mx: -50, My: -50, Pressed: []string{"Escape"}})
	if f.Kit().ModalOpen() || b.dialog.open {
		t.Fatalf("Esc in the name field left %q open", f.Kit().Top())
	}
	if len(f.saved) != 0 || done != 0 {
		t.Fatalf("Cancel and Esc saved %v (done %d)", f.saved, done)
	}
}

func TestSaveDialogRowTakesItsName(t *testing.T) {
	t.Parallel()
	f, b := dialogFake()
	f.slots = []ports.SaveSlot{
		{Name: "old", Path: "old", Modified: time.Unix(100, 0)},
		{Name: "newer", Path: "newer", Modified: time.Unix(200, 0)},
	}
	b.OpenSaveDialog(nil)
	f.click(t, listID+".body.0") // newest first
	if b.dialog.name != "newer" {
		t.Fatalf("clicking the first row gave the name %q, want the newest save's", b.dialog.name)
	}
}

func TestSaveDialogAsksBeforeReplacing(t *testing.T) {
	t.Parallel()
	f, b := dialogFake()
	f.slots = []ports.SaveSlot{{Name: "Old-Base", Path: "Old-Base", Date: "5th of Spring, 5501",
		Colony: "elsewhere", ColonyName: "Far Shore"}}
	done := 0
	b.OpenSaveDialog(func() { done++ })
	f.typeName("old base")
	if note, _ := nameNote(b, b.dialog.name); note != "Same file as Old-Base" {
		t.Fatalf("note %q under a name whose file is Old-Base's", note)
	}
	f.click(t, "modal.save.ok")
	if f.Kit().Top() != overwriteID || len(f.saved) != 0 {
		t.Fatalf("Save over Old-Base: top %q, saved %v; want the question first", f.Kit().Top(), f.saved)
	}
	if text := overwriteText(b, f.slots[0]); !strings.Contains(text, "5th of Spring, 5501") || !strings.Contains(text, "Far Shore") {
		t.Fatalf("the question %q names neither the date nor the other colony", text)
	}
	f.click(t, "modal."+overwriteID+".cancel")
	if f.Kit().Top() != dialogID || len(f.saved) != 0 || done != 0 {
		t.Fatalf("Cancel: top %q, saved %v; want back in the dialog", f.Kit().Top(), f.saved)
	}
	f.click(t, "modal.save.ok")
	f.click(t, "modal."+overwriteID+".ok")
	if len(f.saved) != 1 || f.saved[0] != "old base" || done != 1 || f.Kit().ModalOpen() {
		t.Fatalf("Replace: saved %v done %d top %q", f.saved, done, f.Kit().Top())
	}
	if own := overwriteText(b, ports.SaveSlot{Name: "x", Date: "d", Colony: "c0ffee"}); strings.Contains(own, "another colony") {
		t.Fatalf("the colony's own save is called another colony's: %q", own)
	}
}

func TestSaveDialogShowsWhyANameIsRefused(t *testing.T) {
	t.Parallel()
	f, b := dialogFake()
	b.OpenSaveDialog(nil)
	for _, c := range []struct{ name, why string }{
		{"", savefmt.NameEmpty},
		{"a/b", savefmt.NameChars},
		{"base-autosave-2", savefmt.NameAutosave},
		{"CON", savefmt.NameDevice},
	} {
		f.typeName(c.name)
		if c.name == "" {
			f.step(inputframe.Raw{Mx: -50, My: -50, Pressed: []string{"Backspace"}})
		}
		w := f.widget(t, "modal.save.ok")
		if !w.Disabled || w.Reason != c.why {
			t.Errorf("%q: Save disabled %v reason %q, want %q", c.name, w.Disabled, w.Reason, c.why)
		}
		if note, _ := nameNote(b, b.dialog.name); note != c.why {
			t.Errorf("%q: note under the field %q, want %q", c.name, note, c.why)
		}
	}
	f.click(t, "modal.save.ok")
	if len(f.saved) != 0 {
		t.Fatalf("a refused name was saved: %v", f.saved)
	}
	b.dialog.name = strings.Repeat("a", savefmt.NameMax+1) // a long name taken from an old save's row
	if r := nameReason(b, b.dialog.name); r != savefmt.NameTooLong {
		t.Fatalf("a long name: %q", r)
	}
}

func TestDeleteAsksAndRefreshes(t *testing.T) {
	t.Parallel()
	f, b := dialogFake()
	f.slots = []ports.SaveSlot{{Name: b.session.Name, Path: "a", Colony: "c0ffee"}, {Name: "b", Path: "b"}}
	b.progress.known = true
	b.RefreshSlots()
	if !b.Click(ids.PrefixLoadDelete+"0") || f.Kit().Top() != deleteID || len(f.deleted) != 0 {
		t.Fatalf("delete control: top %q, deleted %v; want the question first", f.Kit().Top(), f.deleted)
	}
	f.slots = f.slots[1:]
	f.click(t, "modal."+deleteID+".ok")
	if len(f.deleted) != 1 || f.deleted[0] != b.session.Name || len(b.Slots) != 1 {
		t.Fatalf("deleted %v, list %v", f.deleted, b.Slots)
	}
	if b.progress.known || b.session.Name == "" {
		t.Fatalf("deleting the session's save: known %v session %+v; want the session kept and the colony unsaved", b.progress.known, b.session)
	}
}

func TestDeleteRefusesTheRunningCommitmentSave(t *testing.T) {
	t.Parallel()
	f, b := dialogFake()
	f.env.Commitment = true
	f.slots = []ports.SaveSlot{{Name: b.SaveSlot, Path: "a", Colony: "c0ffee", Commitment: true}, {Name: "other", Path: "o", Colony: "x"}}
	b.RefreshSlots()
	if b.DeleteReason(0) == "" || b.DeleteReason(1) != "" || b.DeleteReason(5) == "" {
		t.Fatalf("reasons %q %q %q: want the running colony's save refused, another's allowed", b.DeleteReason(0), b.DeleteReason(1), b.DeleteReason(5))
	}
	b.Click(ids.PrefixLoadDelete + "0")
	if f.Kit().ModalOpen() || len(f.deleted) != 0 || f.hints[len(f.hints)-1] != b.DeleteReason(0) {
		t.Fatalf("the commitment save: top %q deleted %v hints %v", f.Kit().Top(), f.deleted, f.hints)
	}
	if bt := deleteButton(b, 0, gfx.Rect{W: 80, H: 24}); !bt.Disabled || bt.Reason == "" || bt.ID != ids.PrefixLoadDelete+"0" {
		t.Fatalf("the Load screen's control %+v", bt)
	}
	f.env.Commitment = false // on the menu nothing runs: any save may go
	if b.DeleteReason(0) != "" {
		t.Fatalf("with no commitment colony running: %q", b.DeleteReason(0))
	}
}

func TestSaveDialogDeleteControlPerRow(t *testing.T) {
	t.Parallel()
	f, b := dialogFake()
	f.slots = []ports.SaveSlot{{Name: "a", Path: "a", Modified: time.Unix(1, 0)}, {Name: "b", Path: "b", Modified: time.Unix(2, 0)}}
	b.OpenSaveDialog(nil)
	f.idle()
	for _, i := range []string{"0", "1"} {
		if _, ok := f.Kit().Find(ids.PrefixLoadDelete + i); !ok {
			t.Fatalf("no delete control for save %s", i)
		}
	}
	f.click(t, ids.PrefixLoadDelete+"0")
	if f.Kit().Top() != deleteID {
		t.Fatalf("the dialog's delete control opened %q", f.Kit().Top())
	}
	f.click(t, "modal."+deleteID+".ok")
	if len(f.deleted) != 1 || f.deleted[0] != "a" || f.Kit().Top() != dialogID {
		t.Fatalf("deleted %v, top %q; want a gone and the dialog still open", f.deleted, f.Kit().Top())
	}
}
