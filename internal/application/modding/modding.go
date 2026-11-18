// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package modding turns plugins into a frozen content catalog (PLG-*,
// .agents/gaps/MODDING_GAP_ANALYSIS.md): manifest checks, load order,
// definition inheritance, patches, typed definitions and provenance.
//
// It never touches a file and imports no third-party code (HEX §20): an
// adapter (adapters/outbound/modloader) reads the folders and hands over
// plain decoded JSON trees through the Source port.
//
// Trees are what encoding/json decodes into an `any`: map[string]any,
// []any, string, float64, bool and nil.
package modding

import (
	"fmt"
	"strings"
)

// Core is the game's own plugin id. It always loads first and no plugin
// may take its id.
const Core = "core"

// Manifest is a plugin's plugin.json (PLG-001).
type Manifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Authors     []string `json:"authors"`
	Version     string   `json:"version"`
	Game        []string `json:"game"` // supported game versions, "major.minor"
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Requires    []string `json:"requires"`  // must be enabled and load earlier
	After       []string `json:"after"`     // load after these when enabled
	Before      []string `json:"before"`    // load before these when enabled
	Conflicts   []string `json:"conflicts"` // never enabled together
}

// File is one decoded JSON file of a plugin.
type File struct {
	Path string // slash path inside the plugin ("defs/backstories.json")
	Data any    // the decoded tree
}

// Blob is one file read as it is: a script or a texture.
type Blob struct {
	Path string // slash path inside the plugin ("scripts/main.star")
	Data []byte
}

// Plugin is one folder as the adapter found it. Files are sorted by
// path, so the file system's listing order never matters.
type Plugin struct {
	Manifest Manifest
	Dir      string    // where it was found (display only, never read)
	Defs     []File    // defs/*.json
	Patches  []File    // patches/*.json
	Lang     []File    // lang/<locale>.json
	Scripts  []Blob    // scripts/**.star (PLG-060), as written
	Textures []Blob    // textures/**.png (PLG-026 items), as written
	Problems []Problem // what the adapter could not read (bad JSON, unsafe path)
}

// Source lists the installed plugins, sorted by id. Implemented by
// adapters/outbound/modloader.
type Source interface {
	Plugins() ([]Plugin, error)
}

// Problem is one diagnostic with its provenance (MASTER_SPEC §52). Warn
// problems do not stop a plugin from loading; the rest do.
type Problem struct {
	Plugin string // plugin id ("" when it is about no single plugin)
	File   string // slash path inside the plugin
	Def    string // "<class>/<key>"
	Field  string // dotted field path inside the def or manifest
	Msg    string
	Warn   bool
}

func (p Problem) String() string {
	var sb strings.Builder
	for _, part := range []string{p.Plugin, p.File, p.Def, p.Field} {
		if part != "" {
			sb.WriteString(part + ": ")
		}
	}
	if p.Warn {
		sb.WriteString("warning: ")
	}
	sb.WriteString(p.Msg)
	return sb.String()
}

// Placed is one plugin in the load order with the reason it sits there
// (screens §46: the order is explained, never just listed).
type Placed struct {
	ID     string
	Reason string
}

// Order is the resolved load order: Loaded in order (core first), and
// the enabled plugins that could not load, by id.
type Order struct {
	Loaded []Placed
	Failed []string
}

// Has reports whether id loads.
func (o Order) Has(id string) bool {
	for _, p := range o.Loaded {
		if p.ID == id {
			return true
		}
	}
	return false
}

// Def is one definition as a tree, before typing.
type Def struct {
	Class    string // "backstory", "recipe"
	Key      string // "<plugin>.<name>"
	Plugin   string // who defined it
	File     string
	Abstract bool           // a template only (PLG-022)
	Parent   string         // key of the template it inherits from
	Body     map[string]any // every other field
	Touched  []Touch        // patches applied, in order (PLG-032)
}

// Ref is "<class>/<key>", the form patches and problems use.
func (d Def) Ref() string { return d.Class + "/" + d.Key }

// Touch is one patch that changed a def.
type Touch struct {
	Plugin, File string
	Op           string // set, add, remove, merge, replace
	Field        string // "" for the whole def
}

// Op is one patch operation (PLG-030, PLG-031).
type Op struct {
	Plugin, File string
	Index        int    // position in its file, for messages
	Kind         string // set, add, remove, merge, replace
	Target       string // "<class>/<key>"
	Path         string // dotted field path; "" means the whole def
	Value        any
	If           string // run only when this plugin loads
	Missing      string // run only when this plugin does not load
	Optional     bool   // a missing target is not an error
}

func (o Op) problem(def, field, msg string, warn bool) Problem {
	return Problem{Plugin: o.Plugin, File: o.File, Def: def, Field: field,
		Msg: fmt.Sprintf("patch %d (%s): %s", o.Index, o.Kind, msg), Warn: warn}
}
