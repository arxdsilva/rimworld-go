package ebiten

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The Form Caravan dialog (MAIN_UI_SCREENS §34, ENG-003 minimal slice):
// pick colonists (downed and drafted ones listed disabled) and a
// destination (a settlement from the list, or the tile picked on the
// globe), see the summary, Send. It emits FormCaravan (Name = the member
// ids, Count = the tile); the simulation walks them to the map edge.

const (
	formCaravanID   = "colony.world.caravan"
	formCaravanW    = 720
	formCaravanH    = 520
	formRowH        = 28
	formNoMembers   = "Pick at least one colonist."
	formNoDest      = "Pick a destination."
	formHostileWhy  = "Hostile: they would not receive a caravan."
	formTilesPerDay = 3 // caravan.BaseTilesPerDay at full Moving, for the summary
)

// formCaravan is the dialog's draft.
type formCaravan struct {
	picked map[entity.EntityID]bool
	dest   int      // -1 none
	world  *worldUI // the World tab it was opened from (its globe pick)
}

// openFormCaravan opens the dialog over the World tab.
func (a *App) openFormCaravan(st *worldUI) {
	f := &formCaravan{picked: map[entity.EntityID]bool{}, dest: st.picked, world: st}
	a.uiPushModal(modal{
		id: formCaravanID, title: "Form caravan", w: formCaravanW, h: formCaravanH,
		draw: func(a *App, body rect) { a.formCaravanBody(f, body) },
		buttons: []modalButton{
			{id: "form.send", label: "Send", style: btnPrimary, closes: true,
				disabled: func() string { return f.reason() }, onClick: func() { a.sendCaravan(f) }},
			{id: "form.cancel", label: "Cancel", closes: true},
		},
	})
}

// reason is why Send is disabled ("" it is not).
func (f *formCaravan) reason() string {
	if len(f.members()) == 0 {
		return formNoMembers
	}
	if f.dest < 0 {
		return formNoDest
	}
	return ""
}

// members are the picked colonists, ascending.
func (f *formCaravan) members() []entity.EntityID {
	var out []entity.EntityID
	for id, on := range f.picked {
		if on {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// memberList is the command's Name field: "1,2,3".
func memberList(ids []entity.EntityID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatUint(uint64(id), 10)
	}
	return strings.Join(parts, ",")
}

// sendCaravan emits the FormCaravan command.
func (a *App) sendCaravan(f *formCaravan) {
	if f.reason() != "" || a.renderer == nil {
		return
	}
	a.renderer.enqueue(command.Command{Kind: command.FormCaravan, Name: memberList(f.members()), Count: f.dest})
}

// caravanPickReason is why colonist p cannot join ("" it can).
func caravanPickReason(p game.PawnView) string {
	switch {
	case p.Status != "alive":
		return "Down: cannot travel."
	case p.Drafted:
		return "Drafted: undraft to send on a caravan."
	}
	return ""
}

// formCaravanBody declares the colonist picks, the destinations and the
// summary.
func (a *App) formCaravanBody(f *formCaravan, body rect) {
	if a.renderer == nil {
		return
	}
	left, right := body.splitH(body.W / 2)
	left.W -= 12
	hs := withColor(tsLabel, uiHeading)
	lh, lrest := left.splitV(26)
	a.uiLabel(lh.X, lh.Y, "Colonists", hs)
	var pawns []game.PawnView
	for _, p := range a.renderer.snapshot() {
		if p.Status != "dead" {
			pawns = append(pawns, p)
		}
	}
	lrest, summary := lrest.splitV(lrest.H - 56)
	if len(pawns) == 0 {
		a.uiLabel(lrest.X, lrest.Y, "No colonists.", dimText())
	} else {
		area := a.uiScrollArea("form.members", lrest, float32(len(pawns)*formRowH), scrollOpts{Step: formRowH * 2})
		y := area.View.Y - area.Off
		for _, p := range pawns {
			rr := rect{area.View.X, y, area.View.W, formRowH - 4}
			y += formRowH
			if rr.Y < area.View.Y || rr.bottom() > area.View.bottom() {
				continue
			}
			why := caravanPickReason(p)
			on := f.picked[p.ID] && why == ""
			if a.uiCheck(fmt.Sprintf("form.member.%d", p.ID), rr, pawnLabel(p), &on, checkOpts{Disabled: why != "", Reason: why}) {
				f.picked[p.ID] = on
			}
		}
	}
	n := len(f.members())
	a.uiLabel(summary.X, summary.Y+6, fmt.Sprintf("%d colonist(s) · about %d tiles a day · 5 days of food", n, formTilesPerDay), tsSmall)
	a.uiLabel(summary.X, summary.Y+26, "They walk to the map edge first; a friendly settlement hosts them a day.", withColor(tsSmall, uiTextDim))

	rh, rrest := right.splitV(26)
	a.uiLabel(rh.X, rh.Y, "Destination", hs)
	var sites []game.WorldSiteView
	if src, ok := a.renderer.game.(worldSource); ok {
		sites = src.WorldSites()
	}
	rows := len(sites) + 1
	area := a.uiScrollArea("form.dests", rrest, float32(rows*formRowH), scrollOpts{Step: formRowH * 2})
	y := area.View.Y - area.Off
	row := func() (rect, bool) {
		rr := rect{area.View.X, y, area.View.W - 4, formRowH - 4}
		y += formRowH
		return rr, rr.Y >= area.View.Y && rr.bottom() <= area.View.bottom()
	}
	if rr, vis := row(); vis {
		picked := f.world.picked
		label, why := "Tile picked on the globe", "Click a tile on the World tab's globe first."
		if picked >= 0 {
			label, why = fmt.Sprintf("Tile %d (picked on the globe)", picked), ""
		}
		if a.uiButton("form.dest.picked", rr, label, btnOpts{Disabled: why != "", Reason: why, Selected: why == "" && f.dest == picked}) {
			f.dest = picked
		}
	}
	for _, s := range sites {
		rr, vis := row()
		if !vis {
			continue
		}
		why := ""
		if !s.Visitable {
			why = formHostileWhy
		}
		label := fmt.Sprintf("%s (%s, %s)", s.Name, s.Faction, s.Relation)
		if a.uiButton(fmt.Sprintf("form.dest.%d", s.Tile), rr, ellipsize(label, tsBody, float64(rr.W-12)), btnOpts{
			Disabled: why != "", Reason: why, Selected: f.dest == s.Tile}) {
			f.dest = s.Tile
		}
	}
}

// caravanHealthNote is the caravan row's note on its sick and hurt
// members (" · Ada: flu · Bo: bleeding"; "" when all are well; WS-M).
func caravanHealthNote(c game.CaravanView) string {
	var b strings.Builder
	for i, h := range c.Health {
		if h == "" || i >= len(c.Members) {
			continue
		}
		b.WriteString(" · " + c.Members[i] + ": " + h)
	}
	return b.String()
}

// caravanTip is the caravan row's hover card: its members, one a line,
// each with its health note.
func caravanTip(c game.CaravanView) string {
	lines := []string{caravanTitle(c)}
	for i, m := range c.Members {
		h := "well"
		if i < len(c.Health) && c.Health[i] != "" {
			h = c.Health[i]
		}
		lines = append(lines, m+": "+h)
	}
	return strings.Join(lines, "\n")
}
