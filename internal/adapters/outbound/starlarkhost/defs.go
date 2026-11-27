// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/application/pluginapi"
	"rimworld-go/internal/application/scripting"
)

// Defs runs plugins' load-time hooks, register_defs() and
// validate_defs(defs), for modding.BuildWith. The calls have no ctx, so
// they have no clock, no RNG and no commands; each runs on a fresh thread
// with the step limit, over modules in path order.
//
// Register loads (and remembers) every plugin it is given, so Validate
// can call a validating plugin's modules: the build asks Register for
// every plugin that may use either hook; each hook runs only when the
// plugin's manifest grants its capability.
type Defs struct{ s *defsState }

type defsState struct {
	steps uint64
	mu    sync.Mutex
	byID  map[string]*loadedDefs
}

// loadedDefs is one plugin's modules after their first run.
type loadedDefs struct {
	caps pluginapi.Set
	mods []*module         // in path order, those that ran
	errs []modding.Problem // first-run failures, in path order
}

var _ modding.ScriptDefs = Defs{}

// maxDefDepth bounds a registered value's nesting (a self-containing
// list is an error, not a hang).
const maxDefDepth = 32

// NewDefs is the load-time script evaluator.
func NewDefs() Defs { return newDefs(scripting.MaxSteps) }

func newDefs(steps uint64) Defs {
	return Defs{s: &defsState{steps: steps, byID: map[string]*loadedDefs{}}}
}

