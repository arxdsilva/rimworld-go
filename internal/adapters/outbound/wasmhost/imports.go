// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"context"
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero/api"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

// importCost is what every import charges the fuel global, before the
// records or 32-byte words it moves.
const importCost = 4

var (
	vI32 = api.ValueTypeI32
	vI64 = api.ValueTypeI64
)

// importDef is one function of the "rimworld" module; fn works on the
// active call's state and the wazero stack (params in, results out).
type importDef struct {
	name            string
	params, results []api.ValueType
	fn              func(c *callState, s []uint64)
}

func importDefs() []importDef {
	p := func(t ...api.ValueType) []api.ValueType { return t }
	return []importDef{
		{"message", p(vI32, vI32), nil, (*callState).message},
		{"letter", p(vI32, vI32, vI32, vI32), nil, (*callState).letter},
		{"spawn", p(vI32, vI32, vI32), nil, (*callState).spawn},
		{"rand", p(vI32), p(vI32), (*callState).rand},
		{"count", p(vI32, vI32), p(vI32), (*callState).count},
		{"day", nil, p(vI32), (*callState).day},
		{"hour", nil, p(vI32), (*callState).hour},
		{"colonists", nil, p(vI32), (*callState).colonists},
		{"wealth", nil, p(vI64), (*callState).wealth},
		{"season", p(vI32, vI32), p(vI32), (*callState).season},
		{"pawns", p(vI32, vI32), p(vI32), (*callState).pawns},
		{"pawn", p(vI64, vI32), p(vI32), (*callState).pawn},
		{"pawn_name", p(vI64, vI32, vI32), p(vI32), (*callState).pawnName},
	}
}

// registerImports instantiates the "rimworld" host module in h's runtime.
// Each function runs on h.cur, the one active call.
func (h *Host) registerImports() error {
	b := h.rt.NewHostModuleBuilder(Namespace)
	for _, d := range importDefs() {
		b = b.NewFunctionBuilder().
			WithGoModuleFunction(api.GoModuleFunc(func(_ context.Context, _ api.Module, s []uint64) {
				d.fn(h.cur, s)
			}), d.params, d.results).
			Export(d.name)
	}
	_, err := b.Instantiate(h.ctx)
	return err
}

// stopf fails the whole call with the import's own message.
func (c *callState) stopf(format string, a ...any) {
	c.fail = fmt.Sprintf(format, a...)
	panic(stopSignal{})
}

// charge takes cost from the fuel global; below 0 the call stops at once.
func (c *callState) charge(cost uint64) {
	left := int64(c.m.fuel.Get()) - int64(cost)
	c.m.fuel.Set(uint64(left))
	if left < 0 {
		panic(stopSignal{})
	}
}

// words is how many 32-byte units n bytes make.
func words(n uint32) uint64 { return (uint64(n) + 31) / 32 }

// world is the call's World; outside a hook (load, save) there is none.
func (c *callState) world(name string) scripting.World {
	if c.w == nil {
		c.stopf("%s: not available outside a hook", name)
	}
	return c.w
}

// view is the n bytes at p in guest memory, bounds-checked. It is a view,
// not a copy: the caller charges fuel, then copies.
func (c *callState) view(name string, p, n uint32) []byte {
	b, ok := c.m.mem.Read(p, n)
	if !ok {
		c.stopf("%s: pointer out of range", name)
	}
	return b
}

// within checks that n bytes at p lie in guest memory, before any charge.
func (c *callState) within(name string, p, n uint32) {
	c.view(name, p, n)
}

// put writes b at p in guest memory, bounds-checked.
func (c *callState) put(name string, p uint32, b []byte) {
	if !c.m.mem.Write(p, b) {
		c.stopf("%s: pointer out of range", name)
	}
}

func (c *callState) checkText(what, s string) {
	switch {
	case strings.TrimSpace(s) == "":
		c.stopf("%s: empty text", what)
	case len(s) > scripting.MaxText:
		c.stopf("%s: longer than %d bytes", what, scripting.MaxText)
	}
}

// queue adds a command within the call's budget.
func (c *callState) queue(name string, cmd command.Command) {
	if c.w == nil {
		c.stopf("%s: not available outside a hook", name)
	}
	if len(c.cmds) >= scripting.MaxCommands {
		c.stopf("more than %d commands in %s", scripting.MaxCommands, c.what)
	}
	c.cmds = append(c.cmds, cmd)
}

func (c *callState) message(s []uint64) {
	p, n := api.DecodeU32(s[0]), api.DecodeU32(s[1])
	b := c.view("message", p, n)
	c.charge(importCost + words(n))
	text := string(b)
	c.checkText("message", text)
	c.queue("message", command.Command{Kind: command.ScriptMessage, Mode: command.ScriptNote, Name: text})
}

func (c *callState) letter(s []uint64) {
	tp, tn, p, n := api.DecodeU32(s[0]), api.DecodeU32(s[1]), api.DecodeU32(s[2]), api.DecodeU32(s[3])
	bt, bx := c.view("letter", tp, tn), c.view("letter", p, n)
	c.charge(importCost + words(tn) + words(n))
	title, text := string(bt), string(bx)
	if strings.Contains(title, "\n") {
		c.stopf("letter: the title is one line")
	}
	c.checkText("letter title", title)
	c.checkText("letter", text)
	c.queue("letter", command.Command{Kind: command.ScriptMessage, Mode: command.ScriptLetter, Name: title + "\n" + text})
}

