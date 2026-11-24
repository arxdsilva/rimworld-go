// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"fmt"

	"github.com/tetratelabs/wazero/api"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/pkg/randx"
)

// callState is one top-level call's state: what it read, drew and queued.
// It lives for the call (for on_pawn, for the file's tick).
type callState struct {
	m    *module
	hook string // names the call in a problem
	ev   scripting.Event
	w    scripting.World // nil outside a hook
	pr   scripting.PawnReader
	rng  randx.Keyed
	cmds []command.Command
	what string // names the command budget in an overrun
	fail string // the import's own message, when an import failed the call
}

// stopSignal is the one internal stop an import panics with; wazero turns
// it into the call's error, and the host reads callState.fail or the
// globals, never the text.
type stopSignal struct{}

// begin sets the call's fuel budget and makes cs the active call.
func (h *Host) begin(cs *callState) {
	h.cur = cs
	cs.m.fuel.Set(h.opts.Fuel)
}

// end accounts the fuel spent since begin and clears the active call.
func (h *Host) end(cs *callState) {
	m := cs.m
	left := int64(m.fuel.Get()) // #nosec G115 -- the fuel global holds i64 bits (wasmrewrite meter)
	switch {
	case left < 0: // out of fuel: the whole budget went
		left = 0
	case uint64(left) > h.opts.Fuel:
		left = int64(h.opts.Fuel) // #nosec G115 -- Fuel < uint64(left) <= MaxInt64 by the case at host_call.go:48
	}
	m.spent += h.opts.Fuel - uint64(left)
	h.cur = nil
}

// call runs fn once inside a begun call: depth reset, one Call counted. It
// returns the problem's message, or "" on success.
func (h *Host) call(cs *callState, fn api.Function, args ...uint64) ([]uint64, string) {
	m := cs.m
	m.depth.Set(0)
	m.calls++
	r, err := fn.Call(h.ctx, args...)
	if err != nil {
		return nil, h.errMsg(cs, err)
	}
	return r, ""
}

// once is begin, call and end for a call with its own budget.
func (h *Host) once(m *module, hook string, fn api.Function, args ...uint64) ([]uint64, string) {
	cs := &callState{m: m, hook: hook, what: "one call"}
	h.begin(cs)
	r, msg := h.call(cs, fn, args...)
	h.end(cs)
	return r, msg
}

// errMsg decides the problem text of a failed call: the import's message,
// else out of fuel or too deep from the reserved globals, else the trap.
func (h *Host) errMsg(cs *callState, err error) string {
	switch {
	case cs.fail != "":
		return cs.fail
	case int64(cs.m.fuel.Get()) < 0: // #nosec G115 -- the fuel global holds i64 bits (wasmrewrite meter)
		return fmt.Sprintf("out of fuel in %s (budget %d)", cs.hook, h.opts.Fuel)
	case cs.m.depth.Get() > StackBudget:
		return "call stack over budget in " + cs.hook
	}
	return fmt.Sprintf("trap in %s: %s", cs.hook, firstLine(err))
}