// load runs p's script files once and remembers them under its id.
func (d Defs) load(p modding.Plugin) *loadedDefs {
	d.s.mu.Lock()
	defer d.s.mu.Unlock()
	id := p.Manifest.ID
	if ld, ok := d.s.byID[id]; ok {
		return ld
	}
	caps, _ := modding.CapsOf(p.Manifest)
	ld := &loadedDefs{caps: caps}
	d.s.byID[id] = ld
	var files []File
	for _, b := range p.Scripts {
		if strings.HasSuffix(b.Path, ".star") {
			files = append(files, File{Path: b.Path, Src: b.Data})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	l := &loader{plugin: Plugin{ID: id, Files: files}, steps: d.s.steps,
		done: map[string]*module{}, errs: map[string]error{}, busy: map[string]bool{}}
	for _, f := range files {
		m, err := l.load(f.Path)
		if err != nil {
			ld.errs = append(ld.errs, problemFor(problemOf(m, err)))
			continue
		}
		ld.mods = append(ld.mods, m)
	}
	return ld
}

// problemFor is a scripting problem as a modding one (the line goes into
// the message).
func problemFor(p scripting.Problem) modding.Problem {
	msg := p.Msg
	if p.Line > 0 {
		msg = fmt.Sprintf("line %d: %s", p.Line, msg)
	}
	return modding.Problem{Plugin: p.Plugin, File: p.File, Msg: msg}
}

// call runs fn(args...) on a fresh thread with the step limit.
func (d Defs) call(m *module, fn starlark.Callable, args starlark.Tuple) (starlark.Value, error) {
	th := &starlark.Thread{Name: m.plugin + "/" + m.path, Print: func(*starlark.Thread, string) {}}
	th.SetMaxExecutionSteps(d.s.steps)
	return starlark.Call(th, fn, args, nil)
}

// Register is p's register_defs() results as defs files of p: one per
// module that defines it, "<script path>#register_defs". A module that
// fails to load, errors or returns a bad shape is a problem and has no file.
func (d Defs) Register(p modding.Plugin) ([]modding.File, []modding.Problem) {
	ld := d.load(p)
	if !ld.caps.Has(pluginapi.Defs) {
		return nil, nil
	}
	probs := slices.Clone(ld.errs)
	var files []modding.File
	for _, m := range ld.mods {
		fn, ok := m.globals["register_defs"].(starlark.Callable)
		if !ok {
			continue
		}
		v, err := d.call(m, fn, nil)
		if err == nil {
			var tree map[string]any
			if tree, err = defsTree(v); err == nil {
				files = append(files, modding.File{Path: m.path + "#register_defs", Data: tree})
				continue
			}
		}
		pr := problemFor(problemOf(m, err))
		pr.File = m.path + "#register_defs"
		probs = append(probs, pr)
	}
	return files, probs
}

// defsTree checks v is {class: [ {def}, ... ]} and converts it to the
// shape json.Unmarshal gives.
func defsTree(v starlark.Value) (map[string]any, error) {
	top, ok := v.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("register_defs must return a dict of class to list of dicts, not %s", v.Type())
	}
	out := make(map[string]any, top.Len())
	for _, kv := range top.Items() {
		class, ok := starlark.AsString(kv[0])
		if !ok {
			return nil, fmt.Errorf("register_defs: class name must be a string, not %s", kv[0].Type())
		}
		list, ok := kv[1].(*starlark.List)
		if !ok {
			return nil, fmt.Errorf("register_defs: %q must be a list of dicts, not %s", class, kv[1].Type())
		}
		defs := make([]any, 0, list.Len())
		for i := 0; i < list.Len(); i++ {
			d, ok := list.Index(i).(*starlark.Dict)
			if !ok {
				return nil, fmt.Errorf("register_defs: %q[%d] must be a dict, not %s", class, i, list.Index(i).Type())
			}
			conv, err := toTree(d, 1)
			if err != nil {
				return nil, fmt.Errorf("register_defs: %q[%d]: %w", class, i, err)
			}
			defs = append(defs, conv)
		}
		out[class] = defs
	}
	return out, nil
}

// toTree converts a string, int, bool, list or dict (string keys) to the
// json shape: ints are float64, lists []any, dicts map[string]any.
func toTree(v starlark.Value, depth int) (any, error) {
	if depth > maxDefDepth {
		return nil, fmt.Errorf("nested deeper than %d", maxDefDepth)
	}
	switch x := v.(type) {
	case starlark.String:
		return string(x), nil
	case starlark.Bool:
		return bool(x), nil
	case starlark.Int:
		n, ok := x.Int64()
		if !ok {
			return nil, fmt.Errorf("int %s is out of range", x.String())
		}
		return float64(n), nil
	case *starlark.List:
		out := make([]any, 0, x.Len())
		for i := 0; i < x.Len(); i++ {
			e, err := toTree(x.Index(i), depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		}
		return out, nil
	case *starlark.Dict:
		out := make(map[string]any, x.Len())
		for _, kv := range x.Items() {
			k, ok := starlark.AsString(kv[0])
			if !ok {
				return nil, fmt.Errorf("dict key must be a string, not %s", kv[0].Type())
			}
			e, err := toTree(kv[1], depth+1)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			out[k] = e
		}
		return out, nil
	}
	return nil, fmt.Errorf("a %s is not a string, int, bool, list or dict", v.Type())
}

// Validate is plugin validator's validate_defs(defs) problems, module by
// module in path order. A module that errors or answers a bad shape is a
// warning (its checks are skipped; the defs are kept).
func (d Defs) Validate(validator string, defs []modding.Def) []modding.Problem {
	d.s.mu.Lock()
	ld := d.s.byID[validator]
	d.s.mu.Unlock()
	if ld == nil {
		return []modding.Problem{{Plugin: validator, Warn: true, Msg: "validate_defs skipped: its scripts were not loaded"}}
	}
	if !ld.caps.Has(pluginapi.Validate) {
		return nil
	}
	var args starlark.Value
	var probs []modding.Problem
	for _, m := range ld.mods {
		fn, ok := m.globals["validate_defs"].(starlark.Callable)
		if !ok {
			continue
		}
		if args == nil {
			args = defsList(defs)
		}
		v, err := d.call(m, fn, starlark.Tuple{args})
		if err == nil {
			var ps []modding.Problem
			if ps, err = problemsOf(validator, v); err == nil {
				probs = append(probs, ps...)
				continue
			}
		}
		pr := problemFor(problemOf(m, err))
		pr.Warn, pr.Msg = true, "validate_defs skipped: "+pr.Msg
		probs = append(probs, pr)
	}
	return probs
}

// problemsOf maps validate_defs's answer, a list of {def, msg, warn?}.
func problemsOf(validator string, v starlark.Value) ([]modding.Problem, error) {
	list, ok := v.(*starlark.List)
	if !ok {
		return nil, fmt.Errorf("validate_defs must return a list of dicts, not %s", v.Type())
	}
	var out []modding.Problem
	for i := 0; i < list.Len(); i++ {
		d, ok := list.Index(i).(*starlark.Dict)
		if !ok {
			return nil, fmt.Errorf("validate_defs[%d] must be a dict, not %s", i, list.Index(i).Type())
		}
		p := modding.Problem{Plugin: validator}
		for _, kv := range d.Items() {
			k, _ := starlark.AsString(kv[0])
			switch k {
			case "def", "msg":
				s, ok := starlark.AsString(kv[1])
				if !ok {
					return nil, fmt.Errorf("validate_defs[%d].%s must be a string", i, k)
				}
				if k == "def" {
					p.Def = s
				} else {
					p.Msg = s
				}
			case "warn":
				b, ok := kv[1].(starlark.Bool)
				if !ok {
					return nil, fmt.Errorf("validate_defs[%d].warn must be a bool", i)
				}
				p.Warn = bool(b)
			default:
				return nil, fmt.Errorf("validate_defs[%d] has no field %q (def, msg, warn)", i, kv[0].String())
			}
		}
		if p.Msg == "" {
			return nil, fmt.Errorf("validate_defs[%d] needs a msg", i)
		}
		out = append(out, p)
	}
	return out, nil
}

// defsList is defs as a frozen list of structs.
func defsList(defs []modding.Def) starlark.Value {
	items := make([]starlark.Value, len(defs))
	for i, d := range defs {
		items[i] = starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"class":    starlark.String(d.Class),
			"key":      starlark.String(d.Key),
			"plugin":   starlark.String(d.Plugin),
			"parent":   starlark.String(d.Parent),
			"abstract": starlark.Bool(d.Abstract),
			"fields":   fromGo(d.Body),
		})
	}
	l := starlark.NewList(items)
	l.Freeze()
	return l
}

// fromGo is a decoded-tree value as a Starlark value (maps by sorted key).
func fromGo(v any) starlark.Value {
	switch x := v.(type) {
	case nil:
		return starlark.None
	case string:
		return starlark.String(x)
	case bool:
		return starlark.Bool(x)
	case int:
		return starlark.MakeInt(x)
	case int64:
		return starlark.MakeInt64(x)
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1<<53 {
			return starlark.MakeInt64(int64(x))
		}
		return starlark.Float(x)
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		items := make([]starlark.Value, rv.Len())
		for i := range items {
			items[i] = fromGo(rv.Index(i).Interface())
		}
		return starlark.NewList(items)
	case reflect.Map:
		keys := rv.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
		dict := starlark.NewDict(len(keys))
		for _, k := range keys {
			_ = dict.SetKey(starlark.String(fmt.Sprint(k)), fromGo(rv.MapIndex(k).Interface()))
		}
		return dict
	}
	return starlark.String(fmt.Sprint(v))
}
