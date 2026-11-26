// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"slices"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// The Form caravan dialog's prisoners, food and trip plan (#14): prisoners
// leave with the colonists and eat like them, the picked food is weighed
// against the simulation's travel estimate, and the shortfall is warned
// about (Send stays enabled: a caravan may forage or resupply).

const formSummaryH = 96 // the summary under the colonist and goods columns

// planMemo is the last FormEstimate answer and what it was asked for, so
// the road is walked again only when the eaters, the destination or the
// load change, not every frame.
type planMemo struct {
	ids   []entity.EntityID
	dest  int
	load  int32
	ticks int
	why   string
	ok    bool
}

// prisoners are the picked prisoners, ascending.
func (f *formCaravan) prisoners() []entity.EntityID {
	var out []entity.EntityID
	for id, on := range f.captives {
		if on {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// eaters are the caravan's mouths: the colonists, then the prisoners.
func (f *formCaravan) eaters() []entity.EntityID {
	return append(f.members(), f.prisoners()...)
}

// loadPermille is the goods' share of what the caravan carries, in
// thousandths (what the estimate and the pace take).
func (f *formCaravan) loadPermille(dl *Dealings) int32 {
	c := f.capKg(dl)
	if c <= 0 {
		return 0
	}
	return int32(1000 * min(f.massKg(dl.sh.Items()), c) / c)
}

// nutrition is the edible nutrition of the picked goods (needs units).
func (f *formCaravan) nutrition() int {
	var stacks []item.Stack
	for name, n := range f.goods {
		if k, ok := item.ParseKind(name); ok {
			stacks = append(stacks, item.Stack{Kind: k, Qty: n})
		}
	}
	return caravan.Nutrition(stacks)
}

// plan is the simulation's trip estimate for the eaters to f.dest (ok false:
// nothing to plan, or no planner). It asks only when the picks changed.
func (f *formCaravan) plan(dl *Dealings, eat []entity.EntityID) (ticks int, why string, ok bool) {
	src, has := dl.sh.Caravans()
	if !has || len(eat) == 0 || f.dest < 0 {
		return 0, "", false
	}
	load := f.loadPermille(dl)
	if m := &f.memo; m.ok && m.dest == f.dest && m.load == load && slices.Equal(m.ids, eat) {
		return m.ticks, m.why, true
	}
	t, w := src.FormEstimate(eat, f.dest, load)
	f.memo = planMemo{ids: slices.Clone(eat), dest: f.dest, load: load, ticks: t, why: w, ok: true}
	return t, w, true
}

// planWhy is the estimate's reason in the player's language. The cases are
// the simulation's own strings (maps.Plan), listed so the catalog has them.
func planWhy(dl *Dealings, why string) string {
	switch why {
	case "No world map.":
		return dl.sh.T("No world map.")
	case "No way to get there.":
		return dl.sh.T("No way to get there.")
	case "Too slow to get there.":
		return dl.sh.T("Too slow to get there.")
	case "Pick a destination for the caravan.":
		return dl.sh.T("Pick a destination for the caravan.")
	}
	return why
}

// summaryLines are the summary's two lines: who goes and what they carry,
// and the food against the trip (short: the food runs out before arrival).
func (f *formCaravan) summaryLines(dl *Dealings) (head, line string, short bool) {
	eat := f.eaters()
	head = dl.sh.Tf("%d colonist(s), %d prisoner(s) · %s", len(f.members()), len(f.prisoners()), dl.caravanLoadLine(f))
	food := float64(caravan.Days(f.nutrition(), len(eat))) / 1000
	line = dl.sh.Tf("Food: %.1f days", food)
	if ticks, why, ok := f.plan(dl, eat); ok {
		if why != "" {
			return head, dl.sh.Tf("Food: %.1f days · travel: %s", food, planWhy(dl, why)), false
		}
		travel := float64(ticks) / float64(clock.TicksPerDay)
		return head, dl.sh.Tf("Food: %.1f days · travel: %.1f days", food, travel), food < travel
	}
	return head, line, false
}

// caravanSummary declares the summary in r: who goes and what they carry,
// the food against the trip, and the standing hint.
func (f *formCaravan) summary(dl *Dealings, r gfx.Rect) {
	k := dl.kit()
	head, line, short := f.summaryLines(dl)
	k.Label(r.X, r.Y+4, head, gfx.Small)
	k.Label(r.X, r.Y+24, line, gfx.Small)
	if short {
		k.Label(r.X, r.Y+44, dl.sh.T("Not enough food for the trip."), gfx.WithColor(gfx.Small, uikit.Warning))
	}
	k.Label(r.X, r.Y+64, dl.sh.T("They gather at a hitching spot if there is one, then leave by the map edge; a friendly settlement hosts them a day."), gfx.WithColor(gfx.Small, uikit.TextDim))
}

// colonistRows are the colonists' pick rows.
func (f *formCaravan) colonistRows(dl *Dealings, pawns []game.PawnView) []func(gfx.Rect) {
	var rows []func(gfx.Rect)
	for _, p := range pawns {
		if p.Prisoner {
			continue
		}
		rows = append(rows, func(rr gfx.Rect) {
			why := caravanPickReason(p)
			on := f.picked[p.ID] && why == ""
			if dl.kit().Check(formKey("form.member.", p.ID), rr, pawnLabel(dl.sh, p), &on, uikit.CheckOpts{Disabled: why != "", Reason: why}) {
				f.picked[p.ID] = on
			}
		})
	}
	if len(rows) == 0 {
		rows = append(rows, func(rr gfx.Rect) {
			dl.kit().Label(rr.X, rr.Y+4, dl.sh.T("No colonists."), uikit.DimText())
		})
	}
	return rows
}

// prisonerRows are the "Prisoners" heading and the prisoners' pick rows
// ("No prisoners." when there are none).
func (f *formCaravan) prisonerRows(dl *Dealings, pawns []game.PawnView) []func(gfx.Rect) {
	rows := []func(gfx.Rect){headingRow(dl, dl.sh.T("Prisoners"))}
	n := 0
	for _, p := range pawns {
		if !p.Prisoner {
			continue
		}
		n++
		rows = append(rows, func(rr gfx.Rect) {
			why := ""
			if p.Status != "alive" {
				why = "Down: cannot travel."
			}
			on := f.captives[p.ID] && why == ""
			if dl.kit().Check(formKey("form.prisoner.", p.ID), rr, pawnLabel(dl.sh, p), &on, uikit.CheckOpts{Disabled: why != "", Reason: why}) {
				f.captives[p.ID] = on
			}
		})
	}
	if n == 0 {
		rows = append(rows, func(rr gfx.Rect) {
			dl.kit().Label(rr.X, rr.Y+4, dl.sh.T("No prisoners."), uikit.DimText())
		})
	}
	return rows
}

// formKey is a widget id of prefix and id.
func formKey(prefix string, id entity.EntityID) string {
	return prefix + strconv.FormatUint(uint64(id), 10)
}
