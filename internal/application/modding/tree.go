// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Trees are what encoding/json decodes into an `any`: map[string]any,
// []any, string, float64, bool and nil. The helpers here copy, compare,
// merge and walk them; none of them lets map iteration order reach a
// result.

// treeSegment is one segment of a dotted field path.
var treeSegment = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// treeCopy deep-copies a tree, so the copy shares no map or slice with v.
func treeCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return treeCopyMap(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = treeCopy(e)
		}
		return out
	default:
		return v
	}
}

// treeCopyMap deep-copies an object; nil stays nil.
func treeCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, e := range m {
		out[k] = treeCopy(e)
	}
	return out
}

// treeEqual reports whether two trees hold the same values.
func treeEqual(a, b any) bool {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, xv := range x {
			yv, ok := y[k]
			if !ok || !treeEqual(xv, yv) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !treeEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	case nil:
		return b == nil
	default:
		switch b.(type) {
		case map[string]any, []any, nil:
			return false
		}
		return a == b
	}
}

// treeMerge deep-merges src into dst, which the caller owns: objects
// merge recursively, lists and scalars replace, and a null in src deletes
// the field (at any depth) and is not kept. src is never aliased.
func treeMerge(dst, src map[string]any) {
	for k, sv := range src {
		switch s := sv.(type) {
		case nil:
			delete(dst, k)
		case map[string]any:
			d, ok := dst[k].(map[string]any)
			if !ok {
				d = map[string]any{}
			}
			treeMerge(d, s)
			dst[k] = d
		default:
			dst[k] = treeCopy(sv)
		}
	}
}

// treeKeys is m's keys in sorted order.
func treeKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// treePath splits a dotted path into segments; "" is the empty path. It
// reports an error for an empty or malformed segment.
func treePath(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	segs := strings.Split(path, ".")
	for _, s := range segs {
		if !treeSegment.MatchString(s) {
			return nil, fmt.Errorf("path %q: segment %q must be letters, digits or _", path, s)
		}
	}
	return segs, nil
}

// treeOverlap reports whether two dotted paths name the same field or one
// holds the other ("" holds every field). Segments compare whole, so
// "stats.hp" does not overlap "stats.hpmax".
func treeOverlap(a, b string) bool {
	return a == "" || b == "" || a == b ||
		strings.HasPrefix(a, b+".") || strings.HasPrefix(b, a+".")
}

// treeError is a failed path operation; missing marks a field or item
// that is not there, which an optional op may ignore.
type treeError struct {
	msg     string
	missing bool
}

func (e *treeError) Error() string { return e.msg }

func treeFail(missing bool, format string, args ...any) *treeError {
	return &treeError{msg: fmt.Sprintf(format, args...), missing: missing}
}

// treeEdit decides the new value of the field a path ends at. cur is the
// current value and exists whether it is there. It returns the new value,
// or del to delete the field (or list item).
type treeEdit func(cur any, exists bool) (val any, del bool, err *treeError)

// treeUpdate walks node along segs and applies edit to the last segment,
// returning the new node (a list that grew or shrank is a new slice, so
// every caller stores the result back). Intermediate segments must exist
// and be objects, or lists with an in-range index. node is changed in
// place, so the caller must own it.
func treeUpdate(node any, segs []string, done []string, edit treeEdit) (any, *treeError) {
	seg := segs[0]
	here := strings.Join(append(done, seg), ".")
	last := len(segs) == 1
	switch n := node.(type) {
	case map[string]any:
		cur, ok := n[seg]
		if last {
			val, del, err := edit(cur, ok)
			if err != nil {
				return nil, err
			}
			if del {
				delete(n, seg)
			} else {
				n[seg] = val
			}
			return n, nil
		}
		if !ok {
			return nil, treeFail(true, "%s is missing", here)
		}
		child, err := treeUpdate(cur, segs[1:], append(done, seg), edit)
		if err != nil {
			return nil, err
		}
		n[seg] = child
		return n, nil
	case []any:
		i, convErr := strconv.Atoi(seg)
		if convErr != nil || !treeDigits(seg) {
			return nil, treeFail(false, "%s: a list needs a numeric index", here)
		}
		if i >= len(n) {
			return nil, treeFail(true, "%s: index %d is out of range (the list has %d items)", here, i, len(n))
		}
		if last {
			val, del, err := edit(n[i], true)
			if err != nil {
				return nil, err
			}
			if del {
				return slices.Delete(n, i, i+1), nil
			}
			n[i] = val
			return n, nil
		}
		child, err := treeUpdate(n[i], segs[1:], append(done, seg), edit)
		if err != nil {
			return nil, err
		}
		n[i] = child
		return n, nil
	default:
		return nil, treeFail(false, "%s: %s is not an object or a list", here, treeWhere(done))
	}
}

// treeWhere names the field a path prefix reaches, for messages.
func treeWhere(done []string) string {
	if len(done) == 0 {
		return "the def"
	}
	return strings.Join(done, ".")
}

// treeDigits reports whether s is a non-empty run of ASCII digits.
func treeDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