func (c *callState) spawn(s []uint64) {
	kp, kn, n := api.DecodeU32(s[0]), api.DecodeU32(s[1]), int(api.DecodeI32(s[2]))
	b := c.view("spawn", kp, kn)
	c.charge(importCost + words(kn))
	k := string(b)
	if n < 1 || n > scripting.MaxSpawn {
		c.stopf("spawn: count must be 1 to %d, got %d", scripting.MaxSpawn, n)
	}
	if _, ok := c.world("spawn").ItemCount(k); !ok {
		c.stopf("spawn: unknown item %q", k)
	}
	c.queue("spawn", command.Command{Kind: command.ScriptSpawn, Name: k, Count: n})
}

func (c *callState) rand(s []uint64) {
	n := int(api.DecodeI32(s[0]))
	c.charge(importCost)
	if n < 1 || n > 1<<30 {
		c.stopf("rand: n must be 1 to %d, got %d", 1<<30, n)
	}
	s[0] = api.EncodeI32(int32(c.rng.IntN(n)))
}

func (c *callState) count(s []uint64) {
	kp, kn := api.DecodeU32(s[0]), api.DecodeU32(s[1])
	b := c.view("count", kp, kn)
	c.charge(importCost + words(kn))
	k := string(b)
	n, ok := c.world("count").ItemCount(k)
	if !ok {
		c.stopf("count: unknown item %q", k)
	}
	s[0] = api.EncodeI32(clampI32(n))
}

func clampI32(n int) int32 {
	switch {
	case n > 1<<31-1:
		return 1<<31 - 1
	case n < -1<<31:
		return -1 << 31
	}
	return int32(n)
}

func (c *callState) day(s []uint64) {
	c.charge(importCost)
	s[0] = api.EncodeI32(clampI32(c.world("day").Day()))
}

func (c *callState) hour(s []uint64) {
	c.charge(importCost)
	s[0] = api.EncodeI32(clampI32(c.world("hour").Hour()))
}

func (c *callState) colonists(s []uint64) {
	c.charge(importCost)
	s[0] = api.EncodeI32(clampI32(c.world("colonists").Colonists()))
}

func (c *callState) wealth(s []uint64) {
	c.charge(importCost)
	s[0] = uint64(int64(c.world("wealth").Wealth()))
}

func (c *callState) season(s []uint64) {
	p, capacity := api.DecodeU32(s[0]), api.DecodeU32(s[1])
	str := c.world("season").Season()
	n := uint32(len(str))
	if capacity < n {
		n = capacity
	}
	c.within("season", p, n)
	c.charge(importCost + words(n))
	c.put("season", p, []byte(str[:n]))
	s[0] = api.EncodeI32(clampI32(len(str)))
}

// pawns writes min(total, cap) records and returns the total.
func (c *callState) pawns(s []uint64) {
	p, capacity := api.DecodeU32(s[0]), api.DecodeU32(s[1])
	var all []scripting.Pawn
	if c.pr != nil {
		all = c.pr.Pawns()
	}
	n := uint32(len(all))
	if capacity < n {
		n = capacity
	}
	c.within("pawns", p, n*recordSize)
	c.charge(importCost + uint64(n))
	if n > 0 {
		b := make([]byte, 0, int(n)*recordSize)
		for _, pw := range all[:n] {
			b = appendRecord(b, pw)
		}
		c.put("pawns", p, b)
	}
	s[0] = api.EncodeI32(clampI32(len(all)))
}

// pawn writes the colonist's record and returns 1, else returns 0.
func (c *callState) pawn(s []uint64) {
	id, p := int64(s[0]), api.DecodeU32(s[1])
	pw, ok := c.lookup(id)
	if !ok {
		c.charge(importCost)
		s[0] = 0
		return
	}
	c.within("pawn", p, recordSize)
	c.charge(importCost + 1)
	c.put("pawn", p, appendRecord(make([]byte, 0, recordSize), pw))
	s[0] = 1
}

// pawnName writes up to cap bytes of the colonist's name and returns its
// full length, or -1.
func (c *callState) pawnName(s []uint64) {
	id, p, capacity := int64(s[0]), api.DecodeU32(s[1]), api.DecodeU32(s[2])
	pw, ok := c.lookup(id)
	if !ok {
		c.charge(importCost)
		s[0] = api.EncodeI32(-1)
		return
	}
	n := uint32(len(pw.Name))
	if capacity < n {
		n = capacity
	}
	c.within("pawn_name", p, n)
	c.charge(importCost + words(n))
	c.put("pawn_name", p, []byte(pw.Name[:n]))
	s[0] = api.EncodeI32(clampI32(len(pw.Name)))
}

// lookup is ctx.pawn(id): ids below 1 and a missing reader find nobody.
func (c *callState) lookup(id int64) (scripting.Pawn, bool) {
	if id < 1 || c.pr == nil {
		return scripting.Pawn{}, false
	}
	return c.pr.Pawn(uint64(id))
}
