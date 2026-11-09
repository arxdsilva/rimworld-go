package saves

import (
	"slices"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game/savefmt"
)

// The Load screen: the list of saves, sorted and filtered by colony, a
// page at a time (SAV-011, SAV-012), and loading one: a save this build
// reads differently asks first (SAV-021), one that failed to load stays
// off until the list is read again (SAV-022), and a running colony with
// unsaved progress asks before it is replaced (SAV-013).

const (
	slotRowH = 40
	// slotRowW is a save row's width; its Delete fills the rest of the
	// list's 600 px.
	slotRowW = 504
	slotRows = 9
	// footerYOff is how far above the screen's bottom the Back button sits
	// (the same footer as every menu screen's).
	footerYOff = 64
)

// commitTag marks a commitment save in the load list; EndingTag a save
// whose ship has left (AX-8: the story is complete).
const (
	commitTag = "Commitment"
	EndingTag = "Complete: escaped by ship"
)

// slotNextPage is the index the "more saves" row sends (load.slot.-1):
// Click hands every load.slot.N click to LoadSlot(N).
const slotNextPage = -1

// The list's orders (ids.LoadSort): as the store lists them (newest
// first), or by name.
const (
	sortNewest = iota
	sortName
	sortOrders
)

// compatModal is the id of the question an older or different save asks.
const compatModal = "saves.compat"

// loadView is the slots the screen shows, as indices into Book.Slots in
// order, and what they were built from (rebuilt when any of it changes).
type loadView struct {
	idx    []int
	base   *ports.SaveSlot
	n      int
	sort   int
	filter string
}

// shown is the slots the list shows: filtered to the chosen colony and in
// the chosen order. Indices stay Book.Slots', so load.slot.N and the main
// menu's Continue name the same save.
func (b *Book) shown() []int {
	var base *ports.SaveSlot
	if len(b.Slots) > 0 {
		base = &b.Slots[0]
	}
	if b.filter != "" && !b.hasColony(b.filter) {
		b.filter = "" // that colony's saves are gone
	}
	v := &b.view
	if v.idx != nil && v.base == base && v.n == len(b.Slots) && v.sort == b.sort && v.filter == b.filter {
		return v.idx
	}
	idx := make([]int, 0, len(b.Slots))
	for i, sl := range b.Slots {
		if b.filter == "" || sl.ColonyName == b.filter {
			idx = append(idx, i)
		}
	}
	if b.sort == sortName {
		slices.SortStableFunc(idx, func(x, y int) int {
			return strings.Compare(strings.ToLower(slotTitle(b.Slots[x])), strings.ToLower(slotTitle(b.Slots[y])))
		})
	}
	*v = loadView{idx: idx, base: base, n: len(b.Slots), sort: b.sort, filter: b.filter}
	return idx
}

// hasColony reports a listed save of the colony named name.
func (b *Book) hasColony(name string) bool {
	for _, sl := range b.Slots {
		if sl.ColonyName == name {
			return true
		}
	}
	return false
}

// colonies is each colony named in the list, in list order.
func (b *Book) colonies() []string {
	var out []string
	for _, sl := range b.Slots {
		if sl.ColonyName != "" && !slices.Contains(out, sl.ColonyName) {
			out = append(out, sl.ColonyName)
		}
	}
	return out
}

// cycleSort is load.sort: Newest, then Name.
func (b *Book) cycleSort() {
	b.sort, b.SlotPage = (b.sort+1)%sortOrders, 0
}

// cycleColony is load.colony: All, then each colony in the list.
func (b *Book) cycleColony() {
	names := b.colonies()
	next := ""                                                // after the last colony: All again
	if i := slices.Index(names, b.filter); i+1 < len(names) { // All is -1
		next = names[i+1]
	}
	b.filter, b.SlotPage = next, 0
}

// slotWindow is the shown saves the Load screen lists now: shown
// start..start+n (all of them up to slotRows; a longer list gives
// slotRows-1 per page and a row to turn the page), and the number of
// pages.
func (b *Book) slotWindow() (start, n, pages int) {
	total := len(b.shown())
	if total <= slotRows {
		return 0, total, 1
	}
	per := slotRows - 1
	pages = (total + per - 1) / per
	page := max(0, min(b.SlotPage, pages-1))
	start = page * per
	return start, min(per, total-start), pages
}

