package dossier

import (
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

const manageLabel = "Manage..."

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

func (m menus) pickArea() {
	if m.p.sc == nil {
		return
	}
	areas := m.p.r.Areas()
	refs := []int{command.AreaUnrestricted, command.AreaHome}
	opts := []string{"Unrestricted", "Home"}
	for _, a := range areas {
		refs = append(refs, int(a.ID))
		opts = append(opts, a.Name)
	}
	cur, now := 0, m.p.sc.AreaOf(m.p.id)
	if now == 0 {
		now = command.AreaUnrestricted
	}
	for i, r := range refs {
		if r == now {
			cur = i
		}
	}
	opts = append(opts, manageLabel)
	uikit.Picker(m.p.k, "dossier.area", "Allowed area", opts, cur, func(i int) { m.areaChoice(refs, i) })
}

// areaChoice runs option i of the area menu: a ref, or the manager.
func (m menus) areaChoice(refs []int, i int) {
	if i >= len(refs) {
		m.p.a.ManageAreas()
		return
	}
	m.p.a.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(m.p.id), Index: refs[i]})
}

// pickHour sets what the schedule has for the current hour.
func (m menus) pickHour() {
	if m.p.sc == nil {
		return
	}
	hour := m.p.sc.Hour()
	opts := []string{"Anything", "Work", "Recreation", "Sleep"}
	uikit.Picker(m.p.k, "dossier.hour", "Hour "+strconv.Itoa(hour), opts, int(m.p.d.p.Activity),
		func(i int) { m.hourChoice(hour, i) })
}

// hourChoice sets hour to activity i for the selected person.
func (m menus) hourChoice(hour, i int) {
	m.p.a.Enqueue(command.Command{Kind: command.SetSchedule, Pawn: uint64(m.p.id), Index: hour, Mode: uint8(i)})
}
