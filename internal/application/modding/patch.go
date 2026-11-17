package modding

import (
	"fmt"
	"strings"
)

// patchFields are the fields a patch operation may carry.
var patchFields = map[string]bool{"op": true, "target": true, "path": true, "value": true,
	"if": true, "missing": true, "optional": true}

// patchKinds are the operations, and whether each needs a value.
var patchKinds = map[string]bool{"set": true, "add": true, "remove": false, "merge": true, "replace": true}

// ParseOps reads p's patches files into operations (PLG-030, PLG-031).
// A file is a list of {"op", "target", "path", "value", "if", "missing",
// "optional"}; a target key without a plugin ("backstory/x") is p's own.
// A broken op drops only itself. A remove whose value is null removes
// the field (a tree cannot tell null from no value).
func ParseOps(p Plugin) ([]Op, []Problem) {
	var ops []Op
	var probs []Problem
	for _, f := range p.Patches {
		list, ok := f.Data.([]any)
		if !ok {
			probs = append(probs, Problem{Plugin: p.Manifest.ID, File: f.Path,
				Msg: "a patches file must be a list of operations"})
			continue
		}
		for i, item := range list {
			op, prob := parseOp(p.Manifest.ID, f.Path, i, item)
			if prob != nil {
				probs = append(probs, *prob)
				continue
			}
			ops = append(ops, op)
		}
	}
	return ops, probs
}

// parseOp reads operation number i of a patches file.
func parseOp(id, file string, i int, item any) (Op, *Problem) {
	op := Op{Plugin: id, File: file, Index: i}
	fail := func(field, msg string) (Op, *Problem) {
		p := op.problem(op.Target, field, msg, false)
		return Op{}, &p
	}
	m, ok := item.(map[string]any)
	if !ok {
		return fail("", "an operation must be an object")
	}
	for _, k := range treeKeys(m) {
		if !patchFields[k] {
			return fail(k, fmt.Sprintf("unknown field %q (allowed: op, target, path, value, if, missing, optional)", k))
		}
	}
	op.Kind, _ = m["op"].(string)
	needsValue, known := patchKinds[op.Kind]
	if !known {
		op.Kind = ""
		return fail("op", fmt.Sprintf("op %#v must be set, add, remove, merge or replace", m["op"]))
	}
	target, ok := patchTarget(id, m["target"])
	if !ok {
		return fail("target", fmt.Sprintf(`target %#v must be "<class>/<name>" or "<class>/<plugin>.<name>"`, m["target"]))
	}
	op.Target = target
	if v, ok := m["path"]; ok {
		if op.Path, ok = v.(string); !ok {
			return fail("path", "path must be a string")
		}
		if _, err := treePath(op.Path); err != nil {
			return fail("path", err.Error())
		}
	}
	for _, c := range []struct {
		field string
		dst   *string
	}{{"if", &op.If}, {"missing", &op.Missing}} {
		if v, ok := m[c.field]; ok {
			if *c.dst, ok = v.(string); !ok || *c.dst == "" {
				return fail(c.field, c.field+" must be a plugin id")
			}
		}
	}
	if v, ok := m["optional"]; ok {
		if op.Optional, ok = v.(bool); !ok {
			return fail("optional", "optional must be true or false")
		}
	}
	value, hasValue := m["value"]
	op.Value = treeCopy(value)
	if field, msg := patchShape(op, needsValue, hasValue); msg != "" {
		return fail(field, msg)
	}
	return op, nil
}

// patchShape checks the path and value an op's kind needs: the field at
// fault and a message, or "" when fine.
func patchShape(op Op, needsValue, hasValue bool) (string, string) {
	switch {
	case needsValue && !hasValue:
		return "value", "value is required"
	case (op.Kind == "set" || op.Kind == "add") && op.Path == "":
		return "path", "path is required"
	case op.Kind == "remove" && op.Value != nil && op.Path == "":
		return "path", "path is required to remove a value from a list"
	case op.Kind == "replace" && op.Path != "":
		return "path", "path must be empty: replace changes the whole def"
	}
	obj, isObj := op.Value.(map[string]any)
	if (op.Kind == "merge" || op.Kind == "replace") && !isObj {
		return "value", "value must be an object"
	}
	if op.Kind == "replace" {
		for _, k := range []string{"name", "abstract", "parent"} {
			if _, ok := obj[k]; ok {
				return "value", "value must not set " + k + ": replace keeps the def's identity"
			}
		}
	}
	return "", ""
}

