// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/pkg/randx"
)

// call is one hook call's state: what it read, drew and queued. It lives
// for the call only.
type call struct {
	ev   scripting.Event
	w    scripting.World
	rng  randx.Keyed
	cmds []command.Command
}

// newCall keys the call's rand stream on (seed, tick, plugin and file,
// hook, the event's own argument): the same call draws the same numbers,
// whatever ran before it.
func newCall(m *module, ev scripting.Event, w scripting.World) *call {
	sub := key(ev.Incident.Key + "\x00" + ev.Name)
	return &call{ev: ev, w: w, rng: randx.NewKeyed(ev.Seed, ev.Tick, key(m.plugin+"\x00"+m.path), uint64(ev.Hook), sub)}
}

// args are the hook's arguments: ctx, then the incident or the name.
func (c *call) args(ev scripting.Event) starlark.Tuple {
	ctx := &ctxValue{c: c}
	switch ev.Hook {
	case scripting.OnIncident:
		inc := starlarkstruct.FromStringDict(starlark.String("incident"), starlark.StringDict{
			"key": starlark.String(ev.Incident.Key), "category": starlark.String(ev.Incident.Category)})
		inc.Freeze()
		return starlark.Tuple{ctx, inc}
	case scripting.OnColonistDied:
		return starlark.Tuple{ctx, starlark.String(ev.Name)}
	}
	return starlark.Tuple{ctx}
}

// ctxValue is the script's ctx: read-only queries and the command calls.
type ctxValue struct{ c *call }

var _ starlark.HasAttrs = (*ctxValue)(nil)

func (v *ctxValue) String() string        { return "<ctx>" }
func (v *ctxValue) Type() string          { return "ctx" }
func (v *ctxValue) Freeze()               {}
func (v *ctxValue) Truth() starlark.Bool  { return starlark.True }
func (v *ctxValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: ctx") }

var ctxAttrs = []string{"colonists", "count", "day", "hour", "letter", "message", "rand", "season", "spawn", "tick", "wealth"}

func (v *ctxValue) AttrNames() []string { return append([]string(nil), ctxAttrs...) }

func (v *ctxValue) Attr(name string) (starlark.Value, error) {
	c := v.c
	switch name {
	case "tick":
		return starlark.MakeUint64(c.ev.Tick), nil
	case "day":
		return starlark.MakeInt(c.w.Day()), nil
	case "hour":
		return starlark.MakeInt(c.w.Hour()), nil
	case "season":
		return starlark.String(c.w.Season()), nil
	case "colonists":
		return starlark.MakeInt(c.w.Colonists()), nil
	case "wealth":
		return starlark.MakeInt(c.w.Wealth()), nil
	case "count":
		return starlark.NewBuiltin("count", c.count), nil
	case "rand":
		return starlark.NewBuiltin("rand", c.rand), nil
	case "message":
		return starlark.NewBuiltin("message", c.message), nil
	case "letter":
		return starlark.NewBuiltin("letter", c.letter), nil
	case "spawn":
		return starlark.NewBuiltin("spawn", c.spawn), nil
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
		return fmt.Errorf("more than %d commands in one call", scripting.MaxCommands)
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
