// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.starlark.net/starlark"

	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/pkg/randx"
)

var _ starlark.HasAttrs = (*ctxValue)(nil)

// baseAttrs are the query and command attributes every ctx serves (the
// legacy set); queryAttr and orderAttr serve the extension groups.
var baseAttrs = map[string]func(c *call) starlark.Value{
	"tick":      func(c *call) starlark.Value { return starlark.MakeUint64(c.ev.Tick) },
	"day":       func(c *call) starlark.Value { return starlark.MakeInt(c.w.Day()) },
	"hour":      func(c *call) starlark.Value { return starlark.MakeInt(c.w.Hour()) },
	"season":    func(c *call) starlark.Value { return starlark.String(c.w.Season()) },
	"colonists": func(c *call) starlark.Value { return starlark.MakeInt(c.w.Colonists()) },
	"wealth":    func(c *call) starlark.Value { return starlark.MakeInt(c.w.Wealth()) },
	"count":     func(c *call) starlark.Value { return starlark.NewBuiltin("count", c.count) },
	"rand":      func(c *call) starlark.Value { return starlark.NewBuiltin("rand", c.rand) },
	"message":   func(c *call) starlark.Value { return starlark.NewBuiltin("message", c.message) },
	"letter":    func(c *call) starlark.Value { return starlark.NewBuiltin("letter", c.letter) },
	"spawn":     func(c *call) starlark.Value { return starlark.NewBuiltin("spawn", c.spawn) },
	"pawns":     func(c *call) starlark.Value { return starlark.NewBuiltin("pawns", c.pawns) },
	"pawn":      func(c *call) starlark.Value { return starlark.NewBuiltin("pawn", c.pawn) },
}

// call is one hook call's state: what it read, drew and queued. It lives
// for the call only.
type call struct {
	ev   scripting.Event
	w    scripting.World
	rng  randx.Keyed
	cmds []command.Command
	pr   scripting.PawnReader
	all  starlark.Tuple    // ctx.pawns(), built on first use
	what string            // names the command budget in an overrun
	hook string            // the hook's function name ("on_pawn", "panel", …)
	caps pluginapi.Set     // the module's plugin's granted set
	q    scripting.Queries // the world capability's readers
}

// newCall keys the call's rand stream on (seed, tick, plugin and file,
// hook, the event's own argument): the same call draws the same numbers,
// whatever ran before it.
func newCall(m *module, ev scripting.Event, w scripting.World, pr scripting.PawnReader) *call {
	c := &call{ev: ev, w: w, pr: pr, what: "one call"}
	c.rekey(m, key(ev.Incident.Key+"\x00"+ev.Name))
	return c
}

// rekey restarts the rand stream under sub. on_pawn keys it by the pawn's
// id, so each pawn draws its own numbers.
func (c *call) rekey(m *module, sub uint64) {
	c.rng = randx.NewKeyed(c.ev.Seed, c.ev.Tick, key(m.plugin+"\x00"+m.path), uint64(c.ev.Hook), sub)
}

// args are the hook's arguments: ctx, then the incident or the name.
func (c *call) args(ev scripting.Event) starlark.Tuple {
	ctx := &ctxValue{c: c}
	switch ev.Hook {
	case scripting.OnIncident:
		return starlark.Tuple{ctx, incidentValue(ev.Incident)}
	case scripting.OnColonistDied:
		return starlark.Tuple{ctx, starlark.String(ev.Name)}
	case scripting.OnPawns:
		return starlark.Tuple{ctx, pawnTuple(ev.Pawns)}
	}
	return starlark.Tuple{ctx}
}

// incidentValue is on_incident's and run_incident's incident argument, a
// frozen struct of key and category.
func incidentValue(inc scripting.Incident) starlark.Value {
	return frozenStruct("incident", starlark.StringDict{
		"key": starlark.String(inc.Key), "category": starlark.String(inc.Category)})
}