// patchTarget resolves a target: "backstory/x" is p's "backstory/<id>.x".
func patchTarget(id string, v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	class, key, ok := strings.Cut(s, "/")
	if !ok || !defClassPattern.MatchString(class) {
		return "", false
	}
	key, ok = defParentKey(id, key)
	return class + "/" + key, ok
}

// Apply runs ops in order over defs (PLG-030…033). loads reports whether
// a plugin id is in the load order (for If and Missing). Bodies are
// copied first, so neither defs nor ops change. Every applied op appends
// a Touch to its def; a different plugin's set, merge or replace over a
// field an earlier plugin set, merged or replaced is a warning naming
// both. The output is defs in input order minus removed ones.
func Apply(defs []Def, ops []Op, loads func(id string) bool) ([]Def, []Problem) {
	out := make([]Def, len(defs))
	byRef := make(map[string]int, len(defs))
	for i, d := range defs {
		d.Body = treeCopyMap(d.Body)
		if d.Body == nil {
			d.Body = map[string]any{}
		}
		d.Touched = append([]Touch(nil), d.Touched...)
		out[i] = d
		if _, dup := byRef[d.Ref()]; !dup {
			byRef[d.Ref()] = i
		}
	}
	a := applier{defs: out, byRef: byRef, removed: make([]bool, len(out)),
		changes: make([][]patchChange, len(out))}
	for _, op := range ops {
		if (op.If != "" && !loads(op.If)) || (op.Missing != "" && loads(op.Missing)) {
			continue
		}
		a.apply(op)
	}
	kept := out[:0]
	for i, d := range out {
		if !a.removed[i] {
			kept = append(kept, d)
		}
	}
	return kept, a.probs
}

// patchChange is one plugin's set, merge or replace of a field of a def.
type patchChange struct {
	path, plugin string
}

// applier is one Apply run over its own copies of the defs.
type applier struct {
	defs    []Def
	byRef   map[string]int
	removed []bool
	changes [][]patchChange // per def, in order
	probs   []Problem
}

func (a *applier) apply(op Op) {
	i, ok := a.byRef[op.Target]
	if !ok || a.removed[i] {
		if !op.Optional {
			a.probs = append(a.probs, op.problem(op.Target, "", "target "+op.Target+" does not exist", false))
		}
		return
	}
	d := &a.defs[i]
	err := a.run(i, op)
	if err != nil {
		if !(err.missing && op.Optional) {
			a.probs = append(a.probs, op.problem(op.Target, op.Path, err.msg, false))
		}
		return
	}
	a.conflict(i, op)
	d.Touched = append(d.Touched, Touch{Plugin: op.Plugin, File: op.File, Op: op.Kind, Field: op.Path})
}

