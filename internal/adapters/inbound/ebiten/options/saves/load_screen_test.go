package saves

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/sim/clock"
)

// slotButtons is the list's save rows (load.slot.N, not the page row).
func slotButtons(bs []uikit.Button) []uikit.Button {
	var out []uikit.Button
	for _, b := range bs {
		if strings.HasPrefix(b.ID, ids.PrefixLoadSlot) && b.ID != ids.PrefixLoadSlot+"-1" {
			out = append(out, b)
		}
	}
	return out
}

// rowNames is the shown rows' titles, in order.
func rowNames(b *Book) []string {
	var out []string
	for _, bt := range slotButtons(b.LoadButtons(1280, 720)) {
		var i int
		fmt.Sscanf(bt.ID, ids.PrefixLoadSlot+"%d", &i)
		out = append(out, slotTitle(b.Slots[i]))
	}
	return out
}

func label(t *testing.T, b *Book, id string) string {
	t.Helper()
	for _, bt := range b.LoadButtons(1280, 720) {
		if bt.ID == id {
			return bt.Label
		}
	}
	t.Fatalf("no %s", id)
	return ""
}

// load.sort toggles the store's order and the name (case-folded, ties
// kept in order); load.colony goes All, each colony, All. Both reset the
// page and say what they show.
func TestLoadSortAndColonyFilter(t *testing.T) {
	t.Parallel()
	f, b := newFake()
	f.env.CanLoad = true
	f.slots = []ports.SaveSlot{
		{Name: "b", Path: "/b", ColonyName: "Ridge"}, {Name: "a2", Path: "/a2", Title: "Alpha", ColonyName: "Marsh"},
		{Name: "C", Path: "/C", ColonyName: "Ridge"}, {Name: "a1", Path: "/a1", Title: "alpha", ColonyName: "Ridge"},
	}
	b.RefreshSlots()
	if got := fmt.Sprint(rowNames(b)); got != "[b Alpha C alpha]" || label(t, b, ids.LoadSort) != "Sort: Newest" {
		t.Fatalf("default order %s (%s), want the store's", got, label(t, b, ids.LoadSort))
	}
	b.SlotPage = 3
	b.Click(ids.LoadSort)
	if got := fmt.Sprint(rowNames(b)); got != "[Alpha alpha b C]" || b.SlotPage != 0 || label(t, b, ids.LoadSort) != "Sort: Name" {
		t.Fatalf("by name %s page %d", got, b.SlotPage)
	}
	want := []string{"Colony: Ridge", "Colony: Marsh", "Colony: All"}
	rows := []string{"[alpha b C]", "[Alpha]", "[Alpha alpha b C]"}
	if l := label(t, b, ids.LoadColony); l != "Colony: All" {
		t.Fatalf("filter %q", l)
	}
	for i := range want {
		b.Click(ids.LoadColony)
		if l, got := label(t, b, ids.LoadColony), fmt.Sprint(rowNames(b)); l != want[i] || got != rows[i] {
			t.Fatalf("click %d: %q %s, want %q %s", i, l, got, want[i], rows[i])
		}
	}
	b.Click(ids.LoadSort)
	b.Click(ids.LoadColony)
	if got := fmt.Sprint(rowNames(b)); got != "[b C alpha]" {
		t.Fatalf("store order filtered %s", got)
	}
	// A row's index is the save's own: loading it loads that save.
	bt := slotButtons(b.LoadButtons(1280, 720))[2]
	b.Click(bt.ID)
	if len(f.loaded) != 1 || f.loaded[0] != f.slots[3].Path {
		t.Fatalf("loaded %v from %s", f.loaded, bt.ID)
	}
}

// Sort and filter page like the plain list.
func TestFilteredListPages(t *testing.T) {
	t.Parallel()
	f, b := newFake()
	f.env.CanLoad = true
	for i := 0; i < 20; i++ {
		f.slots = append(f.slots, ports.SaveSlot{Name: fmt.Sprintf("s%02d", i), ColonyName: []string{"A", "B"}[i%2]})
	}
	b.RefreshSlots()
	b.Click(ids.LoadColony)
	seen := map[string]bool{}
	for page := 0; page < 2; page++ {
		for _, n := range rowNames(b) {
			seen[n] = true
		}
		b.LoadSlot(slotNextPage)
	}
	if len(seen) != 10 {
		t.Fatalf("colony A over two pages: %d saves, want 10", len(seen))
	}
}

// compatBook is a running-colony-free book whose list holds one save per
// compat case.
func compatBook() (*fakeShell, *Book, *uikit.Kit) {
	f, _ := newFake()
	f.env.CanLoad = true
	b, k := withKit(f)
	f.slots = []ports.SaveSlot{
		{Name: "old", Path: "/old", Compat: int(savefmt.CompatOlder)},
		{Name: "new", Path: "/new", Compat: int(savefmt.CompatNewer), Build: "9.9"},
		{Name: "mod", Path: "/mod", Compat: int(savefmt.CompatContent), Notice: "the Orchard pack is missing"},
		{Name: "bad", Path: "/bad", Err: "too old to read"},
		{Name: "ok", Path: "/ok"},
	}
	b.RefreshSlots()
	return f, b, k
}