// ctxValue is the script's ctx: read-only queries and the command calls.
type ctxValue struct{ c *call }

func (v *ctxValue) String() string        { return "<ctx>" }
func (v *ctxValue) Type() string          { return "ctx" }
func (v *ctxValue) Freeze()               {}
func (v *ctxValue) Truth() starlark.Bool  { return starlark.True }
func (v *ctxValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: ctx") }

// AttrNames is every attribute ctx serves, sorted: the base table and
// the groups' lists.
func (v *ctxValue) AttrNames() []string {
	names := slices.Concat(slices.Collect(maps.Keys(baseAttrs)), queryAttrs, orderAttrs)
	slices.Sort(names)
	return names
}

// Attr checks name first (allowed), then serves it from the base table,
// then the world queries, then the orders.
func (v *ctxValue) Attr(name string) (starlark.Value, error) {
	c := v.c
	if err := c.allowed(name); err != nil {
		return nil, err
	}
	if f, ok := baseAttrs[name]; ok {
		return f(c), nil
	}
	if val, ok := c.queryAttr(name); ok {
		return val, nil
	}
	if val, ok := c.orderAttr(name); ok {
		return val, nil
	}
	return nil, nil // no such attribute
}

func (c *call) count(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var k string
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kw, 1, &k); err != nil {
		return nil, err
	}
	n, ok := c.w.ItemCount(k)
	if !ok {
		return nil, fmt.Errorf("count: unknown item %q", k)
	}
	return starlark.MakeInt(n), nil
}

func (c *call) rand(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var n int
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kw, 1, &n); err != nil {
		return nil, err
	}
	if n < 1 || n > 1<<30 {
		return nil, fmt.Errorf("rand: n must be 1 to %d, got %d", 1<<30, n)
	}
	return starlark.MakeInt(c.rng.IntN(n)), nil
}

func (c *call) message(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var text string
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kw, 1, &text); err != nil {
		return nil, err
	}
	if err := checkText("message", text); err != nil {
		return nil, err
	}
	return starlark.None, c.queue(command.Command{Kind: command.ScriptMessage, Mode: command.ScriptNote, Name: text})
}

func (c *call) letter(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var title, text string
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kw, 2, &title, &text); err != nil {
		return nil, err
	}
	if strings.Contains(title, "\n") {
		return nil, fmt.Errorf("letter: the title is one line")
	}
	if err := checkText("letter title", title); err != nil {
		return nil, err
	}
	if err := checkText("letter", text); err != nil {
		return nil, err
	}
	return starlark.None, c.queue(command.Command{Kind: command.ScriptMessage, Mode: command.ScriptLetter, Name: title + "\n" + text})
}

func (c *call) spawn(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var k string
	n := 1
	if err := starlark.UnpackArgs(b.Name(), args, kw, "key", &k, "count?", &n); err != nil {
		return nil, err
	}
	if n < 1 || n > scripting.MaxSpawn {
		return nil, fmt.Errorf("spawn: count must be 1 to %d, got %d", scripting.MaxSpawn, n)
	}
	if _, ok := c.w.ItemCount(k); !ok {
		return nil, fmt.Errorf("spawn: unknown item %q", k)
	}
	return starlark.None, c.queue(command.Command{Kind: command.ScriptSpawn, Name: k, Count: n})
}

// queue adds a command within the call's budget.
func (c *call) queue(cmd command.Command) error {
	if len(c.cmds) >= scripting.MaxCommands {
		return fmt.Errorf("more than %d commands in %s", scripting.MaxCommands, c.what)
	}
	c.cmds = append(c.cmds, cmd)
	return nil
}

func checkText(what, s string) error {
	switch {
	case strings.TrimSpace(s) == "":
		return fmt.Errorf("%s: empty text", what)
	case len(s) > scripting.MaxText:
		return fmt.Errorf("%s: longer than %d bytes", what, scripting.MaxText)
	}
	return nil
}
