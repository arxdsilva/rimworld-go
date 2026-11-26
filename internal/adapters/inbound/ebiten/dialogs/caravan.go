// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
	FormCaravanID  = "colony.world.caravan"
	formCaravanW   = 980
	formCaravanH   = 560
	formRowH       = 28
	FormNoMembers  = "Pick at least one colonist."
	FormNoDest     = "Pick a destination."
	FormTooHeavy   = "Too heavy: pick fewer goods or more pack animals."
	formHostileWhy = "Hostile: they would not receive a caravan."
)

// formCaravan is the dialog's draft: colonists, and (AX-9) pack animals
// and goods by kind.
type formCaravan struct {
	picked   map[entity.EntityID]bool
	captives map[entity.EntityID]bool // prisoners leaving with the colonists
	beasts   map[entity.EntityID]bool
	goods    map[string]int
	dest     int // -1 none
	// globe is the tile last picked on the World tab's globe (-1 none),
	// read live while the dialog is open.
	globe func() int
	memo  planMemo // the trip estimate for the current picks (caravan_plan.go)
}

// OpenFormCaravan opens the dialog over the World tab; globePick reports
// the tile last picked on its globe (-1 none).
func (dl *Dealings) OpenFormCaravan(globePick func() int) {
	f := &formCaravan{picked: map[entity.EntityID]bool{}, captives: map[entity.EntityID]bool{}, beasts: map[entity.EntityID]bool{}, goods: map[string]int{}, dest: globePick(), globe: globePick}
	dl.kit().Push(uikit.Modal{
		ID: FormCaravanID, Title: dl.sh.T("Form caravan"), W: formCaravanW, H: formCaravanH,
		Draw: func(body gfx.Rect) { dl.formCaravanBody(f, body) },
		Buttons: []uikit.ModalButton{
			{ID: "form.send", Label: dl.sh.T("Send"), Style: uikit.BtnPrimary, Closes: true,
				Disabled: func() string { return dl.formReason(f) }, OnClick: func() { dl.sendCaravan(f) }},
			{ID: "form.cancel", Label: dl.sh.T("Cancel"), Closes: true},
		},
	})
}

// reason is why Send is disabled ("" it is not).
func (f *formCaravan) reason() string {
	if len(f.members()) == 0 {
		return FormNoMembers
	}
	if f.dest < 0 {
		return FormNoDest
	}
	return ""
}

// formReason is why Send is disabled ("" it is not): f's reason, or a
// load past what the caravan carries (AX-9).
func (dl *Dealings) formReason(f *formCaravan) string {
	if r := f.reason(); r != "" {
		return r
	}
	if f.massKg(dl.sh.Items()) > f.capKg(dl) {
		return FormTooHeavy
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

// MemberList is the command's Name field: "1,2,3".
func MemberList(ids []entity.EntityID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatUint(uint64(id), 10)
	}
	return strings.Join(parts, ",")
}

// sendCaravan emits the FormCaravan command.
func (dl *Dealings) sendCaravan(f *formCaravan) {
	if dl.formReason(f) != "" {
		return
	}
	dl.sh.Enqueue(command.Command{Kind: command.FormCaravan, Name: dl.caravanList(f), Count: f.dest})
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
func (dl *Dealings) formCaravanBody(f *formCaravan, body gfx.Rect) {
	lm, right := body.SplitH(body.W * 2 / 3)
	lm, summary := lm.SplitV(lm.H - formSummaryH)
	left, mid := lm.SplitH(lm.W / 2)
	left.W, mid.W = left.W-12, mid.W-12
	dl.caravanPacks(f, mid) // caravan_pack.go (AX-9): pack animals and goods
	hs := gfx.WithColor(gfx.Label, uikit.Heading)
	lh, lrest := left.SplitV(26)
	dl.kit().Label(lh.X, lh.Y, dl.sh.T("Colonists"), hs)
	var pawns []game.PawnView
	for _, p := range dl.sh.Pawns() {
		if p.Status != "dead" {
			pawns = append(pawns, p)
		}
	}
	dl.scrollRows("form.members", lrest, append(f.colonistRows(dl, pawns), f.prisonerRows(dl, pawns)...))
	f.summary(dl, summary)

	rh, rrest := right.SplitV(26)
	dl.kit().Label(rh.X, rh.Y, dl.sh.T("Destination"), hs)
	var sites []game.WorldSiteView
	if src, ok := dl.sh.Caravans(); ok {
		sites = src.WorldSites()
	}
	rows := len(sites) + 1
	area := dl.kit().ScrollArea("form.dests", rrest, float32(rows*formRowH), uikit.ScrollOpts{Step: formRowH * 2})
	y := area.View.Y - area.Off
	row := func() (gfx.Rect, bool) {
		rr := gfx.Rect{X: area.View.X, Y: y, W: area.View.W - 4, H: formRowH - 4}
		y += formRowH
		return rr, rr.Y >= area.View.Y && rr.Bottom() <= area.View.Bottom()
	}
	if rr, vis := row(); vis {
		picked := f.globe()
		label, why := dl.sh.T("Tile picked on the globe"), "Click a tile on the World tab's globe first."
		if picked >= 0 {
			label, why = dl.sh.Tf("Tile %d (picked on the globe)", picked), ""
		}
		if dl.kit().Button("form.dest.picked", rr, label, uikit.BtnOpts{Disabled: why != "", Reason: why, Selected: why == "" && f.dest == picked}) {
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
		if dl.kit().Button(fmt.Sprintf("form.dest.%d", s.Tile), rr, gfx.Ellipsize(label, gfx.Body, float64(rr.W-12)), uikit.BtnOpts{
			Disabled: why != "", Reason: why, Selected: f.dest == s.Tile}) {
			f.dest = s.Tile
		}
	}
}

// CaravanHealthNote is the caravan row's note on its sick and hurt
// members (" · Ada: flu · Bo: bleeding"; "" when all are well; WS-M).
func CaravanHealthNote(c game.CaravanView) string {
	var b strings.Builder
	for i, h := range c.Health {
		if h == "" || i >= len(c.Members) {
			continue
		}
		b.WriteString(" · " + c.Members[i] + ": " + h)
	}
	return b.String()
}

// CaravanTip is the caravan row's hover card: its members, one a line,
// each with its health note.
func CaravanTip(c game.CaravanView) string {
	lines := []string{CaravanTitle(c)}
	for i, m := range c.Members {
		h := "well"
		if i < len(c.Health) && c.Health[i] != "" {
			h = c.Health[i]
		}
		lines = append(lines, m+": "+h)
	}
	return strings.Join(lines, "\n")
}

// CaravanTitle is the caravan's name: its members, or "Caravan".
func CaravanTitle(c game.CaravanView) string {
	if len(c.Members) == 0 {
		return "Caravan"
	}
	return "Caravan: " + strings.Join(c.Members, ", ")
}

// pawnLabel is the colonist's name in lists.
func pawnLabel(tr lang.Tr, p game.PawnView) string {
	if p.Name != "" {
		return p.Name
	}
	return lang.Tf(tr, "Colonist %d", p.ID)
}
