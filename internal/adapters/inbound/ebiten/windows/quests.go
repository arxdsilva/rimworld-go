package windows

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	qd "rimworld-go/internal/domain/quest"
)

// The Quests window (QST-120..139, UIS-066): the Available, Active and
// Historical lists over WorldView.Quests on the left, the picked quest's
// detail on the right. Opened from the HUD's Quests tab (F7) or a quest's
// letter (ports.ShowQuest). Its clicks leave as quest commands.

// QuestsModalID is the window's kit id.
const QuestsModalID = "colony.quests"

// The list's tabs.
const (
	questsAvail = iota
	questsActive
	questsHistory
)

// questsState is the window's per-session UI state.
type questsState struct {
	tab int    // questsAvail, questsActive or questsHistory
	sel uint32 // the picked quest's id (0: the tab's first)
}

// Toggle opens the window, or closes it when it is the open one.
func (w Quests) Toggle() { w.toggleKit(QuestsModalID, w.Launch) }

// Launch opens the window on its last tab and pick: a main-tab window in
// the Window rect (window.go).
func (w Quests) Launch() {
	if !w.live() {
		return
	}
	w.OpenKit(uikit.Modal{
		ID: QuestsModalID, Title: w.sh.T("Quests"), W: hudlayout.QuestsW, H: hudlayout.QuestsH, Anchor: w.Anchor, Frameless: true,
		Draw: func(body gfx.Rect) { w.body(body) },
	})
}

// Select opens the window on quest id: the tab its state belongs to, its
// row picked (ports.ShowQuest). An id not in the read model only opens it.
func (w Quests) Select(id uint32) {
	for _, q := range w.sh.World().Quests {
		if uint32(q.ID) == id {
			w.quests.tab, w.quests.sel = questTab(q.State, q.Dismissed), id
		}
	}
	w.Launch()
}

// size is the window's outer size.
func (w Quests) size() (float32, float32) { return hudlayout.QuestsW, hudlayout.QuestsH }

// body declares the list and the detail (QST-3 D-ui-window fills it).
func (w Quests) body(body gfx.Rect) {}

// questTab is the list a quest in state s shows under: a dismissed quest
// and every ended one under Historical (QST-130).
func questTab(s qd.State, dismissed bool) int {
	switch {
	case dismissed || s != qd.Offered && s != qd.Active:
		return questsHistory
	case s == qd.Active:
		return questsActive
	}
	return questsAvail
}
