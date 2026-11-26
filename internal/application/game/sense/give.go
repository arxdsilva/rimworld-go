// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package sense

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Give plans a physical handover of a pack entry to another colonist.
// Count zero means the entry, up to one hand load. Nothing is removed
// from the pack until the job runs.
func (g Gear) Give(carrier, to entity.EntityID, entry, count int) (job.Record, string) {
	if side, up := g.side(carrier); side != SideColonist || !up || g.r.Breaking(carrier) {
		return job.Record{}, WhyNotOurs
	}
	if side, up := g.side(to); carrier == to || side != SideColonist || !up {
		return job.Record{}, "recipient is gone"
	}
	e, ok := entryOf(g.s.Packs[carrier], entry)
	if !ok || e.Stack.Qty <= 0 || count < 0 || e.Stack.Kind.IsCorpse() {
		return job.Record{}, WhyGone
	}
	if count == 0 {
		count = e.Stack.Qty
	}
	count = min(count, e.Stack.Qty, item.CarryLimit, g.r.Items().StackLimit(e.Stack.Kind))
	if count <= 0 {
		return job.Record{}, WhyGone
	}
	x, y := g.r.Pos(carrier)
	return job.Record{Kind: job.GiveItem, Dest: to, TX: x, TY: y, BillIndex: entry, Count: count, WorkLeft: unloadWait, Forced: true}, ""
}

// DoGive moves the actual pack stack, follows a moving recipient and
// deposits it in their pack (or at their feet if it will not fit).
func (g Gear) DoGive(id entity.EntityID, j job.Record) {
	if side, up := g.side(j.Dest); side != SideColonist || !up {
		g.giveBack(id, j)
		return
	}
	if j.WorkLeft > 0 {
		j.WorkLeft--
		g.r.SetJob(id, j)
		return
	}
	if j.Carry == 0 {
		stack, ok := g.TakeOut(id, uint32(j.BillIndex), j.Count)
		if !ok {
			g.end(id, j, false)
			return
		}
		j.Carry = stack
		g.r.SetJob(id, j)
	}
	x, y := g.r.Pos(id)
	px, py := g.r.Pos(j.Dest)
	if !pathing.Adjacent(x, y, px, py) && (x != px || y != py) {
		j.TX, j.TY = px, py
		if !g.r.StartJob(id, j) {
			g.giveBack(id, j)
		}
		return
	}
	st, ok := g.r.Stack(j.Carry)
	if !ok || st.Carrier != id {
		g.giveBack(id, j)
		return
	}
	if g.r.Items().MassGrams(st.Kind)*st.Qty <= g.Room(j.Dest) {
		g.r.RemoveQty(j.Carry, st.Qty)
		p := g.s.Packs[j.Dest]
		p.Add(st)
		g.setPack(j.Dest, p)
	} else {
		g.r.Lay(j.Dest, j.Carry)
	}
	delete(g.s.FromPack, id)
	j.Carry = 0
	g.end(id, j, true)
}

func (g Gear) giveBack(id entity.EntityID, j job.Record) {
	if j.Carry != 0 {
		if st, ok := g.r.Stack(j.Carry); ok && st.Carrier == id && !g.BackToPack(id, j.Carry) {
			g.r.Lay(id, j.Carry)
		}
		delete(g.s.FromPack, id)
	}
	j.Carry = 0
	g.end(id, j, false)
}