// An older save asks before it loads and is upgraded; a newer one is off
// with the build that wrote it; other content asks naming the difference.
func TestLoadAsksForOlderAndContentRefusesNewer(t *testing.T) {
	t.Parallel()
	f, b, k := compatBook()
	b.LoadSlot(0)
	if k.Top() != compatModal || len(f.loaded) != 0 {
		t.Fatalf("older save: modal %q loaded %v", k.Top(), f.loaded)
	}
	if body := k.Modals[0]; body.Title != olderTag {
		t.Fatalf("title %q", body.Title)
	}
	answer(t, k, compatModal, "cancel")
	b.PollSaves()
	if len(f.loaded) != 0 {
		t.Fatal("Cancel loaded")
	}
	b.LoadSlot(0)
	answer(t, k, compatModal, "ok")
	if len(f.loaded) != 0 {
		t.Fatal("loaded inside the modal's callback")
	}
	b.PollSaves()
	if len(f.loaded) != 1 || f.loaded[0] != "/old" {
		t.Fatalf("OK: loaded %v", f.loaded)
	}

	bs := slotButtons(b.LoadButtons(1280, 720))
	if !bs[1].Disabled || bs[1].Reason != "Made with a newer version (build 9.9). Update the game to load it." {
		t.Fatalf("newer row %+v", bs[1])
	}
	if !bs[3].Disabled || !strings.Contains(bs[3].Reason, "too old to read") || bs[0].Disabled || bs[2].Disabled {
		t.Fatalf("rows %+v", bs)
	}
	b.LoadSlot(1)
	if len(f.loaded) != 1 || !strings.HasPrefix(f.hints[len(f.hints)-1], "Made with a newer version") {
		t.Fatalf("a newer save loaded: %v %v", f.loaded, f.hints)
	}

	b.LoadSlot(2)
	if k.Top() != compatModal {
		t.Fatal("other content must ask")
	}
	answer(t, k, compatModal, "ok")
	b.PollSaves()
	if len(f.loaded) != 2 || f.loaded[1] != "/mod" {
		t.Fatalf("content OK: loaded %v", f.loaded)
	}
	b.LoadSlot(4)
	if len(f.loaded) != 3 || k.Top() != "" {
		t.Fatalf("a current save loads at once: %v", f.loaded)
	}
}

// The content question names the difference.
func TestContentQuestionNamesTheDifference(t *testing.T) {
	t.Parallel()
	_, b, k := compatBook()
	b.LoadSlot(2)
	m := k.Modals[0]
	if m.Title != contentTag || len(m.Buttons) != 2 {
		t.Fatalf("modal %+v", m)
	}
	// Confirm wraps its body into the Draw closure; the text is built from
	// the save's notice.
	if got := b.sh.Tf(contentAsk, "the Orchard pack is missing"); !strings.Contains(got, "Orchard") {
		t.Fatalf("body %q", got)
	}
}

// A save that fails to load is marked damaged and stays off until the list
// is read again.
func TestFailedLoadMarksTheRow(t *testing.T) {
	t.Parallel()
	f, b, _ := compatBook()
	f.loadErr = errors.New("unexpected end of file")
	b.LoadSlot(4)
	if len(f.loaded) != 1 || b.Slots[4].Err != "damaged file" {
		t.Fatalf("loaded %v, row %+v", f.loaded, b.Slots[4])
	}
	if bt := slotButtons(b.LoadButtons(1280, 720))[4]; !bt.Disabled || !strings.Contains(bt.Reason, "damaged file") {
		t.Fatalf("row %+v", bt)
	}
	b.LoadSlot(4)
	if len(f.loaded) != 1 {
		t.Fatal("a damaged save was tried again")
	}
	f.loadErr = nil
	f.slots[4].Err = "" // the fake's list is the book's slice; a real List reads the file anew
	b.RefreshSlots()
	b.LoadSlot(4)
	if len(f.loaded) != 2 {
		t.Fatal("a fresh list may try it again")
	}
}

// Loading over a running colony with unsaved progress asks first; a load
// resets the progress to saved.
func TestInGameLoadAsksAboutUnsavedProgress(t *testing.T) {
	t.Parallel()
	f, b, k := reloadAnytime()
	f.slots = []ports.SaveSlot{{Name: "other", Path: "/other"}}
	b.RefreshSlots()
	b.LoadSlot(0)
	if k.Top() != unsavedModal || len(f.loaded) != 0 {
		t.Fatalf("modal %q loaded %v", k.Top(), f.loaded)
	}
	answer(t, k, unsavedModal, "leave")
	b.PollSaves()
	if len(f.loaded) != 1 || b.progress.dirty() || b.progress.savedAt != 3*clock.TicksPerDay {
		t.Fatalf("loaded %v progress %+v", f.loaded, b.progress)
	}
}

// Each shown save has its Delete beside its row, keyed on the save's own
// index, and it follows sort and filter.
func TestLoadRowsHaveDelete(t *testing.T) {
	t.Parallel()
	f, b := newFake()
	f.env.CanLoad = true
	f.slots = []ports.SaveSlot{{Name: "b", Path: "/b", ColonyName: "Ridge"}, {Name: "a", Path: "/a", ColonyName: "Marsh"}}
	b.RefreshSlots()
	b.Click(ids.LoadColony) // Ridge only
	bs := b.LoadButtons(1280, 720)
	var del []uikit.Button
	for _, bt := range bs {
		if strings.HasPrefix(bt.ID, ids.PrefixLoadDelete) {
			del = append(del, bt)
		}
	}
	rows := slotButtons(bs)
	if len(del) != 1 || len(rows) != 1 || del[0].ID != ids.PrefixLoadDelete+"0" {
		t.Fatalf("deletes %+v rows %+v", del, rows)
	}
	r, d := rows[0], del[0]
	if d.Y != r.Y || d.H != r.H || d.X < r.X+r.W || d.Disabled {
		t.Fatalf("delete %+v beside row %+v", d, r)
	}
}
