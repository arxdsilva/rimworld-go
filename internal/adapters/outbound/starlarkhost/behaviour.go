// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"
	"strings"

	"go.starlark.net/starlark"

	"rimworld-go/internal/application/scripting"
)

// RunsIncidents reports that a live module defines run_incident and its
// plugin has incidents: a precomputed flag.
func (h *Host) RunsIncidents() bool { return h.incidents }

// RunIncident runs run_incident(ctx, incident) of the live modules of the
// plugin whose id is ev.Incident.Key's prefix ("<plugin>.<name>"), in path
// order, until one returns True: replaced is then true. Anything else,
// or an error, does not replace the incident; the commands of the calls
// that did not fail are kept.
func (h *Host) RunIncident(ev scripting.Event, w scripting.World, p scripting.PawnReader) (res scripting.Result, replaced bool) {
	res = h.firstResult()
	id, _, ok := strings.Cut(ev.Incident.Key, ".")
	if !h.incidents || !ok || id == "" {
		return res, false
	}
	arg := incidentValue(ev.Incident)
	for _, m := range h.mods {
		fn, ok := m.globals["run_incident"].(starlark.Callable)
		if m.off || !ok || m.plugin != id {
			continue
		}
		c := h.callFor(m, "run_incident", ev, w, p)
		v, err := h.run(m, fn, starlark.Tuple{&ctxValue{c: c}, arg})
		h.collect(&res, m, c, err)
		if err == nil && v == starlark.True {
			return res, true
		}
	}
	return res, false
}

// Validates reports that a live module defines validate_command and its
// plugin has validate: a precomputed flag.
func (h *Host) Validates() bool { return h.validates }

// ValidateCommand asks every validating live module, in order, about cv
// with validate_command(ctx, cmd); cmd is a frozen struct of kind, pawn,
// x, y and name. The first non-blank reason string is returned with its
// module's plugin, and no later module is asked. None or "" passes; any
// other value, or a reason over MaxText bytes, turns the file off.
func (h *Host) ValidateCommand(cv scripting.CommandView, ev scripting.Event, w scripting.World, p scripting.PawnReader) (plugin, reason string, res scripting.Result) {
	res = h.firstResult()
	if !h.validates {
		return "", "", res
	}
	arg := frozenStruct("command", starlark.StringDict{
		"kind": starlark.String(cv.Kind), "pawn": starlark.MakeUint64(cv.Pawn),
		"x": starlark.MakeInt(cv.X), "y": starlark.MakeInt(cv.Y), "name": starlark.String(cv.Name),
	})
	sub := key(fmt.Sprintf("validate_command\x00%s\x00%d\x00%d\x00%d\x00%s", cv.Kind, cv.Pawn, cv.X, cv.Y, cv.Name))
	for _, m := range h.mods {
		fn, ok := m.globals["validate_command"].(starlark.Callable)
		if m.off || !ok {
			continue
		}
		c := h.callFor(m, "validate_command", ev, w, p)
		c.rekey(m, sub) // each command draws its own numbers
		v, err := h.run(m, fn, starlark.Tuple{&ctxValue{c: c}, arg})
		if err == nil {
			reason, err = reasonOf(v)
		}
		h.collect(&res, m, c, err)
		if err == nil && reason != "" {
			return m.plugin, reason, res
		}
	}
	return "", "", res
}

// reasonOf is validate_command's verdict: "" passes.
func reasonOf(v starlark.Value) (string, error) {
	if v == starlark.None {
		return "", nil
	}
	s, ok := v.(starlark.String)
	switch {
	case !ok:
		return "", fmt.Errorf("validate_command returned %s, want a reason string or None", v.Type())
	case len(s) > scripting.MaxText:
		return "", fmt.Errorf("validate_command: the reason is longer than %d bytes", scripting.MaxText)
	case strings.TrimSpace(string(s)) == "":
		return "", nil
	}
	return string(s), nil
}
