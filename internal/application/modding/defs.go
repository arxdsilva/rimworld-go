package modding

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	defClassPattern = regexp.MustCompile(`^[a-z][a-z_]*$`)
	defNamePattern  = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
)

// CollectDefs reads p's defs files into trees (PLG-020): each file is
// {"<class>": [ {def}, ... ]}; a def has "name" (key "<plugin>.<name>"),
// optional "abstract" and "parent", and its fields. Classes are read in
// sorted order, so a file's key order never matters. Whether a class is
// known is Build's concern. A broken def drops only itself.
func CollectDefs(p Plugin) ([]Def, []Problem) {
	id := p.Manifest.ID
	var defs []Def
	var probs []Problem
	firstFile := map[string]string{} // ref -> file that defined it first
	for _, f := range p.Defs {
		classes, ok := f.Data.(map[string]any)
		if !ok {
			probs = append(probs, Problem{Plugin: id, File: f.Path,
				Msg: `a defs file must be an object of classes: {"<class>": [ {def}, ... ]}`})
			continue
		}
		for _, class := range treeKeys(classes) {
			if !defClassPattern.MatchString(class) {
				probs = append(probs, Problem{Plugin: id, File: f.Path, Field: class,
					Msg: fmt.Sprintf("class %q must be lowercase letters or _", class)})
				continue
			}
			list, ok := classes[class].([]any)
			if !ok {
				probs = append(probs, Problem{Plugin: id, File: f.Path, Def: class,
					Msg: "a class must hold a list of definitions"})
				continue
			}
			for i, item := range list {
				d, prob := collectDef(id, f.Path, class, i, item)
				if prob != nil {
					probs = append(probs, *prob)
					continue
				}
				if first, dup := firstFile[d.Ref()]; dup {
					probs = append(probs, Problem{Plugin: id, File: f.Path, Def: d.Ref(), Field: "name",
						Msg: "defined twice in this plugin (first in " + first + ")"})
					continue
				}
				firstFile[d.Ref()] = f.Path
				defs = append(defs, d)
			}
		}
	}
	return defs, probs
}

// collectDef reads one def object, item number i of class's list.
func collectDef(id, file, class string, i int, item any) (Def, *Problem) {
	fail := func(def, field, msg string) (Def, *Problem) {
		return Def{}, &Problem{Plugin: id, File: file, Def: def, Field: field, Msg: msg}
	}
	at := fmt.Sprintf("[%d]", i)
	m, ok := item.(map[string]any)
	if !ok {
		return fail(class, at, "a definition must be an object")
	}
	rawName, ok := m["name"]
	if !ok {
		return fail(class, at, "name is required")
	}
	name, ok := rawName.(string)
	if !ok || !defNamePattern.MatchString(name) {
		return fail(class, at, fmt.Sprintf("name %#v must be 1–64 lowercase letters, digits or _", rawName))
	}
	d := Def{Class: class, Key: id + "." + name, Plugin: id, File: file, Body: map[string]any{}}
	ref := d.Ref()
	if v, ok := m["abstract"]; ok {
		if d.Abstract, ok = v.(bool); !ok {
			return fail(ref, "abstract", "abstract must be true or false")
		}
	}
	if v, ok := m["parent"]; ok {
		if d.Parent, ok = defParentKey(id, v); !ok {
			return fail(ref, "parent", fmt.Sprintf(`parent %#v must be "<name>" or "<plugin>.<name>"`, v))
		}
	}
	for k, v := range m {
		if k != "name" && k != "abstract" && k != "parent" {
			d.Body[k] = treeCopy(v)
		}
	}
	return d, nil
}

// defParentKey resolves a "parent" value: "base" is this plugin's
// "<id>.base", "other.base" is taken as written.
func defParentKey(id string, v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	plugin, name := id, s
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		plugin, name = s[:i], s[i+1:]
	}
	if plugin == "" || strings.Contains(plugin, ".") || !defNamePattern.MatchString(name) {
		return "", false
	}
	return plugin + "." + name, true
}