// LoadButtons are the Load screen's buttons: Back, then one per slot shown,
// then, when the list is longer than a page, the row that turns the page,
// then with two saves or more the sort and colony filter (footer, right),
// then each shown save's Delete beside its row (delete.go).
func (b *Book) LoadButtons(sw, sh int) []uikit.Button {
	fy := float32(sh - footerYOff)
	out := []uikit.Button{{ID: "load.back", Label: b.sh.T("Back"), X: 40, Y: fy, W: 140, H: 36}}
	x := float32(sw)/2 - 300
	shown := b.shown()
	start, n, pages := b.slotWindow()
	var dels []uikit.Button
	for j := 0; j < n; j++ {
		i := shown[start+j]
		y := float32(120 + j*(slotRowH+6))
		dels = append(dels, deleteButton(b, i, gfx.Rect{X: x + slotRowW + 6, Y: y, W: 600 - slotRowW - 6, H: slotRowH}))
		r := FormatRow(b.sh, b.Slots[i]) // the row's text is drawn by DrawLoad
		out = append(out, uikit.Button{ID: ids.PrefixLoadSlot + strconv.Itoa(i), Quiet: true,
			X: x, Y: y, W: slotRowW, H: slotRowH,
			Disabled: r.Bad != "", Reason: r.Bad, Tip: r.Tip})
	}
	if pages > 1 {
		page := start/(slotRows-1) + 1
		out = append(out, uikit.Button{ID: ids.PrefixLoadSlot + strconv.Itoa(slotNextPage),
			Label: b.sh.Tf("More saves (page %d of %d)", page, pages),
			X:     x, Y: float32(120 + n*(slotRowH+6)), W: 600, H: slotRowH})
	}
	if len(b.Slots) > 1 {
		order := b.sh.T("Newest")
		if b.sort == sortName {
			order = b.sh.T("Name")
		}
		colony := b.filter
		if colony == "" {
			colony = b.sh.T("All")
		}
		cx := float32(sw) - 40 - 240
		out = append(out,
			uikit.Button{ID: ids.LoadSort, Label: b.sh.Tf("Sort: %s", order), X: cx - 12 - 200, Y: fy, W: 200, H: 36},
			uikit.Button{ID: ids.LoadColony, Label: b.sh.Tf("Colony: %s", colony), X: cx, Y: fy, W: 240, H: 36})
	}
	return append(out, dels...)
}

// DrawLoad paints the Load screen.
func (b *Book) DrawLoad(screen *ebiten.Image, sw, sh int) {
	body := []string{}
	if len(b.Slots) == 0 {
		body = append(body, b.sh.T("No saves yet."), "", b.sh.T("Save from the pause menu (Esc) while playing; the game also"), b.sh.T("autosaves every morning at 06:00."))
	}
	buttons := b.LoadButtons(sw, sh)
	b.sh.PaintMenu(screen, sw, sh, b.sh.T("Load Game"), body, buttons)
	shown := b.shown()
	start, n, _ := b.slotWindow()
	for j := 0; j < n; j++ {
		bt := buttons[j+1]
		DrawRow(screen, FormatRow(b.sh, b.Slots[shown[start+j]]), bt.X, bt.Y, bt.W, bt.H)
	}
}

// LoadSlot replaces the session with saved game i (the Load screen's
// rows, the main menu's Continue). A save that cannot load says why; an
// older one or one made with other content asks first (SAV-021); a
// running colony with unsaved progress asks before it is replaced (Leave).
func (b *Book) LoadSlot(i int) {
	if i == slotNextPage { // the "more saves" row of a long list
		_, _, pages := b.slotWindow()
		b.SlotPage = (b.SlotPage + 1) % max(1, pages)
		return
	}
	if i < 0 || i >= len(b.Slots) || !b.sh.SaveEnv().CanLoad {
		return
	}
	b.bg().wait("") // never read a file the queue is replacing
	sl := b.Slots[i]
	if why := badReason(b.sh, sl); why != "" {
		b.sh.Hint(why)
		return
	}
	switch savefmt.Compat(sl.Compat) {
	case savefmt.CompatOlder:
		b.confirmLoad(sl, b.sh.T(olderTag), b.sh.T(olderAsk))
		return
	case savefmt.CompatContent:
		notice := sl.Notice
		if notice == "" {
			notice = b.sh.T("its content differs from this game's")
		}
		b.confirmLoad(sl, b.sh.T(contentTag), b.sh.Tf(contentAsk, notice))
		return
	}
	b.Leave(func() { b.load(sl) })
}

// confirmLoad asks before loading sl; Load goes on at the next PollSaves
// (through Leave: unsaved progress asks too).
func (b *Book) confirmLoad(sl ports.SaveSlot, title, body string) {
	b.sh.Kit().Confirm(compatModal, title, body, b.sh.T("Load"), false, func() {
		b.progress.due = func() { b.Leave(func() { b.load(sl) }) }
	})
}

// load reads sl and makes it the running colony. A save that fails to
// load is marked damaged in the list, so it is not tried again until the
// list is read anew (SAV-022).
func (b *Book) load(sl ports.SaveSlot) {
	meta, tick, err := b.sh.Load(sl.Path)
	if err != nil {
		b.sh.Hint(b.sh.Tf("load failed: %s", err.Error()))
		for i := range b.Slots {
			if b.Slots[i].Path == sl.Path {
				b.Slots[i].Err = b.sh.T("damaged file")
			}
		}
		return
	}
	takeSession(b, meta)
	b.AutosaveDay = int(tick) / 60000
	b.progress = progressLoaded(int64(tick))
	b.sh.Hint(b.sh.Tf("loaded %s", sl.Name))
}
