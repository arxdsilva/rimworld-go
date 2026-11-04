package dossier

import (
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

const (
	manageLabel    = "Manage..."
	hourActivities = 4 // Anything, Work, Recreation, Sleep
)

// menus are the card's selectors and click-throughs.
type menus struct{ p *Panel }

// activate runs a click on an item: its selector, then its tab.
func (m menus) activate(it *item) {
	switch it.act {
	case actDiet:
		m.pickDiet()
	case actArea:
		m.pickArea()
	case actHour:
		m.pickHour()
	}
	if it.tab != "" {
		m.openTab(it.tab)
	}
}

// openTab opens an inspector tab, or the Work window.
func (m menus) openTab(tab string) {
	if tab == sheet.TabWork {
		m.p.a.ShowWindow(ids.TabWork)
		return
	}
	m.p.a.OpenTab(tab)
}

func (m menus) pickDiet() {
	if m.p.sc == nil {
		return
	}
	pols := m.p.sc.FoodPolicies()
	cur, now := -1, m.p.d.p.FoodPolicy
	if m.p.d.animal {
		now = m.p.d.a.FoodPolicy
	}
	opts := make([]string, 0, len(pols)+1)
	for i, pol := range pols {
		opts = append(opts, pol.Name)
		if pol.Name == now {
			cur = i
		}
	}
	opts = append(opts, manageLabel)
	uikit.Picker(m.p.k, "dossier.diet", "Food policy", opts, cur, func(i int) { m.dietChoice(pols, i) })
}

// dietChoice runs option i of the food menu: a policy, or the editor.
func (m menus) dietChoice(pols []game.FoodPolicyView, i int) {
	if i >= len(pols) {
		m.p.a.ShowWindow(ids.TabAssign)
		return
	}
	m.p.a.Enqueue(command.Command{Kind: command.SetFoodPolicy, Pawn: uint64(m.p.id), Index: pols[i].ID})
}

// pickArea opens the game's allowed area list: hovering a row draws that area
// on the map, and its Manage row opens the manager (RHUD-089, 092, 094).
func (m menus) pickArea() {
	if m.p.sc == nil {
		return
	}
	m.p.a.AreaPicker("Allowed area", m.p.sc.AreaOf(m.p.id), true, m.areaChoice)
}

// areaChoice assigns the chosen area.
func (m menus) areaChoice(ref int) {
	m.p.a.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(m.p.id), Index: ref})
}

// pickHour sets what the schedule has for the current hour.
func (m menus) pickHour() {
	if m.p.sc == nil {
		return
	}
	hour := m.p.sc.Hour()
	opts := []string{"Anything", "Work", "Recreation", "Sleep", manageLabel}
	uikit.Picker(m.p.k, "dossier.hour", "Hour "+strconv.Itoa(hour), opts, int(m.p.d.p.Activity),
		func(i int) { m.hourChoice(hour, i) })
}

// hourChoice sets hour to activity i for the selected person; the row after
// the activities opens the Schedule window (RHUD-100).
func (m menus) hourChoice(hour, i int) {
	if i >= hourActivities {
		m.p.a.ShowWindow(ids.TabSchedule)
		return
	}
	m.p.a.Enqueue(command.Command{Kind: command.SetSchedule, Pawn: uint64(m.p.id), Index: hour, Mode: uint8(i)})
}