// run applies op to def i.
func (a *applier) run(i int, op Op) *treeError {
	d := &a.defs[i]
	segs, perr := treePath(op.Path)
	if perr != nil {
		return treeFail(false, "%s", perr.Error())
	}
	switch op.Kind {
	case "set":
		if len(segs) == 0 {
			return treeFail(false, "path is required")
		}
		return a.update(d, segs, func(any, bool) (any, bool, *treeError) {
			return treeCopy(op.Value), false, nil
		})
	case "add":
		if len(segs) == 0 {
			return treeFail(false, "path is required")
		}
		return a.update(d, segs, func(cur any, exists bool) (any, bool, *treeError) {
			return patchAdd(op.Path, cur, exists, op.Value)
		})
	case "remove":
		if len(segs) == 0 {
			a.removed[i] = true
			return nil
		}
		return a.update(d, segs, func(cur any, exists bool) (any, bool, *treeError) {
			return patchRemove(op.Path, cur, exists, op.Value)
		})
	case "merge":
		obj, ok := op.Value.(map[string]any)
		if !ok {
			return treeFail(false, "value must be an object")
		}
		if len(segs) == 0 {
			treeMerge(d.Body, obj)
			return nil
		}
		return a.update(d, segs, func(cur any, exists bool) (any, bool, *treeError) {
			m, ok := cur.(map[string]any)
			if !exists {
				return nil, false, treeFail(true, "%s is missing; use set", op.Path)
			}
			if !ok {
				return nil, false, treeFail(false, "%s is not an object", op.Path)
			}
			treeMerge(m, obj)
			return m, false, nil
		})
	case "replace":
		obj, ok := op.Value.(map[string]any)
		if !ok || len(segs) != 0 {
			return treeFail(false, "replace needs an empty path and an object value")
		}
		d.Body = treeCopyMap(obj)
		return nil
	}
	return treeFail(false, "unknown op %q", op.Kind)
}

// update applies edit at segs inside d's body. treeUpdate and the edits
// change nothing before they know they succeed, so a failed op leaves
// the body as it was.
func (a *applier) update(d *Def, segs []string, edit treeEdit) *treeError {
	_, err := treeUpdate(d.Body, segs, nil, edit)
	return err
}

// patchAdd appends value to a list, or adds an object's new entries.
func patchAdd(path string, cur any, exists bool, value any) (any, bool, *treeError) {
	switch c := cur.(type) {
	case []any:
		return append(c, treeCopy(value)), false, nil
	case map[string]any:
		add, ok := value.(map[string]any)
		if !ok {
			return nil, false, treeFail(false, "%s is an object: add needs an object value", path)
		}
		keys := treeKeys(add)
		for _, k := range keys {
			if _, set := c[k]; set {
				return nil, false, treeFail(false, "%s.%s already set; use set", path, k)
			}
		}
		for _, k := range keys {
			c[k] = treeCopy(add[k])
		}
		return c, false, nil
	}
	if !exists {
		return nil, false, treeFail(false, "%s: field missing; use set", path)
	}
	return nil, false, treeFail(false, "%s is not a list or an object; use set", path)
}

// patchRemove deletes a field, or every list item equal to value.
func patchRemove(path string, cur any, exists bool, value any) (any, bool, *treeError) {
	if !exists {
		return nil, false, treeFail(true, "%s is missing", path)
	}
	if value == nil {
		return nil, true, nil
	}
	list, ok := cur.([]any)
	if !ok {
		return nil, false, treeFail(false, "%s is not a list: remove with a value removes list items", path)
	}
	kept := make([]any, 0, len(list))
	for _, e := range list {
		if !treeEqual(e, value) {
			kept = append(kept, e)
		}
	}
	if len(kept) == len(list) {
		return nil, false, treeFail(true, "%s holds no item equal to the value", path)
	}
	return kept, false, nil
}

// conflict warns when op overrides a field another plugin set, merged or
// replaced (PLG-033), naming the latest such plugin, and records op's own
// change. A plugin changing its own field again is not a conflict.
func (a *applier) conflict(i int, op Op) {
	if op.Kind != "set" && op.Kind != "merge" && op.Kind != "replace" {
		return
	}
	list := a.changes[i]
	for k := len(list) - 1; k >= 0; k-- {
		c := list[k]
		if c.plugin == op.Plugin || !treeOverlap(c.path, op.Path) {
			continue
		}
		field := c.path
		if len(op.Path) > len(field) {
			field = op.Path
		}
		if field == "" {
			field = "the whole def"
		}
		a.probs = append(a.probs, op.problem(op.Target, op.Path,
			field+" was changed by "+c.plugin+"; "+op.Plugin+" overrides it", true))
		break
	}
	kept := list[:0:0]
	for _, c := range list {
		if !(c.path == op.Path || strings.HasPrefix(c.path, op.Path+".") || op.Path == "") {
			kept = append(kept, c)
		}
	}
	a.changes[i] = append(kept, patchChange{path: op.Path, plugin: op.Plugin})
}
