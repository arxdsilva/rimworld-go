// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"fmt"
	"strconv"

	"github.com/tetratelabs/wazero/api"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/pkg/randx"
)

// Fire runs every live module's hook for ev. The first call also returns
// the files that failed to load, so the game reports them. p answers
// pawns() and pawn(id) during this call only; it may be nil.
//
// on_pawn runs one fuel budget and one command budget per file per tick,
// shared by all the pawns of the tick.
func (h *Host) Fire(ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	return h.fire(h.mods, ev, w, p)
}

// fire is Fire over mods (all files, or one plugin's view); the unsent
// problems go out in whichever Result comes first.
func (h *Host) fire(mods []*module, ev scripting.Event, w scripting.World, p scripting.PawnReader) scripting.Result {
	var res scripting.Result
	res.Disabled, h.unsent, h.loadUnsent = h.unsent, nil, 0
	if len(ev.Pawns) == 0 && (ev.Hook == scripting.OnPawns || ev.Hook == scripting.OnPawn) {
		return res // nobody is due: no call
	}
	if int(ev.Hook) >= hookSlots {
		return res
	}
	for _, m := range mods {
		fn := m.hooks[ev.Hook]
		if m.off || fn == nil {
			continue
		}
		cs := &callState{m: m, hook: ev.Hook.String(), ev: ev, w: w, pr: p, what: "one call"}
		cs.rekey(fnv64a(ev.Incident.Key + "\x00" + ev.Name))
		h.begin(cs)
		msg := h.runHook(cs, fn)
		h.end(cs)
		if msg != "" {
			res.Disabled = append(res.Disabled, h.disable(m, msg))
			continue // its commands are dropped
		}
		res.Commands = append(res.Commands, cs.cmds...)
	}
	return res
}

// rekey restarts the rand stream under sub (on_pawn keys it by pawn id).
func (c *callState) rekey(sub uint64) {
	c.rng = randx.NewKeyed(c.ev.Seed, c.ev.Tick, c.m.key, uint64(c.ev.Hook), sub)
}

// runHook writes the event and calls the hook, once, or once per due pawn
// for on_pawn; the first failure stops the file.
func (h *Host) runHook(cs *callState, fn api.Function) string {
	ev := cs.ev
	if ev.Hook != scripting.OnPawn {
		if msg := h.writeEvent(cs.m, ev, ev.Pawns); msg != "" {
			return msg
		}
		_, msg := h.call(cs, fn)
		return msg
	}
	cs.what = "on_pawn's per-tick budget"
	for i := range ev.Pawns {
		one := ev.Pawns[i : i+1]
		cs.rekey(fnv64a(strconv.FormatUint(one[0].ID, 10)))
		if msg := h.writeEvent(cs.m, ev, one); msg != "" {
			return msg
		}
		if _, msg := h.call(cs, fn); msg != "" {
			return msg
		}
	}
	return ""
}

// writeEvent encodes ev with pawns into the module's buffer.
func (h *Host) writeEvent(m *module, ev scripting.Event, pawns []scripting.Pawn) string {
	if n := eventSize(ev, len(pawns)); n > int(m.bufSize) {
		return fmt.Sprintf("event of %d bytes does not fit the %d byte buffer", n, m.bufSize)
	}
	h.scratch = appendEvent(h.scratch[:0], ev, pawns)
	if !m.mem.Write(m.buf, h.scratch) {
		return "event does not fit in memory"
	}
	return ""
}
