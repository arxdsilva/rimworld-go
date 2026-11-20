// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"

	"go.starlark.net/starlark"

	"rimworld-go/internal/application/scripting"
)

// pawnValue is a colonist as a script sees it: an immutable snapshot with
// integer and bool fields. It has no SetField, so assigning to a field is
// an error (which turns the script off).
type pawnValue struct{ p scripting.Pawn }

var _ starlark.HasAttrs = pawnValue{}

func (v pawnValue) String() string        { return fmt.Sprintf("<pawn %d %s>", v.p.ID, v.p.Name) }
func (v pawnValue) Type() string          { return "pawn" }
func (v pawnValue) Freeze()               {}
func (v pawnValue) Truth() starlark.Bool  { return starlark.True }
func (v pawnValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: pawn") }

var pawnAttrs = []string{"downed", "drafted", "food", "id", "mood", "name", "rest", "x", "y"}

func (v pawnValue) AttrNames() []string { return append([]string(nil), pawnAttrs...) }

func (v pawnValue) Attr(name string) (starlark.Value, error) {
	p := v.p
	switch name {
	case "id":
		return starlark.MakeUint64(p.ID), nil
	case "name":
		return starlark.String(p.Name), nil
	case "x":
		return starlark.MakeInt(p.X), nil
	case "y":
		return starlark.MakeInt(p.Y), nil
	case "food":
		return starlark.MakeInt(p.Food), nil
	case "rest":
		return starlark.MakeInt(p.Rest), nil
	case "mood":
		return starlark.MakeInt(p.Mood), nil
	case "downed":
		return starlark.Bool(p.Downed), nil
	case "drafted":
		return starlark.Bool(p.Drafted), nil
	}
	return nil, nil // no such attribute
}

// pawnTuple is ps as an immutable tuple of pawn values.
func pawnTuple(ps []scripting.Pawn) starlark.Tuple {
	t := make(starlark.Tuple, len(ps))
	for i, p := range ps {
		t[i] = pawnValue{p}
	}
	return t
}

// pawns is ctx.pawns(): every living home colonist, id ascending. The
// tuple is built once per call; its values are immutable.
func (c *call) pawns(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kw, 0); err != nil {
		return nil, err
	}
	if c.all == nil {
		if c.pr == nil {
			return starlark.Tuple{}, nil
		}
		c.all = pawnTuple(c.pr.Pawns())
	}
	return c.all, nil
}

// pawn is ctx.pawn(id): the colonist, or None for any other id.
func (c *call) pawn(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var id int
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kw, 1, &id); err != nil {
		return nil, err
	}
	if id < 1 || c.pr == nil {
		return starlark.None, nil
	}
	p, ok := c.pr.Pawn(uint64(id))
	if !ok {
		return starlark.None, nil
	}
	return pawnValue{p}, nil
}
